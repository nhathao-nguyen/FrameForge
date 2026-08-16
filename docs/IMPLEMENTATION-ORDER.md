# Implementation Order

T000 was ratified and amended by the owner on 2026-08-16. The amendment supersedes the earlier
Python Product/API decision: Go owns the V2 control plane; Python is isolated to ML/AI and frozen V1
compatibility workloads. T001 was already completed before this amendment and is not restarted here;
T002–T005 remain documentation/baseline tasks, and application code remains blocked until Gate A/
Phase 0 evidence passes. After that, assign one task/PR at a time.

Every task has the required handoff fields. A dependency marked `OQ-x decided` is a hard block; recommendation is not a decision.

## Gate A — Ratification and V1 baseline

### T000 — Ratify specification decisions

- **Goal:** owner reviews audit and records decisions needed for Phase 0/first scaffold.
- **Files/modules affected:** docs only: `OPEN-QUESTIONS.md`, affected specs, optional ADRs.
- **Dependencies:** specification audit complete.
- **Implementation notes:** preserve master invariants; append a dated superseding owner decision
  without erasing the earlier OQ-13 history; record Go control plane/module path, bounded worker
  topology, language-neutral contracts, `nh_media`, all verified V1 surfaces and Tauri 2 in dependent
  contracts, not only OQ text. Do not create `go.mod` or application code in this task.
- **Tests required:** link/terminology/consistency checks; confirm no application source diff.
- **Definition of Done:** Phase -1 amendment signed; OQ-12/OQ-13/OQ-14/OQ-15 decisions recorded;
  old OQ-13 history preserved and marked superseded; affected specs, memory and task dependencies
  updated; no application source diff or T001 restart.

### T001 — Record immutable upstream baseline manifest

- **Goal:** freeze exact origin/tag/peeled commit/license and repository state as implementation evidence.
- **Files/modules affected:** baseline report/manifest under `docs/baselines/`; git remote/tag/branch metadata only.
- **Dependencies:** T000; no application namespace needed.
- **Implementation notes:** record `bc2d276...` audit value then re-verify remote; create baseline ref per owner policy; do not change V1 files.
- **Tests required:** `git status`, remote/peeled-tag comparison, signed/hash manifest, license headers.
- **Definition of Done:** another agent can resolve identical source from report and clean worktree; divergence is explained.

### T002 — Build reproducible Go/Python/Arch environment matrix

- **Goal:** pin a supported stable Go toolchain for Product/control plane and lock isolated Python ML/V1 environments without system Python.
- **Files/modules affected:** environment lock/config and `docs/baselines/environment-*`; container metadata only as approved.
- **Dependencies:** T001; OQ-13 decided.
- **Implementation notes:** Go toolchain is pinned in environment/CI; Python 3.12 is isolated to ML/V1
  locks/images; record Node/Tauri tooling and FFmpeg/ffprobe build; any later Python 3.13 ML move
  requires parity evidence. No Go/Python internal types cross the contract boundary.
- **Tests required:** clean Go install/test, Go version/toolchain hash, isolated Python install/version/
  lock hash, dependency groups, FFmpeg codec/filter probes and language-neutral worker fixtures.
- **Definition of Done:** documented commands reproduce the matrix on clean Arch host/container without
  global pip or floating Go/CI/production toolchains.

### T003 — Freeze V1 compatibility profile

- **Goal:** enumerate exact CLI, REST, config, statuses and outputs guaranteed during strangler.
- **Files/modules affected:** `docs/baselines/v1-compatibility-profile.md`, test selection manifest.
- **Dependencies:** T001–T002; OQ-14 decided.
- **Implementation notes:** include tasks/batches/schedules/DLQ/distributed only per decision; every omission uses BREAKING CHANGE template.
- **Tests required:** invoke each selected CLI/route/config alias; snapshot schemas/status/result/artifact names.
- **Definition of Done:** compatibility matrix has request, response, behavior, test and owner for every preserved surface.

### T004 — Produce V1 golden media outputs

- **Goal:** establish deterministic/metric-aware samples for script/TTS/scene/match/subtitle/render/QA.
- **Files/modules affected:** regression fixtures/manifests/reports; large media in approved fixture storage, not git if unsuitable.
- **Dependencies:** T002–T003.
- **Implementation notes:** store input/output hashes, environment, provider fake/real classification and tolerances; do not assert byte equality for nondeterministic provider output.
- **Tests required:** full sample automatic run, partial/resume, soft/strict, artifact manifest and media probe.
- **Definition of Done:** golden evidence is reviewable and can detect behavioral drift without modifying V1.

### T005 — Freeze runnable V1 rollback image

- **Goal:** preserve a non-root executable V1 deployment for migration rollback.
- **Files/modules affected:** baseline container recipe/image digest/runbook.
- **Dependencies:** T002–T004.
- **Implementation notes:** pin base/dependencies/FFmpeg; no latest tags/default public credentials; preserve AGPL notices.
- **Tests required:** image build, UID 10001/non-root, CLI/daemon health, sample artifacts, graceful stop.
- **Definition of Done:** digest-pinned image reproduces baseline and rollback command is documented.

## Gate B — Product foundation

### T100 — Create approved Go/control-plane and compute package skeleton

- **Goal:** establish the approved client/server layout: `apps/web`, `apps/desktop`,
  `cmd/product-api`, `cmd/media-worker`, `internal/domain`, `internal/application`, `internal/ports`,
  `internal/adapters`, `internal/transport/http`, `services/ml-worker`, `services/legacy-compat`,
  `packages/contracts`, `packages/sdk`, `infra` and `tests`.
- **Files/modules affected:** new package/layout metadata only for the listed boundaries; legacy
  `references/movie-narrator` untouched.
- **Dependencies:** T005; OQ-12 decided.
- **Implementation notes:** Go module path starts at `github.com/nhathao-nguyen/FrameForge`; prefer
  `internal/domain`, `internal/application`, `internal/ports`, `internal/adapters`, `internal/transport/http`.
  Python packages use `nh_media`; frozen code keeps `movie_narrator`. No media/domain implementation.
