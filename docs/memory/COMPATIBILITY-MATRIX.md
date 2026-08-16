---
last_verified: 2026-08-16
source: ../09-MIGRATION-FROM-UPSTREAM.md; ../UPSTREAM-MODULE-AUDIT.md; ../GLOSSARY.md; ../IMPLEMENTATION-ORDER.md
owner: repository owner / compatibility owner
---

# V1/V2 compatibility matrix

This is a target contract and evidence checklist, not a claim that the baseline has already run.
OQ-14 is decided as Option A. T003 must now finalize the executable profile and evidence for every
verified V1 CLI/REST surface. V1 remains unchanged in the reference tree at this stage.

| V1 surface/behavior | V2 representation | Adapter/gateway rule | Regression evidence | Removal criteria |
|---|---|---|---|---|
| CLI create/config/start/resume | Product Job commands and compatibility CLI | map at gateway; keep V1 aliases | T003, T360 | selected clients migrated + approved deprecation |
| REST tasks/status/result/artifacts | `/api/v1` Product resources | V1 listener/gateway maps Task→Job | T003, T350, T360 | route usage/parity and owner approval |
| Batch/schedule/DLQ routes | Product orchestration or explicitly deprecated surface | preserve only OQ-14-selected behavior | T003, T361 | usage evidence + migration guide |
| `TaskStatus` (`pending`, `running`, `retrying`, `completed`, `failed`, `cancelled`, `dead`) | Job/JobStep canonical states | map only in compatibility boundary; V2 uses Glossary states | T003, T322, T360 | replacement status/client parity |
| V1 `PipelineStatus=success` | V2 `JobStep=completed` | quote/translate only at adapter | T003/T322 | all clients consume canonical term |
| Ordered 16-step runner | PipelineRun/JobStep graph | frozen ordered compatibility pipeline behind VideoEngine | T004/T322 | V2 node parity and rollback drill |
| V1 path-based Context | Asset/Artifact refs | materialize paths only in adapter sandbox | T003/T321/T323 | no legacy path consumer and migration evidence |
| `final.mp4`, `script.md`, `matches.json`, subtitle/narration/clips | Artifact roles/download aliases | aliases remain compatibility metadata, not canonical identity | T004/T323 | alias usage zero + announced removal |
| JSON checkpoint/resume | durable Checkpoint/Attempt/Artifact state | import/map checkpoint; PostgreSQL/object storage is durable source | T004/T330 | restart/replay parity |
| V1 provider/config aliases | ProviderConfiguration + typed provider ports | no provider-specific branches in Product API | T003/T500 | provider migration and provenance parity |
| V1 FFmpeg/MoviePy rendering | Timeline compiler/media process port | allowlisted executable and sandbox only | T004/T530/T531 | native renderer parity and rollback |
| V1 plugin entry points | disabled/allowlisted isolated plugins | never auto-load untrusted plugins | security suite T600 | approved replacement policy |

## Compatibility principles

- Preserve behavior before improving quality; record intentional breaks as a dated owner-approved
  change.
- Preserve upstream license/attribution and the `movie_narrator` namespace until removal criteria.
- Test old and new contracts together; a successful V2 render does not prove compatibility.
- Rollback is always to `LegacyMovieNarratorAdapter` and the frozen V1 image/environment until the
  relevant module removal gate passes.
