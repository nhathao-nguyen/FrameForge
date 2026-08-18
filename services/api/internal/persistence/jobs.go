package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/product"
)

func (s *SQLStore) GetJob(ctx context.Context, userID, workspaceID, projectID, jobID string) (JobRecord, error) {
	if _, err := s.GetProject(ctx, userID, workspaceID, projectID); err != nil {
		return JobRecord{}, err
	}
	var value JobRecord
	err := s.DB.QueryRowContext(ctx, `SELECT id::text,project_id::text,kind,status,command,created_at,COALESCE(current_pipeline_run_id::text,'') FROM jobs WHERE id=$1::uuid AND project_id=$2::uuid`, jobID, projectID).Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Status, &value.Command, &value.CreatedAt, &value.PipelineRunID)
	if errors.Is(err, sql.ErrNoRows) {
		return JobRecord{}, ErrNotFound
	}
	return value, err
}

func (b *DurableBackend) GetJob(workspaceID, projectID, jobID string) (*product.Job, error) {
	if workspaceID != b.Workspace {
		return nil, product.ErrNotFound
	}
	value, err := b.SQL.GetJob(context.Background(), b.UserID, b.Workspace, projectID, jobID)
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	return &product.Job{ID: value.ID, ProjectID: value.ProjectID, Kind: value.Kind, Status: domain.JobStatus(value.Status), PipelineRunID: value.PipelineRunID, Command: mapFromJSON(value.Command), CreatedAt: value.CreatedAt}, nil
}

func (b *DurableBackend) StartJob(workspaceID, projectID, jobID string) (*product.Job, error) {
	if workspaceID != b.Workspace {
		return nil, product.ErrNotFound
	}
	current, err := b.SQL.GetJob(context.Background(), b.UserID, b.Workspace, projectID, jobID)
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	if current.Status == string(domain.JobQueued) {
		if err := b.enqueueReadySteps(workspaceID, projectID, current); err != nil {
			return nil, err
		}
		return b.GetJob(workspaceID, projectID, jobID)
	}
	if err := execution.ValidateTransition(execution.EntityJob, current.Status, "queued"); err != nil {
		return nil, product.ErrConflict
	}
	if _, err := b.SQL.Transition(context.Background(), TransitionInput{Entity: execution.EntityJob, WorkspaceID: b.Workspace, ProjectID: projectID, JobID: jobID, PipelineRunID: current.PipelineRunID, FromStatus: current.Status, ToStatus: "queued", CorrelationID: jobID, Payload: json.RawMessage(`{"priority":5}`)}); err != nil {
		if errors.Is(err, ErrTransitionConflict) {
			return nil, product.ErrConflict
		}
		return nil, err
	}
	if _, err := b.SQL.Transition(context.Background(), TransitionInput{Entity: execution.EntityRun, WorkspaceID: b.Workspace, ProjectID: projectID, JobID: jobID, PipelineRunID: current.PipelineRunID, FromStatus: "created", ToStatus: "queued", CorrelationID: jobID}); err != nil {
		return nil, mapPersistenceError(err)
	}
	current.Status = string(domain.JobQueued)
	if err := b.enqueueReadySteps(workspaceID, projectID, current); err != nil {
		return nil, err
	}
	return b.GetJob(workspaceID, projectID, jobID)
}

func (b *DurableBackend) enqueueReadySteps(workspaceID, projectID string, current JobRecord) error {
	if b.Queue == nil {
		return nil
	}
	messages, err := b.SQL.QueueReadySteps(context.Background(), workspaceID, projectID, current.ID, current.PipelineRunID)
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		messages, err = b.SQL.QueueQueuedSteps(context.Background(), workspaceID, projectID, current.ID, current.PipelineRunID)
		if err != nil {
			return err
		}
	}
	for _, message := range messages {
		if _, err := b.Queue.Enqueue(context.Background(), message); err != nil {
			return err
		}
	}
	return nil
}

// RequeueRetryingSteps is the durable retry sweeper. PostgreSQL transitions a
// retrying step to queued before this adapter publishes its ID-only message;
// an API restart can therefore safely run the same sweep again.
func (b *DurableBackend) RequeueRetryingSteps(ctx context.Context) (int, error) {
	if b == nil || b.SQL == nil || b.Queue == nil {
		return 0, nil
	}
	messages, err := b.SQL.QueueRetryingSteps(ctx, b.Workspace)
	if err != nil {
		return 0, err
	}
	for _, message := range messages {
		if _, err := b.Queue.Enqueue(ctx, message); err != nil {
			return 0, err
		}
	}
	return len(messages), nil
}

