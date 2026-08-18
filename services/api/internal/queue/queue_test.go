package queue

import (
	"testing"
	"time"
)

func TestNormalizedBlockMakesZeroDurationPollBounded(t *testing.T) {
	if got := normalizedBlock(0); got <= 0 || got > time.Millisecond {
		t.Fatalf("zero poll duration=%s", got)
	}
	if got := normalizedBlock(-time.Second); got != 0 {
		t.Fatalf("negative poll duration=%s", got)
	}
	if got := normalizedBlock(31 * time.Second); got != 30*time.Second {
		t.Fatalf("maximum poll duration=%s", got)
	}
}
