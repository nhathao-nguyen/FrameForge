# Specification Consistency Matrix

## 1. Master-plan invariants

| Master decision | Specification location | Audit result |
|---|---|---|
| V1 is legacy/reference; no immediate rewrite | `00`, `01`, `09`, module audit | Preserved; Strangler/rollback/module gates explicit. |
| VideoEngine/worker abstraction; Product control plane not implementation-dependent | `00`, `01 §6`, `05 §11`, implementation T320–T323 | Preserved; Go/Python implementations use language-neutral contracts. |
| Engine has no user/billing/subscription logic | `00`, `01 §2`, AGENTS | Preserved. |
| Pipeline nodes + checkpoint/resume/partial | `02 §5`, `05`, `07`, worker spec | Preserved with generic start/stop/review semantics. |
| Timeline is renderer source of truth | `02`, `05 §10`, `06`, implementation T420/T530 | Preserved; matches are proposal only. |
| AI proposal, human override | `02 §7`, `06 §5`, API review/version endpoints | Preserved. |
| No raw path across system; storage abstraction | `02`, `03`, `12`, adapter mapping | Preserved; local handles sandbox-only. |
| Provider abstraction | `05 §8`, `11` | Preserved for LLM/VLM/TTS/ASR/Embedding. |
| Stateless workers and progress/events | `07`, `13` | Preserved; controller/executor trust split clarified. |
| Automatic and studio modes | `00`, `05 node review policies`, `10` | Preserved with same graph/data-driven review policy. |
| Movie recap is one workflow | Workflow entity + `movie_recap_v2` built-in definition | Preserved. |
| Multi-output avoids upstream rerun | `02` invariants, `06 §6`, T532 | Preserved through Timeline/Profile/input fingerprints. |
| Public API vs media worker security | `08`, `13` | Preserved and expanded to three trust boundaries. |
| Bounded Go media + isolated Python ML/V1 workers | `01 §5`, `13 §1` | Product API is Go; worker capability boundaries are explicit and bounded. |
| No mandatory Kubernetes Phase 1 | `01`, `10` non-goals | Preserved. |

## 2. Entity-to-persistence/API mapping

| Domain object | PostgreSQL | REST/read-command surface | Event/provenance |
|---|---|---|---|
| Project | `projects` | `/projects` CRUD | Job/resource audit events |
| Asset | `assets`, `asset_uploads`, `artifact_variants` | Asset/upload session endpoints | `artifact.*`, validation Job events |
| Artifact | `artifacts`, relation tables | project/job Artifact list + download-url | `artifact.created|committed|quarantined` |
| Workflow | `workflows` | read catalog | Job snapshot/provenance |
| Pipeline | `pipelines` | read catalog | `job.created`, `pipeline_run.created` snapshot |
| PipelineNode | `pipeline_nodes`, dependencies | Pipeline graph read | node events carry `pipeline_node_id/node_key` |
| Job | `jobs` | Job CRUD commands/read | `job.*` |
| PipelineRun | `pipeline_runs` | Job run history/read | `pipeline_run.*` |
| JobStep | `job_steps`, attempts | Job steps/attempts read + review commands | `node.*`, `review.*` |
| ReviewRequest/Resolution | `reviews` | Job review approve/reject commands | `review.required|approved|rejected` |
| Script | `scripts` | Script aggregate endpoints | review/resource audit |
| ScriptVersion | `script_versions`, reviews | version/create/approve/reject | review events + JobStep outputs |
| Scene | `scenes` | Scene list/read/annotation | producer JobStep/Artifact provenance |
| Character | `characters`, appearances | Character/merge/appearance endpoints | producer/audit provenance |
| Timeline | `timelines` | Timeline aggregate endpoints | review/resource audit |
| TimelineVersion | `timeline_versions.document` | version/validate/approve/lock | review + Render snapshot |
| Track/Clip/SubtitleTrack | embedded canonical JSON | through TimelineVersion | Timeline content hash/origin/proposal refs |
| Voice | provider catalog, Narration snapshot | provider catalog/Narration request | ProviderRun/Narration snapshot |
| Narration | `narrations` + Artifacts | Narration list/read/create Job | producer/provider provenance |
| RenderProfile | `render_profiles` | read catalog/admin tooling | Render profile snapshot |
| Render | `renders`, `render_artifacts` | Render create/read/cancel/retry | `render.*` + Job events |
| ProviderConfiguration | `provider_configurations` | redacted admin lifecycle | `provider_runs`/audit; secret external |

No required persistent entity is missing. Track/Clip/SubtitleTrack and Voice catalog intentionally do not need canonical standalone tables; reasons are in `03 §10`.

## 3. Job state alignment

| State | DB `jobs.status` | REST `Job.status` | Primary event | Terminal |
|---|---:|---:|---|---:|
| `created` | exact | exact | `job.created` | no |
| `queued` | exact | exact | `job.queued` | no |
| `running` | exact | exact | `job.started` | no |
| `paused` | exact | exact | `job.paused` | no |
| `waiting_for_review` | exact | exact | `job.waiting_for_review`, `review.required` | no |
| `retrying` | exact | exact | `job.retry_scheduled` | no |
| `cancelling` | exact | exact | `job.cancellation_requested` | no |
| `completed` | exact | exact | `job.completed` | yes |
| `failed` | exact | exact | `job.failed` | yes |
| `dead_lettered` | exact | exact | `job.dead_lettered` | yes |
| `cancelled` | exact | exact | `job.cancelled` | yes |

