# 01 — Architecture Specification

## 1. Architectural shape

NH-Media begins as a modular monorepo and a small number of deployable processes. Logical
boundaries are explicit so worker classes can scale independently later.

```text
apps/web ─────HTTPS─────┐
apps/desktop ─HTTPS─────┴──> Go Product API ──SQL──> PostgreSQL
                                  │
                                  ├── Redis: queue/lease/live fan-out
                                  ├── object storage: uploads/Artifacts
                                  └── trusted orchestration
                                         │ versioned worker contracts
                         ┌───────────────┴───────────────┐
                         ▼                               ▼
                  Go media worker                Python ML worker
                  FFmpeg + media I/O              nh_media + models
                         └───────────────┬───────────────┘
                                         ▼
                              disposable executor sandbox
```

Movie Narrator has no runtime position in this diagram.

## 2. Ownership

| Boundary | Owns | Must not own |
|---|---|---|
| Go Product API | auth integration, authorization, Workspace, Project, Asset, Job, DB transaction, upload, public API/events | FFmpeg, model runtime, provider-specific prompts, Python imports |
| Trusted orchestration | state transitions, outbox, scheduling, lease validation, result commit | arbitrary media execution or product authorization bypass |
| Go media worker | media input/output, ffprobe/FFmpeg orchestration, progress, validation, staging | product identity/session, broad DB mutation, ML model ecosystem |
| Python ML worker | `nh_media` AI/ML nodes and provider adapters requiring Python/model runtimes | HTTP product models, user/billing/session, durable business truth |
| Executor sandbox | one declared attempt with scoped inputs/capability | broad storage, DB/Redis credentials, host/workspace access |
| Frontend | view/edit commands, upload, progress and downloads via Product API | provider/worker calls, server secrets, durable product truth |

## 3. Dependency direction

```text
web/desktop → packages/sdk + contracts → Product API → domain/application ports
                                             ├→ PostgreSQL adapter
                                             ├→ Redis adapter
                                             ├→ storage adapter
                                             └→ WorkerDispatchPort

Go media worker → language-neutral worker contract → media/FFmpeg adapters
Python ML worker → language-neutral worker contract → nh_media providers/pipelines
```

Core domain does not import transport structs, Python/Pydantic classes, ORM objects, Redis clients,
S3 SDK objects or UI code. Cross-language boundaries use versioned JSON/Protobuf/schema envelopes.

## 4. Repository layout

```text
apps/
  web/                         # Next.js/React
  desktop/                     # Tauri 2 thin remote client
services/
  api/                         # Go module/service boundary
  media-worker/                # Go FFmpeg/media worker
  ml-worker/
    nh_media/                  # Python AI/ML namespace
packages/
  contracts/                   # language-neutral schemas
  sdk/                         # shared client SDK
infrastructure/
  compose/
  postgres/
  redis/
  object-storage/
  reverse-proxy/
tests/
  unit/
  integration/
  contract/
  reference-behavior/
  e2e/
docs/
tools/
```

Go may organize entry points under `services/api/cmd` and packages under `services/api/internal`,
or use root `cmd`/`internal` if the bootstrap task proves that layout clearer. Either choice must
keep the conceptual service boundaries and module path `github.com/nhathao-nguyen/NH-Media`.

Prohibited production trees include `legacy-compat`, vendored `movie_narrator`, an upstream
submodule, or a container whose purpose is to execute upstream.

## 5. Runtime flows

### Upload

```text
client requests upload session
→ Product API authorizes and presigns
→ client uploads directly to object storage
→ Product API completes session and creates validation Job
→ media worker validates/probes
→ original Artifact committed; Asset becomes ready
```

Large video bytes never pass through Product API or Redis.

### Job

```text
API transaction: Job + JobSteps + outbox
→ queue publish
→ worker claims bounded lease
→ executor materializes declared Artifact refs
→ independent NH-Media node executes
→ output stage/verify/commit
→ checkpoint + event + state transition
```

### Studio edit

Clients create new ScriptVersion/TimelineVersion using optimistic concurrency. A review resolution
selects the exact version for downstream work. Browser/app closure does not affect persisted state.

### Multi-output

Renders reference one TimelineVersion plus different RenderProfiles. Profile-only changes rerun only
profile-dependent media work.

## 6. Deployment profiles

### Local/LAN production-like

One LAN server may run API, PostgreSQL, Redis, object storage and bounded workers. Web and Tauri
clients may run on other LAN machines. This is the first required end-to-end profile and does not
require a VPS, public DNS or public certificates.

### Internet-facing production

Later deployment adds public TLS/reverse proxy, DNS/CDN as needed, managed/private dependencies,
signed desktop updates, canary and public threat controls. Only Product API/web ingress is public.

### Scale-out

Worker queues may later route by capability (`probe`, `ai`, `ml`, `media`, `render`). Pipeline,
JobStep and event contracts remain identical. Distributed rendering is enabled by durable leases,
Artifact refs and capability routing; it is not an initial extra service requirement.

## 7. Reliability

- PostgreSQL is authoritative; Redis messages and worker memory are reconstructable.
- At-least-once delivery plus idempotent JobStep/Artifact commit.
- Lease expiry and heartbeat reconciliation.
- Checkpoints after terminal node outcomes and at declared chunk boundaries.
- Retry only classified transient errors; fail closed on validation/security/user errors.
- Cooperative cancellation then bounded process-tree termination.
- Snapshot + ordered event replay for client reconnect.
- Bounded concurrency and resource admission for every worker.

## 8. Extension/provider architecture

Providers, storage, queue and media subprocesses are ports. Built-in or reviewed adapters are
registered explicitly. No arbitrary Python entry-point auto-loading exists. Future extensions use
versioned capabilities, allowlists and isolation; they never gain implicit product secrets.

## 9. Observability

Structured logs, metrics and traces share request/correlation/project/job/run/step IDs. Track queue
age, lease loss, retries, provider latency/cost, media process duration, Artifact bytes and render
QA. Logs/events expose no secret, expiring URL, durable local path or traceback.

## 10. Upstream reference isolation

Upstream source may be fetched into a disposable research workspace for an explicitly authorized
comparison. It is excluded from normal checkout requirements, Go/Python dependencies, CI, images,
desktop packages and deployment manifests. Recorded behavior informs NH-Media-owned specs and
fixtures only. See `UPSTREAM-REFERENCE-POLICY.md`.

## 11. Non-goals

- Runtime/API/CLI compatibility with Movie Narrator.
- A legacy engine adapter, service, image or rollback route.
- Bundling server compute into Tauri.
- Kubernetes or many small services before measured need.
- Public arbitrary-code plugins.
- Provider/model lock-in.
