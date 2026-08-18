package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
)

type fakeExecutor struct{ err error }

func (f fakeExecutor) Execute(_ context.Context, command worker.Command) (worker.Result, error) {
	if f.err != nil {
		return worker.Result{}, f.err
	}
	return worker.Result{SchemaVersion: worker.ResultSchemaVersion, MessageID: command.MessageID, JobID: command.JobID, JobStepID: command.JobStepID, Status: "completed", OutputRefs: []worker.OutputRef{}}, nil
}

func TestFailedResultIsContractSafeAndBounded(t *testing.T) {
	command := worker.Command{SchemaVersion: worker.CommandSchemaVersion, MessageID: "msg_analysis_001", Capability: "analysis", WorkspaceID: "workspace_analysis_001", ProjectID: "project_analysis_001", JobID: "job_analysis_001", PipelineRunID: "run_analysis_001", JobStepID: "step_analysis_001", NodeKey: "analysis", Attempt: 1, InputRefs: []worker.ArtifactRef{}, Config: map[string]any{}}
	result := failedResult(command, errors.New("internal path C:/not-safe-to-return"))
	if err := worker.ValidateResult(result); err != nil {
		t.Fatal(err)
	}
	if result.SafeError == nil || result.Status != "failed" || len(result.OutputRefs) != 0 {
		t.Fatalf("unexpected failed result: %+v", result)
	}
}

func TestControllerRequiresKnownCapability(t *testing.T) {
	if _, err := NewRedisController(RedisOptions{Addr: "127.0.0.1:6379", Capability: "unknown"}, fakeExecutor{}); err == nil {
		t.Fatal("unknown worker capability was accepted")
	}
}

func TestControllerDrainStopsNewClaims(t *testing.T) {
	controller := &RedisController{}
	if controller.IsDraining() {
		t.Fatal("new controller is draining")
	}
	controller.BeginDrain()
	if !controller.IsDraining() {
		t.Fatal("drain state was not recorded")
	}
}
