# T604 — upstream research refresh drill (2026-08-18)

This is research evidence only. No upstream checkout, source file, package, image, route or
runtime was added to NH-Media. Normal build/test/deployment remains upstream-free.

## Provenance

| Field | Observation |
|---|---|
| Repository | `https://github.com/zcbacxc/movie-narrator.git` |
| Refreshed at | 2026-08-18 (UTC) |
| `HEAD` / `main` | `b332cf9411a09cd8452c7dd84bfffb380be9c621` (metadata-only `git ls-remote`) |
| License identifier | `AGPL-3.0-or-later` as declared by the upstream repository |
| Inspection sources | upstream README, repository root listing and license link; no upstream source checkout retained |

The refresh is repeatable with `tools/refresh-upstream.ps1`. That command records only the remote
ref and provenance metadata; it does not clone or execute upstream code.

## Capability observations and NH-Media disposition

| Observation | NH-Media disposition | Product change |
|---|---|---|
| Narration, subtitles, rendering, candidate race and reference-video imitation remain advertised | Existing native P5/P6 implementations and capability matrix | None |
| Upstream advertises plugins and remote inference | Plugin auto-load remains disabled by default; provider ports stay allowlisted and scoped | None |
| Upstream advertises async task processing, batch/scheduling and distributed features | Existing `DEFER` rows remain deferred until their own owner-approved tasks | None |
| Upstream documents local Ollama/provider environment configuration | NH-Media provider credentials remain server-side SecretStore refs; no upstream env names or APIs are adopted | None |
| Upstream declares Edge-TTS as unofficial/personal or non-commercial in its notice | NH-Media provider policy remains independent and provenance-aware | None |

## Independence checks

- `movie_narrator` is absent from NH-Media source imports, package manifests, images and routes.
- No upstream public API, CLI, state name or compatibility adapter was introduced.
- No NH-Media public contract or task classification changed because of this refresh.
- The upstream license observation is recorded for provenance; this file is not a legal conclusion.
