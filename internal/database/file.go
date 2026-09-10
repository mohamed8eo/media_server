package database

import (
	"mediaserver/internal/models"

	"github.com/google/uuid"
)

func (s *service) CreateFile(id, userID uuid.UUID, filename, mimeType string, size int64, folder string, storagePath string) error {
	if folder == "" {
		folder = "/"
	}
	_, err := s.db.Exec(
		`INSERT INTO files (id, user_id, filename, mime_type, size, folder, storage_path) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.String(), userID.String(), filename, mimeType, size, folder, storagePath,
	)
	return err
}

func (s *service) ListFilesByUser(userID uuid.UUID) ([]models.File, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, filename, mime_type, size, folder, storage_path, created_at FROM files WHERE user_id = ? ORDER BY created_at DESC`,
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
		if err := rows.Scan(&idStr, &userIDStr, &f.Filename, &f.MimeType, &f.Size, &f.Folder, &f.StoragePath, &f.CreatedAt); err != nil {
			return nil, err
		}
		f.ID, _ = uuid.Parse(idStr)
		f.UserID, _ = uuid.Parse(userIDStr)
		files = append(files, f)
	}
	return files, rows.Err()
}

func (s *service) GetFileByID(fileID uuid.UUID) (*models.File, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, filename, mime_type, size, folder, storage_path, created_at FROM files WHERE id = ?`,
		fileID.String(),
	)

	var f models.File
	var idStr, userIDStr string
	if err := row.Scan(&idStr, &userIDStr, &f.Filename, &f.MimeType, &f.Size, &f.Folder, &f.StoragePath, &f.CreatedAt); err != nil {
		return nil, err
	}
	f.ID, _ = uuid.Parse(idStr)
	f.UserID, _ = uuid.Parse(userIDStr)
	return &f, nil
}
