package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
)

func TestPythonWorkerRoundTripUsesVersionedContract(t *testing.T) {
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv is not installed; Python worker integration is environment-gated")
	}
	_, file, _, _ := runtime.Caller(0)
	repository := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	command := worker.Command{SchemaVersion: worker.CommandSchemaVersion, MessageID: "msg_go_python_001", Capability: "analysis", ProjectID: "project_go_python_001", JobID: "job_go_python_001", PipelineRunID: "run_go_python_001", JobStepID: "step_go_python_001", Attempt: 1, InputRefs: []worker.ArtifactRef{}, Config: map[string]any{"mode": "deterministic"}}
	payload, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	process := exec.CommandContext(ctx, "uv", "run", "--project", "services/ml-worker", "python", "-m", "nh_media.worker")
	process.Dir = repository
	process.Stdin = bytes.NewReader(append(payload, '\n'))
	var output bytes.Buffer
	process.Stdout = &output
	process.Stderr = &output
	if err := process.Run(); err != nil {
		t.Fatalf("Python worker process failed: %v; output=%s", err, output.String())
	}
	var result worker.Result
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &result); err != nil {
		t.Fatalf("invalid Python worker JSON: %v; output=%s", err, output.String())
	}
	if err := worker.ValidateResult(result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" || len(result.OutputRefs) != 1 || result.OutputRefs[0].Role != "analysis_report" {
		t.Fatalf("unexpected Python worker result: %+v", result)
	}
	if bytes.Contains(bytes.ToLower(output.Bytes()), []byte("path")) {
		t.Fatal("Python worker exposed a path")
	}
}
