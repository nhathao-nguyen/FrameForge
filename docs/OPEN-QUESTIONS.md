# Open Questions

Recommendation is not an approved decision. Owner must replace `OPEN` with a dated decision and
update affected specs/tasks before dependent implementation. `DECIDED` entries below record the
owner decision and remain subject to their implementation evidence. Architectural invariants already
fixed by the master plan are not reopened here.

## OQ-01 — Product identity provider

**Question**

Which authentication/session mechanism will Product API and Next.js use?

**Why it matters**

It fixes token/session validation, `users.external_subject`, local development, API keys, CSRF/CORS and Phase 1 package dependencies.

**Option A — External OIDC only**

Pros: small password/security surface; standard JWT/OIDC; supports managed or self-hosted IdP.

Cons: local/offline setup and account linking depend on IdP behavior; still needs local authorization records.

**Option B — Product-owned email/password/session**

Pros: full UX/control and offline independence.

Cons: largest security/operations scope (password reset, MFA, session revocation, abuse); not core media value.

**Option C — OIDC identity + local authorization/API keys**

Pros: delegates authentication while Product owns Workspace membership/roles/API integration; clear boundary.

Cons: two identity layers must synchronize subject/email and dev bootstrap.

**Recommendation**

Option C. Select concrete IdP and claims before auth middleware/repository tasks.

**Decision status:** OPEN

Blocks: T110, T120, T210.

## OQ-02 — Timeline read projections

**Question**

Should Phase 2 create normalized Track/Clip read projections in addition to canonical immutable TimelineVersion JSONB?

**Why it matters**

Canonical source is already JSONB; projection timing affects migrations, query/editor complexity and consistency jobs.

**Option A — JSONB only initially**

Pros: one source of truth; smallest migration and atomic edit surface; enough for project-scoped editor load.

Cons: cross-timeline Clip analytics/filter queries are less efficient.

**Option B — Build rebuildable projection immediately**

Pros: fast server-side Clip/Track queries and analytics.

Cons: extra tables/indexes/rebuild logic before measured need; risk consumers treat projection as canonical.

**Option C — External/search projection later**

Pros: isolates analytics/search scale from transactional DB.

Cons: additional infrastructure and eventual consistency.

**Recommendation**

Option A in Phase 2; introduce B only after measured query requirement. Renderer always reads canonical document.

**Decision status:** OPEN

Blocks: only projection-specific schema task; does not block canonical TimelineVersion table/validator.

## OQ-03 — Redis work-queue primitive

**Question**

Which Redis primitive/library implements QueuePort for initial V2?

**Why it matters**

Ack, pending reclaim, delayed retry, priority and operational tooling differ; PostgreSQL/outbox remains source of truth regardless.

**Option A — Redis Streams consumer groups**

Pros: ack/pending/claim/replay primitives fit at-least-once; observable without a large framework.

Cons: delayed retry/priority need explicit design; consumer-group operations must be implemented carefully.

**Option B — Redis Lists + sorted sets + Pub/Sub**

Pros: simple work pop and explicit delayed/priority queues.

Cons: reclaim/atomicity scripts are custom; live Pub/Sub has no durability (DB replay still required).

**Option C — RQ/Celery/other broker framework**

Pros: mature retry/scheduling/worker tooling.

Cons: framework state semantics can conflict with Job/JobStep truth; larger dependency/abstraction leakage.

**Recommendation**

Option A behind QueuePort, with PostgreSQL lease/outbox authoritative and a focused reclaim/load prototype in the queue task—not before Phase 0.

**Decision status:** OPEN

Blocks: T310–T312.

## OQ-04 — Primary browser progress transport

**Question**

Should frontend use SSE or WebSocket as the primary progress transport?

**Why it matters**

It affects gateway timeouts, auth refresh, SDK/reconnect implementation and load tests. Event envelope/replay semantics are already shared.

**Option A — SSE primary, WebSocket optional**

Pros: native server→browser model, simple Last-Event-ID/proxy behavior, lower protocol complexity.

Cons: bidirectional future features need REST/another channel; some gateways need streaming config.

**Option B — WebSocket primary, SSE fallback**

Pros: bidirectional extensibility and one long-lived socket.

Cons: more connection/auth/reconnect/load-balancer complexity; invites state commands over socket.

**Option C — REST polling MVP**

Pros: operationally simplest.

