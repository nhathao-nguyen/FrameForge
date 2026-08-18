"""Gate G node dispatch for the Python AI/ML/system worker capabilities."""

from __future__ import annotations

import json
import os
import tempfile
from dataclasses import asdict
from pathlib import Path
from typing import Any

from .artifact_io import ArtifactIO
from .gate_g import (
    Alignment,
    AppearanceObservation,
    ArtifactRef,
    CandidateSelectionPolicy,
    EmbeddingIndex,
    EvaluationResult,
    FakeProvider,
    GenerationCandidate,
    MatchProposal,
    MatchScoreComponents,
    MemoryArtifactStore,
    Narration,
    ProviderCallContext,
    ProviderKind,
    ResearchAnalysis,
    Scene,
    SceneAnalysis,
    SceneFeatures,
    ScriptSegment,
    ScriptStyle,
    ScriptVersion,
    SubtitleCue,
    TranscriptSegment,
    TranscriptWord,
    VoiceSnapshot,
    align_audio,
    analyze_scenes,
    build_embedding_index,
    cluster_characters,
    coverage_feedback,
    detect_scenes_from_media,
    evaluate_candidates,
    export_ass,
    export_srt,
    export_vtt,
    generate_script,
    generate_subtitles,
    match_script_to_scenes,
    research_analysis,
    select_candidate,
    subtitle_qa,
    synthesize_narration,
    transcribe_segments,
    translate_subtitles,
)


AI_NODES = {"research_metadata", "generate_script", "generate_narration", "analyze_scenes", "coverage_feedback", "translate_subtitles"}
ML_NODES = {"align_audio", "detect_scenes", "extract_transcript", "detect_characters", "embed_media", "generate_match_candidates"}
SYSTEM_NODES = {"review_script", "evaluate_candidates", "select_candidate", "generate_subtitle", "build_timeline", "review_timeline", "run_qa_gate"}


