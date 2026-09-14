package files

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mediaserver/internal/middleware"
	"mediaserver/internal/utils"

	"github.com/google/uuid"
)

type UploadSessionMeta struct {
	Filename  string `json:"filename"`
	TotalSize int64  `json:"total_size"`
	ChunkSize int64  `json:"chunk_size"`
	Folder    string `json:"folder"`
	UserID    string `json:"user_id"`
}

func (h *FileHandler) InitUploadHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req struct {
		Filename  string `json:"filename"`
		TotalSize int64  `json:"total_size"`
		ChunkSize int64  `json:"chunk_size"`
		Folder    string `json:"folder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Filename == "" || req.TotalSize <= 0 {
		utils.RespondWithError(w, http.StatusBadRequest, "Filename and total size are required")
		return
	}

	if req.ChunkSize <= 0 {
		req.ChunkSize = 5 * 1024 * 1024 // default 5MB
	}

	uploadID := uuid.New()
	tempDir := filepath.Join(h.storagePath, "temp", uploadID.String())
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to initialize upload session")
		return
	}

	meta := UploadSessionMeta{
		Filename:  req.Filename,
		TotalSize: req.TotalSize,
		ChunkSize: req.ChunkSize,
		Folder:    req.Folder,
		UserID:    userID.String(),
	}

	metaBytes, _ := json.Marshal(meta)
	_ = os.WriteFile(filepath.Join(tempDir, "meta.json"), metaBytes, 0644)

	totalChunks := int((req.TotalSize + req.ChunkSize - 1) / req.ChunkSize)

	// Check for already uploaded chunks (resumable)
	var uploaded []int
	if entries, err := os.ReadDir(tempDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, "chunk-") {
				idxStr := strings.TrimPrefix(name, "chunk-")
				if idx, err := strconv.Atoi(idxStr); err == nil {
					uploaded = append(uploaded, idx)
				}
			}
		}
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"upload_id":    uploadID.String(),
		"total_chunks": totalChunks,
		"uploaded":     uploaded,
	})
}

func (h *FileHandler) UploadChunkHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	uploadID := r.FormValue("upload_id")
	chunkIndex := r.FormValue("chunk_index")
	if uploadID == "" || chunkIndex == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing upload_id or chunk_index")
		return
	}

	filePart, _, err := r.FormFile("chunk")
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing chunk file")
		return
	}
	defer filePart.Close()

	tempDir := filepath.Join(h.storagePath, "temp", uploadID)
	metaPath := filepath.Join(tempDir, "meta.json")
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Upload session not found")
		return
	}

	var meta UploadSessionMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil || meta.UserID != userID.String() {
		utils.RespondWithError(w, http.StatusForbidden, "Unauthorized upload session")
		return
	}

	chunkPath := filepath.Join(tempDir, fmt.Sprintf("chunk-%s", chunkIndex))
	dest, err := os.Create(chunkPath)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save chunk")
		return
	}
	defer dest.Close()

	_, err = io.Copy(dest, filePart)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to write chunk data")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *FileHandler) CompleteUploadHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req struct {
		UploadID string `json:"upload_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UploadID == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	tempDir := filepath.Join(h.storagePath, "temp", req.UploadID)
	metaPath := filepath.Join(tempDir, "meta.json")
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Upload session not found")
		return
	}

	var meta UploadSessionMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil || meta.UserID != userID.String() {
		utils.RespondWithError(w, http.StatusForbidden, "Unauthorized upload session")
		return
	}

	totalChunks := int((meta.TotalSize + meta.ChunkSize - 1) / meta.ChunkSize)

	folder := normalizeFolder(meta.Folder)
	if folder == "/" || folder == "." {
		folder = ""
	} else {
		folder = strings.TrimPrefix(folder, "/")
	}

	userDir := filepath.Join(h.storagePath, userID.String())
	targetDir := userDir
	if folder != "" {
		targetDir = filepath.Join(userDir, folder)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to prepare storage")
		return
	}

	fileID := uuid.New()
	destPath := filepath.Join(targetDir, fileID.String())

	dest, err := os.Create(destPath)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create destination file")
		return
	}

	var totalWritten int64
	for i := 0; i < totalChunks; i++ {
		chunkPath := filepath.Join(tempDir, fmt.Sprintf("chunk-%d", i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			dest.Close()
			os.Remove(destPath)
			utils.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("Missing chunk %d", i))
			return
		}
		n, err := io.Copy(dest, chunkFile)
		chunkFile.Close()
		if err != nil {
			dest.Close()
			os.Remove(destPath)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to assemble file chunks")
			return
		}
		totalWritten += n
	}
	dest.Close()

	mimeType := "application/octet-stream"
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(meta.Filename), "."))
	switch ext {
	case "mp4":
		mimeType = "video/mp4"
	case "webm":
		mimeType = "video/webm"
	case "mkv":
		mimeType = "video/x-matroska"
	case "mov":
		mimeType = "video/quicktime"
	case "mp3":
		mimeType = "audio/mpeg"
	case "wav":
		mimeType = "audio/wav"
	case "flac":
		mimeType = "audio/flac"
	case "jpg", "jpeg":
		mimeType = "image/jpeg"
	case "png":
		mimeType = "image/png"
	case "webp":
		mimeType = "image/webp"
	case "pdf":
		mimeType = "application/pdf"
	}

	dbFolder := "/"
	if folder != "" {
		dbFolder = "/" + folder
	}
	_ = h.ensureFolderHierarchy(userID, dbFolder)

	if err := h.db.CreateFile(
		fileID,
		userID,
		meta.Filename,
		mimeType,
		totalWritten,
		dbFolder,
		destPath,
		"",
	); err != nil {
		os.Remove(destPath)
		slog.Error("failed to create file record for chunked upload", "error", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save file record")
		return
	}

	// Clean up temp session
	_ = os.RemoveAll(tempDir)

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"message": "Upload completed successfully",
		"file_id": fileID.String(),
	})
}
