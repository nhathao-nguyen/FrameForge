# Upstream Movie Narrator V1 Module Audit

## 1. Audit baseline and method

Source audited: `references/movie-narrator` at peeled `v1.1.0`/remote `main` commit `bc2d276cf477fe3ce1a16f9679dcb1d2978e3a74` on 2026-08-15. Audit used source implementation, tests, Docker/Compose, CI and git diff/history—not README alone.

Disposition definitions are normative in `09-MIGRATION-FROM-UPSTREAM.md`. “Removal” always means removal from supported compatibility path, never immediate deletion during this documentation phase.

Test shorthand:

- `C`: frozen V1 unit/CLI/REST compatibility regression.
- `A`: adapter DTO/path/status/artifact mapping and redaction.
- `P`: provider conformance/error/retry/secret tests.
- `M`: real-media/golden/FFmpeg/scene/audio/render tests.
- `R`: queue/lease/checkpoint/retry/crash/DLQ tests.
- `S`: security/path/subprocess/plugin/upload tests.
- `T`: Timeline/domain/versioning/multi-output tests.

Removal shorthand:

- `legacy-retire`: no remaining compatibility consumer, parity + migration + rollback evidence, announced deprecation window complete.
- `node-parity`: target node passes golden/contract/security tests and can rollback by Pipeline version.
- `never-product`: retained only as upstream reference unless a new product decision opens scope.

## 2. Core modules

| Module | Class | Verified current behavior | Target and technical reason | Compatibility | Tests / removal |
|---|---|---|---|---|---|
| `movie_narrator/__init__.py` | REUSE | Legacy package facade/version. | Keep legacy namespace; no mass rename. | Imported by V1 clients. | `C`; `legacy-retire`. |
| `cli.py` | WRAP | Typer CLI drives create/config/task/server/ops flows. | Compatibility CLI delegates through VideoEngine/Product API without copying business logic. | Preserve frozen flags, aliases and exit behavior. | `C,A`; `legacy-retire`. |
| `config.py` | WRAP | Pydantic settings/env, provider/render/media controls. | Translate allowlisted V1 settings into immutable command/provider snapshots; production forbids unsafe executable overrides. | Legacy `.env` remains in legacy mode. | `C,A,S`; `legacy-retire`. |
| `contract.py` | REUSE | Stable export surface, contract `(1,0,0)`. | Primary adapter import boundary. | Pin contract version and additive policy. | `C,A`; remove only with whole legacy contract. |
| `doctor.py` | REUSE | Environment/FFmpeg/provider diagnostics. | Use unchanged for V1 baseline; port checks later, not output format blindly. | `mn doctor` preserved. | `C`; `legacy-retire`. |
| `models.py` | WRAP | Mutable path-heavy Context plus Script/TimedSegment/Scene/MatchedClip models. | Map to V2 DTOs/refs; do not make Context V2 domain. | Bidirectional adapter with sandbox paths. | `C,A`; `legacy-retire`. |
| `imitate.py` | IGNORE | Reference-video imitation helpers with FFmpeg subprocess. | Not in approved first workflows; retaining avoids unjustified rewrite. | Legacy command only if frozen profile includes it. | `C,S`; `never-product`. |
| `race.py` | IGNORE | Multi-candidate “horse race” comparison. | Outside initial product scope; no target aggregate/acceptance criteria. | Legacy only. | `C`; `never-product`. |
| `plugin_loader.py` | REPLACE | Auto-discovers/executes Python entry points; load failures warn. | Production registry is built-in/allowlisted and isolated because arbitrary code violates trust boundary. | Explicit legacy-plugin mode only, disabled production. | `C,S`; remove auto-load when compatibility window ends. |

## 3. Cloud/task modules

