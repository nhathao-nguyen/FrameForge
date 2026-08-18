---
last_verified: 2026-08-18
source: git status/log/branch metadata; ../SPEC-AUDIT-REPORT.md; ../IMPLEMENTATION-ORDER.md; TEST-EVIDENCE.md; ../evidence/gate-g-dependency-preflight-20260818.md; ../evidence/gate-g-capability-coverage.md
owner: repository owner / task assignee
---

# Current state

## Repository baseline

| Field | Value | Evidence |
|---|---|---|
| Working branch | `implementation/bootstrap` | `git branch --show-current` |
| Baseline commit | `e72e847865a5938126487acfd583797e3e8839d0` (`docs: close Gate G push handoff`) | exact local/remote checkpoint before the worker-execution contract repair |
| Acceptance checkpoint | pending this task commit | implementation, E2E and full acceptance evidence are complete in the working tree |
| Tracking branch | `origin/implementation/bootstrap` | `git status --short --branch` |
| Push/merge action | checkpoint push pending | explicitly authorized by the user; no branch switch or merge |

## Gate and phase

- Specification audit: passed for the owner-ratified Local/LAN-first architecture and final pre-code gate.
- T000 independent specification refactor: complete.
- T001 research provenance: recorded; it is not a runtime baseline.
- T002 toolchain matrix: complete with Windows version, pin, Tauri prerequisite, FFmpeg/ffprobe and Docker evidence.
- T003 reference-behavior fixture policy: complete with a policy README and intentionally empty manifest.
- T004 owner approval/independence certification: complete by 2026-08-17 ratification and final
  documentation consistency evidence; final pre-code certification rerun after T002/T003.
- Application implementation: Gate B foundation T100–T108, Gate C+D, and the Gate E execution slice
  are committed at the baseline above and Gate E acceptance repair is checkpointed at `564ef99`.
- Gate C+D status: T200–T235 implementation plus the narrow C+D repair is committed at the baseline
  above. Focused Go/unit/sqlmock and non-desktop canonical verification remain green. The API selects
  the durable PostgreSQL/Product + MinIO path when `NH_MEDIA_DATABASE_URL` is configured; unit tests
  retain the explicit in-memory backend as a deterministic test adapter.
- Gate E status: T300–T350 are complete for the Local/LAN-first execution slice at checkpoint
  `564ef99`. Durable PostgreSQL Job/Run/Step/Review/Render commands,
  Product API client disconnect/reconnect, native and Python worker/artifact paths, Redis reclaim,
  crash/resume, retry/DLQ/replay, cancellation, partial stop-after resume, and authenticated SSE
  snapshot/replay/reconnect tests pass against the current working tree.
  T400/Gate F and desktop T550 acceptance were explicitly outside that Gate E checkpoint; their
  current status is recorded below. Hardened T603 and public/VPS T605 remain future work.
- Gate F implementation status: PASS for the repaired T400–T434 source behavior and current local
  T550 runtime acceptance. The existing signed T434 staging package is stale against the current
  uncommitted web edit; fresh N/N+1 owner re-signing is required before release-artifact handoff.
  The controlled physical second-device evidence remains separately recorded and is not used to
  hide this current package-freshness limitation.
- Gate G status: PASS for T500–T532 and the reconciled `Txxx-S1` P5/P6 subtasks after the worker-
  execution contract repair. Immutable `movie_recap` v5 dispatches all 25 built-in JobSteps through
  their system/AI/ML/probe/media/render Redis capabilities, passes upstream Artifact refs across each
  dependency frontier, and closes the Job only after real Timeline/render/QA/clip Artifacts commit.
  T500 now has bounded exponential backoff with jitter and provider-configuration/endpoint/capability-
  scoped circuit breakers. Canonical Timeline `source_ref` now carries the same BGM rights fields that
  semantic validation and the renderer require. Fake/local providers remain the accepted boundary
  because no approved external provider credentials or model runtime were available.

## Current evidence

