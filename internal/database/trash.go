package database

import (
	"context"
	"database/sql"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"mediaserver/internal/models"
)

func scanFolder(row *sql.Row) (*models.Folder, error) {
	var id, uid, p string
	var created time.Time
	var deleted sql.NullTime
	if err := row.Scan(&id, &uid, &p, &created, &deleted); err != nil {
		return nil, err
	}
	fid, _ := uuid.Parse(id)
	userID, _ := uuid.Parse(uid)
	f := &models.Folder{ID: fid, UserID: userID, Path: p, CreatedAt: created}
	if deleted.Valid {
		f.DeletedAt = &deleted.Time
	}
	return f, nil
}

func (s *service) CreateFolder(userID uuid.UUID, p string) (*models.Folder, error) {
	p = normalizeFolder(p)
	if p == "/" {
		return nil, fmt.Errorf("cannot create root folder")
	}
	id := uuid.New()
	ctx := context.Background()
	_, err := s.db.ExecContext(ctx, `INSERT INTO folders (id,user_id,path) VALUES (?,?,?) ON CONFLICT(user_id,path) DO UPDATE SET deleted_at=NULL`, id.String(), userID.String(), p)
	if err != nil {
		return nil, err
	}
	return &models.Folder{ID: id, UserID: userID, Path: p, CreatedAt: time.Now()}, nil
}