| Module | Class | Verified current behavior | Target and technical reason | Compatibility | Tests / removal |
|---|---|---|---|---|---|
| `cloud/__init__.py` | REUSE | Re-export facade for V1 cloud package. | Keep for old imports. | Legacy-only facade. | `C`; `legacy-retire`. |
| `cloud/api.py` | REPLACE | stdlib HTTP TaskAPIServer; task/batch/schedule/DLQ/artifact/health routes; X-API-Key; request limits. | Go Product API/control plane owns Project/auth/versioning/DB contracts unavailable here. Keep V1 server unchanged behind compatibility boundary. | Versioned route/status/result gateway or frozen daemon. | `C,A,S`; `legacy-retire`. |
| `cloud/openapi.py` | REPLACE | Hand-built OpenAPI 3.1 for V1 routes. | Product OpenAPI generated/versioned from new contract; V1 document remains compatibility artifact. | Expose exact legacy spec on compatibility listener. | `C`; `legacy-retire`. |
| `cloud/models.py` | WRAP | TaskRequest/Status/Progress/Result/Batch models; status includes `dead`; result exposes paths. | Mapper to Job/PipelineRun/Artifact; V2 status vocabulary remains separate. | Accept `format` alias and frozen fields. | `C,A`; `legacy-retire`. |
| `cloud/queue.py` | REPLACE | `TaskQueue` protocol + in-process ThreadPoolExecutor/JSON state; running tasks can remain stale after crash. | Product requires Redis delivery + PostgreSQL state/lease/outbox; replacement is requirement-driven, not code style. | LocalTaskQueue remains legacy daemon backend. | `C,R`; `legacy-retire`. |
| `cloud/storage.py` | REPLACE | JSON-based task/batch persistence with file locking/atomic writes. | PostgreSQL is product source of truth; JSON cannot provide product relations/transactions/tenant isolation. | Read/import legacy state. | `C,A,R`; remove writer at `legacy-retire`, retain importer. |
| `cloud/worker.py` | WRAP | Executes fixed runner, progress, retry, checkpoint, cancellation, optional distributed render/DLQ. | Legacy executor inside worker adapter; V2 controller/state machine is new. | Translate callbacks/results/checkpoints. | `C,A,R,M`; `legacy-retire`. |
| `cloud/daemon.py` | WRAP | Runs LocalTaskQueue + API + scheduler; loopback default, public bind key guard, graceful drain. | Frozen compatibility service; useful drain/security behavior ported to V2 controller. | Preserve serve behavior. | `C,R,S`; `legacy-retire`. |
| `cloud/remote_queue.py` | WRAP | HTTP client implements task queue semantics against daemon. | Compatibility client/gateway only; not V2 broker abstraction. | Preserve API key/error/poll/download behavior. | `C,A`; `legacy-retire`. |
| `cloud/remote_provider.py` | PORT | Artifact HTTP downloads and remote LLM/TTS registration/proxy. | Port remote adapter concepts to provider/storage ports; current URL/path/global registration is not domain-safe. | Legacy remote provider names remain mapped. | `C,P,A,S`; `node-parity`. |
| `cloud/artifact_store.py` | PORT | Local/S3 StorageBackend, key normalization, local containment, presigned URL. | Valuable protocol/guards become StoragePort adapters; add Asset/Artifact metadata and MinIO conformance. | Legacy task-scoped store wrapped. | `C,A,S`; storage conformance then `node-parity`. |
| `cloud/lifecycle.py` | PORT | TTL/size/keep-last cleanup with active-task protection and sweeper. | Port to reference-aware Artifact retention; current key-prefix protection alone is insufficient. | Legacy sweeper preserved. | `C,R,S`; V2 retention gate + `legacy-retire`. |
| `cloud/checkpoint.py` | PORT | Per-task JSON checkpoint, completed step/context, atomic write/resume plan. | Preserve resume semantics in DB + checkpoint Artifact with fingerprint/schema/invalidation. | Import/map legacy checkpoints. | `C,A,R`; V2 crash-at-each-node parity. |
| `cloud/dlq.py` | PORT | JSON DeadLetterStore and replay under fresh task ID. | Port fresh-ID/audit semantics to DB dead_letters/Job replay. | Legacy DLQ routes map. | `C,R`; V2 exhaustion/replay parity. |
| `cloud/health.py` | PORT | Core/deep dependency checks, ready/health payloads. | Port semantics to product/worker health boundaries; avoid leaking dependency details publicly. | Legacy payload version preserved. | `C,S`; V2 health contract gate. |
| `cloud/metrics.py` | PORT | In-process Prometheus metrics implementation/exposition. | Port metric names/behavior where useful; use bounded labels and V2 run/step concepts. | Legacy `/metrics` remains. | `C,R`; observability acceptance. |
| `cloud/scheduler.py` | IGNORE | Cron schedules persisted locally and submit to LocalTaskQueue. | Scheduling is not master-plan MVP and must not inflate Phase 1. | Preserve only if frozen V1 compatibility profile includes routes. | `C`; `never-product` until explicit decision. |
| `cloud/distributed.py` | IGNORE | Conditional remote render when enabled/healthy/long; remote failure falls back local; shared inputs unresolved. | Go/Python workers use a bounded versioned worker contract; V1 distributed behavior remains a compatibility surface, not evidence of a durable scheduler. | V1 feature remains legacy. | `C,R,M`; future ADR before port. |

