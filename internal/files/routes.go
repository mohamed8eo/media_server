package files

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"mediaserver/internal/database"
	"mediaserver/internal/jobqueue"
	"mediaserver/internal/middleware"
	"mediaserver/internal/models"
	"mediaserver/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxUploadSize = 10 << 30

type FileHandler struct {
	db          database.Service
	storagePath string
	audioPool   *jobqueue.Pool
	thumbPool   *jobqueue.Pool
}

func (h *FileHandler) ensureFolderHierarchy(userID uuid.UUID, folder string) error {
	folder = normalizeFolder(folder)
	if folder == "/" {
		return nil
	}
	current := ""
	for _, part := range strings.Split(strings.TrimPrefix(folder, "/"), "/") {
		current += "/" + part
		if _, err := h.db.CreateFolder(userID, current); err != nil {
			return err
		}
	}
	return nil
}

func NewRouter(db database.Service) http.Handler {
	h := &FileHandler{
		db:          db,
		storagePath: os.Getenv("STORAGE_PATH"),
		audioPool:   GlobalAudioPool,
		thumbPool:   jobqueue.NewPool(runtime.NumCPU(), 50),
	}

	r := chi.NewRouter()

	r.Post("/", h.UploadHandler)
	r.Post("/download-url", h.DownloadURLHandler)
	r.Get("/jobs/{id}", h.GetJobHandler)
	r.Get("/", h.ListHandler)
	r.Post("/mkdir", h.MkdirHandler)
	r.Patch("/folder/{id}/rename", h.RenameFolderHandler)
	r.Get("/trash", h.TrashHandler)
	r.Post("/batch/delete", h.BatchDeleteHandler)
	r.Patch("/batch/move", h.BatchMoveHandler)
	r.Post("/batch/download", h.BatchDownloadHandler)
	r.Post("/trash/restore", h.RestoreTrashHandler)
	r.Post("/trash/purge", h.PurgeTrashHandler)
	r.Post("/trash/empty", h.EmptyTrashHandler)
	r.Get("/{id}", h.GetFileHandler)
	r.Get("/{id}/{filename}", h.GetFileHandler)
	r.Get("/{id}/thumb", h.ThumbnailHandler)
	r.Delete("/{id}", h.DeleteFileHandler)
	r.Patch("/{id}/rename", h.RenameFileHandler)
	r.Patch("/{id}/move", h.MoveFileHandler)
	r.Patch("/{id}/progress", h.UpdatePlaybackProgressHandler)
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

	reader, err := r.MultipartReader()
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid multipart request")
		return
	}

	var filename, mimeType string
	var folder string
	var destPath string
	var fileID uuid.UUID
	var written int64
	var fileSaved bool
	var fileHash string

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Error("MultipartReader NextPart failed", "error", err)
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid form data: "+err.Error())
			return
		}

		fieldName := part.FormName()
		if fieldName == "folder" {
			buf := new(bytes.Buffer)
			_, _ = io.Copy(buf, part)
			folder = buf.String()
			continue
		}

		if fieldName == "file" {
			filename = part.FileName()
			if filename == "" {
				filename = "upload"
			}
			mimeType = part.Header.Get("Content-Type")
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}

			if folder == "" || folder == "/" {
				folder = ""
			} else {
				folder = strings.TrimPrefix(folder, "/")
			}
			folder = filepath.Clean(folder)
			if folder == "." || folder == "" {
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
				slog.Warn("file.upload.rejected", "user_id", userID, "ip", r.RemoteAddr, "reason", "path_traversal")
				utils.RespondWithError(w, http.StatusBadRequest, "Invalid folder path")
				return
			}

			if err = os.MkdirAll(targetDir, 0o755); err != nil {
				utils.RespondWithError(w, http.StatusInternalServerError, "Failed to prepare storage")
				return
			}

			fileID = uuid.New()
			destPath = filepath.Join(targetDir, fileID.String())

			dest, err := os.Create(destPath)
			if err != nil {
				utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save file")
				return
			}

			buf := make([]byte, 4*1024*1024)
			written, err = io.CopyBuffer(dest, part, buf)
			dest.Close()
			if err != nil {
				os.Remove(destPath)
				utils.RespondWithError(w, http.StatusInternalServerError, "Failed to write file")
				return
			}

			fileSaved = true
		}
	}

	if !fileSaved {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing file in request")
		return
	}

	dbFolder := "/"
	if folder != "" {
		dbFolder = "/" + folder
	}
	if err := h.ensureFolderHierarchy(userID, dbFolder); err != nil {
		os.Remove(destPath)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save folder")
		return
	}

	if err := h.db.CreateFile(
		fileID,
		userID,
		filename,
		mimeType,
		written,
		dbFolder,
		destPath,
		fileHash,
	); err != nil {
		os.Remove(destPath)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save file record")
		return
	}

	if strings.HasPrefix(mimeType, "video/") {
		jobID := uuid.New()
		_ = h.db.CreateJob(jobID, fileID, userID, "media_fix")
		h.audioPool.Submit(func() {
			_ = h.db.UpdateJobStatus(jobID, "processing", "")
			if err := h.fixMediaIfNeeded(fileID, jobID, destPath, mimeType); err != nil {
				slog.Error("media fix failed", "file_id", fileID, "error", err)
				_ = h.db.UpdateJobStatus(jobID, "failed", err.Error())
			} else {
				_ = h.db.UpdateJobStatus(jobID, "completed", "")
			}
		})
		h.thumbPool.Submit(func() {
			if _, err := GenerateThumbnail(destPath, mimeType); err != nil {
				slog.Error("thumbnail generation failed", "file_id", fileID, "error", err)
			}
		})
	} else if strings.HasPrefix(mimeType, "image/") {
		h.thumbPool.Submit(func() {
			if _, err := GenerateThumbnail(destPath, mimeType); err != nil {
				slog.Error("thumbnail generation failed", "file_id", fileID, "error", err)
			}
		})
	}

	w.Header().Set("HX-Trigger", "mediaUpdated")
	utils.RespondWithJSON(w, http.StatusCreated, map[string]string{
		"id":       fileID.String(),
		"filename": filename,
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
	if serveInline {
		w.Header().Set(
			"Content-Disposition",
			fmt.Sprintf(`inline; filename="%s"`, file.Filename),
		)
	} else {
		w.Header().Set(
			"Content-Disposition",
			fmt.Sprintf(`attachment; filename="%s"`, file.Filename),
		)
	}
	info, err := f.Stat()
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to read file info")
		return
	}
	http.ServeContent(
		w,
		r,
		file.Filename,
		info.ModTime(),
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

	cleaned := filepath.Clean(strings.TrimPrefix(req.Path, "/"))
	if cleaned == "." || cleaned == "" {
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
	if err := h.ensureFolderHierarchy(userID, "/"+cleaned); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save folder")
		return
	}

	w.Header().Set("HX-Trigger", "mediaUpdated")
	utils.RespondWithJSON(w, http.StatusCreated, map[string]string{
		"path": "/" + cleaned,
	})
}

func (h *FileHandler) RenameFolderHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	folderIDstr := chi.URLParam(r, "id")
	folderID, err := uuid.Parse(folderIDstr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid folder ID")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing folder name")
		return
	}

	newPath, err := h.db.RenameFolder(userID, folderID, req.Name)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("HX-Trigger", "mediaUpdated")
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"path": newPath})
}

