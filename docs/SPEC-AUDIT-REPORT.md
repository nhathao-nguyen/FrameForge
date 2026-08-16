# Specification Audit

## Overall status

**READY**

Meaning: the architecture/specification set is internally consistent and implementation work can be decomposed without inventing architecture. This does **not** authorize application code: T000 owner ratification and each task's Open Question prerequisites still apply.

Audit date: 2026-08-15 (Asia/Ho_Chi_Minh).

## Critical issues

No unresolved critical contradiction remains between the master plan, domain, PostgreSQL, REST, pipeline, Timeline, events, security, migration and implementation order.

Critical issues fixed during this audit:

1. Initial worker topology previously implied specialized AI/ML/Render pools too early. It now requires one Engine Worker class initially and capability pools only after evidence.
2. Job/node state names diverged (`pending`, `succeeded`, `waiting_review`, `retry_wait`, `dead`). Canonical V2 sets now match DB/API/events exactly.
3. Missing PipelineRun/Workflow/ScriptVersion/TimelineVersion/SubtitleTrack/Voice/Narration/RenderProfile/ProviderConfiguration semantics and persistence were added.
4. Asset and Artifact duplicated storage identity. Asset is now logical input and original bytes are an immutable Artifact.
5. V1 queue was implicitly treated as distributed/durable. Source proves it is per-process ThreadPoolExecutor + JSON state; V2 shared queue/state architecture is now explicit.
6. Studio “edit Script then resume” lacked a non-mutating command model. Generic ReviewRequest/Resolution now selects an edited version as node output and invalidates descendants only.

## Major issues

All identified major documentation issues were corrected:

- PipelineNode contract now requires inputs/outputs/artifact roles/dependencies/retry/timeout/progress/checkpoint/idempotency/failure/review policy for every node.
- Timeline JSON now specifies explicit timeline/source in/out, transforms/crop/position/speed/opacity/transitions/audio/subtitle/narration/BGM semantics and immutable versioning.
- REST now covers complete Project/Asset upload/Script/Timeline/Job/Run/Step/review/Render/Artifact/provider configuration lifecycle, concurrency, idempotency and auth boundary.
- Event contract now has one envelope, exact IDs/version/sequence/time, canonical job/node/review/artifact/render events and matching SSE/WebSocket replay semantics.
- PostgreSQL now maps every persistent entity, retry/checkpoint/provider/render/review state and intentionally explains embedded/non-tabular objects.
- Roadmap phases now include scope, prerequisites, deliverables, tests, acceptance criteria and explicit non-goals.
- Implementation order was decomposed into 81 separately reviewable tasks, each with Goal, Files/modules, Dependencies, Implementation notes, Tests and Definition of Done.
- Migration now has current/target/strategy/compatibility/tests/removal criteria and a source-level disposition for all 120 Python modules plus Docker/Compose/CI/tests/plugins.

## Minor issues

No minor inconsistency blocks specification readiness.

Validation tooling note: both Timeline JSON code blocks parse as valid JSON; the available host lacks the `jsonschema` package/CLI, so Draft 2020-12 meta-schema execution is assigned to T232 and CI. The schema/example were also checked manually against required/discriminator/cross-field contracts. This is test-environment evidence, not an unresolved architecture decision.

## Missing specifications

None after remediation. Added dedicated specifications for:

- provider architecture;
- storage/upload/local-cache architecture;
- initial and scale-out worker architecture;
- reproducible Arch Linux environment;
- canonical glossary/status mappings;
- full upstream module disposition;
- domain/DB/API/event consistency and scenarios A–H.

`AGENTS.md` did not exist and was added as a short invariant/index file. `.agents/skills/` does not exist; therefore there were no project-local skills to audit for overlap. No skills were created because specs and AGENTS already provide the requested source-of-truth routing, and skill creation was not needed for readiness.

## Contradictions fixed

| Previous contradiction/ambiguity | Canonical resolution |
|---|---|
| V2 Job `pending` vs queue/node meanings | Job starts `created`; `pending` is JobStep only. |
| `succeeded`/`success`/`completed` | V2 terminal success state is `completed`; V1 `success` maps in adapter. |
| `waiting_review` vs `waiting_for_review` | `waiting_for_review` everywhere in V2. |
| `retry_wait`, `blocked_terminal`, `dead` | `retrying`, `blocked`, `dead_lettered`. |
| Job vs Task | Job is product aggregate; Task is V1 compatibility term. |
| PipelineNode vs JobStep | Definition vs runtime execution, connected through PipelineRun. |
| Script/Timeline “versioned” but one table/entity | Aggregate + immutable ScriptVersion/TimelineVersion tables/resources. |
| Render vs output file | Render is request/result; MP4/audio/subtitle/QA are Artifacts. |
| Match result vs TimelineClip | Match is proposal; TimelineClip is canonical user/AI edit decision. |
| Asset stores object key and Artifact also stores blob | Asset points to original/variant Artifacts; Artifact owns storage locator/checksum. |
| Phase 1 specialized worker pools | Initial one Engine Worker; future capability pools are deployment evolution. |
| V1 JSON/SQLite task store assumption | Source confirms JSON file storage, not SQLite product state. |
| Python 3.13 master vs upstream Docker 3.12 | Product/core target stays 3.13; runtime split is explicit OQ-13 based on actual ML wheel evidence. |
| Compatibility docs omitted batch/schedule/DLQ | Verified routes recorded; preservation breadth is OQ-14, not silently dropped. |