## 4. Pipeline modules

| Module | Class | Verified current behavior | Target and technical reason | Compatibility | Tests / removal |
|---|---|---|---|---|---|
| `pipeline/__init__.py` | REUSE | Legacy step package facade. | Keep imports stable. | Legacy only. | `C`; `legacy-retire`. |
| `pipeline/runner.py` | WRAP | Fixed 16-step order, StepRegistry metadata, start step, pause-after-step, soft/hard/strict handling. | LegacyMovieNarratorAdapter executes it; V2 DAG lives separately. | Ordered compatibility Pipeline and aliases. | `C,A,R`; `legacy-retire`. |
| `pipeline/registry.py` | REFACTOR | Global StepRegistry/hooks and hard/soft metadata. | V2 typed node registry with schema/capability/policy and no untrusted auto-registration. | Legacy registry stays inside adapter. | `C,S`; DAG registry acceptance + rollback. |
| `pipeline/errors.py` | PORT | Pipeline error hierarchy. | Translate into canonical node/provider/media error taxonomy. | Adapter retains original exceptions internally. | `C,A`; target error contract parity. |
| `pipeline/preflight.py` | PORT | Checks LLM/TTS availability before run. | Capability/provider resolver preflight with scoped credentials. | Legacy checks still run in adapter. | `C,P`; `node-parity`. |
| `pipeline/resolve.py` | PORT | Resolves source video/library paths. | `resolve_source_asset` uses Asset/Artifact refs; path discovery only importer/local compatibility. | Adapter materializes legacy path. | `C,A,S`; `node-parity`. |
| `pipeline/assets.py` | PORT | Downloads/organizes video/assets in output/cache paths. | `prepare_media_assets` produces normalized immutable Artifacts via StoragePort. | Sandbox path mapper. | `C,A,M,S`; `node-parity`. |
| `pipeline/research.py` | PORT | Research provider/TMDB/LLM data collection, soft behavior. | Provider-neutral research node + Artifact/provenance. | Legacy soft/strict mapping. | `C,P`; `node-parity`. |
| `pipeline/script.py` | PORT | LLM script generation, prompt/parse/quality logic. | Generates immutable ScriptVersion proposal via LLM port. | Mapper to V1 ScriptSegment/export. | `C,P,T`; `node-parity`. |
| `pipeline/script_export.py` | PORT | Writes Markdown script. | Export ScriptVersion as Artifact role; filename alias remains. | `script.md` download alias. | `C,A`; `node-parity`. |
| `pipeline/tts.py` | PORT | Selects TTS provider, synthesizes/cache/probes narration paths. | Narration + TTS port + Artifact commit. | Legacy path/result mapper. | `C,P,M,A`; `node-parity`. |
| `pipeline/_align_backend.py` | PORT | Selects WhisperX/faster-whisper/FunASR by availability/config. | ASR capability/fallback node policy. | Preserve backend names/order in frozen profile. | `C,P,M`; `node-parity`. |
| `pipeline/align.py` | PORT | Aligns script/audio, timed segments and fallback/degradation. | Alignment JobStep + timing Artifact, exact ScriptVersion/Narration. | TimedSegment mapper. | `C,M,P`; `node-parity`. |
| `pipeline/scenes.py` | PORT | PySceneDetect; optional soft path; full-length fallback if none. | Scene records/artifacts tied to source revision. | Preserve threshold/fallback semantics. | `C,M,A`; `node-parity`. |
| `pipeline/scene_filter.py` | PORT | Intro/dark/window filters using FFmpeg/probe. | Declarative matching candidate filters with audited parameters. | Legacy filter metadata mapped. | `C,M,S`; `node-parity`. |
| `pipeline/match.py` | REFACTOR | ASR transcript, sentence embeddings, heuristics/top-k/diversity/filter/VLM inputs; writes matched clips. | Multimodal scorer + proposal Artifact; needs component scores/Character/coverage and no renderer coupling. | V1 scorer remains fallback by Pipeline version. | `C,M,T,P`; quality gate + user override + rollback. |
| `pipeline/bgm.py` | PORT | BGM selection/mix/duck/loudness helpers with soft behavior. | Timeline audio tracks + mix_audio node/Artifact. | Legacy BGM path/config aliases. | `C,M,T`; `node-parity`. |
| `pipeline/translate.py` | PORT | Chunked subtitle translation with provider/retry/glossary. | LLM/provider-neutral timed-text Artifact, exact language/provenance. | V1 config/output names. | `C,P,M`; `node-parity`. |
| `pipeline/subtitle.py` | PORT | Builds original/translated/bilingual SRT. | SubtitleTrack/cues + export Artifacts. | Legacy filenames and modes. | `C,M,T`; `node-parity`. |
| `pipeline/qa_gate.py` | PORT | Soft intermediate QA before render. | Versioned QA policy/report Artifact. | Soft/strict consequence mapping. | `C,M`; `node-parity`. |
| `pipeline/render.py` | REFACTOR | MoviePy composition + FFmpeg mux, templates/layout/transitions/subtitles, direct paths. | Renderer compiles exact TimelineVersion + RenderProfile and commits Artifacts; current algorithm can be ported incrementally. | V1 renderer wrapped; no V2 rematching. | `C,M,T,S`; three-profile/timeline-only parity + rollback. |
| `pipeline/qa.py` | PORT | Hard deliverable validation step. | `validate_deliverable` node + QA Artifact/canonicalization. | Preserve thresholds/output evidence. | `C,M`; `node-parity`. |
| `pipeline/export_clips.py` | PORT | FFmpeg exports matched scenes as files. | Exports declared Timeline/selection clips as Artifacts. | Legacy `clips/` aliases. | `C,M,S,T`; `node-parity`. |

