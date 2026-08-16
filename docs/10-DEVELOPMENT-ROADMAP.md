# 10 — Development Roadmap

## 1. Delivery rules

- Phase is a release gate, not one PR. PR-sized tasks are in `IMPLEMENTATION-ORDER.md`.
- Do not start a phase until prerequisites and prior acceptance evidence pass.
- Every engine migration has V1 regression, V2 contract, feature-route and rollback evidence.
- Job/Pipeline/Timeline/provider/storage contracts are versioned; changes update docs/ADR before implementation.
- No application code is authorized by this document audit itself.

## 2. Phase -1 — Specification gate

**Scope:** turn master plan and verified V1 behavior into internally consistent implementation contracts.

**Prerequisites:** master `PROJECT_REBUILD_PLAN.md`; local upstream reference available.

**Deliverables:** docs 00–14, Glossary, module audit, consistency matrix, Open Questions, implementation order, audit report and concise AGENTS.md.

**Tests/evidence:** terminology search; domain↔DB↔API↔event status matrix; Timeline JSON parse/schema example checks; upstream commit/route/pipeline/module inventory; scenario A–H trace; filesystem check proving no application source edit.

**Acceptance criteria:**

- all required entities have responsibility/ID/lifecycle/ownership/relations/mutability/persistence;
- canonical Job/JobStep states match DB, REST and events;
- every persistent entity maps to table or documented embedding reason;
- node/provider/storage/worker/timeline contracts are complete;
- every V1 source module classified with reason/tests/removal criteria;
- unresolved decisions have options/pros/cons/recommendation/status and dependent tasks are blocked;
- audit has no critical contradiction with master plan.

**Non-goals:** application scaffolding, migrations, Phase 0 execution, source refactor or “temporary prototype”.

## 3. Phase 0 — Freeze and reproduce upstream

**Scope:** establish immutable, executable V1 behavior baseline before Product/V2 code.

**Prerequisites:** Phase -1 accepted; OQ-13 runtime matrix decision or explicit split test plan.

**Deliverables:** recorded upstream remote/peeled tag/commit; immutable baseline branch/tag; uv lock/environment report; FFmpeg build report; frozen compatibility profile (CLI/routes/config/status/outputs); golden sample outputs and test report; license/dependency/security exception inventory.

**Tests/evidence:** upstream full unit/integration/security commands; Python 3.13 core and decided ML/container matrix; CLI create/config/start/pause/resume; REST tasks/cancel/result/artifacts plus selected batch/schedule/DLQ routes; scene/match/TTS/ASR/render real-media sample; SHA-256 artifact manifest.

**Acceptance criteria:**

- clean environment reproduces V1 or each environment-specific blocker is owner-approved;
- baseline commit and outputs are immutable/reviewable;
- compatibility profile distinguishes preserved/deprecated behavior;
- rollback can run frozen V1 image/environment;
- no V1 behavior changed to make baseline pass.

**Non-goals:** new Product API, V2 abstractions, output-quality improvement or upstream rewrite.

## 4. Phase 1 — Product foundation

**Scope:** repository/package boundaries and local infrastructure, without media pipeline implementation.

**Prerequisites:** Phase 0; OQ-01/OQ-06/OQ-12 decisions needed by auth/ownership/namespace tasks.

**Deliverables:** minimal monorepo skeleton; configuration/secret boundary; FastAPI shell; PostgreSQL/Redis/MinIO dev stack; request/correlation IDs; safe error envelope; health/readiness; empty VideoEngine port seam; baseline single Engine Worker controller shell only when task order reaches it.

**Tests/evidence:** import/dependency-boundary tests; clean infra start/stop; health/readiness dependency failure tests; config/secret redaction; API auth seam/error/OpenAPI smoke; V1 suite still runs unchanged.

**Acceptance criteria:**

- product backend cannot import V1 internals except designated adapter package;
- engine package has no user/billing/HTTP dependency;
- local infra is reproducible, private/default-safe and version-pinned;
- API shell exposes no media upload body/engine key/secret;
- no Kubernetes or premature specialized worker pools required.

