---
last_verified: 2026-08-18
source: owner instruction; git history; docs/memory/
owner: repository owner / task assignee
---

# Memory changelog

## 2026-08-18 — Gate F current physical-client closure

- Verified new external run `lan-client-1353be68-e9ba-41d7-9ee1-b2f97340452e` from
  `DESKTOP-92ICS6C` (`192.168.1.29`) against server `192.168.1.18`.
- The current script hash matched the working-tree script; authentication, Workspace/Project,
  upload, completed Job, REST replay, SSE snapshot/reconnect, no-local-backend and cleanup all
  passed with `external_to_server=true`.
- With current T434 signed N/N+1 acceptance already passing, Gate F is now `PASS` for T400–T434 and
  T550. T603/hardened and public-production gates remain separate.

## 2026-08-18 — Current-tree T434 signed acceptance

- Built and verified `t434-current` and `t434-current-next` with the owner key and exact LAN origin;
  the fresh per-user NSIS harness passed signature identity, install/launch, update/launch, tamper
  rejection and rollback-pointer checks.
- Copied the current LAN client acceptance script to the reachable second-device share at
  `\\192.168.1.29\Users\Public\accept-lan-client-current.ps1`; RPC/WinRM is unavailable, so the
  client must be launched locally on that physical Windows device.
- Gate F remains `NOT PASS` only for the current physical second-device T550 run.

## 2026-08-18 — Current-tree T550 recovery rerun

- Reissued the native policy-complete MovieRecap graph as immutable `movie_recap` v3 because the
  durable developer database already contained a different v2 content hash; no pipeline version
  was mutated in place.
- Reran current-tree local and exact-LAN server acceptance with tracked API/Go/Python/web restart
  recovery. Both durable Jobs and the SSE snapshot remained `completed`/recoverable; same-host LAN
  traffic still is not physical second-device evidence.
- Gate F remains `NOT PASS`: current-tree T434 install/update evidence with owner signing input and
  a controlled physical second-device T550 run are still outstanding.

## 2026-08-18 — Current-tree Gate F repair audit

- Repaired pipeline schema/policy validation, fail-closed timeout/retry/soft-dependency runtime,
  canonical native workflow persistence, exact review-resource validation, structural-vs-durable
  timeline validation, immutable timeline commands, direct artifact upload/download boundaries and
  web/SDK review/editor flows.
- Current live acceptance passed for T550 Local and exact LAN server-path: direct storage upload,
  dedicated Go validation/Artifact Job, SSE snapshot/live/reconnect and Product API → Redis Streams
  → Python `nh_media` analysis Job all reached exact `completed`.
- Gate F remains `NOT PASS`: current-tree T434 NSIS install/update evidence has not run with owner
  signing input, and no controlled physical second-device T550 run was performed in this turn.

## 2026-08-18 — Gate F physical-client acceptance closure

- Captured sanitized physical-client evidence from `DESKTOP-92ICS6C` (`192.168.1.29`) against LAN
  server `192.168.1.18`; Windows PowerShell 5.1 run
  `lan-client-a384adea-f665-4cd0-ba01-1035e8acfef2` reports `status=PASS` and
  `external_to_server=true`.
- The external client passed API/web reachability, authentication, Workspace/Project, upload
  initiation/abort, completed Job status, REST event replay, SSE initial snapshot/reconnect replay,
  no-local-backend and project-cleanup checks. Credentials were interactive and are not evidence.
- With current-tree T434 exact-origin signing/signature/tamper/rollback evidence and canonical
  regression passing, T550 reports `LOCAL FUNCTIONAL ACCEPTANCE: PASS`, and T400–T434/T550 satisfy
  the narrow closure criteria: `GATE F: PASS`.
- Re-ran local and exact-LAN acceptance, current signed-package verification, canonical verification,
  uncached Go tests, focused pipeline/integration tests, recursive TypeScript checks, contracts/memory
  validation and Rust formatting/compile checks; all pass with only documented engine/line-ending warnings.

## 2026-08-18 — Current exact-origin LAN desktop and lifecycle closure

- Repaired Windows process ownership so LAN restart records the API/web listener PIDs, stops child
  processes before parents, rejects occupied fixed ports and reports success only after API live/ready,
  web health and packaged-Tauri CORS checks pass.
- Moved desktop frontend production compilation into a copied `.web-build` workspace outside the live
  web tree; the temporary workspace is removed and the running LAN web remains HTTP 200 after build.
- Built `t434-lan-signed` with exact API origin `http://192.168.1.18:8080`; external signature,
  manifest/public-key fingerprint, tamper rejection and rollback pointer pass. T434 is complete for
  the current tree; Gate F still awaits the final physical-client T550 JSON PASS.

## 2026-08-18 — T434 signed desktop staging closure

- Built the static web export and Windows NSIS staging bundle from the repository-root package flow.
- Added external public-key Minisign verification, public-key fingerprint recording, fail-closed
  package verification and a temporary tamper-rejection test.
- Signed staging artifact `NH-Media_0.1.0_x64-setup.exe` and its `.sig` sidecar passed manifest,
  signature and rollback-pointer checks. Private key/password remain external and are not recorded.
- T434 is complete; overall Gate F remains NOT PASS only because physical second-device T550 LAN
  evidence is still pending.

## 2026-08-18 — Gate F narrow acceptance closure repair

- Added exact-IP LAN startup validation and CORS/web endpoint defaults; LAN no longer silently falls
  back to localhost when the server has multiple interfaces.
- Added `tools/accept-lan-client.ps1` and the second-device runbook. It records client/server
  identity, authentication, Project/Job/upload-initiation/SSE checks and rejects loopback or same-host
  execution; no physical second-device PASS is claimed.
- Connected the static web export to Tauri release builds, enabled the Windows NSIS bundle and made
  desktop package manifest/checksum/signature verification and rollback fail closed without an
  external key. The unsigned NSIS build is development evidence only.
- Re-ran live local and LAN server-path acceptance. Gate F remains NOT PASS pending external signing
  material and controlled second-device evidence.

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

## 2026-08-18 — LAN web restart/port collision repair

- Repaired `tools/dev.ps1` process-tree shutdown so tracked `pnpm`/Next.js children and stale
  NH-Media web entrypoints are stopped together during restart.
- LAN web startup now clears generated `.next` state and requests port `3000`, preventing a stale
  development server from silently moving the client endpoint to another port.
- Server-side checks after repair returned HTTP 200 for `127.0.0.1:3000`, `192.168.1.18:3000` and
  `/api/v1/live`; physical second-device acceptance remains the outstanding Gate F evidence.
- Corrected the client-only SSE probe to use fully qualified `System.Net.Http` types and explicitly
  load that assembly for Windows PowerShell 5.1. Physical-client runs authenticated and completed
  the Project/upload/Job/event-replay path before exposing this script-side compatibility defect;
  no password or token was recorded.
- Hardened the client baseline to Windows PowerShell 5.1/PowerShell 7 with explicit RFC1918 checks,
  bounded REST/SSE waits, proxy-free LAN SSE, interactive credentials by default and OS/runtime
  evidence. Added exact-IP LAN desktop packaging: the static frontend endpoint and Tauri CSP are
  merged at build time, and the API LAN profile allowlists the packaged Windows Tauri origin without
  wildcard CORS. Desktop frontend builds now use a copied and disposable `.web-build` workspace so
  signing cannot corrupt or interrupt the running LAN web dev server. The exact-origin LAN flavor was
  subsequently regenerated and verified above.
