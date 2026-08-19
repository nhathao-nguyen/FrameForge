"""Report API provider readiness without exfiltrating credentials or media."""

from __future__ import annotations

import argparse
import json
import os
import time
from pathlib import Path


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--policy", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    policy = json.loads(Path(args.policy).read_text(encoding="utf-8"))
    capabilities: dict[str, object] = {}
    providers = policy.get("providers", {})
    for kind, adapter_keys in dict(policy.get("adapter_keys", {})).items():
        adapter_key = str(adapter_keys[0]) if adapter_keys else ""
        spec = providers.get(adapter_key, {}) if isinstance(providers, dict) else {}
        key_ref = str(spec.get("key_ref", "")) if isinstance(spec, dict) else ""
        env_name = key_ref[4:] if key_ref.startswith("env:") else ""
        if env_name and os.environ.get(env_name):
            status = "READY_FOR_LIVE_PROBE"
            detail = {"code": "not_probed", "safe_message": "Credential is present; live API probe requires explicit operator authorization."}
        else:
            status = "BLOCKED"
            detail = {"code": "auth_failed", "safe_message": "The configured API credential is unavailable."}
        capabilities[kind] = {
            "status": status,
            "adapter_key": adapter_key,
            "deployment": spec.get("deployment", "remote") if isinstance(spec, dict) else "remote",
            "model": spec.get("model", "") if isinstance(spec, dict) else "",
            "credential_ref": key_ref,
            "error": detail if status == "BLOCKED" else None,
        }
    evidence = {
        "schema_version": "nh-media/provider-smoke/v1",
        "mode": "API",
        "policy_path": str(Path(args.policy).resolve()),
        "started_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "capabilities": capabilities,
        "finished_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "network_probe": "not_run",
        "note": "No credential value, media, request body, or provider response is recorded.",
    }
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(evidence, ensure_ascii=False, indent=2, sort_keys=True), encoding="utf-8")
    statuses = [value.get("status") for value in capabilities.values() if isinstance(value, dict)]
    print(json.dumps({"output": str(output), "statuses": statuses}, ensure_ascii=False))
    return 0 if all(status == "READY_FOR_LIVE_PROBE" for status in statuses) else 2


if __name__ == "__main__":
    raise SystemExit(main())
