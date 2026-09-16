package files

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func getPeaksPath(storagePath string) string {
	return storagePath + ".peaks.json"
}

// PeaksData is the JSON shape served to the browser. It mirrors the
// wavesurfer.js `peaks` load() argument (one array per channel, values in
// roughly [-1, 1]) plus the media duration, so the client can render a
// waveform and know its length without downloading or decoding any audio.
type PeaksData struct {
	Duration float64     `json:"duration"`
	Peaks    [][]float32 `json:"peaks"`
}

const (
	peaksPerSecond = 100   // resolution of the generated waveform
	maxPeakPoints  = 20000 // cap total points so huge files stay small
	minPeakRate    = 100   // ffmpeg output sample rate floor (Hz)
)

func peaksSampleRate(duration float64) int {
	rate := peaksPerSecond
	if duration > 0 && float64(rate)*duration > maxPeakPoints {
		rate = int(maxPeakPoints / duration)
	}
	if rate < minPeakRate {
		rate = minPeakRate
	}
	return rate
}

// GeneratePeaks extracts a lightweight waveform-peaks file for audio/video
// media so the browser can render a waveform instantly instead of
// downloading and decoding the whole media file with WebAudio. Results are
// cached next to the source file (storagePath + ".peaks.json").
func GeneratePeaks(storagePath, mimeType string) (string, error) {
	peaksPath := getPeaksPath(storagePath)

	if info, err := os.Stat(peaksPath); err == nil && info.Size() > 0 {
		return peaksPath, nil
	}

	mime := strings.ToLower(mimeType)
	if !strings.HasPrefix(mime, "audio/") && !strings.HasPrefix(mime, "video/") {
		return "", ErrNoThumbnail
	}

	duration := getVideoDuration(storagePath)
	rate := peaksSampleRate(duration)

	cmd := exec.Command(
		"ffmpeg",
		"-i", storagePath,
		"-map", "0:a:0",
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", rate),
		"-f", "s16le",
		"-acodec", "pcm_s16le",
		"-vn",
		"pipe:1",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg peaks extraction failed: %w (%s)", err, stderr.String())
	}

	raw := stdout.Bytes()
	numSamples := len(raw) / 2
	if numSamples == 0 {
		return "", fmt.Errorf("no audio samples extracted (file may have no audio track)")
	}

	peaks := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		s := int16(binary.LittleEndian.Uint16(raw[i*2 : i*2+2]))
		peaks[i] = float32(s) / 32768.0
	}

	if duration <= 0 {
		duration = float64(numSamples) / float64(rate)
	}

	data := PeaksData{
		Duration: duration,
		Peaks:    [][]float32{peaks},
	}

	tmpPath := peaksPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	if err := json.NewEncoder(f).Encode(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", err
	}
	f.Close()

	if err := os.Rename(tmpPath, peaksPath); err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	return peaksPath, nil
}
