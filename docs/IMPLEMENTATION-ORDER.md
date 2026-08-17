# Implementation Order

NH-Media begins as an independent implementation. Movie Narrator research is not a prerequisite for
normal build, test or runtime. Assign one reviewable task at a time. The 2026-08-17 owner
ratification closes all architecture/product OQs; deferred vendor/optimization choices do not block
implementation.

Every task handoff includes: boundary, tests, independence, security, data/rollback, open questions
and next task.

## Gate A — Specification and research provenance

### T000 — Ratify the independent NH-Media specification

- **Goal:** accept NH-Media identity, architecture, reference policy and final audit.
- **Files/modules:** documentation only.
- **Dependencies:** none.
- **Notes:** confirm no upstream runtime/adapter/compatibility architecture and no application diff.
- **Tests:** links, terminology, capability coverage, independence assertions, `git diff --check`.
- **DoD:** owner accepts spec; status/audit/memory agree; application gate may proceed task-by-task.

### T001 — Record upstream research provenance

- **Goal:** preserve URL/commit/license/module observations as research evidence only.
- **Files/modules:** `docs/baselines`, upstream audits/policy.
- **Dependencies:** T000.
- **Notes:** no source checkout or executable image becomes a product dependency.
- **Tests:** provenance fields, capability coverage and absence from dependency/package graphs.
- **DoD:** another agent can understand observations without needing upstream for normal work.

### T002 — Pin the NH-Media toolchain matrix

- **Goal:** choose reproducible Go, Python/uv, Node/pnpm, Rust/Tauri and FFmpeg versions.
- **Files/modules:** environment/lock/CI metadata and evidence.
- **Dependencies:** T000.
- **Notes:** Go module is `github.com/nhathao-nguyen/NH-Media`; Python package is `nh_media`.
- **Tests:** clean-room version/install/lock/codec checks.
- **DoD:** setup reproduces without system pip, floating toolchains or upstream packages.

### T003 — Define reference-behavior fixture policy

- **Goal:** record lawful/useful fixtures, tolerances and intentional-divergence format.
- **Files/modules:** `tests/reference-behavior` policy/manifests only.
- **Dependencies:** T001.
- **Notes:** recorded outputs are optional comparisons; normal CI never executes upstream.
- **Tests:** provenance/manifest validation and empty-upstream-environment check.
- **DoD:** fixtures cannot be mistaken for a compatibility/runtime gate.

### T004 — Certify the documentation/independence gate

- **Goal:** run the final pre-code audit.
- **Files/modules:** audit, memory and validation tooling.
- **Dependencies:** T000, T001.
- **Notes:** owner ratification is recorded; T002/T003 remain separate bootstrap evidence before T100
  and do not reopen this architecture gate.
- **Tests:** docs links/anchors, task/OQ/status parity, forbidden architecture/source scan.
- **DoD:** audit says `SPEC READY FOR IMPLEMENTATION`, open architecture questions are zero, owner
  approval is recorded and no application code was added.

## Gate B — Repository and service foundation

### T100 — Create the independent repository skeleton

- **Goal:** create approved web/desktop/API/media-worker/ml-worker/contracts/SDK/infra/test boundaries.
- **Files/modules:** layout/package metadata only.
- **Dependencies:** T002, T003, T004.
- **Notes:** no `legacy-compat`, upstream submodule/vendor tree or media/domain behavior.
- **Tests:** import/dependency graph and upstream-absence scan.
- **DoD:** Go, `nh_media` and clients compile/import only through approved boundaries.

### T101 — Add language-neutral primitives

- **Goal:** define IDs, UTC/time ranges, revisions/ETags and safe errors.
- **Files/modules:** `packages/shared-contracts` and tests.
- **Dependencies:** T100.
- **Notes:** no framework/ORM/provider/storage classes.
- **Tests:** schema/serialization/invalid input/redaction snapshots.
- **DoD:** Go/Python/clients consume the same fixtures.

### T102 — Add configuration and redaction boundaries

- **Goal:** separate API, media worker, ML worker and provider configuration.
- **Files/modules:** config packages, `.env.example`, tests.
- **Dependencies:** T101.
- **Notes:** implement SecretStore refs and encrypted-record boundary using a server-owned master key;
  no arbitrary executable from Project config.
- **Tests:** precedence, unknown fields and log/event/checkpoint redaction.
- **DoD:** no secret is serialized or logged.

### T103 — Provision private PostgreSQL

