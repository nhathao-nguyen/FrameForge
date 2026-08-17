# NH-Media — Canonical Project Context and Development Plan

## Owner architecture decision — 2026-08-17

NH-Media is a new, independently designed and independently implemented video/AI media production
system. Movie Narrator (`https://github.com/zcbacxc/movie-narrator.git`) is a research/reference
repository only. It can inform feature discovery, workflows, algorithms, failure modes, acceptance
criteria and comparison fixtures, but it is not part of the NH-Media product.

This decision supersedes every earlier statement that described Movie Narrator as an engine,
legacy runtime, compatibility workload, migration source, rollback target or implementation base.
Historical upstream observations remain useful research evidence. They are not target architecture.

## 1. Product identity and independence

- Product name: **NH-Media**.
- New Python AI/ML namespace: `nh_media`.
- Go module path: `github.com/nhathao-nguyen/NH-Media`.
- `movie_narrator` identifies only the upstream repository/package in research evidence.
- NH-Media does not fork, rename, wrap, embed, import, execute or migrate the Movie Narrator runtime.
- A normal NH-Media build, test, deployment, server, desktop install and worker image must work
  without an upstream checkout.

The only valid relationship is:

```text
Movie Narrator upstream
        │ research and reference only
        ▼
Capability / behavior / algorithm analysis
        ▼
NH-Media specifications and acceptance criteria
        ▼
Independent NH-Media implementation
```

## 2. Product goal

NH-Media is an AI Video Production System, not only a movie-recap command. Movie recap is the first
workflow used to prove the platform. The product supports durable projects, editable versions,
AI-assisted proposals, human review, repeatable rendering and multiple outputs from one timeline.

Core outcomes:

- source media registration and safe ingest;
- metadata and optional external research;
- script generation and versioned human review;
- narration/TTS and alignment/ASR;
- scene segmentation and multimodal analysis;
- match proposals, candidate evaluation and coverage feedback;
- subtitles, translation and bilingual output;
- canonical TimelineVersion editing;
- audio mix, FFmpeg render, QA and clip export;
- durable asynchronous jobs, checkpoints, retry, resume and DLQ;
- web and desktop clients sharing one Product API.

The architecture must remain extensible to general video understanding, OCR, subtitle detection,
tracking, object detection, text/object removal, inpainting, semantic search, multimodal indexing,
automatic reframing, content repurposing and AI-assisted timeline editing. These are later
capabilities unless separately scheduled.

## 3. Target architecture

```text
                         NH-Media

             Web Client          Desktop Client
            Next.js/React            Tauri 2
                  └──────────┬──────────┘
                             │ Product API
                             ▼
                     Go control plane
                ┌────────────┼────────────┐
                ▼            ▼            ▼
           PostgreSQL      Redis      Object Storage
                                             │
                                  versioned worker contracts
                                  ┌──────────┴──────────┐
                                  ▼                     ▼
                            Go media worker      Python ML worker
                              FFmpeg/I/O             nh_media
                                  │                     │
                                  └──────────┬──────────┘
                                             ▼
                              AI / ML / VLM / speech / vision
```

### Product API/control plane

The Go backend owns authentication integration, authorization, Workspace, Project, Asset, Job,
Pipeline metadata, database transactions, upload orchestration, idempotency, scheduling and public
API/events. It does not import Python, execute FFmpeg/ML in HTTP handlers, or know provider SDK
internals.

### Go media worker

The media worker owns media I/O, FFmpeg/ffprobe process orchestration, progress parsing, media
validation, composition and Artifact staging. It executes bounded leases in a disposable sandbox.

### Python ML worker

The Python worker owns model ecosystem workloads under `nh_media`: speech, vision, generation,
matching, evaluation and other ML/AI functions. It has no product identity, billing or session
logic. A dependency-specific Python version may be isolated per worker image.

### Clients

Web and Tauri clients call the same Product API and use shared contracts/SDK. Tauri is a thin,
remote-first client. It does not bundle Python, models, FFmpeg processing, PostgreSQL, Redis,
server-side secrets, ML workers or render workers.

## 4. Repository direction

```text
NH-Media/
├── apps/
│   ├── web/
│   └── desktop/
├── services/
│   ├── api/
│   ├── media-worker/
│   └── ml-worker/
│       └── nh_media/
│           ├── audio/
│           ├── speech/
│           ├── vision/
│           ├── video/
│           ├── generation/
│           ├── matching/
│           ├── subtitles/
│           ├── providers/
│           ├── pipelines/
│           └── evaluation/
├── packages/
│   ├── contracts/
│   └── sdk/
├── infrastructure/
├── docs/
└── tests/
```

The Go service may use `cmd/*` plus domain-oriented `internal/*` packages while remaining within
the conceptual `services/api` and `services/media-worker` boundaries. There is no
`services/legacy-compat`, vendored upstream package or production submodule.

## 5. Domain foundations

NH-Media uses its own language:

- Workspace, Project, Asset and Artifact;
- Workflow, Pipeline, PipelineNode, Job, PipelineRun and JobStep;
- Script, ScriptVersion, Narration and Subtitle;
- Scene, Analysis and ReferenceStyleAnalysis;
- GenerationCandidate, EvaluationResult and CandidateSelectionPolicy;
- Timeline, TimelineVersion, Track and Clip;
- ProviderConfiguration and Model descriptor;
- Render, RenderProfile, Export and QA report.

TimelineVersion is the renderer's source of truth. AI produces proposals. User edits create new
immutable versions and are never silently overwritten by a later AI run.

## 6. Durable execution

```text
Product API command
  → PostgreSQL Job + JobSteps + outbox
  → Redis delivery/coordination
  → worker lease
  → declared node inputs via Asset/Artifact refs
  → checkpointed execution
  → staged and verified output
  → Artifact commit + durable events
  → client snapshot/replay
```

