package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/product"
)

// These methods are the durable counterpart of product.ExecutionSurface. They
// deliberately reuse the existing SQL graph bootstrap so the HTTP command
// surface cannot create an in-memory shadow of PostgreSQL truth.
func (b *DurableBackend) CreateExecutionJob(workspaceID, projectID string, input product.JobCreateInput) (*product.Job, error) {
	if workspaceID != b.Workspace {
		return nil, product.ErrNotFound
	}
	if input.Kind == "" {
		input.Kind = "pipeline"
	}
	if input.Mode == "" {
		input.Mode = "automatic"
	}
	if input.Mode != "automatic" && input.Mode != "studio" && input.Mode != "preview" {
		return nil, product.ErrConflict
	}
	if input.MaxRuns <= 0 {
		input.MaxRuns = 3
	}
	if input.Priority < 0 || input.Priority > 20 {
		return nil, product.ErrConflict
	}
	command := map[string]any{
		"kind": input.Kind, "workflow_key": input.WorkflowKey, "mode": input.Mode,
		"input": input.Input, "params": input.Params, "start_from": input.StartFrom,
		"stop_after": input.StopAfter, "priority": input.Priority, "max_runs": input.MaxRuns,
		"enable_dlq": input.EnableDLQ,
	}
	body := jsonBytes(command)
	record, err := b.SQL.CreateProjectJob(context.Background(), b.UserID, b.Workspace, projectID, input.Kind, body, body)
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	if input.AutoStart {
		return b.StartJob(workspaceID, projectID, record.ID)
	}
	return jobFromRecord(record), nil
}

func (b *DurableBackend) ListJobs(workspaceID, projectID, status string, limit int) ([]product.Job, error) {
	if workspaceID != b.Workspace {
		return nil, product.ErrNotFound
	}
	if _, err := b.SQL.GetProject(context.Background(), b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args := []any{projectID, limit}
	query := `SELECT id::text,project_id::text,kind,status,command,created_at,COALESCE(current_pipeline_run_id::text,'') FROM jobs WHERE project_id=$1::uuid`
	if strings.TrimSpace(status) != "" {
		query += " AND status=$3"
		args = []any{projectID, status, limit}
	}
	query += " ORDER BY created_at,id LIMIT " + fmt.Sprintf("$%d", len(args))
	rows, err := b.SQL.DB.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]product.Job, 0)
	for rows.Next() {
		var record JobRecord
		if err := rows.Scan(&record.ID, &record.ProjectID, &record.Kind, &record.Status, &record.Command, &record.CreatedAt, &record.PipelineRunID); err != nil {
			return nil, err
		}
		result = append(result, *jobFromRecord(record))
	}
	return result, rows.Err()
}

func (b *DurableBackend) RetryExecutionJob(workspaceID, projectID, jobID string, autoStart bool) (*product.Job, error) {
	old, err := b.GetJob(workspaceID, projectID, jobID)
	if err != nil {
		return nil, err
	}
	if old.Status != domain.JobFailed && old.Status != domain.JobDeadLettered && old.Status != domain.JobCancelled {
		return nil, product.ErrConflict
	}
	command := cloneMap(old.Command)
	command["supersedes_job_id"] = old.ID
	record, err := b.SQL.CreateProjectJob(context.Background(), b.UserID, b.Workspace, projectID, old.Kind, jsonBytes(command), jsonBytes(command))
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	if autoStart {
		return b.StartJob(workspaceID, projectID, record.ID)
	}
	return jobFromRecord(record), nil
}

