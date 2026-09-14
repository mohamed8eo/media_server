package files

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"mediaserver/internal/jobqueue"
	"mediaserver/internal/middleware"
	"mediaserver/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var GlobalDownloadPool = jobqueue.NewPool(2, 50)

type DownloadURLRequest struct {
	URL     string `json:"url"`
	Quality string `json:"quality"`
	Folder  string `json:"folder"`
}

var progressRegex = regexp.MustCompile(`\[download\]\s+(\d+(?:\.\d+)?)%`)

func (h *FileHandler) DownloadURLHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req DownloadURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.URL == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "URL is required")
		return
	}

	folder := normalizeFolder(req.Folder)
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

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to prepare storage")
		return
	}

	jobID := uuid.New()
	if err := h.db.CreateJob(jobID, uuid.Nil, userID, "url_download"); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create job")
		return
	}

	fileID := uuid.New()
	outputTemplate := filepath.Join(targetDir, "%(title)s.%(ext)s")

	args := []string{
		"--no-playlist",
		"--newline",
		"--no-color",
		"--progress",
		"-N", "8", // download up to 8 fragments of the video concurrently
		"-o", outputTemplate,
	}

	switch req.Quality {
	case "1080p":
		args = append(args, "-f", "bestvideo[height<=1080]+bestaudio/best[height<=1080]/best")
	case "720p":
		args = append(args, "-f", "bestvideo[height<=720]+bestaudio/best[height<=720]/best")
	case "480p":
		args = append(args, "-f", "bestvideo[height<=480]+bestaudio/best[height<=480]/best")
	case "audio":
		args = append(args, "-x", "--audio-format", "mp3")
	default: // "best"
		args = append(args, "-f", "bestvideo+bestaudio/best")
	}

	args = append(args, req.URL)

	// Submit download task to background worker pool
	GlobalDownloadPool.Submit(func() {
		_ = h.db.UpdateJobStatus(jobID, "processing", "")

		existingFiles := make(map[string]bool)
		if entries, err := os.ReadDir(targetDir); err == nil {
			for _, entry := range entries {
				existingFiles[entry.Name()] = true
			}
		}
		startTime := time.Now()

		cmd := exec.Command("yt-dlp", args...)
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			_ = h.db.UpdateJobStatus(jobID, "failed", "failed to create stdout pipe: "+err.Error())
			return
		}
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			_ = h.db.UpdateJobStatus(jobID, "failed", "failed to create stderr pipe: "+err.Error())
			return
		}

		if err := cmd.Start(); err != nil {
			_ = h.db.UpdateJobStatus(jobID, "failed", "failed to start yt-dlp: "+err.Error())
			return
		}

		// Both pipes must be drained concurrently, or once either OS pipe buffer
		// fills up, yt-dlp blocks on write() and the whole download stalls forever
		// (this is what previously caused jobs to sit at 0% indefinitely).
		var stderrBuf bytes.Buffer
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = io.Copy(&stderrBuf, stderrPipe)
		}()

		lastUpdate := time.Now()
		// yt-dlp (with --newline) prints progress lines like:
		//   [download]  45.2% of 128.34MiB at 2.34MiB/s ETA 00:32
		// to stdout, so we scan stdout for progress updates.
		scanner := bufio.NewScanner(stdoutPipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			matches := progressRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				var pct float64
				_, _ = fmt.Sscanf(matches[1], "%f", &pct)
				intPct := int(pct)
				if intPct > 100 {
					intPct = 100
				}
				if time.Since(lastUpdate) >= time.Second || intPct == 100 {
					_ = h.db.UpdateJobProgress(jobID, intPct)
					lastUpdate = time.Now()
				}
			}
		}

		wg.Wait()
		err = cmd.Wait()
		if err != nil {
			errOutput := strings.TrimSpace(stderrBuf.String())
			slog.Error("yt-dlp download failed", "error", err, "output", errOutput)
			if errOutput == "" {
				errOutput = err.Error()
			}
			_ = h.db.UpdateJobStatus(jobID, "failed", errOutput)
			return
		}

		var finalFilePath string
		if entries, err := os.ReadDir(targetDir); err == nil {
			var newestTime time.Time
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				fullPath := filepath.Join(targetDir, name)
				info, err := entry.Info()
				if err != nil {
					continue
				}
				if !existingFiles[name] || info.ModTime().After(startTime.Add(-2*time.Second)) {
					if info.ModTime().After(newestTime) {
						newestTime = info.ModTime()
						finalFilePath = fullPath
					}
				}
			}
		}

		if finalFilePath == "" {
			matches, err := filepath.Glob(filepath.Join(targetDir, "*"))
			if err == nil && len(matches) > 0 {
				finalFilePath = matches[len(matches)-1]
			}
		}

		if finalFilePath == "" {
			slog.Error("downloaded file not found", "job_id", jobID)
			_ = h.db.UpdateJobStatus(jobID, "failed", "downloaded file not found")
			return
		}
		filename := filepath.Base(finalFilePath)

		info, err := os.Stat(finalFilePath)
		if err != nil {
			slog.Error("failed to stat downloaded file", "error", err)
			_ = h.db.UpdateJobStatus(jobID, "failed", "failed to read downloaded file info")
			return
		}

		ext := strings.TrimPrefix(filepath.Ext(finalFilePath), ".")
		mimeType := "video/mp4"
		switch ext {
		case "mp3":
			mimeType = "audio/mpeg"
		case "webm":
			mimeType = "video/webm"
		case "mkv":
			mimeType = "video/x-matroska"
		}

		dbFolder := "/"
		if folder != "" {
			dbFolder = "/" + folder
		}

		_ = h.ensureFolderHierarchy(userID, dbFolder)
		var fileHash string
		if f, err := os.Open(finalFilePath); err == nil {
			hsh := sha256.New()
			_, _ = io.Copy(hsh, f)
			f.Close()
			fileHash = hex.EncodeToString(hsh.Sum(nil))
		}

		if err := h.db.CreateFile(
			fileID,
			userID,
			filename,
			mimeType,
			info.Size(),
			dbFolder,
			finalFilePath,
			fileHash,
		); err != nil {
			slog.Error("failed to create file record", "error", err)
			_ = h.db.UpdateJobStatus(jobID, "failed", "failed to save file record")
			return
		}

		_ = h.db.UpdateJobProgress(jobID, 100)
		_ = h.db.UpdateJobStatus(jobID, "completed", "")
	})

	utils.RespondWithJSON(w, http.StatusAccepted, map[string]string{
		"message": "Download started successfully",
		"job_id":  jobID.String(),
	})
}

func (h *FileHandler) GetJobHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	_ = userID

	jobIDstr := chi.URLParam(r, "id")
	jobID, err := uuid.Parse(jobIDstr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	job, err := h.db.GetJobByID(jobID)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Job not found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"id":       job.ID.String(),
		"status":   job.Status,
		"progress": job.Progress,
		"error":    job.Error,
	})
}
