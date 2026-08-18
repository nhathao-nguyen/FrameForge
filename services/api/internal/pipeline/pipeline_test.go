package pipeline

import (
	"context"
	"testing"
)

func testNode(key, input, output string) Node {
	return Node{
		Key: key, Type: "fake", DisplayName: key, InputContract: input, OutputContract: output,
		ExecutionClass: "system", FailureMode: "hard", ReviewPolicy: "none",
		Timeout: TimeoutPolicy{WallTimeSec: 1}, Retry: RetryPolicy{MaxAttempts: 1},
		Progress:    ProgressPolicy{Unit: "item", TotalSource: "items", EmitInterval: 1},
		Checkpoint:  CheckpointPolicy{Mode: "terminal"},
		Idempotency: IdempotencyPolicy{FingerprintFields: []string{"input"}, Reuse: true, SideEffects: "none"},
		InputSchema: []byte("{\"type\":\"object\"}"), OutputSchema: []byte("{\"type\":\"object\"}"),
	}
}

func testDefinition(nodes []Node) Definition {
	weights := map[string]float64{}
	for _, node := range nodes {
		weights[node.Key] = 1
	}
	return Definition{
		SchemaVersion: DefinitionSchemaVersion, WorkflowKey: "test_graph", Version: 1,
		Nodes: nodes, Policies: Policies{ArtifactRetention: "protected", ProgressWeights: weights},
		Provenance: Provenance{NHMediaRelease: "test", ContractVersion: "test"},
	}
}

func TestMovieRecapValidatesAndHashIsStable(t *testing.T) {
	definition := MovieRecap()
	if err := definition.Validate(AllCapabilities()); err != nil {
		t.Fatal(err)
	}
	first, err := definition.Hash()
	if err != nil {
		t.Fatal(err)
	}
	second, err := definition.Hash()
	if err != nil || first != second {
		t.Fatalf("hash is not stable: %q %q %v", first, second, err)
	}
	if len(definition.Nodes) < 20 {
		t.Fatalf("native graph is unexpectedly small: %d", len(definition.Nodes))
	}
	if _, err := VerifyImmutableActive(first, definition, AllCapabilities()); err != nil {
		t.Fatal(err)
	}
	definition.Nodes[0].DisplayName = "mutated"
	if _, err := VerifyImmutableActive(first, definition, AllCapabilities()); err == nil {
		t.Fatal("active definition mutation was accepted")
	}
	if catalog := BuiltinCatalog(); len(catalog) != 1 || catalog["movie_recap"].Version != 2 {
		t.Fatalf("native catalog is not registered: %#v", catalog)
	}
}

func TestValidatorRejectsDuplicateCycleContractCapabilityAndIncompletePolicy(t *testing.T) {
	baseNodes := []Node{testNode("a_node", "context/v1", "a/v1"), testNode("b_node", "a/v1", "b/v1")}
	baseNodes[1].DependsOn = []Dependency{{NodeKey: "a_node", Required: true, Condition: "success"}}
	base := testDefinition(baseNodes)
	if err := base.Validate(AllCapabilities()); err != nil {
		t.Fatal(err)
	}
	duplicate := base
	duplicate.Nodes = append(duplicate.Nodes, base.Nodes[0])
	if err := duplicate.Validate(AllCapabilities()); err == nil {
		t.Fatal("duplicate key accepted")
	}
	cycle := base
	cycle.Nodes[0].DependsOn = []Dependency{{NodeKey: "b_node", Required: true, Condition: "success"}}
	if err := cycle.Validate(AllCapabilities()); err == nil {
		t.Fatal("cycle accepted")
	}
	contract := base
	contract.Nodes[1].InputContract = "other/v1"
	if err := contract.Validate(AllCapabilities()); err == nil {
		t.Fatal("contract mismatch accepted")
	}
	capability := base
	capability.Nodes[0].ExecutionClass = "ml"
	if err := capability.Validate(CapabilitySet{"system": true}); err == nil {
		t.Fatal("unavailable capability accepted")
	}
	incomplete := base
	incomplete.Nodes[0].Checkpoint.Mode = ""
	if err := incomplete.Validate(AllCapabilities()); err == nil {
		t.Fatal("incomplete policy accepted")
	}
}

func TestRuntimeCoversRetryReviewSoftFailureStopAfterCancelAndIdempotency(t *testing.T) {
	nodes := []Node{testNode("branch_a", "context/v1", "context/v1"), testNode("branch_b", "context/v1", "context/v1"), testNode("join_node", "context/v1", "context/v1")}
	nodes[0].Retry.MaxAttempts = 2
	nodes[1].FailureMode, nodes[1].Soft = "soft", true
	nodes[2].DependsOn = []Dependency{{NodeKey: "branch_a", Required: true, Condition: "success"}, {NodeKey: "branch_b", Required: false, Condition: "soft"}}
	definition := testDefinition(nodes)
	if err := definition.Validate(AllCapabilities()); err != nil {
		t.Fatal(err)
	}
	executor := &DeterministicExecutor{Results: map[string][]Result{
		"branch_a": {{Outcome: Failed, Error: &NodeError{Category: "transient", Retryable: true}}, {Outcome: Completed, Outputs: map[string]any{"ok": true}}},
		"branch_b": {{Outcome: Failed, Error: &NodeError{Category: "permanent"}}},
	}}
	runtime := &Runtime{Definition: definition, Executors: map[string]Executor{"fake": executor}}
	result := runtime.Run(context.Background(), Request{JobID: "job_1", RunID: "run_1"})
	if result.Outcome != Completed || len(result.Nodes) != 3 {
		t.Fatalf("unexpected run: %+v", result)
	}
	if result.Nodes[0].Attempts != 2 {
		t.Fatalf("retry was not used: %+v", result.Nodes[0])
	}
	if runtime.Run(context.Background(), Request{JobID: "job_1", RunID: "run_1"}).Nodes[0].Reused != true {
		t.Fatal("idempotent output was not reused")
	}
	stopRuntime := &Runtime{Definition: definition, Executors: map[string]Executor{"fake": &DeterministicExecutor{}}}
	stop := stopRuntime.Run(context.Background(), Request{JobID: "job_2", RunID: "run_2", StopAfter: "branch_a"})
	if stop.Outcome != Paused {
		t.Fatalf("stop_after did not pause: %+v", stop)
	}
	reviewNode := testNode("review_node", "context/v1", "context/v1")
	reviewNode.ReviewPolicy, reviewNode.HumanGate = "approval_completes_node", true
	reviewDefinition := testDefinition([]Node{reviewNode})
	reviewRuntime := &Runtime{Definition: reviewDefinition, Executors: map[string]Executor{"fake": &DeterministicExecutor{Results: map[string][]Result{"review_node": {{Outcome: WaitingForReview, Review: map[string]any{"resource_id": "script_v1"}}}}}}}
	review := reviewRuntime.Run(context.Background(), Request{JobID: "job_3", RunID: "run_3"})
	if review.Outcome != WaitingForReview {
		t.Fatalf("review was not durable outcome: %+v", review)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if got := (&Runtime{Definition: reviewDefinition, Executors: map[string]Executor{"fake": &DeterministicExecutor{}}}).Run(cancelled, Request{}).Outcome; got != Cancelled {
		t.Fatalf("cancel was not propagated: %s", got)
	}
}
