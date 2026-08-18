from __future__ import annotations

import hashlib
import os
import subprocess
import wave
from io import BytesIO
from pathlib import Path

import pytest

from nh_media.gate_g import (
    AppearanceObservation,
    CandidateSelectionPolicy,
    EvaluationResult,
    FakeProvider,
    GenerationCandidate,
    ProviderCallContext,
    ProviderKind,
    ProducedBlob,
    ScriptSegment,
    ScriptStyle,
    ScriptVersion,
    VoiceSnapshot,
    MemoryArtifactStore,
    build_embedding_index,
    cluster_characters,
    detect_scenes_from_media,
    evaluate_candidates,
    select_candidate,
    synthesize_narration,
    transcribe_segments,
)
from nh_media.providers.contracts import (
    EmbeddingResponse,
    EmbeddingVector,
    ProviderTranscript,
    ProviderTranscriptSegment,
    ProviderTranscriptWord,
    TTSResponse,
)


def provider_context() -> ProviderCallContext:
    return ProviderCallContext("request_contract", "corr_contract", "project_contract", "job_contract", "step_contract", "config_contract", 1, credential_ref="opaque-credential-ref")


def wav_fixture(duration_sec: float = 0.25, sample_rate: int = 8000) -> bytes:
    frames = max(1, int(duration_sec * sample_rate))
    output = BytesIO()
    with wave.open(output, "wb") as audio:
        audio.setnchannels(1)
        audio.setsampwidth(2)
        audio.setframerate(sample_rate)
        audio.writeframes(b"\x00\x00" * frames)
    return output.getvalue()


def script_fixture() -> ScriptVersion:
    segment = ScriptSegment("segment_contract", "contract speech", 0.0, 1.0)
    content_hash = hashlib.sha256(b"contract-script").hexdigest()
    return ScriptVersion("script_contract", 1, "en", (segment,), ScriptStyle(), ("artifact_source",), {}, {}, content_hash)


class ExactTTS(FakeProvider):
    def __init__(self) -> None:
        super().__init__(ProviderKind.TTS, "exact-tts", model="exact-tts-v1")
        self.audio = wav_fixture()

    def synthesize(self, context, request) -> TTSResponse:
        base = super().synthesize(context, request)
        return TTSResponse(ProducedBlob("audio", "narration", self.audio, "audio/wav", {"fixture": "exact"}), 0.25, None, base.meta)


class ExactASR(FakeProvider):
    def __init__(self, invalid: bool = False) -> None:
        super().__init__(ProviderKind.ASR, "exact-asr", model="exact-asr-v1")
        self.invalid = invalid

    def transcribe(self, context, request) -> ProviderTranscript:
        base = super().transcribe(context, request)
        end = 0.1 if self.invalid else 0.8
        word = ProviderTranscriptWord("provider", 0.1, 0.4, 0.9)
        segment = ProviderTranscriptSegment("provider-segment", "provider transcript", 0.1, end, 0.9, "speaker-1", (word,))
        return ProviderTranscript("en", "exact-asr-v1", 1.0, (segment,), base.meta)


class KnownEmbedding(FakeProvider):
    def __init__(self) -> None:
        super().__init__(ProviderKind.EMBEDDING, "known-embedding", model="known-embedding-v1")

    def embed(self, context, request) -> EmbeddingResponse:
        base = super().embed(context, request)
        values = ((1.0, 0.0, 0.0), (0.0, 1.0, 0.0))
        vectors = tuple(EmbeddingVector(item.item_id, values[index]) for index, item in enumerate(request.items))
        return EmbeddingResponse(3, "cosine", "known-embedding-v1", True, vectors, tuple(item.item_id for item in request.items), None, base.meta)


def test_typed_provider_outputs_are_consumed_and_cache_identity_is_complete() -> None:
    provider = ExactTTS()
    store = MemoryArtifactStore()
    first = synthesize_narration(script_fixture(), provider, provider_context(), store, VoiceSnapshot("config_contract", 1, "exact-tts", "exact-tts-v1", "voice-a", "en"))
    second = synthesize_narration(script_fixture(), provider, provider_context(), store, VoiceSnapshot("config_contract", 2, "exact-tts", "exact-tts-v1", "voice-b", "en"), speed=1.1, style_prompt="calm")
    assert store.read(first.audio_artifact.artifact_id) == provider.audio
    assert first.duration_sec == pytest.approx(0.25)
    assert first.cache_fingerprint != second.cache_fingerprint
    assert first.provider_snapshot["adapter_key"] == "exact-tts"


