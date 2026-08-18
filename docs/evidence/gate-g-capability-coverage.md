# Gate G P5/P6 capability coverage

Status: Gate G local implementation and acceptance complete for the scoped P5/P6 rows. Immutable
`movie_recap` v5 dispatches the listed built-in capabilities through the actual Redis Python/Go
worker path and a 25-step Product API E2E. The `Txxx-S1` labels are subtasks of existing IDs; no
existing task was renumbered or replaced. `DEFER` rows from the matrix are intentionally excluded.

| Capability | Phase/class | Existing task/subtask | Native target | Acceptance evidence | State |
|---|---|---|---|---|---|
| External research/metadata | P5/REIMPLEMENT | T510 | `nh_media.generation.research` | typed input/provider/prompt provenance | implemented |
| Script generation | P5/REIMPLEMENT | T510 | `nh_media.generation.script` | structured immutable ScriptVersion proposal/fallback | implemented |
| Narrator perspective/control | P5/IMPROVE | T510 | `ScriptStyle` | perspective/control/density snapshot survives rerun | implemented |
| Provider diversity/conformance | P5/ADOPT-CONCEPT | T500 | `nh_media.providers` | typed five-kind success/error/cancel/timeout/fallback/redaction/provenance suite | implemented |
| TTS/narration/cache | P5/REIMPLEMENT | T511 | `nh_media.speech.tts` | typed provider WAV ProducedBlob consumption, measured duration, voice/config/style fingerprint→Artifact | implemented |
| ASR/backend selection | P5/REIMPLEMENT+ADOPT | T512 | `nh_media.speech.asr` | typed provider transcript/word/speaker timing, explicit fake backend and precision contract | implemented |
| Audio/script alignment | P5/REIMPLEMENT | T512 | `nh_media.speech.alignment` | monotonic timing, drift tolerance and provenance | implemented |
| Subtitle generation | P5/REIMPLEMENT | T512-S1 | `nh_media.subtitles.generate` | immutable cues plus SRT/VTT/ASS | implemented |
| Subtitle translation | P5/REIMPLEMENT | T512-S1 | `nh_media.subtitles.translate` | target-language/provider/glossary provenance and timing preservation | implemented |
| Bilingual subtitles | P5/REIMPLEMENT | T512-S1 | subtitle track composition | both languages retain cue timing and QA | implemented |
| Subtitle QA | P5/REIMPLEMENT | T512-S1 | `nh_media.subtitles.qa` | CPS/overlap/line-length/duration report | implemented |
| Scene detection/features | P5/REIMPLEMENT | T520 | `nh_media.video.scenes` | reviewed FFmpeg/ffprobe cuts plus frame-derived luma/motion/quality/keyframe refs and source revision | implemented |
| Scene filtering | P6/ADOPT-CONCEPT | T520-S1 | `nh_media.matching.filters` | versioned quality/dark/intro reasons, no source mutation | implemented |
| VLM scene analysis | P5/REIMPLEMENT | T521 | `nh_media.vision.captioning` | per-scene keyframe/source/model provenance and degraded items | implemented |
| Character/appearance | P6/IMPROVE | T522 | `nh_media.vision.characters` | cluster proposals preserve confirmed identities | implemented |
| Text embeddings | P6/REIMPLEMENT | T523 | `nh_media.matching.embeddings` | provider-produced vectors, model/dimension/metric/normalization index and reload boundary | implemented |
| Visual/multimodal embeddings | P6/IMPROVE | T523-S1 | `nh_media.vision.embeddings` | modality items share port; model-space mismatch fails closed | implemented |
| Semantic matching | P6/IMPROVE | T524 | `nh_media.matching.proposals` | score components, deterministic diversity/evidence | implemented |
| Coverage feedback | P6/IMPROVE | T524-S1 | `nh_media.evaluation.coverage` | explicit segment coverage/rationale, approved script immutable | implemented |
| Candidate generation/evaluation/selection | P6/IMPROVE | T525 | `nh_media.evaluation.candidates` | immutable candidates, deterministic policy and user selection | implemented |
| ReferenceStyleAnalysis | P6/IMPROVE | T526 | `nh_media.video.reference_style` | consent/scope, abstract traits, no copied footage | implemented |
| BGM selection/rights | P5/REIMPLEMENT | T530-S1 | typed BGM/audio policy | rights metadata and provenance validation | implemented |
| Loudness/ducking/audio mix | P5/IMPROVE | T530-S1 | `AudioMixPolicy`/report | LUFS/true-peak/fades/ducking report | implemented |
| Timeline/video composition | P4–P5/REIMPLEMENT | T530 | Go `internal/render` | canonical TimelineVersion multi-clip plan with allowlisted transitions/audio/subtitles and no provider/rematch calls | implemented |
| Templates/transitions/text effects | P5/REIMPLEMENT | T530-S1 | Go compiler allowlist | no raw FFmpeg args; unsafe effect/text rejected | implemented |
| Rendering/deliverable QA | P5/REIMPLEMENT | T531/T531-S1 | Go `internal/render` | real FFmpeg output, ffprobe non-empty/dimension/codec/duration/audio QA, real blackdetect/silencedetect policy, incomplete output rejected | PASS |
| Clips/shorts export | P5/REIMPLEMENT | T531-S1 | render/export boundary | real FFmpeg selection, content-safe clip range, codec/profile QA and checksum manifest | PASS |
| Subject-aware auto-reframe | P6/IMPROVE | T532-S1 | `nh_media.video.reframe` + Go renderer | missing subject boxes fails; coordinate-sensitive crop plan/fingerprint changes rendered pixels | PASS |
| 16:9 / 9:16 / 1:1 reuse | P6/REIMPLEMENT | T532 | Go profiles | three real outputs reuse source fingerprint/no provider calls | PASS |

