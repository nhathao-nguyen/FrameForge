package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
)

func TestFailureResultClassifiesRetryBoundary(t *testing.T) {
	t.Parallel()
	command := worker.Command{MessageID: "message_failure_001", JobID: "job_failure_001", JobStepID: "step_failure_001", NodeKey: "render_timeline"}
	tests := []struct {
		name      string
		err       error
		code      string
		category  string
		retryable bool
	}{
		{name: "artifact connection", err: errors.New("resolve Artifact transfer: worker Artifact transfer request failed"), code: "artifact_resolve_connection_failed", category: "transient", retryable: true},
		{name: "artifact unauthorized", err: errors.New("resolve Artifact transfer: worker Artifact is unavailable (HTTP 401)"), code: "artifact_resolve_unauthorized", category: "policy", retryable: false},
		{name: "artifact missing", err: errors.New("resolve Artifact transfer: worker Artifact is unavailable (HTTP 404)"), code: "artifact_resolve_not_found", category: "permanent", retryable: false},
		{name: "deadline", err: context.DeadlineExceeded, code: "media_timeout", category: "transient", retryable: true},
		{name: "cancelled", err: context.Canceled, code: "media_cancelled", category: "cancelled", retryable: false},
		{name: "media input", err: errors.New("required media role is missing"), code: "timeline_render_failed", category: "permanent", retryable: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := failureResult(command, test.err)
			if result.SafeError == nil {
				t.Fatal("safe error is required")
			}
			if result.SafeError.Code != test.code || result.SafeError.Category != test.category || result.SafeError.Retryable != test.retryable {
				t.Fatalf("unexpected classification: %+v", result.SafeError)
			}
		})
	}
}
