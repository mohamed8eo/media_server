package files

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"mediaserver/internal/database"
	"mediaserver/internal/middleware"
	"mediaserver/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxUploadSize = 5 << 30

type FileHandler struct {
	db          database.Service
	storagePath string
}

func NewRouter(db database.Service) http.Handler {
	h := &FileHandler{
		db:          db,
		storagePath: os.Getenv("STORAGE_PATH"),
	}

	r := chi.NewRouter()

	r.Post("/", h.UploadHandler)
	r.Get("/", h.ListHandler)

	return r
}

func (h *FileHandler) UploadHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "File too large or invalid form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing file in request")
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	userDir := filepath.Join(h.storagePath, userID.String())
	if err = os.MkdirAll(userDir, 0o755); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to prepare storage")
		return
	}

	fileID := uuid.New()
	destPath := filepath.Join(userDir, fileID.String())

	dest, err := os.Create(destPath)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save file")
		return
	}
	defer dest.Close()

	written, err := io.Copy(dest, file)
	if err != nil {
		os.Remove(destPath) // clean up a partial file on failure
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to write file")
		return
	}

	if err := h.db.CreateFile(
		fileID,
		userID,
		header.Filename,
		mimeType,
		written,
		destPath,
	); err != nil {
		os.Remove(destPath)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save file record")
		return
	}
	utils.RespondWithJSON(w, http.StatusCreated, map[string]string{
		"id":       fileID.String(),
		"filename": header.Filename,
		"size":     fmt.Sprintf("%d", written),
	})
}

func (h *FileHandler) ListHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	files, err := h.db.ListFilesByUser(userID)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to list files")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, files)
}