- **Goal:** pinned development database with migration/app roles.
- **Files/modules:** infrastructure/postgres.
- **Dependencies:** T100.
- **Notes:** non-default credentials and named data volume.
- **Tests:** clean start/stop, health, privilege denial and restart.
- **DoD:** app role connects but cannot migrate schema.

### T104 — Provision private Redis

- **Goal:** pinned coordination service for later QueuePort/live events.
- **Files/modules:** infrastructure/redis.
- **Dependencies:** T100.
- **Notes:** no queue semantics yet; not source of truth.
- **Tests:** auth/private bind/restart/policy.
- **DoD:** no unauthenticated/public access.

### T105 — Provision private S3-compatible storage

- **Goal:** pinned MinIO/S3 development bucket.
- **Files/modules:** infrastructure/object-storage.
- **Dependencies:** T100.
- **Notes:** non-default credentials; persistent volume and ratified retain-until-explicit-delete defaults.
- **Tests:** health, private ACL, multipart and restart.
- **DoD:** scoped access works and anonymous access fails.

### T106 — Implement the Go Product API shell

- **Goal:** `/api/v1`, request/correlation IDs, safe errors and bounded HTTP settings.
- **Files/modules:** `services/api` transport/application shell.
- **Dependencies:** T101–T105.
- **Notes:** include AuthPort/LocalAuthProvider shell and explicit CORS/profile bounds; no media/domain
  feature and no Python/FFmpeg execution.
- **Tests:** startup, OpenAPI/error smoke, body/CORS bounds, dependency scan.
- **DoD:** API shell runs with mocked ports and no long compute inline.

### T107 — Implement health/readiness

- **Goal:** separate liveness, readiness and protected deep diagnostics.
- **Files/modules:** API/worker health contracts.
- **Dependencies:** T106.
- **Notes:** public responses reveal minimal detail.
- **Tests:** dependency down, timeout, drain and disclosure.
- **DoD:** readiness accurately rejects work while liveness remains meaningful.

### T108 — Establish CI and independence gates

- **Goal:** lint/type/test/security/contracts/SBOM and upstream-absence checks.
- **Files/modules:** CI/workflow/tooling.
- **Dependencies:** T100–T107.
- **Notes:** no inherited advisory exception without owner/scope/expiry.
- **Tests:** intentionally failing checks and clean baseline.
- **DoD:** required checks prove product graphs contain no upstream source/package/image/import.

## Gate C — Domain, database and Product resources

### T200 — Add migration framework

- **Goal:** immutable PostgreSQL migration conventions and harness.
- **Files/modules:** migrations/runner/tests.
- **Dependencies:** T103, T108.
- **Notes:** framework only.
- **Tests:** empty upgrade, repeat, failed rollback, app-role denial.
- **DoD:** later schema tasks are independently reviewable.

### T201 — Add identity and Workspace tables

- **Goal:** users/workspaces/members/API keys.
- **Files/modules:** migration/repositories/tests.
- **Dependencies:** T200.
- **Notes:** LocalAuth identity/session hash only, default Workspace bootstrap and Workspace-scoped repositories.
- **Tests:** FK/index/role/cross-Workspace negative corpus.
- **DoD:** ownership scope is structural.

### T202 — Add Workflow/Pipeline/Node tables

- **Goal:** persist versioned built-in graph definitions.
- **Files/modules:** migration/repositories/validators.
- **Dependencies:** T200.
- **Notes:** active built-in definitions immutable; no public authoring or executor.
- **Tests:** version/hash/dependency and mutation rejection.
- **DoD:** native graph data stores without running work.

### T203 — Add SecretStore, ProviderConfiguration and RenderProfile tables

- **Goal:** encrypted secret records, redacted provider metadata and versioned render profiles.
- **Files/modules:** migration/repositories/tests.
- **Dependencies:** T200–T201.
- **Notes:** server master key remains outside PostgreSQL; no plaintext secret/provider SDK.
- **Tests:** encryption/AAD/key-version/rotation plus status/revision/uniqueness/redaction/immutability.
- **DoD:** snapshots resolve by ID/version.

### T204 — Add Project/Asset/upload tables

- **Goal:** Project, Asset and UploadSession lifecycles.
- **Files/modules:** migration/repositories/tests.
- **Dependencies:** T201–T203.
- **Notes:** Asset is not an object key.
- **Tests:** ownership/status/revision/active-upload uniqueness.
- **DoD:** upload intent persists without binary data.

### T205 — Add Job/PipelineRun/JobStep/review tables

