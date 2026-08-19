---
last_verified: 2026-08-19
source: git status/log/branch metadata; ../SPEC-AUDIT-REPORT.md; ../IMPLEMENTATION-ORDER.md; TEST-EVIDENCE.md; ../evidence/gate-g-dependency-preflight-20260818.md; ../evidence/gate-g-capability-coverage.md
owner: repository owner / task assignee
---

# Current state

## Repository baseline

| Field | Value | Evidence |
|---|---|---|
| Working branch | `implementation/bootstrap` | `git branch --show-current` |
| Baseline commit | `110aac8` (`docs: record Gate G contract repair checkpoint`) | exact local/remote source head before the final Prompt 6 repair |
| Acceptance checkpoint | 2026-08-18 final Prompt 6 working-tree acceptance | full `tools/accept-gate-g.ps1` plus Product API/Redis/worker E2E passed; implementation commit is recorded at handoff |
| Tracking branch | `origin/implementation/bootstrap` | `git status --short --branch` |
| Push/merge action | final implementation/docs commit is authorized for `origin/implementation/bootstrap` | no branch switch or merge; final SHA is recorded after commit |

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
  current status is recorded below. Gate H T600–T604 implementation and the same-machine T603
  hardened acceptance are complete; public/VPS T605 remains unstarted.
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
  scoped circuit breakers. Gate G node execution now reaches those policies through `ProviderResolver`,
  while FakeProviders are confined to the adapter registry boundary. Canonical Timeline `source_ref`
  now carries the same BGM rights fields that semantic validation and the renderer require. Go
  deliverable QA uses real blackdetect/silencedetect and commit-after-QA; subject-aware reframe
  coordinates change crop plans and rendered pixels; automatic clip selection avoids rejected
  black/silence windows. Fake/local providers remain the accepted boundary because no approved
  external provider credentials or model runtime were available.

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
  production is T605. T603 is PASS for this host's one-machine scope; physical second-device
  evidence is not claimed.
- Gate G evidence is in `docs/evidence/gate-g-dependency-preflight-20260818.md`,
  `docs/evidence/gate-g-capability-coverage.md` and the current implementation audit entry in
  `docs/memory/TEST-EVIDENCE.md`; executable acceptance is `tools/accept-gate-g.ps1`.
- The current Gate G E2E starts from Product API authentication/Project/Job commands, snapshots
  `movie_recap` v5 into PostgreSQL, executes all 25 JobSteps through Redis with the actual Python and
  Go worker processes, transfers bytes directly to/from MinIO through scoped worker transfer grants,
  and verifies a canonical multi-scene Timeline plus non-empty video/audio render, mix, QA and clip
  export Artifacts. Scheduler dependency-frontier advancement, terminal hard/soft failure aggregation
  and retry delivery are part of this executable path rather than package-only evidence.
- Prompt 8 audit on 2026-08-19 repaired the Go formatting gate across `packages/` and `services/`,
  removed a stale compatibility-worker test name, refreshed memory metadata/evidence, and reran the
  broad non-live regression suite successfully. Docker Desktop was unavailable for a fresh live
  Compose/T550/T603 rerun; unchanged prior live evidence remains the accepted Local/LAN proof.
- Gate G native coverage includes provider ports, research/script/style, TTS, ASR/alignment,
  subtitles/translation/bilingual QA, scenes/filters/VLM/characters, text and visual embeddings,
  matching/coverage/candidates, reference style, typed audio/BGM policy, Timeline compilation,
  real render/ffprobe plus black/silence QA, content-safe clip export, subject-aware pixel reframe
  and 16:9/9:16/1:1 profile reuse. Deferred matrix rows remain deferred and are not silently
  promoted.
- Gate E includes canonical transitions/events/outbox replay, Redis Streams QueuePort, DAG scheduling,
  leases/reconciliation, sandboxed media process, versioned Go/Python worker contracts, probe/thumbnail
  node, durable execution-graph bootstrap, Job/Run/Step/Review/Render commands, checkpoint/recovery,
  retry/DLQ adapters, cancellation, partial stop-after pause, and SSE. The live evidence and its
  limitations are recorded in TEST-EVIDENCE.md; no later Gate F task is implied.

