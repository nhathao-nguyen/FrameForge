package probe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Limits struct {
	MaxBytes            int64
	MaxDurationSec      float64
	MaxWidth            int
	MaxHeight           int
	MaxStreams          int
	MaxProbeOutputBytes int64
	MaxProbeDuration    time.Duration
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
	Validated   bool    `json:"validated"`
	SafeReason  string  `json:"safe_reason,omitempty"`
}

// ProbeFunc is injectable only at this boundary so deterministic adversarial
// fixtures can exercise the same validation decisions without depending on a
// host-specific ffprobe binary or large media files.
type ProbeFunc func(context.Context, string) ([]byte, error)

type Validator struct {
	FFprobePath string
	Limits      Limits
	Probe       ProbeFunc
}

const defaultMaxProbeOutputBytes int64 = 1 << 20

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
		if v.Probe == nil {
			return result, errors.New("ffprobe path is required")
		}
	}
	probeCtx := ctx
	cancel := func() {}
	if v.Limits.MaxProbeDuration > 0 {
		probeCtx, cancel = context.WithTimeout(ctx, v.Limits.MaxProbeDuration)
	}
	defer cancel()
	output, err, outputLimited := v.runProbe(probeCtx, path)
	if outputLimited {
		result.Quarantined = true
		result.SafeReason = "ffprobe_output_limit"
		return result, nil
	}
	if probeCtx.Err() != nil {
		result.Quarantined = true
		if errors.Is(probeCtx.Err(), context.DeadlineExceeded) {
			result.SafeReason = "probe_timeout"
		} else {
			result.SafeReason = "probe_cancelled"
		}
		return result, nil
	}
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
	if result.Streams == 0 {
		result.Quarantined = true
		result.SafeReason = "ffprobe_no_streams"
		return result, nil
	}
	if v.Limits.MaxStreams > 0 && result.Streams > v.Limits.MaxStreams {
		result.Quarantined = true
		result.SafeReason = "stream_limit"
		return result, nil
	}
	for _, stream := range probe.Streams {
		if stream.CodecType != "audio" && stream.CodecType != "video" {
			result.Quarantined = true
			result.SafeReason = "ffprobe_invalid_stream"
			return result, nil
		}
		if stream.CodecType == "video" && (stream.Width <= 0 || stream.Height <= 0) {
			result.Quarantined = true
			result.SafeReason = "ffprobe_invalid_stream"
			return result, nil
		}
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
		parsed, err := strconv.ParseFloat(probe.Format.Duration, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
			result.Quarantined = true
			result.SafeReason = "ffprobe_invalid_duration"
			return result, nil
		}
		result.DurationSec = parsed
	}
	if v.Limits.MaxDurationSec > 0 && result.DurationSec > v.Limits.MaxDurationSec {
		result.Quarantined = true
		result.SafeReason = "duration_limit"
		return result, nil
	}
	result.Validated = true
	return result, nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit   int64
	limited bool
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	if b.limit <= 0 {
		return len(value), nil
	}
	remaining := b.limit - int64(b.Len())
	if remaining <= 0 {
		b.limited = true
		return len(value), nil
	}
	if int64(len(value)) > remaining {
		_, _ = b.Buffer.Write(value[:remaining])
		b.limited = true
		return len(value), nil
	}
	_, err := b.Buffer.Write(value)
	return len(value), err
}

func (v Validator) runProbe(ctx context.Context, path string) ([]byte, error, bool) {
	limit := v.Limits.MaxProbeOutputBytes
	if limit == 0 {
		limit = defaultMaxProbeOutputBytes
	}
	if v.Probe != nil {
		output, err := v.Probe(ctx, path)
		return output, err, int64(len(output)) > limit
	}
	command := exec.CommandContext(ctx, v.FFprobePath, "-v", "error", "-of", "json", "-show_format", "-show_streams", path)
	buffer := &limitedBuffer{limit: limit}
	command.Stdout = buffer
	command.Stderr = io.Discard
	err := command.Run()
	return buffer.Bytes(), err, buffer.limited
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
