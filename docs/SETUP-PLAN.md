---
last_verified: 2026-08-16
source: docs/00-PROJECT-CONTEXT.md; docs/01-ARCHITECTURE.md; docs/08-SECURITY.md; docs/10-DEVELOPMENT-ROADMAP.md; docs/IMPLEMENTATION-ORDER.md; docs/OPEN-QUESTIONS.md
owner: repository owner / setup owner
---

# NH-Media setup plan — client/server foundation

## 1. Mục tiêu và trạng thái

Mục tiêu của plan này là tạo một nền móng có thể mở rộng cho web client, desktop client, Go Product
API/control plane, Go media worker, isolated Python ML/V1 workers, provider, storage và các service
mới, với control plane/worker/storage có thể chạy trên một server hoặc VPS riêng. Desktop là client
remote-first; nó không nhúng engine, database, provider hoặc business state.

**Trạng thái:** T000 đã được owner approve và amend; T001 đã hoàn tất trước amendment và không được
khởi động lại. Setup chưa được thực thi. Repository hiện chỉ có specification, frozen V1 reference,
memory và consistency tooling. Không tạo application code cho tới khi T002–T005/Phase 0 evidence
hoàn tất.

Setup chỉ được đánh dấu `complete` khi tất cả verification pass và evidence nằm trong
`docs/baselines/` hoặc `docs/memory/TEST-EVIDENCE.md`. “Các thư mục đã tạo” không đủ để pass setup.

## 2. Boundary phải giữ từ đầu

```text
Browser ──HTTPS──┐
Desktop ─HTTPS───┴─> Go Product API/control plane
                         ├─ PostgreSQL: durable product state
                         ├─ Redis: queue/lease/event coordination
                         ├─ S3/MinIO: private bytes/artifacts
                         └─ versioned worker contract
                              ├─ Go media worker → FFmpeg
                              ├─ Python ML worker → `nh_media`
                              └─ frozen V1 compatibility → `movie_narrator`
```

- Web và desktop chỉ phụ thuộc `packages/contracts`/`packages/sdk` và Product API.
- Product API sở hữu identity, Workspace, authorization, Project, Asset, Job và database.
- Engine sở hữu media/AI execution; không biết user/session/billing/UI.
- Worker nhận lease, checkpoint và báo cáo kết quả; không sở hữu business state duy nhất.
- Client upload large media trực tiếp tới object storage bằng presigned URL.
- Desktop chỉ dùng native capability tối thiểu: file picker, download/export, notification nếu cần.
- Server là source of truth; local client cache/draft có thể bị xóa và phải reconstruct được.
- OQ-15 đã được owner quyết định: desktop shell là Tauri 2. Capability phải least-privilege;
  signing/update policy vẫn cần proof trước production.

## 3. Target repository layout

```text
apps/
├── web/                         # Next.js/React browser client
└── desktop/                     # Tauri 2 shell; UI/SDK shared, no engine/database
cmd/
├── product-api/                 # Go Product API entrypoint
└── media-worker/                # bounded Go FFmpeg/media worker
internal/
├── domain/                      # product aggregates
├── application/                 # orchestration/state transitions
├── ports/                       # language-neutral boundary implementations
├── adapters/{postgres,redis,storage}/
└── transport/http/              # versioned API transport
services/
├── ml-worker/                   # Python `nh_media`, isolated ML/AI runtime
└── legacy-compat/               # frozen Python `movie_narrator` boundary
packages/
├── contracts/                   # versioned language-neutral API/event/worker schemas
├── sdk/                         # shared web/desktop Product API client
└── shared/                      # IDs, time, errors, validation, redaction helpers
infra/
├── compose/                     # local/staging profiles
├── postgres/                    # migrations/roles/health policy
├── redis/                       # ACL/private-network policy
├── minio/                       # S3-compatible dev bootstrap
└── reverse-proxy/               # HTTPS, headers, routing, rate limits
tests/
├── unit/                        # package-local tests
├── integration/                 # API/storage/queue/worker boundary tests
├── regression/                  # frozen V1/golden compatibility
├── client/                      # web + desktop client contract/e2e tests
└── e2e/                         # upload → job → review → render
docs/
tools/
```

