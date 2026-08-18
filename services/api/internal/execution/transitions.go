package execution

import (
	"errors"
	"fmt"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
)

// Entity identifies the execution aggregate whose state is being changed.
// Pipeline definitions do not have runtime state; only these three execution
// records are transitionable.
type Entity string

const (
	EntityJob  Entity = "job"
	EntityRun  Entity = "pipeline_run"
	EntityStep Entity = "job_step"
)

var ErrInvalidTransition = errors.New("invalid execution state transition")

type TransitionError struct {
	Entity Entity
	From   string
	To     string
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("%s cannot transition from %q to %q", e.Entity, e.From, e.To)
}

func (e *TransitionError) Unwrap() error { return ErrInvalidTransition }

func valid(from, to string, allowed map[string]map[string]struct{}) bool {
	next, ok := allowed[from]
	if !ok {
		return false
	}
	_, ok = next[to]
	return ok
}

func set(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

var jobTransitions = map[string]map[string]struct{}{
	string(domain.JobCreated):          set(string(domain.JobQueued), string(domain.JobCancelled)),
	string(domain.JobQueued):           set(string(domain.JobRunning), string(domain.JobCancelling), string(domain.JobCancelled)),
	string(domain.JobRunning):          set(string(domain.JobPaused), string(domain.JobWaitingForReview), string(domain.JobRetrying), string(domain.JobCancelling), string(domain.JobCompleted), string(domain.JobFailed)),
	string(domain.JobPaused):           set(string(domain.JobQueued), string(domain.JobCancelling), string(domain.JobCancelled)),
	string(domain.JobWaitingForReview): set(string(domain.JobQueued), string(domain.JobPaused), string(domain.JobCancelling), string(domain.JobFailed), string(domain.JobCancelled)),
	string(domain.JobRetrying):         set(string(domain.JobQueued), string(domain.JobCancelling), string(domain.JobFailed), string(domain.JobDeadLettered), string(domain.JobCancelled)),
	string(domain.JobCancelling):       set(string(domain.JobCancelled), string(domain.JobFailed)),
}

var runTransitions = map[string]map[string]struct{}{
	"created":            set("queued", "cancelled"),
	"queued":             set("running", "cancelled"),
	"running":            set("paused", "waiting_for_review", "completed", "failed", "cancelled"),
	"paused":             set("queued", "cancelled"),
	"waiting_for_review": set("queued", "paused", "completed", "failed", "cancelled"),
}

var stepTransitions = map[string]map[string]struct{}{
	string(domain.StepPending):          set(string(domain.StepReady), string(domain.StepBlocked), string(domain.StepCancelled)),
	string(domain.StepReady):            set(string(domain.StepQueued), string(domain.StepCancelled), string(domain.StepBlocked)),
	string(domain.StepQueued):           set(string(domain.StepRunning), string(domain.StepCancelled)),
	string(domain.StepRunning):          set(string(domain.StepCompleted), string(domain.StepSkipped), string(domain.StepPaused), string(domain.StepWaitingForReview), string(domain.StepRetrying), string(domain.StepFailed), string(domain.StepCancelled)),
	string(domain.StepPaused):           set(string(domain.StepQueued), string(domain.StepCancelled)),
	string(domain.StepWaitingForReview): set(string(domain.StepCompleted), string(domain.StepPaused), string(domain.StepQueued), string(domain.StepFailed), string(domain.StepCancelled)),
	string(domain.StepRetrying):         set(string(domain.StepQueued), string(domain.StepFailed), string(domain.StepCancelled)),
}

// ValidateTransition is the single application/domain gate for all execution
// state changes. Unknown states and terminal-state mutations fail closed.
func ValidateTransition(entity Entity, from, to string) error {
	var allowed map[string]map[string]struct{}
	switch entity {
	case EntityJob:
		if err := domain.ValidateJobStatus(from); err != nil {
			return &TransitionError{Entity: entity, From: from, To: to}
		}
		if err := domain.ValidateJobStatus(to); err != nil {
			return &TransitionError{Entity: entity, From: from, To: to}
		}
		allowed = jobTransitions
	case EntityRun:
		allowed = runTransitions
	case EntityStep:
		if err := domain.ValidateJobStepStatus(from); err != nil {
			return &TransitionError{Entity: entity, From: from, To: to}
		}
		if err := domain.ValidateJobStepStatus(to); err != nil {
			return &TransitionError{Entity: entity, From: from, To: to}
		}
		allowed = stepTransitions
	default:
		return &TransitionError{Entity: entity, From: from, To: to}
	}
	if !valid(from, to, allowed) {
		return &TransitionError{Entity: entity, From: from, To: to}
	}
	return nil
}

// EventType returns a stable, explicit event name for a transition. It never
// creates a generic status alias that clients could mistake for a state.
func EventType(entity Entity, from, to string) string {
	if entity == EntityJob {
		switch to {
		case "queued":
			return "job.queued"
		case "running":
			return "job.started"
		case "paused":
			return "job.paused"
		case "waiting_for_review":
			return "job.waiting_for_review"
		case "retrying":
			return "job.retry_scheduled"
		case "cancelling":
			return "job.cancellation_requested"
		case "completed":
			return "job.completed"
		case "failed":
			return "job.failed"
		case "dead_lettered":
			return "job.dead_lettered"
		case "cancelled":
			return "job.cancelled"
		}
	}
	if entity == EntityRun {
		switch to {
		case "queued":
			return "pipeline_run.queued"
		case "running":
			return "pipeline_run.started"
		case "paused":
			return "pipeline_run.paused"
		case "waiting_for_review":
			return "pipeline_run.waiting_for_review"
		case "completed":
			return "pipeline_run.completed"
		case "failed":
			return "pipeline_run.failed"
		case "cancelled":
			return "pipeline_run.cancelled"
		}
	}
	if entity == EntityStep {
		switch to {
		case "ready":
			return "node.ready"
		case "queued":
			return "node.queued"
		case "running":
			return "node.started"
		case "paused":
			return "node.paused"
		case "waiting_for_review":
			return "node.waiting_for_review"
		case "retrying":
			return "node.retry_scheduled"
		case "completed":
			return "node.completed"
		case "skipped":
			return "node.skipped"
		case "failed":
			return "node.failed"
		case "cancelled":
			return "node.cancelled"
		case "blocked":
			return "node.blocked"
		}
	}
	return ""
}