## Intentionally deferred

Karaoke/word highlighting, OCR, subtitle-from-image detection, object tracking, broad video
understanding, semantic search/index service, inpainting/removal, batch, scheduling and distributed
rendering remain `DEFER` under the current matrix. They are not Gate G omissions.

## Reconciliation result

The previously implicit P5/P6 rows are now assigned to T512-S1, T520-S1, T523-S1, T524-S1,
T530-S1, T531-S1 and T532-S1. Their code/tests stay within existing Gate G boundaries and do not
introduce a new architecture or any Movie Narrator dependency. The local evidence is deterministic
and provider-free; it does not claim external provider quality or production ML coverage.

## Runtime closure

Package-level capability functions are reached by the production-shaped worker boundary. The
Product API snapshots `movie_recap` v5, advances dependency frontiers, maps execution classes to
system/AI/ML/probe/media/render queues and includes completed upstream Artifact refs in each command.
Python dispatches research, script, TTS, ASR, VLM, embeddings, matching, candidate, subtitle,
Timeline-build and QA nodes; Go dispatches source probe/preparation, audio mix, Timeline render,
deliverable QA and clip export. Internal authenticated transfer endpoints issue scoped direct object
transfers, so Redis carries refs and checksums rather than media bytes or signed URLs.

## Prompt 6 final audit

Verified 2026-08-18 on `implementation/bootstrap` with the pinned local toolchain and the
acceptance runner `tools/accept-gate-g.ps1`. Every scoped Gate G row below is PASS; this is
deterministic/local-provider and reviewed-FFmpeg evidence, not external provider quality, GPU
model quality, public production, or Prompt 7/T600–T605 evidence.

| Task | Status | Current proof |
|---|---|---|
| T500 | PASS | Five typed provider ports use `ProviderResolver`; retry/backoff, Retry-After, circuit, auth/policy/permanent/cancel taxonomy, fallback and redaction tests pass through node-path runtime tests. |
| T510 | PASS | Research, script generation and `ScriptStyle` perspective/control/density execute through the resolver boundary with immutable/provenance tests; provider errors are not masked by a node-local fallback. |
| T511 | PASS | Resolver-selected TTS produces typed WAV/`ProducedBlob` output with measured duration and voice/config/style fingerprint. |
| T512 / T512-S1 | PASS | Resolver-selected ASR and alignment validate word/speaker timing; subtitle generation, translation, bilingual composition and CPS/overlap/line-length QA pass. |
| T520 / T520-S1 | PASS | Reviewed FFmpeg/ffprobe scene detection and frame-derived features are exercised on real media; filter reasons preserve inputs. |
| T521 | PASS | VLM scene analysis uses the resolver-selected typed provider and preserves scene/keyframe/source/model provenance. |
| T522 | PASS | Character appearance proposals preserve confirmed identities and remain explicitly proposal-scoped. |
| T523 / T523-S1 | PASS | Text and visual/multimodal embeddings consume resolver-produced typed vectors; model-space mismatch and reload boundaries fail closed. |
| T524 / T524-S1 | PASS | Match proposals, deterministic diversity/evidence and coverage rationale execute without mutating the approved script. |
| T525 | PASS | Candidate generation/evaluation/selection uses immutable candidates, deterministic policy and explicit user selection. |
| T526 | PASS | Reference style analysis enforces consent/scope and stores abstract traits without copied footage. |
| T530 / T530-S1 | PASS | Go compiles canonical TimelineVersion/Profile data with allowlisted transitions/audio/subtitles/BGM rights; no provider/rematch decision or raw FFmpeg argv crosses the boundary. |
| T531 / T531-S1 | PASS | Real render/ffprobe output passes dimensions/codecs/duration/audio plus black/silence content policy; failed QA cannot commit; clip export chooses a QA-safe range and emits checksum manifest. |
| T532 / T532-S1 | PASS | 16:9/9:16/1:1 outputs reuse the upstream source fingerprint; subject-aware coordinates change crop args, plan fingerprints and real output SHA-256/pixels, while missing/invalid tracks fail closed. |