## 4. Node/JobStep state alignment

| State | DB `job_steps.status` | REST `JobStep.status` | Primary event | Terminal |
|---|---:|---:|---|---:|
| `pending` | exact | exact | represented in initial snapshot | no |
| `ready` | exact | exact | `node.ready` | no |
| `queued` | exact | exact | `node.queued` | no |
| `running` | exact | exact | `node.started`, `node.progress` | no |
| `paused` | exact | exact | `node.paused` | no |
| `waiting_for_review` | exact | exact | `review.required` | no |
| `retrying` | exact | exact | `node.retry_scheduled` | no |
| `completed` | exact | exact | `node.completed` | yes |
| `skipped` | exact | exact | `node.skipped` | yes |
| `failed` | exact | exact | `node.failed` | yes |
| `cancelled` | exact | exact | `node.cancelled` | yes |
| `blocked` | exact | exact | `node.blocked` | yes |

## 5. Review-time version substitution

Job command and original input snapshot remain immutable. A review node can legally select an edited replacement resource through a persisted ReviewResolution:

```text
generate_script → proposed ScriptVersion sv1
review_script waiting_for_review
user creates sv2 based on sv1
approve review with selected_resource_id=sv2
review_script completes with output_ref=sv2
checkpoint records sv2 and downstream fingerprints are computed from sv2
```

This is a node output resolution, not mutation of Job command. Only descendants are invalidated. Equivalent rule applies to Timeline/scene-match review. API/event payload must record proposed and selected resource IDs/revisions plus actor.

## 6. Scenario traces

### Scenario A — Upload movie and automatically generate recap

```text
POST Project
→ POST upload-session
→ Browser multipart to object storage
→ complete
→ Asset uploaded/validating + probe Job
→ committed original Artifact + Asset ready
→ POST Job(mode=automatic, movie_recap Pipeline)
→ Job/Run/Steps/outbox
→ Redis → bounded Go/Python worker contract → node checkpoints/Artifacts
→ match proposal → build_timeline → automatic approval policy
→ exact TimelineVersion → render_timeline → QA
→ Render/Artifact committed → Job completed
→ REST/SSE exposes result/download
```

Answer is complete across API (`04`), DB (`03`), pipeline (`05`), events (`07`), storage (`12`) and worker (`13`).

### Scenario B — Generate Script, pause, edit, resume

Studio Pipeline reaches `review_script`; Job/Run/Step become `waiting_for_review`. User creates ScriptVersion `sv2` based on proposal `sv1`, then approves review selecting `sv2`. ReviewResolution completes review node with `sv2`, downstream fingerprints use it, Job returns `queued/running`; upstream research/generate Script is not rerun. See matrix §5 and API Job/Script review commands.

### Scenario C — Change one matched Clip and rerender

User creates TimelineVersion `tv2` from `tv1` with one `origin=user` Clip. `POST /renders` references `tv2`; Render Job starts at Timeline/profile descendants. Match proposal and AI artifacts remain reusable and renderer cannot rematch. Only Timeline/render/profile fingerprint descendants invalidate.

### Scenario D — 16:9, 9:16 and 1:1 outputs

Three Renders reference one TimelineVersion and three RenderProfile versions. Research, ScriptVersion, Narration, Scenes, match proposal and Timeline reuse by fingerprint. Profile-specific reframe/mix/render/QA Artifacts differ; no full pipeline rerun.

### Scenario E — Worker dies between TTS and Scene Detection

Narration Artifact and `generate_narration` checkpoint commit before `node.completed`. Lease on current/incomplete step expires; reconciler verifies committed evidence, marks lost attempt, schedules attempt+1 for first incomplete compatible node. Narration is reused; Redis/local disk is not source of truth.

### Scenario F — VLM provider fails

VLM adapter maps error. Transient errors retry under P-AI/node budget and circuit breaker. Declared fallback may select another authorized ProviderConfiguration; every call records provenance. Per-scene partial result is allowed only when node contract declares it and missing items are explicit. Exhaustion follows `caption_scenes` soft/strict failure semantics; no provider branch in Product API.

### Scenario G — Malicious media upload

Bytes bypass Product API and remain staged/untrusted. Disposable validation executor enforces checksum/magic/probe/scan/resource limits with no DB/Redis/product secrets. Security finding quarantines Asset/Artifact and production Job rejects it. Container/process quotas/egress/path guards reduce blast radius.

### Scenario H — New upstream release

Record old/new remote peeled commits, diff modules/contracts/dependencies/security, run frozen V1/golden suite, update per-module disposition, decide import/port/ignore with rollback, and never merge wholesale across V2. Procedure is `09 §9`, task T604.

## 7. Audit result

All A–H scenarios have a traceable contract. Open product choices are isolated in `OPEN-QUESTIONS.md` and corresponding implementation tasks declare hard dependencies.
