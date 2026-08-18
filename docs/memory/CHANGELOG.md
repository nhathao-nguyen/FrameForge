---
last_verified: 2026-08-17
source: owner instruction; git history; docs/memory/
owner: repository owner / task assignee
---

# Memory changelog

## 2026-08-17 — Final Gate E acceptance repair

- Added the independent Product API durable acceptance harness covering client disconnect/reconnect,
  Redis reclaim, MinIO Artifact commit, PostgreSQL authority, partial `stop_after` pause/resume,
  crash/lease recovery, cancellation, retry exhaustion, DLQ, replay lineage and durable SSE replay.
- Fixed durable execution boundary validation, one-shot `stop_after` resume semantics, exhausted-retry
  DLQ classification, stale worker results after cancellation, stale review revision rejection and
  SSE retention-gap reset signaling.
- Re-ran the full Go integration package with PostgreSQL, Redis, MinIO, Go and real `uv`/Python
  `nh_media`; canonical Go/Python/contracts/memory/TypeScript/Rust/independence/secret/supply-chain
  checks passed. No Gate F implementation was added.

## 2026-08-17 — Gate E checkpoint handoff synchronization

- Recorded checkpoint `2fa58d4` (`checkpoint: close Gate E T322-T350 execution slice`) as the
  committed baseline on `implementation/bootstrap`.
- Confirmed the checkpoint is pushed to `origin/implementation/bootstrap`, the local and remote
  hashes match, and the worktree is clean.
- Synchronized current-state and evidence handoff metadata; T400/Gate F remains the next task.

## 2026-08-17 — Gate E T322–T350 Local/LAN execution closure

- Completed the durable Job/Run/Step command surface: create/list/get/start/pause/resume/cancel/retry,
  pipeline runs/steps, durable Review required/approve/reject-edit/resume, and Render list/get/cancel/retry
  with immutable successor links.
- Fixed PostgreSQL live issues found by the runtime harness: typed JSONB command extraction, review
  bigint/UUID casts, cursor closure before same-transaction mutations, and aggregate `stop_after` pause.
- Added deterministic checkpoint/recovery planning, fail-closed active-lease/review handling, atomic
  graph controls, loopback-authenticated SSE snapshot/replay/reconnect tests, and live evidence for native
  Artifact execution, Python retry/redelivery, worker restart/XAUTOCLAIM, review/replay/cancel commands.
- Final local verification and memory/evidence validation pass. T400/Gate F, desktop T550, hardened T603,
  backup/restore and public/VPS T605 remain outside this task.

## 2026-08-17 — Gate C+D specification repair before Gate E

- Added deterministic media validation fixtures for malformed containers, invalid ffprobe output/
  streams, pathological stream/output limits and probe timeout classification; rejected results never
  become `Validated`.
- Connected Timeline create/version/approval/lock/validate/render paths to fail-closed durable
  Asset/Artifact/Scene/Narration resolution with Workspace/Project/state checks.
- Added RenderProfile draft lifecycle service, active immutability/lifecycle migration guard and exact
  profile snapshot lookup; Render creation no longer auto-creates profiles.
- Reinstalled the pinned Rust 1.97.1 toolchain with cargo/rustfmt/clippy and installed Docker Desktop
  4.87.0 per-user with WSL2; full canonical verification, 0008 migration repeat/app-role denial and
  live private MinIO conformance now pass. T300/Gate E execution was not started.

## 2026-08-17 — T002/T003 pre-code bootstrap evidence

- Pinned the validated Windows baseline with `.go-version`, `.python-version`, `.node-version`,
  `package.json` package-manager metadata and `rust-toolchain.toml`.
- Recorded exact Go/Python/uv/Node/pnpm/Rust/Tauri/FFmpeg/ffprobe/Docker evidence and smoke checks
  in `docs/bootstrap/T002-TOOLCHAIN-MATRIX-WINDOWS.md`.
- Added the T003 reference-behavior policy and empty manifest under `tests/reference-behavior/`.
- Reconciled current branch/status memory and made the branch validator compare against the recorded
  branch rather than a stale hard-coded branch.
- No application code, database schema, service/worker/client runtime or upstream dependency was added.

## 2026-08-17 — Final Local/LAN decision closure

