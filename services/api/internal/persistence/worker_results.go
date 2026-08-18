package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/queue"
)

type ScopedClaimResolver struct {
	SQL       *SQLStore
	Workspace string
	WorkerID  string
	Duration  time.Duration
}

func (r ScopedClaimResolver) ClaimCommand(ctx context.Context, message queue.Message) (worker.Command, execution.Lease, error) {
	if r.SQL == nil || r.Workspace == "" || r.WorkerID == "" {
		return worker.Command{}, execution.Lease{}, errors.New("scoped claim resolver is incomplete")
	}
	claimed, err := r.SQL.ClaimWorkerCommand(ctx, r.Workspace, message, r.WorkerID, r.Duration)
	if err != nil {
		return worker.Command{}, execution.Lease{}, err
	}
	return claimed.Command, claimed.Lease, nil
}

type ScopedResultApplier struct {
	SQL       *SQLStore
	Workspace string
	WorkerID  string
	Artifacts WorkerArtifactCommitter
}

func (r ScopedResultApplier) ApplyClaimedResult(ctx context.Context, value execution.ClaimedResult) error {
	if r.SQL == nil {
		return errors.New("scoped result applier requires database")
	}
	result := value.Delivery.Result
	if result.Status == "completed" && r.Artifacts.Storage != nil && r.Artifacts.Repository != nil {
		var projectID string
		if err := r.SQL.DB.QueryRowContext(ctx, `SELECT j.project_id::text FROM jobs j JOIN projects p ON p.id=j.project_id WHERE p.workspace_id=$1::uuid AND j.id=$2::uuid`, r.Workspace, value.Claim.Message.JobID).Scan(&projectID); err != nil {
			return err
		}
		if value.Claim.Message.NodeKey == "asset_probe" {
			if err := r.Artifacts.FinalizeValidatedUpload(ctx, r.Workspace, projectID, value.Claim.Message.JobID, value.Claim.Command, result); err != nil {
				return err
			}
		}
		for index, ref := range result.OutputRefs {
			committed, err := r.Artifacts.CommitWorkerArtifact(ctx, r.Workspace, projectID, value.Claim.Message.JobID, value.Claim.Message.JobStepID, result.MessageID, ref)
			if err != nil {
				return err
			}
			result.OutputRefs[index] = committed
			if value.Claim.Message.NodeKey == "render_timeline" {
				if err := r.linkRenderArtifact(ctx, value.Claim.Message.JobID, committed); err != nil {
					return err
				}
			}
		}
	}
	return r.SQL.ApplyWorkerResult(ctx, WorkerResultInput{
		WorkspaceID:    r.Workspace,
		ProjectID:      "",
		JobID:          value.Claim.Message.JobID,
		PipelineRunID:  value.Claim.Message.PipelineRunID,
		JobStepID:      value.Claim.Message.JobStepID,
		PipelineNodeID: value.Claim.Message.PipelineNodeID,
		NodeKey:        value.Claim.Message.NodeKey,
		WorkerID:       r.WorkerID,
		LeaseToken:     value.Claim.Lease.Token,
		Attempt:        value.Claim.Lease.Attempt,
		Result:         result,
	})
}

func (r ScopedResultApplier) linkRenderArtifact(ctx context.Context, jobID string, ref worker.OutputRef) error {
	role := map[string]string{"render_video": "video", "render_audio": "audio", "render_metadata": "metadata"}[ref.Role]
	if role == "" {
		return nil
	}
	artifactID, ok := contractUUID("artifact", ref.ArtifactID)
	if !ok {
		return errors.New("render output Artifact ref is invalid")
	}
	_, err := r.SQL.DB.ExecContext(ctx, `INSERT INTO render_artifacts(render_id,artifact_id,role,is_canonical) SELECT r.id,$2::uuid,$3,true FROM renders r JOIN jobs j ON j.id=r.job_id WHERE j.id=$1::uuid ON CONFLICT (render_id,artifact_id,role) DO UPDATE SET is_canonical=EXCLUDED.is_canonical`, jobID, artifactID, role)
	return err
}

type ClaimedWorkerCommand struct {
	Command worker.Command
	Lease   execution.Lease
}

