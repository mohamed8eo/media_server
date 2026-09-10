package database

import (
	"path/filepath"
	"strings"

	"mediaserver/internal/models"

	"github.com/google/uuid"
)

func normalizeFolder(folder string) string {
	if folder == "" || folder == "." {
		return "/"
	}
	cleaned := filepath.Clean("/" + folder)
	cleaned = filepath.ToSlash(cleaned)
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	if cleaned != "/" && strings.HasSuffix(cleaned, "/") {
		cleaned = strings.TrimSuffix(cleaned, "/")
	}
	return cleaned
}

func (s *service) CreateFile(id, userID uuid.UUID, filename, mimeType string, size int64, folder string, storagePath string) error {
	folder = normalizeFolder(folder)
	_, err := s.db.Exec(
		`INSERT INTO files (id, user_id, filename, mime_type, size, folder, storage_path) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.String(), userID.String(), filename, mimeType, size, folder, storagePath,
	)
	return err
}

func (s *service) ListFilesByUser(userID uuid.UUID) ([]models.File, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, filename, mime_type, size, folder, storage_path, created_at, last_accessed FROM files WHERE user_id = ? ORDER BY created_at DESC`,
		userID.String(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []models.File
	for rows.Next() {
		var f models.File
		var idStr, userIDStr string
		if err := rows.Scan(&idStr, &userIDStr, &f.Filename, &f.MimeType, &f.Size, &f.Folder, &f.StoragePath, &f.CreatedAt, &f.LastAccessed); err != nil {
			return nil, err
		}
		f.ID, _ = uuid.Parse(idStr)
		f.UserID, _ = uuid.Parse(userIDStr)
		f.Folder = normalizeFolder(f.Folder)
		files = append(files, f)
	}
	return files, rows.Err()
}

func (s *service) GetFileByID(fileID uuid.UUID) (*models.File, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, filename, mime_type, size, folder, storage_path, created_at, last_accessed FROM files WHERE id = ?`,
		fileID.String(),
	)

	var f models.File
	var idStr, userIDStr string
	if err := row.Scan(&idStr, &userIDStr, &f.Filename, &f.MimeType, &f.Size, &f.Folder, &f.StoragePath, &f.CreatedAt, &f.LastAccessed); err != nil {
		return nil, err
	}
	f.ID, _ = uuid.Parse(idStr)
	f.UserID, _ = uuid.Parse(userIDStr)
	f.Folder = normalizeFolder(f.Folder)
	return &f, nil
}

func (s *service) UpdateLastAccessed(fileID uuid.UUID) error {
	_, err := s.db.Exec(
		`UPDATE files SET last_accessed = CURRENT_TIMESTAMP WHERE id = ?`,
		fileID.String(),
	)
	return err
}

func (s *service) ListRecentUploads(userID uuid.UUID, limit int) ([]models.File, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(
		`SELECT id, user_id, filename, mime_type, size, folder, storage_path, created_at, last_accessed FROM files WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`,
		userID.String(), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []models.File
	for rows.Next() {
		var f models.File
		var idStr, userIDStr string
		if err := rows.Scan(&idStr, &userIDStr, &f.Filename, &f.MimeType, &f.Size, &f.Folder, &f.StoragePath, &f.CreatedAt, &f.LastAccessed); err != nil {
			return nil, err
		}
		f.ID, _ = uuid.Parse(idStr)
		f.UserID, _ = uuid.Parse(userIDStr)
		f.Folder = normalizeFolder(f.Folder)
		files = append(files, f)
	}
	return files, rows.Err()
}

func (s *service) ListRecentlyPlayed(userID uuid.UUID, limit int) ([]models.File, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(
		`SELECT id, user_id, filename, mime_type, size, folder, storage_path, created_at, last_accessed FROM files WHERE user_id = ? AND last_accessed IS NOT NULL ORDER BY last_accessed DESC LIMIT ?`,
		userID.String(), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []models.File
	for rows.Next() {
		var f models.File
		var idStr, userIDStr string
		if err := rows.Scan(&idStr, &userIDStr, &f.Filename, &f.MimeType, &f.Size, &f.Folder, &f.StoragePath, &f.CreatedAt, &f.LastAccessed); err != nil {
			return nil, err
		}
		f.ID, _ = uuid.Parse(idStr)
		f.UserID, _ = uuid.Parse(userIDStr)
		f.Folder = normalizeFolder(f.Folder)
		files = append(files, f)
	}
	return files, rows.Err()
}

func (s *service) GetUserStorageUsage(userID uuid.UUID) (int64, error) {
	var total int64
	err := s.db.QueryRow(
		`SELECT COALESCE(SUM(size), 0) FROM files WHERE user_id = ?`,
		userID.String(),
	).Scan(&total)
	return total, err
}

func (s *service) GetUserFileCountByCategory(userID uuid.UUID) (map[string]int, error) {
	rows, err := s.db.Query(
		`SELECT mime_type FROM files WHERE user_id = ?`,
		userID.String(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var mimeType string
		if err := rows.Scan(&mimeType); err != nil {
			return nil, err
		}
		mime := strings.ToLower(mimeType)
		switch {
		case strings.HasPrefix(mime, "video/"):
			counts["video"]++
		case strings.HasPrefix(mime, "image/"):
			counts["image"]++
		case strings.HasPrefix(mime, "audio/"):
			counts["audio"]++
		case strings.HasPrefix(mime, "application/pdf") || strings.HasPrefix(mime, "text/") || strings.Contains(mime, "document") || strings.Contains(mime, "word") || strings.Contains(mime, "sheet"):
			counts["document"]++
		default:
			counts["other"]++
		}
	}
	return counts, rows.Err()
}

func (s *service) DeleteFile(fileID uuid.UUID) error {
	_, err := s.db.Exec(`DELETE FROM files WHERE id = ?`, fileID.String())
	return err
}

func (s *service) UpdateFilename(fileID uuid.UUID, filename string) error {
	_, err := s.db.Exec(`UPDATE files SET filename = ? WHERE id = ?`, filename, fileID.String())
	return err
}

func (s *service) UpdateFileFolder(fileID uuid.UUID, folder string) error {
	folder = normalizeFolder(folder)
	_, err := s.db.Exec(`UPDATE files SET folder = ? WHERE id = ?`, folder, fileID.String())
	return err
}