- Recorded owner ratification of LocalAuthProvider, Workspace-first authorization, encrypted
  SecretStore records, Timeline JSONB/domain commands, Redis Streams, SSE and MinIO/S3 semantics.
- Closed all 15 former OQs; OQ-07/OQ-11 are deferred nonblocking with concrete baselines.
- Marked T004 complete, added T323 Python worker proof and T550 Local Functional Acceptance, and
  separated T603 Local/LAN Hardened Acceptance from T605 Internet/VPS Production.
- Updated Local/LAN HTTP/CORS/binding, retention/delete, health and canonical Windows startup targets.
- No application code, migration, service/client/worker scaffold or runtime configuration was added.

## 2026-08-17 — Independent specification refactor

- Replaced the former wrapper/migration/compatibility architecture with an independent NH-Media
  architecture.
- Corrected names and topology: NH-Media, `nh_media`, `github.com/nhathao-nguyen/NH-Media`, Go
  Product API, Go media workers, isolated Python ML/AI workers, Next.js web and Tauri 2 client.
- Added upstream reference policy and detailed capability matrix; converted the module audit and
  prior source baseline into non-normative research evidence.
- Added native candidate evaluation/selection, ReferenceStyleAnalysis, scheduling/distributed
  evolution and safer extension direction.
- Replaced migration phases with independent implementation order and a first native vertical slice.
- Made Local/LAN certification a release milestone before public production.
- Updated the memory model and OQ/task mirrors. No application code, runtime, migration or
  deployment implementation was added.

Final documentation/link/table/JSON/task/OQ/terminology/change-boundary checks are recorded in
[`TEST-EVIDENCE.md`](TEST-EVIDENCE.md). T002/T003 then T004 are the next evidence gates.

## 2026-08-17 — Gate E T300–T340 execution foundation

- Added canonical Job/Run/Step transition validation, transactional event/outbox writes, bounded
  outbox replay/publishing, Redis Streams QueuePort, deterministic dependency scheduling and hashed
  lease/reconciliation boundaries.
- Added an argv-only sandboxed FFmpeg/ffprobe process boundary, versioned Go/Python worker contracts,
  probe/thumbnail node, deterministic Python worker and Go-to-uv cross-language integration.
- Added durable execution-graph bootstrap, native Job controls/status/events/SSE, checkpoint/reuse and
  retry/dead-letter adapters. Focused regression gates pass; full Redis-to-worker-to-Artifact runtime,
  crash/resume/review/replay/reconnect acceptance remains the next evidence boundary.
- Added the lease-aware controller/result-reconciler boundary, Python Redis worker runtime, live ephemeral
  Redis integration harness and process-tree cleanup. The live Redis/Python slice passes; Artifact
  staging/commit and PostgreSQL-backed end-to-end acceptance remain intentionally unclaimed.
- Added the durable first-slice result adapter: deterministic report staging/promote through MinIO,
  SQL Artifact producer linkage, lease-guarded Step completion and `node.completed` event. The live
  PostgreSQL/Redis/MinIO harness passes and retains only soft-deleted fixture state plus append-only audit.
- Added the combined Python durable harness: an isolated ML pipeline reaches the real `nh_media` Redis
  worker, result reconciler, PostgreSQL Step state and MinIO Artifact commit. Restart/retry redelivery,
  terminal aggregate and client/LAN acceptance remain the next gates.
- Added bounded Python XAUTOCLAIM recovery and a live worker-restart harness; a second consumer reclaims
  one pending delayed command without duplicate result publication. Failure-category retry remains open.
- Added durable retry scheduling: worker safe-error categories are validated, transient retry is bounded by
  node `max_attempts` and policy backoff, Job/Run/Step terminal aggregation is reconciled, and queued retry
  messages remain ID-only. The live Python/PostgreSQL/Redis/MinIO harness now passes attempt-1 failure,
  backoff, attempt-2 redelivery, Artifact commit and completed aggregate state.
- Corrected Redis zero-duration QueuePort polls so controller dispatch loops cannot block forever on Redis
  `BLOCK 0`; the correction is covered by the live retry harness.
- Added stable-ID recovery for already-queued JobSteps after a PostgreSQL-to-Redis enqueue interruption;
  subsequent start/resume calls can republish the durable frontier without treating Redis as state.
