from __future__ import annotations

import json
from pathlib import Path

from nh_media.contracts import validate_shared_primitives, validate_worker_command, validate_worker_progress, validate_worker_result


def test_go_python_client_fixture_is_shared() -> None:
    repository = Path(__file__).resolve().parents[3]
    fixture = json.loads((repository / "packages/shared-contracts/fixtures/shared-primitives.v1.json").read_text(encoding="utf-8"))
    validate_shared_primitives(fixture)


def test_worker_fixtures_validate_without_local_paths() -> None:
    repository = Path(__file__).resolve().parents[3]
    validate_worker_command(json.loads((repository / "packages/shared-contracts/fixtures/worker-command.v1.json").read_text(encoding="utf-8")))
    validate_worker_result(json.loads((repository / "packages/shared-contracts/fixtures/worker-result.v1.json").read_text(encoding="utf-8")))
    validate_worker_progress(json.loads((repository / "packages/shared-contracts/fixtures/worker-progress.v1.json").read_text(encoding="utf-8")))


def test_worker_contract_rejects_path_and_unknown_status() -> None:
    command = json.loads('{"schema_version":"worker-command/v1","message_id":"msg_probe_001","capability":"probe","project_id":"project_probe_001","job_id":"job_probe_001","pipeline_run_id":"run_probe_001","job_step_id":"step_probe_001","attempt":1,"input_refs":[],"config":{"local_path":"C:/secret"}}')
    try:
        validate_worker_command(command)
    except ValueError:
        pass
    else:
        raise AssertionError("path-bearing worker config was accepted")
    result = json.loads('{"schema_version":"worker-result/v1","message_id":"msg_probe_001","job_id":"job_probe_001","job_step_id":"step_probe_001","status":"processing","output_refs":[]}')
    try:
        validate_worker_result(result)
    except ValueError:
        pass
    else:
        raise AssertionError("non-canonical worker status was accepted")
