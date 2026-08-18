package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/pipeline"
)

// nativePipelineInput is the single persistence adapter for the executable
// native catalog. Keeping this conversion here prevents the SQL bootstrap
// path from inventing a second, simplified graph.
func nativePipelineInput(definition pipeline.Definition, workflowName, description, creator string, status string) (PipelineDefinitionInput, error) {
	if err := definition.Validate(pipeline.AllCapabilities()); err != nil {
		return PipelineDefinitionInput{}, fmt.Errorf("native pipeline %s: %w", definition.WorkflowKey, err)
	}
	if _, err := definition.Hash(); err != nil {
		return PipelineDefinitionInput{}, err
	}
	definitionDocument, err := json.Marshal(definition)
	if err != nil {
		return PipelineDefinitionInput{}, err
	}
	nodes := make([]PipelineNodeInput, 0, len(definition.Nodes))
	dependencies := make([]PipelineDependencyInput, 0)
	for _, node := range definition.Nodes {
		inputSchema, err := cloneJSON(node.InputSchema)
		if err != nil {
			return PipelineDefinitionInput{}, fmt.Errorf("node %s input schema: %w", node.Key, err)
		}
		outputSchema, err := cloneJSON(node.OutputSchema)
		if err != nil {
			return PipelineDefinitionInput{}, fmt.Errorf("node %s output schema: %w", node.Key, err)
		}
		roles, _ := json.Marshal(node.RequiredArtifactRoles)
		produced, _ := json.Marshal(node.ProducedArtifactRoles)
		retry, _ := json.Marshal(node.Retry)
		checkpoint, _ := json.Marshal(node.Checkpoint)
		resources, _ := json.Marshal(node.ResourceRequirements)
		idempotency, _ := json.Marshal(node.Idempotency)
		nodes = append(nodes, PipelineNodeInput{
			NodeKey: node.Key, NodeType: node.Type, DisplayName: node.DisplayName,
			ExecutionClass: node.ExecutionClass, Optional: node.Soft, FailureMode: node.FailureMode,
			ReviewPolicy: node.ReviewPolicy, InputSchema: inputSchema, OutputSchema: outputSchema,
			RequiredArtifactRoles: roles, ProducedArtifactRoles: produced, Config: []byte(`{}`),
			RetryPolicy: retry, CheckpointPolicy: checkpoint, ResourceRequirements: resources,
			IdempotencyPolicy: idempotency, TimeoutSec: node.Timeout.WallTimeSec,
			MaxAttempts: node.Retry.MaxAttempts, ProgressWeight: definition.Policies.ProgressWeights[node.Key],
		})
		for _, dependency := range node.DependsOn {
			condition, _ := json.Marshal(dependency.Condition)
			dependencies = append(dependencies, PipelineDependencyInput{NodeKey: node.Key, DependsOnNodeKey: dependency.NodeKey, Condition: condition, Required: dependency.Required})
		}
	}
	return PipelineDefinitionInput{
		WorkflowKey: definition.WorkflowKey, WorkflowName: workflowName, Description: description,
		SchemaVersion: pipeline.DefinitionSchemaVersion, Definition: definitionDocument,
		Version: definition.Version, Status: status, CreatedBy: creator, Nodes: nodes, Dependencies: dependencies,
	}, nil
}

func nativeMovieRecapInput(creator string) (PipelineDefinitionInput, error) {
	return nativePipelineInput(
		pipeline.MovieRecap(),
		"Movie recap",
		"NH-Media native movie recap graph",
		creator,
		"active",
	)
}

func cloneJSON(value json.RawMessage) (json.RawMessage, error) {
	if len(value) == 0 || !json.Valid(value) {
		return nil, fmt.Errorf("schema must be valid JSON")
	}
	var object map[string]any
	if err := json.Unmarshal(value, &object); err != nil || object == nil {
		return nil, fmt.Errorf("schema must be a JSON object")
	}
	return append(json.RawMessage(nil), value...), nil
}