func (s *service) ListFoldersByUser(userID uuid.UUID) ([]models.Folder, error) {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id,user_id,path,created_at,deleted_at FROM folders WHERE user_id=? AND deleted_at IS NULL ORDER BY path`, userID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Folder
	for rows.Next() {
		var id, uid, p string
		var created time.Time
		var deleted sql.NullTime
		if err := rows.Scan(&id, &uid, &p, &created, &deleted); err != nil {
			return nil, err
		}
		fid, _ := uuid.Parse(id)
		u, _ := uuid.Parse(uid)
		f := models.Folder{ID: fid, UserID: u, Path: p, CreatedAt: created}
		if deleted.Valid {
			f.DeletedAt = &deleted.Time
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func itemRoots(tx *sql.Tx, userID uuid.UUID, items []ItemRef) ([]string, []uuid.UUID, error) {
	var folders []string
	var files []uuid.UUID
	for _, item := range items {
		switch item.Kind {
		case "folder":
			var p string
			if err := tx.QueryRow(`SELECT path FROM folders WHERE id=? AND user_id=? AND deleted_at IS NULL`, item.ID.String(), userID.String()).Scan(&p); err != nil {
				return nil, nil, fmt.Errorf("folder not found")
			}
			folders = append(folders, p)
		case "file":
			var exists int
			if err := tx.QueryRow(`SELECT 1 FROM files WHERE id=? AND user_id=? AND deleted_at IS NULL`, item.ID.String(), userID.String()).Scan(&exists); err != nil {
				return nil, nil, fmt.Errorf("file not found")
			}
			files = append(files, item.ID)
		default:
			return nil, nil, fmt.Errorf("invalid item kind")
		}
	}
	sort.Slice(folders, func(i, j int) bool { return len(folders[i]) < len(folders[j]) })
	roots := folders[:0]
	for _, p := range folders {
		nested := false
		for _, parent := range roots {
			if p == parent || strings.HasPrefix(p, parent+"/") {
				nested = true
				break
			}
		}
		if !nested {
			roots = append(roots, p)
		}
	}
	return roots, files, nil
}

func (s *service) SoftDelete(userID uuid.UUID, items []ItemRef) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	folders, files, err := itemRoots(tx, userID, items)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, id := range files {
		if _, err = tx.Exec(`UPDATE files SET deleted_at=? WHERE id=? AND user_id=?`, now, id.String(), userID.String()); err != nil {
			return err
		}
	}
	for _, p := range folders {
		if _, err = tx.Exec(`UPDATE files SET deleted_at=? WHERE user_id=? AND (folder=? OR folder LIKE ?)`, now, userID.String(), p, p+"/%"); err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE folders SET deleted_at=? WHERE user_id=? AND (path=? OR path LIKE ?)`, now, userID.String(), p, p+"/%"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *service) MoveItems(userID uuid.UUID, items []ItemRef, destination string) error {
	destination = normalizeFolder(destination)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	folders, files, err := itemRoots(tx, userID, items)
	if err != nil {
		return err
	}
	for _, p := range folders {
		if destination == p || strings.HasPrefix(destination, p+"/") {
			return fmt.Errorf("cannot move folder into itself")
		}
		target := normalizeFolder(path.Join(destination, path.Base(p)))
		var n int
		if err = tx.QueryRow(`SELECT COUNT(1) FROM folders WHERE user_id=? AND path=? AND deleted_at IS NULL`, userID.String(), target).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("destination folder already exists")
		}
		if _, err = tx.Exec(`UPDATE files SET folder = CASE WHEN folder=? THEN ? ELSE ? || substr(folder, length(?)+1) END WHERE user_id=? AND (folder=? OR folder LIKE ?) AND deleted_at IS NULL`, p, target, target, p, userID.String(), p, p+"/%"); err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE folders SET path = CASE WHEN path=? THEN ? ELSE ? || substr(path, length(?)+1) END WHERE user_id=? AND (path=? OR path LIKE ?) AND deleted_at IS NULL`, p, target, target, p, userID.String(), p, p+"/%"); err != nil {
			return err
		}
	}
	for _, id := range files {
		if _, err = tx.Exec(`UPDATE files SET folder=? WHERE id=? AND user_id=? AND deleted_at IS NULL`, destination, id.String(), userID.String()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RenameFolder renames a folder in place (keeping the same parent) and
// cascades the change to every descendant folder path and file.folder value,
// reusing the same prefix-rewrite pattern as MoveItems.
func (s *service) RenameFolder(userID, folderID uuid.UUID, newName string) (string, error) {
	newName = strings.Trim(strings.TrimSpace(newName), "/")
	if newName == "" || strings.Contains(newName, "/") {
		return "", fmt.Errorf("invalid folder name")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var p string
	if err := tx.QueryRow(`SELECT path FROM folders WHERE id=? AND user_id=? AND deleted_at IS NULL`, folderID.String(), userID.String()).Scan(&p); err != nil {
		return "", fmt.Errorf("folder not found")
	}
	if p == "/" {
		return "", fmt.Errorf("cannot rename root folder")
	}

	target := normalizeFolder(path.Join(path.Dir(p), newName))
	if target == p {
		return target, tx.Commit()
	}

	var n int
	if err := tx.QueryRow(`SELECT COUNT(1) FROM folders WHERE user_id=? AND path=? AND deleted_at IS NULL`, userID.String(), target).Scan(&n); err != nil {
		return "", err
	}
	if n > 0 {
		return "", fmt.Errorf("a folder with that name already exists")
	}

	if _, err = tx.Exec(`UPDATE files SET folder = CASE WHEN folder=? THEN ? ELSE ? || substr(folder, length(?)+1) END WHERE user_id=? AND (folder=? OR folder LIKE ?) AND deleted_at IS NULL`, p, target, target, p, userID.String(), p, p+"/%"); err != nil {
		return "", err
	}
	if _, err = tx.Exec(`UPDATE folders SET path = CASE WHEN path=? THEN ? ELSE ? || substr(path, length(?)+1) END WHERE user_id=? AND (path=? OR path LIKE ?) AND deleted_at IS NULL`, p, target, target, p, userID.String(), p, p+"/%"); err != nil {
		return "", err
	}

	return target, tx.Commit()
}

func (s *service) ListTrash(userID uuid.UUID) ([]TrashItem, error) {
	ctx := context.Background()
	rows, err := s.db.QueryContext(ctx, `SELECT 'folder',id,path,path,deleted_at,0 FROM folders WHERE user_id=? AND deleted_at IS NOT NULL UNION ALL SELECT 'file',id,filename,folder,deleted_at,size FROM files WHERE user_id=? AND deleted_at IS NOT NULL ORDER BY 5 DESC`, userID.String(), userID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var all []TrashItem
	var deletedFolders []string
	for rows.Next() {
		var x TrashItem
		var id string
		if err = rows.Scan(&x.Kind, &id, &x.Name, &x.Path, &x.DeletedAt, &x.Size); err != nil {
			return nil, err
		}
		x.ID, _ = uuid.Parse(id)
		if x.Kind == "folder" {
			deletedFolders = append(deletedFolders, x.Path)
		}
		all = append(all, x)
	}
	var out []TrashItem
	for _, x := range all {
		hidden := false
		for _, p := range deletedFolders {
			if x.Kind == "folder" && x.Path == p {
				continue
			}
			if strings.HasPrefix(x.Path, p+"/") || (x.Kind == "file" && x.Path == p) {
				hidden = true
				break
			}
		}
		if !hidden {
			out = append(out, x)
		}
	}
	return out, rows.Err()
}

func ensureParents(tx *sql.Tx, userID uuid.UUID, p string) error {
	dir := path.Dir(p)
	if dir == "." || dir == "/" {
		return nil
	}
	cur := ""
	for _, part := range strings.Split(strings.TrimPrefix(dir, "/"), "/") {
		cur += "/" + part
		_, err := tx.Exec(`INSERT INTO folders(id,user_id,path,deleted_at) VALUES(?,?,?,NULL) ON CONFLICT(user_id,path) DO UPDATE SET deleted_at=NULL`, uuid.NewString(), userID.String(), cur)
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *service) RestoreTrash(userID uuid.UUID, items []ItemRef) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range items {
		switch item.Kind {
		case "file":
			var folder string
			if err = tx.QueryRow(`SELECT folder FROM files WHERE id=? AND user_id=? AND deleted_at IS NOT NULL`, item.ID.String(), userID.String()).Scan(&folder); err != nil {
				return fmt.Errorf("file not found")
			}
			if err = ensureParents(tx, userID, folder); err != nil {
				return err
			}
			_, err = tx.Exec(`UPDATE files SET deleted_at=NULL WHERE id=? AND user_id=?`, item.ID.String(), userID.String())
		case "folder":
			var p string
			if err = tx.QueryRow(`SELECT path FROM folders WHERE id=? AND user_id=? AND deleted_at IS NOT NULL`, item.ID.String(), userID.String()).Scan(&p); err != nil {
				return fmt.Errorf("folder not found")
			}
			if err = ensureParents(tx, userID, p); err != nil {
				return err
			}
			_, err = tx.Exec(`UPDATE folders SET deleted_at=NULL WHERE user_id=? AND (path=? OR path LIKE ?)`, userID.String(), p, p+"/%")
			if err == nil {
				_, err = tx.Exec(`UPDATE files SET deleted_at=NULL WHERE user_id=? AND (folder=? OR folder LIKE ?)`, userID.String(), p, p+"/%")
			}
		default:
			return fmt.Errorf("invalid item kind")
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *service) FilesForPurge(userID uuid.UUID, items []ItemRef, expiredOnly bool) ([]models.File, error) {
	if len(items) == 0 {
		return nil, nil
	}
	clauses, args := make([]string, 0), []any{userID.String()}
	for _, it := range items {
		if it.Kind == "file" {
			clauses = append(clauses, "id=?")
			args = append(args, it.ID.String())
			continue
		}
		if it.Kind != "folder" {
			return nil, fmt.Errorf("invalid item kind")
		}
		var p string
		if err := s.db.QueryRow(`SELECT path FROM folders WHERE id=? AND user_id=?`, it.ID.String(), userID.String()).Scan(&p); err != nil {
			return nil, err
		}
		clauses = append(clauses, "(folder=? OR folder LIKE ?)")
		args = append(args, p, p+"/%")
	}
	q := `SELECT id,user_id,filename,mime_type,size,folder,storage_path,created_at,deleted_at FROM files WHERE user_id=? AND deleted_at IS NOT NULL AND (` + strings.Join(clauses, " OR ") + `)`
	if expiredOnly {
		q += " AND deleted_at <= ?"
		args = append(args, time.Now().Add(-30*24*time.Hour))
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.File
	for rows.Next() {
		var f models.File
		var id, uid string
		var d sql.NullTime
		if err = rows.Scan(&id, &uid, &f.Filename, &f.MimeType, &f.Size, &f.Folder, &f.StoragePath, &f.CreatedAt, &d); err != nil {
			return nil, err
		}
		f.ID, _ = uuid.Parse(id)
		f.UserID, _ = uuid.Parse(uid)
		if d.Valid {
			f.DeletedAt = &d.Time
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
func (s *service) PurgeTrash(userID uuid.UUID, items []ItemRef, expiredOnly bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, it := range items {
		if it.Kind == "file" {
			q := `DELETE FROM files WHERE id=? AND user_id=? AND deleted_at IS NOT NULL`
			args := []any{it.ID.String(), userID.String()}
			if expiredOnly {
				q += " AND deleted_at<=?"
				args = append(args, time.Now().Add(-30*24*time.Hour))
			}
			_, err = tx.Exec(q, args...)
		} else if it.Kind == "folder" {
			var p string
			if err = tx.QueryRow(`SELECT path FROM folders WHERE id=? AND user_id=?`, it.ID.String(), userID.String()).Scan(&p); err != nil {
				return err
			}
			q := `DELETE FROM files WHERE user_id=? AND deleted_at IS NOT NULL AND (folder=? OR folder LIKE ?)`
			args := []any{userID.String(), p, p + "/%"}
			if expiredOnly {
				q += " AND deleted_at<=?"
				args = append(args, time.Now().Add(-30*24*time.Hour))
			}
			if _, err = tx.Exec(q, args...); err == nil {
				_, err = tx.Exec(`DELETE FROM folders WHERE user_id=? AND (path=? OR path LIKE ?)`, userID.String(), p, p+"/%")
			}
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *service) ExpiredFilesForPurge(before time.Time) ([]models.File, error) {
	rows, err := s.db.Query(`SELECT id,user_id,filename,mime_type,size,folder,storage_path,created_at,deleted_at FROM files WHERE deleted_at IS NOT NULL AND deleted_at <= ?`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.File
	for rows.Next() {
		var f models.File
		var id, uid string
		var d sql.NullTime
		if err = rows.Scan(&id, &uid, &f.Filename, &f.MimeType, &f.Size, &f.Folder, &f.StoragePath, &f.CreatedAt, &d); err != nil {
			return nil, err
		}
		f.ID, _ = uuid.Parse(id)
		f.UserID, _ = uuid.Parse(uid)
		if d.Valid {
			f.DeletedAt = &d.Time
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *service) PurgeExpired(before time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM files WHERE deleted_at IS NOT NULL AND deleted_at <= ?`, before); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM folders WHERE deleted_at IS NOT NULL AND deleted_at <= ?`, before); err != nil {
		return err
	}
	return tx.Commit()
}
