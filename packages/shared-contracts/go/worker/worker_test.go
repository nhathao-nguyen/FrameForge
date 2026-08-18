package worker

import (
	"encoding/json"
	"testing"
)

func TestWorkerFixturesValidate(t *testing.T) {
	command := Command{SchemaVersion: CommandSchemaVersion, MessageID: "msg_probe_001", Capability: "probe", WorkspaceID: "workspace_probe_001", ProjectID: "project_probe_001", JobID: "job_probe_001", PipelineRunID: "run_probe_001", JobStepID: "step_probe_001", PipelineNodeID: "node_probe_001", NodeKey: "asset_probe", Attempt: 1, InputRefs: []ArtifactRef{{ArtifactID: "artifact_source_001", Role: "source", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}, Config: map[string]any{"thumbnail": false}}
	if err := ValidateCommand(command); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(command)
	if _, err := DecodeCommand(encoded); err != nil {
		t.Fatal(err)
	}
	result := Result{SchemaVersion: ResultSchemaVersion, MessageID: "msg_probe_001", JobID: "job_probe_001", JobStepID: "step_probe_001", Status: "completed", OutputRefs: []OutputRef{{ArtifactID: "artifact_probe_001", Kind: "media_probe", Role: "probe_report", SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", SizeBytes: 4}}}
	if err := ValidateResult(result); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerContractsRejectPathsAndUnknownStatus(t *testing.T) {
	command := Command{SchemaVersion: CommandSchemaVersion, MessageID: "msg_probe_001", Capability: "probe", WorkspaceID: "workspace_probe_001", ProjectID: "project_probe_001", JobID: "job_probe_001", PipelineRunID: "run_probe_001", JobStepID: "step_probe_001", NodeKey: "asset_probe", Attempt: 1, InputRefs: []ArtifactRef{}, Config: map[string]any{"local_path": "C:/secret"}}
	if err := ValidateCommand(command); err == nil {
		t.Fatal("path-bearing config was accepted")
	}
	result := Result{SchemaVersion: ResultSchemaVersion, MessageID: "msg_probe_001", JobID: "job_probe_001", JobStepID: "step_probe_001", Status: "processing"}
	if err := ValidateResult(result); err == nil {
		t.Fatal("non-canonical status was accepted")
	}
}

func TestWorkerContractsEnforceArtifactRefBounds(t *testing.T) {
	command := Command{SchemaVersion: CommandSchemaVersion, MessageID: "msg_probe_001", Capability: "probe", WorkspaceID: "workspace_probe_001", ProjectID: "project_probe_001", JobID: "job_probe_001", PipelineRunID: "run_probe_001", JobStepID: "step_probe_001", NodeKey: "asset_probe", Attempt: 1, InputRefs: make([]ArtifactRef, 101), Config: map[string]any{}}
	if err := ValidateCommand(command); err == nil {
		t.Fatal("oversized input Artifact refs were accepted")
	}
	result := Result{SchemaVersion: ResultSchemaVersion, MessageID: "msg_probe_001", JobID: "job_probe_001", JobStepID: "step_probe_001", Status: "completed", OutputRefs: make([]OutputRef, 101)}
	if err := ValidateResult(result); err == nil {
		t.Fatal("oversized output Artifact refs were accepted")
	}
	result.OutputRefs = nil
	if err := ValidateResult(result); err == nil {
		t.Fatal("null output Artifact refs were accepted")
	}
}
