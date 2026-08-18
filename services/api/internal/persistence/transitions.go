package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
)

var ErrTransitionConflict = errors.New("execution transition conflict")

type TransitionInput struct {
	Entity         execution.Entity
	WorkspaceID    string
	ProjectID      string
	JobID          string
	PipelineRunID  string
	JobStepID      string
	PipelineNodeID string
	NodeKey        string
	FromStatus     string
	ToStatus       string
	SchemaVersion  string
	CorrelationID  string
	Payload        json.RawMessage
}

type TransitionResult struct {
	Entity     execution.Entity
	ID         string
	Status     string
	EventType  string
	Sequence   int64
	OccurredAt string
}

// Transition updates one execution row and appends its durable event and
// outbox record in one database transaction. The conditional status predicate
// makes a competing worker lose with ErrTransitionConflict instead of
// overwriting a newer state.
func (s *SQLStore) Transition(ctx context.Context, input TransitionInput) (TransitionResult, error) {
	if s == nil || s.DB == nil {
		return TransitionResult{}, errors.New("database is required")
	}
	if input.WorkspaceID == "" || input.ProjectID == "" || input.JobID == "" {
		return TransitionResult{}, errors.New("transition scope is incomplete")
	}
	if err := execution.ValidateTransition(input.Entity, input.FromStatus, input.ToStatus); err != nil {
		return TransitionResult{}, err
	}
	if input.SchemaVersion == "" {
		input.SchemaVersion = "1.0"
	}
	if input.CorrelationID == "" {
		input.CorrelationID = input.JobID
	}
	input.Payload = transitionPayload(input.Payload, input.FromStatus, input.ToStatus)
	eventType := execution.EventType(input.Entity, input.FromStatus, input.ToStatus)
	if eventType == "" {
		return TransitionResult{}, fmt.Errorf("no event type for %s %s -> %s", input.Entity, input.FromStatus, input.ToStatus)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return TransitionResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var id, status string
	switch input.Entity {
	case execution.EntityJob:
		id, status, err = transitionJob(ctx, tx, input)
	case execution.EntityRun:
		if input.PipelineRunID == "" {
			return TransitionResult{}, errors.New("pipeline run id is required")
		}
		id, status, err = transitionRun(ctx, tx, input)
	case execution.EntityStep:
		if input.JobStepID == "" || input.PipelineRunID == "" {
			return TransitionResult{}, errors.New("job step and pipeline run ids are required")
		}
		id, status, err = transitionStep(ctx, tx, input)
	default:
		return TransitionResult{}, errors.New("unknown transition entity")
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TransitionResult{}, ErrTransitionConflict
		}
		return TransitionResult{}, err
	}

	sequence, err := appendEventAndOutboxTx(ctx, tx, EventInput{
		SchemaVersion: input.SchemaVersion, EventType: eventType, CorrelationID: input.CorrelationID,
		WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID,
		PipelineRunID: input.PipelineRunID, JobStepID: input.JobStepID, PipelineNodeID: input.PipelineNodeID,
		NodeKey: input.NodeKey, Payload: input.Payload,
	})
	if err != nil {
		return TransitionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return TransitionResult{}, err
	}
	return TransitionResult{Entity: input.Entity, ID: id, Status: status, EventType: eventType, Sequence: sequence}, nil
}

func transitionPayload(raw json.RawMessage, from, to string) json.RawMessage {
	value := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &value); err != nil || value == nil {
			value = map[string]any{}
		}
	}
	value["from_status"] = from
	value["to_status"] = to
	encoded, _ := json.Marshal(value)
	return encoded
}

func transitionJob(ctx context.Context, tx *sql.Tx, input TransitionInput) (string, string, error) {
	query := `UPDATE jobs j SET status=$4, queued_at=CASE WHEN $4='queued' THEN COALESCE(j.queued_at,now()) ELSE j.queued_at END, started_at=CASE WHEN $4='running' THEN COALESCE(j.started_at,now()) ELSE j.started_at END, completed_at=CASE WHEN $4 IN ('completed','failed','dead_lettered','cancelled') THEN COALESCE(j.completed_at,now()) ELSE j.completed_at END, cancel_requested_at=CASE WHEN $4='cancelling' THEN COALESCE(j.cancel_requested_at,now()) ELSE j.cancel_requested_at END, updated_at=now() FROM projects p WHERE j.project_id=p.id AND p.workspace_id=$1::uuid AND p.id=$2::uuid AND j.id=$3::uuid AND j.status=$5 RETURNING j.id::text,j.status`
	var id, status string
	err := tx.QueryRowContext(ctx, query, input.WorkspaceID, input.ProjectID, input.JobID, input.ToStatus, input.FromStatus).Scan(&id, &status)
	return id, status, err
}

