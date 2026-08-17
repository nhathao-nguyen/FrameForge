# Open Questions

Recommendations are not decisions. An OPEN question blocks only tasks listed in that section.
Owner-confirmed architecture is recorded under `RESOLVED BY THIS ARCHITECTURE` and must not be
reopened during implementation without a new dated owner decision.

## OWNER DECISION REQUIRED

## OQ-01 — Product identity provider

**Category:** OWNER DECISION REQUIRED

**Question:** Which authentication/session mechanism will Product API and Next.js/Tauri use?

**Why:** It fixes token/session validation, local bootstrap, CSRF/CORS and identity dependencies.

**Options:** A) external OIDC only; B) product-owned password/session; C) OIDC authentication plus
local authorization/API keys.

**Recommendation:** C, with a concrete IdP/claims contract selected before auth implementation.

**Decision status:** OPEN

Blocks: T106 auth middleware, T201, T210, T430.

## OQ-05 — Provider credential ownership and secret backend

**Category:** OWNER DECISION REQUIRED

**Question:** Are credentials system-, Workspace- or User-owned, and which secret backend supplies
scoped references?

**Why:** Authorization, cost attribution, rotation, local use and worker secret scope depend on it.

**Options:** A) system credentials; B) Workspace credentials in a secret manager; C) user BYOK.

**Recommendation:** B for product deployment plus explicit system/local configuration for a
single-user Local/LAN profile. Never plaintext PostgreSQL storage.

**Decision status:** OPEN

Blocks: T102 real secrets, T203, T235, T500.

## OQ-06 — Workspace scope in MVP

**Category:** OWNER DECISION REQUIRED

**Question:** Workspace-first RBAC or single-user bootstrap with Workspace-shaped repositories?

**Why:** Ownership must not be retrofitted after data and API behavior exist.

**Options:** A) implicit single Workspace; B) Workspace-first schema/RBAC with one-Workspace
bootstrap; C) organization hierarchy now.

**Recommendation:** B; do not add organization/billing yet.

**Decision status:** OPEN

Blocks: T201, T210 and authorization-dependent resources.

## OQ-10 — Retention/privacy defaults

**Category:** OWNER DECISION REQUIRED

**Question:** What retention applies to source media, intermediates, checkpoints, renders, provider
content and logs?

**Why:** Replay, storage cost, privacy and backup depend on it.

**Options:** A) retain until explicit delete; B) class-based Workspace policy; C) auto-delete source
after output.

**Recommendation:** B; source/final outputs retained by default, short TTL only for unprotected
staging/temp/log classes.

**Decision status:** OPEN

Blocks: T602 and production deletion policy; does not block immutable Artifact design.

## IMPLEMENTATION DECISION

## OQ-02 — Timeline read projections

**Category:** IMPLEMENTATION DECISION

**Question:** Add normalized Track/Clip read projections in Phase 2?

**Options:** A) JSONB only; B) rebuildable PostgreSQL projection; C) external/search projection later.

**Recommendation:** A until measured queries justify B. Renderer always reads TimelineVersion.

**Decision status:** OPEN

Blocks: projection-specific part of T208 only.

## OQ-03 — Redis QueuePort primitive

**Category:** IMPLEMENTATION DECISION

**Question:** Redis Streams consumer groups or Lists/sorted sets for initial QueuePort?

**Options:** A) Streams; B) Lists + sorted sets; C) broker framework.

**Recommendation:** A behind QueuePort, with PostgreSQL lease/outbox authoritative.

**Decision status:** OPEN

Blocks: T310–T312.

## OQ-04 — Primary progress transport

**Category:** IMPLEMENTATION DECISION

**Question:** SSE or WebSocket as the primary client progress transport?

**Options:** A) SSE primary, WebSocket optional; B) WebSocket primary; C) polling MVP.

**Recommendation:** A; commands remain REST and both transports share one envelope/replay service.

**Decision status:** OPEN

Blocks: T340 and client transport choice; durable events are not blocked.

## OQ-08 — Timeline edit transport

**Category:** IMPLEMENTATION DECISION

**Question:** JSON Patch, full replacement or domain edit commands?

**Options:** A) RFC 6902; B) full document; C) domain commands plus full import/replace.

**Recommendation:** C, with B for import/replace; avoid array-index patch as primary editor API.

**Decision status:** OPEN

Blocks: T233, T432.

## DEPLOYMENT DECISION

No deployment decision blocks Local/LAN functional development. Public DNS, TLS termination,
CDN, managed services, target OS matrix and canary traffic policy are selected in T605 after T603
Local/LAN certification. They are intentionally not converted into premature service choices here.

## LATER-PHASE DECISION

## OQ-07 — Persistent embedding storage

**Category:** LATER-PHASE DECISION

**Question:** Artifact index, pgvector or external vector service?

**Options:** A) Artifact + index manifest; B) PostgreSQL pgvector; C) external vector service.

**Recommendation:** A until T523 benchmarks justify B; C requires separate scale/privacy evidence.

**Decision status:** DEFERRED

Blocks: persistent index choice in T523 only.

## OQ-11 — Pipeline authoring scope

**Category:** LATER-PHASE DECISION

**Question:** Built-in only, admin declarative graphs or user code?

**Options:** A) built-in versioned Pipelines; B) admin-authored declarative graphs; C) user code.

**Recommendation:** A through the native pipeline release; evaluate B later. C is not a public
default because arbitrary code violates the trust boundary.

**Decision status:** DEFERRED

Blocks: public/admin authoring APIs. T202/T400 may implement built-in-only definitions.

## RESOLVED BY THIS ARCHITECTURE

## OQ-09 — Upstream task ownership mapping

**Category:** RESOLVED BY THIS ARCHITECTURE

**Decision:** No mapping exists. NH-Media does not expose an upstream task/CLI/REST compatibility
surface or import upstream runtime state. External user media uses native Project/Asset upload.

**Decision status:** DECIDED — 2026-08-17

**Owner:** repository owner

Blocks: none.

## OQ-12 — Product and package namespace

**Category:** RESOLVED BY THIS ARCHITECTURE

**Decision:** Product is NH-Media. Python namespace is `nh_media`. Go module starts at
`github.com/nhathao-nguyen/NH-Media`. `movie_narrator` appears only in upstream research/provenance.

**Decision status:** DECIDED — 2026-08-17

**Owner:** repository owner

Blocks: none.

## OQ-13 — Runtime/language topology

**Category:** RESOLVED BY THIS ARCHITECTURE

**Decision:** Go Product API/control plane; bounded Go media workers; isolated Python `nh_media`
ML/AI workers; versioned language-neutral contracts; no Python/FFmpeg/ML inline in Product API.

**Decision status:** DECIDED — 2026-08-17

**Owner:** repository owner

Blocks: none for topology; T002 still selects exact versions.

## OQ-14 — Upstream relationship

**Category:** RESOLVED BY THIS ARCHITECTURE

**Decision:** Movie Narrator is research/reference-only. No runtime, build, deployment, import,
compatibility, migration or rollback dependency. Capability research is classified in the dedicated
matrix and implementations are independently authored.

**Decision status:** DECIDED — 2026-08-17

**Owner:** repository owner

Blocks: none.

## OQ-15 — Desktop runtime

**Category:** RESOLVED BY THIS ARCHITECTURE

**Decision:** Tauri 2 thin remote-first Product API client. It does not bundle Python, FFmpeg
processing, ML models/workers, PostgreSQL, Redis or server/provider secrets.

**Decision status:** DECIDED — 2026-08-17

**Owner:** repository owner

Blocks: signing/update/permission evidence remains T434, not an architecture decision.