Cons: violates requested live progress quality, higher polling load, slower review updates.

**Recommendation**

Option A. Keep commands on REST and implement WebSocket only as same-envelope compatibility/future transport.

**Decision status:** OPEN

Blocks: T340 frontend transport choice; durable event API is not blocked.

## OQ-05 — Provider credential ownership and secret backend

**Question**

Are provider credentials system-, Workspace-, or User-owned, and which secret backend supplies `credential_ref`?

**Why it matters**

It controls authorization, cost attribution, key rotation, worker secret scope, local/offline behavior and ProviderConfiguration API.

**Option A — System credentials only**

Pros: simplest MVP/operations; centralized provider policy.

Cons: poor tenant isolation/cost ownership; all users share quota and risk.

**Option B — Workspace credentials in secret manager**

Pros: clean team policy/cost boundary; admin-managed; worker can receive scoped reference.

Cons: requires secret-manager integration and rotation UX.

**Option C — User BYOK credentials**

Pros: per-user cost/privacy choice.

Cons: most complex permissions/revocation/support; Jobs become sensitive to membership/user deletion.

**Recommendation**

Option B for product deployment plus system/local config for single-user offline mode. Select concrete secret backend before write API; never store plaintext in PostgreSQL.

**Decision status:** OPEN

Blocks: T112, T217, T500 and production provider configuration endpoints.

## OQ-06 — Workspace scope in MVP

**Question**

Will MVP enforce Workspace-first ownership or run a single-user bootstrap while retaining Workspace-shaped repositories?

**Why it matters**

Master plan does not prioritize multi-tenant, but adding ownership later risks cross-project leakage and schema rewrite.

**Option A — Single implicit Workspace/user**

Pros: simplest UX/local setup; no team management UI.

Cons: authorization assumptions can leak into queries; migration to teams needs care.

**Option B — Workspace-first data/RBAC, single Workspace bootstrap allowed**

Pros: tenant scope is structural from day one; local mode stays simple through bootstrap.

Cons: auth/membership code appears earlier than immediate single-user need.

**Option C — Organization → Workspace → Project hierarchy now**

Pros: enterprise hierarchy ready.

Cons: conflicts with “do not over-engineer/billing/multi-tenant early”; largest scope.

**Recommendation**

Option B, without organization/billing. Repositories always require Workspace context; local setup seeds one Workspace.

**Decision status:** OPEN

Blocks: T120, T200–T210 and legacy ownership mapping.

## OQ-07 — Persistent embedding storage

**Question**

Where should persistent text/image embeddings live in Phase 6?

**Why it matters**

It affects model-version invalidation, matching latency, backup, query pattern and dependency footprint.

**Option A — Artifact blobs + metadata/index manifest**

Pros: follows storage abstraction; simple backup/version/checksum; no DB extension initially.

Cons: nearest-neighbor queries require loading/building local index.

**Option B — PostgreSQL pgvector**

Pros: transactional metadata/vector queries; familiar operations at moderate scale.

Cons: adds extension/index tuning and DB storage/load; multimodal/model partitions need design.

**Option C — External vector service**

Pros: specialized scale/search features.

Cons: extra service/cost/consistency/privacy; premature without benchmarks.

**Recommendation**

Option A until Phase 6 benchmarks demonstrate B; do not choose C without workload evidence.

**Decision status:** OPEN

Blocks: persistent index implementation in T523; does not block provider port or Artifact schema.

## OQ-08 — Timeline edit transport

**Question**

How does frontend submit a new TimelineVersion?

**Why it matters**

It determines conflict UX, audit diff, command validation, undo/redo and payload size; immutable versioning/If-Match is already fixed.

**Option A — RFC 6902 JSON Patch**

Pros: standard, compact and generic.

Cons: array-index operations are fragile; semantic intent may be hard to audit.

**Option B — Full document replacement**

Pros: simplest validation and client state model.

Cons: larger payload/diff and less semantic audit; still needs conflict handling.

**Option C — Domain edit commands**

Pros: clear intent/invariants/audit (`MoveClip`, `ReplaceSource`, `SplitClip`).

Cons: more API surface and frontend/server command parity; import still needs full document path.

**Recommendation**

Option C for editor actions plus Option B for import/replace; avoid generic array-index patch as primary. Prototype command set from actual editor operations before freezing API.

**Decision status:** OPEN

Blocks: T421/T530 and Timeline editor mutation client.

