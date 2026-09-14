package files

import (
	"bytes"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getThumbPath(storagePath string) string {
	return storagePath + ".thumb.jpg"
}

func GenerateThumbnail(storagePath, mimeType string) (string, error) {
	thumbPath := getThumbPath(storagePath)

	if _, err := os.Stat(thumbPath); err == nil {
		return thumbPath, nil
	}

	mime := strings.ToLower(mimeType)
	switch {
	case strings.HasPrefix(mime, "video/"):
		if err := generateVideoThumb(storagePath, thumbPath); err != nil {
			return "", err
		}
		return thumbPath, nil
	case strings.HasPrefix(mime, "image/"):
		if err := generateImageThumb(storagePath, thumbPath); err != nil {
			return "", err
		}
		return thumbPath, nil
	case strings.HasPrefix(mime, "application/pdf") || mime == "application/x-pdf":
		if err := generatePDFThumb(storagePath, thumbPath); err != nil {
			return "", err
		}
		return thumbPath, nil
	case strings.HasPrefix(mime, "audio/"):
		if err := generateAudioThumb(storagePath, thumbPath); err != nil {
			return "", err
		}
		return thumbPath, nil
	default:
		return "", ErrNoThumbnail
	}
}

var ErrNoThumbnail = &ThumbError{"no thumbnail available"}

type ThumbError struct {
	msg string
}

func (e *ThumbError) Error() string {
	return e.msg
}

func generateVideoThumb(inputPath, outputPath string) error {
	args := []string{
		"-i", inputPath,
		"-vf", "select='eq(pict_type,I)',scale=320:-1",
		"-frames:v", "1",
		"-q:v", "3",
		"-y",
		outputPath,
	}
	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		args2 := []string{
			"-i", inputPath,
			"-ss", "00:00:01",
			"-vf", "scale=320:-1",
			"-frames:v", "1",
			"-q:v", "3",
			"-y",
			outputPath,
		}
		cmd2 := exec.Command("ffmpeg", args2...)
		cmd2.Stderr = &stderr
		if err2 := cmd2.Run(); err2 != nil {
			return err2
		}
	}
	return nil
}

func generateImageThumb(inputPath, outputPath string) error {
	f, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return generateImageThumbFallback(inputPath, outputPath)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	maxW := 320
	if w > maxW {
		ratio := float64(maxW) / float64(w)
		h = int(float64(h) * ratio)
		w = maxW
	}

	resized := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcX := bounds.Min.X + int(float64(x)*float64(bounds.Dx())/float64(w))
			srcY := bounds.Min.Y + int(float64(y)*float64(bounds.Dy())/float64(h))
			resized.Set(x, y, img.At(srcX, srcY))
		}
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	return jpeg.Encode(out, resized, &jpeg.Options{Quality: 80})
}

func generateImageThumbFallback(inputPath, outputPath string) error {
	args := []string{
		"-i", inputPath,
		"-vf", "scale=320:-1",
		"-frames:v", "1",
		"-q:v", "3",
		"-y",
		outputPath,
	}
	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	return cmd.Run()
}

// generateAudioFallbackThumb generates a simple audio icon fallback.
func generateAudioFallbackThumb(outputPath string) error {
	args := []string{
		"-f", "lavfi",
		"-i", "color=#090d16:d=2",
		"-vf", "drawtext=text='🎵':fontsize=32:x=(w-textw)/2:y=(h-th)/2:fontcolor=#ffffff",
		"-frames:v", "1",
		"-q:v", "3",
		"-y",
		outputPath,
	}
	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	return cmd.Run()
}

// generateAudioThumb creates a waveform thumbnail for audio files using ffmpeg.
func generateAudioThumb(inputPath, outputPath string) error {
	args := []string{
		"-i", inputPath,
	 "-filter_complex",
		"[0:a]aformat=channel_layout=mono,showfps=metadata=1:fps=1/8,scale=320:-1[vid]",
		"-map", "[vid]",
		"-frames:v", "1",
		"-q:v", "3",
		"-y",
		outputPath,
	}
	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Fallback: create a simple static audio icon instead
		return generateAudioIconThumb(inputPath, outputPath)
	}
	return nil
}

// generateAudioIconThumb creates a static audio file icon as fallback thumbnail.
func generateAudioIconThumb(inputPath, outputPath string) error {
	// Use ffmpeg to generate a simple colored rectangle with "AUDIO" text
	// Or we can just copy a static icon
	// For now, create a simple ffmpeg-generated thumbnail
	args := []string{
		"-f", "lavdeter",
		"-i", "color=#090d16:d=2",
		"-vf", "drawtext=text='AUDIO':fontsize=24:x=(w-textw)/2:y=(h-text_h)/2:fontcolor=#ffffff",
		"-frames:v", "1",
		"-q:v", "3",
		"-y",
		outputPath,
	}
	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	return cmd.Run()
}

// generatePDFThumb rasterizes page 1 of a PDF to a JPEG cover image using
// poppler's pdftoppm. With -singlefile, pdftoppm writes exactly "<prefix>.jpg"
// (no page-number suffix), so the prefix is outputPath with its extension
// stripped, making the result land precisely at outputPath.
func generatePDFThumb(inputPath, outputPath string) error {
	prefix := strings.TrimSuffix(outputPath, filepath.Ext(outputPath))
	args := []string{
		"-jpeg",
		"-f", "1",
		"-l", "1",
		"-singlefile",
		"-scale-to", "320",
		inputPath,
		prefix,
	}
	cmd := exec.Command("pdftoppm", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	if _, err := os.Stat(outputPath); err != nil {
		return err
	}
	return nil
}

func getExtensionFromPath(path string) string {
	ext := filepath.Ext(path)
	if ext != "" {
		return strings.ToLower(ext[1:])
	}
	return ""
}