- **Goal:** canonical execution and review state records.
- **Files/modules:** migration/repositories/tests.
- **Dependencies:** T202, T204.
- **Notes:** immutable command snapshots and one active run.
- **Tests:** state checks, aliases rejected, concurrent active-run rejection.
- **DoD:** schema represents Product execution without upstream task terms.

### T206 — Add Artifact/checkpoint/DLQ tables

- **Goal:** immutable blob manifests and recovery metadata.
- **Files/modules:** migration/repositories/tests.
- **Dependencies:** T204–T205.
- **Notes:** bytes outside PostgreSQL; no raw path.
- **Tests:** locator uniqueness, provenance, checkpoint refs, canonical roles.
- **DoD:** all execution/blob refs are durable and typed.

### T207 — Add Script/Narration/Scene/Character/Analysis tables

- **Goal:** content and intelligence aggregates.
- **Files/modules:** migration/repositories/tests.
- **Dependencies:** T204–T206.
- **Notes:** Analysis supports `reference_style`; user confirmations are preserved.
- **Tests:** versions, ranges, ownership, provenance and merge rules.
- **DoD:** core content entities match `02`/`03`.

### T208 — Add Timeline/Render/candidate tables

- **Goal:** immutable TimelineVersion/Render and later candidate/evaluation structures.
- **Files/modules:** migration/repositories/tests.
- **Dependencies:** T203, T206–T207.
- **Notes:** candidate tables may be feature-gated but schema direction is native.
- **Tests:** hashes/current pointers/render dedupe/candidate selection audit.
- **DoD:** exact Timeline/Profile and candidate lineage are representable.

### T209 — Add events/outbox/idempotency tables

- **Goal:** ordered events, transactional outbox and idempotent mutations.
- **Files/modules:** migration/repositories/tests.
- **Dependencies:** T201, T205.
- **Notes:** events append-only; sequence allocated per Job.
- **Tests:** concurrent sequence, duplicate keys, publisher retry identity.
- **DoD:** state/event/outbox can commit atomically.

### T210 — Implement auth context/scoped repositories

- **Goal:** enforce identity, Workspace and roles across resource access.
- **Files/modules:** auth adapter/application/repositories.
- **Dependencies:** T106, T201.
- **Notes:** never trust scope from request payload.
- **Tests:** roles, suspension, guessed IDs and API-key scopes.
- **DoD:** access fails closed.

## Gate D — Storage, upload and native Product APIs

### T220 — Implement StoragePort and LocalStorage

- **Goal:** secure local storage conformance adapter.
- **Files/modules:** storage port/adapter/tests.
- **Dependencies:** T101, T206.
- **Notes:** LocalHandle is non-serializable; explicit root.
- **Tests:** traversal/symlink/preconditions/stage/promote/cleanup.
- **DoD:** reusable conformance suite passes.

### T221 — Implement S3/MinIO adapter

- **Goal:** multipart/presign/version adapter with same contract.
- **Files/modules:** storage adapter/integration tests.
- **Dependencies:** T105, T220.
- **Notes:** exact method/key/TTL; no credential logs.
- **Tests:** conformance, abort/expiry/range/ACL/transient error.
- **DoD:** Local and S3-compatible adapters are interchangeable.

### T222 — Implement Artifact commit service

- **Goal:** stage, validate, promote and atomically publish Artifacts.
- **Files/modules:** application/storage/repository integration.
- **Dependencies:** T209, T220–T221.
- **Notes:** orphan reconciliation and idempotent canonical roles.
- **Tests:** storage/DB split failures, duplicate/checksum/cross-Project cases.
- **DoD:** incomplete bytes never appear committed.

### T223 — Implement upload-session API

- **Goal:** direct multipart upload orchestration.
- **Files/modules:** Product API/application/storage.
- **Dependencies:** T204, T210, T221–T222.
- **Notes:** video bytes bypass API and Redis.
- **Tests:** initiate/parts/complete/abort/expiry/idempotency/ACL.
- **DoD:** completion creates a validation Job, not a ready Asset.

### T224 — Implement validation/probe boundary

- **Goal:** independently validate checksum/MIME/magic/ffprobe/security limits.
- **Files/modules:** Go media worker probe node.
- **Dependencies:** T205–T206, T222–T223.
- **Notes:** security failure quarantines; no soft bypass.
- **Tests:** spoofed/oversized/malformed/resource-bomb corpus.
- **DoD:** Asset becomes ready only after committed validated Artifact.

### T230 — Implement Project/Asset REST resources