// ScheduleReadySteps advances dependency-satisfied frontiers after worker
// results commit. PostgreSQL remains authoritative; Redis receives only the
// stable ID-only messages returned by the transactional scheduler.
func (b *DurableBackend) ScheduleReadySteps(ctx context.Context) (int, error) {
	if b == nil || b.SQL == nil || b.Queue == nil {
		return 0, nil
	}
	rows, err := b.SQL.DB.QueryContext(ctx, `SELECT j.project_id::text,j.id::text,r.id::text
		FROM jobs j
		JOIN projects p ON p.id=j.project_id
		JOIN pipeline_runs r ON r.id=j.current_pipeline_run_id
		WHERE p.workspace_id=$1::uuid AND j.status IN ('queued','running') AND r.status IN ('queued','running')
		ORDER BY j.created_at,j.id`, b.Workspace)
	if err != nil {
		return 0, err
	}
	type activeRun struct{ projectID, jobID, runID string }
	values := make([]activeRun, 0)
	for rows.Next() {
		var value activeRun
		if err := rows.Scan(&value.projectID, &value.jobID, &value.runID); err != nil {
			_ = rows.Close()
			return 0, err
		}
		values = append(values, value)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	count := 0
	for _, value := range values {
		messages, err := b.SQL.QueueReadySteps(ctx, b.Workspace, value.projectID, value.jobID, value.runID)
		if err != nil {
			return count, err
		}
		for _, message := range messages {
			if _, err := b.Queue.Enqueue(ctx, message); err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}

// Reconcile is the safe, repeatable recovery operation used after a
// dependency or worker outage. It only re-enqueues PostgreSQL-authorized
// ready/retrying frontiers; it never fabricates rows or trusts Redis as truth.
func (b *DurableBackend) Reconcile(ctx context.Context) (product.RecoveryReport, error) {
	ready, err := b.ScheduleReadySteps(ctx)
	if err != nil {
		return product.RecoveryReport{}, err
	}
	retrying, err := b.RequeueRetryingSteps(ctx)
	if err != nil {
		return product.RecoveryReport{ReadyStepsRequeued: ready}, err
	}
	return product.RecoveryReport{ReadyStepsRequeued: ready, RetryStepsRequeued: retrying}, nil
}

func (b *DurableBackend) PauseJob(workspaceID, projectID, jobID string) (*product.Job, error) {
	if err := b.controlJob(context.Background(), workspaceID, projectID, jobID, "pause"); err != nil {
		return nil, err
	}
	return b.GetJob(workspaceID, projectID, jobID)
}

func (b *DurableBackend) ResumeJob(workspaceID, projectID, jobID string) (*product.Job, error) {
	if err := b.controlJob(context.Background(), workspaceID, projectID, jobID, "resume"); err != nil {
		return nil, err
	}
	current, err := b.SQL.GetJob(context.Background(), b.UserID, b.Workspace, projectID, jobID)
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	if err := b.enqueueReadySteps(workspaceID, projectID, current); err != nil {
		return nil, err
	}
	return b.GetJob(workspaceID, projectID, jobID)
}

func (b *DurableBackend) CancelJob(workspaceID, projectID, jobID string) (*product.Job, error) {
	if err := b.controlJob(context.Background(), workspaceID, projectID, jobID, "cancel"); err != nil {
		return nil, err
	}
	return b.GetJob(workspaceID, projectID, jobID)
}

// controlJob keeps Job, PipelineRun and cancellable JobSteps in one
// PostgreSQL transaction. The previous implementation changed only the Job,
// which made a paused/cancelled aggregate disagree with its durable graph.
// A running cancellation intentionally remains `cancelling` until the active
// lease reports termination; queued/review/paused work can become terminal
// immediately because no process tree needs to be killed.
func (b *DurableBackend) controlJob(ctx context.Context, workspaceID, projectID, jobID, command string) error {
	if b == nil || b.SQL == nil || workspaceID != b.Workspace {
		return product.ErrNotFound
	}
	tx, err := b.SQL.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var jobStatus, runStatus, runID string
	if err := tx.QueryRowContext(ctx, `SELECT j.status,COALESCE(r.status,''),COALESCE(r.id::text,'') FROM jobs j JOIN projects p ON p.id=j.project_id LEFT JOIN pipeline_runs r ON r.id=j.current_pipeline_run_id WHERE p.workspace_id=$1::uuid AND j.project_id=$2::uuid AND j.id=$3::uuid`, workspaceID, projectID, jobID).Scan(&jobStatus, &runStatus, &runID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return product.ErrNotFound
		}
		return err
	}
	if command == "pause" {
		if err := execution.ValidateTransition(execution.EntityJob, jobStatus, "paused"); err != nil {
			return product.ErrConflict
		}
		if _, _, err := transitionJob(ctx, tx, TransitionInput{Entity: execution.EntityJob, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: jobStatus, ToStatus: "paused", CorrelationID: jobID}); err != nil {
			return mapPersistenceError(err)
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "job.paused", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: json.RawMessage(`{"reason":"safe_pause"}`)}); err != nil {
			return err
		}
		if runID != "" && runStatus == "running" {
			if _, _, err := transitionRun(ctx, tx, TransitionInput{Entity: execution.EntityRun, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: runStatus, ToStatus: "paused", CorrelationID: jobID}); err != nil {
				return err
			}
			if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "pipeline_run.paused", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: json.RawMessage(`{"reason":"safe_pause"}`)}); err != nil {
				return err
			}
		}
		if err := pauseRunningStepsTx(ctx, tx, workspaceID, projectID, jobID, runID); err != nil {
			return err
		}
	} else if command == "resume" {
		if jobStatus != "paused" {
			return product.ErrConflict
		}
		if _, _, err := transitionJob(ctx, tx, TransitionInput{Entity: execution.EntityJob, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: jobStatus, ToStatus: "queued", CorrelationID: jobID}); err != nil {
			return mapPersistenceError(err)
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "job.queued", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: json.RawMessage(`{"reason":"resume"}`)}); err != nil {
			return err
		}
		if runID != "" && runStatus == "paused" {
			if _, _, err := transitionRun(ctx, tx, TransitionInput{Entity: execution.EntityRun, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: runStatus, ToStatus: "queued", CorrelationID: jobID}); err != nil {
				return err
			}
			if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "pipeline_run.queued", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: json.RawMessage(`{"reason":"resume"}`)}); err != nil {
				return err
			}
		}
		if err := queuePausedStepsTx(ctx, tx, workspaceID, projectID, jobID, runID); err != nil {
			return err
		}
	} else if command == "cancel" {
		if jobStatus == "cancelled" || jobStatus == "dead_lettered" || jobStatus == "failed" || jobStatus == "completed" {
			return tx.Commit()
		}
		if jobStatus == "running" || jobStatus == "cancelling" {
			if jobStatus == "running" {
				if _, _, err := transitionJob(ctx, tx, TransitionInput{Entity: execution.EntityJob, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: jobStatus, ToStatus: "cancelling", CorrelationID: jobID}); err != nil {
					return mapPersistenceError(err)
				}
				if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "job.cancellation_requested", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: json.RawMessage(`{"reason":"client_request"}`)}); err != nil {
					return err
				}
			}
		} else {
			if err := cancelNonRunningTx(ctx, tx, workspaceID, projectID, jobID, runID, jobStatus, runStatus); err != nil {
				return err
			}
		}
	} else {
		return errors.New("unknown execution command")
	}
	return tx.Commit()
}

