package domain

import "fmt"

type JobStatus string

const (
	JobCreated          JobStatus = "created"
	JobQueued           JobStatus = "queued"
	JobRunning          JobStatus = "running"
	JobPaused           JobStatus = "paused"
	JobWaitingForReview JobStatus = "waiting_for_review"
	JobRetrying         JobStatus = "retrying"
	JobCancelling       JobStatus = "cancelling"
	JobCompleted        JobStatus = "completed"
	JobFailed           JobStatus = "failed"
	JobDeadLettered     JobStatus = "dead_lettered"
	JobCancelled        JobStatus = "cancelled"
)

type JobStepStatus string

const (
	StepPending          JobStepStatus = "pending"
	StepReady            JobStepStatus = "ready"
	StepQueued           JobStepStatus = "queued"
	StepRunning          JobStepStatus = "running"
	StepPaused           JobStepStatus = "paused"
	StepWaitingForReview JobStepStatus = "waiting_for_review"
	StepRetrying         JobStepStatus = "retrying"
	StepCompleted        JobStepStatus = "completed"
	StepSkipped          JobStepStatus = "skipped"
	StepFailed           JobStepStatus = "failed"
	StepCancelled        JobStepStatus = "cancelled"
	StepBlocked          JobStepStatus = "blocked"
)

var CanonicalJobStatuses = []JobStatus{JobCreated, JobQueued, JobRunning, JobPaused, JobWaitingForReview, JobRetrying, JobCancelling, JobCompleted, JobFailed, JobDeadLettered, JobCancelled}
var CanonicalJobStepStatuses = []JobStepStatus{StepPending, StepReady, StepQueued, StepRunning, StepPaused, StepWaitingForReview, StepRetrying, StepCompleted, StepSkipped, StepFailed, StepCancelled, StepBlocked}

func ValidateJobStatus(value string) error {
	for _, status := range CanonicalJobStatuses {
		if value == string(status) {
			return nil
		}
	}
	return fmt.Errorf("invalid Job status %q", value)
}

func ValidateJobStepStatus(value string) error {
	for _, status := range CanonicalJobStepStatuses {
		if value == string(status) {
			return nil
		}
	}
	return fmt.Errorf("invalid JobStep status %q", value)
}

func IsActiveRunStatus(value string) bool {
	return value == string(JobQueued) || value == string(JobRunning) || value == string(JobPaused) || value == string(JobWaitingForReview)
}

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

func CanRead(role Role) bool {
	return role == RoleOwner || role == RoleAdmin || role == RoleEditor || role == RoleViewer
}
func CanEdit(role Role) bool  { return role == RoleOwner || role == RoleAdmin || role == RoleEditor }
func CanAdmin(role Role) bool { return role == RoleOwner || role == RoleAdmin }
