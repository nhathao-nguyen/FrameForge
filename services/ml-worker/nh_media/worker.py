from __future__ import annotations


def health() -> dict[str, str]:
    """Return non-sensitive worker capability state for local health checks."""

    return {"service": "ml-worker", "namespace": "nh_media", "status": "live"}
