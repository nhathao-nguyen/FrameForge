from __future__ import annotations

import json
from pathlib import Path

from nh_media.contracts import validate_shared_primitives


def test_go_python_client_fixture_is_shared() -> None:
    repository = Path(__file__).resolve().parents[3]
    fixture = json.loads((repository / "packages/shared-contracts/fixtures/shared-primitives.v1.json").read_text(encoding="utf-8"))
    validate_shared_primitives(fixture)
