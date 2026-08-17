# Upstream Capability Coverage Matrix

## Scope and rules

This matrix preserves useful knowledge observed in Movie Narrator while defining independently
authored NH-Media targets. Upstream is research/reference-only and every row has runtime dependency
on Movie Narrator: **NONE**.

Phases refer to `10-DEVELOPMENT-ROADMAP.md`. Acceptance means NH-Media contract evidence; an
upstream comparison is optional unless the row explicitly calls for a recorded reference fixture.

## Product and workflow capabilities

| Capability | Upstream purpose | NH-Media equivalent | Class | Target service/module | Phase | Dependencies | Rationale | Acceptance criteria |
|---|---|---|---|---|---|---|---|---|
| Media metadata/probe | Resolve source and inspect streams | Asset validation/probe Job | REIMPLEMENT | Go media worker `media/probe` | P2 | Storage, FFmpeg port | Core ingest requirement | Checksums, MIME/magic, stream/duration limits and quarantine pass |
| External title/plot research | Enrich movie context, including TMDB concepts | `ResearchAnalysis` with provider provenance | REIMPLEMENT | `nh_media.generation.research` | P5 | Provider ports, Project metadata | Useful but optional workflow input | Typed result, timeout/fallback and provenance tests pass |
| Script generation | Generate narration script | Script + immutable ScriptVersion proposals | REIMPLEMENT | `nh_media.generation.script` | P5 | LLM port, review domain | Core user-visible capability | Structured schema, quality rubric, retry and review tests pass |
| Narrator perspective/control | Control voice and narrative viewpoint | Versioned ScriptStyle/NarrationPolicy | IMPROVE | contracts + `nh_media.generation` | P5 | Script schema | Make behavior explicit/editable | Perspective constraints survive generation, edit and rerun |
| Candidate race | Generate/evaluate alternatives | GenerationCandidate, EvaluationResult, CandidateSelectionPolicy | IMPROVE | `nh_media.evaluation` | P6 | Candidate domain, provider usage | Prevent one-output-only architecture | Multiple candidates persist; deterministic policy selects with evidence |
| Reference-video imitation | Derive production style | ReferenceStyleAnalysis of abstract traits | IMPROVE | `nh_media.video.reference_style` | P6 | Scene/audio/subtitle analysis | Retain style insight without copying footage | Extracts pacing/shot/narration/subtitle/framing/rhythm metrics with provenance |
| Workflow presets/templates | Reusable configuration bundles | Versioned Workflow + RenderProfile templates | REIMPLEMENT | Product catalog/contracts | P4 | Pipeline and profile domain | Product-native reusable recipes | Versioning, validation and deterministic snapshot tests pass |
| Batch processing | Submit multiple independent jobs | Bounded BatchSubmission command | DEFER | Product application service | P7 | Stable Job API, quotas | Useful after single-job operations mature | Bounded/idempotent batch with per-job status and cancel policy |
| Scheduling | Submit work at a future/recurring time | Schedule aggregate issuing authorized Job commands | DEFER | Product control plane | P7 | Auth, Job API, timezone policy | Product capability but not first slice | Durable schedule, dedupe, timezone and missed-run policy tests pass |

## Speech, language and subtitle capabilities

