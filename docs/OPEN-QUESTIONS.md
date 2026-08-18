# Decision Closure Register

Owner ratification dated 2026-08-17 closes every architecture/product question required for
implementation. This register preserves the former OQ identifiers and their final disposition.
There are no unresolved owner or implementation blockers.

Allowed statuses are `RESOLVED`, `DEFERRED-NONBLOCKING` and `OBSOLETE`. A deferred item below is a
later vendor/configuration/optimization choice and explicitly does not block implementation.

## OQ-01 — Product identity provider

**Category:** ARCHITECTURE

**Decision:** Use `AuthPort`. Initial Local/LAN mode uses NH-Media `LocalAuthProvider` with real
authentication, opaque server-controlled sessions and an idempotent bootstrap of one local admin.
Later Internet deployment may add `OIDCAuthProvider`; no external IdP is required for development.
There is no normal `DEV_DISABLE_AUTH` architecture.

**Decision status:** RESOLVED

**Owner/date:** repository owner, 2026-08-17. Blocks: none.

## OQ-02 — Timeline read projections

**Category:** IMPLEMENTATION BASELINE

**Decision:** PostgreSQL JSONB is the canonical initial TimelineVersion document. Track/Clip tables
are not canonical. Rebuildable read projections may be added only after measured query evidence and
can never replace the document as renderer truth.

**Decision status:** RESOLVED

**Owner/date:** repository owner, 2026-08-17. Blocks: none.

## OQ-03 — Redis QueuePort primitive

**Category:** ARCHITECTURE

**Decision:** Use Redis Streams consumer groups behind QueuePort with acknowledgement, pending-entry
reclaim/lease reconciliation, bounded retry and optional transport DLQ. PostgreSQL remains the
canonical Job/attempt/dead-letter state.

**Decision status:** RESOLVED

**Owner/date:** repository owner, 2026-08-17. Blocks: none.

## OQ-04 — Primary progress transport

**Category:** API ARCHITECTURE

**Decision:** SSE is primary for snapshot/replay/progress/completion. REST remains responsible for
commands and authoritative queries. WebSocket is absent from the initial contract and requires a
future genuinely bidirectional use case.

**Decision status:** RESOLVED

**Owner/date:** repository owner, 2026-08-17. Blocks: none.

## OQ-05 — Provider credential ownership and secret backend

**Category:** SECURITY ARCHITECTURE

**Decision:** Use `SecretStore`. Provider credentials are Workspace-owned by default; a system/local
credential may be explicitly configured for the single-server installation. Initial storage is
authenticated-encrypted secret records protected by a server-owned master key supplied outside the
database. Vault/KMS/cloud secret managers are later adapters. Workers receive only the minimum
secret for an execution scope; clients, durable Jobs, Redis messages and logs receive none.

**Decision status:** RESOLVED

**Owner/date:** repository owner, 2026-08-17. Blocks: none.

## OQ-06 — Workspace scope in MVP

**Category:** DOMAIN/AUTHORIZATION

**Decision:** Workspace is first-class from the first migration. Authorization is Workspace-first.
Initial setup creates one default Workspace and one owner membership for the local admin; the domain
supports additional users/Workspaces later without redesign.

**Decision status:** RESOLVED

**Owner/date:** repository owner, 2026-08-17. Blocks: none.

## OQ-07 — Persistent embedding storage

**Category:** LATER OPTIMIZATION

**Decision:** Start with a versioned embedding Artifact plus item-index manifest. T523 may benchmark
pgvector or an external vector service and select one without changing provider/domain contracts.
The exact future index is an operational scale choice and does not block implementation.

**Decision status:** DEFERRED-NONBLOCKING

**Owner/date:** repository owner, 2026-08-17. Blocks: none; T523 has a valid baseline.

## OQ-08 — Timeline edit transport

**Category:** API/DOMAIN