type ListResponse struct {
	Files   []models.File   `json:"files"`
	Folders []models.Folder `json:"folders"`
	Jobs    []database.Job  `json:"jobs"`
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

	folders, err := h.db.ListFoldersByUser(userID)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to list folders")
		return
	}

	jobs, err := h.db.ListActiveJobsByUser(userID)
	if err != nil {
		jobs = []database.Job{}
	}

	utils.RespondWithJSON(w, http.StatusOK, ListResponse{
		Files:   files,
		Folders: folders,
		Jobs:    jobs,
	})
}

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

func extractFolders(files []models.File) []string {
	folderSet := make(map[string]bool)
	folderSet["/"] = true

	for _, f := range files {
		folder := normalizeFolder(f.Folder)
		if folder == "" {
			folder = "/"
		}
		folderSet[folder] = true

		parts := strings.Split(strings.TrimPrefix(folder, "/"), "/")
		curr := ""
		for _, p := range parts {
			if p == "" {
				continue
			}
			curr += "/" + p
			folderSet[curr] = true
		}
	}

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

	newSorted := []string{"/"}
	for _, f := range sorted {
		if f != "/" {
			newSorted = append(newSorted, f)
		}
	}

	return newSorted
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

	thumbPath, genErr := GenerateThumbnail(file.StoragePath, file.MimeType)
	if genErr != nil {
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
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	info, err := f.Stat()
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to read file info")
		return
	}
	http.ServeContent(w, r, "thumb.jpg", info.ModTime(), f)
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

func (h *FileHandler) DeleteFileHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	fileIDstr := chi.URLParam(r, "id")
	fileID, err := uuid.Parse(fileIDstr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	if err := h.db.SoftDelete(userID, []database.ItemRef{{Kind: "file", ID: fileID}}); err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "File not found")
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "trashed"})
}

