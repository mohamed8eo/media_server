package files

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/uuid"
)

var browserSafeAudioCodecs = map[string]bool{
	"aac":    true,
	"opus":   true,
	"vorbis": true,
	"mp3":    true,
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

func (h *FileHandler) fixAudioIfNeeded(fileID uuid.UUID, path string) error {
	codec, err := getAudioCodec(path)
	if err != nil || codec == "" {
		return err
	}

	if browserSafeAudioCodecs[codec] {
		return err
	}

	if err = remuxAudioToAAC(path); err != nil {
		return err
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if err := h.db.UpdateFileSize(fileID, info.Size()); err != nil {
		return err
	}
	return nil
}
