# Gate H acceptance evidence — 2026-08-18

Scope: T600–T604. T605 was not started. PASS statements below are limited to executable
source tests and the available one-machine local environment; no public/VPS or physical
second-device claim is made.

| Task/control | Evidence | Result |
|---|---|---|
| T600 sandbox/secret/egress | Go security/process/runtime tests; reviewed subprocess policy; `tools/verify-gate-h.ps1` | PASS: path/reparse, SSRF/egress, resource, redaction, plugin default-deny, capability secret scope, direct argv and process-tree controls |
| T601 observability/recovery | telemetry/health/inventory tests; live `/live`/`/ready` probes while stopping exact PostgreSQL, Redis and MinIO containers; tracked `tools/dev.ps1 restart` | PASS: live remains 200, readiness becomes 503 for each dependency outage and returns 200 after recovery; PostgreSQL-authorized reconcile path is tested. Redis EOF exits workers, so automatic reconnect is not claimed. |
| T602 inventory | `tools/inventory-artifacts.ps1`; persisted report outside repo | PASS for inventory control: `expected=215`, `observed=928`, `missing=0`, `corrupt=0`, `orphan=713`, invalid object-key count `0`; no object was deleted. The comparator correctly reports orphan findings; canonical policy does not require automatic zero-orphan cleanup. Version-ID metadata is not asserted by this comparator. |
| T602 backup/restore | `tools/backup-local.ps1`; `tools/restore-local.ps1`; exact `pg_dump`/`pg_restore` 16.10; isolated restore targets; migration and API read-through | PASS: custom dump size 1,162,535 bytes, SHA-256 `2d2dc85a8f4e8a36fb88f09b11ce23bf72d35b1ceba0b78ba4d1671f2fc5c1ff`; clean target `nh_media_gate_h_restore_20260818_1820` restored and migrated; restored Project→Job→PipelineRun→TimelineVersion→Render→3 Render Artifact refs were usable through Product API. |
| T602 negative controls | corrupt dump copy with manifest checksum mismatch; rerun into non-clean target | PASS: corrupt backup rejected before restore; non-clean target rejected before restore; source database was not mutated. |
| T603 full local stack | Docker Compose PostgreSQL 16.10, Redis 7.4.5, MinIO; Go API/media worker; Python `nh_media` worker; web listener; scheduler/reconciler | PASS: all tracked processes running, `/api/v1/live=200`, `/api/v1/ready=200`, logs checked after restart. |
| T603 real Product flow | `tools/accept-gate-h-live.ps1 -EvidencePath ...\gate-h-live-20260818-final2.json` | PASS: real MinIO upload and Go asset probe, movie_recap Job status completed, one PipelineRun, 25/25 steps completed, TimelineVersion validated/approved, Render and render Job completed, 39 download/hash checks with 0 mismatches. Reviewed FFmpeg/ffprobe hashes match T002. |
| T603 recovery/readiness | dependency stop/start probes plus full tracked restart after Redis EOF | PASS for reproducible manual restart recovery. Not an automatic worker-reconnect claim. |
| T604 research refresh | `docs/research/upstream-refresh-20260818.md`, `tools/refresh-upstream.ps1` | PASS: metadata/license/capability refresh only; Movie Narrator remains research/reference-only. |

## Current evidence locations

- Inventory: `%LOCALAPPDATA%\NH-Media\evidence\gate-h-inventory-20260818.json`.
- Restored inventory: `%LOCALAPPDATA%\NH-Media\evidence\gate-h-restore-inventory-20260818.json`.
- Backup manifest/dump: `%LOCALAPPDATA%\NH-Media\evidence\backups\20260818\`.
- Hardened flow: `%LOCALAPPDATA%\NH-Media\evidence\gate-h-live-20260818-final2.json`.

## Residual and excluded boundaries

- The 713 orphan findings are retained for owner review. This task did not authorize deletion,
  broad cleanup, or storage-policy invention.
- No malware scanner, AppContainer/container sandbox enforcement, machine power-loss simulation,
  production load test, VPS, DNS, TLS, public internet or physical second-device acceptance is
  claimed.
- T605 Internet/VPS production release remains unstarted and requires explicit approval.

**Gate H result: T600 PASS / T601 PASS / T602 PASS / T603 PASS for same-machine local scope /
T604 PASS. T605 NOT STARTED.**
