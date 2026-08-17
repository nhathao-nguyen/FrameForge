---
last_verified: 2026-08-17
source: ../IMPLEMENTATION-ORDER.md; ../10-DEVELOPMENT-ROADMAP.md; CURRENT-STATE.md
owner: repository owner / task assignee
---

# Implementation status

Normative dependencies and definitions of done are in
[`../IMPLEMENTATION-ORDER.md`](../IMPLEMENTATION-ORDER.md). `pending` means the task can start only
after its listed dependencies; `blocked` means a gate or dependency is unmet; `complete` means
reviewable evidence is recorded.

| Task | Scope | Dependencies | Status | Evidence | Next task / handoff |
|---|---|---|---|---|---|
| T000 | Independent specification | none | complete | current documentation refactor | T002 and T003 |
| T001 | Upstream research provenance | T000 | complete | dated provenance record | T003 |
| T002 | Toolchain matrix | T000 | pending | none | execute T002 |
| T003 | Reference-behavior fixture policy | T001 | pending | none | execute T003 |
| T004 | Documentation/independence gate | T000–T003 | blocked | none | owner approval then T100 |
| T100 | Repository skeleton | T002, T004 | blocked | none | after Gate A |
| T101 | Language-neutral primitives | T100 | blocked | none | after T100 |
| T102 | Configuration/redaction | T101; OQ-05 for credentials | blocked | none | after T101 |
| T103 | PostgreSQL | T100 | blocked | none | after T100 |
| T104 | Redis | T100 | blocked | none | after T100 |
| T105 | Object storage | T100 | blocked | none | after T100 |
| T106 | Go Product API shell | T101–T105; OQ-01 for auth | blocked | none | after dependencies and OQ-01 |
| T107 | Health/readiness | T106 | blocked | none | after dependencies |
| T108 | CI/independence gates | T100-T107 | blocked | none | after foundation |
| T200 | Migration framework | T103, T108 | blocked | none | after dependencies |
| T201 | Identity/Workspace tables | T200; OQ-01/OQ-06 | blocked | none | after OQ-01/OQ-06 |
| T202 | Workflow/Pipeline tables | T200; OQ-11 scope | blocked | none | after T200 |
| T203 | Provider/RenderProfile tables | T200–T201; OQ-05 | blocked | none | after OQ-05 |
| T204 | Project/Asset tables | T201–T203 | blocked | none | after T201 |
| T205 | Execution/review tables | T202, T204 | blocked | none | after dependencies |
| T206 | Artifact/checkpoint/DLQ tables | T204–T205 | blocked | none | after T205 |
| T207 | Content/analysis tables | T204–T206 | blocked | none | after dependencies |
| T208 | Timeline/render/candidate tables | T203, T206–T207; OQ-02 | blocked | none | after dependencies and scoped OQs |
| T209 | Event/outbox/idempotency tables | T201, T205 | blocked | none | after T205 |
| T210 | Auth/scoped repositories | T106, T201; OQ-01/OQ-06 | blocked | none | after OQ-01/OQ-06 |
| T220 | StoragePort/LocalStorage | T101, T206 | blocked | none | after dependencies |
| T221 | S3/MinIO adapter | T105, T220 | blocked | none | after T220 |
| T222 | Artifact commit | T209, T220–T221 | blocked | none | after dependencies |
| T223 | Upload-session API | T204, T210, T221–T222 | blocked | none | after dependencies |
| T224 | Validation/probe boundary | T205–T206, T222–T223 | blocked | none | after dependencies |
| T230 | Project/Asset REST | T210, T223–T224 | blocked | none | after dependencies |
| T231 | Script/Narration APIs | T207, T210 | blocked | none | after dependencies |
| T232 | Timeline validator | T208 | blocked | none | after OQ-02 |
| T233 | Timeline/Render REST | T210, T232; OQ-08 | blocked | none | after OQ-08 |
| T234 | Scene/Analysis/candidate APIs | T207–T210 | blocked | none | after dependencies |
| T235 | ProviderConfiguration API | T203, T210; OQ-05 | blocked | none | after OQ-05 |
| T300 | State transitions | T205, T209 | blocked | none | after dependencies |
| T301 | Outbox/replay | T209, T300 | blocked | none | after dependencies |
| T310 | QueuePort | T104, T301; OQ-03 | blocked | none | after OQ-03 |
| T311 | Dependency scheduler | T202, T300, T310 | blocked | none | after dependencies |
| T312 | Lease/reconciliation | T300, T310 | blocked | none | after dependencies |
| T313 | Sandboxed MediaProcessPort | T224, T312 | blocked | none | after dependencies |
| T320 | Go/Python contracts | T101, T312–T313 | blocked | none | after dependencies |
| T321 | Probe/thumbnail node | T222, T313, T320 | blocked | none | after dependencies |
| T322 | First native E2E slice | T230, T300-T321 | blocked | none | prove independent slice |
| T330 | Checkpoint/crash resume | T206, T312, T322 | blocked | none | after slice |
| T331 | Pause/partial/review | T330 | blocked | none | after dependencies |
| T332 | Cancel/retry/DLQ | T300, T330 | blocked | none | after dependencies |
| T340 | SSE progress | T301, T331–T332; OQ-04 | blocked | none | after OQ-04 |
| T341 | Optional WebSocket | T340 and explicit decision | blocked | none | after SSE evidence |
| T350 | Job/Run/Step/Render commands | T230–T235, T300–T340 | blocked | none | after dependencies |
| T400 | DAG validation | T202, T320; OQ-11 scope | blocked | none | after dependencies |
| T401 | Node conformance | T311–T313, T330, T400 | blocked | none | after dependencies |
| T402 | Built-in movie-recap graph | T400, T401 | blocked | none | after dependencies |
| T410 | Review orchestration | T231, T233–T234, T331, T402 | blocked | none | after dependencies |
| T420 | Build timeline | T232, T401–T410 | blocked | none | after dependencies |
| T430 | Web shell/SDK | T210, T230, T340; OQ-01/OQ-04 | blocked | none | after OQ-01 |
| T431 | Script editor | T231, T410, T430 | blocked | none | after dependencies |
| T432 | Timeline/Scene editor | T233–T234, T410, T420, T430; OQ-08 | blocked | none | after OQ-08 |
| T433 | Tauri client | T223, T340, T430 | blocked | none | after web client contract |
| T434 | Desktop packaging/security | T433 | blocked | none | after T433 |
| T500 | Provider ports/resolver | T235, T401; OQ-05 | blocked | none | after OQ-05 |
| T510 | Research/script nodes | T402, T500 | blocked | none | after dependencies |
| T511 | TTS/Narration | T231, T500, T510 | blocked | none | after dependencies |
| T512 | ASR/alignment | T500–T511 | blocked | none | after dependencies |
| T520 | Scene detection/features | T224, T401 | blocked | none | after dependencies |
| T521 | VLM scene analysis | T500, T520 | blocked | none | after dependencies |
| T522 | Character analysis | T207, T520–T521 | blocked | none | after dependencies |
| T523 | Embedding/index Artifact | T500, T521; OQ-07 | blocked | none | after OQ-07 scope |
| T524 | Match proposals/coverage | T510, T512, T520–T523 | blocked | none | after dependencies |
| T525 | Candidate evaluation/selection | T234, T500, T524 | blocked | none | after dependencies |
| T526 | ReferenceStyleAnalysis | T234, T512, T520–T521 | blocked | none | after dependencies |
| T530 | Timeline compiler/media plan | T313, T420, T524 | blocked | none | after dependencies |
| T531 | Render/deliverable QA | T222, T332, T350, T530 | blocked | none | after dependencies |
| T532 | Multi-profile reuse | T531 | blocked | none | after T531 |
| T600 | Sandbox/secret/egress hardening | T313, T500, T531 | blocked | none | after dependencies |
| T601 | Observability/recovery | T340, T600 | blocked | none | after dependencies |
| T602 | Backup/restore/inventory | T222, T601; OQ-10 | blocked | none | after OQ-10 |
| T603 | Local/LAN certification | T322, T430–T434, T531–T532, T600–T602 | blocked | none | certify Local/LAN |
| T604 | Upstream research refresh | T001, T003, T108 | blocked | none | research evidence only |
| T605 | Internet production release | T603; production OQs decided | blocked | none | explicit production approval |

No application implementation is claimed in this ledger.
