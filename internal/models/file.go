package models

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type File struct {
	ID               uuid.UUID `json:"id"`
	UserID           uuid.UUID `json:"user_id"`
	Filename         string    `json:"filename"`
	MimeType         string    `json:"mime_type"`
	Size             int64     `json:"size"`
	Folder           string    `json:"folder"`
	StoragePath      string    `json:"-"`
	PlaybackProgress int       `json:"playback_progress"`
	CreatedAt        time.Time `json:"created_at"`
	LastAccessed     *time.Time `json:"last_accessed"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

// Folder is a first-class directory. Paths are unique per user and normalized
// with a leading slash; the root itself is implicit and has no row.
type Folder struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	Path      string     `json:"path"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

func (f *File) DisplayName() string {
	name := f.Filename
	ext := filepath.Ext(name)
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	return name
}

func (f *File) Extension() string {
	ext := filepath.Ext(f.Filename)
	if ext == "" {
		return ""
	}
	return strings.TrimPrefix(ext, ".")
}

func (f *File) SizeFormatted() string {
	return FormatSize(f.Size)
}

func FormatSize(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func (f *File) Category() string {
	mime := strings.ToLower(f.MimeType)
	switch {
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	case strings.HasPrefix(mime, "application/pdf"):
		return "document"
	case strings.HasPrefix(mime, "text/"):
		return "document"
	case strings.Contains(mime, "document") || strings.Contains(mime, "word") || strings.Contains(mime, "sheet") || strings.Contains(mime, "presentation"):
		return "document"
	default:
		return "other"
	}
}

func (f *File) TypeBadge() string {
	ext := strings.ToUpper(f.Extension())
	if ext == "" {
		mime := strings.ToLower(f.MimeType)
		switch {
		case strings.HasPrefix(mime, "video/"):
			return strings.TrimPrefix(mime, "video/")
		case strings.HasPrefix(mime, "image/"):
			return strings.TrimPrefix(mime, "image/")
		case strings.HasPrefix(mime, "audio/"):
			return strings.TrimPrefix(mime, "audio/")
		default:
			return "FILE"
		}
	}
	return ext
}