func nativeAssetValidationInput(creator string) PipelineDefinitionInput {
	return PipelineDefinitionInput{
		WorkflowKey: "asset_validation", WorkflowName: "Asset validation", Description: "Dedicated upload validation graph", SchemaVersion: "1.0", Version: 1, Status: "active", CreatedBy: creator,
		Definition: []byte(`{"workflow_key":"asset_validation","version":1,"nodes":[{"key":"asset_probe","type":"asset_probe","execution_class":"probe"}]}`),
		Nodes:      []PipelineNodeInput{{NodeKey: "asset_probe", NodeType: "asset_probe", DisplayName: "Validate uploaded asset", ExecutionClass: "probe", FailureMode: "hard", ReviewPolicy: "none", InputSchema: []byte(`{"type":"object"}`), OutputSchema: []byte(`{"type":"object"}`), ProducedArtifactRoles: []byte(`[]`), Config: []byte(`{"purpose":"upload_validation"}`), TimeoutSec: 900, MaxAttempts: 1, RetryPolicy: []byte(`{"retryable_categories":[]}`), CheckpointPolicy: []byte(`{"mode":"terminal"}`), ResourceRequirements: []byte(`{"network":"none"}`), IdempotencyPolicy: []byte(`{"fingerprint_fields":["asset_id"],"reuse":true,"side_effects":"none"}`), ProgressWeight: 1}},
	}
}

func nativeAcceptanceAnalysisInput(creator string) PipelineDefinitionInput {
	return PipelineDefinitionInput{
		WorkflowKey: "acceptance_analysis", WorkflowName: "Acceptance analysis", Description: "Deterministic Redis/Python worker acceptance graph", SchemaVersion: "1.0", Version: 1, Status: "active", CreatedBy: creator,
		Definition: []byte(`{"workflow_key":"acceptance_analysis","version":1,"nodes":["analysis"]}`),
		Nodes:      []PipelineNodeInput{{NodeKey: "analysis", NodeType: "analysis", DisplayName: "Deterministic analysis acceptance", ExecutionClass: "ml", FailureMode: "hard", ReviewPolicy: "none", InputSchema: []byte(`{"type":"object"}`), OutputSchema: []byte(`{"type":"object"}`), ProducedArtifactRoles: []byte(`[]`), Config: []byte(`{"mode":"deterministic"}`), TimeoutSec: 120, MaxAttempts: 2, RetryPolicy: []byte(`{"retryable_categories":["transient"]}`), CheckpointPolicy: []byte(`{"mode":"terminal"}`), ResourceRequirements: []byte(`{"network":"none"}`), IdempotencyPolicy: []byte(`{"fingerprint_fields":["message_id"],"reuse":true,"side_effects":"artifact_commit"}`), ProgressWeight: 1}},
	}
}

func validatePipelineDefinitionInput(input PipelineDefinitionInput) error {
	if len(input.Definition) == 0 {
		return errors.New("pipeline definition is required")
	}
	if len(input.Nodes) == 0 {
		return errors.New("pipeline must contain at least one node")
	}
	seenNodes := make(map[string]bool, len(input.Nodes))
	for _, node := range input.Nodes {
		if node.NodeKey == "" || seenNodes[node.NodeKey] {
			return fmt.Errorf("pipeline node key %q is missing or duplicated", node.NodeKey)
		}
		seenNodes[node.NodeKey] = true
		for name, schema := range map[string]json.RawMessage{"input_schema": node.InputSchema, "output_schema": node.OutputSchema} {
			if len(schema) == 0 {
				if input.SchemaVersion == pipeline.DefinitionSchemaVersion {
					return fmt.Errorf("node %s %s is required", node.NodeKey, name)
				}
				continue
			}
			if !json.Valid(schema) {
				return fmt.Errorf("node %s %s is malformed JSON", node.NodeKey, name)
			}
			var object map[string]any
			if err := json.Unmarshal(schema, &object); err != nil || object == nil {
				return fmt.Errorf("node %s %s must be a JSON object", node.NodeKey, name)
			}
		}
	}
	seenDependencies := map[string]bool{}
	for _, dependency := range input.Dependencies {
		key := dependency.NodeKey + "\x00" + dependency.DependsOnNodeKey
		if dependency.NodeKey == "" || dependency.DependsOnNodeKey == "" || dependency.NodeKey == dependency.DependsOnNodeKey || !seenNodes[dependency.NodeKey] || !seenNodes[dependency.DependsOnNodeKey] || seenDependencies[key] {
			return errors.New("pipeline dependency references an invalid or duplicated node")
		}
		seenDependencies[key] = true
		condition := strings.TrimSpace(string(dependency.Condition))
		if condition == "" {
			if input.SchemaVersion == pipeline.DefinitionSchemaVersion {
				return errors.New("native pipeline dependency condition is required")
			}
			continue
		}
		var value any
		if err := json.Unmarshal(dependency.Condition, &value); err != nil {
			return fmt.Errorf("pipeline dependency condition is malformed JSON: %w", err)
		}
		if text, ok := value.(string); ok {
			if text != "success" && text != "soft" && text != "success_or_declared_soft" {
				return fmt.Errorf("unsupported pipeline dependency condition %q", text)
			}
		}
	}
	return nil
}
