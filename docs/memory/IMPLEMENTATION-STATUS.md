---
last_verified: 2026-08-16
source: ../IMPLEMENTATION-ORDER.md; ../10-DEVELOPMENT-ROADMAP.md; CURRENT-STATE.md
owner: repository owner / task assignee
---

# Implementation status

The normative task definitions, dependencies and definitions of done are in
[`../IMPLEMENTATION-ORDER.md`](../IMPLEMENTATION-ORDER.md). This ledger intentionally records no
application implementation: T000 is complete, and only the documentation/baseline tasks T001–T005
are now eligible. Application implementation remains blocked until Gate A/Phase 0 passes.

## Status vocabulary

- `pending` — eligible only after listed prerequisites and owner approval.
- `blocked` — cannot start because a gate or hard OQ dependency is unresolved.
- `complete` — evidence and handoff are recorded.

For every row, `Evidence` is currently `none (documentation-only baseline)`, `Compatibility` is
`V1 unchanged`, and `Rollback` is `not applicable before implementation` unless the row explicitly
states the future V1 adapter requirement. Detailed task-specific evidence must replace these
defaults when a task is authorized.

| ID | Task | Dependency / status | Evidence | Compatibility / rollback | Next |
|---|---|---|---|---|---|
| T000 | Ratify specification decisions | complete — owner approved 2026-08-16 | user approval + docs/spec/memory update | V1 unchanged; docs-only | T001 |
| T001 | Record immutable upstream baseline manifest | pending — T000 complete | none | preserve frozen V1; rollback image later | T002 |
| T002 | Build reproducible Arch/uv environment matrix | blocked by T001 | none | Product/core 3.13; frozen legacy/ML 3.12 | T003 |
| T003 | Freeze V1 compatibility profile | blocked by T001–T002 | none | all verified V1 CLI/REST surfaces preserved | T004 |
| T004 | Produce V1 golden media outputs | blocked by T001–T003 | none | golden outputs protect V1 parity | T005 |
| T005 | Freeze runnable V1 rollback image | blocked by T001–T004 | none | required rollback target; no image yet | T100 |
| T100 | Create approved package skeleton | blocked by T005/Gate A; OQ-12 decided | none | V1 remains isolated; rollback to no V2 code | T101 |
| T101 | Add shared identity/time/error primitives | blocked by T100 | none | contracts must not change V1 surface | T102 |
| T102 | Add configuration and redaction boundary | blocked by T100 | none | preserve V1 config through adapter | T103 |
| T103 | Provision PostgreSQL dev service | blocked by T100 | none | no V1 state migration yet | T104 |
| T104 | Provision Redis dev service | blocked by T100 | none | no V1 queue replacement yet | T105 |
| T105 | Provision MinIO dev service | blocked by T100 | none | preserve V1 artifact compatibility | T106 |
| T106 | Implement FastAPI shell/error middleware | blocked by T100–T105 | none | no legacy route removal; rollback shell | T107 |
| T107 | Implement health/readiness endpoints | blocked by T100–T106 | none | no V1 behavior change | T108 |
| T108 | Establish V2 CI gates | blocked by T100–T107 | none | V1 regression remains required; revert CI change if needed | T200 |
| T200 | Add migration framework/schema invariants | blocked by Gate B | none | no legacy DB rewrite; downgrade plan required | T201 |
| T201 | Add identity/Workspace tables | blocked by T200 and OQ-01/OQ-06 | none | no V1 ownership inference | T202 |
| T202 | Add Workflow/Pipeline/Node tables | blocked by T200 | none | V1 graph remains adapter-owned | T203 |
| T203 | Add ProviderConfiguration/RenderProfile | blocked by T200 and OQ-05 | none | no plaintext secrets; rollback migration required | T204 |
| T204 | Add Project/Asset/upload metadata | blocked by T200 | none | no raw paths; preserve source bytes | T205 |
| T205 | Add Job/PipelineRun/JobStep tables | blocked by T200 | none | canonical states; no V1 Task rename | T206 |
| T206 | Add Artifact/checkpoint/DLQ tables | blocked by T200 | none | V1 artifacts mapped, not deleted | T207 |
| T207 | Add Script/Narration/Scene/Character tables | blocked by T200 | none | preserve V1 output aliases via adapter | T208 |
| T208 | Add Timeline/Render tables | blocked by T200 | none | TimelineVersion renderer source; migration reversible | T209 |
| T209 | Add event/outbox/idempotency tables | blocked by T200 | none | V1 event compatibility at gateway | T210 |
| T210 | Add auth context/scoped repositories | blocked by T201 and OQ-01/OQ-06 | none | cross-owner rollback/negative tests required | T220 |
| T220 | Define StoragePort/LocalStorage | blocked by Gate C | none | V1 path materialization only in adapter | T221 |
| T221 | Implement S3/MinIO adapter | blocked by T220 | none | preserve artifact aliases; fallback LocalStorage | T222 |
| T222 | Implement Artifact stage/promote/commit | blocked by T206/T220–T221 | none | atomic commit and orphan reconciliation | T223 |
| T223 | Implement presigned upload sessions | blocked by T204/T221 | none | browser direct upload; rollback session API | T224 |
| T224 | Implement validation/probe Job boundary | blocked by T205–T206 and OQ-10 | none | quarantine on failure; no raw path leakage | T230 |
| T230 | Implement Project/Asset REST resources | blocked by T204/T210/T223 | none | no large media proxy; rollback routes | T231 |
| T231 | Implement Script/ScriptVersion/Narration API | blocked by T207/T210 | none | immutable versions; preserve V1 export aliases | T232 |
| T232 | Implement Timeline schema/validator | blocked by T208 | none | renderer contract guarded; reject invalid versions | T233 |
| T233 | Implement Timeline version REST API | blocked by T208/T210/T232 and OQ-08 | none | optimistic concurrency; version rollback | T234 |
| T234 | Implement Scene/Character APIs | blocked by T207/T210 | none | proposal provenance and user override preserved | T235 |
| T235 | Implement ProviderConfiguration API | blocked by T203/T210 and OQ-05 | none | secret refs only; disable/revoke rollback | T300 |
| T300 | Implement Job/Run/Step transition service | blocked by Gate D | none | canonical states; adapter rollback required | T301 |
| T301 | Implement durable event replay/outbox publisher | blocked by T209/T300 | none | append-only replay; no lost V1 progress | T310 |
| T310 | Implement Redis QueuePort adapter | blocked by T301 and OQ-03 | none | QueuePort replacement must be reversible | T311 |
| T311 | Implement dependency scheduler | blocked by T202/T300/T310 | none | V1 ordered path remains adapter option | T312 |
| T312 | Implement worker lease/heartbeat | blocked by T300/T310 | none | crash resume and lease rollback required | T313 |
| T313 | Implement sandboxed MediaProcessPort | blocked by T312 and security gate | none | V1 executor isolated; kill/drain rollback | T320 |
| T320 | Define VideoEngine/neutral contracts | blocked by T300/T313 | none | mandatory LegacyMovieNarratorAdapter route | T321 |
| T321 | Implement legacy DTO/workspace mapper | blocked by T320 and OQ-09 | none | V1 aliases/statuses map only at adapter | T322 |
| T322 | Execute frozen 16-step V1 through adapter | blocked by T321 and T005 | none | parity and rollback to frozen V1 | T323 |
| T323 | Commit legacy outputs as Artifacts | blocked by T222/T322 | none | no deletion of V1 outputs; artifact rollback | T330 |
| T330 | Implement checkpoints/crash resume | blocked by T206/T300/T312/T323 | none | reuse committed V1 artifacts; retry safe boundary | T331 |
| T331 | Implement pause/partial/review commands | blocked by T330 and T340 | none | preserve V1 soft/strict semantics | T332 |
| T332 | Implement cancel/retry/DLQ | blocked by T300/T330 | none | no retry security/user errors; DLQ replay | T340 |
| T340 | Implement SSE progress stream | blocked by T301 and OQ-04 | none | snapshot/replay compatibility; transport rollback | T341 |
| T341 | Implement optional WebSocket transport | blocked by T340/OQ-04 | none | SSE remains fallback | T350 |
| T350 | Implement Job/Run/Step/Render commands | blocked by T230/T300/T330 | none | API state matches DB/events; route rollback | T360 |
| T360 | Implement V1 REST/CLI compatibility gateway | blocked by T003/T321–T350 and OQ-09; OQ-14 decided | none | frozen V1 contract and rollback route | T361 |
| T361 | Implement optional batch/schedule/DLQ compatibility | blocked by T360; all verified surfaces are in approved profile | none | preserve approved profile; deprecation path | T400 |
| T400 | Implement DAG validator/activation gate | blocked by Gate E and OQ-11 | none | V1 ordered adapter remains selectable | T401 |
| T401 | Implement native node runtime conformance | blocked by T400/T300/T330 | none | every native node has V1 rollback policy | T402 |
| T402 | Register built-in movie recap V2 graph | blocked by T400/T401 and OQ-11 | none | V1 graph remains available | T410 |
| T410 | Implement review orchestration backend | blocked by T331/T401 | none | user selection/version rollback | T420 |
| T420 | Implement build_timeline native node | blocked by T232/T401/T410 | none | TimelineVersion remains renderer source | T430 |
| T430 | Build frontend shell/API client | blocked by T350/T410 and OQ-04 | none | UI calls Product API only | T431 |
| T431 | Build Script review editor | blocked by T231/T410/T430 | none | preserve approved ScriptVersion; undo/version | T432 |
| T432 | Build Timeline/Scene editor slice | blocked by T233/T234/T420/T430 and OQ-08 | none | user overrides survive AI rerun | T500 |
| T433 | Build desktop client shell/remote session | blocked by T101/T106/T210/T223/T340/T430; OQ-15 decided | none | desktop is API-only; no local engine; rollback to previous client | T434 |
| T434 | Package/secure desktop release baseline | blocked by T433; OQ-15 decided | none | signed build/update rollback required; no secrets in binary | T500 |
| T500 | Implement provider ports/resolver/conformance | blocked by Gate F and OQ-05 | none | provider-agnostic nodes; revoke/fallback | T510 |
| T510 | Port LLM Script provider/node | blocked by T500/T231/T401 | none | V1 script fallback and provenance | T511 |
| T511 | Port TTS/Narration node | blocked by T500/T207/T401 | none | V1 narration fallback; artifact reuse | T512 |
| T512 | Port ASR/alignment node | blocked by T500/T207/T401 | none | V1 alignment fallback | T520 |
| T520 | Port scene detection/features | blocked by T500/T401 | none | V1 scene fallback | T521 |
| T521 | Add VLM caption adapter/node | blocked by T500/T520 | none | declared soft/hard fallback; provider rollback | T522 |
| T522 | Add Character/Appearance node | blocked by T500/T234/T401 | none | no unapproved identity inference; V1 fallback | T523 |
| T523 | Add Embedding adapter/index artifact | blocked by T500 and OQ-07 | none | Artifact index can be rebuilt; no DB blob lock-in | T524 |
| T524 | Refactor multimodal matching/coverage | blocked by T520–T523/T420 | none | V1 matching fallback; proposal not canonical timeline | T530 |
| T530 | Implement Timeline compiler/media adapters | blocked by T420/T524 and OQ-08 | none | renderer reads TimelineVersion; V1 renderer rollback | T531 |
| T531 | Implement native render/deliverable QA | blocked by T530 | none | V1 render remains selectable until parity | T532 |
| T532 | Add three-profile reuse | blocked by T530/T531 | none | no AI rerun on profile-only render; revert profile path | T600 |
| T600 | Enforce worker sandbox/secret/egress policy | blocked by Gate G | none | fail closed; rollback only to approved safe image | T601 |
| T601 | Add observability/graceful recovery | blocked by T300/T312/T600 | none | preserve replay/diagnostic contract; rollback dashboards | T602 |
| T602 | Add backup/restore/Artifact inventory | blocked by T206/T222/T601 | none | restore drill and inventory before release | T603 |
| T603 | Implement legacy data importer | blocked by T003/T004/T206/T601 and OQ-09 | none | copy/verify/switch/retain; rollback source untouched | T604 |
| T604 | Execute upstream update comparison drill | blocked by T001/T003/T005/T603 | none | module-aware port/ignore; never wholesale merge | T605 |
| T605 | Publish compatibility deprecation/removal gate | blocked by all release evidence | none | removal only after parity/window/owner approval | release review |

## Handoff rule

Before changing a row to `complete`, attach command, environment, commit, artifact/report location,
result, known limitation, acceptance criterion, compatibility/security review and rollback evidence
to [`TEST-EVIDENCE.md`](TEST-EVIDENCE.md) and update [`CURRENT-STATE.md`](CURRENT-STATE.md).
