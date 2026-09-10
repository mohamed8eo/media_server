package files

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mediaserver/internal/database"
	"mediaserver/internal/middleware"
	"mediaserver/internal/models"
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
	r.Post("/mkdir", h.MkdirHandler)
	r.Get("/{id}", h.GetFileHandler)

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

	folder := r.FormValue("folder")
	if folder == "" || folder == "/" {
		folder = ""
	}
	folder = filepath.Clean(folder)
	if folder == "." {
		folder = ""
	}

	userDir := filepath.Join(h.storagePath, userID.String())
	targetDir := userDir
	if folder != "" {
		targetDir = filepath.Join(userDir, folder)
	}
	if err = os.MkdirAll(targetDir, 0o755); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to prepare storage")
		return
	}

	fileID := uuid.New()
	destPath := filepath.Join(targetDir, fileID.String())

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

	dbFolder := "/"
	if folder != "" {
		dbFolder = "/" + folder
	}

	if err := h.db.CreateFile(
		fileID,
		userID,
		header.Filename,
		mimeType,
		written,
		dbFolder,
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

func (h *FileHandler) GetFileHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	fileIDstr := chi.URLParam(r, "id")
	fileID, err := uuid.Parse(fileIDstr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "invalid file ID")
		return
	}

	file, err := h.db.GetFileByID(fileID)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if file.UserID != userID {
		utils.RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	f, err := os.Open(file.StoragePath)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to open file")
		return
	}
	defer f.Close()

	if strings.HasPrefix(file.MimeType, "video/") {
		w.Header().Set("Content-Type", file.MimeType)
		http.ServeContent(
			w,
			r,
			file.Filename,
			file.CreatedAt,
			f,
		)
		return
	}

	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set(
		"Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, file.Filename),
	)
	http.ServeContent(
		w,
		r,
		file.Filename,
		file.CreatedAt,
		f,
	)
}

func (h *FileHandler) MkdirHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing folder path")
		return
	}

	cleaned := filepath.Clean(req.Path)
	if cleaned == "." || cleaned == "/" {
		cleaned = ""
	}

	userDir := filepath.Join(h.storagePath, userID.String())
	targetDir := userDir
	if cleaned != "" {
		targetDir = filepath.Join(userDir, cleaned)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create folder")
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, map[string]string{
		"path": "/" + cleaned,
	})
}

type ListResponse struct {
	Files   []models.File `json:"files"`
	Folders []string      `json:"folders"`
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

	userDir := filepath.Join(h.storagePath, userID.String())
	folders := scanFolders(userDir, userDir)

	utils.RespondWithJSON(w, http.StatusOK, ListResponse{
		Files:   files,
		Folders: folders,
	})
}

func scanFolders(root, base string) []string {
	folderSet := make(map[string]bool)
	folderSet["/"] = true

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}
		folder := "/" + filepath.ToSlash(rel)
		folderSet[folder] = true
		return nil
	})

	folders := make([]string, 0, len(folderSet))
	for f := range folderSet {
		folders = append(folders, f)
	}

	sorted := make([]string, 0, len(folders))
	roots := make([]string, 0)
	children := make(map[string][]string)

	for _, f := range folders {
		if f == "/" {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(f, "/"), "/")
		if len(parts) == 1 {
			roots = append(roots, f)
		} else {
			parent := "/" + strings.Join(parts[:len(parts)-1], "/")
			children[parent] = append(children[parent], f)
		}
	}

	var walk func(folder string)
	walk = func(folder string) {
		sorted = append(sorted, folder)
		for _, child := range children[folder] {
			walk(child)
		}
	}

	for _, r := range roots {
		walk(r)
	}

	return sorted
}
