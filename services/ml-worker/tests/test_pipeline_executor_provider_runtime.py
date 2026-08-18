from __future__ import annotations

import json
from typing import Any

import pytest

from nh_media.artifact_io import MemoryArtifactIO
from nh_media.pipeline_executor import GateGProviderRuntime, execute_gate_g_node
from nh_media.providers.contracts import (
    FakeProvider,
    LLMRequest,
    LLMResponse,
    ProviderCallContext,
    ProviderError,
    ProviderKind,
)
from nh_media.providers.resolver import CircuitBreakerPolicy, RetryPolicy


def command(node_key: str, step: str, input_refs: list[dict[str, object]] | None = None) -> dict[str, object]:
    return {
        "node_key": node_key,
        "capability": "ai" if node_key in {"research_metadata", "generate_script", "generate_narration"} else "ml",
        "message_id": f"message-{step}",
        "job_id": "job-runtime",
        "project_id": "project-runtime",
        "job_step_id": step,
        "workspace_id": "workspace-runtime",
        "input_refs": input_refs or [],
        "provider_policy": {
            "configuration_id": "config-runtime",
            "configuration_revision": 3,
            "privacy_policy": "local_only",
        },
    }


def source_ref() -> dict[str, object]:
    return {"artifact_id": "source-artifact", "role": "source", "sha256": "0" * 64, "size_bytes": 1}


class ScriptedLLM(FakeProvider):
    def __init__(self, adapter_key: str, failures: list[ProviderError]) -> None:
        super().__init__(ProviderKind.LLM, adapter_key)
        self.failures = failures
        self.calls = 0

    def complete(self, context: ProviderCallContext, request: LLMRequest) -> LLMResponse:
        self.calls += 1
        if self.failures:
            raise self.failures.pop(0)
        return super().complete(context, request)


def runtime(*providers: object, adapter_keys: tuple[str, ...] = ("node-primary", "node-secondary")) -> GateGProviderRuntime:
    return GateGProviderRuntime(
        tuple(providers),
        adapter_keys={ProviderKind.LLM: adapter_keys},
        configuration_id="config-runtime",
        configuration_revision=3,
        retry_policy=RetryPolicy(max_attempts=2, base_delay_sec=0, max_delay_sec=0, jitter_ratio=0),
        circuit_policy=CircuitBreakerPolicy(failure_threshold=5, recovery_timeout_sec=10),
        sleep=lambda _delay: None,
    )


def read_state(io: MemoryArtifactIO, ref: dict[str, object]) -> dict[str, Any]:
    return json.loads(io.values[str(ref["artifact_id"])].decode())


def test_research_node_retries_and_records_primary_provider() -> None:
    primary = ScriptedLLM("node-primary", [ProviderError("unavailable", "node-primary")])
    io = MemoryArtifactIO()
    outputs = execute_gate_g_node(command("research_metadata", "research" , [source_ref()]), io, runtime(primary, ScriptedLLM("node-secondary", [])))
    research = read_state(io, outputs[0])["state"]["research"]
    assert primary.calls == 2
    assert research["provider_snapshot"]["adapter_key"] == "node-primary"


def test_research_node_falls_back_and_records_secondary_provider() -> None:
    primary = ScriptedLLM("node-primary", [ProviderError("unavailable", "node-primary"), ProviderError("unavailable", "node-primary")])
    secondary = ScriptedLLM("node-secondary", [])
    io = MemoryArtifactIO()
    outputs = execute_gate_g_node(command("research_metadata", "fallback", [source_ref()]), io, runtime(primary, secondary))
    research = read_state(io, outputs[0])["state"]["research"]
    assert primary.calls == 2 and secondary.calls == 1
    assert research["provider_snapshot"]["adapter_key"] == "node-secondary"
    assert "config-runtime" in json.dumps(research, sort_keys=True)


def test_node_path_does_not_fallback_auth_failure_or_leak_policy_secrets() -> None:
    primary = ScriptedLLM("node-primary", [ProviderError("auth_failed", "node-primary")])
    secondary = ScriptedLLM("node-secondary", [])
    io = MemoryArtifactIO()
    with pytest.raises(ProviderError, match="Provider request failed"):
        execute_gate_g_node(command("research_metadata", "auth", [source_ref()]), io, runtime(primary, secondary))
    assert primary.calls == 1 and secondary.calls == 0
    assert "api_key" not in str(io.values)
    assert "password" not in str(io.values)