Each directory gets a narrow public boundary and its own tests. A new feature should normally add
a contract, domain/service boundary and adapter/test rather than importing an implementation from
another layer.

## 4. Deployment profiles

### Profile A — Local development

```text
Developer machine
├── web dev server
├── desktop dev shell
├── Go Product API
├── bounded Go media worker
├── isolated Python ML/V1 worker (when needed)
└── PostgreSQL + Redis + MinIO
```

Use pinned Go, isolated uv-managed Python, pinned FFmpeg, Node/pnpm for web/shared client tooling
and a disposable containerized infrastructure profile. Desktop remote mode points to the local API; it does not
start a second database or engine.

### Profile B — Single VPS/server baseline

```text
HTTPS reverse proxy
├── web static/SSR delivery
├── Go Product API replicas
├── bounded Go media worker host(s)
├── isolated Python ML/V1 worker host(s) as required
├── PostgreSQL (private)
├── Redis (private)
└── S3/MinIO (private)
```

Only the HTTPS reverse proxy is public. Go API, workers, PostgreSQL, Redis and object storage are on a
private network. This is the first supported remote-server deployment; Kubernetes and specialized
GPU pools are not setup prerequisites.

### Profile C — Split production

```text
Web/CDN or desktop installers
        │ HTTPS
API VPS / API replicas
        ├── managed PostgreSQL
        ├── managed Redis
        ├── private S3
        ├── Go media worker host(s), optionally GPU-capable
        └── Python ML/V1 worker host(s), optionally GPU-capable
```

The contracts, queue messages, leases and artifact references remain unchanged when services move
to separate machines. Do not expose worker or engine ports to clients.

## 5. Ordered setup stages

| Stage | Scope | Required work | Gate |
|---|---|---|---|
| S0 | Ratification | T000 complete/amended; client/server requirement, Go control plane and OQ-12/OQ-13/OQ-14/OQ-15 recorded | Phase 0 authorized; application code still blocked |
| S1 | V1 baseline | T001–T005; immutable upstream manifest, environment, compatibility, golden outputs, rollback image | Gate A pass |
| S2 | Monorepo skeleton | T100; create package boundaries including `apps/desktop` and `packages/sdk` | import-boundary tests |
| S3 | Shared contracts/config | T101–T102; opaque IDs, time, revision/ETag, safe errors, config namespaces and redaction | schema/redaction tests |
| S4 | Private infrastructure | T103–T105; pinned PostgreSQL, Redis, MinIO with roles, ACLs and health checks | clean start/stop and negative access tests |
| S5 | API shell | T106–T107; `/api/v1`, request/correlation IDs, safe errors, liveness/readiness/deep diagnostics | OpenAPI/health smoke |
| S6 | Client boundaries | T100–T102; create web/desktop package boundaries, shared contract/SDK seams and forbidden-import tests; functional clients wait for T430/T433 | package/import/contract smoke |
| S7 | CI and environments | T108; lint/type/unit/security/V1 regression, Compose profiles, server deployment manifest | required checks pass |
| S8 | Setup certification | run all verification passes below, record report and owner sign-off | setup `complete` |

T200+ schema/feature work is not part of setup certification. The setup base must support it
without prematurely implementing domain behavior or media pipeline nodes.

## 6. Desktop foundation requirements

The setup stage creates the desktop boundary and contract seam only. The functional desktop slice is
T433 in Phase 5, after the Product control plane and review backend are available. Setup must still
prove the future client cannot bypass the server:

- package/configure a server URL seam without hard-coded production/localhost behavior;
- compile/import the desktop boundary and shared SDK/contracts;
- provide contract fixtures for auth, upload, Job snapshot/event and Artifact download;
- reject imports to engine/provider/DB/Redis from web/desktop packages;
- document the later T433 requirements: login/logout, presigned upload, Job reconnect, safe errors
  and Artifact download;