type batchRequest struct {
	Items []database.ItemRef `json:"items"`
}

func requestUser(r *http.Request) (uuid.UUID, bool) {
	id, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	return id, ok
}

func decodeBatch(w http.ResponseWriter, r *http.Request) (uuid.UUID, []database.ItemRef, bool) {
	userID, ok := requestUser(r)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return uuid.Nil, nil, false
	}
	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Items) == 0 {
		utils.RespondWithError(w, http.StatusBadRequest, "Select at least one item")
		return uuid.Nil, nil, false
	}
	return userID, req.Items, true
}

func (h *FileHandler) BatchDeleteHandler(w http.ResponseWriter, r *http.Request) {
	userID, items, ok := decodeBatch(w, r)
	if !ok {
		return
	}
	if err := h.db.SoftDelete(userID, items); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "trashed"})
}

func (h *FileHandler) BatchMoveHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUser(r)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	var req struct {
		Items  []database.ItemRef `json:"items"`
		Folder string             `json:"folder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Items) == 0 {
		utils.RespondWithError(w, http.StatusBadRequest, "Select at least one item")
		return
	}
	if err := h.db.MoveItems(userID, req.Items, req.Folder); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"folder": normalizeFolder(req.Folder)})
}

func (h *FileHandler) BatchDownloadHandler(w http.ResponseWriter, r *http.Request) {
	userID, items, ok := decodeBatch(w, r)
	if !ok {
		return
	}
	files, err := h.filesForDownload(userID, items)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="media-download.zip"`)
	zw := zip.NewWriter(w)
	for _, file := range files {
		in, err := os.Open(file.StoragePath)
		if err != nil {
			zw.Close()
			return
		}
		name := strings.TrimPrefix(filepath.ToSlash(filepath.Join(file.Folder, file.Filename)), "/")
		out, err := zw.Create(name)
		if err == nil {
			_, err = io.Copy(out, in)
		}
		in.Close()
		if err != nil {
			zw.Close()
			return
		}
	}
	_ = zw.Close()
}

func (h *FileHandler) filesForDownload(userID uuid.UUID, items []database.ItemRef) ([]models.File, error) {
	files, err := h.db.ListFilesByUser(userID)
	if err != nil {
		return nil, err
	}
	folders, err := h.db.ListFoldersByUser(userID)
	if err != nil {
		return nil, err
	}
	wantedFiles := map[uuid.UUID]bool{}
	roots := []string{}
	for _, item := range items {
		if item.Kind == "file" {
			wantedFiles[item.ID] = true
			continue
		}
		if item.Kind != "folder" {
			return nil, fmt.Errorf("invalid item kind")
		}
		found := false
		for _, folder := range folders {
			if folder.ID == item.ID {
				roots = append(roots, folder.Path)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("folder not found")
		}
	}
	var out []models.File
	for _, file := range files {
		if wantedFiles[file.ID] {
			out = append(out, file)
			continue
		}
		for _, root := range roots {
			if file.Folder == root || strings.HasPrefix(file.Folder, root+"/") {
				out = append(out, file)
				break
			}
		}
	}
	return out, nil
}

func (h *FileHandler) TrashHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUser(r)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	items, err := h.db.ListTrash(userID)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to list recycle bin")
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *FileHandler) RestoreTrashHandler(w http.ResponseWriter, r *http.Request) {
	userID, items, ok := decodeBatch(w, r)
	if !ok {
		return
	}
	if err := h.db.RestoreTrash(userID, items); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}