- **Goal:** native Project/Asset CRUD and upload metadata.
- **Files/modules:** API/application tests.
- **Dependencies:** T210, T223–T224.
- **Notes:** no compatibility routes or raw paths.
- **Tests:** auth, pagination, ETag, idempotency, safe errors.
- **DoD:** web/desktop can manage Project/Asset through `/api/v1`.

### T231 — Implement Script/Narration APIs

- **Goal:** immutable ScriptVersion and Narration requests.
- **Files/modules:** API/application tests.
- **Dependencies:** T207, T210.
- **Notes:** no provider output paths.
- **Tests:** version conflict, approval, exact refs and role denial.
- **DoD:** exact ScriptVersion can drive a Narration Job.

### T232 — Implement Timeline schema/validator

- **Goal:** enforce JSON Schema and cross-reference/range/user-origin rules.
- **Files/modules:** contracts/validator/negative corpus.
- **Dependencies:** T208.
- **Notes:** renderer source remains document, not projection.
- **Tests:** all rules in `06` and deterministic content hash.
- **DoD:** invalid TimelineVersion cannot be approved/rendered.

### T233 — Implement Timeline/Render REST resources

- **Goal:** version/edit/approve/lock and Render request resources.
- **Files/modules:** API/application tests.
- **Dependencies:** T210, T232.
- **Notes:** typed domain commands, optimistic concurrency and exact version refs; full document only
  through validated create/import.
- **Tests:** conflicts, role checks, profile dedupe and user-origin preservation.
- **DoD:** native clients create and render versioned Timelines.

### T234 — Implement Scene/Analysis/candidate APIs

- **Goal:** expose native intelligence results and candidate lineage.
- **Files/modules:** API/application tests.
- **Dependencies:** T207–T210.
- **Notes:** ReferenceStyleAnalysis exposes abstract metrics only.
- **Tests:** ownership/provenance/selection immutability and pagination.
- **DoD:** analyses/candidates are first-class native resources.

### T235 — Implement ProviderConfiguration API

- **Goal:** redacted provider lifecycle and validation commands.
- **Files/modules:** API/application/secret adapter tests.
- **Dependencies:** T203, T210.
- **Notes:** plaintext never returned or persisted.
- **Tests:** rotation/disable/revoke/validation/redaction/authorization.
- **DoD:** workers resolve scoped bindings without exposing secrets.

## Gate E — Durable execution and first independent vertical slice

### T300 — Implement canonical state transition service

- **Goal:** guard Job/Run/Step transitions and atomic event writes.
- **Files/modules:** Go application/domain tests.
- **Dependencies:** T205, T209.
- **Notes:** state vocabulary is exact.
- **Tests:** every valid/invalid transition and concurrency race.
- **DoD:** DB/API/event state cannot diverge.

### T301 — Implement outbox publisher and durable event replay

- **Goal:** publish idempotently and replay ordered events.
- **Files/modules:** orchestration/event tests.
- **Dependencies:** T209, T300.
- **Notes:** Redis is fan-out/delivery only.
- **Tests:** crash windows, duplicate publish, sequence gaps.
- **DoD:** events reconstruct from PostgreSQL.

### T310 — Implement Redis QueuePort

- **Goal:** at-least-once bounded delivery behind a port.
- **Files/modules:** queue adapter/load tests.
- **Dependencies:** T104, T301.
- **Notes:** Redis Streams consumer groups; message contains IDs only and PostgreSQL remains truth.
- **Tests:** duplicates, reclaim, delayed retry, restart and bounds.
- **DoD:** queue loss does not lose durable Job state.

### T311 — Implement generic dependency scheduler

- **Goal:** move dependency-satisfied steps to ready/queued.
- **Files/modules:** orchestration tests.
- **Dependencies:** T202, T300, T310.
- **Notes:** no workflow-specific branches.
- **Tests:** branch/join/optional/blocked/partial cases.
- **DoD:** deterministic frontier and bounded publishing.

### T312 — Implement lease/heartbeat/reconciliation

- **Goal:** safe worker claims and lost-worker recovery.
- **Files/modules:** execution-state port/tests.
- **Dependencies:** T300, T310.
- **Notes:** stale tokens fail closed.
- **Tests:** expiry, split brain, competing workers and drain.
- **DoD:** one canonical attempt can commit.

### T313 — Implement sandboxed MediaProcessPort

- **Goal:** reviewed argv-only FFmpeg/ffprobe execution.
- **Files/modules:** Go media worker/process tests.
- **Dependencies:** T224, T312.
- **Notes:** resource/time/network/path policy enforced.
- **Tests:** injection, timeout/cancel/tree kill, malicious media.
- **DoD:** process results are safe manifests, not durable paths.

