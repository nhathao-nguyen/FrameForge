---
last_verified: 2026-08-17
source: git status/log/branch metadata; ../SPEC-AUDIT-REPORT.md; ../IMPLEMENTATION-ORDER.md
owner: repository owner / task assignee
---

# Current state

## Repository baseline

| Field | Value | Evidence |
|---|---|---|
| Working branch | `implementation/bootstrap` | `git branch --show-current` |
| Baseline commit | `2fa58d4` (`checkpoint: close Gate E T322-T350 execution slice`) | committed Gate C+D plus Gate E T300–T350 Local/LAN execution checkpoint |
| Tracking branch | `origin/implementation/bootstrap` | `git status --short --branch` |
| Push/merge action | checkpoint pushed to `origin/implementation/bootstrap` on 2026-08-17 | `git push origin implementation/bootstrap`; local and remote hashes match |

## Gate and phase

- Specification audit: passed for the owner-ratified Local/LAN-first architecture and final pre-code gate.
- T000 independent specification refactor: complete.
- T001 research provenance: recorded; it is not a runtime baseline.
- T002 toolchain matrix: complete with Windows version, pin, Tauri prerequisite, FFmpeg/ffprobe and Docker evidence.
- T003 reference-behavior fixture policy: complete with a policy README and intentionally empty manifest.
- T004 owner approval/independence certification: complete by 2026-08-17 ratification and final
  documentation consistency evidence; final pre-code certification rerun after T002/T003.
- Application implementation: Gate B foundation T100–T108, Gate C+D, and the Gate E execution slice
  are committed at checkpoint `2fa58d4`; the working tree is clean.
- Gate C+D status: T200–T235 implementation plus the narrow C+D repair is committed at the baseline
  above. Focused Go/unit/sqlmock and non-desktop canonical verification remain green. The API selects
  the durable PostgreSQL/Product + MinIO path when `NH_MEDIA_DATABASE_URL` is configured; unit tests
  retain the explicit in-memory backend as a deterministic test adapter.
- Gate E status: T300–T350 are complete for the Local/LAN-first execution slice. Durable PostgreSQL
  Job/Run/Step/Review/Render commands, live native and Python worker/artifact paths, retry/redelivery,
  cancellation, recovery planning, and authenticated SSE snapshot/replay/reconnect tests now pass.
  T400/Gate F, desktop T550 acceptance, hardened T603, and public/VPS T605 remain explicitly out of scope.

## Current evidence

- Target topology and product identity are consistent across the normative set.
- Upstream reference policy, detailed capability matrix and research module audit exist.
- T002 Windows toolchain evidence is in `docs/bootstrap/T002-TOOLCHAIN-MATRIX-WINDOWS.md`.
- T003 policy and manifest are in `tests/reference-behavior/`; the manifest is empty and normal CI
  remains upstream-free.
- First deterministic native slice is T321; first Python worker slice is T323. T322/T323 now have
  live Go durable queue-to-lease-to-MinIO-Artifact-to-event, Redis-to-Python-to-PostgreSQL/MinIO,
  Python restart/XAUTOCLAIM, and transient retry/redelivery harnesses. T330–T332 have durable
  recovery/review/cancel/replay command evidence; T340 has authenticated SSE snapshot/replay/terminal
  reconnect and invalid-cursor coverage. The remaining client-shell/T550 and hardened/public release
  gates are not claimed.
- Gate B evidence covers repository boundaries, shared primitives, redaction/config boundaries, private
  PostgreSQL/Redis/MinIO Compose services, Go API shell, health/readiness, CI and independence scans.
- Gate C+D evidence includes eight PostgreSQL migrations, the checksum/dirty migration runner, scoped SQL
  repositories, durable hashed sessions, canonical domain state/timeline validation, interchangeable
  LocalStorage/S3 adapters, artifact commit compensation plus orphan reconciliation, ffprobe validation,
  and native `/api/v1` Project/Asset/Script/Narration/Timeline/Render/Scene/Analysis/Candidate/Provider
  resources. The repair adds deterministic malformed/pathological media quarantine evidence, durable
  Timeline reference resolution and RenderProfile lifecycle enforcement.
- Prior baseline migration-runner integration on 2026-08-17 applied the original seven migrations.
  Repair rerun after environment setup applied and repeated all eight migrations, including
  `0008_gate_cd_repairs.sql`, with clean checksums; `nh_media_app` was denied `CREATE TABLE`. The
  private MinIO S3 conformance suite also passed against the live Compose network.
- Durable runtime smoke on 2026-08-17: PostgreSQL-backed login survived an API restart using the same
  hashed `auth_sessions` row; direct multipart PUT to private MinIO returned an ETag, complete created a
  durable `asset_probe` Job, `asset_uploads.status=completed`, and Asset status `validating`; duplicate
  Project request replayed the same response with one database row.
- Local Functional Acceptance is T550; Local/LAN Hardened Acceptance is T603; Internet/VPS
  production is T605.
- Gate E includes canonical transitions/events/outbox replay, Redis Streams QueuePort, DAG scheduling,
  leases/reconciliation, sandboxed media process, versioned Go/Python worker contracts, probe/thumbnail
  node, durable execution-graph bootstrap, Job/Run/Step/Review/Render commands, checkpoint/recovery,
  retry/DLQ adapters, cancellation, partial stop-after pause, and SSE. The live evidence and its
  limitations are recorded in TEST-EVIDENCE.md; no later Gate F task is implied.

## Decision status

Open architectural/product questions: **0**. Owner-decision blockers: **0**. OQ-07 and OQ-11 are
`DEFERRED-NONBLOCKING` with concrete initial baselines; exact future VPS/OIDC/KMS/S3/monitoring
vendors are later configuration choices.

## Handoff

```text
Task: Gate E — T300–T350 Local/LAN execution slice
Status: complete; Local/LAN runtime evidence pass, later client/hardened/public gates pending
Boundary: Canonical execution transitions/events, queue, scheduling, leases, media/AI worker contracts,
  durable graph bootstrap, controls, checkpoint/DLQ and SSE boundary
  Application code: API, persistence, media worker, Python worker, contracts, and execution adapters
Upstream relationship: research/reference only; no operational dependency
Next: preserve this Gate E boundary; only start T400/Gate F after explicit owner approval and a new task.
```