## OQ-09 — Legacy Task ownership mapping

**Question**

How does a V1 `/tasks` request without Project/Workspace identity map into Product domain?

**Why it matters**

Compatibility must not create unowned data or let an engine API key bypass product authorization.

**Option A — One temporary Project per Task**

Pros: deterministic isolation/history; easy cleanup policy.

Cons: project clutter and weaker grouping for repeat client.

**Option B — Authenticated API key/user has configured default Project**

Pros: coherent history/assets; natural for long-lived client.

Cons: needs provisioning and collision/concurrency policy.

**Option C — Compatibility namespace outside Project until import**

Pros: closest to V1 and minimal immediate product records.

Cons: splits source of truth and hides Artifacts/Jobs from product UI.

**Recommendation**

Option B for provisioned clients, fallback A for local/import-only profile. Never default C.

**Decision status:** OPEN

Blocks: T360/T361 and legacy data importer.

## OQ-10 — Retention/privacy defaults

**Question**

What default retention applies to source media, intermediate Artifacts, checkpoints, renders, provider content and logs?

**Why it matters**

Replay/resume, storage cost, privacy/legal obligations and backup all depend on it.

**Option A — Retain everything until user delete**

Pros: maximum reproducibility; simple reference policy.

Cons: highest cost/privacy exposure; temp/log accumulation.

**Option B — Class-based Workspace policy**

Pros: source/renders can be durable while temp/proxy/checkpoint/log have bounded TTL; configurable.

Cons: needs reference-aware sweeper and understandable UX.

**Option C — Privacy-first auto-delete source after output**

Pros: minimizes sensitive source retention/cost.

Cons: prevents rerender/resume unless user pins; surprising and destructive if default.

**Recommendation**

Option B. Keep source/final renders until explicit delete by default; short TTL only for staging/temp/log and expired checkpoints after protection checks.

**Decision status:** OPEN

Blocks: T224/T610 and production policy; does not block immutable Artifact model.

## OQ-11 — Pipeline authoring scope

**Question**

Who can author/activate Pipeline definitions in the initial product?

**Why it matters**

Public graph/node/plugin authoring greatly expands validation, security and UI surface.

**Option A — Built-in versioned Pipelines only**

Pros: controlled contracts/security; enough for movie recap strangler.

Cons: no user custom workflow initially.

**Option B — Admin-authored declarative graph**

Pros: flexibility without arbitrary code if node registry fixed.

Cons: needs activation UI/API, schema migration and stronger graph/resource validation.

**Option C — User Python/plugin nodes**

Pros: maximal extension.

Cons: arbitrary code, supply-chain and secret/sandbox risk; conflicts with production plugin rule.

**Recommendation**

Option A through Phase 4; evaluate B after built-in DAG/runtime is stable. C is trusted-deployment-only, never public default.

**Decision status:** OPEN

Blocks: pipeline mutation/admin API; built-in Pipeline implementation can proceed after owner confirms A.

## OQ-12 — V2 package/product namespace

**Question**

What stable package/import namespace should new code use?

**Why it matters**

It affects repository layout, imports, publishing, SDK names and long-lived adapter boundaries. Legacy namespace must remain unchanged.

**Option A — Temporary `your_engine`**

Pros: matches master placeholder.

Cons: guaranteed later rename/churn; easy to leak placeholder publicly.

**Option B — Final product namespace after naming decision**

Pros: stable public imports/packages.

Cons: blocks scaffold until naming owner decides.

**Option C — Neutral internal `video_engine` package**

Pros: descriptive and product-name independent.

Cons: generic/collision risk and may not fit publishing.

**Recommendation**

Option B. Until decided, docs use conceptual `services/engine`; do not create placeholder application package.

**Decision status:** DECIDED — 2026-08-16

**Decision owner:** repository owner

**Decision:** Use `nh_media` as the V2 Python/package namespace. Do not use `your_engine`,
`video_engine` or `frameforge` as the public V2 namespace at this time. Keep the legacy
`movie_narrator` namespace unchanged.

Blocks: resolved for T100; Phase 0 baseline tasks remain required before application scaffold.

## OQ-13 — Python version split for legacy ML/container

**Question**

Should all packages/containers standardize on Python 3.13, or keep V1/ML container on 3.12 while Product/API development uses 3.13?

**Why it matters**