- Target topology and product identity are consistent across the normative set.
- Upstream reference policy, detailed capability matrix and research module audit exist.
- T002 Windows toolchain evidence is in `docs/bootstrap/T002-TOOLCHAIN-MATRIX-WINDOWS.md`.
- T003 policy and manifest are in `tests/reference-behavior/`; the manifest is empty and normal CI
  remains upstream-free.
- First deterministic native slice is T321; first Python worker slice is T323. T322/T323 now have
  live Product API/Go durable queue-to-lease-to-MinIO-Artifact-to-event, Redis-to-Python-to-
  PostgreSQL/MinIO, Python restart/XAUTOCLAIM, and transient retry/redelivery harnesses. T330–T332
  have durable queued-frontier/crash-lease recovery, review/pause/partial resume, cancel/exhaustion/
  DLQ/replay evidence; T340 has durable authenticated SSE snapshot/replay/terminal reconnect,
  bounded replay, REST equivalence and invalid-cursor coverage. T550 now has current local,
  LAN-server and controlled physical second-device acceptance evidence. Hardened/public release
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
- Current working-tree audit evidence covers the native immutable DAG catalog/runtime, durable typed
  artifact-role contracts, canonical Timeline schema/semantic validation and server-authoritative
  edit/reload/undo chain, in addition to the existing Gate F evidence covering the native immutable DAG catalog/runtime, durable typed
  review decisions, timeline builder, web SDK/editor, Tauri remote-only shell, artifact signed URL
  boundary, direct browser upload, local/LAN startup and the upload-to-MinIO/Go-worker/Python/FFmpeg
  acceptance harness, current-tree signed NSIS install/update/tamper/rollback, and the current
  physical-client T550 run. The external client authenticated, created Workspace/Project data,
  exercised upload and Job/event/SSE paths, proved no local backend dependency, and cleaned up.
- Local Functional Acceptance is T550; Local/LAN Hardened Acceptance is T603; Internet/VPS
  production is T605.
- Gate G evidence is in `docs/evidence/gate-g-dependency-preflight-20260818.md`,
  `docs/evidence/gate-g-capability-coverage.md` and the current implementation audit entry in
  `docs/memory/TEST-EVIDENCE.md`; executable acceptance is `tools/accept-gate-g.ps1`.
- The current Gate G E2E starts from Product API authentication/Project/Job commands, snapshots
  `movie_recap` v5 into PostgreSQL, executes all 25 JobSteps through Redis with the actual Python and
  Go worker processes, transfers bytes directly to/from MinIO through scoped worker transfer grants,
  and verifies a canonical multi-scene Timeline plus non-empty video/audio render, mix, QA and clip
  export Artifacts. Scheduler dependency-frontier advancement, terminal hard/soft failure aggregation
  and retry delivery are part of this executable path rather than package-only evidence.
- Gate G native coverage includes provider ports, research/script/style, TTS, ASR/alignment,
  subtitles/translation/bilingual QA, scenes/filters/VLM/characters, text and visual embeddings,
  matching/coverage/candidates, reference style, typed audio/BGM policy, Timeline compilation,
  real render/ffprobe QA, clip export and 16:9/9:16/1:1 profile reuse. Deferred matrix rows remain
  deferred and are not silently promoted.
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
  Task: Gate G — T500–T532 native media/AI capability slice
  Status: LOCAL ACCEPTANCE PASS; all 25 movie_recap v5 JobSteps execute through real Redis workers,
  typed provider retry/circuit contracts and canonical BGM rights pass, and the reviewed FFmpeg
  Timeline/render/clip path plus full repository regression/security checks are current.
  T434 signing remains a separate owner-controlled release boundary.
Boundary: Canonical execution transitions/events, queue, scheduling, leases, media/AI worker contracts,
  durable graph bootstrap, controls, checkpoint/DLQ and SSE boundary
Application code: API, persistence, media worker, Python worker, contracts, and execution adapters
Upstream relationship: research/reference only; no operational dependency
  Next: preserve the evidence and await a separately approved T600 hardening task; do not begin T600
  in this Gate G task.
```
