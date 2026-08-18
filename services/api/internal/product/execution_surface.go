package product

import (
	"errors"
	"sort"
	"strings"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
)

// CreateExecutionJob is the native command boundary used by the first client
// fixture. The command is copied into the immutable Job snapshot; subsequent
// state is always server-owned and is never accepted back from the client.
func (s *Store) CreateExecutionJob(workspaceID, projectID string, input JobCreateInput) (*Job, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	if input.Kind == "" {
		input.Kind = "pipeline"
	}
	switch input.Kind {
	case "pipeline", "render", "analysis", "asset_probe", "export":
	default:
		return nil, errors.New("invalid job kind")
	}
	if input.Mode == "" {
		input.Mode = "automatic"
	}
	if input.Mode != "automatic" && input.Mode != "studio" && input.Mode != "preview" {
		return nil, errors.New("invalid job mode")
	}
	if input.MaxRuns <= 0 {
		input.MaxRuns = 3
	}
	if input.Priority < 0 || input.Priority > 20 {
		return nil, errors.New("job priority is out of range")
	}
	nodes := executionNodes(input)
	if input.StartFrom != "" && !contains(nodes, input.StartFrom) {
		return nil, errors.New("start_from boundary is not in the pipeline snapshot")
	}
	if input.StopAfter != "" && !contains(nodes, input.StopAfter) {
		return nil, errors.New("stop_after boundary is not in the pipeline snapshot")
	}
	command := map[string]any{
		"kind": input.Kind, "workflow_key": input.WorkflowKey, "mode": input.Mode,
		"input": copyMap(input.Input), "params": copyMap(input.Params), "nodes": append([]string(nil), nodes...),
		"start_from": input.StartFrom, "stop_after": input.StopAfter,
		"priority": input.Priority, "max_runs": input.MaxRuns, "enable_dlq": input.EnableDLQ,
	}
	nowValue := now()
	job := &Job{ID: newID("job"), ProjectID: projectID, Kind: input.Kind, Status: domain.JobCreated, PipelineRunID: newID("run"), Command: command, CreatedAt: nowValue}
	run := &PipelineRun{ID: job.PipelineRunID, JobID: job.ID, RunNumber: 1, Status: "created", Contract: "worker/v1", CreatedAt: nowValue}
	stepValues := make([]JobStep, 0, len(nodes))
	for _, node := range nodes {
		stepValues = append(stepValues, JobStep{ID: newID("step"), PipelineRunID: run.ID, NodeKey: node, Status: "pending", Revision: 1})
	}
	s.mu.Lock()
	s.jobs[job.ID] = job
	s.runs[run.ID] = run
	s.steps[run.ID] = stepValues
	s.jobEvents[job.ID] = []JobEvent{
		{ID: newID("evt"), EventType: "job.created", Sequence: 1, Payload: map[string]any{"kind": input.Kind, "mode": input.Mode}, OccurredAt: nowValue},
		{ID: newID("evt"), EventType: "pipeline_run.created", Sequence: 2, Payload: map[string]any{"run_number": 1, "contract_version": "worker/v1"}, OccurredAt: nowValue},
	}
	s.mu.Unlock()
	if input.AutoStart {
		return s.StartJob(workspaceID, projectID, job.ID)
	}
	return cloneJob(job), nil
}

