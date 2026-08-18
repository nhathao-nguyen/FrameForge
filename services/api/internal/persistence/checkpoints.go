package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
)

type CheckpointInput struct {
	WorkspaceID       string
	ProjectID         string
	JobID             string
	PipelineRunID     string
	JobStepID         string
	PipelineNodeID    string
	NodeKey           string
	ContextArtifactID string
	StateHash         string
	SchemaVersion     string
	InputFingerprint  string
}

func (s *SQLStore) CommitCheckpoint(ctx context.Context, input CheckpointInput) (execution.Checkpoint, error) {
	if s == nil || s.DB == nil {
		return execution.Checkpoint{}, errors.New("database is required")
	}
	checkpoint := execution.Checkpoint{ID: "checkpoint_pending", PipelineRunID: input.PipelineRunID, JobStepID: input.JobStepID, CompletedNodeKey: input.NodeKey, ContextArtifactID: input.ContextArtifactID, StateHash: input.StateHash, SchemaVersion: input.SchemaVersion, InputFingerprint: input.InputFingerprint, Valid: true}
	if err := execution.ValidateCheckpoint(checkpoint); err != nil {
		return execution.Checkpoint{}, err
	}
	if input.WorkspaceID == "" || input.ProjectID == "" || input.JobID == "" || input.PipelineNodeID == "" {
		return execution.Checkpoint{}, errors.New("checkpoint scope is incomplete")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return execution.Checkpoint{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := tx.QueryRowContext(ctx, `INSERT INTO job_checkpoints(pipeline_run_id,job_step_id,completed_node_key,context_artifact_id,state_hash,schema_version,input_fingerprint,is_valid) SELECT $1::uuid,$2::uuid,$3,NULLIF($4,'')::uuid,$5,$6,$7,true FROM pipeline_runs r JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE r.id=$1::uuid AND r.job_id=$8::uuid AND j.project_id=$9::uuid AND p.workspace_id=$10::uuid RETURNING id::text`, input.PipelineRunID, input.JobStepID, input.NodeKey, input.ContextArtifactID, input.StateHash, input.SchemaVersion, input.InputFingerprint, input.JobID, input.ProjectID, input.WorkspaceID).Scan(&checkpoint.ID); err != nil {
		return execution.Checkpoint{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE job_steps SET checkpoint_id=$2::uuid,updated_at=now() WHERE id=$1::uuid AND pipeline_run_id=$3::uuid AND status IN ('completed','skipped')`, input.JobStepID, checkpoint.ID, input.PipelineRunID)
	if err != nil {
		return execution.Checkpoint{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return execution.Checkpoint{}, ErrTransitionConflict
	}
	if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "node.checkpointed", CorrelationID: input.JobID, WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, JobID: input.JobID, PipelineRunID: input.PipelineRunID, JobStepID: input.JobStepID, PipelineNodeID: input.PipelineNodeID, NodeKey: input.NodeKey, Payload: checkpointPayload(checkpoint)}); err != nil {
		return execution.Checkpoint{}, err
	}
	if err := tx.Commit(); err != nil {
		return execution.Checkpoint{}, err
	}
	return checkpoint, nil
}

func (s *SQLStore) FindReusableCheckpoint(ctx context.Context, workspaceID, projectID, jobID, pipelineRunID, nodeKey, schemaVersion, inputFingerprint string) (execution.Checkpoint, error) {
	var value execution.Checkpoint
	err := s.DB.QueryRowContext(ctx, `SELECT c.id::text,c.pipeline_run_id::text,c.job_step_id::text,c.completed_node_key,COALESCE(c.context_artifact_id::text,''),c.state_hash,c.schema_version,c.input_fingerprint,c.is_valid FROM job_checkpoints c JOIN pipeline_runs r ON r.id=c.pipeline_run_id JOIN jobs j ON j.id=r.job_id JOIN projects p ON p.id=j.project_id WHERE p.workspace_id=$1::uuid AND j.project_id=$2::uuid AND j.id=$3::uuid AND c.pipeline_run_id=$4::uuid AND c.completed_node_key=$5 AND c.schema_version=$6 AND c.input_fingerprint=$7 AND c.is_valid=true ORDER BY c.created_at DESC LIMIT 1`, workspaceID, projectID, jobID, pipelineRunID, nodeKey, schemaVersion, inputFingerprint).Scan(&value.ID, &value.PipelineRunID, &value.JobStepID, &value.CompletedNodeKey, &value.ContextArtifactID, &value.StateHash, &value.SchemaVersion, &value.InputFingerprint, &value.Valid)
	if errors.Is(err, sql.ErrNoRows) {
		return execution.Checkpoint{}, ErrNotFound
	}
	return value, err
}

func checkpointPayload(value execution.Checkpoint) json.RawMessage {
	payload, _ := json.Marshal(map[string]any{"checkpoint_id": value.ID, "input_fingerprint": value.InputFingerprint, "state_hash": value.StateHash})
	return payload
}
