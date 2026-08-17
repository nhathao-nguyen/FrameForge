---
last_verified: 2026-08-17
source: ../../PROJECT_REBUILD_PLAN.md; ../00-PROJECT-CONTEXT.md; ../GLOSSARY.md; ../CODEX-INSTRUCTIONS.md
owner: repository owner / task assignee
---

# Project memory

## Product objective

NH-Media is an independent AI video production product. Movie recap is the first workflow, not the
product boundary. Movie Narrator is external research/reference material only.

## System boundary

- Go Product API owns identity, authorization, Project, Asset, Job, database transactions,
  idempotency, uploads and product events. It does not import Python, FFmpeg or ML runtimes.
- Bounded Go media workers own FFmpeg/media orchestration.
- Isolated Python workers use the `nh_media` namespace for ML/AI capabilities.
- PostgreSQL owns durable product state, Redis is delivery/coordination, and private object storage
  owns large bytes. Contracts use Asset/Artifact refs.
- Next.js/React web and Tauri 2 desktop are thin Product API clients.
- Local/LAN is a valid release target; public internet/VPS is a later deployment phase.

## Non-negotiable invariants

- Do not add application code before T004 records approval of the documentation gate.
- No Movie Narrator runtime, build, deployment, import, compatibility, migration or rollback
  dependency is allowed by the current architecture.
- `TimelineVersion` is the renderer's sole decision input; AI creates proposals and user overrides
  survive reruns.
- Candidate generation, evaluation and selection are persisted and provenance-aware.
- `ReferenceStyleAnalysis` produces abstract traits/evidence; it does not copy reference footage.
- Workers are disposable, checkpoint/retry/idempotency aware and least-privilege.
- Product API enqueues bounded asynchronous work and never runs media/ML inline.
- Go/Python boundaries use versioned language-neutral contracts; no pickle, gob, ORM or framework
  internals are durable/public payloads.
- Providers, storage, queue and media processes use ports/adapters.
- No `shell=True`, arbitrary executable/flag injection, unrestricted plugin auto-loading, secret/
  traceback/path leakage or large-media proxying through Product API.

## Source-of-truth order

1. Current owner instruction and dated approved decisions.
2. Root [`PROJECT_REBUILD_PLAN.md`](../../PROJECT_REBUILD_PLAN.md) and canonical master context.
3. [`../GLOSSARY.md`](../GLOSSARY.md) and `docs/00`–`docs/14`.
4. [`../OPEN-QUESTIONS.md`](../OPEN-QUESTIONS.md).
5. [`../IMPLEMENTATION-ORDER.md`](../IMPLEMENTATION-ORDER.md).
6. [`../UPSTREAM-REFERENCE-POLICY.md`](../UPSTREAM-REFERENCE-POLICY.md), capability matrix and
   research audit for non-normative upstream evidence.

## Canonical vocabulary

| Term | Meaning |
|---|---|
| `Job` | Product command/aggregate. |
| `PipelineRun` | Execution of an immutable pipeline graph snapshot. |
| `PipelineNode` | Graph definition; runtime execution is a JobStep. |
| `Asset` | Product-owned input/media resource. |
| `Artifact` | Immutable produced byte/metadata reference with checksum and provenance. |
| `Analysis` | Versioned analytical result and evidence set. |
| `ReferenceStyleAnalysis` | Abstract style-trait analysis of a user-provided reference Asset. |
| `GenerationCandidate` | One alternative result produced by a candidate-generating node. |
| `TimelineVersion` | Immutable canonical EDL decision input for rendering. |
| `waiting_for_review` | Persisted human gate, not a failure. |
| `completed` | Canonical successful terminal state. |
| `dead_lettered` | Canonical terminal state after exhausted policy. |
| `local path` | Executor/storage-adapter detail only. |

## Task intake

Read the four required memory files, locate the task and dependencies, check relevant OQs, identify
the owning service/trust boundary, and update evidence/memory in the same change. Stop affected
implementation when an owner decision is genuinely missing.