func (h *FileHandler) removeFiles(files []models.File) error {
	for _, file := range files {
		if err := os.Remove(file.StoragePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Remove(file.StoragePath + ".thumb.jpg"); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (h *FileHandler) PurgeTrashHandler(w http.ResponseWriter, r *http.Request) {
	userID, items, ok := decodeBatch(w, r)
	if !ok {
		return
	}
	files, err := h.db.FilesForPurge(userID, items, false)
	if err == nil {
		err = h.removeFiles(files)
	}
	if err == nil {
		err = h.db.PurgeTrash(userID, items, false)
	}
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to permanently delete items")
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "purged"})
}

func (h *FileHandler) EmptyTrashHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUser(r)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	trash, err := h.db.ListTrash(userID)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to empty recycle bin")
		return
	}
	items := make([]database.ItemRef, 0, len(trash))
	for _, x := range trash {
		items = append(items, database.ItemRef{Kind: x.Kind, ID: x.ID})
	}
	files, err := h.db.FilesForPurge(userID, items, false)
	if err == nil {
		err = h.removeFiles(files)
	}
	if err == nil {
		err = h.db.PurgeTrash(userID, items, false)
	}
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to empty recycle bin")
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "emptied"})
}

// PurgeExpired removes files whose deleted root is outside the retention window.
// It is intentionally best-effort: a failed filesystem removal leaves database
// records in place for the next scheduled run.
func PurgeExpired(db database.Service, storagePath string) {
	files, err := db.ExpiredFilesForPurge(time.Now().Add(-30 * 24 * time.Hour))
	if err != nil {
		return
	}
	for _, file := range files {
		if err := os.Remove(file.StoragePath); err != nil && !os.IsNotExist(err) {
			return
		}
		if err := os.Remove(file.StoragePath + ".thumb.jpg"); err != nil && !os.IsNotExist(err) {
			return
		}
	}
	_ = storagePath // storage is intentionally read from each persisted file record.
	_ = db.PurgeExpired(time.Now().Add(-30 * 24 * time.Hour))
}

func (h *FileHandler) RenameFileHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	fileIDstr := chi.URLParam(r, "id")
	fileID, err := uuid.Parse(fileIDstr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	var req struct {
		Filename string `json:"filename"`
	}
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil || req.Filename == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing filename")
		return
	}

	file, err := h.db.GetFileByID(fileID)
	if err != nil || file.UserID != userID {
		utils.RespondWithError(w, http.StatusNotFound, "File not found")
		return
	}

	if err := h.db.UpdateFilename(fileID, req.Filename); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update filename")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"filename": req.Filename})
}

func (h *FileHandler) MoveFileHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	fileIDstr := chi.URLParam(r, "id")
	fileID, err := uuid.Parse(fileIDstr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	var req struct {
		Folder string `json:"folder"`
	}
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	file, err := h.db.GetFileByID(fileID)
	if err != nil || file.UserID != userID {
		utils.RespondWithError(w, http.StatusNotFound, "File not found")
		return
	}

	if err := h.db.UpdateFileFolder(fileID, req.Folder); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to move file")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"folder": req.Folder})
}

func (h *FileHandler) UpdatePlaybackProgressHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	fileIDstr := chi.URLParam(r, "id")
	fileID, err := uuid.Parse(fileIDstr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	var req struct {
		Progress int `json:"progress"`
	}
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil || req.Progress < 0 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid progress value")
		return
	}

	file, err := h.db.GetFileByID(fileID)
	if err != nil || file.UserID != userID {
		utils.RespondWithError(w, http.StatusNotFound, "File not found")
		return
	}

	if err := h.db.UpdatePlaybackProgress(fileID, req.Progress); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update playback progress")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{"playback_progress": req.Progress})
}
