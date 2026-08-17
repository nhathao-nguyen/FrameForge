package primitives

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type fixture struct {
	SchemaVersion string            `json:"schema_version"`
	IDs           map[string]string `json:"ids"`
	Timestamp     string            `json:"timestamp"`
	Media         MediaRange        `json:"media"`
	Concurrency   Concurrency       `json:"concurrency"`
	Error         SafeError         `json:"error"`
}

func TestSharedFixture(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "fixtures", "shared-primitives.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var value fixture
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	if value.SchemaVersion != "shared-primitives/v1" {
		t.Fatalf("schema version: %s", value.SchemaVersion)
	}
	for _, id := range value.IDs {
		if err := ValidateOpaqueID(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateTimestamp(value.Timestamp); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMediaRange(value.Media); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConcurrency(value.Concurrency); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSafeError(value.Error); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidPrimitivesRejected(t *testing.T) {
	if ValidateOpaqueID("C:\\secret") == nil {
		t.Fatal("path-like ID accepted")
	}
	if ValidateTimestamp("2026-08-17T05:00:00+07:00") == nil {
		t.Fatal("non-UTC timestamp accepted")
	}
	if ValidateMediaRange(MediaRange{StartSec: 2, EndSec: 1, DurationSec: 1}) == nil {
		t.Fatal("reversed range accepted")
	}
	if ValidateSafeError(SafeError{Code: "bad", Category: "permanent", SafeMessage: "C:\\secret"}) != nil {
		t.Fatal("safe code fixture rejected unexpectedly")
	}
}
