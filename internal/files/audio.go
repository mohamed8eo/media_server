package files

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

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
 			if err := h.fixMediaIfNeeded(fileID, jobID, filePath, mimeType); err != nil {
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

var ffmpegTimeRegex = regexp.MustCompile(`time=([0-9:.]+)`)

func parseFFmpegTime(timeStr string) float64 {
	timeStr = strings.TrimSpace(timeStr)
	if strings.Contains(timeStr, ":") {
		parts := strings.Split(timeStr, ":")
		var h, m, s float64
		if len(parts) == 3 {
			_, _ = fmt.Sscanf(parts[0], "%f", &h)
			_, _ = fmt.Sscanf(parts[1], "%f", &m)
			_, _ = fmt.Sscanf(parts[2], "%f", &s)
			return h*3600 + m*60 + s
		} else if len(parts) == 2 {
			_, _ = fmt.Sscanf(parts[0], "%f", &m)
			_, _ = fmt.Sscanf(parts[1], "%f", &s)
			return m*60 + s
		}
	} else {
		var s float64
		_, _ = fmt.Sscanf(timeStr, "%f", &s)
		return s
	}
	return 0
}

func scanLinesOrCarriageReturns(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' || data[i] == '\r' {
			advance = i + 1
			if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
				advance++
			}
			return advance, data[0:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func getVideoDuration(path string) float64 {
	cmd := exec.Command(
		"ffprobe", "-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0
	}
	var dur float64
	_, _ = fmt.Sscanf(strings.TrimSpace(out.String()), "%f", &dur)
	return dur
}

func remuxAudioToAAC(db database.Service, jobID uuid.UUID, path string) error {
	tmpPath := path + ".remux.tmp"
	duration := getVideoDuration(path)

	cmd := exec.Command(
		"ffmpeg", "-y",
		"-i", path,
		"-c:v", "copy",
		"-c:a", "aac", "-b:a", "192k",
		"-f", "matroska",
		tmpPath,
	)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	var stderrBuf bytes.Buffer
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start failed: %w", err)
	}

	lastUpdate := time.Now()
	scanner := bufio.NewScanner(stderrPipe)
	scanner.Split(scanLinesOrCarriageReturns)
	for scanner.Scan() {
		line := scanner.Text()
		stderrBuf.WriteString(line + "\n")
		if duration > 0 {
			matches := ffmpegTimeRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				totalSecs := parseFFmpegTime(matches[1])
				pct := int((totalSecs / duration) * 100)
				if pct > 99 {
					pct = 99
				}
				if pct < 0 {
					pct = 0
				}
				if time.Since(lastUpdate) >= time.Second || pct == 100 {
					_ = db.UpdateJobProgress(jobID, pct)
					lastUpdate = time.Now()
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("ffmpeg remux failed: %w (%s)", err, stderrBuf.String())
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to replace original file: %w", err)
	}

	return nil
}

func transcodeVideoToBrowserSafe(db database.Service, jobID uuid.UUID, path string) error {
	tmpPath := path + ".transcode.tmp"
	duration := getVideoDuration(path)

	cmd := exec.Command(
		"ffmpeg", "-y",
		"-i", path,
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "23",
		"-c:a", "aac", "-b:a", "192k",
		tmpPath,
	)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	var stderrBuf bytes.Buffer
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start failed: %w", err)
	}

	lastUpdate := time.Now()
	scanner := bufio.NewScanner(stderrPipe)
	scanner.Split(scanLinesOrCarriageReturns)
	for scanner.Scan() {
		line := scanner.Text()
		stderrBuf.WriteString(line + "\n")
		if duration > 0 {
			matches := ffmpegTimeRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				totalSecs := parseFFmpegTime(matches[1])
				pct := int((totalSecs / duration) * 100)
				if pct > 99 {
					pct = 99
				}
				if pct < 0 {
					pct = 0
				}
				if time.Since(lastUpdate) >= time.Second || pct == 100 {
					_ = db.UpdateJobProgress(jobID, pct)
					lastUpdate = time.Now()
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("ffmpeg transcode failed: %w (%s)", err, stderrBuf.String())
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to replace original file: %w", err)
	}

	return nil
}

func (h *FileHandler) fixMediaIfNeeded(fileID uuid.UUID, jobID uuid.UUID, path string, mimeType string) error {
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
			err = transcodeVideoToBrowserSafe(h.db, jobID, path)
		} else {
			err = remuxAudioToAAC(h.db, jobID, path)
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