### T320 — Implement versioned Go/Python worker contracts

- **Goal:** commands/progress/results/errors/capabilities shared across workers.
- **Files/modules:** contracts, Go/Python validators and fixtures.
- **Dependencies:** T101, T312–T313.
- **Notes:** no pickle/gob/framework types.
- **Tests:** cross-language round-trip/version rejection/redaction.
- **DoD:** worker implementation is replaceable without state redesign.

### T321 — Implement independent probe/thumbnail node

- **Goal:** first real NH-Media media node.
- **Files/modules:** Go media worker node/tests.
- **Dependencies:** T222, T313, T320.
- **Notes:** consumes Artifact ref; produces probe/thumbnail Artifacts.
- **Tests:** deterministic fixture, progress, cancel, duplicate commit.
- **DoD:** node runs with no upstream package/source/environment.

### T322 — Prove first end-to-end NH-Media slice

- **Goal:** client/API→Job→queue→worker→Artifact→event/result.
- **Files/modules:** integration/e2e/LAN harness.
- **Dependencies:** T230, T300–T321.
- **Notes:** browser and Tauri fixture use native Product API.
- **Tests:** client disconnect, worker restart, retry and upstream-absence scan.
- **DoD:** independent Local/LAN vertical slice is green.

### T323 — Prove the first Python worker integration slice

- **Goal:** route one minimal versioned task through Redis Streams to a real `nh_media` worker.
- **Files/modules:** Python worker shell, shared contracts and integration harness.
- **Dependencies:** T320, T322.
- **Notes:** use lightweight analysis or deterministic protocol work; no hosted LLM, Whisper, CUDA,
  VLM or TTS prerequisite.
- **Tests:** cross-language version rejection, health, retry/restart, result/Artifact commit and redaction.
- **DoD:** Go API→Job→Redis Streams→Python worker→Artifact/result completes with no upstream runtime.

### T330 — Implement checkpoints and crash resume

- **Goal:** persist compatible references/fingerprints and resume descendants.
- **Files/modules:** orchestration/storage tests.
- **Dependencies:** T206, T312, T322.
- **Notes:** no worker memory/path in checkpoint.
- **Tests:** crash at each boundary, stale input and reuse.
- **DoD:** first incomplete compatible node resumes without duplicate output.

### T331 — Implement pause/partial/review commands

- **Goal:** `start_from`, `stop_after` and durable review semantics.
- **Files/modules:** application/API/event tests.
- **Dependencies:** T330.
- **Notes:** pause and review remain distinct.
- **Tests:** valid/invalid boundaries, edit-then-resume and browser loss.
- **DoD:** commands need no node-specific hack.

### T332 — Implement cancel/retry/DLQ

- **Goal:** bounded cancellation, classified retry and new-Job replay.
- **Files/modules:** orchestration/API tests.
- **Dependencies:** T300, T330.
- **Notes:** validation/security/user errors do not retry.
- **Tests:** kill/reconcile/backoff/exhaustion/replay/audit.
- **DoD:** terminal history is immutable.

### T340 — Implement SSE progress

- **Goal:** snapshot/replay/live stream with canonical envelope.
- **Files/modules:** API/event/client tests.
- **Dependencies:** T301, T331–T332.
- **Notes:** commands remain REST.
- **Tests:** order/dedupe/reconnect/gap/auth/backpressure.
- **DoD:** client reconstructs same state as REST.

### T341 — Evaluate a future bidirectional transport requirement

- **Goal:** document and validate a genuinely bidirectional feature before adding WebSocket.
- **Files/modules:** decision/evidence only unless a future approved feature requires implementation.
- **Dependencies:** T340.
- **Notes:** `DEFERRED-NONBLOCKING`; SSE remains primary and progress is not sufficient justification.
- **Tests:** requirement, threat model and contract compatibility review if activated.
- **DoD:** either remain explicitly deferred or approve a scoped transport task without blocking clients.

### T350 — Implement Job/Run/Step/Render commands

- **Goal:** expose native execution resources/commands.
- **Files/modules:** API/application tests.
- **Dependencies:** T230–T235, T300–T340.
- **Notes:** no upstream route/status/config aliases.
- **Tests:** full command/state/HTTP/idempotency/pagination matrix.
- **DoD:** native API traces exact state and Artifact refs.

## Gate F — Native workflow and clients

### T400 — Implement DAG validator/activation

