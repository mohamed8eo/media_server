package files

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"mediaserver/internal/database"
	"mediaserver/internal/jobqueue"

	"github.com/google/uuid"
)

var GlobalAudioPool = jobqueue.NewPool(runtime.NumCPU(), 20)

func ResumePendingJobs(db database.Service, storagePath string) {
	jobs, err := db.GetPendingOrProcessingJobs()
	if err != nil {
		return
	}
	for _, job := range jobs {
		file, err := db.GetFileByID(job.FileID)
		if err != nil {
			_ = db.UpdateJobStatus(job.ID, "failed", "file not found on disk")
			continue
		}
		jobID := job.ID
		fileID := file.ID
		filePath := file.StoragePath
		mimeType := file.MimeType

		GlobalAudioPool.Submit(func() {
			_ = db.UpdateJobStatus(jobID, "processing", "")
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				_ = db.UpdateJobStatus(jobID, "failed", "file missing on disk")
				return
			}
			h := &FileHandler{db: db, storagePath: storagePath}
			if err := h.fixMediaIfNeeded(fileID, filePath, mimeType); err != nil {
				slog.Error("resumed media fix failed", "job_id", jobID, "error", err)
				_ = db.UpdateJobStatus(jobID, "failed", err.Error())
			} else {
				_ = db.UpdateJobStatus(jobID, "completed", "")
			}
		})
	}
}

var browserSafeVideoCodecs = map[string]bool{
	"h264": true,
	"avc1": true,
	"vp8":  true,
	"vp9":  true,
	"av1":  true,
}

var browserSafeAudioCodecs = map[string]bool{
	"aac":    true,
	"opus":   true,
	"vorbis": true,
	"mp3":    true,
}

func getVideoCodec(path string) (string, error) {
	cmd := exec.Command(
		"ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}

func getAudioCodec(path string) (string, error) {
	cmd := exec.Command(
		"ffprobe", "-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}

func remuxAudioToAAC(path string) error {
	tmpPath := path + ".remux.tmp"

	cmd := exec.Command(
		"ffmpeg", "-y",
		"-i", path,
		"-c:v", "copy",
		"-c:a", "aac", "-b:a", "192k",
		"-f", "matroska",
		tmpPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("ffmpeg remux failed: %w (%s)", err, stderr.String())
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to replace original file: %w", err)
	}

	return nil
}

func transcodeVideoToBrowserSafe(path string) error {
	tmpPath := path + ".transcode.tmp"

	cmd := exec.Command(
		"ffmpeg", "-y",
		"-i", path,
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "23",
		"-c:a", "aac", "-b:a", "192k",
		tmpPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("ffmpeg transcode failed: %w (%s)", err, stderr.String())
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to replace original file: %w", err)
	}

	return nil
}

func (h *FileHandler) fixMediaIfNeeded(fileID uuid.UUID, path string, mimeType string) error {
	vCodec, _ := getVideoCodec(path)
	aCodec, _ := getAudioCodec(path)

	vSupported := vCodec == "" || browserSafeVideoCodecs[vCodec]
	aSupported := aCodec == "" || browserSafeAudioCodecs[aCodec]

	thumbPath := getThumbPath(path)

	if !vSupported || !aSupported {
		// If video/audio codec is not browser supported, remove the cache until codec is solved
		_ = os.Remove(thumbPath)

		var err error
		if !vSupported {
			err = transcodeVideoToBrowserSafe(path)
		} else {
			err = remuxAudioToAAC(path)
		}

		if err != nil {
			return err
		}

		info, err := os.Stat(path)
		if err != nil {
			return err
		}

		if err := h.db.UpdateFileSize(fileID, info.Size()); err != nil {
			return err
		}

		_, _ = GenerateThumbnail(path, mimeType)
	} else {
		// If supported, leave the cache intact
	}

	return nil
}
