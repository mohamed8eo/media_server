package models

import (
	"time"

	"github.com/google/uuid"
)

type File struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Filename    string    `json:"filename"`
	MimeType    string    `json:"mime_type"`
	Size        int64     `json:"size"`
	Folder      string    `json:"folder"`
	StoragePath string    `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}