| Capability | Upstream purpose | NH-Media equivalent | Class | Target service/module | Phase | Dependencies | Rationale | Acceptance criteria |
|---|---|---|---|---|---|---|---|---|
| TTS | Create narration audio | Narration via TTSProvider | REIMPLEMENT | `nh_media.speech.tts` | P5 | Provider ports, Artifact commit | Core pipeline capability | Voice snapshot, cache fingerprint, cancel and audio probe pass |
| Edge/OpenAI/MiMo TTS concepts | Offer provider choices | Explicit reviewed TTS adapters | ADOPT-CONCEPT | `nh_media.providers.tts` | P5 | Provider policy/secrets | Provider diversity, not source reuse | Each accepted adapter passes one conformance suite; Edge limited by policy |
| ASR | Transcribe source/narration | Transcript Artifact via ASRProvider | REIMPLEMENT | `nh_media.speech.asr` | P5 | Audio Artifact, ML worker | Core alignment/intelligence input | Segment schema, provenance, cancellation and corpus tolerance pass |
| WhisperX/faster-whisper/FunASR selection | Backend/fallback options | Capability-resolved ASR adapters | ADOPT-CONCEPT | `nh_media.providers.asr` | P5 | Worker dependency matrix | Preserve backend flexibility | Explicit capability/fallback; no silent precision loss |
| Audio/script alignment | Time narration/script | Alignment Artifact tied to exact versions | REIMPLEMENT | `nh_media.speech.alignment` | P5 | Narration, ASR | Required for subtitles/timeline | Timing monotonicity, drift, resume and provenance tests pass |
| Subtitle generation | Create timed text files | SubtitleTrack + export Artifacts | REIMPLEMENT | `nh_media.subtitles.generate` | P5 | Alignment, Timeline schema | Core output capability | Cue limits/timing/style and SRT/VTT/ASS export tests pass |
| Subtitle translation | Localize timed text | Immutable localized timed-text Artifact | REIMPLEMENT | `nh_media.subtitles.translate` | P5 | LLM/translation provider | Important localization capability | Language/provenance/glossary/retry and timing preservation pass |
| Bilingual subtitles | Combine languages | Multi-language SubtitleTracks/profile policy | REIMPLEMENT | Timeline/subtitle renderer | P5 | Translation, Timeline | Retain useful output mode | Both languages retain timing, safe layout and export integrity |
| Karaoke/word highlighting | Dynamic subtitle presentation | Word-timed style schema/render effect | DEFER | Timeline style + renderer | P6 | Word timing, renderer | Valuable but not first E2E | Active-word timing and safe rendering pass |
| Subtitle QA | Detect CPS/overlap/layout problems | Subtitle QA report Artifact | REIMPLEMENT | `nh_media.subtitles.qa` | P5 | Subtitle output | User-visible quality gate | CPS, overlap, line length and safe-area corpus passes |

## Vision, analysis and matching

| Capability | Upstream purpose | NH-Media equivalent | Class | Target service/module | Phase | Dependencies | Rationale | Acceptance criteria |
|---|---|---|---|---|---|---|---|---|
| Scene detection | Segment source video | Scene records tied to source Artifact revision | REIMPLEMENT | `nh_media.video.scenes` | P5 | Probe/proxy, ML worker | Core matching input | Deterministic ranges, zero-scene policy and corpus tolerance pass |
| Scene filtering | Exclude dark/intro/poor candidates | Versioned candidate filters | ADOPT-CONCEPT | `nh_media.matching.filters` | P6 | Scene features | Useful concept; thresholds need NH design | Filter components and reasons are traceable and tested |
| VLM captioning | Describe frames/scenes | Analysis records via VLMProvider | REIMPLEMENT | `nh_media.vision.captioning` | P5 | Keyframes, provider ports | Multimodal understanding | Per-item provenance, partial policy and malformed-output tests pass |
| Semantic matching | Map script regions to footage | MatchProposal with score components | IMPROVE | `nh_media.matching` | P6 | Script, scenes, embeddings, alignment | Decouple proposal from Timeline and expose reasoning | Determinism, coverage, diversity and user override tests pass |
| Text embeddings | Semantic similarity | Versioned Embedding Artifact/index | REIMPLEMENT | `nh_media.matching.embeddings` | P6 | EmbeddingProvider | Useful matching primitive | Model/dimension/normalization fingerprint and reload tests pass |
| Visual embeddings | Visual similarity | Multimodal feature/index Artifact | IMPROVE | `nh_media.vision.embeddings` | P6 | Keyframes, EmbeddingProvider | Expand beyond text-only matching | Visual corpus benchmark and model-space validation pass |
| Character detection/tracking/clustering | Identify recurring people | Character/Appearance proposals | IMPROVE | `nh_media.vision.characters` | P6 | Scenes, model policy | Better footage selection; user confirmation required | Tracking/cluster provenance and confirmed-user override preservation pass |
| Coverage feedback | Detect script without matching footage | CoverageAnalysis + optional ScriptVersion proposal | IMPROVE | `nh_media.evaluation.coverage` | P6 | MatchProposal, Script | Editor-aware generation | No mutation of approved script; segment-level rationale persists |
| General video understanding | Broader semantic analysis | Versioned Analysis framework | DEFER | `nh_media.video.analysis` | P7 | Core scenes/providers | Future platform scope | New analysis types plug in without schema/runtime redesign |
| OCR/text detection | Detect on-screen text | TextRegion/TextTrack Analysis | DEFER | `nh_media.vision.ocr` | P7 | Frame sampling/tracking | Future editing/search capability | Temporal boxes, confidence, language and provenance tests pass |
| Subtitle detection from image | Recover burned-in text regions/timing | SubtitleDetection Analysis | DEFER | `nh_media.vision.subtitles` | P7 | OCR/tracking | Valid future capability; separate from ASR | Cue coverage and visual-region accuracy meet corpus threshold |
| Object detection/tracking | Understand/track subjects | ObjectTrack Analysis | DEFER | `nh_media.vision.objects` | P7 | Vision models | Enables reframe/removal/search | Stable temporal IDs, boxes and model provenance pass |
| Semantic search/indexing | Search media by meaning | MultimodalIndex + Search API | DEFER | analysis/index service | P7 | Embeddings, storage decision | Useful after analysis scale exists | Query benchmark, ACL isolation and rebuildability pass |

