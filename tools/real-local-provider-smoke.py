"""Run a real local provider smoke and emit safe, reproducible evidence."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import tempfile
import time
from pathlib import Path

WORKER_ROOT = Path(__file__).resolve().parents[1] / "services" / "ml-worker"
sys.path.insert(0, str(WORKER_ROOT))

from nh_media.providers.contracts import (  # noqa: E402
    EmbeddingItem,
    EmbeddingRequest,
    LLMMessage,
    LLMRequest,
    ProviderCallContext,
    ProviderError,
    ProviderKind,
    TTSRequest,
    VLMInput,
    VLMRequest,
)
from nh_media.providers.real import build_real_providers  # noqa: E402


def _policy() -> dict[str, object]:
    return {
        "mode": "local",
        "adapter_keys": {
            "llm": ["ollama-openai-llm"],
            "vlm": ["ollama-vlm"],
            "tts": ["sapi-tts"],
            "asr": ["faster-whisper-asr"],
            "embedding": ["ollama-embedding"],
        },
        "providers": {
            "ollama-openai-llm": {"deployment": "local", "endpoint": "http://127.0.0.1:11434/v1", "model": "qwen2.5:3b"},
            "ollama-vlm": {"deployment": "local", "endpoint": "http://127.0.0.1:11434", "model": "moondream"},
            "sapi-tts": {"deployment": "local", "voice_id": "voice-local"},
            "faster-whisper-asr": {"deployment": "local", "model": "tiny.en", "device": "cpu", "compute_type": "int8"},
            "ollama-embedding": {"deployment": "local", "endpoint": "http://127.0.0.1:11434", "model": "nomic-embed-text", "dimension": 768},
        },
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    run_id = Path(args.output).stem
    context = ProviderCallContext(run_id, run_id, "project_smoke", "job_smoke", "step_smoke", "config_real_local", 1, timeout_sec=180, privacy_policy="local_only")
    providers = build_real_providers(_policy())
    by_kind = {provider.descriptor.kind: provider for provider in providers}
    evidence: dict[str, object] = {"run_id": run_id, "mode": "LOCAL", "started_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "capabilities": {}}

    def record(kind: ProviderKind, call: object) -> object | None:
        try:
            value = call()  # type: ignore[operator]
            meta = getattr(value, "meta", None)
            evidence["capabilities"][kind.value] = {"status": "PASS", "adapter_key": by_kind[kind].descriptor.adapter_key, "adapter_version": by_kind[kind].descriptor.adapter_version, "model": by_kind[kind].descriptor.models[0], "provider_request_id": getattr(meta, "provider_request_id", "")}
            return value
        except ProviderError as exc:
            evidence["capabilities"][kind.value] = {"status": "BLOCKED", "adapter_key": by_kind[kind].descriptor.adapter_key, "error": exc.as_dict()}
        except Exception as exc:  # noqa: BLE001 - safe smoke evidence boundary
            evidence["capabilities"][kind.value] = {"status": "BLOCKED", "adapter_key": by_kind[kind].descriptor.adapter_key, "error": {"code": "internal", "safe_message": type(exc).__name__}}
        return None

    llm = record(ProviderKind.LLM, lambda: by_kind[ProviderKind.LLM].complete(context, LLMRequest("Return a concise response.", (LLMMessage("user", "Say local provider is ready.", False),), max_output_tokens=64)))
    tts = record(ProviderKind.TTS, lambda: by_kind[ProviderKind.TTS].synthesize(context, TTSRequest("This is a local narration smoke test.", "en", "voice-local", {"source": "real-local-smoke"})))
    if tts is not None:
        asr = record(ProviderKind.ASR, lambda: by_kind[ProviderKind.ASR].transcribe(context, __import__("nh_media.providers.contracts", fromlist=["ASRRequest"]).ASRRequest("artifact_smoke_audio", "en", "tiny.en", True, False, (), "", None, tts.audio_blob.data)))
        if asr is not None:
            evidence["capabilities"]["asr"]["segment_count"] = len(asr.segments)
    embedding = record(ProviderKind.EMBEDDING, lambda: by_kind[ProviderKind.EMBEDDING].embed(context, EmbeddingRequest("nomic-embed-text", "text", (EmbeddingItem("item_a", "text", "local scene"), EmbeddingItem("item_b", "text", "local narration")))))
    if embedding is not None:
        evidence["capabilities"]["embedding"]["dimension"] = embedding.dimension
    with tempfile.TemporaryDirectory(prefix="nh-media-local-smoke-") as root:
        image = Path(root) / "frame.jpg"
        ffmpeg = os.environ.get("NH_MEDIA_FFMPEG_PATH", "")
        if ffmpeg:
            result = subprocess.run((ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=steelblue:s=640x360:d=1", "-frames:v", "1", "-q:v", "3", str(image), "-y"), shell=False, check=False, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=60)  # nosec
            if result.returncode == 0 and image.is_file():
                record(ProviderKind.VLM, lambda: by_kind[ProviderKind.VLM].analyze(context, VLMRequest((VLMInput("frame_1", "artifact_smoke_frame", "image/jpeg", "scene_001", "source_smoke", image_bytes=image.read_bytes()),), "Describe the visible scene in one sentence.")))
            else:
                evidence["capabilities"]["vlm"] = {"status": "BLOCKED", "error": {"code": "unavailable", "safe_message": "Reviewed FFmpeg could not create the smoke keyframe."}}
        else:
            evidence["capabilities"]["vlm"] = {"status": "BLOCKED", "error": {"code": "invalid_request", "safe_message": "Reviewed FFmpeg path is not configured."}}
    evidence["capabilities"]["llm"]["sample_text"] = getattr(llm, "text", "")[:200] if llm is not None else ""
    evidence["finished_at_utc"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(evidence, ensure_ascii=False, indent=2, sort_keys=True), encoding="utf-8")
    statuses = [value.get("status") for value in evidence["capabilities"].values() if isinstance(value, dict)]
    print(json.dumps({"output": str(output), "statuses": statuses}, ensure_ascii=False))
    return 0 if all(status == "PASS" for status in statuses) else 2


if __name__ == "__main__":
    raise SystemExit(main())