- **Tests required:** Go module/test, client/package checks, Python boundary/import checks, forbidden
  dependency checks, language-neutral contract fixtures and V1 baseline still clean.
- **Definition of Done:** the exact approved layout exists with boundary tests proving Go control
  plane, Go media, Python ML/V1 and clients are replaceable and cannot depend on each other's internals.

### T101 — Add shared identity/time/error contract primitives

- **Goal:** define opaque IDs, UTC/time range, revision/ETag and safe error envelope types.
- **Files/modules affected:** `packages/contracts` and unit tests only.
- **Dependencies:** T100.
- **Implementation notes:** no framework/provider/storage types; canonical terminology from Glossary.
- **Tests required:** serialization, invalid IDs/time ranges, error redaction, schema snapshots.
- **Definition of Done:** API/engine packages can depend on primitives without reverse dependency.

### T102 — Add configuration and redaction boundary

- **Goal:** separate Product, worker, legacy and provider configuration namespaces.
- **Files/modules affected:** configuration package, `.env.example`, redaction tests/docs.
- **Dependencies:** T100–T101; OQ-05 for credential model.
- **Implementation notes:** secret references only in serializable config; reject project executable override in production.
- **Tests required:** precedence, required values, unknown keys, secret serialization/log redaction.
- **Definition of Done:** no secret appears in repr/log/checkpoint/event fixture; V1 translator remains separate.

### T103 — Provision PostgreSQL dev service

- **Goal:** create pinned/private PostgreSQL dev infrastructure and migration connectivity.
- **Files/modules affected:** `infra/postgres`, compose/podman config, ops docs.
- **Dependencies:** T100.
- **Implementation notes:** non-default credentials, healthcheck, persistent named volume, least-privilege app/migration roles.
- **Tests required:** clean start/stop, TLS/local policy, role privilege negative test, volume recovery.
- **Definition of Done:** API/test process connects with app role; app role cannot migrate schema.

### T104 — Provision Redis dev service

- **Goal:** create pinned/private Redis for future QueuePort/event fan-out.
- **Files/modules affected:** `infra/redis`, config/docs.
- **Dependencies:** T100.
- **Implementation notes:** ACL/password where supported; no queue implementation yet.
- **Tests required:** private bind/auth, health, restart, max-memory/persistence policy evidence.
- **Definition of Done:** reproducible service with no public/default-insecure exposure.

### T105 — Provision MinIO dev service

- **Goal:** create pinned/private S3-compatible object storage baseline.
- **Files/modules affected:** `infra/minio`, bucket bootstrap/config/docs.
- **Dependencies:** T100.
- **Implementation notes:** random/non-default credentials, private bucket, no `latest`, lifecycle disabled until OQ-10.
- **Tests required:** health/auth/private ACL, restart, multipart capability.
- **Definition of Done:** scoped dev bucket works and unauthenticated object access fails.

### T106 — Implement Go Product API shell and error middleware

- **Goal:** expose `/api/v1` shell with request/correlation IDs and canonical errors.
- **Files/modules affected:** Go `cmd/product-api`, `internal/transport/http`, application error middleware/tests.
- **Dependencies:** T101–T105; OQ-01 auth seam decision.
- **Implementation notes:** no Project/media route; no V1 internal import; bounded request body/CORS placeholder policy.
- **Tests required:** startup, request IDs, safe 4xx/5xx, no traceback, API contract/OpenAPI smoke and
  proof that no Python/FFmpeg/ML runtime is imported or executed.
- **Definition of Done:** API shell passes contract tests with mocked dependencies.

### T107 — Implement health/readiness endpoints

- **Goal:** distinguish liveness, readiness and protected deep dependency diagnostics.
- **Files/modules affected:** Go Product API health application service/adapters/tests.
- **Dependencies:** T103–T106.
- **Implementation notes:** public response minimal; deep checks protected; preserve compatibility listener semantics separately.
- **Tests required:** each dependency down, draining state, timeout, information disclosure.
- **Definition of Done:** health remains alive while readiness accurately rejects work.

### T108 — Establish V2 CI gates

- **Goal:** run Go format/vet/test/security, isolated Python ML/V1 checks, contract tests and V1 regression without weakening upstream checks.
- **Files/modules affected:** CI workflows/config/scripts.
- **Dependencies:** T100–T107.
- **Implementation notes:** pin Go and Python toolchains; scoped advisory exceptions need owner/expiry;
  do not copy Bandit B404/B603 skip blindly; contract tests must be language-independent.
- **Tests required:** intentionally failing lint/type/security/contract jobs, V1 baseline job, artifact reports.
- **Definition of Done:** branch gate names/stages documented and required checks pass on clean baseline.

## Gate C — Persistence and product resources

### T200 — Add migration framework and schema invariants

- **Goal:** bootstrap extensions, migration roles/conventions and schema test harness.
- **Files/modules affected:** `infra/postgres/migrations`, migration runner/tests.
- **Dependencies:** T103, T108.
- **Implementation notes:** immutable migrations; this PR creates framework/extension only, not all tables.
- **Tests required:** empty upgrade, failed migration rollback, repeat invocation, app-role denial.
- **Definition of Done:** subsequent table PRs can be reviewed independently with clean DB evidence.

### T201 — Add identity/Workspace ownership tables

- **Goal:** implement users/workspaces/members/API keys exactly per `03`.
- **Files/modules affected:** one migration, repository interfaces/DB tests.
- **Dependencies:** T200; OQ-01/OQ-06 decided.
- **Implementation notes:** API key hash only; no auth UI; repository requires Workspace scope.
- **Tests required:** FK/unique/partial indexes, role checks, cross-Workspace negative queries.
- **Definition of Done:** schema inventory and repository tests match fields/indexes/ownership contract.

### T202 — Add Workflow/Pipeline/Node definition tables

