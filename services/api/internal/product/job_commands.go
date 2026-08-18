package product

import (
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
)

func (s *Store) PauseJob(workspaceID, projectID, jobID string) (*Job, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.jobs[jobID]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	if err := execution.ValidateTransition(execution.EntityJob, string(value.Status), string(domain.JobPaused)); err != nil {
		return nil, ErrConflict
	}
	run := s.runs[value.PipelineRunID]
	if run != nil && run.Status != "running" {
		return nil, ErrConflict
	}
	s.transitionJobLocked(value, domain.JobPaused)
	if run != nil {
		s.transitionRunLocked(value.ID, run, "paused")
		for index := range s.steps[run.ID] {
			step := &s.steps[run.ID][index]
			if step.Status != string(domain.StepRunning) {
				continue
			}
			s.transitionStepLocked(value.ID, step, string(domain.StepPaused))
		}
	}
	return cloneJob(value), nil
}

func (s *Store) ResumeJob(workspaceID, projectID, jobID string) (*Job, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.jobs[jobID]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	if value.Status != domain.JobPaused {
		return nil, ErrConflict
	}
	run := s.runs[value.PipelineRunID]
	if run != nil && run.Status != "paused" {
		return nil, ErrConflict
	}
	s.transitionJobLocked(value, domain.JobQueued)
	if run != nil {
		s.transitionRunLocked(value.ID, run, "queued")
		for index := range s.steps[run.ID] {
			step := &s.steps[run.ID][index]
			if step.Status == string(domain.StepPaused) {
				s.transitionStepLocked(value.ID, step, string(domain.StepQueued))
			}
		}
	}
	return cloneJob(value), nil
}

func (s *Store) CancelJob(workspaceID, projectID, jobID string) (*Job, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.jobs[jobID]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	to := domain.JobCancelled
	if value.Status == domain.JobRunning {
		to = domain.JobCancelling
	}
	if value.Status == domain.JobCancelling || value.Status == domain.JobCancelled {
		return cloneJob(value), nil
	}
	if err := execution.ValidateTransition(execution.EntityJob, string(value.Status), string(to)); err != nil {
		return nil, ErrConflict
	}
	s.transitionJobLocked(value, to)
	if to == domain.JobCancelled {
		if run := s.runs[value.PipelineRunID]; run != nil {
			if run.Status != "cancelled" && run.Status != "completed" && run.Status != "failed" {
				s.transitionRunLocked(value.ID, run, "cancelled")
			}
			for index := range s.steps[run.ID] {
				step := &s.steps[run.ID][index]
				if step.Status == string(domain.StepPending) || step.Status == string(domain.StepReady) || step.Status == string(domain.StepQueued) || step.Status == string(domain.StepPaused) || step.Status == string(domain.StepWaitingForReview) || step.Status == string(domain.StepRetrying) {
					s.transitionStepLocked(value.ID, step, string(domain.StepCancelled))
				}
			}
		}
	}
	return cloneJob(value), nil
}

func (s *Store) transitionJob(workspaceID, projectID, jobID string, to domain.JobStatus) (*Job, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.jobs[jobID]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	if value.Status == to {
		return cloneJob(value), nil
	}
	if err := execution.ValidateTransition(execution.EntityJob, string(value.Status), string(to)); err != nil {
		return nil, ErrConflict
	}
	return s.transitionJobLocked(value, to), nil
}

func (s *Store) transitionJobLocked(value *Job, to domain.JobStatus) *Job {
	from := value.Status
	value.Status = to
	eventType := execution.EventType(execution.EntityJob, string(from), string(to))
	s.jobEvents[value.ID] = append(s.jobEvents[value.ID], JobEvent{ID: newID("evt"), EventType: eventType, Sequence: int64(len(s.jobEvents[value.ID]) + 1), Payload: map[string]any{"from_status": from, "to_status": to}, OccurredAt: now()})
	return cloneJob(value)
}

func (s *Store) transitionRunLocked(jobID string, run *PipelineRun, to string) {
	from := run.Status
	if from == to {
		return
	}
	run.Status = to
	s.appendJobEventLocked(jobID, execution.EventType(execution.EntityRun, from, to), map[string]any{"from_status": from, "to_status": to})
}

func (s *Store) transitionStepLocked(jobID string, step *JobStep, to string) {
	from := step.Status
	if from == to {
		return
	}
	step.Status = to
	step.Revision++
	s.appendJobEventLocked(jobID, execution.EventType(execution.EntityStep, from, to), map[string]any{"node_key": step.NodeKey, "from_status": from, "to_status": to})
}