**Decision:** Use typed domain commands with `expected_version`/`If-Match`, stable object IDs and a
new immutable TimelineVersion result. Baseline commands cover add/remove/move/trim clip, subtitle,
narration, track order and scene updates. A privileged validated full-document create/import command
exists only for imports; arbitrary replacement and array-index JSON Patch are not the editor model.

**Decision status:** RESOLVED

**Owner/date:** repository owner, 2026-08-17. Blocks: none.

## OQ-09 — Upstream task ownership mapping

**Category:** UPSTREAM BOUNDARY

**Decision:** No mapping exists. NH-Media does not expose upstream task/CLI/REST compatibility or
import upstream runtime state. External media enters native Workspace/Project/Asset flows.

**Decision status:** RESOLVED

**Owner/date:** repository owner, 2026-08-17. Blocks: none.

## OQ-10 — Retention/privacy defaults

**Category:** INITIAL OPERATIONS

**Decision:** Local/LAN PostgreSQL backup is manual/on-demand initially; MinIO uses a persistent
local volume; source, final and resume/re-edit-required Artifacts are retained until explicit audited
deletion; executor scratch is cleanup-eligible after success; protected intermediates are not
automatically deleted. Later class-based policies are configuration, not architecture.

**Decision status:** RESOLVED

**Owner/date:** repository owner, 2026-08-17. Blocks: none.

## OQ-11 — Pipeline authoring scope

**Category:** LATER PRODUCT CAPABILITY

**Decision:** Initial Pipelines are built-in, versioned and explicitly registered. Later admin
declarative authoring may be evaluated. Arbitrary third-party/user code and unrestricted Python
entry-point loading are excluded. The later authoring UX does not block built-in pipeline work.

**Decision status:** DEFERRED-NONBLOCKING

**Owner/date:** repository owner, 2026-08-17. Blocks: none.

## OQ-12 — Product and package namespace

**Category:** PRODUCT IDENTITY

**Decision:** Product is NH-Media; Python namespace is `nh_media`; Go module is
`github.com/nhathao-nguyen/NH-Media`. `movie_narrator` is research/provenance text only.

**Decision status:** RESOLVED

**Owner/date:** repository owner, 2026-08-17. Blocks: none.

## OQ-13 — Runtime/language topology

**Category:** ARCHITECTURE

**Decision:** Go Product API/control plane; Go media worker with FFmpeg; isolated Python `nh_media`
ML/AI worker; versioned language-neutral contracts. Toolchain patch versions are pinned by T002 and
do not constitute an architecture question.

**Decision status:** RESOLVED

**Owner/date:** repository owner, 2026-08-17. Blocks: none.

## OQ-14 — Upstream relationship

**Category:** INDEPENDENCE

**Decision:** Movie Narrator is research/reference-only. No runtime, build, deployment, import,
compatibility, migration, frozen V1 or rollback dependency is permitted.

**Decision status:** RESOLVED

**Owner/date:** repository owner, 2026-08-17. Blocks: none.

## OQ-15 — Desktop runtime

**Category:** CLIENT ARCHITECTURE

**Decision:** Tauri 2 is a thin configurable remote-first Product API client. It does not bundle
Python, FFmpeg processing, models/workers, PostgreSQL, Redis or provider/server secrets.

**Decision status:** RESOLVED

**Owner/date:** repository owner, 2026-08-17. Blocks: none.

## Later operational choices

Exact VPS/provider, public domain, production OIDC vendor, production KMS/Vault, cloud S3 vendor,
monitoring SaaS, public ingress and certificate automation are `DEFERRED-NONBLOCKING`. They belong to
the Internet/VPS production profile and do not block Local Functional Acceptance or current coding.

## Gate G implementation disposition — 2026-08-18

The T500–T532 implementation audit is complete on the requested `implementation/bootstrap` baseline.
No architecture question was reopened: typed provider contracts, local deterministic conformance,
real reviewed FFmpeg/ffprobe media proof and post-QA Artifact boundaries follow the approved
decisions above. External provider credentials/model quality, hardened T600–T603 work and public
T605 deployment remain explicitly outside this Gate G acceptance.