// ClaimWorkerCommand is the trusted controller boundary. It resolves the
// immutable command before claiming the current queued attempt, then returns
// the raw lease token only to the controller memory that launched the executor.
func (s *SQLStore) ClaimWorkerCommand(ctx context.Context, workspaceID string, message queue.Message, workerID string, duration time.Duration) (ClaimedWorkerCommand, error) {
	if s == nil || s.DB == nil {
		return ClaimedWorkerCommand{}, errors.New("database is required")
	}
	if workerID == "" || message.Attempt < 1 {
		return ClaimedWorkerCommand{}, errors.New("worker claim is incomplete")
	}
	command, err := s.ResolveWorkerCommand(ctx, workspaceID, message)
	if err != nil {
		return ClaimedWorkerCommand{}, err
	}
	var projectID, pipelineNodeID, nodeKey string
	if err := s.DB.QueryRowContext(ctx, `SELECT j.project_id::text,s.pipeline_node_id::text,s.node_key FROM job_steps s JOIN pipeline_runs r ON r.id=s.pipeline_run_id JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE p.workspace_id=$1::uuid AND j.id=$2::uuid AND r.id=$3::uuid AND s.id=$4::uuid AND s.status='queued' AND s.current_attempt=$5`, workspaceID, message.JobID, message.PipelineRunID, message.JobStepID, message.Attempt-1).Scan(&projectID, &pipelineNodeID, &nodeKey); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ClaimedWorkerCommand{}, execution.ErrLeaseConflict
		}
		return ClaimedWorkerCommand{}, err
	}
	lease, err := s.ClaimStep(ctx, execution.LeaseClaim{WorkspaceID: workspaceID, ProjectID: projectID, JobID: message.JobID, PipelineRunID: message.PipelineRunID, JobStepID: message.JobStepID, PipelineNodeID: pipelineNodeID, NodeKey: nodeKey, WorkerID: workerID, ExpectedAttempt: message.Attempt - 1, Duration: duration})
	if err != nil {
		return ClaimedWorkerCommand{}, err
	}
	return ClaimedWorkerCommand{Command: command, Lease: lease.Lease}, nil
}

type WorkerResultInput struct {
	WorkspaceID    string
	ProjectID      string
	JobID          string
	PipelineRunID  string
	JobStepID      string
	PipelineNodeID string
	NodeKey        string
	WorkerID       string
	LeaseToken     string
	Attempt        int
	Result         worker.Result
}