## 5. Presets, providers, reliability, TTS and Vision

| Module | Class | Verified current behavior | Target and technical reason | Compatibility | Tests / removal |
|---|---|---|---|---|---|
| `presets/__init__.py` | PORT | Preset facade. | Workflow/RenderProfile preset catalog. | Legacy names map. | `C,T`; `node-parity`. |
| `presets/base.py` | PORT | Preset protocol/data and application to config. | Typed versioned preset data, no direct mutable Context. | Translator. | `C,T`; `node-parity`. |
| `presets/bilibili_long.py` | PORT | Long-form pacing/render defaults. | Built-in Workflow/Profile version. | Preserve preset key/output expectations. | `C,M`; `node-parity`. |
| `presets/douyin_fast.py` | PORT | Fast vertical pacing defaults. | Built-in Workflow/Profile version. | Preserve preset key. | `C,M`; `node-parity`. |
| `presets/mainstream_dry.py` | PORT | Mainstream commentary defaults. | Built-in Workflow/Profile version. | Preserve preset key. | `C,M`; `node-parity`. |
| `presets/registry.py` | REFACTOR | Global preset registration/lookup. | Built-in versioned catalog; no arbitrary production registration. | Legacy registry inside adapter. | `C,S`; catalog acceptance. |
| `providers/__init__.py` | REFACTOR | Unified registry exports. | ProviderResolver/typed ports facade. | Legacy exports remain. | `C,P`; `legacy-retire`. |
| `providers/registry.py` | REFACTOR | Global mutable factories; validates TTS/Vision protocol, not uniform LLM/research contract. | Dependency-injected resolver, ProviderConfiguration and conformance suite. | Legacy factory adapter. | `C,P,S`; no product provider branch/global mutation. |
| `providers/tmdb.py` | PORT | TMDB research provider with retry/circuit. | Research adapter with typed request/provenance/egress policy. | Legacy provider key. | `C,P,S`; `node-parity`. |
| `providers/asr/__init__.py` | PORT | ASR package facade/lazy backend exposure. | ASR adapters facade. | Keep imports/provider names. | `C,P`; `node-parity`. |
| `providers/asr/funasr.py` | PORT | Lazy optional FunASR backend yielding compatible segments. | ASRProvider adapter preserving lazy dependency and timing contract. | Legacy backend key. | `C,P,M`; `node-parity`. |
| `reliability/__init__.py` | PORT | Retry/circuit toolkit facade. | Engine/provider reliability ports/policies. | Legacy decorators remain. | `C,P,R`; target conformance. |
| `reliability/circuit_breaker.py` | PORT | CLOSED/OPEN/HALF_OPEN breaker around external calls. | Per-configuration/endpoint/capability breaker with observability. | Preserve legacy behavior in adapter. | `C,P,R`; target conformance. |
| `reliability/retry.py` | PORT | Exponential backoff/jitter retry policy/decorator. | Canonical node/provider retry policy with error categories/budget. | Translate V1 retry settings. | `C,P,R`; target conformance. |
| `tts/__init__.py` | PORT | TTS abstraction exports. | TTS port adapters. | Legacy names. | `C,P`; `node-parity`. |
| `tts/protocol.py` | PORT | Async `synthesize(text, voice, output_path)` interface. | Provider-neutral request/result blob contract; preserve async/cancel semantics. | Path adapter. | `C,P,A`; `node-parity`. |
| `tts/base.py` | PORT | Base provider and CI silent fallback. | Shared adapter base; silent fallback allowed only explicit test policy, never production success. | CI behavior frozen separately. | `C,P,M`; `node-parity`. |
| `tts/cache.py` | PORT | Filesystem cache key/layout helper. | Artifact/cache fingerprint including provider/model/voice/settings/schema. | Read legacy cache if safe. | `C,P,A`; cache parity. |
| `tts/edge.py` | PORT | Edge-TTS wrapper. | Local/test/personal adapter under policy. | Provider key/voices preserved. | `C,P,S`; legal/production policy gate. |
| `tts/openai_provider.py` | PORT | OpenAI SDK TTS adapter via thread. | Typed TTS adapter with scoped client/timeouts/results. | Legacy key. | `C,P`; `node-parity`. |
| `tts/mimo_provider.py` | PORT | MiMo OpenAI-compatible chat-based audio adapter. | Typed adapter with response validation/provenance. | Legacy key. | `C,P`; `node-parity`. |
| `tts/factory.py` | REFACTOR | Settings-based provider selection. | ProviderResolver; no scattered setting/provider branches. | Translator maps setting to configuration. | `C,P`; no direct product usage. |
| `tts/voice_map.py` | PORT | Language → default voice map. | Versioned Voice catalog/default policy. | Preserve language defaults in compatibility profile. | `C,P`; `node-parity`. |
| `vision/__init__.py` | PORT | VLM caption abstraction exports. | VLM port facade. | Legacy imports. | `C,P`; `node-parity`. |
| `vision/protocol.py` | PORT | `caption_scenes(scenes, video_path)` protocol. | VLM analyze request over Artifact/keyframe refs. | Path/Scene mapper. | `C,P,A`; `node-parity`. |
| `vision/factory.py` | REFACTOR | Settings-based captioner factory. | ProviderResolver. | Legacy setting translator. | `C,P`; no direct product usage. |
| `vision/stub.py` | PORT | Deterministic placeholder captions for CI/dev. | Fake VLM conformance adapter. | Preserve test outputs where frozen. | `C,P`; retain as test utility. |
| `vision/vlm.py` | PORT | FFmpeg keyframe extraction + OpenAI-compatible VLM, retry/circuit and per-scene fallback. | Separate keyframe media node + VLM adapter over refs; preserve useful fallback/provenance. | Legacy captioner wrapped. | `C,P,M,S`; `node-parity`. |