**Non-goals:** full DB schema, Project CRUD, actual Job execution, Timeline editor or V1 node port.

## 5. Phase 2 — Product domain, persistence and upload

**Scope:** implement metadata aggregates, storage and large-file ingest before engine execution.

**Prerequisites:** Phase 1; OQ-02/OQ-05/OQ-06/OQ-08/OQ-10 decisions for affected tasks.

**Deliverables:** schema/migrations/repositories for Workflow/Pipeline/Project/Asset/Artifact/Script/Timeline/ProviderConfiguration/RenderProfile; Local/S3/MinIO StoragePort; presigned multipart upload; validation/probe boundary; CRUD/versioning/idempotency/outbox foundations.

**Tests/evidence:** clean/upgrade migration tests; FK/check/index inventory; storage conformance; traversal/symlink/presign ACL; multipart complete/abort/expiry/idempotency; MIME/checksum/probe/quarantine; Script/Timeline optimistic version tests; cross-owner negative tests.

**Acceptance criteria:**

- DB matches `03`; no binary media stored in PostgreSQL;
- Browser uploads directly to object storage and Asset is `ready` only after validation;
- Asset/Artifact semantics and references are unambiguous;
- ScriptVersion/TimelineVersion immutable and no lost update;
- API/domain/event responses contain no durable local path.

**Non-goals:** full V1 pipeline execution, editor UI, GPU workers, AI provider implementation or final renderer.

## 6. Phase 3 — Legacy vertical slice through V2 control plane

**Scope:** execute frozen V1 end-to-end behind VideoEngine while V2 owns Job/PipelineRun/JobStep/events/checkpoints/artifacts.

**Prerequisites:** Phase 2; OQ-03/OQ-04/OQ-09 and worker-state transport/security decisions.

**Deliverables:** canonical state transition service; DB outbox/Redis queue; one Engine Worker controller + sandbox executor; lease/heartbeat/reconcile; VideoEngine + LegacyMovieNarratorAdapter; status/progress/artifact mapping; pause/resume/partial/cancel/retry/DLQ; SSE and optional WebSocket; legacy REST/CLI gateway.

**Tests/evidence:** duplicate delivery/competing workers; process death after each V1 step; stale checkpoint/input; cancel/kill/drain; event ordering/replay/reset; exact V1 soft/hard/strict/status/output aliases; full Browser/API→queue→worker→V1→Artifact trace.

**Acceptance criteria:**

- V1 sample runs as Product Job without path/secret leak;
- Job/Run/Step DB, REST and events always agree;
- `start_from`, `stop_after`, pause/review distinction and resume work without node-specific hacks;
- worker death resumes first incomplete compatible node and does not duplicate Artifact;
- compatibility clients pass frozen profile and rollback route remains usable.

**Non-goals:** porting all V1 nodes, specialized worker pools, collaborative editor or changing output quality.

## 7. Phase 4 — V2 pipeline and studio backend

**Scope:** establish native DAG/node contracts and editable Script/Timeline/Scene/Character backend while execution can still call V1 nodes.

**Prerequisites:** Phase 3; OQ-02/OQ-08/OQ-11.

**Deliverables:** DAG validator/scheduler; typed JobContext/NodeResult; node policy registry; built-in movie recap V2 definition; Script/Timeline/review commands; Scene/Character APIs; canonical Timeline validator/compiler boundary; proposal/user-override merge rules.

**Tests/evidence:** cycle/schema/capability/dependency failure; all node lifecycle/retry/timeout/manual gate paths; partial graph boundaries; Timeline schema/cross-ref/negative corpus; version conflict/approval/reject; user override preserved across AI proposal rerun.

**Acceptance criteria:**

- every activated node has full contract from `05`;
- automatic and studio modes use same graph plus declared review policy;
- TimelineVersion is renderer decision input even if rendering remains V1-adapted;
- changing Script/Timeline invalidates only dependency descendants;
- no Product API endpoint exposes V1 implementation details.

**Non-goals:** full frontend editor polish, multimodal quality claims, all providers or final multi-output renderer.

## 8. Phase 5 — Studio frontend and review workflow

