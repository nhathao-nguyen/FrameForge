package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/queue"
)

type ScopedCommandResolver struct {
	SQL       *SQLStore
	Workspace string
}

func (r ScopedCommandResolver) ResolveCommand(ctx context.Context, message queue.Message) (worker.Command, error) {
	if r.SQL == nil {
		return worker.Command{}, errors.New("database is required")
	}
	return r.SQL.ResolveWorkerCommand(ctx, r.Workspace, message)
}

// QueueReadySteps advances only dependency-satisfied steps and returns the
// ID-only messages that may be delivered after the transaction commits. The
// caller must enqueue the returned messages after commit; PostgreSQL state is
// still authoritative if Redis is unavailable.
func (s *SQLStore) QueueReadySteps(ctx context.Context, workspaceID, projectID, jobID, runID string) ([]queue.Message, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("database is required")
	}
	if workspaceID == "" || projectID == "" || jobID == "" || runID == "" {
		return nil, errors.New("queue scope is incomplete")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT s.id::text,s.pipeline_run_id::text,s.pipeline_node_id::text,s.node_key,n.execution_class,s.current_attempt
		FROM job_steps s
		JOIN pipeline_runs r ON r.id=s.pipeline_run_id
		JOIN jobs j ON j.id=r.job_id
		JOIN projects p ON p.id=j.project_id
		JOIN pipeline_nodes n ON n.id=s.pipeline_node_id
		WHERE p.workspace_id=$1::uuid AND j.project_id=$2::uuid AND j.id=$3::uuid AND r.id=$4::uuid
		  AND s.status IN ('pending','ready')
		  AND NOT EXISTS (
			SELECT 1 FROM pipeline_node_dependencies d
			JOIN job_steps dependency_step ON dependency_step.pipeline_run_id=s.pipeline_run_id AND dependency_step.pipeline_node_id=d.depends_on_node_id
			WHERE d.pipeline_id=n.pipeline_id AND d.node_id=s.pipeline_node_id
			  AND dependency_step.status NOT IN ('completed','skipped')
		  )
		ORDER BY s.node_key
		FOR UPDATE OF s`, workspaceID, projectID, jobID, runID)
	if err != nil {
		return nil, err
	}
	type readyStep struct {
		id, runID, nodeID, nodeKey, executionClass string
		attempt                                    int
	}
	steps := make([]readyStep, 0)
	for rows.Next() {
		var value readyStep
		if err := rows.Scan(&value.id, &value.runID, &value.nodeID, &value.nodeKey, &value.executionClass, &value.attempt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		steps = append(steps, value)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	messages := make([]queue.Message, 0, len(steps))
	for _, step := range steps {
		capability, err := capabilityForExecutionClass(step.executionClass)
		if err != nil {
			return nil, err
		}
		status := "pending"
		// A retrying step is explicitly re-queued by the retry command. This
		// method only handles the initial dependency frontier.
		if step.attempt < 0 {
			return nil, errors.New("negative step attempt")
		}
		if _, _, err := transitionStep(ctx, tx, TransitionInput{Entity: execution.EntityStep, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, JobStepID: step.id, PipelineNodeID: step.nodeID, NodeKey: step.nodeKey, FromStatus: status, ToStatus: "ready", CorrelationID: jobID, Payload: json.RawMessage(`{"reason":"dependency_satisfied"}`)}); err != nil {
			return nil, fmt.Errorf("ready step %s: %w", step.nodeKey, err)
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "node.ready", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, JobStepID: step.id, PipelineNodeID: step.nodeID, NodeKey: step.nodeKey, Payload: json.RawMessage(`{"reason":"dependency_satisfied"}`)}); err != nil {
			return nil, err
		}
		if _, _, err := transitionStep(ctx, tx, TransitionInput{Entity: execution.EntityStep, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, JobStepID: step.id, PipelineNodeID: step.nodeID, NodeKey: step.nodeKey, FromStatus: "ready", ToStatus: "queued", CorrelationID: jobID, Payload: json.RawMessage(`{"capability":"` + capability + `"}`)}); err != nil {
			return nil, fmt.Errorf("queue step %s: %w", step.nodeKey, err)
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "node.queued", CorrelationID: jobID, WorkspaceID: workspaceID, ProjectID: projectID, JobID: jobID, PipelineRunID: runID, JobStepID: step.id, PipelineNodeID: step.nodeID, NodeKey: step.nodeKey, Payload: json.RawMessage(`{"capability":"` + capability + `"}`)}); err != nil {
			return nil, err
		}
		messages = append(messages, queue.Message{MessageID: fmt.Sprintf("msg_%s_%s_1", strings.ReplaceAll(jobID, "-", "_"), strings.ReplaceAll(step.id, "-", "_")), Capability: capability, JobID: jobID, PipelineRunID: runID, JobStepID: step.id, PipelineNodeID: step.nodeID, NodeKey: step.nodeKey, Attempt: step.attempt + 1})
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return messages, nil
}

// QueueQueuedSteps recovers ID-only messages for steps whose PostgreSQL state
// is already queued. It is intentionally read-only and uses stable
// job/step/attempt message IDs, so a retry after an enqueue outage is safe even
// if the original Redis append actually succeeded.
func (s *SQLStore) QueueQueuedSteps(ctx context.Context, workspaceID, projectID, jobID, runID string) ([]queue.Message, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("database is required")
	}
	if workspaceID == "" || projectID == "" || jobID == "" || runID == "" {
		return nil, errors.New("queue scope is incomplete")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT s.id::text,s.pipeline_node_id::text,s.node_key,n.execution_class,s.current_attempt
		FROM job_steps s
		JOIN pipeline_runs r ON r.id=s.pipeline_run_id
		JOIN jobs j ON j.id=r.job_id
		JOIN projects p ON p.id=j.project_id
		JOIN pipeline_nodes n ON n.id=s.pipeline_node_id
		WHERE p.workspace_id=$1::uuid AND j.project_id=$2::uuid AND j.id=$3::uuid AND r.id=$4::uuid AND s.status='queued'
		ORDER BY s.node_key`, workspaceID, projectID, jobID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := make([]queue.Message, 0)
	for rows.Next() {
		var stepID, nodeID, nodeKey, executionClass string
		var currentAttempt int
		if err := rows.Scan(&stepID, &nodeID, &nodeKey, &executionClass, &currentAttempt); err != nil {
			return nil, err
		}
		capability, err := capabilityForExecutionClass(executionClass)
		if err != nil {
			return nil, err
		}
		messages = append(messages, queue.Message{MessageID: fmt.Sprintf("msg_%s_%s_%d", strings.ReplaceAll(jobID, "-", "_"), strings.ReplaceAll(stepID, "-", "_"), currentAttempt+1), Capability: capability, JobID: jobID, PipelineRunID: runID, JobStepID: stepID, PipelineNodeID: nodeID, NodeKey: nodeKey, Attempt: currentAttempt + 1})
	}
	return messages, rows.Err()
}