def test_node_path_honors_retry_after_and_open_circuit_uses_approved_fallback() -> None:
    delays: list[float] = []
    primary = ScriptedLLM("node-primary", [ProviderError("rate_limited", "node-primary", retry_after_sec=0.5)])
    io = MemoryArtifactIO()
    gate_runtime = GateGProviderRuntime(
        (primary, ScriptedLLM("node-secondary", [])),
        adapter_keys={ProviderKind.LLM: ("node-primary", "node-secondary")},
        configuration_id="config-runtime",
        configuration_revision=3,
        retry_policy=RetryPolicy(max_attempts=2, base_delay_sec=0.1, max_delay_sec=1, jitter_ratio=0),
        circuit_policy=CircuitBreakerPolicy(failure_threshold=1, recovery_timeout_sec=30),
        sleep=delays.append,
    )
    execute_gate_g_node(command("research_metadata", "retry-after", [source_ref()]), io, gate_runtime)
    assert primary.calls == 2 and delays == pytest.approx([0.5])

    failing_primary = ScriptedLLM("node-primary", [ProviderError("unavailable", "node-primary")])
    fallback = ScriptedLLM("node-secondary", [])
    circuit_runtime = GateGProviderRuntime(
        (failing_primary, fallback),
        adapter_keys={ProviderKind.LLM: ("node-primary", "node-secondary")},
        configuration_id="config-runtime",
        configuration_revision=3,
        retry_policy=RetryPolicy(max_attempts=1),
        circuit_policy=CircuitBreakerPolicy(failure_threshold=1, recovery_timeout_sec=30),
        sleep=lambda _delay: None,
    )
    execute_gate_g_node(command("research_metadata", "circuit-first", [source_ref()]), io, circuit_runtime)
    execute_gate_g_node(command("research_metadata", "circuit-open", [source_ref()]), io, circuit_runtime)
    assert failing_primary.calls == 1 and fallback.calls == 2


@pytest.mark.parametrize("code", ["content_rejected", "cancelled"])
def test_node_path_policy_and_cancellation_errors_do_not_retry(code: str) -> None:
    primary = ScriptedLLM("node-primary", [ProviderError(code, "node-primary")])
    fallback = ScriptedLLM("node-secondary", [])
    io = MemoryArtifactIO()
    with pytest.raises(ProviderError):
        execute_gate_g_node(command("research_metadata", f"no-retry-{code}", [source_ref()]), io, runtime(primary, fallback))
    assert primary.calls == 1 and fallback.calls == 0


def test_tts_and_asr_nodes_use_resolver_selected_typed_outputs() -> None:
    io = MemoryArtifactIO()
    providers = tuple(FakeProvider(kind) for kind in (ProviderKind.LLM, ProviderKind.VLM, ProviderKind.TTS, ProviderKind.ASR, ProviderKind.EMBEDDING))
    gate_runtime = GateGProviderRuntime(providers, configuration_id="config-runtime", configuration_revision=3, retry_policy=RetryPolicy(max_attempts=1), sleep=lambda _delay: None)
    research = execute_gate_g_node(command("research_metadata", "research-typed", [source_ref()]), io, gate_runtime)
    script = execute_gate_g_node(command("generate_script", "script-typed", research), io, gate_runtime)
    narration = execute_gate_g_node(command("generate_narration", "tts-typed", script), io, gate_runtime)
    manifest = read_state(io, narration[1])["state"]["narration"]
    assert io.values[str(narration[0]["artifact_id"])].startswith(b"RIFF")
    assert manifest["provider_snapshot"]["adapter_key"] == "fake-tts"
    alignment = execute_gate_g_node(command("align_audio", "asr-typed", script + narration), io, gate_runtime)
    alignment_state = read_state(io, alignment[0])["state"]["alignment"]
    assert alignment_state["provider_snapshot"]["adapter_key"] == "fake-asr"
    scene_analysis = io.stage(command("analyze_scenes", "scene-input"), "gate_g_json", "scene_analysis", json.dumps({"schema_version": "gate-g-node/v1", "state": {"scene_analyses": [{"scene_id": "scene-001", "text": "A deterministic scene", "entities": [], "source_revision": "source-v1", "keyframe_refs": [], "provider_snapshot": {}, "status": "completed", "actions": [], "confidence": 1.0}]}}).encode(), "application/json")
    embeddings = execute_gate_g_node(command("embed_media", "embedding-typed", script + [scene_analysis]), io, gate_runtime)
    embedding_state = read_state(io, embeddings[0])["state"]["embedding_index"]
    assert embedding_state["provider_snapshot"]["adapter_key"] == "fake-embedding"


def test_runtime_default_binding_registers_fakes_only_at_adapter_boundary() -> None:
    io = MemoryArtifactIO()
    outputs = execute_gate_g_node(command("research_metadata", "default-binding", [source_ref()]), io)
    research = read_state(io, outputs[0])["state"]["research"]
    assert research["provider_snapshot"]["adapter_key"] == "fake-llm"