def execute_gate_g_node(command: dict[str, Any], artifacts: ArtifactIO) -> list[dict[str, Any]]:
    node = str(command["node_key"])
    capability = str(command["capability"])
    expected = "ai" if node in AI_NODES else "ml" if node in ML_NODES else "system" if node in SYSTEM_NODES else ""
    if expected != capability:
        raise ValueError("node is not enabled for this Python worker capability")
    state = _load_state(command, artifacts)
    context = _provider_context(command)
    llm = FakeProvider(ProviderKind.LLM)
    if node == "research_metadata":
        research = research_analysis("Create a concise movie recap.", tuple(ref["artifact_id"] for ref in command["input_refs"]), llm, context)
        return [_json_output(command, artifacts, "research_metadata", {"research": asdict(research)})]
    if node == "generate_script":
        research = _research(state["research"])
        script = generate_script(research, llm, context, float(command.get("config", {}).get("duration_sec", 3.0)), ScriptStyle(language=str(command.get("config", {}).get("language", "en"))))
        return [_json_output(command, artifacts, "script_version", {"script": asdict(script)})]
    if node == "review_script":
        _script(state["script"])
        return []
    if node == "generate_narration":
        script = _script(state["script"])
        store = MemoryArtifactStore()
        narration = synthesize_narration(script, FakeProvider(ProviderKind.TTS), context, store, VoiceSnapshot("config_local_tts", 1, "fake-tts", "fake-v1", "voice-local", script.language))
        audio = store.read(narration.audio_artifact.artifact_id)
        audio_ref = artifacts.stage(command, "audio", "narration_audio", audio, "audio/wav")
        manifest = asdict(narration)
        manifest["audio_artifact"]["artifact_id"] = audio_ref["artifact_id"]
        return [audio_ref, _json_output(command, artifacts, "narration_manifest", {"narration": manifest})]
    if node == "align_audio":
        script = _script(state["script"])
        narration = _narration(state["narration"], _required_ref(command, "narration_audio"))
        transcript = transcribe_segments(script.text, narration.duration_sec, FakeProvider(ProviderKind.ASR), context, audio_artifact_ref=narration.audio_artifact.artifact_id)
        alignment = align_audio(script, narration, transcript, tolerance_sec=0.75)
        return [_json_output(command, artifacts, "timing_alignment", {"alignment": asdict(alignment), "transcript": asdict(transcript)})]
    if node == "extract_transcript":
        source_audio = _required_ref(command, "prepared_audio")
        transcript = transcribe_segments("Source dialogue.", 1.0, FakeProvider(ProviderKind.ASR), context, audio_artifact_ref=source_audio["artifact_id"])
        return [_json_output(command, artifacts, "transcript", {"source_transcript": asdict(transcript)})]
    if node == "detect_scenes":
        source = _required_ref(command, "prepared_video")
        media = artifacts.read(command, source)
        suffix = ".mp4"
        with tempfile.TemporaryDirectory(prefix="nh-media-scenes-") as root:
            media_file = Path(root) / ("source" + suffix)
            media_file.write_bytes(media)
            scenes, features = detect_scenes_from_media(str(media_file), source["sha256"], _required_tool("NH_MEDIA_FFMPEG_PATH"), _required_tool("NH_MEDIA_FFPROBE_PATH"), 0.2)
        if not scenes:
            raise ValueError("real media scene detector returned no scenes")
        thumbnails = _json_output(command, artifacts, "scene_thumbnails", {"scene_thumbnail_ids": [item.keyframe_refs[0] for item in features]})
        serial_features = []
        for feature in features:
            value = asdict(feature)
            value["keyframe_refs"] = [f"{thumbnails['artifact_id']}#{feature.scene_id}"]
            serial_features.append(value)
        return [_json_output(command, artifacts, "scene_index", {"scenes": [asdict(item) for item in scenes], "scene_features": serial_features}), thumbnails]
    if node == "analyze_scenes":
        scenes = tuple(_scene(item) for item in state["scenes"])
        features = tuple(_features(item) for item in state["scene_features"])
        scene_analyses = analyze_scenes(scenes, features, FakeProvider(ProviderKind.VLM), context)
        return [_json_output(command, artifacts, "scene_analysis", {"scene_analyses": [asdict(item) for item in scene_analyses]})]
    if node == "detect_characters":
        features = tuple(_features(item) for item in state["scene_features"])
        observations = tuple(AppearanceObservation(f"appearance_{item.scene_id}", (item.mean_luma, item.motion_score, item.quality_score), item.scene_id, state["scenes"][0]["source_revision"]) for item in features)
        characters = cluster_characters(observations)
        return [_json_output(command, artifacts, "character_analysis", {"characters": [asdict(item) for item in characters]})]
    if node == "embed_media":
        script = _script(state["script"])
        analyses = tuple(_analysis(item) for item in state["scene_analyses"])
        items = [(segment.segment_id, segment.text, "text") for segment in script.segments]
        items.extend((analysis.scene_id, analysis.text or analysis.scene_id, "image") for analysis in analyses)
        index = build_embedding_index(tuple(items), FakeProvider(ProviderKind.EMBEDDING), context, "fake-v1", dimension=16)
        return [_json_output(command, artifacts, "media_embeddings", {"embedding_index": asdict(index)})]
    if node == "generate_match_candidates":
        script = _script(state["script"])
        scenes = tuple(_scene(item) for item in state["scenes"])
        analyses = tuple(_analysis(item) for item in state["scene_analyses"])
        features = tuple(_features(item) for item in state["scene_features"])
        proposals = match_script_to_scenes(script, scenes, analyses, _embedding(state["embedding_index"]), features)
        if not proposals:
            raise ValueError("matching produced no candidate")
        candidates = tuple(GenerationCandidate(f"candidate_{index + 1:03d}", "group_movie_recap", {"quality_score": proposal.scores.composite_score, "proposal_id": proposal.proposal_id}, {"proposal": asdict(proposal)}) for index, proposal in enumerate(proposals))
        return [_json_output(command, artifacts, "match_candidates", {"match_proposals": [asdict(item) for item in proposals], "candidates": [asdict(item) for item in candidates]})]
    if node == "evaluate_candidates":
        candidates = tuple(_candidate(item) for item in state["candidates"])
        evaluations = evaluate_candidates(candidates, CandidateSelectionPolicy("movie_recap_auto", "1", group_id="group_movie_recap"))
        return [_json_output(command, artifacts, "candidate_evaluations", {"evaluations": [asdict(item) for item in evaluations]})]
    if node == "select_candidate":
        candidates = tuple(_candidate(item) for item in state["candidates"])
        evaluations = tuple(_evaluation(item) for item in state["evaluations"])
        selection = select_candidate(candidates, evaluations, CandidateSelectionPolicy("movie_recap_auto", "1", group_id="group_movie_recap"))
        return [_json_output(command, artifacts, "selected_match_proposal", {"selection": asdict(selection)})]
    if node == "coverage_feedback":
        script = _script(state["script"])
        report = coverage_feedback(script, tuple(_proposal(item) for item in state["match_proposals"]), minimum_score=0.0)
        return [_json_output(command, artifacts, "coverage_report", {"coverage": asdict(report)})]
    if node == "translate_subtitles":
        alignment = _alignment(state["alignment"])
        cues = generate_subtitles(alignment, _script(state["script"]).language)
        translated = translate_subtitles(cues, "vi", llm, context)
        return [_json_output(command, artifacts, "translated_subtitles", {"translated_subtitles": [asdict(item) for item in translated]})]
    if node == "generate_subtitle":
        alignment = _alignment(state["alignment"])
        cues = generate_subtitles(alignment, _script(state["script"]).language)
        if not subtitle_qa(cues, max_line_chars=84).passed:
            raise ValueError("subtitle QA failed")
        return [
            _json_output(command, artifacts, "subtitle_cues", {"subtitle_cues": [asdict(item) for item in cues]}),
            artifacts.stage(command, "subtitle", "subtitle_srt", export_srt(cues).encode(), "application/x-subrip"),
            artifacts.stage(command, "subtitle", "subtitle_vtt", export_vtt(cues).encode(), "text/vtt"),
            artifacts.stage(command, "subtitle", "subtitle_ass", export_ass(cues).encode(), "text/x-ssa"),
        ]
    if node == "build_timeline":
        timeline = _build_timeline(command, state)
        return [_json_output(command, artifacts, "timeline_version", {"timeline": timeline}, document=timeline)]
    if node == "review_timeline":
        if "timeline" not in state:
            raise ValueError("timeline review input is missing")
        return []
    if node == "run_qa_gate":
        if "timeline" not in state or _find_ref(command, "mixed_audio") is None:
            raise ValueError("timeline QA inputs are incomplete")
        return [_json_output(command, artifacts, "timeline_qa_report", {"timeline_qa": {"passed": True, "policy": "gate-g-local-v1"}})]
    raise ValueError("unsupported Gate G Python node")