def test_asr_uses_provider_timing_and_rejects_invalid_timing() -> None:
    transcript = transcribe_segments(provider=ExactASR(), context=provider_context(), audio_artifact_ref="artifact_real_audio")
    assert transcript.segments[0].text == "provider transcript"
    assert transcript.segments[0].start_sec == pytest.approx(0.1)
    with pytest.raises(ValueError, match="timing"):
        transcribe_segments(provider=ExactASR(invalid=True), context=provider_context(), audio_artifact_ref="artifact_real_audio")


def test_embedding_index_consumes_provider_vectors_and_rejects_space_mismatch() -> None:
    index = build_embedding_index((("item-a", "first", "text"), ("item-b", "second", "text")), KnownEmbedding(), provider_context(), "known-space", dimension=3)
    assert index.vectors["item-a"] == (1.0, 0.0, 0.0)
    assert index.compare("item-a", "item-b", "known-space") == 0.0
    with pytest.raises(ValueError, match="model-space"):
        index.compare("item-a", "item-b", "other-space")


def test_typed_character_groups_and_candidate_negative_paths_are_traceable() -> None:
    observations = (
        AppearanceObservation("appearance-a", (1.0, 0.0), "scene-a", "source-r1"),
        AppearanceObservation("appearance-b", (0.99, 0.01), "scene-b", "source-r1"),
        AppearanceObservation("appearance-c", (0.0, 1.0), "scene-c", "source-r1"),
    )
    proposals = cluster_characters(observations, {"appearance-a": "Alice"})
    assert len(proposals) == 2
    assert any(proposal.confirmed_identity == "Alice" for proposal in proposals)
    candidates = (GenerationCandidate("candidate-a", "group-a", {"quality_score": 0.8}, {}), GenerationCandidate("candidate-b", "group-a", {"quality_score": 0.7}, {}))
    policy = CandidateSelectionPolicy("selection-contract", "1", group_id="group-a")
    evaluations = evaluate_candidates(candidates, policy)
    assert select_candidate(candidates, evaluations, policy).selected_candidate_id == "candidate-a"
    with pytest.raises(ValueError, match="unknown candidate"):
        select_candidate(candidates, (*evaluations, EvaluationResult("evaluation-missing", "missing", "1", 0.5, {}, {})), policy)


def test_real_ffmpeg_scene_detection_and_features_are_not_fixture_constants(tmp_path: Path) -> None:
    ffmpeg = os.environ.get("NH_MEDIA_FFMPEG_PATH")
    ffprobe = os.environ.get("NH_MEDIA_FFPROBE_PATH")
    if not ffmpeg or not ffprobe:
        raise AssertionError("Gate G real-media proof requires reviewed NH_MEDIA_FFMPEG_PATH and NH_MEDIA_FFPROBE_PATH; unexpected skip")
    source = tmp_path / "scene-fixture.mp4"
    command = [
        ffmpeg,
        "-hide_banner",
        "-loglevel",
        "error",
        "-y",
        "-f",
        "lavfi",
        "-i",
        "color=c=blue:s=160x90:d=1",
        "-f",
        "lavfi",
        "-i",
        "color=c=red:s=160x90:d=1",
        "-filter_complex",
        "[0:v][1:v]concat=n=2:v=1:a=0[v]",
        "-map",
        "[v]",
        "-an",
        "-c:v",
        "libx264",
        "-pix_fmt",
        "yuv420p",
        str(source),
    ]
    completed = subprocess.run(command, check=False, stdout=subprocess.PIPE, stderr=subprocess.PIPE, shell=False)
    assert completed.returncode == 0, completed.stderr.decode("utf-8", "replace")
    scenes, features = detect_scenes_from_media(str(source), "source-real-v1", ffmpeg, ffprobe, threshold=0.2)
    assert len(scenes) >= 2
    assert len(features) == len(scenes)
    assert all(item.keyframe_refs for item in features)
    assert len({(item.mean_luma, item.motion_score, item.quality_score) for item in features}) > 1
