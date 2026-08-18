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
		{"embed_media", "embed_media", "ml", "context/v1", "embeddings/v1", []string{"analyze_scenes", "generate_script", "detect_characters"}, "soft", "none"},
		{"generate_match_candidates", "generate_match_candidates", "ml", "context/v1", "candidates/v1", []string{"generate_script", "detect_scenes", "align_audio", "extract_transcript", "analyze_scenes", "detect_characters", "embed_media"}, "hard", "none"},
		{"evaluate_candidates", "evaluate_candidates", "system", "candidates/v1", "evaluations/v1", []string{"generate_match_candidates"}, "hard", "none"},
		{"select_candidate", "select_candidate", "system", "evaluations/v1", "selection/v1", []string{"evaluate_candidates"}, "hard", "none"},
		{"coverage_feedback", "coverage_feedback", "ai", "selection/v1", "coverage/v1", []string{"select_candidate"}, "soft", "none"},
		{"translate_subtitles", "translate_subtitles", "ai", "alignment/v1", "subtitles/v1", []string{"align_audio"}, "soft", "none"},
		{"generate_subtitle", "generate_subtitle", "system", "context/v1", "subtitles/v1", []string{"align_audio", "translate_subtitles"}, "hard", "none"},
		{"build_timeline", "build_timeline", "system", "context/v1", "timeline/v1", []string{"select_candidate", "generate_narration", "generate_subtitle"}, "hard", "none"},
		{"review_timeline", "review_timeline", "system", "timeline/v1", "timeline/v1", []string{"build_timeline"}, "hard", "approval_completes_node"},
		{"mix_audio", "mix_audio", "media", "timeline/v1", "mixed-audio/v1", []string{"build_timeline", "review_timeline"}, "hard", "none"},
		{"run_qa_gate", "run_qa_gate", "system", "context/v1", "qa/v1", []string{"build_timeline", "mix_audio"}, "hard", "none"},
		{"render_timeline", "render_timeline", "render", "context/v1", "rendered/v1", []string{"review_timeline", "run_qa_gate"}, "hard", "none"},
		{"validate_deliverable", "validate_deliverable", "probe", "rendered/v1", "deliverable/v1", []string{"render_timeline"}, "hard", "none"},
		{"export_clips", "export_clips", "render", "deliverable/v1", "export/v1", []string{"validate_deliverable"}, "soft", "none"},
	}
	nodes := make([]Node, 0, len(entries))
	weights := make(map[string]float64, len(entries))
	for _, entry := range entries {
		deps := make([]Dependency, 0, len(entry.deps))
		for _, dep := range entry.deps {
			dependency := Dependency{NodeKey: dep, Required: true, Condition: "success_or_declared_soft"}
			switch {
			case entry.key == "analyze_scenes" && dep == "extract_transcript":
				dependency.Required, dependency.Condition = false, "soft"
			case entry.key == "embed_media" && (dep == "generate_script" || dep == "detect_characters"):
				dependency.Required, dependency.Condition = false, "soft"
			case entry.key == "generate_match_candidates" && (dep == "extract_transcript" || dep == "analyze_scenes" || dep == "detect_characters" || dep == "embed_media"):
				dependency.Required, dependency.Condition = false, "soft"
			case entry.key == "generate_subtitle" && dep == "translate_subtitles":
				dependency.Required, dependency.Condition = false, "soft"
			}
			deps = append(deps, dependency)
		}
		timeout, retry := nativePolicy(entry.class)
		roles := artifactRoleContracts[entry.typ]
		inputSchema, outputSchema := nativeNodeSchemas(entry.key)
		node := Node{
			Key: entry.key, Type: entry.typ, DisplayName: entry.key, DependsOn: deps,
			InputContract: entry.in, OutputContract: entry.out, ExecutionClass: entry.class,
			ResourceRequirements: map[string]any{"network": "deny_by_default", "sandbox": true},
			Timeout:              timeout,
			Retry:                retry,
			Progress:             ProgressPolicy{Unit: "item", TotalSource: "declared_inputs", EmitInterval: 1},
			Checkpoint:           CheckpointPolicy{Mode: "terminal", ResumeCompatibility: "exact_inputs", InvalidationInputs: []string{"input_refs", "config_snapshot"}},
			Idempotency:          IdempotencyPolicy{FingerprintFields: []string{"node_config", "pipeline_version", "input_revisions", "provider_snapshot"}, Reuse: true, SideEffects: "artifact_commit"},
			FailureMode:          entry.failure, Soft: entry.failure == "soft", HumanGate: entry.review != "none", ReviewPolicy: entry.review,
			RequiredArtifactRoles: append([]string{}, roles.required...), ProducedArtifactRoles: append([]string{}, roles.produced...),
			InputSchema: inputSchema, OutputSchema: outputSchema,
		}
		nodes = append(nodes, node)
		weights[entry.key] = 1
	}
	return Definition{
		// Versions 2 and 3 are already durable in developer databases. Keep
		// them immutable and publish the corrected catalog as the next native
		// version instead of mutating an active graph in place.
		SchemaVersion: DefinitionSchemaVersion, WorkflowKey: "movie_recap", Version: 4, Nodes: nodes,
		Policies:   Policies{Strict: false, Checkpoint: true, ArtifactRetention: "protected_until_explicit_delete", ProgressWeights: weights},
		Provenance: Provenance{NHMediaRelease: "0.1.0-gate-f", ContractVersion: "worker/v1"},
	}
}

