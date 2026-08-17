---
last_verified: 2026-08-17
source: ../IMPLEMENTATION-ORDER.md; ../10-DEVELOPMENT-ROADMAP.md; CURRENT-STATE.md; TEST-EVIDENCE.md
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
| T002 | Toolchain matrix | T000 | complete | `docs/bootstrap/T002-TOOLCHAIN-MATRIX-WINDOWS.md` and native version/codec evidence | T003 |
| T003 | Reference-behavior fixture policy | T001 | complete | `tests/reference-behavior/README.md` and empty manifest validation | T100 |
| T004 | Documentation/independence gate | T000, T001 | complete | owner ratification, final spec audit and post-bootstrap certification | T100 |
| T100 | Repository skeleton | T002, T003, T004 | complete | approved boundaries, Go module, clients, workers, contracts/SDK, infra/test trees | T101–T108 |
| T101 | Language-neutral primitives | T100 | complete | shared JSON schemas/fixtures pass Go/Python/TypeScript validation | T102 |
| T102 | Configuration/redaction | T101 | complete | separate namespaces, SecretStore AES-GCM boundary, redaction tests and placeholder env | T103–T105 |
| T103 | PostgreSQL | T100 | complete | private Compose service, distinct app/migration roles, privilege/restart smoke | T104 |
| T104 | Redis | T100 | complete | authenticated private service, AOF/RDB persistence and restart smoke | T105 |
| T105 | Object storage | T100 | complete | private MinIO bucket/bootstrap, object/range/anonymous-denial/restart smoke | T106 |
| T106 | Go Product API shell | T101–T105 | complete | `/api/v1`, IDs/errors/CORS/bounds, LocalAuth shell, mocked-port tests | T107 |
| T107 | Health/readiness | T106 | complete | liveness/readiness/diagnostics with dependency-down, timeout and drain tests | T108 |
| T108 | CI/independence gates | T100-T107 | complete | local verification, CI workflow, negative independence proof, lock/SBOM/secret scans | T200 |
| T200 | Migration framework | T103, T108 | complete | `services/api/internal/persistence/migrations.go`, eight versioned SQL migrations, runner empty/repeat/dirty tests and live PostgreSQL apply/repeat/app-role denial including migration 0008 |
| T201 | Identity/Workspace tables | T200 | complete | `0001_identity.sql`, transactional bootstrap, scoped role repository and PostgreSQL-backed hashed session/revoke/expiry path | later OIDC/API-key policy remains outside initial LocalAuth |
| T202 | Workflow/Pipeline tables | T200 | complete | `0002_workflows_providers.sql`, `EnsurePipelineDefinition` and active-definition trigger | add broader graph integration corpus later |
| T203 | SecretStore/Provider/RenderProfile tables | T200–T201 | complete | encrypted SecretStore boundary, `secret_records`, durable RenderProfile draft/edit/activate/deprecate/disable service, active immutability trigger and redacted Provider API | external KMS/Vault adapter remains deferred by the ratified Local baseline |
| T204 | Project/Asset tables | T201–T203 | complete | `0003_projects_assets.sql`, scoped SQL Project/Asset/upload repository and native upload metadata API | no large media bytes are stored in PostgreSQL |
| T205 | Execution/review tables | T202, T204 | complete | `0004_execution.sql` canonical status checks, snapshots, active-run uniqueness and reviews | transition service is explicitly Gate E/T300 |
| T206 | Artifact/checkpoint/DLQ tables | T204–T205 | complete | artifact/checkpoint/DLQ schema, typed Artifact commit boundary, SQL Artifact metadata repository and orphan reconciliation table | physical orphan sweeper remains a later worker/recovery concern |
| T207 | Content/analysis tables | T204–T206 | complete | `0005_content_timeline.sql`, durable Script/Narration/Scene/Analysis paths and native provenance boundary | character command APIs remain outside the current T230–T235 route set |
| T208 | Timeline/render/candidate tables | T203, T206–T207 | complete | timeline/render/candidate schema, validated immutable SQL Timeline/Render/candidate paths and content hash | candidate evaluation execution remains later worker work |
| T209 | Event/outbox/idempotency tables | T201, T205 | complete | `0006_events_and_idempotency.sql`, append-only trigger and transactional event/outbox method | publisher/state transition remains Gate E |
| T210 | Auth/scoped repositories | T106, T201 | complete | Principal context, role matrix, SQL workspace scoping, active user/Workspace fail-closed checks, guessed-ID negative test and durable session adapter | API-key scope enforcement beyond schema remains later auth work |
| T220 | StoragePort/LocalStorage | T101, T206 | complete | typed StoragePort, root/symlink/traversal/range/precondition tests | conformance expansion remains possible |
| T221 | S3/MinIO adapter | T105, T220 | complete | live opt-in MinIO conformance: staged checksum, promote precondition, range, private presigned download, direct multipart PUT/ETag/complete, expiry and abort | cloud-vendor-specific IAM remains deployment configuration |
| T222 | Artifact commit | T209, T220–T221 | complete | promote/publish/orphan compensation, duplicate/checksum tests, SQL Artifact metadata and durable reconciliation records | reconciliation worker is later execution work |
| T223 | Upload-session API | T204, T210, T221–T222 | complete | StoragePort-backed durable initiate/presign/complete/abort, direct MinIO PUT smoke, atomic validating+asset_probe Job insert, restart-safe provider IDs and HTTP idempotency | validation execution remains T224/T300+ boundary |
| T224 | Validation/probe boundary | T205–T206, T222–T223 | complete | magic/MIME/size/SHA256/ffprobe boundary with deterministic spoofed, oversized, malformed-container, invalid-stream, output-limit, stream-limit and timeout quarantine tests; `Validated` is set only after all checks pass; SQL Asset readiness requires a committed Artifact | worker orchestration and scan execution remain outside Gate C+D/T300 |
| T230 | Project/Asset REST | T210, T223–T224 | complete | native CRUD/upload metadata routes, durable repository wiring, ETag checks, idempotency replay and live upload smoke | Gate E only for Job commands |
| T231 | Script/Narration APIs | T207, T210 | complete | immutable ScriptVersion approval, approved-version Narration Job boundary and durable repository wiring | Gate E only for execution |
| T232 | Timeline validator | T208 | complete | JSON Schema, deterministic hash, range/source/security negative corpus, fail-closed Asset/Artifact/Scene/Narration resolver and durable sqlmock ownership/state tests | live Compose infrastructure is available; no separate live resolver corpus is claimed |
| T233 | Timeline/Render REST | T210, T232 | complete | typed commands, versioning, approval/lock and full durable validation on create/version/approve/lock/validate/render; Render requires exact eligible profile snapshot and never auto-creates profiles | Gate E only for Render execution |
| T234 | Scene/Analysis/candidate APIs | T207–T210 | complete | scoped native routes and provenance/reference-style policy boundary | add selection immutability/pagination corpus later |
| T235 | ProviderConfiguration API | T203, T210 | complete | redacted durable lifecycle, admin checks, plaintext-secret rejection and SQL-scoped provider repository | external secret manager remains deferred |
| T300 | State transitions | T205, T209 | blocked | none | after dependencies |
| T301 | Outbox/replay | T209, T300 | blocked | none | after dependencies |
| T310 | QueuePort | T104, T301 | blocked | none | after dependencies |
| T311 | Dependency scheduler | T202, T300, T310 | blocked | none | after dependencies |
| T312 | Lease/reconciliation | T300, T310 | blocked | none | after dependencies |
| T313 | Sandboxed MediaProcessPort | T224, T312 | blocked | none | after dependencies |
| T320 | Go/Python contracts | T101, T312–T313 | blocked | none | after dependencies |
| T321 | Probe/thumbnail node | T222, T313, T320 | blocked | none | after dependencies |
| T322 | First native E2E slice | T230, T300-T321 | blocked | none | prove independent slice |
| T323 | First Python worker slice | T320, T322 | blocked | none | after deterministic slice |
| T330 | Checkpoint/crash resume | T206, T312, T322 | blocked | none | after slice |
| T331 | Pause/partial/review | T330 | blocked | none | after dependencies |
| T332 | Cancel/retry/DLQ | T300, T330 | blocked | none | after dependencies |
| T340 | SSE progress | T301, T331–T332 | blocked | none | after dependencies |
| T341 | Future bidirectional transport evaluation | T340 | blocked | none | deferred nonblocking after SSE evidence |
| T350 | Job/Run/Step/Render commands | T230–T235, T300–T340 | blocked | none | after dependencies |
| T400 | DAG validation | T202, T320 | blocked | none | after dependencies |
| T401 | Node conformance | T311–T313, T330, T400 | blocked | none | after dependencies |
| T402 | Built-in movie-recap graph | T400, T401 | blocked | none | after dependencies |
| T410 | Review orchestration | T231, T233–T234, T331, T402 | blocked | none | after dependencies |
| T420 | Build timeline | T232, T401–T410 | blocked | none | after dependencies |
| T430 | Web shell/SDK | T210, T230, T340 | blocked | none | after dependencies |
| T431 | Script editor | T231, T410, T430 | blocked | none | after dependencies |
| T432 | Timeline/Scene editor | T233–T234, T410, T420, T430 | blocked | none | after dependencies |
| T433 | Tauri client | T223, T340, T430 | blocked | none | after web client contract |
| T550 | Local Functional Acceptance | T323, T340, T430, T433 | blocked | none | certify functional local/LAN path |
| T434 | Desktop packaging/security | T433 | blocked | none | after T433 |
| T500 | Provider ports/resolver | T235, T401 | blocked | none | after dependencies |
| T510 | Research/script nodes | T402, T500 | blocked | none | after dependencies |
| T511 | TTS/Narration | T231, T500, T510 | blocked | none | after dependencies |
| T512 | ASR/alignment | T500–T511 | blocked | none | after dependencies |
| T520 | Scene detection/features | T224, T401 | blocked | none | after dependencies |
| T521 | VLM scene analysis | T500, T520 | blocked | none | after dependencies |
| T522 | Character analysis | T207, T520–T521 | blocked | none | after dependencies |
| T523 | Embedding/index Artifact | T500, T521 | blocked | none | Artifact/index baseline |
| T524 | Match proposals/coverage | T510, T512, T520–T523 | blocked | none | after dependencies |
| T525 | Candidate evaluation/selection | T234, T500, T524 | blocked | none | after dependencies |
| T526 | ReferenceStyleAnalysis | T234, T512, T520–T521 | blocked | none | after dependencies |
| T530 | Timeline compiler/media plan | T313, T420, T524 | blocked | none | after dependencies |
| T531 | Render/deliverable QA | T222, T332, T350, T530 | blocked | none | after dependencies |
| T532 | Multi-profile reuse | T531 | blocked | none | after T531 |
| T600 | Sandbox/secret/egress hardening | T313, T500, T531 | blocked | none | after dependencies |
| T601 | Observability/recovery | T340, T600 | blocked | none | after dependencies |
| T602 | Backup/restore/inventory | T222, T601 | blocked | none | after dependencies |
| T603 | Local/LAN Hardened Acceptance | T434, T531–T532, T550, T600–T602 | blocked | none | harden accepted functional system |
| T604 | Upstream research refresh | T001, T003, T108 | blocked | none | research evidence only |
| T605 | Internet/VPS production release | T603 | blocked | none | explicit production approval |

Gate C+D implementation and this repair pass are claimed in the current working tree; T300 and
later execution work remain blocked and were not started.