- **Goal:** validate keys/cycles/schemas/dependencies/capabilities/policies.
- **Files/modules:** pipeline validator tests.
- **Dependencies:** T202, T320.
- **Notes:** definition is data.
- **Tests:** complete negative corpus and immutable active hash.
- **DoD:** only complete native node contracts activate.

### T401 — Implement node runtime conformance

- **Goal:** execute fake DAG nodes through all state/policy paths.
- **Files/modules:** runtime/conformance tests.
- **Dependencies:** T311–T313, T330, T400.
- **Notes:** no real provider/media capability.
- **Tests:** branch/join/retry/timeout/cancel/review/idempotency.
- **DoD:** fake graph passes all contracts.

### T402 — Register built-in movie-recap graph

- **Goal:** encode independent node catalog and automatic/studio policies.
- **Files/modules:** Pipeline definition/seed/tests.
- **Dependencies:** T400–T401.
- **Notes:** no upstream aliases or executor fallback.
- **Tests:** graph snapshot, start/stop and required policy fields.
- **DoD:** graph activates and produces deterministic JobSteps.

### T410 — Implement review orchestration

- **Goal:** connect Script/Timeline/match reviews to generic review nodes.
- **Files/modules:** application/API/event tests.
- **Dependencies:** T231, T233–T234, T331, T402.
- **Notes:** exact version/actor/audit and declared rejection action.
- **Tests:** approve/edit/reject/reconnect/stale/unauthorized.
- **DoD:** durable studio gates resume correct descendants.

### T420 — Implement `build_timeline`

- **Goal:** create a canonical TimelineVersion from selected proposals/content.
- **Files/modules:** timeline builder/node tests.
- **Dependencies:** T232, T401–T410.
- **Notes:** no rendering/rematching and user origins preserved.
- **Tests:** schema/cross-refs/fingerprint/degraded inputs/override merge.
- **DoD:** renderer requires no match file.

### T430 — Build web shell and SDK client

- **Goal:** auth-aware Project/Job navigation, errors and reconnect.
- **Files/modules:** web/SDK/tests.
- **Dependencies:** T210, T230, T340.
- **Notes:** Product API only.
- **Tests:** auth/pagination/reconnect/secret scan/accessibility smoke.
- **DoD:** dashboard survives stream reconnect.

### T431 — Build Script review editor

- **Goal:** create/approve/reject versions and resume review.
- **Files/modules:** web/e2e.
- **Dependencies:** T231, T410, T430.
- **Notes:** ETag conflict UX; local draft non-authoritative.
- **Tests:** edit/reload/conflict/role denial/audit.
- **DoD:** pause→edit→resume is deterministic.

### T432 — Build Timeline/Scene editor slice

- **Goal:** inspect Scenes and replace/move Clips in a new TimelineVersion.
- **Files/modules:** web/e2e.
- **Dependencies:** T233–T234, T410, T420, T430.
- **Notes:** preserve origin/proposal refs.
- **Tests:** range/version/undo/conflict/reload/rerun override.
- **DoD:** one Clip edit rerenders without unrelated AI work.

### T433 — Build Tauri remote client

- **Goal:** shared API/SDK auth, upload, events, review and download.
- **Files/modules:** desktop/SDK/e2e.
- **Dependencies:** T223, T340, T430.
- **Notes:** no bundled server compute/data services/secrets.
- **Tests:** endpoint config, session revocation, restart reconnect, permissions.
- **DoD:** desktop completes native remote flow on Local/LAN.

### T550 — Certify LOCAL FUNCTIONAL ACCEPTANCE

- **Goal:** prove the complete owner-machine functional stack and an explicit LAN client before hardening.
- **Files/modules:** local/lan profiles, startup orchestration, integration/e2e evidence.
- **Dependencies:** T323, T340, T430, T433.
- **Notes:** start PostgreSQL, Redis, MinIO, Go API/media worker, Python worker, web and Tauri dev;
  prove LocalAuth/default Workspace, Project/upload/Job, FFmpeg Artifact, Python task, SSE, restart
  recovery and configured LAN access. VPS/public DNS/public TLS are not required.
- **Tests:** clean start, login/authz, upload, Redis Streams dispatch, both workers, result access,
  dependency/worker restart, second-device LAN connection and upstream-absence scan.
- **DoD:** report says `LOCAL FUNCTIONAL ACCEPTANCE: PASS` with no Movie Narrator dependency.

### T434 — Package and secure desktop baseline