func pauseRunningStepsTx(ctx context.Context, tx *sql.Tx, workspaceID, projectID, jobID, runID string) error {
	rows, err := tx.QueryContext(ctx, `SELECT s.id::text,s.pipeline_node_id::text,s.node_key FROM job_steps s JOIN pipeline_runs r ON r.id=s.pipeline_run_id JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE p.workspace_id=$1::uuid AND j.project_id=$2::uuid AND j.id=$3::uuid AND r.id=$4::uuid AND s.status='running' FOR UPDATE`, workspaceID, projectID, jobID, runID)
	if err != nil {
		return err
	}
	steps := make([]struct{ id, nodeID, nodeKey string }, 0)
	for rows.Next() {
		var step struct{ id, nodeID, nodeKey string }
		if err := rows.Scan(&step.id, &step.nodeID, &step.nodeKey); err != nil {
			return err
		}
		steps = append(steps, step)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, step := range steps {
		if _, _, err := transitionStep(ctx, tx, TransitionInput{Entity: execution.EntityStep, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, JobStepID: step.id, PipelineNodeID: step.nodeID, NodeKey: step.nodeKey, FromStatus: "running", ToStatus: "paused", CorrelationID: jobID}); err != nil {
			return err
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "node.paused", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, JobStepID: step.id, PipelineNodeID: step.nodeID, NodeKey: step.nodeKey, Payload: json.RawMessage(`{"reason":"safe_pause"}`)}); err != nil {
			return err
		}
	}
	return nil
}