- **Goal:** persist built-in versioned Workflow/Pipeline/Node/dependency definitions.
- **Files/modules affected:** one migration, definition repositories/validators tests.
- **Dependencies:** T200; OQ-11 decision confirms authoring scope.
- **Implementation notes:** no executor; active definitions immutable; graph validation can be stubbed to schema checks until T400.
- **Tests required:** version/content hash uniqueness, same-Pipeline dependencies, active mutation rejection.
- **Definition of Done:** movie recap definition can be stored/read as data without running it.

### T203 — Add ProviderConfiguration and RenderProfile tables

- **Goal:** persist redacted provider metadata and immutable active render profiles.
- **Files/modules affected:** one migration/repositories/tests.
- **Dependencies:** T200–T201; OQ-05.
- **Implementation notes:** no plaintext secret/provider SDK; correct system-scope partial uniqueness for NULL Workspace.
- **Tests required:** revision/status/uniqueness, credential redaction, active profile immutability.
- **Definition of Done:** configuration/profile snapshots can be resolved by ID/version without secret material.

### T204 — Add Project/Asset/upload metadata tables

- **Goal:** implement Project, Asset and AssetUpload lifecycle metadata.
- **Files/modules affected:** one migration/repositories/state tests.
- **Dependencies:** T201–T203.
- **Implementation notes:** Artifact pointer FK deferred to T206; Asset has no object key identity.
- **Tests required:** ownership/FK/status/revision/partial active-upload uniqueness.
- **Definition of Done:** Product can persist upload intent but no binary or ready Asset yet.

### T205 — Add Job/PipelineRun/JobStep tables

- **Goal:** implement execution records, ReviewRequest/Resolution and canonical status checks without queue logic.
- **Files/modules affected:** one migration/repositories/tests.
- **Dependencies:** T202, T204.
- **Implementation notes:** include attempts/current run/supersedes, generic review rows and partial unique one active run.
- **Tests required:** checks/FKs/indexes, immutable snapshots, concurrent active-run rejection.
- **Definition of Done:** schema represents Job→Run→Step/attempt exactly and rejects aliases.

### T206 — Add Artifact/checkpoint/DLQ tables and Asset links

- **Goal:** implement immutable Artifact manifest, variants, checkpoints and dead letters.
- **Files/modules affected:** one migration/repositories/tests.
- **Dependencies:** T204–T205.
- **Implementation notes:** add deferred Asset original/checkpoint/current FKs; bytes remain outside DB.
- **Tests required:** storage-key/version uniqueness, producer/project consistency, checkpoint refs, canonical variant uniqueness.
- **Definition of Done:** all blob/execution refs persist without raw paths/binary.

### T207 — Add Script/Narration/Scene/Character tables

- **Goal:** implement content/intelligence aggregates and immutable ScriptVersion.
- **Files/modules affected:** one migration/repositories/tests.
- **Dependencies:** T204–T206.
- **Implementation notes:** include reviews, exact source Artifact revision and user-confirmation lineage.
- **Tests required:** version/hash/status, Scene ranges, Character merge/appearance ownership, Narration refs.
- **Definition of Done:** domain fields/relations/indexes from `02`/`03` are covered by schema assertions.

### T208 — Add Timeline/Render tables

- **Goal:** implement TimelineVersion JSONB, Render and output relations.
- **Files/modules affected:** one migration/repositories/tests.
- **Dependencies:** T203, T206–T207; OQ-02 decision.
- **Implementation notes:** no Track/Clip canonical tables unless approved projection task; deferred Project current pointers.
- **Tests required:** immutable version/hash, current-pointer same Project, profile/timeline Render dedupe, output role uniqueness.
- **Definition of Done:** exact TimelineVersion/Profile can identify a Render request/output relation.

### T209 — Add events/outbox/idempotency tables

- **Goal:** persist ordered Job events, transactional outbox and request idempotency.
- **Files/modules affected:** one migration/repositories/tests.
- **Dependencies:** T201, T205.
- **Implementation notes:** append-only Job events; allocate per-Job sequence safely.
- **Tests required:** concurrent sequence allocation, duplicate key/hash, outbox retry identity, append-only permissions.
- **Definition of Done:** state transaction can atomically include event/outbox with deterministic replay.

### T210 — Implement auth context and scoped repositories

- **Goal:** enforce Product identity/Workspace/role across application repository calls.
- **Files/modules affected:** API auth adapter, application authorization, repository query tests.
- **Dependencies:** T106, T201, OQ-01/OQ-06.
- **Implementation notes:** distinguish authentication from authorization; do not trust Workspace ID in payload.
- **Tests required:** every role, suspended identity, cross-Workspace guessed ID, API key scopes, audit context.
- **Definition of Done:** scoped resource access fails closed before Product CRUD is added.

## Gate D — Storage, upload and versioned content APIs

### T220 — Define StoragePort and LocalStorage adapter

- **Goal:** implement storage contract plus secure local adapter.
- **Files/modules affected:** storage port/local adapter/conformance tests.
- **Dependencies:** T101, T206.
- **Implementation notes:** local handle never serializable; explicit root; atomic stage/promote; no home/repo-root default.
- **Tests required:** contract operations, traversal/encoded/backslash/symlink, preconditions, cleanup.
- **Definition of Done:** LocalStorage passes reusable conformance suite and returns locators/handles at correct boundaries.

### T221 — Implement S3/MinIO adapter

- **Goal:** satisfy same StoragePort with multipart/presign/version support.
- **Files/modules affected:** S3 adapter and integration tests.
- **Dependencies:** T105, T220.
- **Implementation notes:** MinIO uses same adapter; exact method/key/TTL; no credentials in result/log.
- **Tests required:** conformance against MinIO, multipart abort/expiry, range read, presign ACL/TTL, transient errors.
- **Definition of Done:** Local and S3 adapters are interchangeable at application port.

### T222 — Implement Artifact stage/promote/commit service

- **Goal:** atomically publish validated blobs and metadata/relations/events.
- **Files/modules affected:** Artifact application service/repository/storage integration.
- **Dependencies:** T209, T220–T221.
- **Implementation notes:** orphan reconciliation hooks; checksum before commit; idempotent canonical role.
- **Tests required:** storage succeeds/DB fails and inverse, duplicate commit, checksum mismatch, cross-Project refs.
- **Definition of Done:** incomplete bytes never appear as committed Artifact and retry does not duplicate canonical output.

