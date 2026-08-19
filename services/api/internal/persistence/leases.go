package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
)

type LeaseClaimResult struct {
	Lease execution.Lease
}

// ClaimStep atomically claims a queued JobStep, creates its attempt record and
// emits node.started. The raw token is returned once and never written to the
// database or an event.
func (s *SQLStore) ClaimStep(ctx context.Context, input execution.LeaseClaim) (LeaseClaimResult, error) {
	if s == nil || s.DB == nil {
		return LeaseClaimResult{}, errors.New("database is required")
	}
	if input.WorkerID == "" || input.JobID == "" || input.JobStepID == "" || input.PipelineRunID == "" || input.WorkspaceID == "" || input.ProjectID == "" {
		return LeaseClaimResult{}, errors.New("lease scope is incomplete")
	}
	if input.ExpectedAttempt < 0 {
		return LeaseClaimResult{}, errors.New("expected attempt cannot be negative")
	}
	duration := input.Duration
	if duration <= 0 {
		duration = 2 * time.Minute
	}
	if duration > time.Hour {
		return LeaseClaimResult{}, errors.New("lease duration is too long")
	}
	lease, err := execution.NewLease(duration, input.ExpectedAttempt+1, time.Now().UTC())
	if err != nil {
		return LeaseClaimResult{}, err
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return LeaseClaimResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var jobStatus, runStatus string
	if err := tx.QueryRowContext(ctx, `SELECT j.status,r.status FROM pipeline_runs r JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE r.id=$1::uuid AND r.job_id=$2::uuid AND j.project_id=$3::uuid AND p.workspace_id=$4::uuid`, input.PipelineRunID, input.JobID, input.ProjectID, input.WorkspaceID).Scan(&jobStatus, &runStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LeaseClaimResult{}, execution.ErrLeaseConflict
		}
		return LeaseClaimResult{}, err
	}
	if jobStatus != "queued" && jobStatus != "running" {
		return LeaseClaimResult{}, execution.ErrLeaseConflict
	}
	if runStatus != "queued" && runStatus != "running" {
		return LeaseClaimResult{}, execution.ErrLeaseConflict
	}
	if jobStatus == "queued" {
		if _, _, err := transitionJob(ctx, tx, TransitionInput{Entity: execution.EntityJob, WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID, PipelineRunID: input.PipelineRunID, FromStatus: "queued", ToStatus: "running", CorrelationID: input.JobID}); err != nil {
			return LeaseClaimResult{}, err
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "job.started", CorrelationID: input.JobID, WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID, PipelineRunID: input.PipelineRunID, Payload: transitionPayload(nil, "queued", "running")}); err != nil {
			return LeaseClaimResult{}, err
		}
	}
	if runStatus == "queued" {
		if _, _, err := transitionRun(ctx, tx, TransitionInput{Entity: execution.EntityRun, WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID, PipelineRunID: input.PipelineRunID, FromStatus: "queued", ToStatus: "running", CorrelationID: input.JobID}); err != nil {
			return LeaseClaimResult{}, err
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "pipeline_run.started", CorrelationID: input.JobID, WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID, PipelineRunID: input.PipelineRunID, Payload: transitionPayload(nil, "queued", "running")}); err != nil {
			return LeaseClaimResult{}, err
		}
	}
	var attempt int
	query := `UPDATE job_steps s SET status='running',current_attempt=s.current_attempt+1,heartbeat_at=now(),started_at=COALESCE(s.started_at,now()),updated_at=now() FROM pipeline_runs r JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE s.id=$1::uuid AND s.pipeline_run_id=$2::uuid AND r.job_id=$3::uuid AND j.project_id=$4::uuid AND p.workspace_id=$5::uuid AND s.status='queued' AND s.current_attempt=$6 RETURNING s.current_attempt`
	if err := tx.QueryRowContext(ctx, query, input.JobStepID, input.PipelineRunID, input.JobID, input.ProjectID, input.WorkspaceID, input.ExpectedAttempt).Scan(&attempt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LeaseClaimResult{}, execution.ErrLeaseConflict
		}
		return LeaseClaimResult{}, err
	}
	var attemptID string
	if err := tx.QueryRowContext(ctx, `INSERT INTO job_step_attempts(job_step_id,attempt,status,worker_id,lease_token_hash,lease_expires_at,input_fingerprint,started_at,heartbeat_at) VALUES($1::uuid,$2,'running',$3,$4,$5,NULLIF($6,''),now(),now()) RETURNING id::text`, input.JobStepID, attempt, input.WorkerID, lease.TokenHash, lease.ExpiresAt, input.InputFingerprint).Scan(&attemptID); err != nil {
		return LeaseClaimResult{}, err
	}
	if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{
		SchemaVersion: "1.0", EventType: "node.started", CorrelationID: input.JobID,
		WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID,
		PipelineRunID: input.PipelineRunID, JobStepID: input.JobStepID, PipelineNodeID: input.PipelineNodeID,
		NodeKey: input.NodeKey, Payload: transitionPayload(nil, "queued", "running"),
	}); err != nil {
		return LeaseClaimResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return LeaseClaimResult{}, err
	}
	lease.Attempt = attempt
	lease.AttemptID = attemptID
	return LeaseClaimResult{Lease: lease}, nil
}

