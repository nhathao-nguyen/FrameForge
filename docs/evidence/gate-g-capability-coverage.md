# Gate G P5/P6 capability coverage

Status: Gate G local implementation and acceptance complete for the scoped P5/P6 rows. The new
`Txxx-S1` labels are subtasks of existing IDs; no existing task was renumbered or replaced.
`DEFER` rows from the matrix are intentionally excluded.

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
| Rendering/deliverable QA | P5/REIMPLEMENT | T531/T531-S1 | Go `internal/render` | real FFmpeg output, ffprobe non-empty/dimension/codec/duration/audio QA, incomplete output rejected | implemented |
| Clips/shorts export | P5/REIMPLEMENT | T531-S1 | render/export boundary | real FFmpeg selection, codec/profile QA and checksum manifest | implemented |
| Subject-aware auto-reframe | P6/IMPROVE | T532-S1 | `nh_media.video.reframe` | missing subject boxes fails; typed intermediate/reuse fingerprint | implemented |
| 16:9 / 9:16 / 1:1 reuse | P6/REIMPLEMENT | T532 | Go profiles | three real outputs reuse source fingerprint/no provider calls | implemented |

## Intentionally deferred

Karaoke/word highlighting, OCR, subtitle-from-image detection, object tracking, broad video
understanding, semantic search/index service, inpainting/removal, batch, scheduling and distributed
rendering remain `DEFER` under the current matrix. They are not Gate G omissions.

## Reconciliation result

The previously implicit P5/P6 rows are now assigned to T512-S1, T520-S1, T523-S1, T524-S1,
T530-S1, T531-S1 and T532-S1. Their code/tests stay within existing Gate G boundaries and do not
introduce a new architecture or any Movie Narrator dependency. The local evidence is deterministic
and provider-free; it does not claim external provider quality or production ML coverage.