### T223 — Implement presigned upload-session API

- **Goal:** Browser→object-storage multipart initiate/complete/abort lifecycle.
- **Files/modules affected:** Asset upload application service/API routes/tests.
- **Dependencies:** T210–T222.
- **Implementation notes:** no request-body media proxy; server-generated staging key; idempotency required.
- **Tests required:** single/multipart, same/different idempotency hash, expiry/abort, declared quota/type/status.
- **Definition of Done:** trace proves media bytes bypass API/Redis and complete yields Asset `validating`, not `ready`.

### T224 — Implement validation/probe Job boundary

- **Goal:** validate checksum/MIME/media limits/security and commit original Artifact/Asset ready state.
- **Files/modules affected:** probe command/worker node contract, ffprobe adapter, quarantine tests.
- **Dependencies:** T223, minimal execution harness may use synchronous test adapter until Gate E; OQ-10 policy.
- **Implementation notes:** actual probe runs sandboxed; fail closed; do not implement general pipeline here.
- **Tests required:** spoofed MIME, checksum, oversized/duration/stream/resource bomb fixtures, timeout, quarantine.
- **Definition of Done:** only validated committed original makes Asset `ready`; unsafe input cannot reach production pipeline.

### T230 — Implement Project/Asset REST resources

- **Goal:** expose scoped Project/Asset CRUD/list/metadata/download actions.
- **Files/modules affected:** API schemas/routes/application services/tests.
- **Dependencies:** T210, T223–T224.
- **Implementation notes:** cursor/ETag/soft delete; no local path/object key; download presign after ACL.
- **Tests required:** contract/OpenAPI, pagination, ETag, roles/cross-scope, active references/delete.
- **Definition of Done:** all section 2–3 API cases pass and responses are redaction-safe.

### T231 — Implement Script/ScriptVersion/Narration API

- **Goal:** expose immutable versions/reviews and Narration request metadata.
- **Files/modules affected:** API/application/repositories/tests; no TTS provider yet.
- **Dependencies:** T207, T210, T209.
- **Implementation notes:** edit creates version; approval metadata transition; Narration Job may remain disabled until execution gate.
- **Tests required:** version race/ETag, approve/reject/audit, current pointer, cross-Project refs.
- **Definition of Done:** exact ScriptVersion can be snapshotted and approved content cannot mutate.

### T232 — Implement Timeline schema and validator library

- **Goal:** validate canonical JSON Schema 1.0 plus cross-field/source ownership rules.
- **Files/modules affected:** contracts schema, validator, positive/negative corpus.
- **Dependencies:** T208, T210.
- **Implementation notes:** no renderer/editor; Track/Clip embedded; bounded document counts/size.
- **Tests required:** all rules in `06`, malformed/oversized/source range/ownership, canonical hash.
- **Definition of Done:** canonical example passes and every negative category fails with safe pointer/code.

### T233 — Implement Timeline version REST API

- **Goal:** create/list/read/validate/approve/lock immutable TimelineVersions.
- **Files/modules affected:** API/application/repositories/tests.
- **Dependencies:** T232; OQ-08 decided.
- **Implementation notes:** implement approved edit transport; conflicts never merge array indexes silently.
- **Tests required:** ETag race, version lineage, user-origin preservation, approval/lock, import/replace path.
- **Definition of Done:** API section 6 passes and renderer-ready exact version is addressable.

### T234 — Implement Scene/Character APIs

- **Goal:** expose read/annotation/confirmation/merge without allowing client-owned model artifacts.
- **Files/modules affected:** API/application/repository tests.
- **Dependencies:** T207, T210.
- **Implementation notes:** source range protected; Character merge preserves lineage.
- **Tests required:** roles, revision conflict, cross-Project IDs, confirmed identity rerun guard.
- **Definition of Done:** Scene/Character product state can support studio review safely.

### T235 — Implement ProviderConfiguration API

- **Goal:** expose redacted admin lifecycle/validation boundary.
- **Files/modules affected:** API/application/secret-backend port/tests.
- **Dependencies:** T203, T210; OQ-05.
- **Implementation notes:** validation adapter may be fake initially; credential create/rotate never echoes plaintext.
- **Tests required:** roles, redaction, revision, disable/delete references, secret-backend failure.
- **Definition of Done:** Product can manage configuration metadata without provider-specific logic in routes.

## Gate E — Job control, worker and legacy vertical slice

### T300 — Implement Job/Run/Step transition service

- **Goal:** enforce canonical state machines and aggregate invariants transactionally.
- **Files/modules affected:** domain/application transition module + repository tests.
- **Dependencies:** T205, T209.
- **Implementation notes:** one transition API; state + event + outbox atomic; no Redis yet.
- **Tests required:** every valid/invalid edge, terminal immutability, concurrent CAS, Run↔Job matrix.
- **Definition of Done:** no caller can persist alias/illegal transition or event mismatch.

### T301 — Implement durable event replay/outbox publisher port

- **Goal:** publish committed events idempotently and replay per-Job sequence.
- **Files/modules affected:** event repository/outbox publisher interface/tests.
- **Dependencies:** T209, T300.
- **Implementation notes:** fake transport first; same event ID on retry; payload schema registry.
- **Tests required:** crash before/after publish, duplicate, sequence gap/reset metadata, redaction.
- **Definition of Done:** DB replay reconstructs ordered feed even if transport loses live message.

### T310 — Implement Redis QueuePort adapter

- **Goal:** enqueue/consume/ack/reclaim canonical work messages.
- **Files/modules affected:** queue port/Redis adapter/integration tests.
- **Dependencies:** T104, T301; OQ-03.
- **Implementation notes:** DB lease/state remains truth; queue payload contains IDs only.
- **Tests required:** duplicate delivery, consumer death/reclaim, delayed retry, priority ordering policy, Redis restart.
- **Definition of Done:** no Job is lost between DB commit/outbox/queue and stale messages are harmless.

