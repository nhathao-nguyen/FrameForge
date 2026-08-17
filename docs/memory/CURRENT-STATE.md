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
| Baseline commit | `4e97c9fc52f1ccc5396885d81a5bbe7f5c8783ca` | checkpoint under audit; Gate C+D repair remains uncommitted |
| Tracking branch | `origin/implementation/bootstrap` | `git status --short --branch` |
| Push/merge action | none performed | task scope |

## Gate and phase

- Specification audit: passed for the owner-ratified Local/LAN-first architecture and final pre-code gate.
- T000 independent specification refactor: complete.
- T001 research provenance: recorded; it is not a runtime baseline.
- T002 toolchain matrix: complete with Windows version, pin, Tauri prerequisite, FFmpeg/ffprobe and Docker evidence.
- T003 reference-behavior fixture policy: complete with a policy README and intentionally empty manifest.
- T004 owner approval/independence certification: complete by 2026-08-17 ratification and final
  documentation consistency evidence; final pre-code certification rerun after T002/T003.
- Application implementation: Gate B foundation T100–T108 plus the current uncommitted Gate C+D
  domain, persistence, storage and native API boundary are present in the working tree.
- Gate C+D status: T200–T235 implementation plus the narrow C+D repair is present in the current
  working tree. Focused Go/unit/sqlmock and non-desktop canonical verification are green. The API
  selects the durable PostgreSQL/Product + MinIO path when `NH_MEDIA_DATABASE_URL` is configured;
  unit tests retain the explicit in-memory backend as a deterministic test adapter.
- T300 and all later execution/queue work were not started.

## Current evidence

- Target topology and product identity are consistent across the normative set.
- Upstream reference policy, detailed capability matrix and research module audit exist.
- T002 Windows toolchain evidence is in `docs/bootstrap/T002-TOOLCHAIN-MATRIX-WINDOWS.md`.
- T003 policy and manifest are in `tests/reference-behavior/`; the manifest is empty and normal CI
  remains upstream-free.
- First deterministic native slice is T321/T322; first Python worker slice is T323.
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
- No Gate E/T300 state-transition service, QueuePort, or media/AI execution workflow was implemented;
  the Gate C+D declarative pipeline and upload validation Job intent are intentionally non-executing.

## Decision status

Open architectural/product questions: **0**. Owner-decision blockers: **0**. OQ-07 and OQ-11 are
`DEFERRED-NONBLOCKING` with concrete initial baselines; exact future VPS/OIDC/KMS/S3/monitoring
vendors are later configuration choices.

## Handoff

```text
Task: Gate C+D — T200–T235
Status: Gate C+D repair PASS; implementation remains uncommitted in working tree
Boundary: Domain, durable persistence, storage, upload/probe boundary and native Product APIs
  Application code: API shell, contracts, config, client/worker boundaries, Gate C+D domain/persistence/
  storage/API boundary, and private dev infrastructure
Upstream relationship: research/reference only; no operational dependency
Next: Gate E/T300 is the next separately authorized task; it owns canonical transitions/events and
  execution orchestration, which were intentionally not implemented here.
```
