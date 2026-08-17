from __future__ import annotations

import re
from datetime import datetime
from typing import Any


_OPAQUE_ID = re.compile(r"^[a-z][a-z0-9_-]{2,127}$")
_ETAG = re.compile(r'^"[A-Za-z0-9._~-]+"$')
_CATEGORIES = {"transient", "permanent", "policy", "cancelled", "internal"}


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