func transitionRun(ctx context.Context, tx *sql.Tx, input TransitionInput) (string, string, error) {
	query := `UPDATE pipeline_runs r SET status=$5, queued_at=CASE WHEN $5='queued' THEN COALESCE(r.queued_at,now()) ELSE r.queued_at END, started_at=CASE WHEN $5='running' THEN COALESCE(r.started_at,now()) ELSE r.started_at END, completed_at=CASE WHEN $5 IN ('completed','failed','cancelled') THEN COALESCE(r.completed_at,now()) ELSE r.completed_at END, updated_at=now() FROM jobs j JOIN projects p ON p.id=j.project_id WHERE r.job_id=j.id AND r.id=$1::uuid AND r.job_id=$2::uuid AND j.project_id=$3::uuid AND p.workspace_id=$4::uuid AND r.status=$6 RETURNING r.id::text,r.status`
	var id, status string
	err := tx.QueryRowContext(ctx, query, input.PipelineRunID, input.JobID, input.ProjectID, input.WorkspaceID, input.ToStatus, input.FromStatus).Scan(&id, &status)
	return id, status, err
}

func transitionStep(ctx context.Context, tx *sql.Tx, input TransitionInput) (string, string, error) {
	query := `UPDATE job_steps s SET status=$6, queued_at=CASE WHEN $6='queued' THEN COALESCE(s.queued_at,now()) ELSE s.queued_at END, started_at=CASE WHEN $6='running' THEN COALESCE(s.started_at,now()) ELSE s.started_at END, completed_at=CASE WHEN $6 IN ('completed','skipped','failed','cancelled','blocked') THEN COALESCE(s.completed_at,now()) ELSE s.completed_at END, heartbeat_at=CASE WHEN $6='running' THEN now() ELSE s.heartbeat_at END, updated_at=now() FROM pipeline_runs r JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE s.pipeline_run_id=r.id AND s.id=$1::uuid AND s.pipeline_run_id=$2::uuid AND r.job_id=$3::uuid AND j.project_id=$4::uuid AND p.workspace_id=$5::uuid AND s.status=$7 RETURNING s.id::text,s.status`
	var id, status string
	err := tx.QueryRowContext(ctx, query, input.JobStepID, input.PipelineRunID, input.JobID, input.ProjectID, input.WorkspaceID, input.ToStatus, input.FromStatus).Scan(&id, &status)
	return id, status, err
}

func appendEventAndOutboxTx(ctx context.Context, tx *sql.Tx, input EventInput) (int64, error) {
	if len(input.Payload) == 0 {
		input.Payload = []byte(`{}`)
	}
	if input.JobID == "" || input.WorkspaceID == "" || input.ProjectID == "" {
		return 0, errors.New("event scope is incomplete")
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, input.JobID); err != nil {
		return 0, err
	}
	var seq int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence),0)+1 FROM job_events WHERE job_id=$1::uuid`, input.JobID).Scan(&seq); err != nil {
		return 0, err
	}
	var eventID string
	if err := tx.QueryRowContext(ctx, `INSERT INTO job_events(schema_version,workspace_id,project_id,job_id,pipeline_run_id,job_step_id,pipeline_node_id,node_key,sequence,event_type,payload,correlation_id,occurred_at) VALUES($1,$2::uuid,$3::uuid,$4::uuid,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,NULLIF($7,'')::uuid,NULLIF($8,''),$9,$10,$11,$12,now()) RETURNING event_id::text`, input.SchemaVersion, input.WorkspaceID, input.ProjectID, input.JobID, input.PipelineRunID, input.JobStepID, input.PipelineNodeID, input.NodeKey, seq, input.EventType, input.Payload, input.CorrelationID).Scan(&eventID); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO outbox_events(id,aggregate_type,aggregate_id,event_type,payload,status) VALUES($1::uuid,'job',$2::uuid,$3,$4,'pending')`, eventID, input.JobID, input.EventType, input.Payload); err != nil {
		return 0, err
	}
	return seq, nil
}

// EventInput is retained for T301 and other non-transition event producers.
// It is deliberately scoped to a Job aggregate: Redis delivery is never the
// source of durable state.
type EventInput struct {
	SchemaVersion, EventType, CorrelationID                                          string
	WorkspaceID, ProjectID, JobID, PipelineRunID, JobStepID, PipelineNodeID, NodeKey string
	Payload                                                                          json.RawMessage
}

// AppendEventAndOutbox appends a standalone event atomically. State-changing
// callers should use Transition so the state update shares this transaction.
func (s *SQLStore) AppendEventAndOutbox(ctx context.Context, input EventInput) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	sequence, err := appendEventAndOutboxTx(ctx, tx, input)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return sequence, nil
}

func ensureUUIDScope(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("uuid scope value is required")
	}
	return nil
}