// nativeNodeSchemas is the executable contract for the built-in catalog. The
// graph's string contracts describe compatibility between nodes; these JSON
// schemas describe the durable refs and values an executor may actually read
// or produce. Keeping them here makes the catalog snapshot self-contained and
// prevents a generic object schema from silently weakening a node boundary.
func nativeNodeSchemas(nodeKey string) (json.RawMessage, json.RawMessage) {
	ref := func() map[string]any { return map[string]any{"type": "string", "minLength": 1} }
	number := func() map[string]any { return map[string]any{"type": "number", "exclusiveMinimum": 0} }
	object := func(required []string, properties map[string]any) json.RawMessage {
		value := map[string]any{"type": "object", "additionalProperties": false, "properties": properties}
		if len(required) > 0 {
			value["required"] = required
		}
		encoded, _ := json.Marshal(value)
		return encoded
	}
	refs := func(names ...string) map[string]any {
		properties := map[string]any{}
		for _, name := range names {
			properties[name] = ref()
		}
		return properties
	}
	input := []string{}
	output := []string{}
	inputProperties := map[string]any{}
	outputProperties := map[string]any{}
	switch nodeKey {
	case "resolve_source_asset":
		input, output = []string{"asset_id"}, []string{"source_ref", "probe_artifact_id"}
		inputProperties, outputProperties = refs("asset_id"), refs("source_ref", "probe_artifact_id")
	case "prepare_media_assets":
		input, output = []string{"source_ref"}, []string{"prepared_video_artifact_id", "prepared_audio_artifact_id", "source_thumbnails_artifact_id"}
		inputProperties, outputProperties = refs("source_ref"), refs("prepared_video_artifact_id", "prepared_audio_artifact_id", "source_thumbnails_artifact_id")
	case "research_metadata":
		input, output = []string{"source_ref"}, []string{"research_artifact_id"}
		inputProperties, outputProperties = refs("source_ref"), refs("research_artifact_id")
	case "generate_script":
		input, output = []string{"language", "duration_sec"}, []string{"script_id", "script_version_id"}
		inputProperties, outputProperties = refs("language", "research_artifact_id"), refs("script_id", "script_version_id")
		inputProperties["duration_sec"] = number()
		inputProperties["style"] = map[string]any{"type": "object", "maxProperties": 100}
	case "review_script":
		input, output = []string{"script_version_id"}, []string{"script_version_id", "review_id"}
		inputProperties, outputProperties = refs("script_version_id", "review_id"), refs("script_version_id", "review_id")
	case "generate_narration":
		input, output = []string{"script_version_id", "voice_snapshot"}, []string{"narration_id", "narration_artifact_id"}
		inputProperties, outputProperties = refs("script_version_id", "voice_snapshot"), refs("narration_id", "narration_artifact_id")
	case "align_audio":
		input, output = []string{"narration_artifact_id", "script_version_id"}, []string{"timing_alignment_artifact_id"}
		inputProperties, outputProperties = refs("narration_artifact_id", "script_version_id"), refs("timing_alignment_artifact_id")
	case "detect_scenes":
		input, output = []string{"prepared_video_artifact_id"}, []string{"scene_index_artifact_id", "scene_thumbnails_artifact_id"}
		inputProperties, outputProperties = refs("prepared_video_artifact_id"), refs("scene_index_artifact_id", "scene_thumbnails_artifact_id")
	case "extract_transcript":
		input, output = []string{"prepared_audio_artifact_id"}, []string{"transcript_artifact_id"}
		inputProperties, outputProperties = refs("prepared_audio_artifact_id"), refs("transcript_artifact_id")
	case "analyze_scenes":
		input, output = []string{"scene_index_artifact_id", "scene_thumbnails_artifact_id"}, []string{"scene_analysis_artifact_id"}
		inputProperties, outputProperties = refs("scene_index_artifact_id", "scene_thumbnails_artifact_id", "transcript_artifact_id"), refs("scene_analysis_artifact_id")
	case "detect_characters":
		input, output = []string{"scene_thumbnails_artifact_id"}, []string{"character_analysis_artifact_id"}
		inputProperties, outputProperties = refs("scene_thumbnails_artifact_id"), refs("character_analysis_artifact_id")
	case "embed_media":
		input, output = []string{"scene_analysis_artifact_id"}, []string{"media_embeddings_artifact_id"}
		inputProperties, outputProperties = refs("scene_analysis_artifact_id", "script_id", "character_analysis_artifact_id"), refs("media_embeddings_artifact_id")
	case "generate_match_candidates":
		input, output = []string{"script_version_id", "scene_index_artifact_id", "timing_alignment_artifact_id"}, []string{"match_candidates_artifact_id"}
		inputProperties, outputProperties = refs("script_version_id", "scene_index_artifact_id", "timing_alignment_artifact_id", "transcript_artifact_id", "scene_analysis_artifact_id", "character_analysis_artifact_id", "media_embeddings_artifact_id"), refs("match_candidates_artifact_id")
	case "evaluate_candidates":
		input, output = []string{"match_candidates_artifact_id", "selection_policy_id"}, []string{"candidate_evaluations_artifact_id"}
		inputProperties, outputProperties = refs("match_candidates_artifact_id", "selection_policy_id"), refs("candidate_evaluations_artifact_id")
	case "select_candidate":
		input, output = []string{"candidate_evaluations_artifact_id"}, []string{"selected_match_proposal_artifact_id"}
		inputProperties, outputProperties = refs("candidate_evaluations_artifact_id", "user_constraints"), refs("selected_match_proposal_artifact_id")
	case "coverage_feedback":
		input, output = []string{"script_version_id", "selected_match_proposal_artifact_id"}, []string{"coverage_report_artifact_id"}
		inputProperties, outputProperties = refs("script_version_id", "selected_match_proposal_artifact_id"), refs("coverage_report_artifact_id")
	case "translate_subtitles":
		input, output = []string{"timing_alignment_artifact_id", "target_language"}, []string{"translated_subtitles_artifact_id"}
		inputProperties, outputProperties = refs("timing_alignment_artifact_id", "target_language"), refs("translated_subtitles_artifact_id")
	case "generate_subtitle":
		input, output = []string{"timing_alignment_artifact_id", "script_version_id"}, []string{"subtitle_cues_artifact_id", "subtitle_srt_artifact_id", "subtitle_vtt_artifact_id", "subtitle_ass_artifact_id"}
		inputProperties, outputProperties = refs("timing_alignment_artifact_id", "script_version_id", "translated_subtitles_artifact_id"), refs("subtitle_cues_artifact_id", "subtitle_srt_artifact_id", "subtitle_vtt_artifact_id", "subtitle_ass_artifact_id")
	case "build_timeline":
		input, output = []string{"script_version_id", "selected_match_proposal_artifact_id", "narration_artifact_id", "subtitle_cues_artifact_id"}, []string{"timeline_version_id"}
		inputProperties, outputProperties = refs("script_version_id", "selected_match_proposal_artifact_id", "narration_artifact_id", "subtitle_cues_artifact_id", "source_refs"), refs("timeline_version_id")
	case "review_timeline":
		input, output = []string{"timeline_version_id"}, []string{"timeline_version_id", "review_id"}
		inputProperties, outputProperties = refs("timeline_version_id", "review_id"), refs("timeline_version_id", "review_id")
	case "mix_audio":
		input, output = []string{"timeline_version_id", "narration_artifact_id"}, []string{"mixed_audio_artifact_id", "loudness_report_artifact_id"}
		inputProperties, outputProperties = refs("timeline_version_id", "narration_artifact_id", "bgm_artifact_ids", "sfx_artifact_ids"), refs("mixed_audio_artifact_id", "loudness_report_artifact_id")
	case "run_qa_gate":
		input, output = []string{"timeline_version_id", "mixed_audio_artifact_id"}, []string{"timeline_qa_report_artifact_id"}
		inputProperties, outputProperties = refs("timeline_version_id", "mixed_audio_artifact_id"), refs("timeline_qa_report_artifact_id")
	case "render_timeline":
		input, output = []string{"timeline_version_id", "render_profile", "mixed_audio_artifact_id"}, []string{"render_video_artifact_id", "render_audio_artifact_id", "render_metadata_artifact_id"}
		inputProperties, outputProperties = refs("timeline_version_id", "render_profile", "mixed_audio_artifact_id", "source_refs"), refs("render_video_artifact_id", "render_audio_artifact_id", "render_metadata_artifact_id")
	case "validate_deliverable":
		input, output = []string{"render_video_artifact_id", "render_profile"}, []string{"deliverable_qa_artifact_id"}
		inputProperties, outputProperties = refs("render_video_artifact_id", "render_audio_artifact_id", "render_metadata_artifact_id", "render_profile"), refs("deliverable_qa_artifact_id")
	case "export_clips":
		input, output = []string{"timeline_version_id", "render_video_artifact_id"}, []string{"clip_exports_artifact_id", "clip_manifest_artifact_id"}
		inputProperties, outputProperties = refs("timeline_version_id", "render_video_artifact_id", "clip_selection"), refs("clip_exports_artifact_id", "clip_manifest_artifact_id")
	default:
		inputProperties, outputProperties = map[string]any{}, map[string]any{}
	}
	return object(input, inputProperties), object(output, outputProperties)
}

