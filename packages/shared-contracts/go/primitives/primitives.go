package primitives

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var opaqueIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,127}$`)
var codePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)
var etagPattern = regexp.MustCompile(`^"[A-Za-z0-9._~-]+"$`)

type MediaRange struct {
	StartSec    float64 `json:"start_sec"`
	EndSec      float64 `json:"end_sec"`
	DurationSec float64 `json:"duration_sec"`
}

type Concurrency struct {
	Revision        int64  `json:"revision"`
	ETag            string `json:"etag"`
	ExpectedVersion int64  `json:"expected_version"`
}

type SafeError struct {
	Code        string `json:"code"`
	Category    string `json:"category"`
	Retryable   bool   `json:"retryable"`
	SafeMessage string `json:"safe_message"`
}

func ValidateOpaqueID(value string) error {
	if !opaqueIDPattern.MatchString(value) {
		return fmt.Errorf("invalid opaque ID")
	}
	return nil
}

func ValidateTimestamp(value string) error {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || !strings.HasSuffix(value, "Z") || t.Location() != time.UTC {
		return fmt.Errorf("timestamp must be UTC RFC3339")
	}
	return nil
}

func ValidateMediaRange(value MediaRange) error {
	if value.StartSec < 0 || value.EndSec < 0 || value.DurationSec < 0 ||
		value.EndSec < value.StartSec || value.DurationSec < value.EndSec-value.StartSec {
		return fmt.Errorf("invalid media range")
	}
	for _, part := range []float64{value.StartSec, value.EndSec, value.DurationSec} {
		if part*1000 != float64(int64(part*1000)) {
			return fmt.Errorf("media time precision must be at least one millisecond")
		}
	}
	return nil
}

func ValidateConcurrency(value Concurrency) error {
	if value.Revision < 0 || value.ExpectedVersion < 0 || !etagPattern.MatchString(value.ETag) {
		return fmt.Errorf("invalid concurrency primitive")
	}
	return nil
}

func ValidateSafeError(value SafeError) error {
	allowed := map[string]bool{"transient": true, "permanent": true, "policy": true, "cancelled": true, "internal": true}
	if !codePattern.MatchString(value.Code) || !allowed[value.Category] || strings.TrimSpace(value.SafeMessage) == "" || len(value.SafeMessage) > 512 {
		return fmt.Errorf("invalid safe error")
	}
	return nil
}