// ApplyWorkerResult validates the result, closes the matching leased attempt,
// transitions the JobStep and appends the canonical event in one transaction.
// Artifact bytes/metadata must already be committed through the ArtifactPort;
// this method stores only safe output references and never accepts a local path.
func (s *SQLStore) ApplyWorkerResult(ctx context.Context, input WorkerResultInput) error {
	if s == nil || s.DB == nil {
		return errors.New("database is required")
	}
	if input.WorkspaceID == "" || input.JobID == "" || input.PipelineRunID == "" || input.JobStepID == "" || input.WorkerID == "" || input.LeaseToken == "" || input.Attempt < 1 {
		return errors.New("worker result scope is incomplete")
	}
	if err := worker.ValidateResult(input.Result); err != nil {
		return err
	}
	if input.Result.JobID == "" || input.Result.JobStepID == "" {
		return errors.New("worker result identity is incomplete")
	}
	outputRefs, err := json.Marshal(input.Result.OutputRefs)
	if err != nil {
		return err
	}
	status := input.Result.Status
	attemptStatus := status
	if status == "skipped" {
		attemptStatus = "completed"
	}
	if status != "completed" && status != "failed" && status != "cancelled" && status != "skipped" {
		return errors.New("worker result status cannot be applied")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if input.ProjectID == "" {
		if err := tx.QueryRowContext(ctx, `SELECT j.project_id::text FROM pipeline_runs r JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE p.workspace_id=$1::uuid AND j.id=$2::uuid AND r.id=$3::uuid AND EXISTS (SELECT 1 FROM job_steps s WHERE s.id=$4::uuid AND s.pipeline_run_id=r.id)`, input.WorkspaceID, input.JobID, input.PipelineRunID, input.JobStepID).Scan(&input.ProjectID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return execution.ErrLeaseConflict
			}
			return err
		}
	}
	var maxAttempts, currentAttempt int
	var jobStatus, runStatus, failureMode string
	if err := tx.QueryRowContext(ctx, `SELECT n.max_attempts,s.current_attempt,j.status,r.status,n.failure_mode FROM job_steps s JOIN pipeline_nodes n ON n.id=s.pipeline_node_id JOIN pipeline_runs r ON r.id=s.pipeline_run_id JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE p.workspace_id=$1::uuid AND j.project_id=$2::uuid AND j.id=$3::uuid AND r.id=$4::uuid AND s.id=$5::uuid`, input.WorkspaceID, input.ProjectID, input.JobID, input.PipelineRunID, input.JobStepID).Scan(&maxAttempts, &currentAttempt, &jobStatus, &runStatus, &failureMode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return execution.ErrLeaseConflict
		}
		return err
	}
	if currentAttempt != input.Attempt {
		return execution.ErrLeaseConflict
	}
	if (jobStatus == "cancelling" || jobStatus == "cancelled" || runStatus == "cancelled") && status != "cancelled" {
		return execution.ErrLeaseConflict
	}
	failureCategory, errorCode, errorMessage := "", "", ""
	retryable := false
	if input.Result.SafeError != nil {
		failureCategory = input.Result.SafeError.Category
		errorCode = input.Result.SafeError.Code
		errorMessage = input.Result.SafeError.SafeMessage
		retryable = retryableWorkerFailure(input.Result.SafeError) && input.Attempt < maxAttempts
	}
	targetStatus := status
	if status == "failed" && retryable {
		targetStatus = "retrying"
	}
	if err := execution.ValidateTransition(execution.EntityStep, "running", targetStatus); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE job_step_attempts a SET status=$7,output_refs=$8,failure_category=NULLIF($9,''),error_code=NULLIF($10,''),error_message=NULLIF($11,''),retryable=$12,completed_at=now(),updated_at=now() FROM job_steps s JOIN pipeline_runs r ON r.id=s.pipeline_run_id JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE a.job_step_id=s.id AND a.attempt=$1 AND a.status='running' AND a.worker_id=$2 AND a.lease_token_hash=$3 AND a.lease_expires_at > now() AND s.id=$4::uuid AND s.pipeline_run_id=$5::uuid AND r.job_id=$6::uuid AND j.project_id=$13::uuid AND p.workspace_id=$14::uuid`, input.Attempt, input.WorkerID, execution.HashLeaseToken(input.LeaseToken), input.JobStepID, input.PipelineRunID, input.JobID, attemptStatus, outputRefs, failureCategory, errorCode, errorMessage, retryable, input.ProjectID, input.WorkspaceID)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return execution.ErrLeaseConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE job_steps SET output_refs=$2,updated_at=now() WHERE id=$1::uuid AND current_attempt=$3`, input.JobStepID, outputRefs, input.Attempt); err != nil {
		return err
	}
	if _, _, err := transitionStep(ctx, tx, TransitionInput{Entity: execution.EntityStep, WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID, PipelineRunID: input.PipelineRunID, JobStepID: input.JobStepID, PipelineNodeID: input.PipelineNodeID, NodeKey: input.NodeKey, FromStatus: "running", ToStatus: targetStatus, CorrelationID: input.JobID, Payload: workerResultPayload(input.Result)}); err != nil {
		return fmt.Errorf("transition worker result: %w", err)
	}
	if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: execution.EventType(execution.EntityStep, "running", targetStatus), CorrelationID: input.JobID, WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID, PipelineRunID: input.PipelineRunID, JobStepID: input.JobStepID, PipelineNodeID: input.PipelineNodeID, NodeKey: input.NodeKey, Payload: workerResultPayload(input.Result)}); err != nil {
		return err
	}
	if targetStatus == "retrying" && jobStatus == "running" {
		if _, _, err := transitionJob(ctx, tx, TransitionInput{Entity: execution.EntityJob, WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID, PipelineRunID: input.PipelineRunID, FromStatus: "running", ToStatus: "retrying", CorrelationID: input.JobID, Payload: json.RawMessage(`{"reason":"worker_retryable_failure"}`)}); err != nil {
			return err
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "job.retry_scheduled", CorrelationID: input.JobID, WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID, PipelineRunID: input.PipelineRunID, Payload: json.RawMessage(`{"reason":"worker_retryable_failure"}`)}); err != nil {
			return err
		}
	}
	if targetStatus == "failed" {
		if failureMode != "soft" {
			if err := terminateRemainingStepsTx(ctx, tx, input, "hard_dependency_failed"); err != nil {
				return err
			}
			exhaustedRetry := input.Result.SafeError != nil && retryableWorkerFailure(input.Result.SafeError) && input.Attempt >= maxAttempts
			if err := failAggregateTx(ctx, tx, input.WorkspaceID, input.ProjectID, input.JobID, input.PipelineRunID, exhaustedRetry); err != nil {
				return err
			}
		}
	}
	if targetStatus == "cancelled" {
		if err := cancelAggregateTx(ctx, tx, input.WorkspaceID, input.ProjectID, input.JobID, input.PipelineRunID); err != nil {
			return err
		}
	}
	if targetStatus == "completed" || targetStatus == "skipped" {
		if err := completeAggregateTx(ctx, tx, input.WorkspaceID, input.ProjectID, input.JobID, input.PipelineRunID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func retryableWorkerFailure(value *worker.SafeError) bool {
	if value == nil || !value.Retryable {
		return false
	}
	switch value.Category {
	case "transient", "internal":
		return true
	default:
		return false
	}
}

func terminateRemainingStepsTx(ctx context.Context, tx *sql.Tx, input WorkerResultInput, reason string) error {
	rows, err := tx.QueryContext(ctx, `SELECT s.id::text,s.pipeline_node_id::text,s.node_key,s.status,s.current_attempt
		FROM job_steps s WHERE s.pipeline_run_id=$1::uuid
		  AND s.status IN ('pending','ready','queued','running','paused','waiting_for_review','retrying')
		ORDER BY s.node_key FOR UPDATE OF s`, input.PipelineRunID)
	if err != nil {
		return err
	}
	type activeStep struct {
		id, nodeID, nodeKey, status string
		attempt                     int
	}
	values := make([]activeStep, 0)
	for rows.Next() {
		var value activeStep
		if err := rows.Scan(&value.id, &value.nodeID, &value.nodeKey, &value.status, &value.attempt); err != nil {
			_ = rows.Close()
			return err
		}
		values = append(values, value)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"reason": reason, "failed_node_key": input.NodeKey})
	for _, value := range values {
		target := "cancelled"
		if value.status == "pending" || value.status == "ready" {
			target = "blocked"
		}
		if value.status == "running" && value.attempt > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE job_step_attempts SET status='cancelled',error_code='upstream_failed',error_message='An upstream hard dependency failed.',retryable=false,completed_at=now(),updated_at=now() WHERE job_step_id=$1::uuid AND attempt=$2 AND status='running'`, value.id, value.attempt); err != nil {
				return err
			}
		}
		if _, _, err := transitionStep(ctx, tx, TransitionInput{Entity: execution.EntityStep, WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID, PipelineRunID: input.PipelineRunID, JobStepID: value.id, PipelineNodeID: value.nodeID, NodeKey: value.nodeKey, FromStatus: value.status, ToStatus: target, CorrelationID: input.JobID, Payload: payload}); err != nil {
			return err
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: execution.EventType(execution.EntityStep, value.status, target), CorrelationID: input.JobID, WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID, PipelineRunID: input.PipelineRunID, JobStepID: value.id, PipelineNodeID: value.nodeID, NodeKey: value.nodeKey, Payload: payload}); err != nil {
			return err
		}
	}
	return nil
}

func failAggregateTx(ctx context.Context, tx *sql.Tx, workspaceID, projectID, jobID, runID string, exhaustedRetry bool) error {
	var failed, active int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE status IN ('failed','cancelled')),count(*) FILTER (WHERE status IN ('pending','ready','queued','running','paused','waiting_for_review','retrying')) FROM job_steps WHERE pipeline_run_id=$1::uuid`, runID).Scan(&failed, &active); err != nil {
		return err
	}
	if failed == 0 || active != 0 {
		return nil
	}
	var runStatus, jobStatus string
	var command json.RawMessage
	if err := tx.QueryRowContext(ctx, `SELECT r.status,j.status,j.command FROM pipeline_runs r JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE p.workspace_id=$1::uuid AND j.project_id=$2::uuid AND j.id=$3::uuid AND r.id=$4::uuid`, workspaceID, projectID, jobID, runID).Scan(&runStatus, &jobStatus, &command); err != nil {
		return err
	}
	if runStatus == "running" {
		if _, _, err := transitionRun(ctx, tx, TransitionInput{Entity: execution.EntityRun, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: "running", ToStatus: "failed", CorrelationID: jobID, Payload: json.RawMessage(`{"reason":"step_failed"}`)}); err != nil {
			return err
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "pipeline_run.failed", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: json.RawMessage(`{"reason":"step_failed"}`)}); err != nil {
			return err
		}
	}
	if jobStatus == "running" || jobStatus == "retrying" {
		to := "failed"
		if commandEnablesDLQ(command) && (jobStatus == "retrying" || exhaustedRetry) {
			to = "dead_lettered"
		}
		if _, _, err := transitionJob(ctx, tx, TransitionInput{Entity: execution.EntityJob, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: jobStatus, ToStatus: to, CorrelationID: jobID, Payload: json.RawMessage(`{"reason":"step_failed"}`)}); err != nil {
			return err
		}
		eventType := "job.failed"
		payload := json.RawMessage(`{"reason":"step_failed"}`)
		if to == "dead_lettered" {
			var deadLetterID string
			if err := tx.QueryRowContext(ctx, `INSERT INTO dead_letters(job_id,pipeline_run_id,command_snapshot,safe_error) VALUES($1::uuid,$2::uuid,$3,'{"code":"retry_exhausted","category":"permanent","retryable":false,"safe_message":"Retry budget was exhausted."}'::jsonb) RETURNING id::text`, jobID, runID, command).Scan(&deadLetterID); err != nil {
				return err
			}
			eventType = "job.dead_lettered"
			payload, _ = json.Marshal(map[string]any{"dead_letter_id": deadLetterID, "safe_error": map[string]any{"code": "retry_exhausted", "category": "permanent", "retryable": false, "safe_message": "Retry budget was exhausted."}})
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: eventType, CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: payload}); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE renders SET status='failed',error_code='render_job_failed',error_message='The render Job failed.',updated_at=now() WHERE job_id=$1::uuid AND status NOT IN ('completed','cancelled')`, jobID); err != nil {
			return err
		}
	}
	return nil
}

func commandEnablesDLQ(command json.RawMessage) bool {
	var value map[string]any
	if json.Unmarshal(command, &value) != nil {
		return false
	}
	enabled, _ := value["enable_dlq"].(bool)
	return enabled
}

func cancelAggregateTx(ctx context.Context, tx *sql.Tx, workspaceID, projectID, jobID, runID string) error {
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM job_steps WHERE pipeline_run_id=$1::uuid AND status IN ('pending','ready','queued','running','paused','waiting_for_review','retrying')`, runID).Scan(&active); err != nil {
		return err
	}
	if active != 0 {
		return nil
	}
	var runStatus, jobStatus string
	if err := tx.QueryRowContext(ctx, `SELECT r.status,j.status FROM pipeline_runs r JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE p.workspace_id=$1::uuid AND j.project_id=$2::uuid AND j.id=$3::uuid AND r.id=$4::uuid`, workspaceID, projectID, jobID, runID).Scan(&runStatus, &jobStatus); err != nil {
		return err
	}
	if runStatus != "cancelled" && runStatus != "completed" && runStatus != "failed" {
		if _, _, err := transitionRun(ctx, tx, TransitionInput{Entity: execution.EntityRun, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: runStatus, ToStatus: "cancelled", CorrelationID: jobID}); err != nil {
			return err
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "pipeline_run.cancelled", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: json.RawMessage(`{"reason":"cancellation_confirmed"}`)}); err != nil {
			return err
		}
	}
	if jobStatus == "cancelling" {
		if _, _, err := transitionJob(ctx, tx, TransitionInput{Entity: execution.EntityJob, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: jobStatus, ToStatus: "cancelled", CorrelationID: jobID}); err != nil {
			return err
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "job.cancelled", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: json.RawMessage(`{"reason":"cancellation_confirmed"}`)}); err != nil {
			return err
		}
	}
	return nil
}