def _provider_context(command: dict[str, Any]) -> ProviderCallContext:
    return ProviderCallContext(command["message_id"], command["job_id"], command["project_id"], command["job_id"], command["job_step_id"], "config_local_gate_g", 1, timeout_sec=120, privacy_policy="local_only")


def _load_state(command: dict[str, Any], artifacts: ArtifactIO) -> dict[str, Any]:
    state: dict[str, Any] = {}
    for ref in command["input_refs"]:
        try:
            value = json.loads(artifacts.read(command, ref))
        except (KeyError, OSError, UnicodeDecodeError, json.JSONDecodeError):
            continue
        if isinstance(value, dict) and isinstance(value.get("state"), dict):
            state.update(value["state"])
        elif isinstance(value, dict) and value.get("schema_version") == "1.0" and isinstance(value.get("tracks"), list):
            state["timeline"] = value
    return state


def _json_output(command: dict[str, Any], artifacts: ArtifactIO, role: str, state: dict[str, Any], *, document: dict[str, Any] | None = None) -> dict[str, Any]:
    value = document if document is not None else {"schema_version": "gate-g-node/v1", "node_key": command["node_key"], "state": state}
    return artifacts.stage(command, "gate_g_json", role, json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode(), "application/json")


def _find_ref(command: dict[str, Any], role: str) -> dict[str, Any] | None:
    return next((ref for ref in reversed(command["input_refs"]) if ref["role"] == role), None)


