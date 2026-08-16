---
last_verified: 2026-08-16
source: ../../PROJECT_REBUILD_PLAN.md; ../00-PROJECT-CONTEXT.md; ../GLOSSARY.md; ../CODEX-INSTRUCTIONS.md
owner: repository owner / T000 ratifier
---

# Project memory

## Product objective

NH-Media is an AI Video Production Engine. Movie Narrator V1 is the frozen reference and legacy
engine behind a `VideoEngine` adapter. The V2 product owns project state, editable versions,
authorization, durable jobs, artifacts and user-facing workflows. Movie recap is the first
workflow, not the long-term product boundary.

## System boundary

- **Product API** owns identity, Workspace membership/authorization, Project, Asset, Job,
  database transactions, upload orchestration, idempotency and product events.
- **Engine** owns media/AI execution, pipeline nodes, provider ports, timeline proposals and
  rendering. It has no user, session, billing or UI responsibility.
- **Worker controller** owns leases, heartbeats, sandbox launch and guarded execution reports; it
  does not become the durable business-state owner.
- **Media executor** is disposable and least-privilege. It handles untrusted media in an isolated
  workspace and receives only declared inputs/capabilities.
- **Storage** keeps metadata in PostgreSQL, coordination in Redis and large bytes in private
  S3-compatible/MinIO object storage. API/domain/events use Asset/Artifact references, not paths.
- **Web/desktop clients** call Product API only. They never call providers/engine directly and
  never receive broad storage or engine credentials. Desktop is a thin remote-first shell with
  limited native capabilities, not a second business backend.

## Non-negotiable invariants

- Application code is blocked until the documentation gate is explicitly owner-approved.
- `TimelineVersion` is the renderer's sole decision input; AI creates proposals and user overrides
  survive reruns.
- V1 stays behind `VideoEngine`/`LegacyMovieNarratorAdapter`; no wholesale rewrite or namespace
  rename is allowed during migration.
- Job/JobStep states use the canonical vocabulary in [`../GLOSSARY.md`](../GLOSSARY.md).
- Providers, storage, queue and media execution are ports/adapters; provider branches do not leak
  into the pipeline.
- Workers are disposable, checkpoint-aware, retry-aware, idempotent and least-privilege.
- Queue messages contain IDs/references, never media bytes, secrets, user tokens or local paths.
- No `shell=True`, untrusted plugin auto-loading, arbitrary production executable override,
  traceback/path/secret leakage, or large-media proxying through Product API.
- Approved ScriptVersion/TimelineVersion is immutable; edits create a new version with concurrency
  protection.
- Compatibility removal requires usage evidence, replacement parity, a deprecation window, a
  migration guide, rollback and owner approval.

## Source-of-truth order

1. Current user instruction and approved decisions.
2. Root [`PROJECT_REBUILD_PLAN.md`](../../PROJECT_REBUILD_PLAN.md) and its canonical master context.
3. [`../GLOSSARY.md`](../GLOSSARY.md), `docs/00`–`docs/14` and approved ADRs.
4. [`../OPEN-QUESTIONS.md`](../OPEN-QUESTIONS.md) for unresolved choices and hard blocks.
5. [`../IMPLEMENTATION-ORDER.md`](../IMPLEMENTATION-ORDER.md) for task order and DoD.
6. [`../UPSTREAM-MODULE-AUDIT.md`](../UPSTREAM-MODULE-AUDIT.md) and frozen
   [`../../references/movie-narrator`](../../references/movie-narrator) behavior.

Memory is an index and handoff aid. It cannot override a normative specification. Any discrepancy
must be recorded in [`CHANGELOG.md`](CHANGELOG.md), then repaired before dependent implementation.

## Canonical vocabulary

| Term | Meaning |
|---|---|
| `Job` | Product command/aggregate; not a V1 `Task`. |
| `PipelineRun` | Execution of a versioned pipeline graph. |
| `PipelineNode` | Definition in a graph; its runtime execution is a `JobStep`. |
| `Asset` | Product-owned source/registered media resource. |
| `Artifact` | Immutable produced byte/metadata reference with checksum and provenance. |
| `ScriptVersion` | Immutable script content version. |
| `TimelineVersion` | Immutable canonical EDL decision input for rendering. |
| `waiting_for_review` | Persisted human gate, not a failure. |
| `completed` | Canonical successful terminal state; V1 `success` is adapter vocabulary only. |
| `dead_lettered` | Canonical terminal state after exhausted policy; V1 `dead` maps only in compatibility. |
| `local path` | Adapter/executor-sandbox detail only; never an API/domain/event identifier. |

## Things agents must not do

Do not implement `apps/`, `services/`, `packages/`, migrations, runtime/API/frontend/desktop/worker
behavior, or provider integrations before T001–T005/Phase 0 evidence and the task prerequisites
pass. Do not merge `main` implicitly,
push `develop` without owner request, infer OQ recommendations as decisions, expose raw paths,
secrets or tracebacks, or mutate the frozen V1 reference to make a baseline pass.

## Task intake checklist

1. Read this file, `CURRENT-STATE.md`, `DECISIONS.md` and `IMPLEMENTATION-STATUS.md`.
2. Locate the task in [`../IMPLEMENTATION-ORDER.md`](../IMPLEMENTATION-ORDER.md) and verify all
   prerequisite tasks and OQs are approved.
3. Identify the normative docs, boundary owner, compatibility behavior, security controls and
   rollback path before editing.
4. If a decision is open or contradictory, stop the affected implementation and update the root
   OQ with context/options/trade-offs; do not choose a production default silently.
5. Complete the task handoff and update relevant memory/evidence in the same change.
