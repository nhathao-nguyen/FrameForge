# Glossary and Canonical Terminology

This file is normative for NH-Media naming. Upstream terms appear only in explicit research or
provenance context.

## Components

| Term | Canonical meaning | Do not conflate with |
|---|---|---|
| NH-Media | Independent product and source tree. | Movie Narrator or any upstream runtime. |
| Product API | Go control plane owning identity, authorization, Project, commands, persistence and public contracts. | Media/ML execution or frontend. |
| Trusted orchestration | Application services guarding Job/Run/Step transitions, outbox, scheduling and commit. | Executor sandbox. |
| Media worker | Go worker for media I/O, FFmpeg process orchestration, progress and Artifact staging. | Product API handler or codec implementation. |
| ML worker | Isolated Python `nh_media` worker for model/AI workloads. | Product identity/control plane. |
| Worker controller | Queue consumer claiming leases and launching an executor. | PipelineNode or browser API. |
| Executor | Disposable sandbox running one node attempt over untrusted inputs. | Durable state owner. |
| Frontend | Next.js web or Tauri 2 desktop client using Product API. | Timeline truth or secret holder. |
| Language-neutral contract | Versioned JSON/Protobuf/schema shared across Go/Python/clients. | Go struct, Python class, ORM object, pickle or gob. |
| Upstream reference | External research source used for capability/behavior observations. | Product dependency, implementation base or compatibility target. |

## Domain

| Term | Canonical meaning | Do not conflate with |
|---|---|---|
| Workspace | Authorization/ownership scope containing Projects. | Deployment host. |
| Project | Aggregate for inputs, versions, jobs and outputs. | Local output directory. |
| Workflow | Product recipe/capability such as `movie_recap`. | Running Job. |
| Pipeline | One versioned executable DAG for a Workflow. | Fixed source function chain. |
| PipelineNode | Immutable node definition. | JobStep execution. |
| PipelineRun | Execution of an exact Pipeline snapshot for a Job. | Job intent. |
| Job | Product command/aggregate. | Upstream Task. |
| JobStep | Runtime PipelineNode instance. | PipelineNode definition or generic stage. |
| Asset | Logical Project-owned input/media with validation lifecycle. | Blob, file path or Artifact. |
| Artifact | Immutable blob manifest with checksum, storage locator and provenance. | Asset or Render request. |
| Analysis | Versioned typed result over exact inputs and model/tool provenance. | Unstructured log. |
| ReferenceStyleAnalysis | Abstract style metrics from an authorized reference asset. | Copied video or imitation runtime. |
| GenerationCandidate | One immutable alternative produced in a candidate group. | Selected canonical output. |
| EvaluationResult | Scoring/evidence for a candidate under an exact policy/evaluator snapshot. | Candidate payload mutation. |
| CandidateSelectionPolicy | Versioned rules for candidate selection. | Provider fallback policy. |
| Scene | Detected range in a source Asset/Artifact. | Timeline Clip. |
| MatchProposal | Scored proposal linking content to source ranges. | Canonical Timeline decision. |
| TimelineVersion | Exact immutable canonical EDL rendered. | Mutable editor buffer or match file. |
| Track | Ordered layer embedded in TimelineVersion. | DB table unless explicitly a projection. |
| Clip | Placement/source decision in a Track. | Scene. |
| ScriptVersion | Exact immutable script content. | Mutable editor text. |
| Narration | One synthesis/recording for exact ScriptVersion/Voice snapshot. | Audio bytes. |
| RenderProfile | Versioned output constraints and policy. | Timeline edit decisions. |
| Render | Product request/result for exact TimelineVersion + RenderProfile. | Output MP4 Artifact. |
| ProviderConfiguration | Redacted product policy plus secret reference selecting an adapter. | Provider SDK or plaintext key. |

## State vocabulary

### Job

```text
created, queued, running, paused, waiting_for_review, retrying,
cancelling, completed, failed, dead_lettered, cancelled
```

Do not use `pending`, `processing`, `executing`, `waiting_review`, `dead`, `success` or `succeeded`
as Job states.

### JobStep

```text
pending, ready, queued, running, paused, waiting_for_review,
retrying, completed, skipped, failed, cancelled, blocked
```

Do not use `retry_wait`, `processing`, `executing`, `success` or `succeeded`.

`paused` is an intentional checkpoint boundary; `waiting_for_review` is a persisted human gate;
`retrying` is non-terminal backoff; `dead_lettered` is terminal after exhausted policy.

## Storage/location

| Term | Meaning |
|---|---|
| Artifact ID/ref | Domain/API identity and typed reference. |
| Storage locator | Internal backend/key/version metadata, never public identity. |
| LocalHandle | Lease-scoped executor materialization; never serialized across components. |
| Presigned URL | Short-lived authorized transport capability, never a durable ref. |

## Pipeline/output

- `node` means definition; `job step` means execution.
- `checkpoint` is persisted compatible state/references, not worker memory/cache.
- `soft failure` may produce `skipped` only when the node contract declares consequence.
- `Render` is the request/result aggregate; `Render Artifact` is produced media.
- `Export` creates another representation; `Deliverable` passed required QA; `Preview` is explicit
  non-deliverable output.

## Upstream terminology policy

`Movie Narrator`, `movie_narrator`, its CLI/REST routes, task statuses and filenames may be quoted in
the upstream policy, audits, capability matrix or provenance baseline. They have no mapping into
NH-Media runtime contracts and are not accepted as aliases.
