from __future__ import annotations

import hashlib
import json
import time
from dataclasses import dataclass
from typing import Any

from .contracts import validate_worker_command, validate_worker_result


def health() -> dict[str, str]:
    """Return non-sensitive worker capability state for local health checks."""

    return {"service": "ml-worker", "namespace": "nh_media", "status": "live"}


def process_command(command: dict[str, Any]) -> dict[str, Any]:
    """Run the dependency-free protocol slice used before real ML adapters."""

    validate_worker_command(command)
    if command["capability"] != "analysis":
        return {
            "schema_version": "worker-result/v1",
            "message_id": command["message_id"],
            "job_id": command["job_id"],
            "job_step_id": command["job_step_id"],
            "status": "failed",
            "output_refs": [],
            "safe_error": {"code": "capability_not_enabled", "category": "permanent", "retryable": False, "safe_message": "This worker slice does not enable the requested capability."},
        }
    fail_attempt = command.get("config", {}).get("test_fail_attempt")
    if isinstance(fail_attempt, int) and fail_attempt == command["attempt"]:
        result = {
            "schema_version": "worker-result/v1",
            "message_id": command["message_id"],
            "job_id": command["job_id"],
            "job_step_id": command["job_step_id"],
            "status": "failed",
            "output_refs": [],
            "safe_error": {"code": "transient_test_failure", "category": "transient", "retryable": True, "safe_message": "The deterministic test failure is retryable."},
        }
        validate_worker_result(result)
        return result
    # The first integration slice commits a tiny deterministic report. The
    # report body is intentionally empty; larger media/model outputs must be
    # staged through the worker Artifact boundary rather than Redis.
    empty_sha256 = hashlib.sha256(b"").hexdigest()
    result = {
        "schema_version": "worker-result/v1",
        "message_id": command["message_id"],
        "job_id": command["job_id"],
        "job_step_id": command["job_step_id"],
        "status": "completed",
        "output_refs": [{"artifact_id": f"artifact_{command['message_id']}", "kind": "deterministic_analysis", "role": "analysis_report", "sha256": empty_sha256, "size_bytes": 0}],
    }
    validate_worker_result(result)
    return result


@dataclass
class RedisWorker:
    """Small Redis Streams adapter; PostgreSQL/Artifact commit stays outside."""

    client: Any
    stream: str
    group: str
    consumer: str
    result_stream: str | None = None
    reclaim_idle_ms: int = 60_000

    def ensure_group(self) -> None:
        try:
            self.client.xgroup_create(self.stream, self.group, id="0", mkstream=True)
        except Exception as exc:
            if "BUSYGROUP" not in str(exc).upper():
                raise

    def run_once(self, count: int = 1, block_ms: int = 1000) -> list[dict[str, Any]]:
        self.ensure_group()
        count = max(1, min(count, 100))
        entries: list[tuple[Any, list[tuple[Any, dict[Any, Any]]]]] = []
        if self.reclaim_idle_ms > 0:
            _next_id, reclaimed, _deleted = self.client.xautoclaim(self.stream, self.group, self.consumer, self.reclaim_idle_ms, start_id="0-0", count=count)
            if reclaimed:
                entries.append((self.stream, reclaimed))
        if sum(len(values) for _stream, values in entries) < count:
            streams = self.client.xreadgroup(self.group, self.consumer, {self.stream: ">"}, count=count - sum(len(values) for _stream, values in entries), block=max(0, min(block_ms, 30_000)))
            entries.extend(streams or [])
        results: list[dict[str, Any]] = []
        for _stream, stream_entries in entries:
            for entry_id, fields in stream_entries:
                raw = fields.get(b"command", fields.get("command"))
                if isinstance(raw, bytes):
                    raw = raw.decode("utf-8")
                if not isinstance(raw, (str, bytes, bytearray)):
                    raise ValueError("worker command field is invalid")
                command = json.loads(raw)
                delay_ms = int(command.get("config", {}).get("test_delay_ms", 0))
                if delay_ms > 0:
                    time.sleep(min(delay_ms, 30_000) / 1000)
                result = process_command(command)
                validate_worker_result(result)
                if self.result_stream:
                    self.client.xadd(
                        self.result_stream,
                        {
                            "message_id": result["message_id"],
                            "job_id": result["job_id"],
                            "job_step_id": result["job_step_id"],
                            "attempt": str(command["attempt"]),
                            "result": json.dumps(result, separators=(",", ":")),
                        },
                    )
                self.client.xack(self.stream, self.group, entry_id)
                results.append(result)
        return results


def run_stdio() -> None:
    import sys

    for line in sys.stdin:
        if not line.strip():
            continue
        try:
            result = process_command(json.loads(line))
        except (ValueError, json.JSONDecodeError):
            result = {"schema_version": "worker-result/v1", "message_id": "msg_invalid", "job_id": "job_invalid", "job_step_id": "step_invalid", "status": "failed", "output_refs": [], "safe_error": {"code": "invalid_command", "category": "permanent", "retryable": False, "safe_message": "The worker command is invalid."}}
        sys.stdout.write(json.dumps(result, separators=(",", ":")) + "\n")
        sys.stdout.flush()


def run_redis() -> None:
    """Run the bounded worker hop against Redis Streams when configured."""

    import os

    import redis

    address = os.environ.get("NH_QUEUE_ENDPOINT", "redis://127.0.0.1:6379")
    capability = os.environ.get("NH_MEDIA_QUEUE_CAPABILITY", "analysis")
    prefix = os.environ.get("NH_QUEUE_STREAM_PREFIX", "nh-media").strip(":") or "nh-media"
    group = os.environ.get("NH_QUEUE_CONSUMER_GROUP", "nh-media-workers")
    consumer = os.environ.get("NH_MEDIA_WORKER_CONSUMER", "ml-worker-1")
    reclaim_idle_ms = max(0, min(int(os.environ.get("NH_MEDIA_RECLAIM_IDLE_MS", "60000")), 3_600_000))
    password = os.environ.get("NH_QUEUE_PASSWORD") or os.environ.get("REDIS_PASSWORD")
    client = redis.Redis.from_url(address, password=password, decode_responses=False)
    worker_stream = f"{prefix}:{capability}:worker"
    result_stream = f"{prefix}:{capability}:results"
    worker = RedisWorker(client, worker_stream, group, consumer, result_stream, reclaim_idle_ms)
    while True:
        worker.run_once(count=1, block_ms=1000)


if __name__ == "__main__":
    import os

    if os.environ.get("NH_MEDIA_REDIS_WORKER") == "1":
        run_redis()
    else:
        run_stdio()