## 6. Utility modules

| Module | Class | Verified current role | Target and reason | Compatibility | Tests / removal |
|---|---|---|---|---|---|
| `utils/__init__.py` | REUSE | Legacy utility facade. | Keep imports. | Legacy only. | `C`; `legacy-retire`. |
| `utils/alignment_qa.py` | PORT | Timing remap/confidence/drift validation. | Alignment QA library/Artifact. | Preserve thresholds. | `C,M`; `node-parity`. |
| `utils/async_utils.py` | PORT | Async bridge/helpers. | Scoped executor/cancellation helpers after review. | Legacy unchanged. | `C,R`; target unit tests. |
| `utils/audio_mix.py` | PORT | Normalize, loudness and BGM ducking via FFmpeg. | Timeline audio compiler/media adapter. | Preserve settings. | `C,M,S`; `node-parity`. |
| `utils/audio_qa.py` | PORT | Clipping/SNR/silence checks. | Narration/render QA metrics. | Preserve evidence. | `C,M`; `node-parity`. |
| `utils/console.py` | IGNORE | CLI/UI console abstraction and progress rendering. | Product progress uses events/frontend; no engine dependency on console. | Legacy CLI only. | `C`; `never-product`. |
| `utils/cost_tracker.py` | PORT | In-run token/TTS counters. | ProviderRun usage/cost persistence with redaction. | Map legacy metadata. | `C,P`; provider audit gate. |
| `utils/deliverable_qa.py` | PORT | ffprobe/FFmpeg publishability checks. | Deliverable QA node under reviewed subprocess port. | Preserve thresholds/report. | `C,M,S`; `node-parity`. |
| `utils/emotion_track.py` | PORT | Time/emotion/intensity value object. | Timeline/voice/music analysis value contract. | Map metadata. | `C,T`; feature acceptance. |
| `utils/environment.py` | PORT | Environment inventory. | Baseline/health diagnostics with redaction. | Legacy output frozen. | `C,S`; ops acceptance. |
| `utils/errors.py` | PORT | Shared error classes. | Canonical error taxonomy/mapping. | Adapter catches/translates. | `C,A`; target error tests. |
| `utils/ffmpeg_bin.py` | REFACTOR | Resolves env override, bundled imageio FFmpeg, then system. | Reviewed MediaProcessPort with pinned allowlist; production env override disabled. | Legacy resolution only in compatibility/local. | `C,M,S`; all V2 callsites use port. |
| `utils/font.py` | PORT | Font discovery/loading. | Asset/font resolver in sandbox. | Legacy paths mapped. | `C,M,S`; `node-parity`. |
| `utils/glossary.py` | PORT | Translation glossary extraction/enforcement. | Translation node data/provenance. | Preserve behavior. | `C,P`; `node-parity`. |
| `utils/gpu_detect.py` | PORT | Probes encoder/GPU availability via subprocess. | Worker capability discovery under command allowlist. | Legacy probe preserved. | `C,S,R`; worker capability gate. |
| `utils/json_parser.py` | PORT | Extracts/parses model JSON. | Structured provider response validator; stricter schema. | Preserve fallback only where frozen. | `C,P,S`; conformance. |
| `utils/llm.py` | REFACTOR | LLM client factory backed by global registry. | LLMProvider adapter/resolver. | Legacy factory remains. | `C,P`; no product direct import. |
| `utils/log.py` | PORT | File logging. | Structured logging sink with retention/redaction. | Legacy files remain. | `C,S`; observability gate. |
| `utils/logging_config.py` | PORT | Structured logging/correlation IDs. | Shared request/job/run/step context; bounded labels/redaction. | Preserve correlation behavior. | `C,S,R`; observability gate. |
| `utils/match_quality.py` | REFACTOR | Composite match/rhythm/diversity scoring. | Explicit multimodal score components/versioned scorer. | V1 scorer fallback. | `C,M,T`; quality gate. |
| `utils/metadata_export.py` | PORT | Writes aggregate `metadata.json`. | Metadata/QA Artifact exporter; normalized DB remains truth. | Legacy filename/content fields. | `C,A`; `node-parity`. |
| `utils/optional_deps.py` | PORT | Lazy dependency/capability probes. | Worker capability descriptor, no import-time heavy failure. | Legacy extras behavior. | `C,R`; capability tests. |
| `utils/preview.py` | PORT | Preview duration/render options. | Preview RenderProfile/policy. | Legacy flags. | `C,M,T`; `node-parity`. |
| `utils/prompts.py` | PORT | Prompt templates/language support. | Versioned prompt assets behind provider/node config. | Freeze prompt version in compatibility Pipeline. | `C,P`; output parity. |
| `utils/prosody.py` | PORT | Emotion-to-TTS prosody mapping. | Voice/Narration policy. | Preserve mappings. | `C,P,M`; `node-parity`. |
| `utils/qa_report.py` | PORT | Human/JSON QA report export. | Immutable QA Artifact schema. | Legacy JSON alias. | `C,A,M`; `node-parity`. |
| `utils/quality_dashboard.py` | PORT | Aggregates cross-step quality scores. | Product read model/QA Artifact, not worker truth. | Map legacy metadata. | `C,M`; studio acceptance. |
| `utils/retention.py` | PORT | Rotates timestamped log files. | Log/Artifact retention policy with reference protection. | Legacy logs. | `C,S`; retention gate. |
| `utils/sanitize.py` | PORT | Filename sanitization. | Display/download filename defense; keys remain server-generated. | Preserve safe legacy filenames. | `C,S`; security gate. |
| `utils/subtitle_qa.py` | PORT | CPS/overlap/line/display checks. | SubtitleTrack/render QA. | Preserve thresholds. | `C,M,T`; `node-parity`. |
| `utils/text_anim.py` | PORT | Render text animation effects. | Render adapter implementation from Timeline style/profile allowlist. | Legacy templates. | `C,M,S,T`; renderer parity. |
| `utils/text_image.py` | PORT | Pillow subtitle/text rasterization. | Sandboxed render media adapter with font/text limits. | Legacy output. | `C,M,S`; renderer parity. |
| `utils/transitions.py` | PORT | MoviePy transition effects. | Timeline transition compiler allowlist. | Legacy names map. | `C,M,T`; renderer parity. |
| `utils/video_layout.py` | PORT | Pure geometry fit helper. | Timeline transform/render layout library. | Preserve geometry. | `C,M,T`; renderer parity. |
| `utils/video_qa.py` | PORT | Codec/bitrate/resolution/frame checks via subprocess. | Render QA under MediaProcessPort. | Preserve report fields. | `C,M,S`; `node-parity`. |
| `utils/visual_features.py` | REFACTOR | Scene feature extraction scaffold using FFmpeg/OpenCV-like metrics. | Versioned feature extractor/embedding Artifact; current skeleton is not full V2 intelligence. | Optional V1 matching input. | `C,M,S`; benchmark/quality gate. |
| `utils/warnings.py` | PORT | Accumulates structured warnings. | NodeResult warnings/event-safe codes. | Map existing warnings. | `C,A`; target error/warning tests. |

