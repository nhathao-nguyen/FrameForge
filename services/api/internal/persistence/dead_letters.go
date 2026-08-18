package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
)

type DeadLetterInput struct {
	WorkspaceID   string
	ProjectID     string
	JobID         string
	PipelineRunID string
	SafeError     json.RawMessage
}

// MarkDeadLettered makes the terminal Job state, canonical dead-letter row and
// job.dead_lettered event one commit. The old Job history remains immutable
// after this transition; replay is a separate new Job command.
func (s *SQLStore) MarkDeadLettered(ctx context.Context, input DeadLetterInput) error {
	if s == nil || s.DB == nil {
		return errors.New("database is required")
	}
	if input.WorkspaceID == "" || input.ProjectID == "" || input.JobID == "" || input.PipelineRunID == "" {
		return errors.New("dead-letter scope is incomplete")
	}
	if len(input.SafeError) == 0 {
		input.SafeError = []byte(`{"code":"retry_exhausted","category":"permanent","retryable":false,"safe_message":"Retry budget was exhausted."}`)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var command json.RawMessage
	if err := tx.QueryRowContext(ctx, `UPDATE jobs j SET status='dead_lettered',completed_at=COALESCE(j.completed_at,now()),updated_at=now() FROM projects p WHERE j.id=$1::uuid AND j.project_id=$2::uuid AND p.id=j.project_id AND p.workspace_id=$3::uuid AND j.status='retrying' RETURNING j.command`, input.JobID, input.ProjectID, input.WorkspaceID).Scan(&command); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return execution.ErrInvalidTransition
		}
		return err
	}
	var deadLetterID string
	if err := tx.QueryRowContext(ctx, `INSERT INTO dead_letters(job_id,pipeline_run_id,command_snapshot,safe_error) VALUES($1::uuid,$2::uuid,$3,$4) RETURNING id::text`, input.JobID, input.PipelineRunID, command, input.SafeError).Scan(&deadLetterID); err != nil {
		return err
	}
	if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "job.dead_lettered", CorrelationID: input.JobID, WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID, PipelineRunID: input.PipelineRunID, Payload: deadLetterPayload(deadLetterID, input.SafeError)}); err != nil {
		return err
	}
	return tx.Commit()
}

func deadLetterPayload(id string, safeError json.RawMessage) json.RawMessage {
	value := map[string]any{"dead_letter_id": id, "safe_error": json.RawMessage(safeError)}
	encoded, _ := json.Marshal(value)
	return encoded
}
