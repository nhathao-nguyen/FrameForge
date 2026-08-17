package probe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Limits struct {
	MaxBytes       int64
	MaxDurationSec float64
	MaxWidth       int
	MaxHeight      int
	MaxStreams     int
}
type Result struct {
	SHA256      string  `json:"sha256"`
	SizeBytes   int64   `json:"size_bytes"`
	MIME        string  `json:"mime"`
	DurationSec float64 `json:"duration_sec"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	Streams     int     `json:"streams"`
	Quarantined bool    `json:"quarantined"`
	SafeReason  string  `json:"safe_reason,omitempty"`
}
type Validator struct {
	FFprobePath string
	Limits      Limits
}

func (v Validator) Validate(ctx context.Context, path string, declaredMIME string) (Result, error) {
	if strings.TrimSpace(path) == "" {
		return Result{}, errors.New("probe path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return Result{}, err
	}
	if info.IsDir() {
		return Result{}, errors.New("probe input must be a file")
	}
	result := Result{SizeBytes: info.Size()}
	if v.Limits.MaxBytes > 0 && result.SizeBytes > v.Limits.MaxBytes {
		result.Quarantined = true
		result.SafeReason = "size_limit"
		return result, nil
	}
	hash, magicErr := hashAndCheckMagic(path, declaredMIME)
	if magicErr != nil {
		result.Quarantined = true
		result.SafeReason = "magic_mismatch"
		return result, nil
	}
	result.SHA256 = hash
	if v.FFprobePath == "" {
		return result, errors.New("ffprobe path is required")
	}
	command := exec.CommandContext(ctx, v.FFprobePath, "-v", "error", "-of", "json", "-show_format", "-show_streams", path)
	output, err := command.Output()
	if err != nil {
		result.Quarantined = true
		result.SafeReason = "ffprobe_failed"
		return result, nil
	}
	var probe struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &probe); err != nil {
		result.Quarantined = true
		result.SafeReason = "ffprobe_invalid"
		return result, nil
	}
	result.Streams = len(probe.Streams)
	if v.Limits.MaxStreams > 0 && result.Streams > v.Limits.MaxStreams {
		result.Quarantined = true
		result.SafeReason = "stream_limit"
		return result, nil
	}
	for _, stream := range probe.Streams {
		if stream.Width > result.Width {
			result.Width = stream.Width
		}
		if stream.Height > result.Height {
			result.Height = stream.Height
		}
	}
	if v.Limits.MaxWidth > 0 && result.Width > v.Limits.MaxWidth || v.Limits.MaxHeight > 0 && result.Height > v.Limits.MaxHeight {
		result.Quarantined = true
		result.SafeReason = "resolution_limit"
		return result, nil
	}
	if probe.Format.Duration != "" {
		_, _ = fmt.Sscanf(probe.Format.Duration, "%f", &result.DurationSec)
	}
	if v.Limits.MaxDurationSec > 0 && result.DurationSec > v.Limits.MaxDurationSec {
		result.Quarantined = true
		result.SafeReason = "duration_limit"
	}
	return result, nil
}

func hashAndCheckMagic(path, declaredMIME string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	header := make([]byte, 12)
	count, err := io.ReadFull(file, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", err
	}
	lower := strings.ToLower(declaredMIME)
	valid := count >= 8 && ((strings.Contains(lower, "video/mp4") || strings.Contains(lower, "audio/mp4")) && string(header[4:8]) == "ftyp" || strings.Contains(lower, "video/webm") && (string(header[:4]) == "\x1a\x45\xdf\xa3") || strings.Contains(lower, "audio/wav") && string(header[:4]) == "RIFF" && string(header[8:12]) == "WAVE" || lower == "" && count > 0)
	if !valid {
		return "", errors.New("declared MIME does not match media magic")
	}
	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