Master specifies Arch + uv + Python 3.13. Upstream supports 3.13 in CI, but its Dockerfile intentionally uses 3.12 for ML wheel compatibility.

**Option A — Python 3.13 everywhere**

Pros: one runtime; exactly follows master setup.

Cons: optional ML/CUDA wheels may fail or require different pins/builds; must be proven, not assumed.

**Option B — Product/API/core 3.13; frozen legacy/ML image 3.12 temporarily**

Pros: honors development target while preserving upstream container compatibility and strangler isolation.

Cons: two runtime matrices/lock groups and serialization boundary discipline.

**Option C — Python 3.12 everywhere initially**

Pros: closest to upstream image/ML availability.

Cons: contradicts explicit master development recommendation without sufficient need for Product/API.

**Recommendation**

Option B until Phase 0 dependency/ML matrix proves A. Record separate locks/images and remove split when 3.13 parity passes.

**Decision status:** DECIDED — 2026-08-16

**Decision owner:** repository owner

**Decision:** Product/API/core target Python 3.13. The frozen legacy/ML image temporarily uses
Python 3.12 with separate locks/images. Merge the runtimes only after Phase 0 proves dependency,
ML and CUDA parity on Python 3.13.

Blocks: resolved for T002; the separate runtime/lock/image evidence is still required by Phase 0.

## OQ-14 — Frozen V1 compatibility profile breadth

**Question**

Must V2 compatibility preserve only core task routes/CLI, or all V1 batch/schedule/DLQ/distributed features?

**Why it matters**

Source verification found more public behavior than the initial docs listed. Preserving all may enlarge migration scope; dropping any is a documented breaking change.

**Option A — Preserve all verified public CLI/REST behavior**

Pros: strongest backward compatibility; no surprise for existing consumers.

Cons: imports scheduler/batch/distributed scope not central to product.

**Option B — Preserve tasks/results/artifacts/health plus DLQ; deprecate batch/schedule/distributed**

Pros: focuses on likely core clients while retaining reliability behavior.

Cons: requires usage evidence and explicit breaking-change window.

**Option C — Preserve only `mn create` and core task submission/status**

Pros: smallest compatibility implementation.

Cons: highest breakage and conflicts with “keep behavior” absent strong evidence.

**Recommendation**

Option A for the frozen baseline unless owner can prove unused surfaces and approve B with a deprecation window. Do not choose C by convenience.

**Decision status:** DECIDED — 2026-08-16

**Decision owner:** repository owner

**Decision:** Option A. Preserve all verified public V1 CLI/REST behavior for the frozen
compatibility baseline, including batch, schedule, DLQ and distributed surfaces. Any later
deprecation/removal requires usage evidence, a compatibility replacement, an announced
owner-approved deprecation window and a rollback path.

Blocks: resolved for T003/T360–T362; T003 must still produce the executable profile and evidence.

## OQ-15 — Desktop client shell/runtime

**Question**

Which desktop shell should package the first-class remote Product API client?

**Why it matters**

It fixes native permission boundaries, build/signing/update tooling, OS support, local filesystem
integration and CI runners. It does not change the Product API, Engine, Worker or storage boundary.

**Option A — Tauri 2 with web UI**

Pros: small native footprint, explicit capability permissions, suitable for a remote-first client,
and avoids bundling Python/FFmpeg/ML runtimes.

Cons: Rust toolchain/native build matrix and plugin review are additional skills/CI requirements.

**Option B — Electron**

Pros: mature web ecosystem and straightforward reuse of the web client.

Cons: larger runtime, broader native surface and stronger hardening/update discipline.

**Option C — Native client per operating system**

Pros: strongest platform integration and native UX.

Cons: highest maintenance cost and duplicated client behavior; does not improve the server boundary.

**Recommendation**

Option A for a lightweight remote-first client, subject to owner approval and a signed-build proof.
The desktop client must remain a Product API client; it must not embed the engine or become a second
business backend.

**Decision status:** DECIDED — 2026-08-16

**Decision owner:** repository owner

**Decision:** Use Tauri 2 for the first-class desktop client. It is a lightweight remote-first
Product API client and must not bundle Python, FFmpeg, ML runtime, database, Redis, engine or
provider secrets. Tauri capabilities are least-privilege; signing and update policy require proof
before production release.

Blocks: resolved for T433–T434; signing/update/permission evidence remains a task acceptance gate.
