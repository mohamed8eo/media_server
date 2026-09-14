package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"mediaserver/internal/database/sqlc"
	"mediaserver/internal/models"

	"github.com/google/uuid"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// Service represents a service that interacts with a database.
type Service interface {
	Health() map[string]string
	Close() error
	CreateUser(email, passwordHash string) (uuid.UUID, error)
	GetUserByEmail(email string) (*models.User, error)
	GetUserByID(id uuid.UUID) (*models.User, error)
	UpdatePasswordByEmail(email, passwordHash string) error
	StoreRefreshToken(userID uuid.UUID, token string, expiresAt time.Time) (uuid.UUID, error)
	GetRefreshToken(token string) (*models.RefreshToken, error)
	RevokeRefreshToken(token string) error
	RevokeAllUserRefreshTokens(userID uuid.UUID) error
	CreateFile(id, userID uuid.UUID, filename, mimeType string, size int64, folder string, storagePath string, sha256 string) error
	ListFilesByUser(userID uuid.UUID) ([]models.File, error)
	GetFileByID(fileID uuid.UUID) (*models.File, error)
	GetFileByUserAndSHA256(userID uuid.UUID, sha256 string) (*models.File, error)
	UpdateLastAccessed(fileID uuid.UUID) error
	ListRecentUploads(userID uuid.UUID, limit int) ([]models.File, error)
	ListRecentlyPlayed(userID uuid.UUID, limit int) ([]models.File, error)
	GetUserStorageUsage(userID uuid.UUID) (int64, error)
	GetUserFileCountByCategory(userID uuid.UUID) (map[string]int, error)
	DeleteFile(fileID uuid.UUID) error
	UpdateFilename(fileID uuid.UUID, filename string) error
	UpdateFileFolder(fileID uuid.UUID, folder string) error
	UpdateFileSize(fileID uuid.UUID, size int64) error
	UpdatePlaybackProgress(fileID uuid.UUID, progress int) error
	CreateFolder(userID uuid.UUID, path string) (*models.Folder, error)
	ListFoldersByUser(userID uuid.UUID) ([]models.Folder, error)
	SoftDelete(userID uuid.UUID, items []ItemRef) error
	MoveItems(userID uuid.UUID, items []ItemRef, destination string) error
	RenameFolder(userID, folderID uuid.UUID, newName string) (string, error)
	ListTrash(userID uuid.UUID) ([]TrashItem, error)
	RestoreTrash(userID uuid.UUID, items []ItemRef) error
	FilesForPurge(userID uuid.UUID, items []ItemRef, expiredOnly bool) ([]models.File, error)
	PurgeTrash(userID uuid.UUID, items []ItemRef, expiredOnly bool) error
	ExpiredFilesForPurge(before time.Time) ([]models.File, error)
	PurgeExpired(before time.Time) error
	CreateJob(id, fileID, userID uuid.UUID, taskType string) error
	GetJobByID(id uuid.UUID) (Job, error)
	GetPendingOrProcessingJobs() ([]Job, error)
	ListActiveJobsByUser(userID uuid.UUID) ([]Job, error)
	UpdateJobStatus(id uuid.UUID, status string, errMsg string) error
	UpdateJobProgress(id uuid.UUID, progress int) error
}

type ItemRef struct {
	Kind string    `json:"kind"`
	ID   uuid.UUID `json:"id"`
}
type TrashItem struct {
	Kind      string    `json:"kind"`
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	DeletedAt time.Time `json:"deleted_at"`
	Size      int64     `json:"size,omitempty"`
}

type service struct {
	db      *sql.DB
	queries *sqlc.Queries
}

var dbInstance *service

func New() Service {
	// Reuse Connection
	if dbInstance != nil {
		return dbInstance
	}

	dburl := os.Getenv("BLUEPRINT_DB_URL")
	if dburl == "" {
		slog.Error("BLUEPRINT_DB_URL is required; refusing to open an ephemeral SQLite database")
		os.Exit(1)
	}

	if !strings.Contains(dburl, "_journal_mode") {
		if strings.Contains(dburl, "?") {
			dburl += "&_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000"
		} else {
			dburl += "?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000"
		}
	}

	db, err := sql.Open("sqlite3", dburl)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		slog.Error("failed to set goose dialect", "error", err)
		os.Exit(1)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		slog.Error("failed to run goose migrations", "error", err)
		os.Exit(1)
	}

	dbInstance = &service{
		db:      db,
		queries: sqlc.New(db),
	}
	return dbInstance
}

// Health checks the health of the database connection by pinging the database.
func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	stats := make(map[string]string)

	err := s.db.PingContext(ctx)
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)
		return stats
	}

	stats["status"] = "up"
	stats["message"] = "It's healthy"

	dbStats := s.db.Stats()
	stats["open_connections"] = strconv.Itoa(dbStats.OpenConnections)
	stats["in_use"] = strconv.Itoa(dbStats.InUse)
	stats["idle"] = strconv.Itoa(dbStats.Idle)
	stats["wait_count"] = strconv.FormatInt(dbStats.WaitCount, 10)
	stats["wait_duration"] = dbStats.WaitDuration.String()
	stats["max_idle_closed"] = strconv.FormatInt(dbStats.MaxIdleClosed, 10)
	stats["max_lifetime_closed"] = strconv.FormatInt(dbStats.MaxLifetimeClosed, 10)

	return stats
}

// Close closes the database connection.
func (s *service) Close() error {
	slog.Info("Disconnected from database", "url", os.Getenv("BLUEPRINT_DB_URL"))
	return s.db.Close()
}