### T311 — Implement dependency scheduler

- **Goal:** move JobSteps `pending→ready→queued` from DAG dependencies/partial boundaries.
- **Files/modules affected:** orchestration scheduler tests.
- **Dependencies:** T202, T300, T310.
- **Implementation notes:** no node-specific workflow hacks; required failure produces `blocked`; optional skip reason explicit.
- **Tests required:** chain/branch/join, dependency fail/cancel, start_from/stop_after frontier, concurrent scheduler.
- **Definition of Done:** ready-set behavior is deterministic for arbitrary valid Pipeline definition.

### T312 — Implement worker controller lease/heartbeat

- **Goal:** claim one JobStep attempt, heartbeat, report result and reconcile expired leases.
- **Files/modules affected:** Go `cmd/media-worker` controller, `internal/ports` ExecutionStatePort adapter, tests.
- **Dependencies:** T300–T311.
- **Implementation notes:** Go controller has restricted identity; Python ML/V1 workers receive only
  language-neutral manifests; executor has no DB/Redis; exact attempt token and bounded concurrency.
- **Tests required:** two workers, stale token, lease expiry, graceful drain, progress without heartbeat.
- **Definition of Done:** only one attempt commits and worker death transitions by retry policy.

### T313 — Implement sandboxed MediaProcessPort

- **Goal:** launch/timeout/cancel argv-only subprocesses in isolated workspace.
- **Files/modules affected:** worker executor/sandbox/process adapter/security tests.
- **Dependencies:** T102, T220, T312.
- **Implementation notes:** non-root, quotas, process-group kill, allowlisted executables, bounded output, scoped egress/secret manifest.
- **Tests required:** shell injection, executable override, timeout child tree, path mount escape, resource limit, redaction.
- **Definition of Done:** executor cannot reach product credentials/host paths and cancellation cleans process tree.

### T320 — Define VideoEngine and engine-neutral contracts

- **Goal:** create execution lifecycle port over PipelineRun/Artifact refs.
- **Files/modules affected:** engine contract package/tests only.
- **Dependencies:** T101, T300.
- **Implementation notes:** no V1 imports/framework/database model in port; methods match `01`.
- **Tests required:** schema/serialization, cancellation/resume commands, forbidden local path.
- **Definition of Done:** fake engine passes conformance suite and Product application depends only on port.

### T321 — Implement legacy DTO/workspace mapper

- **Goal:** map Asset/Artifact snapshots to isolated V1 Context/TaskRequest and map outputs back.
- **Files/modules affected:** legacy adapter mapping module/tests.
- **Dependencies:** T003–T004, T222, T313, T320.
- **Implementation notes:** import `movie_narrator.contract` first; temp paths are adapter-local; scrub result.
- **Tests required:** every preserved input/alias/model/status/output role, traversal/secret/path redaction.
- **Definition of Done:** mapper round-trip covers frozen profile without Product domain importing V1 models.

### T322 — Execute frozen 16-step V1 through adapter

- **Goal:** run one Product PipelineRun via LegacyMovieNarratorAdapter.
- **Files/modules affected:** adapter executor/worker integration tests.
- **Dependencies:** T312–T321.
- **Implementation notes:** call public contract/runner; preserve fixed order/soft-hard/strict; no V1 refactor.
- **Tests required:** golden automatic run, soft/strict, start step, provider fake, rollback route.
- **Definition of Done:** V1 completes under VideoEngine with exact status/progress callback mapping.

### T323 — Commit legacy outputs as Artifacts

- **Goal:** convert V1 files/metadata into immutable Artifact roles and safe result links.
- **Files/modules affected:** legacy output importer + Artifact service integration.
- **Dependencies:** T222, T322.
- **Implementation notes:** preserve aliases, checksum/probe; unknown outputs retained with safe role; no raw directory listing.
- **Tests required:** full output manifest, duplicate retry, missing/partial soft output, filename/path guard.
- **Definition of Done:** Product Job result is queryable entirely by Artifact IDs/roles.

### T330 — Implement checkpoints and crash resume

- **Goal:** persist DB/Artifact checkpoint and restore first incomplete compatible JobStep.
- **Files/modules affected:** orchestration checkpoint service + legacy mapper/tests.
- **Dependencies:** T206, T311–T323.
- **Implementation notes:** schema/input/pipeline/provider fingerprints; import V1 checkpoint evidence; no secret/path.
- **Tests required:** crash after every baseline node, corrupt/missing checkpoint, stale input, duplicate output.
- **Definition of Done:** Scenario E resumes without rerunning committed TTS and without fake success.

### T331 — Implement pause/partial/review commands

- **Goal:** support `start_from`, `stop_after`, pause, resume and durable human gate generically.
- **Files/modules affected:** Job command services/API tests.
- **Dependencies:** T300, T311, T330.
- **Implementation notes:** distinguish `paused` and `waiting_for_review`; approval policies data-driven.
- **Tests required:** boundaries on branch/join, edit ScriptVersion then resume, stale review revision, reject branches.
- **Definition of Done:** Scenario B works and no workflow-specific conditional is required in scheduler.

### T332 — Implement cancel/retry/DLQ

- **Goal:** bounded cooperative cancel, canonical retry/backoff and fresh-ID DLQ replay.
- **Files/modules affected:** orchestration/worker command services/tests.
- **Dependencies:** T300, T312–T313, T330.
- **Implementation notes:** cancel active state `cancelling`; retry terminal Job creates new Job; no retry security errors.
- **Tests required:** cancel each state, process kill, retry taxonomy/budget/jitter, DLQ on/off/replay lineage.
- **Definition of Done:** terminal history immutable and outputs/checkpoints reconciled correctly.

### T340 — Implement SSE progress stream