func (b *DurableBackend) ListPipelineRuns(workspaceID, projectID, jobID string) ([]product.PipelineRun, error) {
	if _, err := b.GetJob(workspaceID, projectID, jobID); err != nil {
		return nil, err
	}
	rows, err := b.SQL.DB.QueryContext(context.Background(), `SELECT id::text,job_id::text,run_number,status,contract_version,COALESCE(resume_checkpoint_id::text,''),created_at FROM pipeline_runs WHERE job_id=$1::uuid ORDER BY run_number`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]product.PipelineRun, 0)
	for rows.Next() {
		var value product.PipelineRun
		if err := rows.Scan(&value.ID, &value.JobID, &value.RunNumber, &value.Status, &value.Contract, &value.CheckpointID, &value.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (b *DurableBackend) GetPipelineRun(workspaceID, projectID, jobID, runID string) (*product.PipelineRun, error) {
	if _, err := b.GetJob(workspaceID, projectID, jobID); err != nil {
		return nil, err
	}
	var value product.PipelineRun
	err := b.SQL.DB.QueryRowContext(context.Background(), `SELECT id::text,job_id::text,run_number,status,contract_version,COALESCE(resume_checkpoint_id::text,''),created_at FROM pipeline_runs WHERE id=$1::uuid AND job_id=$2::uuid`, runID, jobID).Scan(&value.ID, &value.JobID, &value.RunNumber, &value.Status, &value.Contract, &value.CheckpointID, &value.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, product.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (b *DurableBackend) ListJobSteps(workspaceID, projectID, jobID, runID string) ([]product.JobStep, error) {
	if _, err := b.GetJob(workspaceID, projectID, jobID); err != nil {
		return nil, err
	}
	rows, err := b.SQL.DB.QueryContext(context.Background(), `SELECT s.id::text,s.pipeline_run_id::text,s.node_key,s.status,s.current_attempt,s.output_refs,COALESCE(s.skip_reason,''),1 FROM job_steps s JOIN pipeline_runs r ON r.id=s.pipeline_run_id JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE p.workspace_id=$1::uuid AND j.project_id=$2::uuid AND j.id=$3::uuid AND s.pipeline_run_id=$4::uuid ORDER BY s.node_key`, workspaceID, projectID, jobID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]product.JobStep, 0)
	for rows.Next() {
		var value product.JobStep
		var refs []byte
		if err := rows.Scan(&value.ID, &value.PipelineRunID, &value.NodeKey, &value.Status, &value.Attempt, &refs, &value.SkipReason, &value.Revision); err != nil {
			return nil, err
		}
		var decoded []map[string]any
		_ = json.Unmarshal(refs, &decoded)
		value.OutputRefs = decoded
		result = append(result, value)
	}
	return result, rows.Err()
}

func (b *DurableBackend) ApproveReview(workspaceID, projectID, jobID, stepID, resourceID string, resourceRevision int64) (*product.Job, error) {
	if resourceID == "" || resourceRevision < 1 {
		return nil, product.ErrConflict
	}
	return b.resolveReview(context.Background(), workspaceID, projectID, jobID, stepID, "approve", "", resourceID, resourceRevision)
}

func (b *DurableBackend) ApproveReviewWithType(workspaceID, projectID, jobID, stepID, resourceType, resourceID string, resourceRevision int64) (*product.Job, error) {
	if resourceType == "" {
		return nil, product.ErrConflict
	}
	var proposedType string
	if err := b.SQL.DB.QueryRowContext(context.Background(), `SELECT r.proposed_resource_type FROM reviews r JOIN projects p ON p.id=r.project_id WHERE p.workspace_id=$1::uuid AND r.project_id=$2::uuid AND r.job_id=$3::uuid AND r.job_step_id=$4::uuid AND r.status='open'`, workspaceID, projectID, jobID, stepID).Scan(&proposedType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, product.ErrNotFound
		}
		return nil, err
	}
	if proposedType != resourceType {
		return nil, product.ErrConflict
	}
	return b.ApproveReview(workspaceID, projectID, jobID, stepID, resourceID, resourceRevision)
}

func (b *DurableBackend) RejectReview(workspaceID, projectID, jobID, stepID, action string) (*product.Job, error) {
	if action == "" {
		action = "fail"
	}
	if action != "fail" && action != "edit_then_resume" {
		return nil, product.ErrConflict
	}
	return b.resolveReview(context.Background(), workspaceID, projectID, jobID, stepID, "reject", action, "", 0)
}

func (b *DurableBackend) resolveReview(ctx context.Context, workspaceID, projectID, jobID, stepID, decision, action, resourceID string, resourceRevision int64) (*product.Job, error) {
	if b == nil || b.SQL == nil || workspaceID != b.Workspace {
		return nil, product.ErrNotFound
	}
	tx, err := b.SQL.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var reviewID, runID, nodeID, nodeKey string
	var reviewType, proposedType, proposedID, jobStatus, runStatus, stepStatus string
	var proposedRevision int64
	if err := tx.QueryRowContext(ctx, `SELECT r.id::text,r.pipeline_run_id::text,s.pipeline_node_id::text,s.node_key,r.review_type,r.proposed_resource_type,r.proposed_resource_id::text,COALESCE(r.proposed_resource_revision,0),j.status,rrun.status,s.status FROM reviews r JOIN job_steps s ON s.id=r.job_step_id JOIN pipeline_runs rrun ON rrun.id=s.pipeline_run_id JOIN jobs j ON j.id=rrun.job_id JOIN projects p ON p.id=j.project_id WHERE p.workspace_id=$1::uuid AND j.project_id=$2::uuid AND j.id=$3::uuid AND s.id=$4::uuid AND r.status='open' AND r.job_id=j.id AND r.pipeline_run_id=rrun.id FOR UPDATE`, workspaceID, projectID, jobID, stepID).Scan(&reviewID, &runID, &nodeID, &nodeKey, &reviewType, &proposedType, &proposedID, &proposedRevision, &jobStatus, &runStatus, &stepStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, product.ErrNotFound
		}
		return nil, err
	}
	if jobStatus != "waiting_for_review" || runStatus != "waiting_for_review" || stepStatus != "waiting_for_review" {
		return nil, product.ErrConflict
	}
	if decision == "approve" && (resourceID == "" || resourceRevision < 1) {
		return nil, product.ErrConflict
	}
	if decision == "approve" && resourceRevision < proposedRevision {
		return nil, product.ErrConflict
	}
	if decision != "approve" && decision != "reject" {
		return nil, product.ErrConflict
	}
	toStep, toRun, toJob := "completed", "queued", "queued"
	eventType := "review.approved"
	payload := map[string]any{"review_id": reviewID, "review_type": reviewType, "proposed_resource_type": proposedType, "proposed_resource_id": proposedID, "proposed_resource_revision": proposedRevision}
	if decision == "reject" {
		eventType = "review.rejected"
		payload["action"] = action
		if action == "fail" {
			toStep, toRun, toJob = "failed", "failed", "failed"
		} else if action == "edit_then_resume" {
			toStep, toRun, toJob = "paused", "paused", "paused"
		} else {
			return nil, product.ErrConflict
		}
	} else {
		payload["selected_resource_type"] = proposedType
		payload["selected_resource_id"] = resourceID
		payload["selected_resource_revision"] = resourceRevision
	}
	status := "approved"
	if decision == "reject" {
		status = "rejected"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reviews SET status=$5,selected_resource_type=CASE WHEN $5='approved' THEN $6 ELSE NULL END,selected_resource_id=CASE WHEN $5='approved' THEN NULLIF($7,'')::uuid ELSE NULL END,selected_resource_revision=CASE WHEN $5='approved' THEN $8::bigint ELSE NULL END,decision=$9,decision_comment=NULLIF($10,''),resolved_by=NULLIF($11,'')::uuid,resolved_at=now(),updated_at=now() WHERE id=$1::uuid AND job_id=$2::uuid AND job_step_id=$3::uuid AND project_id=$4::uuid AND status='open'`, reviewID, jobID, stepID, projectID, status, proposedType, resourceID, resourceRevision, decision, action, b.UserID); err != nil {
		return nil, err
	}
	if _, _, err := transitionStep(ctx, tx, TransitionInput{Entity: execution.EntityStep, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, JobStepID: stepID, PipelineNodeID: nodeID, NodeKey: nodeKey, FromStatus: stepStatus, ToStatus: toStep, CorrelationID: jobID}); err != nil {
		return nil, mapPersistenceError(err)
	}
	if _, _, err := transitionRun(ctx, tx, TransitionInput{Entity: execution.EntityRun, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: runStatus, ToStatus: toRun, CorrelationID: jobID}); err != nil {
		return nil, mapPersistenceError(err)
	}
	if _, _, err := transitionJob(ctx, tx, TransitionInput{Entity: execution.EntityJob, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: jobStatus, ToStatus: toJob, CorrelationID: jobID}); err != nil {
		return nil, mapPersistenceError(err)
	}
	eventPayload, _ := json.Marshal(payload)
	if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: eventType, CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, JobStepID: stepID, PipelineNodeID: nodeID, NodeKey: nodeKey, Payload: eventPayload}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return b.GetJob(workspaceID, projectID, jobID)
}

func (b *DurableBackend) OpenReview(workspaceID, projectID, jobID, stepID, reviewType, resourceType, resourceID string, resourceRevision int64) (*product.Review, error) {
	if b == nil || b.SQL == nil || workspaceID != b.Workspace || reviewType == "" || resourceType == "" || resourceID == "" || resourceRevision < 1 {
		return nil, product.ErrConflict
	}
	tx, err := b.SQL.DB.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var runID, nodeID, nodeKey, jobStatus, runStatus, stepStatus string
	if err := tx.QueryRowContext(context.Background(), `SELECT r.id::text,s.pipeline_node_id::text,s.node_key,j.status,r.status,s.status FROM job_steps s JOIN pipeline_runs r ON r.id=s.pipeline_run_id JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE p.workspace_id=$1::uuid AND j.project_id=$2::uuid AND j.id=$3::uuid AND s.id=$4::uuid`, workspaceID, projectID, jobID, stepID).Scan(&runID, &nodeID, &nodeKey, &jobStatus, &runStatus, &stepStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, product.ErrNotFound
		}
		return nil, err
	}
	if jobStatus != "running" || runStatus != "running" || stepStatus != "running" {
		return nil, product.ErrConflict
	}
	if reviewType != "script" && reviewType != "timeline" && reviewType != "scene_match" && reviewType != "subtitle" && reviewType != "voice" {
		return nil, product.ErrConflict
	}
	var reviewID string
	if err := tx.QueryRowContext(context.Background(), `INSERT INTO reviews(project_id,job_id,pipeline_run_id,job_step_id,review_type,status,proposed_resource_type,proposed_resource_id,proposed_resource_revision,allowed_actions) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,'open',$6,$7::uuid,$8,'["approve","reject","edit_then_resume"]'::jsonb) RETURNING id::text`, projectID, jobID, runID, stepID, reviewType, resourceType, resourceID, resourceRevision).Scan(&reviewID); err != nil {
		return nil, err
	}
	if _, _, err := transitionJob(context.Background(), tx, TransitionInput{Entity: execution.EntityJob, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: jobStatus, ToStatus: "waiting_for_review", CorrelationID: jobID}); err != nil {
		return nil, err
	}
	if _, _, err := transitionRun(context.Background(), tx, TransitionInput{Entity: execution.EntityRun, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: runStatus, ToStatus: "waiting_for_review", CorrelationID: jobID}); err != nil {
		return nil, err
	}
	if _, _, err := transitionStep(context.Background(), tx, TransitionInput{Entity: execution.EntityStep, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, JobStepID: stepID, PipelineNodeID: nodeID, NodeKey: nodeKey, FromStatus: stepStatus, ToStatus: "waiting_for_review", CorrelationID: jobID}); err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]any{"review_id": reviewID, "review_type": reviewType, "resource_type": resourceType, "resource_id": resourceID, "resource_revision": resourceRevision})
	if _, err := appendEventAndOutboxTx(context.Background(), tx, EventInput{SchemaVersion: "1.0", EventType: "review.required", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, JobStepID: stepID, Payload: payload}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &product.Review{ID: reviewID, JobID: jobID, JobStepID: stepID, PipelineRunID: runID, Status: "open", ReviewType: reviewType, ProposedResourceType: resourceType, ProposedResourceID: resourceID, ProposedResourceRevision: resourceRevision}, nil
}

func jobFromRecord(value JobRecord) *product.Job {
	return &product.Job{ID: value.ID, ProjectID: value.ProjectID, Kind: value.Kind, Status: domain.JobStatus(value.Status), PipelineRunID: value.PipelineRunID, Command: mapFromJSON(value.Command), CreatedAt: value.CreatedAt}
}