**Scope:** user-facing project/script/scene/subtitle/voice/timeline review over Product API only.

**Prerequisites:** Phase 4; primary progress transport and edit command format decided.

**Deliverables:** project dashboard; upload/status; Script editor/version review; Scene/Character browser; match/Clip override; subtitle/voice controls; Timeline editor; render panel; SSE reconnect/snapshot reducer.

**Tests/evidence:** browser authorization; reload/reconnect/retention-gap; optimistic conflict; undo/version creation; review approve/reject; accessibility/basic responsive behavior; no secret/direct provider/engine calls; e2e edit→resume→render.

**Acceptance criteria:**

- user generates Script, pauses, edits exact version and resumes deterministically;
- user replaces a Clip and rerenders without rerunning unrelated AI;
- AI rerun cannot overwrite user-origin edit silently;
- browser never receives engine/provider/storage credentials beyond scoped presigned URLs;
- persisted state survives tab/browser loss.

**Non-goals:** real-time multi-user collaboration/CRDT, billing, mobile-native app or public plugin marketplace.

## 9. Phase 6 — AI intelligence and provider ports

**Scope:** port/upgrade Script/TTS/ASR/Scene/VLM/Embedding/Character/Matching nodes one at a time.

**Prerequisites:** Phase 4; provider credential/vector decisions; Phase 0 corpora/golden metrics.

**Deliverables:** typed provider adapters/resolver; Narration; ASR/alignment; scene captions; text/visual embeddings; Character appearances; multimodal scorer; diversity/quality filters; coverage feedback creating proposal versions.

**Tests/evidence:** provider conformance/fake/local/approved remote paths; timeout/rate-limit/auth/content/fallback; model/provenance/cache fingerprint; scene/character corpus; deterministic scoring components; quality comparison against V1; privacy/redaction/egress.

**Acceptance criteria:**

- pipeline contains no provider-specific branch outside adapters;
- provider outage behavior follows declared retry/fallback/soft-hard policy;
- matching outputs trace component scores and exact features/provider versions;
- Character confirmation and approved ScriptVersion survive reruns;
- every migrated node can route back to V1 until removal gate.

**Non-goals:** promise of a specific commercial provider, unsupported face-recognition use, fully autonomous rewrite of approved content or V1 wholesale removal.

## 10. Phase 7 — Timeline renderer, multi-output and production hardening

**Scope:** native rendering from Timeline/Profile, three output families, then security/operations/migration readiness.

**Prerequisites:** Phase 5–6 required nodes; retention/security/deployment decisions; V1 renderer baseline.

**Deliverables:** Timeline compiler; reviewed FFmpeg/media process port; 16:9, 9:16, 1:1 profiles; profile-specific reframe; narration/BGM/SFX/subtitle mix; render QA/dedupe; sandbox/resource/egress controls; observability/backups/restore/drain; legacy import/deprecation runbook.

**Tests/evidence:** three profiles from one Timeline; no upstream AI calls on profile-only rerender; codecs/dimensions/loudness/subtitles/safe area; timeout/cancel/process-tree; malicious media/security suite; load/failure drills; DB/object restore inventory; legacy import/checksum/rollback.

**Acceptance criteria:**

- renderer makes no scene-match decision outside Timeline;
- intermediate artifacts are reused safely across outputs;
- untrusted media impact is contained and render worker has no unnecessary secrets;
- backup/restore and worker/provider/storage failure scenarios are demonstrated;
- compatibility removal only follows module removal criteria and announced window.

**Non-goals:** mandatory Kubernetes, premature microservices, billing/subscription or removing all legacy code merely because V2 renders successfully.

## 11. Phase exit evidence template

Every phase stores:

```text
Phase / date / owner
Prerequisites satisfied
Deliverables and versions
Commands/tests and reports
Acceptance criteria evidence
Security/compatibility review
Migration/rollback
Known limitations/open questions
Explicit non-goals unchanged
Decision: pass | fail
```

Compile success is not phase acceptance; behavior, state consistency, artifact durability, security boundary and rollback must be demonstrated.