- **Goal:** durable replay + live fan-out using canonical envelope.
- **Files/modules affected:** API event endpoint/publisher/frontend test client.
- **Dependencies:** T301, T310, T331–T332; OQ-04.
- **Implementation notes:** Last-Event-ID, snapshot/reset, keepalive, auth stream lifetime.
- **Tests required:** ordering/dedupe/reconnect/gap/terminal close, cross-Workspace stream denial, slow consumer.
- **Definition of Done:** client reconstructs same state as REST after disconnect/reconnect.

### T341 — Implement optional WebSocket transport

- **Goal:** expose same read-only event semantics if approved/needed.
- **Files/modules affected:** WebSocket route/transport tests only.
- **Dependencies:** T340 and OQ-04 decision requiring it.
- **Implementation notes:** no state commands; reuse event/replay service.
- **Tests required:** subscribe/reconnect/gap/auth/backpressure and attempted mutation rejection.
- **Definition of Done:** envelope/order behavior is identical to SSE.

### T350 — Implement Job/Run/Step/Render REST commands

- **Goal:** expose canonical API sections 7–8 over completed control plane.
- **Files/modules affected:** API schemas/routes/application tests.
- **Dependencies:** T230–T235, T300–T340.
- **Implementation notes:** idempotency and exact state errors; Render can route legacy renderer initially.
- **Tests required:** every command/state/HTTP code, pagination, optimistic snapshots, artifact links.
- **Definition of Done:** Scenario A traces API→DB→queue→worker→Timeline/result Artifact with consistent IDs/states.

### T360 — Implement core V1 REST/CLI compatibility gateway

- **Goal:** preserve core tasks/status/cancel/result/artifacts and selected CLI profile over Product Job.
- **Files/modules affected:** compatibility gateway/CLI adapter/tests.
- **Dependencies:** T003, T321–T350; OQ-09/OQ-14.
- **Implementation notes:** separate auth listener/policy; project ownership mapping explicit; status projection documented.
- **Tests required:** frozen client fixtures, `format` alias, legacy filenames/status/error/auth.
- **Definition of Done:** selected core compatibility profile passes unchanged clients and browser never receives engine key.

### T361 — Implement optional batch/schedule/DLQ compatibility

- **Goal:** preserve additional V1 surfaces selected by OQ-14 in isolated PRs/subtasks.
- **Files/modules affected:** compatibility-only batch, schedule and DLQ adapters.
- **Dependencies:** T360; OQ-14.
- **Implementation notes:** split one PR per surface; scheduling is not Product Pipeline architecture.
- **Tests required:** exact frozen route/schema/cancel/progress/replay cases.
- **Definition of Done:** each selected surface passes profile or has approved BREAKING CHANGE record.

## Gate F — Native V2 pipeline and studio

### T400 — Implement DAG validator/activation gate

- **Goal:** validate unique keys, cycles, schemas, dependencies, capabilities and policies before Pipeline active.
- **Files/modules affected:** engine pipeline definition/validator tests.
- **Dependencies:** T202, T320; OQ-11.
- **Implementation notes:** definition is data; no execution in this task.
- **Tests required:** all invalid graph/policy/schema/capability cases and active hash immutability.
- **Definition of Done:** only complete node contracts from `05` can activate.

### T401 — Implement native node runtime conformance

- **Goal:** execute fake nodes with validate/execute/resume/result/checkpoint/idempotency semantics.
- **Files/modules affected:** V2 engine runtime and conformance suite.
- **Dependencies:** T311–T313, T330, T400.
- **Implementation notes:** no real media/provider node; scheduler remains generic.
- **Tests required:** every JobStep state, retry/timeout/cancel/review, branch/join and output declaration violation.
- **Definition of Done:** fake DAG passes full state/event/checkpoint suite.

### T402 — Register built-in movie recap V2 graph

- **Goal:** encode node catalog/dependencies/policies as versioned built-in Pipeline.
- **Files/modules affected:** Pipeline definition seed + validation tests.
- **Dependencies:** T400–T401.
- **Implementation notes:** node executors may route legacy adapter; include automatic/studio policy variants without graph hacks.
- **Tests required:** graph snapshot, V1 alias mapping, start/stop boundaries, every required contract field.
- **Definition of Done:** graph activates and produces deterministic JobSteps for both modes.

### T410 — Implement review orchestration backend

- **Goal:** connect Script/Timeline/scene-match review resources to generic review nodes.
- **Files/modules affected:** review application service/API/event tests.
- **Dependencies:** T231, T233–T234, T331, T402.
- **Implementation notes:** exact resource version/actor/audit; reject action declared by node.
- **Tests required:** approve/edit/reject/reconnect/stale revision/unauthorized actor.
- **Definition of Done:** durable studio gates resume correct graph without browser state.

### T420 — Implement `build_timeline` native node

- **Goal:** transform Script/match/Narration/subtitle proposals into canonical TimelineVersion.
- **Files/modules affected:** engine Timeline builder/node tests.
- **Dependencies:** T232, T401–T402; adapter-produced proposal fixtures.
- **Implementation notes:** no renderer/matching decision; preserve provenance/user overrides.
- **Tests required:** schema/cross-refs, deterministic fingerprint, degraded inputs, user-origin merge.
- **Definition of Done:** node output is a valid proposed TimelineVersion and renderer needs no matches file.

### T430 — Build frontend shell and API client

- **Goal:** create auth-aware Project navigation/API client/error/progress snapshot foundation.
- **Files/modules affected:** `apps/web`, `packages/sdk`, shared client contract tests.
- **Dependencies:** T106, T210, T230, T340; OQ-01/OQ-04.
- **Implementation notes:** no direct worker/provider/S3 credential except presigned URL.
- **Tests required:** auth/error/pagination/reconnect, secret scan, accessibility smoke.
- **Definition of Done:** dashboard lists Projects/Jobs and survives SSE reconnect.

### T431 — Build Script review editor

- **Goal:** edit version, approve/reject and resume review Job.
- **Files/modules affected:** web Script UI/e2e tests.
- **Dependencies:** T231, T410, T430.
- **Implementation notes:** immutable versions/ETag conflict UX; no local-only authoritative draft.
- **Tests required:** Scenario B, conflict, reload, approval audit, role denial.
- **Definition of Done:** generated Script can pause→edit→new version→resume deterministically.

