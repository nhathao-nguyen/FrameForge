# Specification Consistency Matrix

## 1. Architecture invariants

| Decision | Normative locations | Result |
|---|---|---|
| NH-Media is an independent product | `00`, `01`, `09`, reference policy | Consistent; no upstream product identity or public contract is inherited. |
| Movie Narrator is research/reference only | reference policy, capability matrix, module audit | Consistent; normal build, test, runtime and deployment do not require it. |
| Product API/control plane is Go | `01`, `04`, `13`, implementation order | Consistent; no Python, FFmpeg or ML runtime is linked into Product API. |
| Media orchestration is Go; ML/AI is isolated Python `nh_media` | `01`, `05`, `13`, `14` | Consistent; versioned language-neutral contracts cross the boundary. |
| Long work is bounded and asynchronous | `04`, `07`, `13` | Consistent; Product API persists commands/outbox and workers claim bounded work. |
| TimelineVersion is renderer source of truth | `02`, `05`, `06`, T420/T530 | Consistent; AI outputs proposals and user overrides survive reruns. |
| Bytes and paths stay behind storage/executor boundaries | `02`, `03`, `08`, `12`, `13` | Consistent; contracts carry Asset/Artifact refs. |
| Provider/storage/queue/media operations use ports | `05`, `11`, `12`, `13` | Consistent; provider-specific branches do not leak into orchestration. |
| Candidate race and reference style are native concepts | `02`, `03`, `04`, `05`, matrix, T525/T526 | Consistent; provenance and selection are explicit. |
| Web and Tauri are thin Product API clients | `00`, `01`, `10`, setup/production plans | Consistent; neither client embeds the engine or server secrets. |
| Local/LAN is a valid release milestone | `00`, `10`, `14`, setup/production plans, T603 | Consistent; VPS/public internet is a later release choice. |
| Auth/authorization are concrete | `02`–`04`, `08`, OQ-01/OQ-06 | LocalAuthProvider plus Workspace-first bootstrap; future OIDC is nonblocking. |
| Secrets are concrete | `03`, `08`, `11`, OQ-05 | Server-owned master key encrypts SecretStore records; future Vault/KMS is an adapter. |
| Queue/progress are concrete | `01`, `04`, `07`, `13`, OQ-03/OQ-04 | Redis Streams for execution; SSE for progress; PostgreSQL/REST remain authoritative. |
| Timeline storage/editing are concrete | `03`, `04`, `06`, OQ-02/OQ-08 | Versioned JSONB plus typed domain commands and optimistic concurrency. |

## 2. Entity-to-contract mapping

| Domain concept | Persistence | API/event surface |
|---|---|---|
| Project | `projects` | `/projects`, scoped events |
| Asset/Artifact | `assets`, upload and artifact tables | upload sessions, artifact lifecycle events |
| Workflow/Pipeline/PipelineNode | workflow and graph tables | read catalog, immutable Job snapshot |
| Job/PipelineRun/JobStep | durable execution tables | commands, replayable job/node events |
| ReviewRequest/Resolution | reviews | review commands/events |
| Script/ScriptVersion | script tables | version and review resources |
| Scene/Character | analysis-owned tables | scene/annotation resources |
| Analysis/ReferenceStyleAnalysis | `analyses` plus result Artifacts | analysis resources and provenance |
| GenerationCandidate/EvaluationResult | candidate/evaluation tables | candidate list, evaluation and selection resources |
| Timeline/TimelineVersion | timeline aggregate plus canonical JSON | version/validate/approve/lock resources |
| Narration | narration metadata plus Artifacts | narration resource and provider provenance |
| RenderProfile/Render | profile/render tables | render commands/events and deliverable Artifacts |
| ProviderConfiguration | redacted configuration plus external secret ref | authorized admin lifecycle; no secret value returned |

Track, Clip and SubtitleTrack remain canonical TimelineVersion document members. Provider catalog
voices are snapshots rather than independent product aggregates.

## 3. Canonical execution states

Job states are `created`, `queued`, `running`, `paused`, `waiting_for_review`, `retrying`,
`cancelling`, `completed`, `failed`, `dead_lettered`, `cancelled`.

JobStep states are `pending`, `ready`, `queued`, `running`, `paused`, `waiting_for_review`,
`retrying`, `completed`, `skipped`, `failed`, `cancelled`, `blocked`.

The database, REST resources and event catalogue use these exact terms. There is no upstream state
translation layer.

## 4. Scenario traces

### A — Native upload to rendered output

```text
Project → direct staged upload → validation/probe → Asset ready
→ native Job/PipelineRun/JobSteps → independently authored analysis nodes
→ proposal/review policy → TimelineVersion → Go render worker → QA Artifact
```

### B — Edit Script and resume

A review node proposes ScriptVersion `sv1`; the user creates `sv2` and approves the review with
`sv2` as selected output. The immutable Job command remains unchanged. Only descendant fingerprints
are invalidated.

### C — Change one Clip and rerender

The user creates TimelineVersion `tv2` from `tv1`. Rendering references `tv2`; the renderer cannot
silently rematch or overwrite the user Clip.

### D — 16:9, 9:16 and 1:1

Three Render requests reuse one TimelineVersion and reusable preceding pipeline Artifacts. Profile-specific
reframe/mix/render/QA work differs; analysis is not rerun without a changed fingerprint.

### E — Worker death

After lease expiry, reconciliation verifies committed checkpoint/Artifact evidence and retries the
first incomplete compatible node. Redis and executor-local disk are not durable truth.

### F — Provider failure and candidate race

The adapter maps a safe error, bounded retry/circuit/fallback policy runs, and provenance is
recorded. Candidate evaluation never hides missing results; the selected candidate and policy are
persisted before Timeline compilation.

### G — Malicious media

Staged bytes are probed in a disposable least-privilege executor. Resource, path, egress and process
limits apply; failures quarantine the input and expose no traceback, secret or durable path.

### H — ReferenceStyleAnalysis

A reference Asset produces abstract style traits and evidence Artifacts. The workflow does not copy
reference footage or import upstream data structures; users can review/override the proposal.

### I — Local/LAN certification

One pinned local stack runs Go Product API, Go media worker, isolated Python `nh_media` worker,
PostgreSQL, Redis, private object storage, Next.js and Tauri clients. T603 proves functional and
security behavior before any VPS/public-network decision.

### J — Upstream research refresh

T604 records a new upstream identity and reclassifies capability evidence. It cannot change an
NH-Media public contract, add a source dependency or enter normal CI/build/deploy without a new
owner-approved architecture decision.

### K — Local Functional Acceptance

The first milestone starts all server/data/worker/client components, proves LocalAuth and default
Workspace bootstrap, executes deterministic Go media and minimal Python worker paths, persists an
Artifact, streams SSE progress, recovers from restart and accepts one explicitly configured LAN
client. It requires neither VPS nor public TLS.

## 5. Result

The specifications use one independent product identity, one ownership topology, one state
vocabulary, one Artifact boundary and one implementation order. `OPEN-QUESTIONS.md` is now a closed
decision register: open architectural questions are zero; later vendor/optimization choices are
explicitly deferred nonblocking.
