package probe

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func smallMP4Fixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.mp4")
	// A tiny ISO-BMFF signature is enough to pass the magic gate. The
	// ffprobe stub decides whether the container/streams are actually valid.
	if err := os.WriteFile(path, []byte{0, 0, 0, 0, 'f', 't', 'y', 'p', 'i', 's', 'o', 0, 0}, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func validProbeOutput(t *testing.T) []byte {
	t.Helper()
	value := map[string]any{
		"format":  map[string]any{"duration": "4.25"},
		"streams": []any{map[string]any{"codec_type": "video", "width": 1920, "height": 1080}},
	}
	output, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return output
}

func TestValidatorQuarantinesOversizedAndSpoofedMedia(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.mp4")
	if err := os.WriteFile(path, []byte("not-a-media-file"), 0o600); err != nil {
		t.Fatal(err)
	}
	validator := Validator{Limits: Limits{MaxBytes: 4}}
	result, err := validator.Validate(context.Background(), path, "video/mp4")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Quarantined || result.SafeReason != "size_limit" {
		t.Fatalf("oversized file was not quarantined: %+v", result)
	}
	validator.Limits.MaxBytes = 1024
	result, err = validator.Validate(context.Background(), path, "video/mp4")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Quarantined || result.SafeReason != "magic_mismatch" {
		t.Fatalf("spoofed file was not quarantined: %+v", result)
	}
	if result.Validated {
		t.Fatal("spoofed media was marked validated")
	}
}

func TestValidatorRejectsMalformedContainerAndInvalidStreams(t *testing.T) {
	path := smallMP4Fixture(t)
	validator := Validator{Probe: func(context.Context, string) ([]byte, error) {
		return nil, errors.New("invalid atom layout")
	}}
	result, err := validator.Validate(context.Background(), path, "video/mp4")
	if err != nil || !result.Quarantined || result.SafeReason != "ffprobe_failed" || result.Validated {
		t.Fatalf("malformed container was not rejected safely: result=%+v err=%v", result, err)
	}

	validator.Probe = func(context.Context, string) ([]byte, error) {
		return []byte(`{"format":{"duration":"4"},"streams":[{"codec_type":"data"}]}`), nil
	}
	result, err = validator.Validate(context.Background(), path, "video/mp4")
	if err != nil || !result.Quarantined || result.SafeReason != "ffprobe_invalid_stream" || result.Validated {
		t.Fatalf("invalid stream was not rejected: result=%+v err=%v", result, err)
	}

	validator.Probe = func(context.Context, string) ([]byte, error) {
		return []byte(`{"format":{"duration":"4"},"streams":[]}`), nil
	}
	result, err = validator.Validate(context.Background(), path, "video/mp4")
	if err != nil || !result.Quarantined || result.SafeReason != "ffprobe_no_streams" || result.Validated {
		t.Fatalf("empty stream set was not rejected: result=%+v err=%v", result, err)
	}

	validator.Probe = func(context.Context, string) ([]byte, error) { return []byte(`{"format":`), nil }
	result, err = validator.Validate(context.Background(), path, "video/mp4")
	if err != nil || !result.Quarantined || result.SafeReason != "ffprobe_invalid" || result.Validated {
		t.Fatalf("malformed ffprobe output was not rejected: result=%+v err=%v", result, err)
	}
}

func TestValidatorRejectsPathologicalProbeOutputAndTimeout(t *testing.T) {
	path := smallMP4Fixture(t)
	validator := Validator{
		Limits: Limits{MaxStreams: 2, MaxProbeOutputBytes: 1024},
		Probe: func(context.Context, string) ([]byte, error) {
			return []byte(`{"format":{"duration":"4"},"streams":[{"codec_type":"video","width":1,"height":1},{"codec_type":"audio"},{"codec_type":"audio"}]}`), nil
		},
	}
	result, err := validator.Validate(context.Background(), path, "video/mp4")
	if err != nil || !result.Quarantined || result.SafeReason != "stream_limit" || result.Validated {
		t.Fatalf("pathological stream count was not bounded: result=%+v err=%v", result, err)
	}

	validator.Limits = Limits{MaxProbeOutputBytes: 32}
	validator.Probe = func(context.Context, string) ([]byte, error) {
		return []byte(strings.Repeat("x", 33)), nil
	}
	result, err = validator.Validate(context.Background(), path, "video/mp4")
	if err != nil || !result.Quarantined || result.SafeReason != "ffprobe_output_limit" || result.Validated {
		t.Fatalf("oversized probe output was not bounded: result=%+v err=%v", result, err)
	}

	validator.Limits = Limits{MaxProbeDuration: 5 * time.Millisecond}
	validator.Probe = func(ctx context.Context, _ string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	result, err = validator.Validate(context.Background(), path, "video/mp4")
	if err != nil || !result.Quarantined || result.SafeReason != "probe_timeout" || result.Validated {
		t.Fatalf("probe timeout was not classified safely: result=%+v err=%v", result, err)
	}
}

func TestValidatorMarksOnlyFullyCheckedMediaValidated(t *testing.T) {
	path := smallMP4Fixture(t)
	validator := Validator{Probe: func(context.Context, string) ([]byte, error) { return validProbeOutput(t), nil }}
	result, err := validator.Validate(context.Background(), path, "video/mp4")
	if err != nil || result.Quarantined || !result.Validated || result.SafeReason != "" {
		t.Fatalf("valid fixture was not accepted after complete checks: result=%+v err=%v", result, err)
	}
}
