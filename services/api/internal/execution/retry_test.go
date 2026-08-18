package execution

import (
	"testing"
	"time"
)

func TestRetryTaxonomyIsFailClosed(t *testing.T) {
	if !Retryable(FailureTransient, false, false) || !Retryable(FailureProviderThrottle, false, false) || !Retryable(FailureTimeout, true, false) || Retryable(FailureTimeout, false, false) || Retryable(FailureInvalidInput, true, true) || Retryable(FailureCancellation, true, true) {
		t.Fatal("retry taxonomy violated")
	}
	delay, err := Backoff(3, time.Second, 10*time.Second)
	if err != nil || delay != 4*time.Second {
		t.Fatalf("unexpected backoff: %v %v", delay, err)
	}
	delay, _ = Backoff(20, time.Second, 10*time.Second)
	if delay != 10*time.Second {
		t.Fatal("backoff was not capped")
	}
}