## 7. Workflow modules

| Module | Class | Verified current behavior | Target and reason | Compatibility | Tests / removal |
|---|---|---|---|---|---|
| `workflow/__init__.py` | REUSE | Legacy workflow facade. | Keep imports. | Legacy only. | `C`; `legacy-retire`. |
| `workflow/errors.py` | PORT | Workflow validation/load errors. | Canonical command/Pipeline validation errors. | Translate to safe API errors. | `C,A`; target validation tests. |
| `workflow/load.py` | WRAP | `yaml.safe_load`, file/config-path resolution and typed validation. | Legacy import translator; V2 Product API accepts validated structured command, not arbitrary file path. | CLI/YAML preserved. | `C,A,S`; `legacy-retire`. |
| `workflow/merge.py` | WRAP | Merges CLI/YAML/Settings and aliases/params. | Preserve precedence in compatibility translator; V2 snapshot is explicit. | CLI > YAML > Settings. | `C,A`; `legacy-retire`. |
| `workflow/schema.py` | WRAP | Extra-forbid JobConfig/steps/large allowlisted JobParams schema. | Freeze compatibility schema; map allowed fields to Pipeline/Render/provider config. | Unknown/alias behavior preserved. | `C,A,S`; `legacy-retire`. |

## 8. Deployment, plugins, tests and CI