- **Goal:** reproducible staging packages and safe update/rollback boundary.
- **Files/modules:** packaging/CI/runbook.
- **Dependencies:** T433.
- **Notes:** signing keys stay external; origins allowlisted.
- **Tests:** build/install/tamper/update/permission/rollback.
- **DoD:** signed staging client can roll back safely.

## Gate G — Core capabilities, intelligence and rendering

### T500 — Implement provider ports/resolver

- **Goal:** LLM/VLM/TTS/ASR/Embedding ports, errors and fake adapters.
- **Files/modules:** `nh_media.providers`/contracts/tests.
- **Dependencies:** T235, T401.
- **Notes:** explicit allowlisted registration and scoped credentials.
- **Tests:** conformance/error/cancel/timeout/fallback/redaction.
- **DoD:** five provider kinds pass one framework.

### T510 — Implement research/script nodes

- **Goal:** native ResearchAnalysis and ScriptVersion proposal.
- **Files/modules:** `nh_media.generation`.
- **Dependencies:** T402, T500.
- **Notes:** structured schema/prompt provenance; independent authorship.
- **Tests:** quality rubric, error/fallback/review and optional reference comparison.
- **DoD:** exact inputs create traceable ScriptVersion.

### T511 — Implement TTS/Narration

- **Goal:** synthesize traceable Narration/Artifact.
- **Files/modules:** `nh_media.speech.tts`.
- **Dependencies:** T231, T500, T510.
- **Notes:** full cache fingerprint and provider policy.
- **Tests:** voice/cache/audio probe/cancel/provider conformance.
- **DoD:** exact ScriptVersion produces reusable Narration.

### T512 — Implement ASR/alignment

- **Goal:** typed transcript/timing Artifacts through approved adapters.
- **Files/modules:** `nh_media.speech`.
- **Dependencies:** T500–T511.
- **Notes:** backend precision/fallback explicit.
- **Tests:** timing corpus, optional dependencies, degradation and model fingerprint.
- **DoD:** alignment meets NH-Media tolerances/provenance.

### T520 — Implement Scene detection/features

- **Goal:** source-revision Scenes and thumbnails/features.
- **Files/modules:** `nh_media.video.scenes` plus media worker support.
- **Dependencies:** T224, T401.
- **Notes:** zero-scene policy explicit.
- **Tests:** ranges/thresholds/timeout/malicious media/source revision.
- **DoD:** scene corpus passes.

### T521 — Implement VLM scene analysis

- **Goal:** typed per-scene Analysis with exact provenance.
- **Files/modules:** `nh_media.vision`.
- **Dependencies:** T500, T520.
- **Notes:** keyframe extraction separate; partial items explicit.
- **Tests:** malformed/fallback/privacy/egress/cancel/item provenance.
- **DoD:** results trace source/model and degraded items.

### T522 — Implement Character/Appearance analysis

- **Goal:** detect/track/cluster without overwriting confirmed identities.
- **Files/modules:** `nh_media.vision.characters`.
- **Dependencies:** T207, T520–T521.
- **Notes:** model/privacy policy required.
- **Tests:** corpus/cluster/merge/confirmation rerun/provenance.
- **DoD:** Character results support review safely.

### T523 — Implement Embedding/index Artifact

- **Goal:** versioned text/image embeddings for matching.
- **Files/modules:** `nh_media.matching.embeddings`.
- **Dependencies:** T500, T521.
- **Notes:** baseline persistence is Artifact + item-index manifest; no cross-model comparisons.
- **Tests:** batch/partial/reload/model-space/benchmark.
- **DoD:** deterministic item index through provider-neutral contract.

### T524 — Implement match proposals and coverage

- **Goal:** scored MatchProposals and optional coverage-driven Script proposal.
- **Files/modules:** `nh_media.matching`/evaluation.
- **Dependencies:** T510, T512, T520–T523.
- **Notes:** approved content immutable; renderer never reads proposal directly.
- **Tests:** determinism/diversity/quality/degraded/override/coverage.
- **DoD:** proposal and score components are traceable.

### T525 — Implement candidate evaluation/selection

- **Goal:** multiple GenerationCandidates, EvaluationResults and audited selection.
- **Files/modules:** `nh_media.evaluation`/domain/API.
- **Dependencies:** T234, T500, T524.
- **Notes:** direct single-candidate path remains valid for simple workflows.
- **Tests:** multiple candidates, deterministic policy, user selection and provenance.
- **DoD:** selection never mutates candidates and graph remains resumable.

### T526 — Implement ReferenceStyleAnalysis