func executionNodes(input JobCreateInput) []string {
	if raw, ok := input.Input["nodes"].([]any); ok {
		result := make([]string, 0, len(raw))
		for _, value := range raw {
			if node, ok := value.(string); ok && strings.TrimSpace(node) != "" {
				result = append(result, node)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	if raw, ok := input.Input["nodes"].([]string); ok && len(raw) > 0 {
		return append([]string(nil), raw...)
	}
	switch input.Kind {
	case "asset_probe":
		return []string{"asset_probe"}
	case "analysis":
		return []string{"analysis"}
	case "render":
		return []string{"render"}
	default:
		return []string{"pipeline"}
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *Store) ListJobs(workspaceID, projectID, status string, limit int) ([]Job, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	values := make([]Job, 0)
	for _, value := range s.jobs {
		if value.ProjectID != projectID || (status != "" && string(value.Status) != status) {
			continue
		}
		values = append(values, *cloneJob(value))
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].CreatedAt.Equal(values[j].CreatedAt) {
			return values[i].ID < values[j].ID
		}
		return values[i].CreatedAt.Before(values[j].CreatedAt)
	})
	if len(values) > limit {
		values = values[:limit]
	}
	return values, nil
}

func (s *Store) RetryExecutionJob(workspaceID, projectID, jobID string, autoStart bool) (*Job, error) {
	old, err := s.GetJob(workspaceID, projectID, jobID)
	if err != nil {
		return nil, err
	}
	if old.Status != domain.JobFailed && old.Status != domain.JobDeadLettered && old.Status != domain.JobCancelled {
		return nil, ErrConflict
	}
	input := JobCreateInput{Kind: old.Kind, Mode: stringFromMap(old.Command, "mode", "automatic"), WorkflowKey: stringFromMap(old.Command, "workflow_key", ""), Input: copyMapMap(old.Command, "input"), Params: copyMapMap(old.Command, "params"), StartFrom: stringFromMap(old.Command, "start_from", ""), StopAfter: stringFromMap(old.Command, "stop_after", ""), EnableDLQ: boolFromMap(old.Command, "enable_dlq"), AutoStart: autoStart}
	value, err := s.CreateExecutionJob(workspaceID, projectID, input)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	value.Command["supersedes_job_id"] = old.ID
	if stored := s.jobs[value.ID]; stored != nil {
		stored.Command["supersedes_job_id"] = old.ID
	}
	s.mu.Unlock()
	return value, nil
}

func (s *Store) ListPipelineRuns(workspaceID, projectID, jobID string) ([]PipelineRun, error) {
	if _, err := s.GetJob(workspaceID, projectID, jobID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	values := make([]PipelineRun, 0)
	for _, run := range s.runs {
		if run.JobID == jobID {
			values = append(values, *run)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].RunNumber < values[j].RunNumber })
	return values, nil
}

func (s *Store) GetPipelineRun(workspaceID, projectID, jobID, runID string) (*PipelineRun, error) {
	if _, err := s.GetJob(workspaceID, projectID, jobID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[runID]
	if !ok || run.JobID != jobID {
		return nil, ErrNotFound
	}
	copyValue := *run
	return &copyValue, nil
}

func (s *Store) ListJobSteps(workspaceID, projectID, jobID, runID string) ([]JobStep, error) {
	if _, err := s.GetJob(workspaceID, projectID, jobID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[runID]
	if !ok || run.JobID != jobID {
		return nil, ErrNotFound
	}
	return append([]JobStep(nil), s.steps[runID]...), nil
}

func (s *Store) ApproveReview(workspaceID, projectID, jobID, stepID, resourceID string, resourceRevision int64) (*Job, error) {
	return s.resolveReview(workspaceID, projectID, jobID, stepID, "approve", resourceID, resourceRevision, "")
}

func (s *Store) ApproveReviewWithType(workspaceID, projectID, jobID, stepID, resourceType, resourceID string, resourceRevision int64) (*Job, error) {
	s.mu.RLock()
	var expectedType string
	for _, review := range s.reviews {
		if review.JobID == jobID && review.JobStepID == stepID && review.Status == "open" {
			expectedType = review.ProposedResourceType
			break
		}
	}
	s.mu.RUnlock()
	if expectedType == "" || resourceType != expectedType {
		return nil, ErrConflict
	}
	return s.resolveReview(workspaceID, projectID, jobID, stepID, "approve", resourceID, resourceRevision, "")
}

// OpenReview is called by orchestration when a node reaches a human gate. It
// is intentionally separate from pause: a review has a resource/version and
// an actor decision, while pause is only a safe execution checkpoint.
func (s *Store) OpenReview(workspaceID, projectID, jobID, stepID, reviewType, resourceType, resourceID string, resourceRevision int64) (*Review, error) {
	if _, err := s.GetJob(workspaceID, projectID, jobID); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.jobs[jobID]
	run := s.runs[job.PipelineRunID]
	if job == nil || run == nil || job.Status != domain.JobRunning || run.Status != "running" {
		return nil, ErrConflict
	}
	var step *JobStep
	for index := range s.steps[run.ID] {
		if s.steps[run.ID][index].ID == stepID {
			step = &s.steps[run.ID][index]
			break
		}
	}
	if step == nil || step.Status != "running" || reviewType == "" || resourceType == "" || resourceID == "" || resourceRevision < 1 {
		return nil, ErrConflict
	}
	if !validReviewType(reviewType) {
		return nil, ErrConflict
	}
	if err := execution.ValidateTransition(execution.EntityJob, string(job.Status), string(domain.JobWaitingForReview)); err != nil {
		return nil, ErrConflict
	}
	if err := execution.ValidateTransition(execution.EntityRun, run.Status, "waiting_for_review"); err != nil {
		return nil, ErrConflict
	}
	if err := execution.ValidateTransition(execution.EntityStep, step.Status, "waiting_for_review"); err != nil {
		return nil, ErrConflict
	}
	s.transitionJobLocked(job, domain.JobWaitingForReview)
	s.transitionRunLocked(job.ID, run, "waiting_for_review")
	s.transitionStepLocked(job.ID, step, "waiting_for_review")
	review := &Review{ID: newID("review"), JobID: jobID, JobStepID: stepID, PipelineRunID: run.ID, Status: "open", ReviewType: reviewType, ProposedResourceType: resourceType, ProposedResourceID: resourceID, ProposedResourceRevision: resourceRevision}
	s.reviews[review.ID] = review
	s.appendJobEventLocked(jobID, "review.required", map[string]any{"review_id": review.ID, "review_type": reviewType, "resource_id": resourceID, "resource_revision": resourceRevision})
	return cloneReview(review), nil
}

func (s *Store) RejectReview(workspaceID, projectID, jobID, stepID, action string) (*Job, error) {
	if action == "" {
		action = "fail"
	}
	if action != "fail" && action != "edit_then_resume" {
		return nil, ErrConflict
	}
	return s.resolveReview(workspaceID, projectID, jobID, stepID, "reject", "", 0, action)
}

func (s *Store) resolveReview(workspaceID, projectID, jobID, stepID, decision, resourceID string, resourceRevision int64, action string) (*Job, error) {
	if _, err := s.GetJob(workspaceID, projectID, jobID); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var review *Review
	for _, value := range s.reviews {
		if value.JobID == jobID && value.JobStepID == stepID && value.Status == "open" {
			review = value
			break
		}
	}
	if review == nil {
		return nil, ErrNotFound
	}
	value := s.jobs[jobID]
	run := s.runs[review.PipelineRunID]
	var step *JobStep
	for index := range s.steps[review.PipelineRunID] {
		if s.steps[review.PipelineRunID][index].ID == stepID {
			step = &s.steps[review.PipelineRunID][index]
			break
		}
	}
	if value == nil || run == nil || step == nil || value.Status != domain.JobWaitingForReview || run.Status != "waiting_for_review" || step.Status != "waiting_for_review" {
		return nil, ErrConflict
	}
	if decision == "approve" && (resourceID == "" || resourceRevision < 1) {
		return nil, ErrConflict
	}
	if decision == "approve" && resourceRevision < review.ProposedResourceRevision {
		return nil, ErrConflict
	}
	if decision != "approve" && decision != "reject" {
		return nil, ErrConflict
	}
	if decision == "reject" && action != "fail" && action != "edit_then_resume" {
		return nil, ErrConflict
	}
	review.Status = map[string]string{"approve": "approved", "reject": "rejected"}[decision]
	review.Decision = decision
	review.SelectedResourceType = review.ProposedResourceType
	review.SelectedResourceID = resourceID
	review.SelectedResourceRevision = resourceRevision
	review.DecisionComment = action
	if decision == "reject" && action == "fail" {
		s.transitionStepLocked(jobID, step, "failed")
		s.transitionRunLocked(jobID, run, "failed")
		s.transitionJobLocked(value, domain.JobFailed)
		s.appendJobEventLocked(jobID, "review.rejected", map[string]any{"review_id": review.ID, "action": action})
	} else if decision == "reject" && action == "edit_then_resume" {
		s.transitionStepLocked(jobID, step, "paused")
		s.transitionRunLocked(jobID, run, "paused")
		s.transitionJobLocked(value, domain.JobPaused)
		s.appendJobEventLocked(jobID, "review.rejected", map[string]any{"review_id": review.ID, "action": action})
	} else {
		s.transitionStepLocked(jobID, step, "completed")
		s.transitionRunLocked(jobID, run, "queued")
		s.transitionJobLocked(value, domain.JobQueued)
		s.appendJobEventLocked(jobID, "review.approved", map[string]any{"review_id": review.ID, "selected_resource_id": resourceID, "selected_resource_revision": resourceRevision})
	}
	return cloneJob(s.jobs[jobID]), nil
}

func validReviewType(value string) bool {
	switch value {
	case "script", "timeline", "scene_match", "subtitle", "voice":
		return true
	default:
		return false
	}
}

func (s *Store) appendJobEventLocked(jobID, eventType string, payload map[string]any) {
	values := s.jobEvents[jobID]
	values = append(values, JobEvent{ID: newID("evt"), EventType: eventType, Sequence: int64(len(values) + 1), Payload: copyMap(payload), OccurredAt: now()})
	s.jobEvents[jobID] = values
}

func cloneReview(value *Review) *Review {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func stringFromMap(value map[string]any, key, fallback string) string {
	result, ok := value[key].(string)
	if !ok || result == "" {
		return fallback
	}
	return result
}

func boolFromMap(value map[string]any, key string) bool {
	result, _ := value[key].(bool)
	return result
}

func copyMapMap(value map[string]any, key string) map[string]any {
	result, _ := value[key].(map[string]any)
	return copyMap(result)
}
