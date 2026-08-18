package execution

import "testing"

func TestCheckpointReuseRequiresExactSchemaAndInputFingerprint(t *testing.T) {
	value := Checkpoint{ID: "checkpoint_001", PipelineRunID: "run_001", JobStepID: "step_001", CompletedNodeKey: "probe", ContextArtifactID: "artifact_001", StateHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SchemaVersion: "worker/v1", InputFingerprint: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Valid: true}
	if err := ValidateCheckpoint(value); err != nil {
		t.Fatal(err)
	}
	if !CanReuseCheckpoint(value, "worker/v1", value.InputFingerprint) {
		t.Fatal("compatible checkpoint was not reusable")
	}
	if CanReuseCheckpoint(value, "worker/v2", value.InputFingerprint) || CanReuseCheckpoint(value, "worker/v1", "cccc") {
		t.Fatal("stale checkpoint was reused")
	}
}

func TestBuildResumePlanCoversCrashBoundariesAndBoundaries(t *testing.T) {
	checkpoint := valueForRecoveryTest()
	plan, err := BuildResumePlan([]RecoveryStep{
		{NodeKey: "probe", Status: "completed", SchemaVersion: "worker/v1", InputFingerprint: checkpoint.InputFingerprint, Checkpoint: &checkpoint},
		{NodeKey: "analysis", Status: "retrying", SchemaVersion: "worker/v1", InputFingerprint: checkpoint.InputFingerprint},
		{NodeKey: "render", Status: "pending", SchemaVersion: "worker/v1", InputFingerprint: checkpoint.InputFingerprint},
	}, "worker/v1", "analysis", "analysis")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ReuseNodeKeys) != 1 || plan.ReuseNodeKeys[0] != "probe" || len(plan.QueueNodeKeys) != 1 || plan.QueueNodeKeys[0] != "analysis" || plan.PauseAfterNode != "analysis" {
		t.Fatalf("unexpected recovery plan: %#v", plan)
	}
	if _, err := BuildResumePlan([]RecoveryStep{{NodeKey: "analysis", Status: "waiting_for_review"}}, "worker/v1", "", ""); err == nil {
		t.Fatal("review gate was bypassed")
	}
	if _, err := BuildResumePlan([]RecoveryStep{{NodeKey: "analysis", Status: "running"}}, "worker/v1", "", ""); err == nil {
		t.Fatal("active lease was resumed without reconciliation")
	}
}

func valueForRecoveryTest() Checkpoint {
	return Checkpoint{ID: "checkpoint_002", PipelineRunID: "run_002", JobStepID: "step_002", CompletedNodeKey: "probe", ContextArtifactID: "artifact_002", StateHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SchemaVersion: "worker/v1", InputFingerprint: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Valid: true}
}