// QueueRetryingSteps durably advances retrying steps after their bounded
// backoff. The state transition and retry intent are committed before Redis
// receives the ID-only message, so a transport failure leaves a recoverable
// queued row rather than losing the retry.
func (s *SQLStore) QueueRetryingSteps(ctx context.Context, workspaceID string) ([]queue.Message, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("database is required")
	}
	if workspaceID == "" {
		return nil, errors.New("retry queue workspace is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT j.project_id::text,j.id::text,r.id::text,s.id::text,s.pipeline_node_id::text,s.node_key,n.execution_class,j.status,r.status,s.current_attempt,n.max_attempts,n.retry_policy,s.updated_at
		FROM job_steps s
		JOIN pipeline_runs r ON r.id=s.pipeline_run_id
		JOIN jobs j ON j.id=r.job_id
		JOIN projects p ON p.id=j.project_id
		JOIN pipeline_nodes n ON n.id=s.pipeline_node_id
		WHERE p.workspace_id=$1::uuid AND s.status='retrying'
		ORDER BY s.updated_at,s.id
		FOR UPDATE OF s SKIP LOCKED`, workspaceID)
	if err != nil {
		return nil, err
	}
	type candidate struct {
		projectID, jobID, runID, stepID, nodeID, nodeKey, executionClass, jobStatus, runStatus string
		attempt, maxAttempts                                                                   int
		policy                                                                                 []byte
		updatedAt                                                                              time.Time
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var value candidate
		if err := rows.Scan(&value.projectID, &value.jobID, &value.runID, &value.stepID, &value.nodeID, &value.nodeKey, &value.executionClass, &value.jobStatus, &value.runStatus, &value.attempt, &value.maxAttempts, &value.policy, &value.updatedAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		candidates = append(candidates, value)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	now := time.Now().UTC()
	messages := make([]queue.Message, 0, len(candidates))
	for _, value := range candidates {
		if value.jobStatus == "cancelling" || value.jobStatus == "cancelled" || value.jobStatus == "dead_lettered" || value.runStatus == "cancelled" {
			continue
		}
		if value.attempt >= value.maxAttempts {
			if _, _, err := transitionStep(ctx, tx, TransitionInput{Entity: execution.EntityStep, WorkspaceID: workspaceID, ProjectID: value.projectID, JobID: value.jobID, PipelineRunID: value.runID, JobStepID: value.stepID, PipelineNodeID: value.nodeID, NodeKey: value.nodeKey, FromStatus: "retrying", ToStatus: "failed", CorrelationID: value.jobID, Payload: json.RawMessage(`{"code":"retry_exhausted","safe_message":"Retry budget was exhausted."}`)}); err != nil {
				return nil, err
			}
			if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "node.failed", CorrelationID: value.jobID, WorkspaceID: workspaceID, ProjectID: value.projectID, JobID: value.jobID, PipelineRunID: value.runID, JobStepID: value.stepID, PipelineNodeID: value.nodeID, NodeKey: value.nodeKey, Payload: json.RawMessage(`{"code":"retry_exhausted","safe_message":"Retry budget was exhausted."}`)}); err != nil {
				return nil, err
			}
			if err := failAggregateTx(ctx, tx, workspaceID, value.projectID, value.jobID, value.runID); err != nil {
				return nil, err
			}
			continue
		}
		policy := execution.ParseRetryPolicy(value.policy)
		delay, err := execution.Backoff(value.attempt, policy.BaseDelay, policy.MaxDelay)
		if err != nil {
			return nil, err
		}
		if now.Sub(value.updatedAt) < delay {
			continue
		}
		if value.jobStatus == "running" {
			if _, _, err := transitionJob(ctx, tx, TransitionInput{Entity: execution.EntityJob, WorkspaceID: workspaceID, ProjectID: value.projectID, JobID: value.jobID, PipelineRunID: value.runID, FromStatus: "running", ToStatus: "retrying", CorrelationID: value.jobID, Payload: json.RawMessage(`{"reason":"step_retry"}`)}); err != nil {
				return nil, err
			}
			if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "job.retry_scheduled", CorrelationID: value.jobID, WorkspaceID: workspaceID, ProjectID: value.projectID, JobID: value.jobID, PipelineRunID: value.runID, Payload: json.RawMessage(`{"reason":"step_retry"}`)}); err != nil {
				return nil, err
			}
			value.jobStatus = "retrying"
		}
		if value.jobStatus == "retrying" {
			if _, _, err := transitionJob(ctx, tx, TransitionInput{Entity: execution.EntityJob, WorkspaceID: workspaceID, ProjectID: value.projectID, JobID: value.jobID, PipelineRunID: value.runID, FromStatus: "retrying", ToStatus: "queued", CorrelationID: value.jobID, Payload: json.RawMessage(`{"reason":"retry_ready"}`)}); err != nil {
				return nil, err
			}
			if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "job.queued", CorrelationID: value.jobID, WorkspaceID: workspaceID, ProjectID: value.projectID, JobID: value.jobID, PipelineRunID: value.runID, Payload: json.RawMessage(`{"reason":"retry_ready"}`)}); err != nil {
				return nil, err
			}
			value.jobStatus = "queued"
		}
		if value.jobStatus != "queued" {
			continue
		}
		if _, _, err := transitionStep(ctx, tx, TransitionInput{Entity: execution.EntityStep, WorkspaceID: workspaceID, ProjectID: value.projectID, JobID: value.jobID, PipelineRunID: value.runID, JobStepID: value.stepID, PipelineNodeID: value.nodeID, NodeKey: value.nodeKey, FromStatus: "retrying", ToStatus: "queued", CorrelationID: value.jobID, Payload: json.RawMessage(`{"reason":"retry_ready"}`)}); err != nil {
			return nil, err
		}
		if _, err := appendEventAndOutboxTx(ctx, tx, EventInput{SchemaVersion: "1.0", EventType: "node.queued", CorrelationID: value.jobID, WorkspaceID: workspaceID, ProjectID: value.projectID, JobID: value.jobID, PipelineRunID: value.runID, JobStepID: value.stepID, PipelineNodeID: value.nodeID, NodeKey: value.nodeKey, Payload: json.RawMessage(`{"reason":"retry_ready"}`)}); err != nil {
			return nil, err
		}
		capability, err := capabilityForExecutionClass(value.executionClass)
		if err != nil {
			return nil, err
		}
		messages = append(messages, queue.Message{MessageID: fmt.Sprintf("msg_%s_%s_%d", strings.ReplaceAll(value.jobID, "-", "_"), strings.ReplaceAll(value.stepID, "-", "_"), value.attempt+1), Capability: capability, JobID: value.jobID, PipelineRunID: value.runID, JobStepID: value.stepID, PipelineNodeID: value.nodeID, NodeKey: value.nodeKey, Attempt: value.attempt + 1})
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return messages, nil
}

// ResolveWorkerCommand turns a durable ID-only queue delivery into the
// versioned command consumed by a worker. It accepts only a still-queued
// current attempt, preventing a stale delivery from launching work.
func (s *SQLStore) ResolveWorkerCommand(ctx context.Context, workspaceID string, message queue.Message) (worker.Command, error) {
	if s == nil || s.DB == nil {
		return worker.Command{}, errors.New("database is required")
	}
	var projectID, runID, stepID, nodeID, nodeKey, executionClass, status string
	var configJSON, inputRefsJSON []byte
	var currentAttempt int
	err := s.DB.QueryRowContext(ctx, `SELECT j.project_id::text,r.id::text,s.id::text,n.id::text,s.node_key,n.execution_class,n.config,s.input_refs,s.status,s.current_attempt
		FROM job_steps s
		JOIN pipeline_runs r ON r.id=s.pipeline_run_id
		JOIN jobs j ON j.id=r.job_id
		JOIN projects p ON p.id=j.project_id
		JOIN pipeline_nodes n ON n.id=s.pipeline_node_id
		WHERE p.workspace_id=$1::uuid AND j.id=$2::uuid AND r.id=$3::uuid AND s.id=$4::uuid
		  AND s.status='queued' AND s.current_attempt=$5`, workspaceID, message.JobID, message.PipelineRunID, message.JobStepID, message.Attempt-1).Scan(&projectID, &runID, &stepID, &nodeID, &nodeKey, &executionClass, &configJSON, &inputRefsJSON, &status, &currentAttempt)
	if errors.Is(err, sql.ErrNoRows) {
		return worker.Command{}, execution.ErrLeaseConflict
	}
	if err != nil {
		return worker.Command{}, err
	}
	capability, err := capabilityForExecutionClass(executionClass)
	if err != nil || capability != message.Capability || status != "queued" || currentAttempt+1 != message.Attempt {
		return worker.Command{}, errors.New("queue delivery does not match current step")
	}
	refs := make([]worker.ArtifactRef, 0)
	if len(inputRefsJSON) > 0 && string(inputRefsJSON) != "{}" {
		if err := json.Unmarshal(inputRefsJSON, &refs); err != nil {
			// Upload/bootstrap commands may still contain a domain input
			// snapshot rather than Artifact refs. That is not a path-bearing
			// command; the node resolves it through its declared boundary.
			refs = make([]worker.ArtifactRef, 0)
		}
	}
	command := worker.Command{SchemaVersion: worker.CommandSchemaVersion, MessageID: message.MessageID, Capability: capability, ProjectID: contractID("project", projectID), JobID: contractID("job", message.JobID), PipelineRunID: contractID("run", runID), JobStepID: contractID("step", stepID), PipelineNodeID: contractID("node", nodeID), Attempt: message.Attempt, InputRefs: refs, Config: mapFromJSON(configJSON)}
	if err := worker.ValidateCommand(command); err != nil {
		return worker.Command{}, err
	}
	_ = nodeKey
	return command, nil
}

func capabilityForExecutionClass(value string) (string, error) {
	switch value {
	case "probe":
		return "probe", nil
	case "ai", "ml":
		return "analysis", nil
	case "media", "render":
		return "thumbnail", nil
	default:
		return "", fmt.Errorf("execution class %q has no enabled worker capability", value)
	}
}

func contractID(prefix, value string) string {
	value = strings.NewReplacer("-", "_", ".", "_").Replace(strings.TrimSpace(value))
	return prefix + "_" + value
}
