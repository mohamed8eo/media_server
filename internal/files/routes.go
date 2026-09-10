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
	r.Get("/{id}/thumb", h.ThumbnailHandler)
	r.Get("/recent", h.RecentHandler)
	r.Get("/stats", h.StatsHandler)

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
	absTarget, err1 := filepath.Abs(targetDir)
	absUser, err2 := filepath.Abs(userDir)
	if err1 != nil || err2 != nil || !strings.HasPrefix(absTarget, absUser) {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid folder path")
		return
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

	_ = h.db.UpdateLastAccessed(fileID)

	f, err := os.Open(file.StoragePath)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to open file")
		return
	}
	defer f.Close()

	inlineTypes := []string{"video/", "image/", "application/pdf"}
	serveInline := false
	for _, prefix := range inlineTypes {
		if strings.HasPrefix(file.MimeType, prefix) {
			serveInline = true
			break
		}
	}

	w.Header().Set("Content-Type", file.MimeType)
	if !serveInline {
		w.Header().Set(
			"Content-Disposition",
			fmt.Sprintf(`attachment; filename="%s"`, file.Filename),
		)
	}
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

	absTarget, err1 := filepath.Abs(targetDir)
	absUser, err2 := filepath.Abs(userDir)
	if err1 != nil || err2 != nil || !strings.HasPrefix(absTarget, absUser) {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid folder path")
		return
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

func (h *FileHandler) ThumbnailHandler(w http.ResponseWriter, r *http.Request) {
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
		utils.RespondWithError(w, http.StatusNotFound, "File not found")
		return
	}

	if file.UserID != userID {
		utils.RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	thumbPath, err := GenerateThumbnail(file.StoragePath, file.MimeType)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Thumbnail not available")
		return
	}

	f, err := os.Open(thumbPath)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Thumbnail not available")
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, "thumb.jpg", file.CreatedAt, f)
}

type RecentResponse struct {
	RecentUploads  []models.File `json:"recent_uploads"`
	RecentlyPlayed []models.File `json:"recently_played"`
}

func (h *FileHandler) RecentHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	limit := 10

	recentUploads, err := h.db.ListRecentUploads(userID, limit)
	if err != nil {
		recentUploads = []models.File{}
	}

	recentlyPlayed, err := h.db.ListRecentlyPlayed(userID, limit)
	if err != nil {
		recentlyPlayed = []models.File{}
	}

	utils.RespondWithJSON(w, http.StatusOK, RecentResponse{
		RecentUploads:  recentUploads,
		RecentlyPlayed: recentlyPlayed,
	})
}

type StatsResponse struct {
	TotalSize          int64          `json:"total_size"`
	FileCount          int            `json:"file_count"`
	Categories         map[string]int `json:"categories"`
	TotalSizeFormatted string         `json:"total_size_formatted"`
}

func (h *FileHandler) StatsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	totalSize, err := h.db.GetUserStorageUsage(userID)
	if err != nil {
		totalSize = 0
	}

	categories, err := h.db.GetUserFileCountByCategory(userID)
	if err != nil {
		categories = make(map[string]int)
	}

	fileCount := 0
	for _, count := range categories {
		fileCount += count
	}

	utils.RespondWithJSON(w, http.StatusOK, StatsResponse{
		TotalSize:          totalSize,
		TotalSizeFormatted: models.FormatSize(totalSize),
		FileCount:          fileCount,
		Categories:         categories,
	})
}
