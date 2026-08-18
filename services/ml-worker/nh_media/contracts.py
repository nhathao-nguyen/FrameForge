from __future__ import annotations

import re
from datetime import datetime
from typing import Any


_OPAQUE_ID = re.compile(r"^[a-z][a-z0-9_-]{2,127}$")
_ETAG = re.compile(r'^"[A-Za-z0-9._~-]+"$')
_CATEGORIES = {"transient", "permanent", "policy", "cancelled", "internal"}
_CAPABILITIES = {"probe", "thumbnail", "analysis", "ai", "ml", "media", "render", "system"}
_WORKER_FORBIDDEN_FIELDS = ("path", "secret", "token", "password", "traceback", "presigned", "authorization")


def validate_shared_primitives(value: dict[str, Any]) -> None:
    if value.get("schema_version") != "shared-primitives/v1":
        raise ValueError("unsupported shared primitive schema")
    for identifier in value["ids"].values():
        if not isinstance(identifier, str) or not _OPAQUE_ID.fullmatch(identifier):
            raise ValueError("invalid opaque ID")
    timestamp = value["timestamp"]
    parsed = datetime.fromisoformat(timestamp.replace("Z", "+00:00"))
    offset = parsed.utcoffset()
    if not timestamp.endswith("Z") or offset is None or offset.total_seconds() != 0:
        raise ValueError("timestamp must be UTC")
    media = value["media"]
    if media["start_sec"] < 0 or media["end_sec"] < media["start_sec"] or media["duration_sec"] < media["end_sec"] - media["start_sec"]:
        raise ValueError("invalid media range")
    concurrency = value["concurrency"]
    if concurrency["revision"] < 0 or concurrency["expected_version"] < 0 or not _ETAG.fullmatch(concurrency["etag"]):
        raise ValueError("invalid concurrency primitive")
    error = value["error"]
    if error["category"] not in _CATEGORIES or not error["safe_message"] or len(error["safe_message"]) > 512:
        raise ValueError("invalid safe error")


def validate_worker_command(value: dict[str, Any]) -> None:
    if value.get("schema_version") != "worker-command/v1":
        raise ValueError("unsupported worker command schema")
    for key in ("message_id", "workspace_id", "project_id", "job_id", "pipeline_run_id", "job_step_id"):
        if not isinstance(value.get(key), str) or not _OPAQUE_ID.fullmatch(value[key]):
            raise ValueError(f"invalid worker command ID: {key}")
    if value.get("pipeline_node_id") is not None and not _OPAQUE_ID.fullmatch(value["pipeline_node_id"]):
        raise ValueError("invalid worker pipeline node ID")
    if not isinstance(value.get("node_key"), str) or not re.fullmatch(r"^[a-z][a-z0-9_-]{1,63}$", value["node_key"]):
        raise ValueError("invalid worker node key")
    if value.get("capability") not in _CAPABILITIES or not isinstance(value.get("attempt"), int) or not 1 <= value["attempt"] <= 1000:
        raise ValueError("invalid worker capability or attempt")
    if not isinstance(value.get("input_refs"), list) or len(value["input_refs"]) > 100:
        raise ValueError("invalid worker input refs")
    for ref in value["input_refs"]:
        if not isinstance(ref, dict) or not _OPAQUE_ID.fullmatch(ref.get("artifact_id", "")) or not re.fullmatch(r"^[a-z][a-z0-9_-]{1,63}$", ref.get("role", "")) or not re.fullmatch(r"^[0-9a-f]{64}$", ref.get("sha256", "")):
            raise ValueError("invalid worker Artifact ref")
    _validate_safe_mapping(value.get("config", {}))


def validate_worker_result(value: dict[str, Any]) -> None:
    if value.get("schema_version") != "worker-result/v1":
        raise ValueError("unsupported worker result schema")
    for key in ("message_id", "job_id", "job_step_id"):
        if not isinstance(value.get(key), str) or not _OPAQUE_ID.fullmatch(value[key]):
            raise ValueError(f"invalid worker result ID: {key}")
    if value.get("status") not in {"completed", "skipped", "failed", "cancelled"}:
        raise ValueError("invalid worker result status")
    if not isinstance(value.get("output_refs"), list) or len(value["output_refs"]) > 100:
        raise ValueError("invalid worker output refs")
    for ref in value["output_refs"]:
        if not isinstance(ref, dict):
            raise ValueError("invalid worker output ref")
        content_type = ref.get("content_type")
        if not _OPAQUE_ID.fullmatch(ref.get("artifact_id", "")) or not re.fullmatch(r"^[a-z][a-z0-9_-]{1,63}$", ref.get("kind", "")) or not re.fullmatch(r"^[a-z][a-z0-9_-]{1,63}$", ref.get("role", "")) or not re.fullmatch(r"^[0-9a-f]{64}$", ref.get("sha256", "")) or not isinstance(ref.get("size_bytes"), int) or ref["size_bytes"] < 0 or (content_type is not None and (not isinstance(content_type, str) or not content_type or len(content_type) > 128 or "\r" in content_type or "\n" in content_type)):
            raise ValueError("invalid worker output ref")
    if value.get("checkpoint_ref") is not None and not _OPAQUE_ID.fullmatch(value["checkpoint_ref"]):
        raise ValueError("invalid worker checkpoint ref")
    if value.get("safe_error") is not None:
        error = value["safe_error"]
        if not error.get("code") or error.get("category") not in _CATEGORIES or not error.get("safe_message") or len(error["safe_message"]) > 512:
            raise ValueError("invalid worker safe error")


def validate_worker_progress(value: dict[str, Any]) -> None:
    if value.get("schema_version") != "worker-progress/v1":
        raise ValueError("unsupported worker progress schema")
    for key in ("message_id", "job_id", "job_step_id"):
        if not isinstance(value.get(key), str) or not _OPAQUE_ID.fullmatch(value[key]):
            raise ValueError("invalid worker progress ID")
    if not isinstance(value.get("percent"), (int, float)) or not 0 <= value["percent"] <= 100:
        raise ValueError("invalid worker progress percent")
    if len(value.get("safe_message", "")) > 256:
        raise ValueError("worker progress message is too long")


def _validate_safe_mapping(value: dict[str, Any]) -> None:
    if not isinstance(value, dict):
        raise ValueError("worker config must be an object")
    for key, child in value.items():
        if any(forbidden in key.lower() for forbidden in _WORKER_FORBIDDEN_FIELDS):
            raise ValueError(f"forbidden worker field: {key}")
        if isinstance(child, dict):
            _validate_safe_mapping(child)
        elif isinstance(child, list):
            for item in child:
                if isinstance(item, dict):
                    _validate_safe_mapping(item)
