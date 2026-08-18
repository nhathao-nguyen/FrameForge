package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
)

func (s *SQLStore) ClaimOutbox(ctx context.Context, limit int) ([]execution.OutboxRecord, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("database is required")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT id::text,aggregate_type,aggregate_id::text,event_type,payload,attempts,available_at FROM outbox_events WHERE status='pending' AND available_at <= now() ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]execution.OutboxRecord, 0, limit)
	for rows.Next() {
		var value execution.OutboxRecord
		if err := rows.Scan(&value.ID, &value.AggregateType, &value.AggregateID, &value.EventType, &value.Payload, &value.Attempts, &value.AvailableAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, value := range result {
		if _, err := tx.ExecContext(ctx, `UPDATE outbox_events SET attempts=attempts+1,updated_at=now() WHERE id=$1::uuid AND status='pending'`, value.ID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *SQLStore) MarkOutboxPublished(ctx context.Context, id string) error {
	if s == nil || s.DB == nil {
		return errors.New("database is required")
	}
	if id == "" {
		return errors.New("outbox id is required")
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE outbox_events SET status='published',published_at=COALESCE(published_at,now()),updated_at=now(),last_error=NULL WHERE id=$1::uuid`, id)
	return err
}

func (s *SQLStore) MarkOutboxFailed(ctx context.Context, id, message string, availableAt time.Time) error {
	if s == nil || s.DB == nil {
		return errors.New("database is required")
	}
	if id == "" || availableAt.IsZero() {
		return errors.New("outbox retry metadata is incomplete")
	}
	if len(message) > 240 {
		message = message[:240]
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE outbox_events SET status='pending',available_at=$2,last_error=$3,updated_at=now() WHERE id=$1::uuid`, id, availableAt.UTC(), message)
	return err
}

func (s *SQLStore) ReplayJobEvents(ctx context.Context, workspaceID, projectID, jobID string, afterSequence int64, limit int) ([]execution.EventRecord, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("database is required")
	}
	if workspaceID == "" || projectID == "" || jobID == "" {
		return nil, ErrScopeDenied
	}
	if afterSequence < 0 {
		afterSequence = 0
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT e.event_id::text,e.schema_version,e.event_type,e.sequence,e.occurred_at,e.workspace_id::text,e.project_id::text,e.job_id::text,COALESCE(e.pipeline_run_id::text,''),COALESCE(e.job_step_id::text,''),COALESCE(e.pipeline_node_id::text,''),COALESCE(e.node_key,''),e.correlation_id,e.payload FROM job_events e JOIN jobs j ON j.id=e.job_id JOIN projects p ON p.id=j.project_id WHERE e.workspace_id=$1::uuid AND e.project_id=$2::uuid AND e.job_id=$3::uuid AND e.sequence>$4 ORDER BY e.sequence ASC LIMIT $5`, workspaceID, projectID, jobID, afterSequence, limit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []execution.EventRecord{}, nil
		}
		return nil, err
	}
	defer rows.Close()
	result := make([]execution.EventRecord, 0, limit)
	for rows.Next() {
		var value execution.EventRecord
		if err := rows.Scan(&value.ID, &value.SchemaVersion, &value.EventType, &value.Sequence, &value.OccurredAt, &value.WorkspaceID, &value.ProjectID, &value.JobID, &value.PipelineRunID, &value.JobStepID, &value.PipelineNodeID, &value.NodeKey, &value.CorrelationID, &value.Payload); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func eventPayload(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	return value
}
