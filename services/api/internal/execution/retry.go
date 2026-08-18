package execution

import (
	"encoding/json"
	"errors"
	"math"
	"time"
)

type FailureCategory string

const (
	FailureTransient        FailureCategory = "transient"
	FailurePermanent        FailureCategory = "permanent"
	FailureCancellation     FailureCategory = "cancellation"
	FailureTimeout          FailureCategory = "timeout"
	FailureProviderThrottle FailureCategory = "provider_throttling"
	FailureResource         FailureCategory = "resource_exhaustion"
	FailureInvalidInput     FailureCategory = "invalid_user_input"
)

func Retryable(category FailureCategory, policyAllowsTimeout, policyAllowsResource bool) bool {
	switch category {
	case FailureTransient, FailureProviderThrottle:
		return true
	case FailureTimeout:
		return policyAllowsTimeout
	case FailureResource:
		return policyAllowsResource
	default:
		return false
	}
}

func Backoff(attempt int, base, maximum time.Duration) (time.Duration, error) {
	if attempt < 1 || base <= 0 || maximum <= 0 {
		return 0, errors.New("retry backoff parameters are invalid")
	}
	if attempt > 31 {
		attempt = 31
	}
	value := time.Duration(float64(base) * math.Pow(2, float64(attempt-1)))
	if value > maximum || value < 0 {
		return maximum, nil
	}
	return value, nil
}

// RetryPolicy is the bounded, data-only retry policy snapshot read from a
// PipelineNode. Delays are evaluated by the durable scheduler, never by a
// worker holding a lease.
type RetryPolicy struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

func ParseRetryPolicy(raw []byte) RetryPolicy {
	policy := RetryPolicy{BaseDelay: 250 * time.Millisecond, MaxDelay: 30 * time.Second}
	var value map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return policy
	}
	if milliseconds, ok := value["base_delay_ms"].(float64); ok && milliseconds > 0 && milliseconds <= 30_000 {
		policy.BaseDelay = time.Duration(milliseconds) * time.Millisecond
	}
	if milliseconds, ok := value["max_delay_ms"].(float64); ok && milliseconds > 0 && milliseconds <= 300_000 {
		policy.MaxDelay = time.Duration(milliseconds) * time.Millisecond
	}
	if policy.MaxDelay < policy.BaseDelay {
		policy.MaxDelay = policy.BaseDelay
	}
	return policy
}