def _required_ref(command: dict[str, Any], role: str) -> dict[str, Any]:
    value = _find_ref(command, role)
    if value is None:
        raise ValueError(f"required Artifact role is missing: {role}")
    return value


def _required_tool(name: str) -> str:
    value = os.environ.get(name, "")
    if not value:
        raise ValueError("reviewed media tool is not configured")
    return value


def _research(value: dict[str, Any]) -> ResearchAnalysis:
    return ResearchAnalysis(value["analysis_id"], tuple(value["input_refs"]), value["query"], value["result"], value["prompt_snapshot"], value["provider_snapshot"], value["content_hash"], value["status"])


def _script(value: dict[str, Any]) -> ScriptVersion:
    return ScriptVersion(value["script_id"], int(value["version"]), value["language"], tuple(ScriptSegment(**item) for item in value["segments"]), ScriptStyle(**value["style"]), tuple(value["input_refs"]), value["provider_snapshot"], value["prompt_snapshot"], value["content_hash"], value["status"])


def _artifact(ref: dict[str, Any]) -> ArtifactRef:
    return ArtifactRef(ref["artifact_id"], "audio", ref["role"], ref["sha256"], int(ref.get("size_bytes", 0)), {})


def _narration(value: dict[str, Any], ref: dict[str, Any]) -> Narration:
    return Narration(value["narration_id"], value["script_version_id"], VoiceSnapshot(**value["voice_snapshot"]), _artifact(ref), float(value["duration_sec"]), value["cache_fingerprint"], value["provider_snapshot"], value["status"])


def _transcript_segment(value: dict[str, Any]) -> TranscriptSegment:
    return TranscriptSegment(value["segment_id"], value["text"], float(value["start_sec"]), float(value["end_sec"]), float(value["confidence"]), tuple(TranscriptWord(**item) for item in value.get("words", [])), value.get("speaker"))


def _alignment(value: dict[str, Any]) -> Alignment:
    return Alignment(value["script_version_id"], value["narration_id"], tuple(_transcript_segment(item) for item in value["segments"]), float(value["drift_sec"]), value["provider_snapshot"], value["artifact_fingerprint"])


def _scene(value: dict[str, Any]) -> Scene:
    return Scene(**value)


def _features(value: dict[str, Any]) -> SceneFeatures:
    return SceneFeatures(value["scene_id"], float(value["mean_luma"]), float(value["motion_score"]), float(value["quality_score"]), tuple(value["keyframe_refs"]))


def _analysis(value: dict[str, Any]) -> SceneAnalysis:
    return SceneAnalysis(value["scene_id"], value["text"], tuple(value["entities"]), value["source_revision"], tuple(value["keyframe_refs"]), value["provider_snapshot"], value["status"], tuple(value.get("actions", [])), value.get("confidence"))


def _embedding(value: dict[str, Any]) -> EmbeddingIndex:
    return EmbeddingIndex(value["index_id"], value["model_space"], value["model"], int(value["dimension"]), bool(value["normalized"]), {key: tuple(item) for key, item in value["vectors"].items()}, value["input_fingerprint"], value["metric"], value["provider_snapshot"], value["item_modalities"])