## Audio, composition and rendering

| Capability | Upstream purpose | NH-Media equivalent | Class | Target service/module | Phase | Dependencies | Rationale | Acceptance criteria |
|---|---|---|---|---|---|---|---|---|
| BGM handling | Select/place background music | Timeline music track and BGM policy | REIMPLEMENT | Timeline builder + Go media worker | P5 | Asset, Timeline | Core production feature | Selection provenance, duration, rights metadata field and mix pass |
| Loudness/ducking/audio mix | Balance narration/music | Versioned AudioMix policy + report | IMPROVE | Go media worker `audio` | P5 | Timeline/audio artifacts | Explicit LUFS/peak quality | Loudness, true peak, fades and ducking fixtures pass |
| Video composition | Combine footage/audio/subtitles | Timeline compiler | REIMPLEMENT | Go media worker `render/compiler` | P4–P5 | TimelineVersion | Independent render foundation | Deterministic plan uses exact refs and no match-time decisions |
| FFmpeg pipelines | Execute media transforms | Reviewed MediaProcessPort | ADOPT-CONCEPT | Go media worker | P3–P5 | Sandbox, toolchain | Use FFmpeg as tool, not upstream orchestration | Allowlisted argv, progress, timeout/cancel/process-tree tests pass |
| Rendering | Produce deliverable | Render Job from TimelineVersion/Profile | REIMPLEMENT | Go media worker `render` | P5 | Compiler, storage | Core product output | Valid streams/profile/QA; incomplete files never canonical |
| Clips/shorts export | Create derivative clips | Export Job/Artifact from Timeline/selection | REIMPLEMENT | Go media worker `export` | P5 | Timeline/render | Useful output family | Exact selection, codec, checksum and manifest tests pass |
| Auto-reframe/aspect conversion | Adapt framing for vertical/square | Profile-specific ReframeAnalysis/transform | IMPROVE | `nh_media.video.reframe` + media worker | P6 | Object/face tracking optional | Avoid center-crop-only architecture | Smooth subject retention and safe-area benchmark pass |
| Video text/watermark/object removal | Remove selected regions where appropriate | RemovalProposal + inpainting pipeline | DEFER | `nh_media.video.inpainting` | P7 | OCR/object tracking, policy | Broader product direction; high quality/safety scope | Explicit user selection/rights policy, temporal consistency and QA pass |
| Templates/transitions/text effects | Reusable visual style | RenderProfile/Timeline style catalogs | REIMPLEMENT | contracts + media worker | P5 | Timeline compiler | Product-native edit/render behavior | Allowlisted deterministic effects, safe text/font tests pass |

## Reliability, storage, API and operations

