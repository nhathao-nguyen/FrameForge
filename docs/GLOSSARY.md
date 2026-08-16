# Glossary and Canonical Terminology

This file is normative for V2 naming. V1 terms may appear only in quoted upstream evidence, compatibility mapping or migration discussion.

## Components

| Term | Canonical meaning | Do not conflate with |
|---|---|---|
| Product backend / Product API | FastAPI/application layer owning identity, authorization, Project, commands, persistence and public contracts. | Engine algorithm, FFmpeg worker, frontend. |
| Trusted orchestration | Application services guarding Job/Run/Step transitions, outbox, queue scheduling and commit. | Untrusted media executor. |
| AI/video engine | Provider-neutral pipeline/media/AI/Timeline/render contracts and algorithms. | User, billing, session, HTTP shape. |
| Worker controller | Queue consumer with scoped execution identity; claims leases and launches executor. | PipelineNode definition or browser API. |
| Media executor | Disposable sandbox process/container running one node attempt over untrusted media. | Durable state owner. |
| Frontend | Next.js/React client using Product API/events. | Timeline source of truth or secret holder. |
| Storage | StoragePort implementations for blob bytes. | PostgreSQL metadata or local cache. |

## Domain

| Term | Canonical meaning | Prohibited synonym/ambiguity |
|---|---|---|
| Workflow | Product recipe/capability such as `movie_recap`. | A running Job. |
| Pipeline | One versioned executable DAG for a Workflow. | Fixed Python function chain. |
| PipelineNode | Immutable node definition in Pipeline. | JobStep execution. |
| PipelineRun | One engine execution of exact Pipeline snapshot for a Job. | Job product intent. |
| Job | Product command/aggregate requested by actor/system. | V1 Task except at adapter. |
| JobStep | Runtime PipelineNode instance in PipelineRun. | PipelineNode definition or generic “stage”. |
| Asset | Logical Project-owned input/media resource with upload/validation lifecycle. | Blob/file/path. |
| Artifact | Immutable committed blob manifest with checksum/storage locator/provenance. | Asset, Render request or raw directory. |
| Scene | Detected range in source Asset/Artifact. | Timeline Clip. |
| Match proposal | Scored AI/algorithm proposal linking script region to Scene. | Canonical Timeline Clip. |
| Timeline | Aggregate holding TimelineVersions. | `matches.json`. |
| TimelineVersion | Exact immutable canonical EDL JSON rendered. | Mutable editor state. |
| Track | Ordered Timeline layer embedded in TimelineVersion. | DB table/source file track unless context explicit. |
| Clip | Placement/source decision embedded in a Track. | Scene. |
| SubtitleTrack | Track with `kind=subtitle`; contains cue Clips. | SRT/VTT Artifact. |
| Script | Aggregate holding ScriptVersions. | Exported `script.md` Artifact. |
| ScriptVersion | Exact immutable content used by Job/TTS. | Mutable text buffer. |
| Voice | Provider-neutral catalog descriptor/selection. | Generated narration audio. |
| Narration | One synthesis/recording instance for exact ScriptVersion/Voice snapshot. | Audio Artifact bytes. |
| RenderProfile | Versioned output constraints/policy. | Timeline edit decisions. |
| Render | Product request/result for exact TimelineVersion + RenderProfile. | Output MP4 Artifact. |
| ProviderConfiguration | Product metadata/policy + secret reference selecting provider adapter. | Provider SDK/client or plaintext key. |

## State vocabulary

### Job

Canonical only:

```text
created, queued, running, paused, waiting_for_review, retrying,
cancelling, completed, failed, dead_lettered, cancelled
```

Do not use `pending`, `processing`, `executing`, `waiting_review`, `dead`, `success` or `succeeded` for V2 Job state. These may appear in V1 mapping.

### JobStep / node execution

Canonical only:

```text
pending, ready, queued, running, paused, waiting_for_review,
retrying, completed, skipped, failed, cancelled, blocked
```

Do not use `retry_wait`, `blocked_terminal`, `processing`, `executing`, `success` or `succeeded`.

### Important distinctions

- `paused`: execution intentionally stopped at safe checkpoint/partial boundary.
- `waiting_for_review`: persisted human decision gate with resource/review payload.
- `retrying`: non-terminal backoff/recovery.
- `dead_lettered`: Job terminal after retry budget and DLQ record commit.
- `completed`: canonical successful terminal state; use “success” only as natural-language outcome or V1 `PipelineStatus` quote.

## Storage/location terminology

| Term | Meaning |
|---|---|
| Artifact ID | Domain/API identity. |
| Storage locator | Internal `{backend, object_key, object_version}` metadata; not public identity. |
| Artifact ref | Typed reference by Artifact ID and optional role/checksum/version metadata. |
| Local handle/path | Lease-scoped materialization in worker sandbox; never serialized across components. |
| Presigned URL | Short-lived authorized transport capability; never durable reference. |
| URI | Use only for external/schema identifiers or explicit transport URI, not as universal Artifact identity. |

Avoid generic “storage path” or “file path” in V2 contract. Say `object_key` for internal object storage, `Artifact ID/ref` for domain, or `local handle` inside adapter/executor.

## Pipeline terminology

- Use `node` for Pipeline definition and `job step` for execution.
- Use `stage`/`step` without `JobStep` only when quoting V1’s fixed 16-step pipeline.
- Use `partial execution`, `start_from`, `stop_after`, `checkpoint`, `resume`; do not call arbitrary skipped functions “resume”.
- `checkpoint` is persisted compatible execution state/reference, not worker memory/cache.
- `soft failure` produces `skipped` with reason/consequence only when node contract permits; it is not swallowed exception.

## Output terminology

| Term | Meaning |
|---|---|
| Render | Request/execution aggregate. |
| Render Artifact | Produced immutable video/audio/subtitle/etc. |
| Export | A node/action producing an Artifact in another representation (SRT, clip, Markdown). |
| Deliverable | Render Artifact that passed required profile/QA policy. |
| Preview | Non-deliverable Render allowed from draft/proposed Timeline by explicit policy. |

Do not use `render`, `export` and `deliverable` interchangeably.

## V1 compatibility mappings

| V1 term | V2 interpretation |
|---|---|
| Task | Job + PipelineRun compatibility projection. |
| TaskStatus `pending` | Projected from V2 pre-active/review/pause state according to frozen profile. |
| TaskStatus `dead` | Job `dead_lettered`. |
| PipelineStatus `success` | JobStep `completed`. |
| V1 step | PipelineNode alias / JobStep execution. |
| `Context` path field | Adapter-local materialized Artifact ref. |
| `matches.json` | Match proposal Artifact, never Timeline source of truth. |
| output path | Compatibility download alias backed by Artifact. |