func nativePolicy(executionClass string) (TimeoutPolicy, RetryPolicy) {
	switch executionClass {
	case "probe":
		return TimeoutPolicy{WallTimeSec: 600, IdleTimeSec: 120, KillGraceSec: 15}, RetryPolicy{MaxAttempts: 2, BaseDelayMS: 250, MaxDelayMS: 4000, RetryableCategories: []string{"storage_interruption", "worker_lost", "transient"}}
	case "ai":
		return TimeoutPolicy{WallTimeSec: 300, IdleTimeSec: 120, KillGraceSec: 10}, RetryPolicy{MaxAttempts: 3, BaseDelayMS: 250, MaxDelayMS: 4000, RetryableCategories: []string{"timeout", "rate_limit", "provider_unavailable"}}
	case "ml":
		return TimeoutPolicy{WallTimeSec: 3600, IdleTimeSec: 300, KillGraceSec: 30}, RetryPolicy{MaxAttempts: 2, BaseDelayMS: 500, MaxDelayMS: 5000, RetryableCategories: []string{"worker_lost", "transient", "oom_other_capability"}}
	case "media":
		return TimeoutPolicy{WallTimeSec: 1800, IdleTimeSec: 300, KillGraceSec: 30}, RetryPolicy{MaxAttempts: 2, BaseDelayMS: 500, MaxDelayMS: 5000, RetryableCategories: []string{"storage_interruption", "process_interruption", "transient"}}
	case "render":
		return TimeoutPolicy{WallTimeSec: 14400, IdleTimeSec: 600, KillGraceSec: 60}, RetryPolicy{MaxAttempts: 2, BaseDelayMS: 1000, MaxDelayMS: 10000, RetryableCategories: []string{"worker_lost", "storage_interruption", "encoder_interruption", "transient"}}
	default:
		return TimeoutPolicy{WallTimeSec: 120, KillGraceSec: 10}, RetryPolicy{MaxAttempts: 1, BaseDelayMS: 0, MaxDelayMS: 0, RetryableCategories: nil}
	}
}

// BuiltinCatalog is the immutable source for native workflow definitions.
// Persistence may expose a compatible version as active or draft depending on
// the migration stage; the catalog itself never executes a workflow.
func BuiltinCatalog() map[string]Definition {
	return map[string]Definition{"movie_recap": MovieRecap()}
}
