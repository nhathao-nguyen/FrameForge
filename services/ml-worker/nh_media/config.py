from __future__ import annotations

import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Config:
    namespace: str = "nh_media"
    max_concurrency: int = 1

    @classmethod
    def from_env(cls) -> "Config":
        raw = os.getenv("NH_ML_MAX_CONCURRENCY", "1")
        value = int(raw)
        if value < 1:
            raise ValueError("NH_ML_MAX_CONCURRENCY must be positive")
        return cls(max_concurrency=value)
