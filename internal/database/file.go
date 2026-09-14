package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"time"

	"mediaserver/internal/database/sqlc"
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

func (s *service) CreateFile(id, userID uuid.UUID, filename, mimeType string, size int64, folder string, storagePath string, sha256 string) error {
	folder = normalizeFolder(folder)
	ctx := context.Background()
	return s.queries.CreateFile(ctx, sqlc.CreateFileParams{
		ID:          id.String(),
		UserID:      userID.String(),
		Filename:    filename,
		MimeType:    mimeType,
		Size:        size,
		Folder:      folder,
		StoragePath: storagePath,
		Sha256:      sql.NullString{String: sha256, Valid: sha256 != ""},
	})
}

func (s *service) GetFileByUserAndSHA256(userID uuid.UUID, sha256 string) (*models.File, error) {
	if sha256 == "" {
		return nil, sql.ErrNoRows
	}
	ctx := context.Background()
	f, err := s.queries.GetFileByUserAndSHA256(ctx, sqlc.GetFileByUserAndSHA256Params{
		UserID: userID.String(),
		Sha256: sql.NullString{String: sha256, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	fid, _ := uuid.Parse(f.ID)
	uid, _ := uuid.Parse(f.UserID)
	var createdAt time.Time
	if f.CreatedAt.Valid {
		createdAt = f.CreatedAt.Time
	}
	var lastAccessed *time.Time
	if f.LastAccessed.Valid {
		t := f.LastAccessed.Time
		lastAccessed = &t
	}
	var shaVal string
	if f.Sha256.Valid {
		shaVal = f.Sha256.String
	}

	return &models.File{
		ID:               fid,
		UserID:           uid,
		Filename:         f.Filename,
		MimeType:         f.MimeType,
		Size:             f.Size,
		Folder:           normalizeFolder(f.Folder),
		StoragePath:      f.StoragePath,
		PlaybackProgress: int(f.PlaybackProgress.Int64),
		SHA256:           shaVal,
		CreatedAt:        createdAt,
		LastAccessed:     lastAccessed,
	}, nil
}

func (s *service) ListFilesByUser(userID uuid.UUID) ([]models.File, error) {
	ctx := context.Background()
	dbFiles, err := s.queries.ListFilesByUser(ctx, userID.String())
	if err != nil {
		return nil, err
	}

	var files []models.File
	for _, f := range dbFiles {
		fid, _ := uuid.Parse(f.ID)
		uid, _ := uuid.Parse(f.UserID)
		var createdAt time.Time
		if f.CreatedAt.Valid {
			createdAt = f.CreatedAt.Time
		}
		var lastAccessed *time.Time
		if f.LastAccessed.Valid {
			t := f.LastAccessed.Time
			lastAccessed = &t
		}

		files = append(files, models.File{
			ID:               fid,
			UserID:           uid,
			Filename:         f.Filename,
			MimeType:         f.MimeType,
			Size:             f.Size,
			Folder:           normalizeFolder(f.Folder),
			StoragePath:      f.StoragePath,
			PlaybackProgress: int(f.PlaybackProgress.Int64),
			SHA256:           f.Sha256.String,
			CreatedAt:        createdAt,
			LastAccessed:     lastAccessed,
		})
	}
	return files, nil
}

func (s *service) GetFileByID(fileID uuid.UUID) (*models.File, error) {
	ctx := context.Background()
	f, err := s.queries.GetFileByID(ctx, fileID.String())
	if err != nil {
		return nil, err
	}

	fid, _ := uuid.Parse(f.ID)
	uid, _ := uuid.Parse(f.UserID)
	var createdAt time.Time
	if f.CreatedAt.Valid {
		createdAt = f.CreatedAt.Time
	}
	var lastAccessed *time.Time
	if f.LastAccessed.Valid {
		t := f.LastAccessed.Time
		lastAccessed = &t
	}

	return &models.File{
		ID:               fid,
		UserID:           uid,
		Filename:         f.Filename,
		MimeType:         f.MimeType,
		Size:             f.Size,
		Folder:           normalizeFolder(f.Folder),
		StoragePath:      f.StoragePath,
		PlaybackProgress: int(f.PlaybackProgress.Int64),
		SHA256:           f.Sha256.String,
		CreatedAt:        createdAt,
		LastAccessed:     lastAccessed,
	}, nil
}

func (s *service) UpdateLastAccessed(fileID uuid.UUID) error {
	ctx := context.Background()
	return s.queries.UpdateLastAccessed(ctx, fileID.String())
}

func (s *service) ListRecentUploads(userID uuid.UUID, limit int) ([]models.File, error) {
	if limit <= 0 {
		limit = 10
	}
	ctx := context.Background()
	dbFiles, err := s.queries.ListRecentUploads(ctx, sqlc.ListRecentUploadsParams{
		UserID: userID.String(),
		Limit:  int64(limit),
	})
	if err != nil {
		return nil, err
	}

	var files []models.File
	for _, f := range dbFiles {
		fid, _ := uuid.Parse(f.ID)
		uid, _ := uuid.Parse(f.UserID)
		var createdAt time.Time
		if f.CreatedAt.Valid {
			createdAt = f.CreatedAt.Time
		}
		var lastAccessed *time.Time
		if f.LastAccessed.Valid {
			t := f.LastAccessed.Time
			lastAccessed = &t
		}

		files = append(files, models.File{
			ID:               fid,
			UserID:           uid,
			Filename:         f.Filename,
			MimeType:         f.MimeType,
			Size:             f.Size,
			Folder:           normalizeFolder(f.Folder),
			StoragePath:      f.StoragePath,
			PlaybackProgress: int(f.PlaybackProgress.Int64),
			SHA256:           f.Sha256.String,
			CreatedAt:        createdAt,
			LastAccessed:     lastAccessed,
		})
	}
	return files, nil
}

func (s *service) ListRecentlyPlayed(userID uuid.UUID, limit int) ([]models.File, error) {
	if limit <= 0 {
		limit = 10
	}
	ctx := context.Background()
	dbFiles, err := s.queries.ListRecentlyPlayed(ctx, sqlc.ListRecentlyPlayedParams{
		UserID: userID.String(),
		Limit:  int64(limit),
	})
	if err != nil {
		return nil, err
	}

	var files []models.File
	for _, f := range dbFiles {
		fid, _ := uuid.Parse(f.ID)
		uid, _ := uuid.Parse(f.UserID)
		var createdAt time.Time
		if f.CreatedAt.Valid {
			createdAt = f.CreatedAt.Time
		}
		var lastAccessed *time.Time
		if f.LastAccessed.Valid {
			t := f.LastAccessed.Time
			lastAccessed = &t
		}

		files = append(files, models.File{
			ID:               fid,
			UserID:           uid,
			Filename:         f.Filename,
			MimeType:         f.MimeType,
			Size:             f.Size,
			Folder:           normalizeFolder(f.Folder),
			StoragePath:      f.StoragePath,
			PlaybackProgress: int(f.PlaybackProgress.Int64),
			SHA256:           f.Sha256.String,
			CreatedAt:        createdAt,
			LastAccessed:     lastAccessed,
		})
	}
	return files, nil
}

func (s *service) GetUserStorageUsage(userID uuid.UUID) (int64, error) {
	ctx := context.Background()
	val, err := s.queries.GetUserStorageUsage(ctx, userID.String())
	if err != nil {
		return 0, err
	}
	switch v := val.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, nil
	}
}

func (s *service) GetUserFileCountByCategory(userID uuid.UUID) (map[string]int, error) {
	ctx := context.Background()
	rows, err := s.queries.GetFileCountByCategory(ctx, userID.String())
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, row := range rows {
		counts[row.Category] = int(row.Count)
	}
	return counts, nil
}

func (s *service) DeleteFile(fileID uuid.UUID) error {
	ctx := context.Background()
	return s.queries.DeleteFile(ctx, fileID.String())
}

func (s *service) UpdateFilename(fileID uuid.UUID, filename string) error {
	ctx := context.Background()
	return s.queries.UpdateFilename(ctx, sqlc.UpdateFilenameParams{
		Filename: filename,
		ID:       fileID.String(),
	})
}

func (s *service) UpdateFileFolder(fileID uuid.UUID, folder string) error {
	folder = normalizeFolder(folder)
	ctx := context.Background()
	return s.queries.UpdateFileFolder(ctx, sqlc.UpdateFileFolderParams{
		Folder: folder,
		ID:     fileID.String(),
	})
}

func (s *service) UpdateFileSize(fileID uuid.UUID, size int64) error {
	ctx := context.Background()
	return s.queries.UpdateFileSize(ctx, sqlc.UpdateFileSizeParams{
		Size: size,
		ID:   fileID.String(),
	})
}

func (s *service) UpdatePlaybackProgress(fileID uuid.UUID, progress int) error {
	ctx := context.Background()
	return s.queries.UpdatePlaybackProgress(ctx, sqlc.UpdatePlaybackProgressParams{
		PlaybackProgress: sql.NullInt64{Int64: int64(progress), Valid: true},
		ID:               fileID.String(),
	})
}