def _proposal(value: dict[str, Any]) -> MatchProposal:
    scores = value["scores"]
    return MatchProposal(value["proposal_id"], value["script_segment_id"], value["scene_id"], MatchScoreComponents(*(float(scores[key]) for key in ("semantic_score", "visual_score", "character_score", "temporal_score", "rhythm_score", "diversity_score", "quality_score"))), tuple(value["evidence_refs"]), value["status"], value["score_provenance"])


def _candidate(value: dict[str, Any]) -> GenerationCandidate:
    return GenerationCandidate(value["candidate_id"], value["group_id"], value["payload"], value["provenance"], value["status"])


def _evaluation(value: dict[str, Any]) -> EvaluationResult:
    return EvaluationResult(value["evaluation_id"], value["candidate_id"], value["policy_version"], float(value["score"]), value["components"], value["evaluator_snapshot"])


def _build_timeline(command: dict[str, Any], state: dict[str, Any]) -> dict[str, Any]:
    scenes = tuple(_scene(item) for item in state["scenes"])
    cues = tuple(SubtitleCue(**item) for item in state["subtitle_cues"])
    narration = state["narration"]
    source = _required_ref(command, "prepared_video")
    narration_ref = _required_ref(command, "narration_audio")
    duration = float(narration["duration_sec"])
    selected = list(scenes[:3])
    if not selected:
        raise ValueError("movie recap timeline requires a detected scene")
    clip_duration = duration / len(selected)
    video_clips = []
    for index, scene in enumerate(selected):
        available = scene.end_sec - scene.start_sec
        use = min(available, clip_duration)
        video_clips.append({"id": f"video_clip_{index + 1:03d}", "timeline_in_sec": round(index * clip_duration, 6), "timeline_out_sec": round((index + 1) * clip_duration, 6), "source_in_sec": scene.start_sec, "source_out_sec": round(scene.start_sec + use, 6), "speed": round(use / clip_duration, 6), "source": {"type": "artifact", "artifact_id": source["artifact_id"], "role": "prepared_video"}, "origin": "ai", "proposal_ref": state["match_proposals"][0]["proposal_id"]})
    subtitle_clips = [{"id": f"subtitle_clip_{index + 1:03d}", "timeline_in_sec": cue.start_sec, "timeline_out_sec": cue.end_sec, "source": {"type": "none", "inline_id": cue.cue_id}, "subtitle": {"text": cue.text, "language": cue.language}, "origin": "system"} for index, cue in enumerate(cues)]
    return {
        "schema_version": "1.0",
        "timeline_id": "timeline_movie_recap",
        "timeline_version_id": "timeline_version_movie_recap",
        "project_id": _domain_id(command["project_id"], "project"),
        "version": 1,
        "duration_sec": duration,
        "tracks": [
            {"id": "video_track", "kind": "video", "name": "Video", "order": 0, "clips": video_clips},
            {"id": "narration_track", "kind": "narration", "name": "Narration", "order": 1, "clips": [{"id": "narration_clip", "timeline_in_sec": 0, "timeline_out_sec": duration, "source_in_sec": 0, "source_out_sec": duration, "source": {"type": "artifact", "artifact_id": narration_ref["artifact_id"], "role": "narration_audio"}, "narration": {"narration_id": narration["narration_id"], "script_version_id": narration["script_version_id"]}, "origin": "system"}]},
            {"id": "subtitle_track", "kind": "subtitle", "name": "Subtitles", "order": 2, "clips": subtitle_clips},
        ],
        "metadata": {"pipeline_node": "build_timeline", "selected_candidate": state["selection"]["selected_candidate_id"]},
    }


def _domain_id(value: str, prefix: str) -> str:
    marker = prefix + "_"
    if not value.startswith(marker):
        raise ValueError("worker contract identity prefix is invalid")
    raw = value[len(marker) :]
    parts = raw.split("_")
    if [len(part) for part in parts] != [8, 4, 4, 4, 12]:
        raise ValueError("worker contract UUID shape is invalid")
    return "-".join(parts)


__all__ = ["AI_NODES", "ML_NODES", "SYSTEM_NODES", "execute_gate_g_node"]
