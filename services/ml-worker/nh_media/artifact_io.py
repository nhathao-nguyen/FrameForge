"""Executor-scoped Artifact materialization and direct staging client."""

from __future__ import annotations

import hashlib
import json
import os
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any, Protocol


class ArtifactIO(Protocol):
    def read(self, command: dict[str, Any], ref: dict[str, Any]) -> bytes: ...

    def stage(self, command: dict[str, Any], kind: str, role: str, data: bytes, content_type: str) -> dict[str, Any]: ...


class HTTPArtifactIO:
    def __init__(self, api_url: str, token: str, local_root: str = "") -> None:
        if not api_url.startswith(("http://", "https://")) or len(token) < 24:
            raise ValueError("worker Artifact transfer configuration is invalid")
        self.api_url = api_url.rstrip("/")
        self.token = token
        self.local_root = Path(local_root).resolve() if local_root else None

    def read(self, command: dict[str, Any], ref: dict[str, Any]) -> bytes:
        transfer = self._post(
            "/internal/v1/worker/artifacts/resolve",
            {"project_id": command["project_id"], "artifact_id": ref["artifact_id"], "sha256": ref["sha256"]},
        )
        data = self._transfer(transfer, None)
        if hashlib.sha256(data).hexdigest() != ref["sha256"]:
            raise OSError("materialized Artifact checksum mismatch")
        return data

    def stage(self, command: dict[str, Any], kind: str, role: str, data: bytes, content_type: str) -> dict[str, Any]:
        digest = hashlib.sha256(data).hexdigest()
        artifact_id = f"artifact_{role}_{digest[:16]}"
        transfer = self._post(
            "/internal/v1/worker/artifacts/stage",
            {
                "workspace_id": command["workspace_id"],
                "project_id": command["project_id"],
                "job_id": command["job_id"],
                "job_step_id": command["job_step_id"],
                "message_id": command["message_id"],
                "artifact_id": artifact_id,
                "content_type": content_type,
            },
        )
        self._transfer(transfer, data)
        return {
            "artifact_id": artifact_id,
            "kind": kind,
            "role": role,
            "sha256": digest,
            "size_bytes": len(data),
            "content_type": content_type,
        }

    def _post(self, path: str, payload: dict[str, Any]) -> dict[str, Any]:
        request = urllib.request.Request(
            self.api_url + path,
            data=json.dumps(payload, separators=(",", ":")).encode(),
            method="POST",
            headers={"Authorization": f"Bearer {self.token}", "Content-Type": "application/json"},
        )
        try:
            with urllib.request.urlopen(request, timeout=30) as response:  # nosec B310 - configured internal API only
                value = json.load(response)
        except (urllib.error.URLError, TimeoutError, json.JSONDecodeError) as error:
            raise OSError("worker Artifact transfer request failed") from error
        if not isinstance(value, dict) or not isinstance(value.get("url"), str):
            raise OSError("worker Artifact transfer response is invalid")
        return value

    def _transfer(self, transfer: dict[str, Any], data: bytes | None) -> bytes:
        url = str(transfer["url"])
        method = str(transfer.get("method", "GET"))
        headers = {str(key): str(value) for key, value in dict(transfer.get("headers") or {}).items()}
        if url.startswith("nh-local://"):
            return self._local_transfer(url, method, data)
        if not url.startswith(("http://", "https://")):
            raise OSError("worker transfer scheme is not allowed")
        request = urllib.request.Request(url, data=data, method=method, headers=headers)
        try:
            with urllib.request.urlopen(request, timeout=300) as response:  # nosec B310 - signed object-store URL
                return response.read() if data is None else b""
        except (urllib.error.URLError, TimeoutError) as error:
            raise OSError("worker object transfer failed") from error

    def _local_transfer(self, url: str, method: str, data: bytes | None) -> bytes:
        if self.local_root is None:
            raise OSError("local worker storage root is not configured")
        parsed = urllib.parse.urlparse(url)
        key = urllib.parse.unquote(parsed.netloc + parsed.path).lstrip("/")
        candidate = (self.local_root / Path(key.replace("/", os.sep))).resolve()
        if self.local_root != candidate and self.local_root not in candidate.parents:
            raise OSError("local worker transfer escaped its configured root")
        if method == "GET" and data is None:
            return candidate.read_bytes()
        if method != "PUT" or data is None:
            raise OSError("local worker transfer method is invalid")
        candidate.parent.mkdir(parents=True, exist_ok=True)
        candidate.write_bytes(data)
        return b""


class MemoryArtifactIO:
    def __init__(self, values: dict[str, bytes] | None = None) -> None:
        self.values = dict(values or {})

    def read(self, command: dict[str, Any], ref: dict[str, Any]) -> bytes:
        del command
        value = self.values[ref["artifact_id"]]
        if hashlib.sha256(value).hexdigest() != ref["sha256"]:
            raise OSError("materialized Artifact checksum mismatch")
        return value

    def stage(self, command: dict[str, Any], kind: str, role: str, data: bytes, content_type: str) -> dict[str, Any]:
        del command
        digest = hashlib.sha256(data).hexdigest()
        artifact_id = f"artifact_{role}_{digest[:16]}"
        self.values[artifact_id] = data
        return {"artifact_id": artifact_id, "kind": kind, "role": role, "sha256": digest, "size_bytes": len(data), "content_type": content_type}


__all__ = ["ArtifactIO", "HTTPArtifactIO", "MemoryArtifactIO"]
