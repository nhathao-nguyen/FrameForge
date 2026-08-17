from __future__ import annotations

import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def main() -> None:
    primitive = json.loads((ROOT / "packages/shared-contracts/fixtures/shared-primitives.v1.json").read_text(encoding="utf-8"))
    if primitive["schema_version"] != "shared-primitives/v1":
        raise SystemExit("unexpected primitive schema")
    if not primitive["timestamp"].endswith("Z"):
        raise SystemExit("timestamp is not UTC")
    if primitive["media"]["start_sec"] < 0 or primitive["media"]["end_sec"] < primitive["media"]["start_sec"]:
        raise SystemExit("invalid media range")
    if primitive["concurrency"]["revision"] != primitive["concurrency"]["expected_version"]:
        raise SystemExit("revision/expected_version fixture mismatch")
    if not primitive["concurrency"]["etag"].startswith('"'):
        raise SystemExit("invalid ETag fixture")

    redaction = json.loads((ROOT / "packages/shared-contracts/fixtures/redaction-safe-error.v1.json").read_text(encoding="utf-8"))
    if redaction["schema_version"] != "redaction-safe-error/v1" or "secret" in redaction["expected_public_fields"]:
        raise SystemExit("secret became public")
    for key in ("secret", "authorization", "path", "presigned_url", "traceback"):
        if not str(redaction["input"][key]).startswith("[REDACTED"):
            raise SystemExit(f"redaction fixture leaked {key}")

    manifest = json.loads((ROOT / "tests/reference-behavior/manifest.json").read_text(encoding="utf-8"))
    if manifest["upstream_execution_allowed_in_normal_ci"] is not False or manifest["fixtures"] != []:
        raise SystemExit("reference behavior policy is not upstream-free")
    print("shared contract, redaction and reference policy fixtures: pass")


if __name__ == "__main__":
    main()
