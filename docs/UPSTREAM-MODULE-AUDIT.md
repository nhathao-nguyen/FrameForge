# Upstream Movie Narrator Research Module Audit

## 1. Status

**REFERENCE RESEARCH — NOT TARGET ARCHITECTURE**

The prior audit inspected Movie Narrator at commit
`bc2d276cf477fe3ce1a16f9679dcb1d2978e3a74` using source, tests, Docker/Compose, CI and history.
This file preserves module-level observations so capability knowledge is not lost. Normative product
dispositions are in [`UPSTREAM-CAPABILITY-MATRIX.md`](UPSTREAM-CAPABILITY-MATRIX.md).

All rows have NH-Media runtime/build/deployment dependency on upstream: **NONE**.

Classification:

- `REFERENCE`: research evidence only;
- `ADOPT-CONCEPT`: use an idea, not source;
- `REIMPLEMENT`: independently implement capability;
- `IMPROVE`: independently implement with intentional improvements;
- `DEFER`/`IGNORE`: later or explicitly excluded with rationale.

## 2. Package and product surfaces

| Upstream modules/areas inspected | Observed behavior | NH-Media classification | NH-Media consequence |
|---|---|---|---|
| `__init__.py`, `contract.py`, `models.py` | Package facade, versioned contract, path-heavy mutable context/models. | REFERENCE | Do not preserve imports/DTOs. Use native domain and Artifact refs. |
| `cli.py`, `doctor.py` | Typer commands and environment diagnostics. | REFERENCE / DEFER | No CLI compatibility. Optional native client/admin CLI may be added later. |
| `config.py`, `workflow/{load,merge,schema}.py` | Typed config, safe YAML, precedence/aliases/path resolution. | ADOPT-CONCEPT | Native Product commands/config validation; no upstream files or aliases. |
| `imitate.py` | Reference-video-oriented workflow/helpers. | IMPROVE | `ReferenceStyleAnalysis` extracts abstract traits without copying footage. |
| `race.py` | Multiple generated alternatives and comparison. | IMPROVE | Native GenerationCandidate/EvaluationResult/SelectionPolicy. |
| `plugin_loader.py`, example plugins | Python entry-point discovery executes arbitrary code. | IGNORE / DEFER | Unrestricted loading excluded for security; reviewed extension manifests later. |

## 3. Task, API and reliability surfaces

| Upstream modules/areas inspected | Observed behavior | NH-Media classification | NH-Media consequence |
|---|---|---|---|
| `cloud/api.py`, `cloud/openapi.py`, `cloud/models.py` | Task REST, status/results, batch, schedule, DLQ, artifacts, health. | REFERENCE | Native `/api/v1` resources only; no route/status compatibility. |
| `cloud/queue.py`, `cloud/storage.py` | In-process ThreadPoolExecutor and JSON state; not shared durable queue. | REIMPLEMENT | PostgreSQL state/outbox + Redis delivery + leases. JSON task store excluded. |
| `cloud/worker.py`, `cloud/daemon.py`, `cloud/remote_queue.py` | Fixed-runner worker/daemon/client, cancellation and graceful drain. | ADOPT-CONCEPT | Native bounded worker/controller/executor and Product API client. |
| `cloud/checkpoint.py` | Atomic step-level JSON checkpoints/resume. | IMPROVE | Durable schema/fingerprint/Artifact checkpoints with selective invalidation. |
| `cloud/dlq.py`, `reliability/{retry,circuit_breaker}.py` | Backoff, circuit breaker and fresh-ID replay concepts. | ADOPT-CONCEPT | Native error taxonomy, provider/node policy and audited new-Job replay. |
| `cloud/health.py`, `cloud/metrics.py` | Health/readiness/Prometheus behavior. | REIMPLEMENT | Native service-specific health/metrics with bounded labels and redaction. |
| `cloud/scheduler.py` | Local persisted cron submission. | DEFER | Product scheduling after core Job API; timezone/idempotency required. |
| `cloud/distributed.py` | Best-effort remote render with local fallback; input sharing unresolved. | ADOPT-CONCEPT | Future capability routing/leases/Artifact refs; no early distributed claim. |
| `cloud/artifact_store.py`, `cloud/lifecycle.py` | Local/S3 storage, path guards and cleanup. | ADOPT-CONCEPT | Independently authored StoragePort/conformance/retention with product ownership. |
| `cloud/remote_provider.py` | Remote artifact/provider proxy concepts. | REIMPLEMENT | Native provider/storage ports; no upstream proxy protocol. |

## 4. Pipeline capabilities

| Upstream modules inspected | Observation retained | Class | NH-Media target |
|---|---|---|---|
| `pipeline/runner.py`, `registry.py`, `errors.py`, `preflight.py` | Ordered stages, hard/soft behavior, preflight and registry risks. | IMPROVE | Versioned DAG, complete node policy, capability validation and canonical errors. |
| `pipeline/resolve.py`, `assets.py` | Source resolution and asset preparation. | REIMPLEMENT | Asset validation and prepared-media Artifacts. |
| `pipeline/research.py`, `script.py`, `script_export.py` | Research, structured script generation and Markdown export. | REIMPLEMENT | ResearchAnalysis, ScriptVersion and export Artifact. |
| `pipeline/tts.py`, `_align_backend.py`, `align.py` | TTS, backend selection, alignment and degradation. | REIMPLEMENT | Narration/TTS/ASR/alignment ports and typed Artifacts. |
| `pipeline/scenes.py`, `scene_filter.py` | Scene detection, filtering and full-length fallback. | REIMPLEMENT / ADOPT-CONCEPT | Native Scene records, explicit fallback/filter policies. |
| `pipeline/match.py` | Transcript/embedding/heuristic/VLM matching with diversity/filtering. | IMPROVE | Score-component MatchProposals, candidate evaluation and Timeline separation. |
| `pipeline/bgm.py` | Music mix, ducking/loudness. | IMPROVE | Timeline audio tracks and explicit loudness/mix report. |
| `pipeline/translate.py`, `subtitle.py` | Translation and original/translated/bilingual subtitles. | REIMPLEMENT | Timed-text Artifacts and SubtitleTracks. |
| `pipeline/qa_gate.py`, `qa.py` | Intermediate/deliverable QA. | REIMPLEMENT | Versioned QA policies/reports and canonicalization gate. |
| `pipeline/render.py`, `export_clips.py` | MoviePy/FFmpeg composition and derivative clips. | REIMPLEMENT | Go Timeline compiler/media worker, render/QA/export from exact TimelineVersion. |

