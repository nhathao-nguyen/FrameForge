package execution

import (
	"errors"
	"testing"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
)

func TestValidateTransitionAcceptsCanonicalGraphs(t *testing.T) {
	cases := []struct {
		entity Entity
		from   string
		to     string
	}{
		{EntityJob, string(domain.JobCreated), string(domain.JobQueued)},
		{EntityJob, string(domain.JobRunning), string(domain.JobRetrying)},
		{EntityJob, string(domain.JobRetrying), string(domain.JobDeadLettered)},
		{EntityRun, "created", "queued"},
		{EntityRun, "running", "waiting_for_review"},
		{EntityStep, string(domain.StepPending), string(domain.StepReady)},
		{EntityStep, string(domain.StepRunning), string(domain.StepSkipped)},
		{EntityStep, string(domain.StepWaitingForReview), string(domain.StepCompleted)},
	}
	for _, value := range cases {
		if err := ValidateTransition(value.entity, value.from, value.to); err != nil {
			t.Errorf("%s %s -> %s: unexpected error: %v", value.entity, value.from, value.to, err)
		}
	}
}

func TestValidateTransitionRejectsInvalidAndTerminalMutations(t *testing.T) {
	cases := []struct {
		entity Entity
		from   string
		to     string
	}{
		{EntityJob, string(domain.JobCreated), string(domain.JobRunning)},
		{EntityJob, string(domain.JobCompleted), string(domain.JobQueued)},
		{EntityJob, string(domain.JobCancelled), string(domain.JobFailed)},
		{EntityRun, "completed", "queued"},
		{EntityStep, string(domain.StepPending), string(domain.StepCompleted)},
		{EntityStep, string(domain.StepFailed), string(domain.StepQueued)},
		{EntityStep, "processing", string(domain.StepRunning)},
		{Entity("unknown"), "created", "queued"},
	}
	for _, value := range cases {
		err := ValidateTransition(value.entity, value.from, value.to)
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("%s %s -> %s: expected ErrInvalidTransition, got %v", value.entity, value.from, value.to, err)
		}
	}
}

func TestEventTypeUsesExplicitNames(t *testing.T) {
	if got := EventType(EntityJob, "created", "queued"); got != "job.queued" {
		t.Fatalf("unexpected job event: %q", got)
	}
	if got := EventType(EntityStep, "running", "completed"); got != "node.completed" {
		t.Fatalf("unexpected node event: %q", got)
	}
	if got := EventType(EntityRun, "created", "queued"); got != "pipeline_run.queued" {
		t.Fatalf("unexpected run event: %q", got)
	}
}