## Open decisions

The original 14 decisions use the required Question/Why/Options/Pros/Cons/Recommendation/`OPEN`
format and hard-block only dependent tasks. A later desktop-client scope extension adds OQ-15 in
the same format; it blocks only desktop shell/package work and does not change the original V2
server/engine boundaries.

Highest-priority decisions:

- OQ-12: final V2 package namespace—blocks first application scaffold T100.
- OQ-13: Python 3.13 vs temporary legacy/ML 3.12 split—blocks reproducible environment T002.
- OQ-14: breadth of frozen V1 compatibility—blocks compatibility profile T003.
- OQ-01 and OQ-06: identity and Workspace MVP scope—block auth/ownership schema/API.
- OQ-03: Redis queue primitive—blocks QueuePort implementation.
- OQ-05: provider credential ownership/secret backend—blocks provider configuration/real adapters.

Other open choices (Timeline projection/edit transport, event transport, vector storage, legacy ownership, retention, pipeline authoring and desktop shell/runtime) have safe fixed invariants and explicit task gates.

## Upstream assumptions verified

- Remote origin/main and local HEAD matched `bc2d276...`; annotated v1.1.0 peeled to that commit. v1.0.0 peeled to `121b883...`.
- V1 runner has exactly 16 ordered steps and StepRegistry hard/soft metadata; `strict` escalates soft failure.
- Workflow loading uses safe YAML, extra-forbid typed schema, aliases/path resolution and CLI > YAML > Settings merge behavior.
- REST implementation exposes tasks/result/artifacts/download, batch/batches, schedules/runs, deadletters/replay, health/ready/info/metrics/OpenAPI; public unauthenticated bind is guarded.
- LocalTaskQueue is in-process ThreadPoolExecutor; task/batch persistence is JSON. Compose replicas do not consume a shared broker.
- CheckpointStore performs atomic JSON replacement and step-level resume; completion removes checkpoints while failure/cancel retains them.
- Retry/backoff/DLQ and best-effort distributed render exist; distributed input sharing is explicitly out of scope upstream and remote failure falls back local.
- Local/S3 ArtifactStore has key normalization/path containment; TaskResult/Context still expose raw paths.
- Plugin loader auto-executes entry points; provider registry is global and not uniformly typed across LLM/research/TTS/Vision.
- TTS has Edge/OpenAI/MiMo providers/cache/voice mapping; ASR includes WhisperX/faster-whisper/FunASR fallback paths.
- Scene detection uses PySceneDetect with full-length fallback; matching uses ASR/text embeddings/heuristics/top-k/diversity/filters and optional VLM/visual features.
- Rendering uses MoviePy + FFmpeg; subtitle/audio/QA/export implementations and tests exist.
- No `shell=True` or `os.system` was found; FFmpeg subprocess callsites remain scattered and Bandit skips B404/B603.
- Docker is multi-stage/non-root UID 10001 and intentionally Python 3.12; upstream CI tests Python 3.10–3.13 with unit/integration/media/plugin/security jobs.
- V1.1.0 changed 234 files versus v1.0.0, so future updates require module-aware compare/port, not wholesale merge.

## Security findings

Resolved in specification:

- explicit Public API → trusted orchestration/controller → disposable media executor boundaries;
- direct presigned uploads, staged validation and quarantine before production use;
- executor has no PostgreSQL/Redis/product credential and only exact provider secret when needed;
- non-root/read-only/resource/time/PID/disk/egress/process-tree controls;
- path/symlink/archive/SSRF/object storage/presigned URL/plugin/subprocess/FFmpeg/security scan controls;
- no arbitrary FFmpeg flags/executable/project `.env` override in production;
- third-party plugins disabled/allowlisted/isolated;
- secret manager reference, redacted logs/events/checkpoints/provider runs;
- dependency/image pinning and scoped advisory exception requirements.

Residual risks are implementation/test obligations in T224, T313, T600–T602, not missing architecture.

## Implementation blockers

- Current user instruction and documentation gate prohibit application code.
- T000 owner ratification is required before Phase 0 tasks.
- OQ-12/OQ-13/OQ-14 must be decided before T100/T002/T003 respectively; OQ-15 must be decided before T433–T434.
- Each later task lists specific OQ and prior-task dependencies; agents must not infer a recommendation as approval.
- `.agents/skills/` is absent; this is not a blocker because AGENTS routes agents to canonical docs.

## Scenario audit

All required scenarios A–H have complete traces in `SPEC-CONSISTENCY-MATRIX.md §6`:

- A: direct upload → Asset validation → automatic Pipeline → Timeline → Render → Artifact;
- B: Script proposal → durable review → edited ScriptVersion resolution → downstream resume;
- C: user Clip override → new TimelineVersion → render-only descendants;
- D: three profiles reuse upstream intermediates;
- E: worker death resumes first incomplete node after committed TTS checkpoint;
- F: VLM retry/circuit/fallback/partial/soft-hard semantics;
- G: malicious media quarantine/sandbox/least-secret containment;
- H: remote commit/module diff/regression/port decision/rollback procedure.

## Final recommendation

Accept the specification set as **READY**, keep application code blocked until T000 decisions are ratified, then begin with T001 (record immutable upstream baseline manifest). Do not begin Product scaffold or Phase 1 before Phase 0 evidence and OQ-12 are complete.