- local drafts are explicitly non-authoritative and disposable;
- no Python, FFmpeg, model, PostgreSQL, Redis or provider secret required in remote mode;
- native permissions, deep links, installer/update origin and signing are reviewed before release.

Desktop feature parity with the web editor is a later implementation milestone. Shared contracts and
SDK parity are setup requirements; duplicated business logic is prohibited.

## 7. Required verification passes

Run these as separate passes. A failure in an earlier pass invalidates later pass evidence.

| Pass | Check | Evidence required | Failure condition |
|---|---|---|---|
| V0 | Source/gate | branch, commit, clean/dirty status, gate/OQ report | wrong branch, unapproved gate, V1 modified |
| V1 | Layout | expected directories, package metadata, no forbidden reverse imports | missing boundary or cycle |
| V2 | Documentation | internal links, canonical terms, OQ mirror, task dependency/coverage, stale memory | checker failure or undocumented decision |
| V3 | Static contracts | schema snapshots, type/lint checks, safe-error/secret/path scan | client/server contract drift or leakage |
| V4 | Infrastructure | clean start/stop, health/readiness, private ports, least-privilege roles | public DB/Redis/MinIO or non-reproducible setup |
| V5 | API smoke | auth seam, `/api/v1`, request IDs, health, error envelope, OpenAPI | traceback/secret/path exposure or wrong status |
| V6 | Client-server boundary | web/desktop contract fixtures, SDK serialization, endpoint configuration and forbidden direct-call scan | client bypasses API or contract drift |
| V7 | Security | CORS/origin, token storage, presign scope, desktop permission, dependency/image scan | unsafe origin, secret in binary/log, unsigned/untrusted update |
| V8 | Remote VPS | deploy on clean server profile, TLS/reverse proxy, private dependencies, restart/drain | only-local proof or public internal service |
| V9 | Clean-room | new checkout + documented commands reproduces setup without hidden local state | undeclared dependency, manual secret/path, missing step |
| V10 | Review/handoff | owner reviews report, rollback and next task; memory updated | no acceptance sign-off or incomplete rollback |

The setup report must include command, environment, commit, result, artifact/report path, known
limitation, compatibility impact, security review, rollback result and next task for every pass.

## 8. Setup completion criteria

Setup is complete only when:

- T000 and all setup-affecting OQs are approved; T001–T005/Phase 0 evidence is complete;
- all target package boundaries exist and dependency direction tests pass;
- web and desktop package boundaries compile/import in the selected development profile; full client
  login/upload/reconnect is accepted later by T430/T433;
- both clients use the same SDK/contracts and cannot access engine/provider/DB directly;
- API shell, liveness/readiness, safe errors and auth seam pass;
- PostgreSQL/Redis/MinIO are private, pinned and reproducible;
- local, single-VPS and clean-room setup instructions have executable evidence;
- no secret, token, presigned URL, user data, raw path or traceback leaks;
- V1 reference and compatibility baseline remain unchanged;
- rollback returns to the pre-setup baseline without deleting user/media data;
- `CURRENT-STATE.md`, `IMPLEMENTATION-STATUS.md` and `TEST-EVIDENCE.md` are updated;
- owner signs off setup and names the next implementation task.

## 9. Setup handoff format

```text
Setup: client/server foundation
Status: complete/blocked
Implemented boundary: web, desktop, API, contracts, infra, worker shell
Verification: V0–V10 commands, environments and reports
Compatibility: V1 baseline and legacy adapter unchanged
Security: client/secret/path/presign/sandbox review
Migration/rollback: setup rollback and data safety result
Open questions: OQ-01–OQ-11 remain unresolved; OQ-12–OQ-15 are decided
Next task: exact Txxx from IMPLEMENTATION-ORDER.md
```