func completeAggregateTx(ctx context.Context, tx *sql.Tx, workspaceID, projectID, jobID, runID string) error {
	var total, terminal, active int
	if err := tx.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER (WHERE s.status IN ('completed','skipped') OR (s.status='failed' AND n.failure_mode='soft')),count(*) FILTER (WHERE s.status IN ('pending','ready','queued','running','paused','waiting_for_review','retrying')) FROM job_steps s JOIN pipeline_nodes n ON n.id=s.pipeline_node_id WHERE s.pipeline_run_id=$1::uuid`, runID).Scan(&total, &terminal, &active); err != nil {
		return err
	}
	var runStatus, jobStatus, stopAfter string
	if err := tx.QueryRowContext(ctx, `SELECT r.status,j.status FROM pipeline_runs r JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE p.workspace_id=$1::uuid AND j.project_id=$2::uuid AND j.id=$3::uuid AND r.id=$4::uuid`, workspaceID, projectID, jobID, runID).Scan(&runStatus, &jobStatus); err != nil {
		return err
	}
	stopAfterPaused := false
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(stop_after,'') FROM jobs WHERE id=$1::uuid`, jobID).Scan(&stopAfter); err != nil {
		return err
	}
	if stopAfter != "" {
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM job_events WHERE job_id=$1::uuid AND event_type='job.paused' AND payload->>'reason'='stop_after')`, jobID).Scan(&stopAfterPaused); err != nil {
			return err
		}
	}
	if stopAfter != "" && !stopAfterPaused {
		var targetStatus string
		err := tx.QueryRowContext(ctx, `SELECT status FROM job_steps WHERE pipeline_run_id=$1::uuid AND node_key=$2`, runID, stopAfter).Scan(&targetStatus)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if targetStatus == "completed" || targetStatus == "skipped" {
			payload, _ := json.Marshal(map[string]any{"reason": "stop_after", "stop_after": stopAfter})
			if runStatus == "running" {
				if _, _, err := transitionRun(ctx, tx, TransitionInput{Entity: execution.EntityRun, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: "running", ToStatus: "paused", CorrelationID: jobID, Payload: payload}); err != nil {
					return err
				}
				if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "pipeline_run.paused", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: payload}); err != nil {
					return err
				}
			}
			if jobStatus == "running" {
				if _, _, err := transitionJob(ctx, tx, TransitionInput{Entity: execution.EntityJob, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: "running", ToStatus: "paused", CorrelationID: jobID, Payload: payload}); err != nil {
					return err
				}
				if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "job.paused", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: payload}); err != nil {
					return err
				}
			}
			return nil
		}
	}
	if total == 0 || terminal != total || active != 0 {
		return nil
	}
	if runStatus == "running" {
		if _, _, err := transitionRun(ctx, tx, TransitionInput{Entity: execution.EntityRun, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: "running", ToStatus: "completed", CorrelationID: jobID, Payload: json.RawMessage(`{"reason":"all_steps_terminal"}`)}); err != nil {
			return err
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "pipeline_run.completed", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: json.RawMessage(`{"reason":"all_steps_terminal"}`)}); err != nil {
			return err
		}
	}
	if jobStatus == "running" {
		if _, _, err := transitionJob(ctx, tx, TransitionInput{Entity: execution.EntityJob, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: "running", ToStatus: "completed", CorrelationID: jobID, Payload: json.RawMessage(`{"reason":"all_steps_terminal"}`)}); err != nil {
			return err
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "job.completed", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: json.RawMessage(`{"reason":"all_steps_terminal"}`)}); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE renders SET status='completed',completed_at=now(),updated_at=now() WHERE job_id=$1::uuid AND status IN ('created','queued','running')`, jobID); err != nil {
			return err
		}
	}
	return nil
}

func workerResultPayload(result worker.Result) json.RawMessage {
	value, _ := json.Marshal(map[string]any{"status": result.Status, "output_refs": result.OutputRefs, "checkpoint_ref": result.CheckpointRef, "safe_error": result.SafeError})
	return value
}