### T432 — Build Timeline/Scene editor minimum slice

- **Goal:** inspect Scenes, replace/move Clip and create approved TimelineVersion.
- **Files/modules affected:** web Timeline/Scene UI/e2e tests.
- **Dependencies:** T233–T234, T410, T420, T430; OQ-08.
- **Implementation notes:** use approved command/full-replace contract; retain origin/proposal refs.
- **Tests required:** Clip replace/version/undo-conflict/reload, invalid range, user override rerun.
- **Definition of Done:** Scenario C edits one Clip and schedules rerender without AI-stage commands.

### T433 — Build desktop client shell and remote-session integration

- **Goal:** provide a first-class desktop client that uses the same Product API, SDK, contracts,
  auth/session, upload, event reconnect and artifact download behavior as web.
- **Files/modules affected:** `apps/desktop`, `packages/sdk`, desktop integration/e2e tests; no engine code.
- **Dependencies:** T101, T106, T210, T223, T340, T430; desktop shell choice approved in setup gate.
- **Implementation notes:** remote-server mode is default; native permissions are limited to file
  picker/download/notification as needed; no provider/DB/engine secret or authoritative local state.
- **Tests required:** server URL per environment, login/logout/token revocation, direct multipart
  upload, reconnect after app restart, safe error display, role denial, no secret/path leakage.
- **Definition of Done:** desktop can authenticate, upload, follow a Job, review a version and
  download an Artifact through Product API without direct engine/provider access.

### T434 — Package and secure desktop release baseline

- **Goal:** produce reproducible desktop dev/staging builds, platform permission manifest and
  signed-update decision boundary without coupling the client to server internals.
- **Files/modules affected:** desktop packaging metadata, CI matrix, release/runbook tests.
- **Dependencies:** T433; desktop framework and distribution policy approved.
- **Implementation notes:** signing keys stay outside the repository; update/deep-link origins are
  allowlisted; client releases never hard-code a production secret or localhost endpoint.
- **Tests required:** clean build, install/uninstall, update verification, tampered package rejection,
  endpoint configuration, OS permission review and rollback to previous desktop build.
- **Definition of Done:** a signed/reproducible staging client can be rolled back and connects only
  to an authorized API origin.

## Gate G — Providers, intelligence and rendering

### T500 — Implement provider ports/resolver/conformance suite

- **Goal:** establish LLM/VLM/TTS/ASR/Embedding interfaces, resolver, errors and fake adapters.
- **Files/modules affected:** engine provider contracts/registry/tests.
- **Dependencies:** T102, T203, T235, T401; OQ-05.
- **Implementation notes:** no vendor branch in nodes; allowlisted registration; credential scope outside serialization.
- **Tests required:** every contract/error/cancel/timeout/fallback/redaction/capability path.
- **Definition of Done:** fake adapters for all five kinds pass one conformance framework.

### T510 — Port LLM Script provider/node

- **Goal:** replace legacy generate_script via one approved LLM adapter while preserving rollback.
- **Files/modules affected:** LLM adapter, Script node, Pipeline route/tests.
- **Dependencies:** T004, T402, T500.
- **Implementation notes:** structured schema validation, prompt version/provenance, feature route.
- **Tests required:** golden structure/quality tolerances, retry/fallback/auth/content errors, rollback.
- **Definition of Done:** native node parity gate passes and old node remains selectable.

### T511 — Port TTS/Narration node

- **Goal:** synthesize Narration/Artifact through TTS port with compatible cache/voice behavior.
- **Files/modules affected:** TTS adapter/node/cache/Narration tests.
- **Dependencies:** T231, T500, T510.
- **Implementation notes:** one approved provider first; Edge policy enforced; cache full fingerprint.
- **Tests required:** provider conformance, voice/cache, audio probe, cancellation, golden metadata, rollback.
- **Definition of Done:** exact ScriptVersion produces traceable ready Narration and reusable Artifact.

### T512 — Port ASR/alignment node

- **Goal:** expose configured WhisperX/faster-whisper/FunASR fallback under ASR/timing contract.
- **Files/modules affected:** ASR adapters/alignment node/tests.
- **Dependencies:** T500–T511.
- **Implementation notes:** one backend per PR if needed; segment/word fallback explicit.
- **Tests required:** backend matrix, timing corpus, optional deps, degradation/strict, model fingerprint.
- **Definition of Done:** timing Artifact parity and fallback provenance pass.

### T520 — Port Scene detection/feature extraction

- **Goal:** create source-revision Scene records/thumbnails/features without path identity.
- **Files/modules affected:** scene/media nodes and corpus tests.
- **Dependencies:** T224, T401, T313.
- **Implementation notes:** port PySceneDetect first; full-length fallback policy explicit; visual scaffold separate executor.
- **Tests required:** ranges/thresholds/zero-scene/timeout/malicious media/source revision.
- **Definition of Done:** native Scene output passes corpus and rollback route.

### T521 — Add VLM scene caption adapter/node

- **Goal:** analyze keyframe Artifacts with typed VLM response/provenance.
- **Files/modules affected:** VLM adapter/keyframe node/tests.
- **Dependencies:** T500, T520.
- **Implementation notes:** extraction separate from provider; partial per-scene policy explicit.
- **Tests required:** fallback/provider errors, malformed output, item provenance, privacy/egress/cancel.
- **Definition of Done:** captions trace exact Scene/source/model and degraded items are visible.

### T522 — Add Character/Appearance node

- **Goal:** detect/track/cluster characters while preserving user-confirmed identity.
- **Files/modules affected:** ML node/domain persistence/tests.
- **Dependencies:** T207, T520–T521.
- **Implementation notes:** model/legal/privacy policy recorded; AI cluster version does not mutate confirmed entity.
- **Tests required:** corpus, cluster stability, merge/confirmation rerun, artifact/model provenance.
- **Definition of Done:** Character outputs support studio review without identity loss.

### T523 — Add Embedding adapter/index artifact

