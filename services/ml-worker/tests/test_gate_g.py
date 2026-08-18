from __future__ import annotations

from nh_media.gate_g import (
    AudioMixPolicy,
    BGMTrack,
    CandidateSelectionPolicy,
    FakeProvider,
    GenerationCandidate,
    MemoryArtifactStore,
    ProviderCallContext,
    ProviderError,
    ProviderKind,
    ProviderRegistry,
    ScriptStyle,
    analyze_reference_style,
    analyze_scenes,
    artifact_manifest,
    auto_reframe,
    bilingual_subtitles,
    build_embedding_index,
    cluster_characters,
    coverage_feedback,
    detect_scenes,
    evaluate_candidates,
    export_ass,
    export_srt,
    export_vtt,
    extract_scene_features,
    filter_scenes,
    generate_script,
    generate_subtitles,
    match_script_to_scenes,
    mix_audio,
    provider_conformance,
    research_analysis,
    resolve_with_fallback,
    select_candidate,
    subtitle_qa,
    synthesize_narration,
    transcribe_segments,
    translate_subtitles,
    align_audio,
    VoiceSnapshot,
)


def context(cancelled: bool = False) -> ProviderCallContext:
    return ProviderCallContext(
        "request_gate_g",
        "correlation_gate_g",
        "project_gate_g",
        "job_gate_g",
        "step_gate_g",
        "provider_config_gate_g",
        1,
        credential_ref="secret-ref-gate-g",
        cancelled=lambda: cancelled,
    )


def test_provider_conformance_and_fallback_are_allowlisted() -> None:
    first = FakeProvider(ProviderKind.LLM, "fake-llm-fails", fail_code="provider_unavailable")
    second = FakeProvider(ProviderKind.LLM, "fake-llm")
    registry = ProviderRegistry(
        [first, second, *(FakeProvider(kind) for kind in (ProviderKind.VLM, ProviderKind.TTS, ProviderKind.ASR, ProviderKind.EMBEDDING))]
    )
    evidence = provider_conformance(registry)
    assert set(evidence) == {kind.value for kind in ProviderKind}
    response = resolve_with_fallback(registry, ProviderKind.LLM, [first.descriptor.adapter_key, second.descriptor.adapter_key], {"untrusted_input": {"title": "fallback"}}, context())
    assert response.meta.adapter_key == "fake-llm"
    try:
        registry.resolve(ProviderKind.LLM, "not-allowlisted")
    except ProviderError as error:
        assert error.cause_category == "policy"
    else:
        raise AssertionError("unallowlisted provider was resolved")


def test_provider_timeout_cancel_and_credential_scope_are_safe() -> None:
    timeout_provider = FakeProvider(ProviderKind.LLM, delay_sec=1.0)
    try:
        timeout_provider.call({"untrusted_input": {}}, ProviderCallContext("r", "c", "p", "j", "s", "cfg", 1, timeout_sec=0.01))
    except ProviderError as error:
        assert error.code == "timeout" and error.retryable is True
    else:
        raise AssertionError("provider timeout was not classified")
    try:
        FakeProvider(ProviderKind.LLM).call({"untrusted_input": {}}, context(cancelled=True))
    except ProviderError as error:
        assert error.code == "cancelled" and error.retryable is False
    else:
        raise AssertionError("provider cancellation was not classified")
    assert "secret-value" not in str(context())


def test_native_gate_g_workflow_is_traceable_and_reusable() -> None:
    llm = FakeProvider(ProviderKind.LLM)
    vlm = FakeProvider(ProviderKind.VLM)
    tts = FakeProvider(ProviderKind.TTS)
    asr = FakeProvider(ProviderKind.ASR)
    embedding = FakeProvider(ProviderKind.EMBEDDING)
    call_context = context()

    research = research_analysis("A short deterministic story", ("asset_source_revision_1",), llm, call_context)
    script = generate_script(research, llm, call_context, 5.0, ScriptStyle(perspective="first_person", narrator_control="warm", language="en"))
    store = MemoryArtifactStore()
    narration = synthesize_narration(script, tts, call_context, store, VoiceSnapshot("provider_config_gate_g", 1, "fake-tts", "fake-v1", "voice-1", "en"))
    transcript = transcribe_segments(script.text, narration.duration_sec, asr, call_context)
    alignment = align_audio(script, narration, transcript)
    original = generate_subtitles(alignment)
    translated = translate_subtitles(original, "vi", llm, call_context, {"story": "câu chuyện"})
    bilingual = bilingual_subtitles(original, translated)
    assert subtitle_qa(bilingual, max_line_chars=84).passed
    assert "WEBVTT" in export_vtt(original)
    assert "-->" in export_srt(original)
    assert "[Events]" in export_ass(original)

    scenes = detect_scenes(5.0, "source_revision_1", (1.0, 3.0))
    features = extract_scene_features(scenes)
    filtered, reasons = filter_scenes(scenes, features, exclude_intro_sec=0.5)
    assert len(filtered) == 2
    assert reasons["scene_001"] == ("intro",)
    analyses = analyze_scenes(filtered, features, vlm, call_context)
    characters = cluster_characters((("appearance_1", "alice"), ("appearance_2", "alice")), {"alice": "Alice"})
    assert characters[0].status == "confirmed"
    index = build_embedding_index((("segment_001", script.text, "text"), ("scene_002", analyses[0].text, "image")), embedding, call_context, "fake-space-v1")
    proposals = match_script_to_scenes(script, filtered, analyses, index)
    coverage = coverage_feedback(script, proposals)
    assert coverage.covered_segment_ids == ("segment_001",)
    assert proposals[0].scores.composite_score == round(sum(proposals[0].scores.as_dict()[key] for key in ("semantic_score", "visual_score", "character_score", "temporal_score", "rhythm_score", "diversity_score", "quality_score")) / 7, 6)

    candidates = (
        GenerationCandidate("candidate_a", "group_1", {"quality_score": 0.7}, {"source": "native"}),
        GenerationCandidate("candidate_b", "group_1", {"quality_score": 0.9}, {"source": "native"}),
    )
    policy = CandidateSelectionPolicy("selection_1", "1")
    evaluations = evaluate_candidates(candidates, policy)
    selection = select_candidate(candidates, evaluations, policy)
    assert selection.selected_candidate_id == "candidate_b"
    assert candidates[1].payload["quality_score"] == 0.9
    assert analyze_reference_style("asset_reference_1", "generation_policy", scenes, narration).provenance["copied_footage"] is False
    mix = mix_audio(narration, (BGMTrack("bgm_artifact_1", 10.0, "owned", {"owner": "workspace"}),), AudioMixPolicy())
    assert mix.ducking_applied is True and mix.measured_lufs == -16.0
    assert auto_reframe("shorts_9_16", alignment.artifact_fingerprint, ((0.2, 0.2, 0.2, 0.4),)).mode == "subject_aware"
    manifest = artifact_manifest(tuple(blob for blob in store._blobs.values()))
    assert manifest["schema_version"] == "artifact-manifest/v1"