## Real local/API continuation — 2026-08-19

- The prior provider-free Gate G boundary was extended with explicit real adapters and a
  policy-bound worker snapshot. Local smoke passed for Ollama `qwen2.5:3b`/`moondream`, Windows
  SAPI, CPU/int8 `faster-whisper tiny.en`, and Ollama `nomic-embed-text` (768 dimensions).
- Full Product API trace [real-local-gate-h-live-20260819-r8.json](../evidence/real-local-gate-h-live-20260819-r8.json)
  is PASS: upload/validation through PostgreSQL, Redis and MinIO; real local AI nodes; 25/25
  completed steps; TimelineVersion validation/approval; mix, Go render/ffprobe QA, clip export;
  all artifact downloads hash-match; `trace_complete=true`.
- API parity is implemented at the typed adapter/policy boundary but is not live-accepted: the
  smoke reports all five API capabilities BLOCKED because no owner credential is present. No API
  key is stored in policy, Job params, Redis, logs or evidence. Remote live quality is unverified.
- Future WEB_SESSION is a separate, unimplemented explicit adapter boundary. Cookies/session
  tokens must stay in the server SecretStore and never enter frontend state, Job params, events,
  Artifact metadata, logs or prompts.
- T605 remains unstarted; this evidence is same-machine local only. See the
  [provider capability matrix](../evidence/provider-capability-matrix-20260819.md).

## Local/LAN acceptance continuation — 2026-08-19

- Real local provider smoke and Product API traces are recorded in
  [`../evidence/local-lan-acceptance-20260819.md`](../evidence/local-lan-acceptance-20260819.md)
  and `docs/evidence/real-local-gate-h-live-20260819-r14.json` through `r17.json`,
  with current artifact-level proofs in `r31.json` and `r35.json`.
- r14 through r17 remain retained PASS traces; current-tree full artifact
  validation is PASS for landscape r31 and portrait r35. Standalone profile
  smoke is PASS for `youtube_16_9` 640x360, `shorts_9_16` 360x640 and
  `square_1_1` 480x480, with no cross-profile artifact aliasing.
- Recovery acceptance now passes locally and on the server-side LAN profile,
  including the Python Redis analysis Job after tracked process restart and
  SSE snapshot/reconnect. The Python startup capability set includes
  `analysis,ai,ml,system`.
- Physical second-device acceptance remains **NOT_PASS / EXTERNAL INPUT
  REQUIRED** in this continuation. `accept-lan-client.ps1` was run on the
  server and correctly rejected same-machine identity; rerun it from another
  Windows device before claiming physical LAN PASS.
- r10/r11/r12 remain retained failure evidence for the scene-selection/render
  repairs; r13 and r16 provide the successful portrait recovery traces. T605
  remains unstarted.

## Prompt continuation — 2026-08-19

- Artifact verification now checks signed downloads, SHA-256, nonempty
  versioned worker payloads, provenance, timed transcript/subtitle cues,
  embeddings, matching, TimelineVersion ranges, mixed audio, ffprobe streams,
  profile dimensions and deliverable QA. r31 and r35 are artifact-level PASS.
- API render requests persist the exact profile in the Job command; worker
  command resolution and artifact commits preserve per-profile outputs. A
  completed duplicate render returns safe HTTP 412 without creating another
  Job (r34).
- The local corpus now has an owned edge fixture for fast cuts, static content,
  quiet audio and silence gaps. Real scene detection on it returned 4 scenes,
  4 feature records and 4 keyframes (PASS).
- `tools/verify.ps1` now fails on nonzero native exit codes. With the reviewed
  FFmpeg/ffprobe environment, full Go/Python/Rust/TypeScript/contracts,
  independence, secrets, supply-chain and Gate H live verification pass.
- Web login surface smoke and Tauri process/build smoke pass; authenticated UI
  form submission was not performed. Resource observation pass shows healthy
  Compose, API/web listeners, GPU/process usage and no runaway process.
- r22/r23/r29 remain honest retained failure evidence. Physical second-device
  LAN acceptance remains **NOT_PASS / EXTERNAL INPUT REQUIRED** because no
  different physical client is available on this run.

## Current-tree recovery repair and final local evidence — 2026-08-19