PostgreSQL is authoritative for product and execution state. Redis is coordination, not truth.
Large bytes live in private object storage. Worker local disks are disposable. All boundaries use
versioned language-neutral contracts; no pickle, gob, ORM or framework-internal durable payloads.

## 7. Pipeline direction

The first independent NH-Media pipeline is a versioned DAG with nodes such as:

```text
resolve_source_asset → prepare_media_assets
  ├→ research_metadata → generate_script → review_script
  ├→ detect_scenes → analyze_scenes
  └→ extract_transcript

review_script → generate_narration → align_audio
analysis + script + alignment → generate_match_candidates
generate_match_candidates → evaluate_candidates → select_candidate
selection + narration + subtitles → build_timeline → review_timeline
review_timeline → mix_audio → render_timeline → validate_deliverable → export_clips
```

Candidate generation/evaluation may be deferred from the earliest slice, but the domain and graph
cannot make it impossible. Reference-style analysis extracts abstract production traits—pacing,
shot duration, narration density, subtitle style, framing, transition frequency, music intensity,
rhythm and scene categories—not copyrighted footage.

## 8. Local/LAN first

Functional development is not blocked on a VPS.

```text
LAN server machine
  ├── Product API
  ├── PostgreSQL
  ├── Redis
  ├── object storage
  ├── media workers
  └── ML workers

LAN client machines
  ├── web browser
  └── Tauri desktop client
```

This profile must prove remote client/server separation, durable jobs, worker orchestration,
rendering, recovery, persistence and multiple clients. Internet-facing DNS/TLS/CDN/public ingress
is a later deployment phase. Local/LAN production-like operation still uses authentication,
private service ports and explicit endpoint configuration.

## 9. Upstream knowledge policy

The upstream repository may be inspected to learn behavior, sequencing, algorithms, provider
integrations, failure modes and test ideas. The allowed classification vocabulary is:

- `REIMPLEMENT`: independently implement the capability in NH-Media.
- `ADOPT-CONCEPT`: use an idea or architecture, not upstream source.
- `IMPROVE`: implement an intentionally improved NH-Media-native capability.
- `REFERENCE`: retain information or observed output only as research evidence.
- `DEFER`: valid capability scheduled later.
- `IGNORE`: intentionally outside scope with a reason.

Do not copy code blindly. Do not preserve undocumented quirks as permanent contracts. Optional
behavioral comparison can run in a separate research environment, but NH-Media acceptance is based
on NH-Media contracts and quality criteria. Details are in `docs/UPSTREAM-REFERENCE-POLICY.md` and
`docs/UPSTREAM-CAPABILITY-MATRIX.md`.

## 10. Golden/reference testing

Reference fixtures and observed outputs may be retained when lawful and technically useful:

```text
Input fixture
  ├── recorded upstream reference output (optional research artifact)
  └── NH-Media output
          ▼
  behavior/quality comparison
```

NH-Media tests do not execute upstream in normal CI. A different implementation is valid when its
contract passes, deterministic requirements pass and quality is equivalent or intentionally better.
Intentional divergence is documented.

## 11. Security and provenance

- Untrusted media runs in disposable, non-root, resource-limited sandboxes.
- Workers receive least privilege and only required short-lived secrets.
- Subprocesses use argv lists; never `shell=True`.
- Arbitrary Python entry points are not auto-loaded. Extensions are built-in or reviewed,
  allowlisted and isolated.
- Product API never proxies large media or exposes paths, tracebacks or secrets.
- Upstream URL, research commit and license identification are recorded without asserting legal
  conclusions. Upstream source is not owned by NH-Media and is excluded from runtime/build/release.

## 12. Independent implementation phases

```text
R0  Ratify architecture and upstream-reference policy
R1  Bootstrap NH-Media repository/toolchains/contracts
R2  Establish database, storage, jobs and worker protocol
R3  Prove first independent vertical slice
R4  Add native workflow/domain/editor foundations
R5  Reimplement capabilities incrementally
R6  Add intelligence, candidate evaluation and advanced analysis
R7  Harden Local/LAN operations, then public production deployment
```

For each reference-informed capability:

```text
research → NH-Media behavior spec → NH-Media interface
→ independent implementation → unit/integration tests
→ optional upstream comparison → NH-Media acceptance
```

The first vertical slice is:

```text
client request → Go Product API → persistent Job → queue/lease
→ independent media worker action → Artifact storage
→ durable completion/event → client result
```

It must not require Movie Narrator.

## 13. Non-goals for the initial implementation

- Public plugin marketplace or unrestricted user code.
- Kubernetes, premature microservices or automatic internet scaling.
- Billing/subscriptions and enterprise organization hierarchy.
- All future vision/editing capabilities in the first release.
- Compatibility with Movie Narrator CLI, REST, config files, Python imports or output paths.
- Shipping the upstream repository, its runtime or a rollback image.

## 14. Specification and implementation gate

The current repository phase is documentation/specification. This refactor may edit Markdown,
diagrams, matrices, task plans, reference policy, audit evidence and documentation validation tools.
It must not add Product API, workers, clients, database migrations or media/AI implementation.

Application implementation begins only after the owner accepts the independent specification and
the next task's explicit prerequisites. Open implementation/deployment choices block only their
dependent tasks, not unrelated documentation cleanup.

## 15. Final architecture assertions

```text
Independent implementation: YES
Movie Narrator runtime dependency: NO
Movie Narrator build dependency: NO
Movie Narrator deployment dependency: NO
Movie Narrator import dependency: NO
Legacy compatibility service: NO
Product: NH-Media
Python namespace: nh_media
Local/LAN validation: SUPPORTED
```
