---
last_verified: 2026-08-16
source: ../00-PROJECT-CONTEXT.md; ../01-ARCHITECTURE.md; ../05-PIPELINE-SPEC.md; ../12-STORAGE-ARCHITECTURE.md; ../13-WORKER-ARCHITECTURE.md
owner: repository owner / architecture ratifier
---

# Architecture map

## Runtime topology

```text
User
  ├─ Browser → apps/web (Next.js/React) ──HTTPS──┐
  └─ Desktop → apps/desktop (thin native shell) ─┴──> apps/api (FastAPI Product API)
                                                    │ Product auth/session and commands
                                                    ▼
                                            Product API server
  ├── PostgreSQL: identity, ownership, versions, jobs, events, audit
  ├── Redis: queue, lease and event coordination
  ├── private S3/MinIO: source media and immutable artifacts
  └── VideoEngine port
        ├── LegacyMovieNarratorAdapter → frozen movie_narrator V1
        └── V2 Pipeline Runtime → nodes/providers/timeline/compiler
                  │
                  ▼
          Engine Worker controller
                  │
                  ▼
          disposable media executor sandbox
```

## Upload → job → render flow

```text
Project
  → upload session
  → browser multipart upload to private object storage
  → complete upload + checksum/MIME/magic/probe/quarantine
  → committed Asset/Artifact reference
  → Product API creates Job/PipelineRun/JobSteps + outbox
  → queue message contains IDs/references only
  → controller claims lease and starts sandbox attempt
  → node checkpoint/proposal/artifact/event commits through guarded state boundary
  → ScriptVersion/TimelineVersion review and user overrides
  → compiler reads exact TimelineVersion + RenderProfile
  → Render and immutable output Artifacts
```

## Ownership and dependency direction

```text
web → Product API → domain/application ports
                  ├→ repository ports → PostgreSQL
                  ├→ StoragePort → S3/MinIO/LocalStorage
                  ├→ QueuePort → Redis
                  └→ VideoEngine → engine/provider/media ports
worker controller → ExecutionStatePort + VideoEngine
media executor → declared node contract + scoped artifact/provider access
legacy adapter → stable movie_narrator V1 contract only
```

The core domain must not import FastAPI HTTP models, Redis clients, boto3 or UI code. The engine
must not own identity, authorization, billing or sessions. Web and desktop clients must not call
engine or provider endpoints directly. Desktop is remote-first and does not own durable product
state.

## Durable object ownership

| Object | Owner | Boundary rule |
|---|---|---|
| User/Workspace/membership | Product API | authorization scope is resolved before repository access |
| Project/Asset/Job | Product API | mutable metadata has revision and audit |
| Pipeline/PipelineNode | Product/engine contract | definition is separate from runtime JobStep |
| PipelineRun/JobStep/Attempt | Product orchestration | transitions are guarded and durable |
| ScriptVersion/TimelineVersion | Product domain | immutable after creation; edits create a version |
| Artifact/Checkpoint | Product/storage boundary | checksum/provenance/reference, no raw path in API |
| Provider call/node output | Engine | provider-specific behavior stays behind ports/adapters |
| Render | Product command + engine execution | renderer reads TimelineVersion, not match proposals |
| Web/desktop session and local draft | Client shell | session is scoped; local draft is disposable, server is source of truth |

## Trust and secret boundaries

- Public API accepts metadata and presigned upload orchestration; large media bytes bypass it.
- Quarantine/probe runs on untrusted bytes without Product DB/Redis or unrelated provider secrets.
- Controller has scoped execution-state/queue identity; executor has only its declared capability.
- Render workers do not receive LLM/TTS credentials unless a node explicitly requires them.
- Secrets are references from a secret backend, never plaintext in config, events, checkpoints or
  memory.

## Source links

See [`../01-ARCHITECTURE.md`](../01-ARCHITECTURE.md), [`../05-PIPELINE-SPEC.md`](../05-PIPELINE-SPEC.md),
[`../12-STORAGE-ARCHITECTURE.md`](../12-STORAGE-ARCHITECTURE.md) and
[`../13-WORKER-ARCHITECTURE.md`](../13-WORKER-ARCHITECTURE.md) for normative contracts.