| Area | Class | Verified evidence | Target/compatibility | Tests / removal |
|---|---|---|---|---|
| `Dockerfile` | PORT | Multi-stage CPU/GPU targets, system FFmpeg, non-root UID 10001, Python 3.12 due ML wheel support. | Port non-root/stages; version choice OQ-13; retain frozen V1 image. | Build/run/UID/codec/CPU-GPU matrix; remove only with archived rollback image. |
| `docker-compose.yml` | REPLACE | Multiple worker services expose independent daemon/LocalTaskQueue state; shared output volume but no shared broker. Optional MinIO uses insecure dev defaults/image choices. | V2 Compose has PostgreSQL/Redis/private MinIO/shared queue; V1 daemon remains regression profile. | Shared-consumer/crash/storage/security tests. |
| `.github/workflows/ci.yml` | PORT | Ruff/mypy; Python 3.10–3.13 tests; 90% coverage on 3.11; smoke, plugin, media, integration, Bandit/pip-audit. | Preserve useful matrix/regressions; add V2 contracts; review B404/B603 and Pillow exceptions. | CI required checks with scoped security exception owner/expiry. |
| `.github/workflows/publish.yml` | IGNORE | Publishes upstream package/release. | V2 release topology/name undecided; keep reference only. | New release ADR before use. |
| `tests/` | REUSE | Broad unit/integration/security/coverage suite including queue, checkpoint, API, artifacts, render and providers. | Run unchanged against frozen V1; port fixtures/golden behavior, do not rewrite tests to hide drift. | Legacy-retire only after archived evidence/equivalent coverage. |
| `examples/plugins/timeline_export` | PORT | Exports Jianying/optional OTIO timeline as plugin. | Useful interoperability reference after canonical Timeline exists; never auto-load production. | Plugin smoke + Timeline mapping; feature not baseline blocker. |
| other `examples/plugins/*` | IGNORE | Demonstrate arbitrary entry-point extensions (watermark/research/template). | Security/reference only until trusted extension ADR. | Legacy smoke; `never-product`. |

## 9. Verified risk/assumption summary

1. V1 is not a shared distributed queue: LocalTaskQueue is per-process ThreadPoolExecutor + JSON state.
2. Distributed render is optional/best-effort, health-gated, duration-thresholded and falls back local; shared input wiring is out of scope upstream.
3. V1 checkpointing is real but step-granular JSON and not a product human-review state machine.
4. StorageBackend has valuable Local/S3 abstraction/path guards, while TaskResult/Context still leak filesystem paths.
5. TTS and Vision have protocols; LLM/research registry interfaces are less uniform; there is no unified V1 Embedding provider port.
6. Scene detection, ASR fallback, matching, rendering, subtitles, FFmpeg and QA are implemented in source and covered by tests—not merely README claims.
7. Subprocess audit found argv-list calls and no `shell=True`/`os.system`, but calls are scattered and CI Bandit skips B404/B603.
8. Plugin entry points execute arbitrary Python and therefore cannot be production default.
9. Docker is non-root; Compose/MinIO examples are not production security architecture.
10. V1.1.0 materially changed 234 files versus v1.0.0, including FunASR, FFmpeg helper, visual features, timeline export plugin, API/worker/reliability and many tests; upstream sync must be module-aware.