- **Goal:** produce versioned text/image embedding artifacts/index for matching.
- **Files/modules affected:** Embedding adapter/node/storage tests.
- **Dependencies:** T500, T521; OQ-07.
- **Implementation notes:** model/dimension/metric in fingerprint; no cross-model comparison.
- **Tests required:** conformance, batch/partial, artifact/index reload, invalid model space, benchmark.
- **Definition of Done:** deterministic item index can be consumed through provider-neutral contract.

### T524 — Refactor multimodal matching and coverage feedback

- **Goal:** produce score-component match proposal and optional Script rewrite proposal.
- **Files/modules affected:** matching/coverage nodes/tests.
- **Dependencies:** T510, T512, T520–T523.
- **Implementation notes:** port V1 scorer before improvements; renderer never consumes raw match directly; approved script immutable.
- **Tests required:** V1 quality baseline, determinism, diversity/quality, degraded inputs, user override, coverage proposal.
- **Definition of Done:** proposal is traceable and quality gate passes with rollback.

### T530 — Implement Timeline compiler and reviewed media process adapters

- **Goal:** compile exact TimelineVersion/Profile into deterministic render graph/argv plan.
- **Files/modules affected:** rendering compiler/media adapters/tests.
- **Dependencies:** T232, T313, T420, T524.
- **Implementation notes:** compile only; no scene rematch; allowlisted codecs/filters/options.
- **Tests required:** Clip ranges/transforms/crop/transitions/audio/subtitles, command injection, deterministic plan.
- **Definition of Done:** same input hashes generate same plan and no matches/AI provider dependency exists.

### T531 — Implement native render and deliverable QA nodes

- **Goal:** execute compiled plan, commit Render Artifacts and validate profile/QA.
- **Files/modules affected:** render/QA nodes, Artifact integration, media tests.
- **Dependencies:** T222, T332, T350, T530.
- **Implementation notes:** incomplete output never canonical; timeout/cancel/checkpoint; V1 renderer remains fallback.
- **Tests required:** golden media, missing streams/duration/black/silence, retry duplicate, process kill, rollback.
- **Definition of Done:** native Render completes solely from TimelineVersion/Profile with QA evidence.

### T532 — Add 16:9, 9:16 and 1:1 profile reuse

- **Goal:** render three target profiles while reusing upstream intermediates.
- **Files/modules affected:** profile seeds/reframe/render dedupe tests.
- **Dependencies:** T531.
- **Implementation notes:** profile-specific reframe is downstream Artifact; no Script/Scene/Match rerun.
- **Tests required:** Scenario D dimensions/codecs/safe-area, provider-call audit, cache invalidation.
- **Definition of Done:** three outputs share unchanged upstream fingerprints/artifacts and only render descendants execute.

## Gate H — Security, operations and migration closeout

### T600 — Enforce worker sandbox and secret/egress policy

- **Goal:** productionize controller/executor boundary and least capability.
- **Files/modules affected:** worker/container/security policy/tests.
- **Dependencies:** T313, T500, T531.
- **Implementation notes:** non-root/read-only/quotas/network allowlist/scoped secrets; render pool receives no provider secret.
- **Tests required:** malicious media/plugin/subprocess/SSRF/resource/secret attempts (Scenario G).
- **Definition of Done:** security matrix in `08` passes with documented residual risks.

### T601 — Add operational observability and graceful recovery

- **Goal:** dashboards/alerts/traces for queue/lease/node/provider/storage/render plus drain/reconcile.
- **Files/modules affected:** telemetry/ops/runbooks/tests.
- **Dependencies:** T340, T600.
- **Implementation notes:** bounded labels/redacted content; durable state remains DB.
- **Tests required:** worker/provider/storage/Redis outages, stuck lease, stream gap, graceful drain, alert firing.
- **Definition of Done:** failure drills identify/recover issue without manual DB state fabrication.

### T602 — Add backup/restore and Artifact inventory verification

- **Goal:** restore PostgreSQL plus object refs/checksums into clean environment.
- **Files/modules affected:** backup/restore tools/runbooks/tests.
- **Dependencies:** T222, T601; OQ-10.
- **Implementation notes:** no destructive cleanup in restore; missing/orphan report explicit.
- **Tests required:** clean restore, point-in-time policy, missing/corrupt object, current pointers/checkpoints.
- **Definition of Done:** restored Product traces selected Job→Timeline→Render→Artifact exactly.

### T603 — Implement legacy data importer

- **Goal:** copy/verify V1 output/tasks/checkpoints into Product domain without path leakage.
- **Files/modules affected:** migration tool/reports/tests.
- **Dependencies:** T003–T004, T206–T208, T330, OQ-09.
- **Implementation notes:** copy→verify→switch→retain; V1 stale running not imported as running; imported proposal warnings.
- **Tests required:** known/unknown outputs, corrupt JSON/checkpoint, ownership, checksum, rollback/report.
- **Definition of Done:** sample legacy Project imports with complete evidence and source remains recoverable.

### T604 — Execute upstream update comparison drill

- **Goal:** prove Scenario H process against a selected upstream delta.
- **Files/modules affected:** comparison report/module audit/compatibility tests; no automatic merge.
- **Dependencies:** T001, T003, T108.
- **Implementation notes:** compare peeled commits/modules/contracts/dependencies/security; classify import/port/ignore.
- **Tests required:** candidate V1 suite and affected golden/adapter tests.
- **Definition of Done:** team can decide upstream change disposition with rollback and no V2 boundary overwrite.

### T605 — Publish compatibility deprecation/removal gate

- **Goal:** decide each remaining V1 surface/module using evidence, not code age/style.
- **Files/modules affected:** release profile/runbook/module audit/docs.
- **Dependencies:** T360–T604; all relevant OQs decided.
- **Implementation notes:** removal only if module-specific parity, usage, migration and rollback criteria pass.
- **Tests required:** full legacy profile, Product e2e A–H, migration/rollback rehearsal.
- **Definition of Done:** approved release declares retained/deprecated/BREAKING CHANGE behavior and no unsupported silent removal.
