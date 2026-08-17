package probe

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

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
}
