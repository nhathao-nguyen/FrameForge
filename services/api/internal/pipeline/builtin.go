package pipeline

import "encoding/json"

// MovieRecap returns the independently authored native baseline from
// docs/05-PIPELINE-SPEC.md. It contains policy data even when a later node is
// backed by a deterministic fixture rather than a real provider.
func MovieRecap() Definition {
	entries := []struct {
		key, typ, class, in, out string
		deps                     []string
		failure, review          string
	}{
		{"resolve_source_asset", "resolve_source_asset", "probe", "asset/v1", "source/v1", nil, "hard", "none"},
		{"prepare_media_assets", "prepare_media_assets", "media", "source/v1", "prepared-media/v1", []string{"resolve_source_asset"}, "hard", "none"},
		{"research_metadata", "research_metadata", "ai", "source/v1", "research/v1", []string{"resolve_source_asset"}, "soft", "none"},
		{"generate_script", "generate_script", "ai", "context/v1", "script/v1", []string{"research_metadata"}, "hard", "none"},
		{"review_script", "review_script", "system", "script/v1", "script/v1", []string{"generate_script"}, "hard", "approval_completes_node"},
		{"generate_narration", "generate_narration", "ai", "script/v1", "narration/v1", []string{"review_script"}, "hard", "none"},
		{"align_audio", "align_audio", "ml", "narration/v1", "alignment/v1", []string{"generate_narration"}, "soft", "none"},
		{"detect_scenes", "detect_scenes", "ml", "prepared-media/v1", "scenes/v1", []string{"prepare_media_assets"}, "soft", "none"},
		{"extract_transcript", "extract_transcript", "ml", "prepared-media/v1", "transcript/v1", []string{"prepare_media_assets"}, "soft", "none"},
		{"analyze_scenes", "analyze_scenes", "ai", "context/v1", "scene-analysis/v1", []string{"detect_scenes", "extract_transcript"}, "soft", "none"},
		{"detect_characters", "detect_characters", "ml", "scenes/v1", "characters/v1", []string{"detect_scenes"}, "soft", "none"},
		{"embed_media", "embed_media", "ml", "context/v1", "embeddings/v1", []string{"analyze_scenes"}, "soft", "none"},
		{"generate_match_candidates", "generate_match_candidates", "ml", "context/v1", "candidates/v1", []string{"generate_script", "detect_scenes", "align_audio"}, "hard", "none"},
		{"evaluate_candidates", "evaluate_candidates", "system", "candidates/v1", "evaluations/v1", []string{"generate_match_candidates"}, "hard", "none"},
		{"select_candidate", "select_candidate", "system", "evaluations/v1", "selection/v1", []string{"evaluate_candidates"}, "hard", "none"},
		{"coverage_feedback", "coverage_feedback", "ai", "selection/v1", "coverage/v1", []string{"select_candidate"}, "soft", "none"},
		{"translate_subtitles", "translate_subtitles", "ai", "alignment/v1", "subtitles/v1", []string{"align_audio"}, "soft", "none"},
		{"generate_subtitle", "generate_subtitle", "system", "context/v1", "subtitles/v1", []string{"generate_script"}, "hard", "none"},
		{"build_timeline", "build_timeline", "system", "context/v1", "timeline/v1", []string{"select_candidate", "generate_narration", "generate_subtitle"}, "hard", "none"},
		{"review_timeline", "review_timeline", "system", "timeline/v1", "timeline/v1", []string{"build_timeline"}, "hard", "approval_completes_node"},
		{"mix_audio", "mix_audio", "media", "timeline/v1", "mixed-audio/v1", []string{"review_timeline"}, "hard", "none"},
		{"run_qa_gate", "run_qa_gate", "system", "context/v1", "qa/v1", []string{"build_timeline", "mix_audio"}, "hard", "none"},
		{"render_timeline", "render_timeline", "render", "context/v1", "rendered/v1", []string{"review_timeline", "run_qa_gate"}, "hard", "none"},
		{"validate_deliverable", "validate_deliverable", "probe", "rendered/v1", "deliverable/v1", []string{"render_timeline"}, "hard", "none"},
		{"export_clips", "export_clips", "media", "deliverable/v1", "export/v1", []string{"validate_deliverable"}, "soft", "none"},
	}
	nodes := make([]Node, 0, len(entries))
	weights := make(map[string]float64, len(entries))
	for _, entry := range entries {
		deps := make([]Dependency, 0, len(entry.deps))
		for _, dep := range entry.deps {
			deps = append(deps, Dependency{NodeKey: dep, Required: true, Condition: "success_or_declared_soft"})
		}
		node := Node{
			Key: entry.key, Type: entry.typ, DisplayName: entry.key, DependsOn: deps,
			InputContract: entry.in, OutputContract: entry.out, ExecutionClass: entry.class,
			ResourceRequirements: map[string]any{"network": "deny_by_default", "sandbox": true},
			Timeout:              TimeoutPolicy{WallTimeSec: 120, KillGraceSec: 10},
			Retry:                RetryPolicy{MaxAttempts: 1, RetryableCategories: []string{"transient"}},
			Progress:             ProgressPolicy{Unit: "item", TotalSource: "declared_inputs", EmitInterval: 1},
			Checkpoint:           CheckpointPolicy{Mode: "terminal", ResumeCompatibility: "exact_inputs", InvalidationInputs: []string{"input_refs", "config_snapshot"}},
			Idempotency:          IdempotencyPolicy{FingerprintFields: []string{"node_config", "pipeline_version", "input_revisions", "provider_snapshot"}, Reuse: true, SideEffects: "artifact_commit"},
			FailureMode:          entry.failure, Soft: entry.failure == "soft", HumanGate: entry.review != "none", ReviewPolicy: entry.review,
			InputSchema: json.RawMessage("{\"type\":\"object\"}"), OutputSchema: json.RawMessage("{\"type\":\"object\"}"),
		}
		nodes = append(nodes, node)
		weights[entry.key] = 1
	}
	return Definition{
		SchemaVersion: DefinitionSchemaVersion, WorkflowKey: "movie_recap", Version: 2, Nodes: nodes,
		Policies:   Policies{Strict: false, Checkpoint: true, ArtifactRetention: "protected_until_explicit_delete", ProgressWeights: weights},
		Provenance: Provenance{NHMediaRelease: "0.1.0-gate-f", ContractVersion: "worker/v1"},
	}
}

// BuiltinCatalog is the immutable source for native workflow definitions.
// Persistence may expose a compatible version as active or draft depending on
// the migration stage; the catalog itself never executes a workflow.
func BuiltinCatalog() map[string]Definition {
	return map[string]Definition{"movie_recap": MovieRecap()}
}
