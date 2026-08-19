# Real provider capability matrix — 2026-08-19

This matrix separates executable local evidence from API readiness. A `PASS` means the
capability was called through the typed provider port and produced/consumed a real result. A
`BLOCKED` API row means the adapter boundary is implemented and the policy is safe, but no
owner-supplied credential was available for a live remote call. It is not an API quality claim.

## Provider matrix

| Capability | LOCAL execution | API mode | Evidence / notes |
|---|---|---|---|
| LLM | PASS — Ollama `qwen2.5:3b` | BLOCKED — `OPENAI_API_KEY` unavailable | [local smoke](real-local-provider-smoke-20260819-r2.json), [API smoke](real-api-provider-smoke-20260819.json) |
| VLM | PASS — Ollama `moondream` with real JPEG keyframes | BLOCKED — credential unavailable | Local smoke and `analyze_scenes` in [r8](real-local-gate-h-live-20260819-r8.json) |
| TTS | PASS — Windows SAPI via `pyttsx3` | BLOCKED — credential unavailable | Real WAV narration, then consumed by ASR/alignment/render |
| ASR | PASS — `faster-whisper` `tiny.en`, CPU/int8 | BLOCKED — credential unavailable | Real narration transcription and segment artifact in r8 |
| Alignment | PASS — real ASR output aligned to generated narration | BLOCKED as dependent on remote ASR/TTS/LLM | r8 `timing_alignment` and completed `align_audio` step |
| Text embeddings | PASS — Ollama `nomic-embed-text`, 768 dimensions | BLOCKED — credential unavailable | Local smoke and r8 `media_embeddings` |
| Research/script/translation | PASS — real local LLM path through typed ports | BLOCKED — credential unavailable | r8 `research_metadata`, `script_version`, `translated_subtitles` |
| Scene detection/features | PASS — reviewed FFmpeg against uploaded MP4 | N/A | Media capability, not a remote model claim |
| Visual/matching | PASS — VLM descriptions + text embedding + persisted candidate evaluation/selection | BLOCKED — dependent on remote VLM/embedding/LLM | r8 `scene_analysis`, `media_embeddings`, matching artifacts |
| Character/coverage/candidate selection | PASS — product pipeline artifacts completed | BLOCKED — dependent on remote AI rows if switched | r8 `character_analysis`, `coverage_report`, `candidate_evaluations` |
| Reference style analysis | CONTRACT PASS — bounded abstract style traits/evidence | BLOCKED for remote model enhancement | No reference footage is copied or used as a runtime dependency |
| Timeline/mix/render/QA/download | PASS — real Go worker, FFmpeg, ffprobe, MinIO signed downloads | N/A | r8: Timeline approved, render completed, 25/25 steps, all download hashes match |

## Local configuration and runtime boundary

- Provider selection is immutable per Job through `params.provider_policy`; the worker receives
  the snapshot and resolves adapters inside the worker boundary.
- Durable commands contain adapter/model/deployment metadata and opaque `key_ref` values only.
  Credential values, cookies, media bytes and local paths are not provider policy fields.
- LOCAL evidence used Ollama at loopback, Windows SAPI and CPU/int8 faster-whisper. The VLM model
  used the available NVIDIA GPU; ASR used CPU/int8. Render evidence used the reviewed FFmpeg/
  ffprobe pair and the existing H.264/AAC profile.
- The legal-safe corpus is generated color-card video, generated SAPI narration, generated sine
  music and generated subtitle text. It contains no third-party footage, image, music or voice.

## API and future WEB_SESSION boundary

The API policy is [provider-policy-real-api.json](../../tools/provider-policy-real-api.json). It
uses `key_ref: env:OPENAI_API_KEY`; the value is resolved only inside the isolated provider
adapter. The current smoke intentionally performs no network call and records `BLOCKED` for all
five API capability rows because the credential is absent.

Future `WEB_SESSION` access is a separate, explicit adapter boundary and is not implemented or
used by this evidence. Browser cookies/session tokens must remain in an owner-controlled secret
store, never enter frontend state, Job params, Redis events, Artifact metadata, logs or model
prompts. Any future adapter needs explicit allowlisting, scope, expiry, redaction and a separate
consent/policy gate; it must not be a fallback that silently changes provider mode.

## Acceptance anchor

[real-local-gate-h-live-20260819-r8.json](real-local-gate-h-live-20260819-r8.json) is the
authoritative full Product API trace for this continuation: upload to private MinIO, PostgreSQL
asset validation, Redis-dispatched Go/Python workers, real local AI calls, TimelineVersion
approval, mix/render/QA, clip export, and signed artifact downloads. It reports
`trace_complete: true`, `step_count: 25`, `completed_step_count: 25`, and zero SHA-256 download
mismatches.