func queuePausedStepsTx(ctx context.Context, tx *sql.Tx, workspaceID, projectID, jobID, runID string) error {
	rows, err := tx.QueryContext(ctx, `SELECT s.id::text,s.pipeline_node_id::text,s.node_key,s.status FROM job_steps s JOIN pipeline_runs r ON r.id=s.pipeline_run_id JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE p.workspace_id=$1::uuid AND j.project_id=$2::uuid AND j.id=$3::uuid AND r.id=$4::uuid AND s.status='paused' ORDER BY s.node_key FOR UPDATE`, workspaceID, projectID, jobID, runID)
	if err != nil {
		return err
	}
	steps := make([]struct{ id, nodeID, nodeKey, status string }, 0)
	for rows.Next() {
		var step struct{ id, nodeID, nodeKey, status string }
		if err := rows.Scan(&step.id, &step.nodeID, &step.nodeKey, &step.status); err != nil {
			return err
		}
		steps = append(steps, step)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, step := range steps {
		if _, _, err := transitionStep(ctx, tx, TransitionInput{Entity: execution.EntityStep, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, JobStepID: step.id, PipelineNodeID: step.nodeID, NodeKey: step.nodeKey, FromStatus: step.status, ToStatus: "queued", CorrelationID: jobID}); err != nil {
			return err
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "node.queued", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, JobStepID: step.id, PipelineNodeID: step.nodeID, NodeKey: step.nodeKey, Payload: json.RawMessage(`{"reason":"resume"}`)}); err != nil {
			return err
		}
	}
	return nil
}

func cancelNonRunningTx(ctx context.Context, tx *sql.Tx, workspaceID, projectID, jobID, runID, jobStatus, runStatus string) error {
	if _, _, err := transitionJob(ctx, tx, TransitionInput{Entity: execution.EntityJob, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: jobStatus, ToStatus: "cancelled", CorrelationID: jobID}); err != nil {
		return mapPersistenceError(err)
	}
	if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "job.cancelled", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: json.RawMessage(`{"reason":"client_request"}`)}); err != nil {
		return err
	}
	if runID != "" && runStatus != "" && runStatus != "cancelled" {
		if _, _, err := transitionRun(ctx, tx, TransitionInput{Entity: execution.EntityRun, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, FromStatus: runStatus, ToStatus: "cancelled", CorrelationID: jobID}); err != nil {
			return err
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "pipeline_run.cancelled", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, Payload: json.RawMessage(`{"reason":"client_request"}`)}); err != nil {
			return err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT s.id::text,s.pipeline_node_id::text,s.node_key,s.status FROM job_steps s JOIN pipeline_runs r ON r.id=s.pipeline_run_id JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE p.workspace_id=$1::uuid AND j.project_id=$2::uuid AND j.id=$3::uuid AND r.id=$4::uuid AND s.status IN ('pending','ready','queued','paused','waiting_for_review','retrying') FOR UPDATE`, workspaceID, projectID, jobID, runID)
	if err != nil {
		return err
	}
	steps := make([]struct{ id, nodeID, nodeKey, status string }, 0)
	for rows.Next() {
		var step struct{ id, nodeID, nodeKey, status string }
		if err := rows.Scan(&step.id, &step.nodeID, &step.nodeKey, &step.status); err != nil {
			return err
		}
		steps = append(steps, step)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, step := range steps {
		if _, _, err := transitionStep(ctx, tx, TransitionInput{Entity: execution.EntityStep, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, JobStepID: step.id, PipelineNodeID: step.nodeID, NodeKey: step.nodeKey, FromStatus: step.status, ToStatus: "cancelled", CorrelationID: jobID}); err != nil {
			return err
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "node.cancelled", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, JobStepID: step.id, PipelineNodeID: step.nodeID, NodeKey: step.nodeKey, Payload: json.RawMessage(`{"reason":"client_request"}`)}); err != nil {
			return err
		}
	}
	return nil
}

func (b *DurableBackend) transitionJob(workspaceID, projectID, jobID, to string) (*product.Job, error) {
	if workspaceID != b.Workspace {
		return nil, product.ErrNotFound
	}
	current, err := b.SQL.GetJob(context.Background(), b.UserID, b.Workspace, projectID, jobID)
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	if current.Status == to {
		return b.GetJob(workspaceID, projectID, jobID)
	}
	if err := execution.ValidateTransition(execution.EntityJob, current.Status, to); err != nil {
		return nil, product.ErrConflict
	}
	if _, err := b.SQL.Transition(context.Background(), TransitionInput{Entity: execution.EntityJob, WorkspaceID: b.Workspace, ProjectID: projectID, JobID: jobID, PipelineRunID: current.PipelineRunID, FromStatus: current.Status, ToStatus: to, CorrelationID: jobID}); err != nil {
		return nil, mapPersistenceError(err)
	}
	return b.GetJob(workspaceID, projectID, jobID)
}

func (b *DurableBackend) ListJobEvents(workspaceID, projectID, jobID string, after int64, limit int) ([]product.JobEvent, error) {
	if workspaceID != b.Workspace {
		return nil, product.ErrNotFound
	}
	values, err := b.SQL.ReplayJobEvents(context.Background(), b.Workspace, projectID, jobID, after, limit)
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	result := make([]product.JobEvent, 0, len(values))
	for _, value := range values {
		result = append(result, product.JobEvent{ID: value.ID, EventType: value.EventType, Sequence: value.Sequence, Payload: mapFromJSON(value.Payload), OccurredAt: value.OccurredAt})
	}
	return result, nil
}
