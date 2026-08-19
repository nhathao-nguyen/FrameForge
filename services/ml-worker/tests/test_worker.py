from nh_media import __version__
import json
from typing import Any, cast

import nh_media.worker as worker_module
from nh_media.artifact_io import ArtifactIO
from nh_media.worker import health, process_command
from nh_media.providers.contracts import ProviderError


def test_worker_namespace_and_health() -> None:
    assert __version__ == "0.1.0"
    assert health() == {"service": "ml-worker", "namespace": "nh_media", "status": "live"}


def test_deterministic_analysis_worker_returns_contract_safe_artifact() -> None:
    command = {
        "schema_version": "worker-command/v1",
        "message_id": "msg_analysis_001",
        "capability": "analysis",
        "workspace_id": "workspace_analysis_001",
        "project_id": "project_analysis_001",
        "job_id": "job_analysis_001",
        "pipeline_run_id": "run_analysis_001",
        "job_step_id": "step_analysis_001",
        "node_key": "analysis",
        "attempt": 1,
        "input_refs": [],
        "config": {"mode": "deterministic"},
    }
    first = process_command(command)
    second = process_command(command)
    assert first == second
    assert first["status"] == "completed"
    assert "path" not in json.dumps(first).lower()


def test_deterministic_worker_can_emit_bounded_retryable_fixture_failure() -> None:
    command = {
        "schema_version": "worker-command/v1",
        "message_id": "msg_analysis_002",
        "capability": "analysis",
        "workspace_id": "workspace_analysis_001",
        "project_id": "project_analysis_001",
        "job_id": "job_analysis_001",
        "pipeline_run_id": "run_analysis_001",
        "job_step_id": "step_analysis_001",
        "node_key": "analysis",
        "attempt": 1,
        "input_refs": [],
        "config": {"test_fail_attempt": 1},
    }
    result = process_command(command)

    assert result["status"] == "failed"
    assert result["safe_error"]["category"] == "transient"
    assert result["safe_error"]["retryable"] is True


def test_worker_preserves_safe_provider_failure_taxonomy(monkeypatch: Any) -> None:
    command = {
        "schema_version": "worker-command/v1",
        "message_id": "msg_provider_failure_001",
        "capability": "ai",
        "workspace_id": "workspace_provider_001",
        "project_id": "project_provider_001",
        "job_id": "job_provider_001",
        "pipeline_run_id": "run_provider_001",
        "job_step_id": "step_provider_001",
        "node_key": "generate_script",
        "attempt": 1,
        "input_refs": [],
        "config": {},
    }

    def fail_with_provider_error(_command: dict[str, Any], _artifact_io: ArtifactIO) -> None:
        raise ProviderError("fallback_exhausted", "ollama-openai-llm", retryable=False, category="transient", safe_message="All approved provider fallbacks exhausted their bounded retry policy.")

    monkeypatch.setattr(worker_module, "execute_gate_g_node", fail_with_provider_error)
    result = process_command(command, artifact_io=cast(ArtifactIO, object()))

    assert result["status"] == "failed"
    assert result["safe_error"] == {
        "code": "fallback_exhausted",
        "provider": "ollama-openai-llm",
        "category": "transient",
        "retryable": False,
        "safe_message": "All approved provider fallbacks exhausted their bounded retry policy.",
        "provider_request_id": None,
        "retry_after_sec": None,
    }
