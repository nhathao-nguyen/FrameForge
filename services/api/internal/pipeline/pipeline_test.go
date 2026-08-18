package pipeline

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func testNode(key, input, output string) Node {
	return Node{
		Key: key, Type: "fake", DisplayName: key, InputContract: input, OutputContract: output,
		ExecutionClass: "system", FailureMode: "hard", ReviewPolicy: "none",
		Timeout: TimeoutPolicy{WallTimeSec: 1}, Retry: RetryPolicy{MaxAttempts: 1},
		Progress:             ProgressPolicy{Unit: "item", TotalSource: "items", EmitInterval: 1},
		Checkpoint:           CheckpointPolicy{Mode: "terminal"},
		Idempotency:          IdempotencyPolicy{FingerprintFields: []string{"input"}, Reuse: true, SideEffects: "none"},
		ResourceRequirements: map[string]any{"network": "none"},
		InputSchema:          []byte("{\"type\":\"object\"}"), OutputSchema: []byte("{\"type\":\"object\"}"),
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
	if catalog := BuiltinCatalog(); len(catalog) != 1 || catalog["movie_recap"].Version != 5 {
		t.Fatalf("native catalog is not registered: %#v", catalog)
	}
}

func TestMovieRecapCriticalSemanticsAreExact(t *testing.T) {
	definition := MovieRecap()
	nodes := definition.NodeMap()
	dependency := func(nodeKey, dependencyKey string) Dependency {
		for _, value := range nodes[nodeKey].DependsOn {
			if value.NodeKey == dependencyKey {
				return value
			}
		}
		t.Fatalf("%s is missing dependency %s", nodeKey, dependencyKey)
		return Dependency{}
	}
	if value := dependency("generate_subtitle", "align_audio"); !value.Required || value.Condition != "success_or_declared_soft" {
		t.Fatalf("generate_subtitle must require alignment: %+v", value)
	}
	if value := dependency("generate_subtitle", "translate_subtitles"); value.Required || value.Condition != "soft" {
		t.Fatalf("translation must be optional/soft: %+v", value)
	}
	for _, value := range nodes["generate_subtitle"].DependsOn {
		if value.NodeKey == "generate_script" {
			t.Fatal("generate_subtitle regressed to a direct generate_script dependency")
		}
	}
	if nodes["export_clips"].ExecutionClass != "render" {
		t.Fatalf("export_clips must use render execution class, got %q", nodes["export_clips"].ExecutionClass)
	}
	assertRoles := func(nodeKey string, required, produced []string) {
		node := nodes[nodeKey]
		if len(node.RequiredArtifactRoles) != len(required) || len(node.ProducedArtifactRoles) != len(produced) {
			t.Fatalf("%s artifact roles drifted: required=%v produced=%v", nodeKey, node.RequiredArtifactRoles, node.ProducedArtifactRoles)
		}
		for index, role := range required {
			if node.RequiredArtifactRoles[index] != role {
				t.Fatalf("%s required role %d: got %q want %q", nodeKey, index, node.RequiredArtifactRoles[index], role)
			}
		}
		for index, role := range produced {
			if node.ProducedArtifactRoles[index] != role {
				t.Fatalf("%s produced role %d: got %q want %q", nodeKey, index, node.ProducedArtifactRoles[index], role)
			}
		}
	}
	assertRoles("align_audio", []string{"narration_audio"}, []string{"timing_alignment"})
	assertRoles("generate_subtitle", []string{"timing_alignment"}, []string{"subtitle_cues", "subtitle_srt", "subtitle_vtt", "subtitle_ass"})
	assertRoles("export_clips", []string{"render_video"}, []string{"clip_exports", "clip_manifest"})
}

func TestMovieRecapCatalogMatchesCanonicalNodePolicies(t *testing.T) {
	definition := MovieRecap()
	nodes := definition.NodeMap()
	type expectedNode struct {
		class, failure, review string
		deps                   map[string]Dependency
	}
	required := func(keys ...string) map[string]Dependency {
		result := map[string]Dependency{}
		for _, key := range keys {
			result[key] = Dependency{NodeKey: key, Required: true, Condition: "success_or_declared_soft"}
		}
		return result
	}
	optional := func(result map[string]Dependency, keys ...string) map[string]Dependency {
		for _, key := range keys {
			result[key] = Dependency{NodeKey: key, Required: false, Condition: "soft"}
		}
		return result
	}
	expected := map[string]expectedNode{
		"resolve_source_asset":      {"probe", "hard", "none", required()},
		"prepare_media_assets":      {"media", "hard", "none", required("resolve_source_asset")},
		"research_metadata":         {"ai", "soft", "none", required("resolve_source_asset")},
		"generate_script":           {"ai", "hard", "none", required("research_metadata")},
		"review_script":             {"system", "hard", "approval_completes_node", required("generate_script")},
		"generate_narration":        {"ai", "hard", "none", required("review_script")},
		"align_audio":               {"ml", "soft", "none", required("generate_narration")},
		"detect_scenes":             {"ml", "soft", "none", required("prepare_media_assets")},
		"extract_transcript":        {"ml", "soft", "none", required("prepare_media_assets")},
		"analyze_scenes":            {"ai", "soft", "none", optional(required("detect_scenes"), "extract_transcript")},
		"detect_characters":         {"ml", "soft", "none", required("detect_scenes")},
		"embed_media":               {"ml", "soft", "none", optional(required("analyze_scenes"), "generate_script", "detect_characters")},
		"generate_match_candidates": {"ml", "hard", "none", optional(required("generate_script", "detect_scenes", "align_audio"), "extract_transcript", "analyze_scenes", "detect_characters", "embed_media")},
		"evaluate_candidates":       {"system", "hard", "none", required("generate_match_candidates")},
		"select_candidate":          {"system", "hard", "none", required("evaluate_candidates")},
		"coverage_feedback":         {"ai", "soft", "none", required("select_candidate")},
		"translate_subtitles":       {"ai", "soft", "none", required("align_audio")},
		"generate_subtitle":         {"system", "hard", "none", optional(required("align_audio"), "translate_subtitles")},
		"build_timeline":            {"system", "hard", "none", required("select_candidate", "generate_narration", "generate_subtitle")},
		"review_timeline":           {"system", "hard", "approval_completes_node", required("build_timeline")},
		"mix_audio":                 {"media", "hard", "none", required("build_timeline", "review_timeline")},
		"run_qa_gate":               {"system", "hard", "none", required("build_timeline", "mix_audio")},
		"render_timeline":           {"render", "hard", "none", required("review_timeline", "run_qa_gate")},
		"validate_deliverable":      {"probe", "hard", "none", required("render_timeline")},
		"export_clips":              {"render", "soft", "none", required("validate_deliverable")},
	}
	if len(nodes) != len(expected) {
		t.Fatalf("canonical movie_recap node count drifted: got %d want %d", len(nodes), len(expected))
	}
	for key, want := range expected {
		node, ok := nodes[key]
		if !ok {
			t.Fatalf("canonical movie_recap node %q is missing", key)
		}
		if node.ExecutionClass != want.class || node.FailureMode != want.failure || node.ReviewPolicy != want.review {
			t.Fatalf("node %s policy drifted: class=%q failure=%q review=%q", key, node.ExecutionClass, node.FailureMode, node.ReviewPolicy)
		}
		gotDeps := map[string]Dependency{}
		for _, dependency := range node.DependsOn {
			gotDeps[dependency.NodeKey] = dependency
		}
		if !reflect.DeepEqual(gotDeps, want.deps) {
			t.Fatalf("node %s dependencies drifted: got=%v want=%v", key, gotDeps, want.deps)
		}
		for direction, raw := range map[string]json.RawMessage{"input": node.InputSchema, "output": node.OutputSchema} {
			if string(raw) == `{"type":"object"}` {
				t.Fatalf("node %s has an unbounded generic %s schema", key, direction)
			}
			var schema map[string]any
			if err := json.Unmarshal(raw, &schema); err != nil {
				t.Fatalf("node %s %s schema is invalid: %v", key, direction, err)
			}
			if schema["additionalProperties"] != false {
				t.Fatalf("node %s %s schema must close additional properties", key, direction)
			}
		}
	}
	var subtitleInput map[string]any
	if err := json.Unmarshal(nodes["generate_subtitle"].InputSchema, &subtitleInput); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(subtitleInput["required"])
	if string(encoded) != `["timing_alignment_artifact_id","script_version_id"]` {
		t.Fatalf("subtitle input must require alignment and script version: %s", encoded)
	}
	if _, ok := subtitleInput["properties"].(map[string]any)["translated_subtitles_artifact_id"]; !ok {
		t.Fatal("subtitle input lost optional translated subtitle reference")
	}
	if got := nodes["export_clips"].Retry.MaxAttempts; got != 2 {
		t.Fatalf("export_clips must retain P-RENDER retry budget, got %d", got)
	}
	if got := nodes["export_clips"].Timeout.WallTimeSec; got != 14400 {
		t.Fatalf("export_clips must retain P-RENDER wall timeout, got %d", got)
	}
	if err := definition.Validate(AllCapabilities()); err != nil {
		t.Fatalf("canonical graph no longer validates: %v", err)
	}
}

func TestValidatorFailsClosedWhenNativeArtifactRoleIsMissing(t *testing.T) {
	node := testNode("align_audio", "narration/v1", "alignment/v1")
	node.Type = "align_audio"
	definition := testDefinition([]Node{node})
	if err := definition.Validate(AllCapabilities()); err == nil {
		t.Fatal("native artifact-bearing node without required roles was accepted")
	}
	node = testNode("review_script", "script/v1", "script/v1")
	node.Type = "review_script"
	node.ProducedArtifactRoles = []string{"invented_role"}
	definition = testDefinition([]Node{node})
	if err := definition.Validate(AllCapabilities()); err == nil {
		t.Fatal("native node with an undeclared artifact role was accepted")
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
	retryPolicy := base
	retryPolicy.Nodes[0].Retry.MaxAttempts = 2
	retryPolicy.Nodes[0].Retry.RetryableCategories = nil
	if err := retryPolicy.Validate(AllCapabilities()); err == nil {
		t.Fatal("retry policy without categories accepted")
	}
}

func TestValidatorRejectsMalformedSchemasAndUnsupportedDependencyCondition(t *testing.T) {
	node := testNode("a_node", "context/v1", "context/v1")
	definition := testDefinition([]Node{node})
	definition.Nodes[0].InputSchema = []byte(`{"type":`)
	if err := definition.Validate(AllCapabilities()); err == nil {
		t.Fatal("malformed JSON schema accepted")
	}
	definition = testDefinition([]Node{testNode("a_node", "context/v1", "context/v1")})
	definition.Nodes[0].OutputSchema = []byte(`{"type":"not-a-json-type"}`)
	if err := definition.Validate(AllCapabilities()); err == nil {
		t.Fatal("unsupported JSON schema type accepted")
	}
	definition = testDefinition([]Node{testNode("a_node", "context/v1", "context/v1")})
	definition.Nodes[0].InputSchema = []byte(`{"$ref":7}`)
	if err := definition.Validate(AllCapabilities()); err == nil {
		t.Fatal("malformed JSON schema reference accepted")
	}
	first, second := testNode("a_node", "context/v1", "context/v1"), testNode("b_node", "context/v1", "context/v1")
	second.DependsOn = []Dependency{{NodeKey: first.Key, Required: true, Condition: "upstream_maybe"}}
	definition = testDefinition([]Node{first, second})
	if err := definition.Validate(AllCapabilities()); err == nil {
		t.Fatal("unsupported dependency condition accepted")
	}
}

func TestRuntimeCoversRetryReviewSoftFailureStopAfterCancelAndIdempotency(t *testing.T) {
	nodes := []Node{testNode("branch_a", "context/v1", "context/v1"), testNode("branch_b", "context/v1", "context/v1"), testNode("join_node", "context/v1", "context/v1")}
	nodes[0].Retry.MaxAttempts = 2
	nodes[0].Retry.RetryableCategories = []string{"transient"}
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

func TestRuntimeRequiredSoftDependencyContinuesOnlyForDeclaredSoftFailure(t *testing.T) {
	upstream := testNode("upstream", "context/v1", "context/v1")
	upstream.FailureMode, upstream.Soft = "soft", true
	downstream := testNode("downstream", "context/v1", "context/v1")
	downstream.DependsOn = []Dependency{{NodeKey: upstream.Key, Required: true, Condition: "success_or_declared_soft"}}
	definition := testDefinition([]Node{upstream, downstream})
	runtime := &Runtime{Definition: definition, Executors: map[string]Executor{"fake": &DeterministicExecutor{Results: map[string][]Result{
		"upstream": {{Outcome: Failed, Error: &NodeError{Category: "permanent"}}},
	}}}}
	result := runtime.Run(context.Background(), Request{JobID: "soft-job", RunID: "soft-run"})
	if result.Outcome != Completed || len(result.Nodes) != 2 || result.Nodes[1].Key != "downstream" {
		t.Fatalf("declared soft failure did not continue: %+v", result)
	}

	hard := testNode("upstream", "context/v1", "context/v1")
	hard.FailureMode, hard.Soft = "hard", false
	downstream = testNode("downstream", "context/v1", "context/v1")
	downstream.DependsOn = []Dependency{{NodeKey: hard.Key, Required: true, Condition: "success_or_declared_soft"}}
	definition = testDefinition([]Node{hard, downstream})
	runtime = &Runtime{Definition: definition, Executors: map[string]Executor{"fake": &DeterministicExecutor{Results: map[string][]Result{
		"upstream": {{Outcome: Failed, Error: &NodeError{Category: "permanent"}}},
	}}}}
	result = runtime.Run(context.Background(), Request{JobID: "hard-job", RunID: "hard-run"})
	if result.Outcome != Failed || len(result.Nodes) != 1 {
		t.Fatalf("hard failure incorrectly continued: %+v", result)
	}
}

func TestRuntimeRejectsUnsupportedExecutorOutcome(t *testing.T) {
	node := testNode("bad_result", "context/v1", "context/v1")
	definition := testDefinition([]Node{node})
	runtime := &Runtime{Definition: definition, Executors: map[string]Executor{"fake": &DeterministicExecutor{Results: map[string][]Result{
		"bad_result": {{Outcome: Outcome("not_supported")}},
	}}}}
	result := runtime.Run(context.Background(), Request{JobID: "bad-job", RunID: "bad-run"})
	if result.Outcome != Failed || len(result.Nodes) != 1 || result.Nodes[0].Error == nil {
		t.Fatalf("unsupported executor outcome was not failed closed: %+v", result)
	}
}

type blockingExecutor struct{}

func (blockingExecutor) Execute(ctx context.Context, _ Node, _ State) Result {
	<-ctx.Done()
	return Result{Outcome: Failed, Error: &NodeError{Category: "transient", Retryable: true}}
}

func TestRuntimeFailsClosedOnIdleBudgetWhenExecutorDoesNotHeartbeat(t *testing.T) {
	node := testNode("blocked", "context/v1", "context/v1")
	node.Timeout.WallTimeSec = 2
	node.Timeout.IdleTimeSec = 1
	definition := testDefinition([]Node{node})
	result := (&Runtime{Definition: definition, Executors: map[string]Executor{"fake": blockingExecutor{}}}).Run(context.Background(), Request{})
	if result.Outcome != Failed || len(result.Nodes) != 1 || result.Nodes[0].Error == nil || result.Nodes[0].Error.Category != "timeout" {
		t.Fatalf("idle timeout was not fail-closed: %+v", result)
	}
}
