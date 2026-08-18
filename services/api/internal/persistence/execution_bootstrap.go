package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// createExecutionGraphTx creates the immutable PipelineRun/JobStep snapshot
// before any queue delivery. It is deliberately transaction-local so a job
// can never be visible as executable without its graph and job.created event.
func createExecutionGraphTx(ctx context.Context, tx *sql.Tx, jobID, projectID, pipelineID, workspaceID string, pipelineSnapshot, inputSnapshot, command json.RawMessage) (string, error) {
	if jobID == "" || projectID == "" || pipelineID == "" || workspaceID == "" {
		return "", errors.New("execution graph scope is incomplete")
	}
	if len(inputSnapshot) == 0 {
		inputSnapshot = command
	}
	var runID string
	if err := tx.QueryRowContext(ctx, `INSERT INTO pipeline_runs(job_id,pipeline_id,run_number,status,executor_kind,executor_version,contract_version,input_snapshot,config_snapshot) VALUES($1::uuid,$2::uuid,1,'created','system','nh-media-bootstrap','worker/v1',$3,$4) RETURNING id::text`, jobID, pipelineID, inputSnapshot, jsonOrEmpty(pipelineSnapshot)).Scan(&runID); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE jobs SET current_pipeline_run_id=$2::uuid,current_run_number=1,updated_at=now() WHERE id=$1::uuid`, jobID, runID); err != nil {
		return "", err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id::text,node_key,execution_class,progress_weight FROM pipeline_nodes WHERE pipeline_id=$1::uuid ORDER BY node_key`, pipelineID)
	if err != nil {
		return "", err
	}
	type node struct {
		id, key, class string
		weight         float64
	}
	nodes := make([]node, 0)
	for rows.Next() {
		var value node
		if err := rows.Scan(&value.id, &value.key, &value.class, &value.weight); err != nil {
			_ = rows.Close()
			return "", err
		}
		nodes = append(nodes, value)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return "", err
	}
	_ = rows.Close()
	if len(nodes) == 0 {
		return "", errors.New("active pipeline has no nodes")
	}
	var commandMeta struct {
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(command, &commandMeta)
	for _, node := range nodes {
		status, skipReason := "pending", ""
		if commandMeta.Kind == "render" && node.key != "render_timeline" && node.key != "validate_deliverable" {
			status, skipReason = "skipped", "render_job_boundary"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO job_steps(pipeline_run_id,pipeline_node_id,node_key,status,input_refs,provider_snapshot,resource_class,skip_reason,created_at,updated_at) VALUES($1::uuid,$2::uuid,$3,$4,$5,'{}'::jsonb,$6,NULLIF($7,''),now(),now())`, runID, node.id, node.key, status, jsonOrEmpty(inputSnapshot), node.class, skipReason); err != nil {
			return "", fmt.Errorf("create JobStep %s: %w", node.key, err)
		}
	}
	if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{
		SchemaVersion: "1.0", EventType: "job.created", CorrelationID: jobID,
		WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID,
		PipelineRunID: runID, Payload: json.RawMessage(`{"kind":"pipeline","mode":"automatic"}`),
	}); err != nil {
		return "", err
	}
	if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{
		SchemaVersion: "1.0", EventType: "pipeline_run.created", CorrelationID: jobID,
		WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID,
		PipelineRunID: runID, Payload: json.RawMessage(`{"run_number":1,"contract_version":"worker/v1"}`),
	}); err != nil {
		return "", err
	}
	return runID, nil
}

// validateExecutionBoundaries checks the client-provided partial-execution
// selectors against the immutable pipeline snapshot before a Job becomes
// durable. The selector is a node key, never an executor-specific shortcut.
func validateExecutionBoundaries(pipelineSnapshot, command json.RawMessage) error {
	var snapshot struct {
		Nodes []struct {
			NodeKey string `json:"node_key"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(pipelineSnapshot, &snapshot); err != nil {
		return fmt.Errorf("pipeline snapshot is invalid: %w", err)
	}
	keys := make(map[string]struct{}, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		if node.NodeKey != "" {
			keys[node.NodeKey] = struct{}{}
		}
	}
	var boundaries struct {
		StartFrom string `json:"start_from"`
		StopAfter string `json:"stop_after"`
	}
	if err := json.Unmarshal(command, &boundaries); err != nil {
		return fmt.Errorf("execution command is invalid: %w", err)
	}
	for name, value := range map[string]string{"start_from": boundaries.StartFrom, "stop_after": boundaries.StopAfter} {
		if value == "" {
			continue
		}
		if _, ok := keys[value]; !ok {
			return fmt.Errorf("%s boundary %q is not in the pipeline snapshot", name, value)
		}
	}
	return nil
}