- `real-local-gate-h-live-20260819-r38.json` plus
  `real-local-artifact-validation-20260819-r38.json` are PASS on the current
  code after the recovery repair: 25/25 steps, real local providers, signed
  downloads with matching SHA-256, subtitles/transcript/embeddings/matching,
  TimelineVersion QA and 640x360 H.264/AAC render QA.
- `real-local-ai-failure-20260819-r3.json` is an intentional local Ollama
  endpoint failure. The Product API job failed in about 13 seconds with no
  fake success; failure evidence preserves `fallback_exhausted`, transient
  category and `retryable=false`. The retry sweeper repair deduplicates the
  `retrying -> queued` Job transition when several steps retry together, so the
  job reaches terminal failure instead of remaining stuck.
- `real-local-service-restart-matrix-20260819-r38.json` and
  `real-local-resource-observation-20260819-r2.json` are PASS for service
  readiness, durable completed-job state, healthy Compose, Redis pending=0,
  observed Python workers, Ollama/GPU and no runaway process. The restart
  matrix is between completed jobs and does not prove in-flight power-loss
  chaos recovery.
- Current regression is green: `tools/verify.ps1` and
  `tools/verify-gate-h.ps1 -IncludeLive` pass with reviewed FFmpeg/ffprobe;
  Python has 38 tests. Authenticated UI submission and physical second-device
  LAN acceptance remain unproven; do not claim overall Prompt acceptance PASS.

## Final active-restart and recovery-fencing evidence — 2026-08-19

- `real-local-active-restart-probe-20260819.json` is **PASS** on the current
  tree: PostgreSQL, Redis, MinIO and the tracked API/Go/Python/web set were
  restarted while an active Product API job was running; all four services
  returned ready and the job ended `completed`.
- `real-local-recovery-fencing-20260819.json` records the current recovery
  contract: durable `attempt_id` values are carried in worker results, pending
  Redis results are reclaimable after controller restart, and result replay
  fences against the durable attempt before applying output. Final-object
  promotion verifies an existing object by size and SHA-256 before reuse.
- The final regression rerun passed with the reviewed FFmpeg/ffprobe pair;
  Python reported 38 passing tests. The strict Prompt verdict remains
  **NOT_PASS / EXTERNAL INPUT REQUIRED** because no different physical LAN
  client was available and authenticated UI submission was not performed.

## Web login smoke after SDK repair — 2026-08-19

- `web-login-smoke-20260819.json` is **PASS**. Computer Use submitted the
  configured Local Admin form at `http://192.168.1.19:3000` and reached the
  server-authoritative `Workspace dashboard`; Projects loaded and `Sign out`
  was visible.
- The cause was repaired in `packages/sdk/src/client.ts`: the default global
  `fetch` is now bound to `globalThis` before invocation. SDK tests (4) and web
  typecheck passed, and the LAN dev stack restarted successfully.
- Chrome showed a save-password prompt, which was not accepted. No project,
  upload or job mutation was performed in this smoke.

## Decision status

Open architectural/product questions: **0**. Owner-decision blockers: **0**. OQ-07 and OQ-11 are
`DEFERRED-NONBLOCKING` with concrete initial baselines; exact future VPS/OIDC/KMS/S3/monitoring
vendors are later configuration choices.

## Handoff

```text
  Task: Gate G — T500–T532 native media/AI capability slice
  Status: LOCAL ACCEPTANCE PASS; all 25 movie_recap v5 JobSteps execute through real Redis workers,
  typed provider retry/circuit contracts and canonical BGM rights pass, and the reviewed FFmpeg
  Timeline/render/clip path, real black/silence QA, subject-aware pixel reframe and full repository
  regression/security checks are current.
  T434 signing remains a separate owner-controlled release boundary.
Boundary: Canonical execution transitions/events, queue, scheduling, leases, media/AI worker contracts,
  durable graph bootstrap, controls, checkpoint/DLQ and SSE boundary
Application code: API, persistence, media worker, Python worker, contracts, and execution adapters
Upstream relationship: research/reference only; no operational dependency
  Next: preserve the Gate H evidence and obtain explicit production approval before any T605
  work; do not begin public deployment from this local checkpoint.
```