| Capability | Upstream purpose | NH-Media equivalent | Class | Target service/module | Phase | Dependencies | Rationale | Acceptance criteria |
|---|---|---|---|---|---|---|---|---|
| Recorded upstream fixtures/outputs | Observe external behavior and failure modes | Dated research evidence and optional comparison corpus | REFERENCE | `docs/baselines` / research fixtures | P0, refresh P7 | Provenance and license review | Retain knowledge without making behavior/source a product contract | Normal NH-Media tests run without upstream; intentional divergences are documented |
| Async task processing | Avoid blocking requests | Durable Job/PipelineRun/JobStep | REIMPLEMENT | Go Product API/orchestrator | P2–P3 | PostgreSQL, Redis | Core architecture | API returns bounded async Job; state remains durable after restart |
| Checkpoint/resume | Continue after partial work | Versioned checkpoint refs/fingerprints | IMPROVE | orchestration + Artifact storage | P3 | Job state, storage | Need durable selective invalidation | Crash-at-boundary and stale-input tests pass |
| Retry/backoff | Recover transient errors | Error taxonomy + node/provider retry policy | ADOPT-CONCEPT | orchestration/providers | P3 | JobStep attempts | Useful reliability concept | Only declared transient errors retry with budget/jitter |
| DLQ/replay | Retain exhausted work | DeadLetter + new-Job replay | REIMPLEMENT | Product control plane | P3 | Job state/events | Durable operator workflow | Exhaustion/replay/audit/authorization tests pass |
| Provider fallback/circuit breaker | Survive provider outages | Per-configuration policy/provenance | IMPROVE | provider resolver | P5 | Provider ports | Make fallback explicit and auditable | Error matrix, breaker and per-call provenance pass |
| Artifact storage | Store outputs locally/S3 | StoragePort + Asset/Artifact domain | REIMPLEMENT | Product/storage adapters | P2 | PostgreSQL/object storage | Core boundary | Local/S3-compatible conformance, checksum and ACL pass |
| Presigned uploads/downloads | Transfer large media | Scoped UploadSession/download capability | REIMPLEMENT | Product API/storage | P2 | Auth/storage | Keep bytes out of API | Multipart/TTL/method/key/cross-Workspace tests pass |
| Health/readiness/metrics | Operate services | Product/worker health and observability | REIMPLEMENT | all services | P1–P7 | Service boundaries | Required operations | Failure-specific readiness, bounded labels and redaction pass |
| REST API | Submit/inspect tasks | NH-Media-native `/api/v1` resources | REIMPLEMENT | Go Product API | P1–P4 | Domain/auth/contracts | Upstream routes are not product requirements | OpenAPI, auth, idempotency and canonical state tests pass |
| CLI | Local control | Optional NH-Media admin/client CLI | DEFER | client tooling | P7 | Stable Product API | Convenience only; no upstream CLI compatibility | Calls Product API with product auth and native commands |
| Distributed rendering | Remote worker execution | Capability queues + leases + Artifact refs | ADOPT-CONCEPT | worker scheduler | P7 | Single-host correctness/load evidence | Architecture supports future scale | Multi-host duplicate/lease/failure tests pass before enablement |
| Local filesystem task database | Persist upstream tasks | PostgreSQL product state | IGNORE | none | n/a | n/a | Cannot satisfy product durability/ownership | Absence verified; no JSON task store in product graph |
| Unrestricted Python plugins | Extend pipeline via arbitrary entry points | Reviewed adapters/extension manifests | IGNORE | none in product default | n/a | Security and extension policy | Arbitrary code violates trust and secret boundaries | CI/release scan proves no auto-loading path |
| Upstream CLI/REST compatibility | Preserve external clients | None | IGNORE | none | n/a | n/a | NH-Media has native API and no compatibility requirement | No compatibility routes, packages or contract fixtures exist |

## Coverage conclusion

Candidate race and ReferenceStyleAnalysis are explicitly retained as improved later capabilities.
Scheduling and distributed execution remain possible without burdening the first slice. All ignored
items have explicit security/product-boundary rationale. No row requires Movie Narrator at runtime,
build, deployment or normal test time.