- **Goal:** extract abstract production characteristics from authorized references.
- **Files/modules:** `nh_media.video.reference_style`.
- **Dependencies:** T234, T512, T520–T521.
- **Notes:** no copied footage as semantic style output.
- **Tests:** pacing/shot/narration/subtitle/framing/music/rhythm metrics and provenance.
- **DoD:** analysis is reusable input to generation policy with explicit consent/scope.

### T530 — Implement Timeline compiler/MediaProcess plan

- **Goal:** deterministic render plan from exact TimelineVersion/Profile.
- **Files/modules:** Go media worker compiler.
- **Dependencies:** T313, T420, T524.
- **Notes:** no rematching/provider call; allowlisted options.
- **Tests:** ranges/transforms/transitions/audio/subtitles/injection/determinism.
- **DoD:** same inputs produce same safe plan.

### T531 — Implement render and deliverable QA

- **Goal:** execute compiled plan and commit validated Render Artifacts.
- **Files/modules:** Go media worker render/QA.
- **Dependencies:** T222, T332, T350, T530.
- **Notes:** incomplete outputs never canonical.
- **Tests:** real media, missing streams, duration/black/silence, retry/cancel.
- **DoD:** Render completes solely from NH-Media Timeline/Profile.

### T532 — Add 16:9, 9:16 and 1:1 reuse

- **Goal:** multiple profiles from shared upstream-of-render Artifacts.
- **Files/modules:** profiles/reframe/dedupe tests.
- **Dependencies:** T531.
- **Notes:** profile-specific work does not rerun script/analysis/matching.
- **Tests:** dimensions/codecs/safe area/provider-call audit/cache invalidation.
- **DoD:** three outputs reuse unchanged inputs correctly.

## Gate H — Security and LOCAL/LAN HARDENED ACCEPTANCE

### T600 — Harden sandbox/secret/egress policy

- **Goal:** production-grade least capability for workers/executors.
- **Files/modules:** containers/security policy/tests.
- **Dependencies:** T313, T500, T531.
- **Notes:** render workers receive no unnecessary provider secrets.
- **Tests:** malicious media/extension/subprocess/SSRF/resource/secret attacks.
- **DoD:** security matrix passes with residual risks documented.

### T601 — Add observability and graceful recovery

- **Goal:** dashboards/alerts/traces and drain/reconcile operations.
- **Files/modules:** telemetry/runbooks/tests.
- **Dependencies:** T340, T600.
- **Notes:** bounded labels/redacted content.
- **Tests:** worker/provider/storage/Redis outages and alerts.
- **DoD:** failures recover without manual DB fabrication.

### T602 — Add backup/restore and Artifact inventory

- **Goal:** clean restore of PostgreSQL plus object refs/checksums.
- **Files/modules:** tools/runbooks/tests.
- **Dependencies:** T222, T601.
- **Notes:** restore is non-destructive and reports missing/orphans.
- **Tests:** clean restore/corrupt object/current pointers/checkpoints.
- **DoD:** selected Job→Timeline→Render→Artifact traces exactly.

### T603 — Certify LOCAL/LAN HARDENED ACCEPTANCE

- **Goal:** prove production-like operation without a VPS.
- **Files/modules:** deployment profile/e2e/report.
- **Dependencies:** T434, T531–T532, T550, T600–T602.
- **Notes:** harden the already functional system; server and clients run on separate LAN machines where practical.
- **Tests:** auth/upload/review/render/reconnect/restart/restore/multi-client.
- **DoD:** owner accepts Local/LAN release evidence and upstream-absence scan.

### T604 — Run an upstream research refresh drill

- **Goal:** prove capability-aware research update without importing code.
- **Files/modules:** research/audit/capability matrix only.
- **Dependencies:** T001, T003, T108.
- **Notes:** compare behavior/security/dependencies and update NH-Media dispositions/tests.
- **Tests:** provenance, matrix coverage, no source/dependency/artifact changes.
- **DoD:** research can evolve without product coupling.

### T605 — Prepare INTERNET / VPS PRODUCTION

- **Goal:** public staging/canary/TLS/operations after Local/LAN pass.
- **Files/modules:** deployment/runbook/release evidence.
- **Dependencies:** T603.
- **Notes:** exact VPS, DNS, OIDC, KMS, storage and monitoring vendors are deployment configuration
  choices selected here; they are not functional implementation blockers.
- **Tests:** staging/canary/load/security/migration/restore/client update/rollback.
- **DoD:** public release passes `PRODUCTION-PLAN.md` and uses only NH-Media rollback artifacts.