The observed stage order remains research input to capability coverage, not a required graph or
compatibility contract.

## 5. Presets, providers, speech and vision

| Upstream modules inspected | Observation retained | Class | NH-Media target |
|---|---|---|---|
| `presets/*` | Long/fast/mainstream production defaults and registry. | REIMPLEMENT | Versioned Workflow/RenderProfile templates; no global arbitrary registry. |
| `providers/registry.py`, `utils/llm.py`, provider facades | Global mutable registry and uneven protocols. | IMPROVE | Dependency-injected typed ProviderResolver/conformance. |
| `providers/tmdb.py` | Metadata research, retry and circuit concepts. | REIMPLEMENT | Typed research adapter with egress/provenance. |
| `providers/asr/funasr.py`, ASR facades | Lazy optional ASR backend. | REIMPLEMENT | Capability-declared ASR adapters in `nh_media`. |
| `tts/{protocol,base,cache,edge,openai_provider,mimo_provider,factory,voice_map}.py` | Async TTS, providers, cache and voices. | REIMPLEMENT | Native TTSProvider/Narration/cache fingerprint; Edge policy limited. |
| `vision/{protocol,factory,stub,vlm}.py` | Keyframes, VLM captions, fake provider, retry/partial behavior. | REIMPLEMENT / IMPROVE | Separate media extraction and typed VLM Analysis with per-item provenance. |

## 6. Utility evidence

| Upstream modules inspected | Capability observation | Classification |
|---|---|---|
| `alignment_qa.py`, `audio_qa.py`, `subtitle_qa.py`, `video_qa.py`, `deliverable_qa.py`, `qa_report.py`, `quality_dashboard.py` | Quality metrics/reports across speech/audio/subtitle/video. | REIMPLEMENT |
| `audio_mix.py`, `prosody.py`, `emotion_track.py` | Loudness/ducking/prosody/emotion concepts. | IMPROVE |
| `match_quality.py`, `visual_features.py` | Composite scoring, rhythm/diversity and visual feature scaffold. | IMPROVE |
| `metadata_export.py`, `warnings.py`, `cost_tracker.py`, `logging_config.py` | Metadata/warning/cost/correlation observations. | ADOPT-CONCEPT |
| `ffmpeg_bin.py`, `gpu_detect.py`, `optional_deps.py`, `environment.py` | Executable/capability/environment detection. | REIMPLEMENT with stricter allowlists |
| `font.py`, `text_anim.py`, `text_image.py`, `transitions.py`, `video_layout.py`, `preview.py` | Font/text/effect/layout/preview rendering concepts. | REIMPLEMENT |
| `glossary.py`, `prompts.py`, `json_parser.py` | Translation glossary, prompt assets and model JSON parsing. | REIMPLEMENT with versioned schemas |
| `sanitize.py`, `errors.py`, `log.py`, `retention.py`, `async_utils.py` | Filename/error/log/retention/async utility concerns. | REIMPLEMENT under native boundaries |
| `console.py` | CLI console/progress rendering. | IGNORE for product core; optional native CLI later |

## 7. Deployment, tests and supply chain observations

| Area | Observation | Classification / target |
|---|---|---|
| Upstream Dockerfile | Multi-stage CPU/GPU, non-root UID, Python/ML constraints. | ADOPT-CONCEPT; build independent Go/`nh_media` images. |
| Upstream Compose | Separate local queues and insecure development MinIO examples. | REFERENCE warning; native shared PostgreSQL/Redis/private storage topology. |
| Upstream CI/tests | Broad unit/integration/media/plugin/security coverage; some subprocess/advisory skips. | ADOPT-CONCEPT for coverage; do not copy exceptions without native review. |
| Upstream publish workflow | Publishes external package/release. | IGNORE; unrelated to NH-Media release. |
| Timeline export example | Jianying/OTIO interoperability idea. | DEFER until canonical Timeline is stable. |

## 8. Verified risks retained as research

1. In-process queue/JSON state is not a durable distributed product queue.
2. Best-effort remote render does not prove shared scheduling/input durability.
3. Path-heavy context/results motivate Asset/Artifact refs.
4. Global provider/plugin registries expand secret and arbitrary-code risk.
5. Media subprocess callsites require one reviewed port despite argv-list observations.
6. Scene/ASR/matching/render/subtitle/audio/QA capabilities exist and are not silently discarded.
7. Candidate race, style analysis, batch, schedule, DLQ and distribution are explicitly represented
   in the capability matrix.

## 9. Audit conclusion

The useful research inventory is retained, but no module is designated for reuse, wrapping,
porting, runtime compatibility or migration. Future agents use the capability matrix and independently
author NH-Media behavior, interfaces, tests and implementation.