func (s *SQLStore) HeartbeatStep(ctx context.Context, workspaceID, projectID, jobID, pipelineRunID, jobStepID, workerID, token string, duration time.Duration) (time.Time, error) {
	if s == nil || s.DB == nil {
		return time.Time{}, errors.New("database is required")
	}
	if workspaceID == "" || projectID == "" || jobID == "" || pipelineRunID == "" || jobStepID == "" || workerID == "" || token == "" {
		return time.Time{}, errors.New("heartbeat scope is incomplete")
	}
	if duration <= 0 || duration > time.Hour {
		return time.Time{}, errors.New("heartbeat duration is invalid")
	}
	expiresAt := time.Now().UTC().Add(duration)
	result, err := s.DB.ExecContext(ctx, `UPDATE job_step_attempts a SET lease_expires_at=$9,heartbeat_at=now(),updated_at=now() FROM job_steps s JOIN pipeline_runs r ON r.id=s.pipeline_run_id JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE a.job_step_id=s.id AND a.status='running' AND a.worker_id=$1 AND a.lease_token_hash=$2 AND a.lease_expires_at > now() AND s.id=$3::uuid AND r.id=$4::uuid AND j.id=$5::uuid AND j.project_id=$6::uuid AND p.workspace_id=$7::uuid AND a.attempt=(SELECT current_attempt FROM job_steps WHERE id=s.id)`, workerID, execution.HashLeaseToken(token), jobStepID, pipelineRunID, jobID, projectID, workspaceID, expiresAt)
	if err != nil {
		return time.Time{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return time.Time{}, err
	}
	if count != 1 {
		return time.Time{}, execution.ErrLeaseConflict
	}
	return expiresAt, nil
}

// ReconcileExpiredStep closes the current attempt and moves its JobStep into
// retrying. A stale worker cannot win because both updates require the active
// attempt's hash, status and expiry predicate.
func (s *SQLStore) ReconcileExpiredStep(ctx context.Context, input execution.LeaseClaim) error {
	if s == nil || s.DB == nil {
		return errors.New("database is required")
	}
	if input.WorkspaceID == "" || input.ProjectID == "" || input.JobID == "" || input.PipelineRunID == "" || input.JobStepID == "" {
		return errors.New("reconciliation scope is incomplete")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var attempt int
	if err := tx.QueryRowContext(ctx, `UPDATE job_step_attempts a SET status='failed',failure_category='worker_lost',error_code='LEASE_EXPIRED',retryable=true,completed_at=now(),updated_at=now() FROM job_steps s JOIN pipeline_runs r ON r.id=s.pipeline_run_id JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE a.job_step_id=s.id AND a.status='running' AND a.lease_expires_at <= now() AND s.id=$1::uuid AND r.id=$2::uuid AND j.id=$3::uuid AND j.project_id=$4::uuid AND p.workspace_id=$5::uuid RETURNING a.attempt`, input.JobStepID, input.PipelineRunID, input.JobID, input.ProjectID, input.WorkspaceID).Scan(&attempt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return execution.ErrLeaseConflict
		}
		return err
	}
	var stepID string
	if err := tx.QueryRowContext(ctx, `UPDATE job_steps SET status='retrying',failure_category='worker_lost',error_code='LEASE_EXPIRED',retryable=true,updated_at=now() WHERE id=$1::uuid AND pipeline_run_id=$2::uuid AND status='running' RETURNING id::text`, input.JobStepID, input.PipelineRunID).Scan(&stepID); err != nil {
		return fmt.Errorf("reconcile job step: %w", err)
	}
	if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{
		SchemaVersion: "1.0", EventType: "node.retry_scheduled", CorrelationID: input.JobID,
		WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID,
		PipelineRunID: input.PipelineRunID, JobStepID: input.JobStepID, PipelineNodeID: input.PipelineNodeID,
		NodeKey: input.NodeKey, Payload: transitionPayload(leasePayload(attempt), "running", "retrying"),
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// ReconcileExpiredSteps finds active attempts abandoned by a worker or
// controller restart and moves each still-running step into the durable retry
// state. The per-step update remains guarded by the lease expiry predicate, so
// a late heartbeat or result can only win before the lease actually expires.
func (s *SQLStore) ReconcileExpiredSteps(ctx context.Context, workspaceID string) (int, error) {
	if s == nil || s.DB == nil {
		return 0, errors.New("database is required")
	}
	if workspaceID == "" {
		return 0, errors.New("lease reconciliation workspace is required")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT j.project_id::text,j.id::text,r.id::text,s.id::text,s.pipeline_node_id::text,s.node_key
		FROM job_step_attempts a
		JOIN job_steps s ON s.id=a.job_step_id
		JOIN pipeline_runs r ON r.id=s.pipeline_run_id
		JOIN jobs j ON j.id=r.job_id
		JOIN projects p ON p.id=j.project_id
		WHERE p.workspace_id=$1::uuid AND a.status='running' AND a.lease_expires_at <= now()
		  AND s.status='running' AND a.attempt=s.current_attempt
		ORDER BY a.lease_expires_at,s.id`, workspaceID)
	if err != nil {
		return 0, err
	}
	type expiredLease struct {
		projectID, jobID, runID, stepID, nodeID, nodeKey string
	}
	values := make([]expiredLease, 0)
	for rows.Next() {
		var value expiredLease
		if err := rows.Scan(&value.projectID, &value.jobID, &value.runID, &value.stepID, &value.nodeID, &value.nodeKey); err != nil {
			_ = rows.Close()
			return 0, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	reconciled := 0
	for _, value := range values {
		err := s.ReconcileExpiredStep(ctx, execution.LeaseClaim{
			WorkspaceID: workspaceID, ProjectID: value.projectID, JobID: value.jobID,
			PipelineRunID: value.runID, JobStepID: value.stepID, PipelineNodeID: value.nodeID,
			NodeKey: value.nodeKey,
		})
		if errors.Is(err, execution.ErrLeaseConflict) {
			continue
		}
		if err != nil {
			return reconciled, err
		}
		reconciled++
	}
	return reconciled, nil
}

func leasePayload(attempt int) json.RawMessage {
	value, _ := json.Marshal(map[string]any{"attempt": attempt, "failure_category": "worker_lost"})
	return value
}
