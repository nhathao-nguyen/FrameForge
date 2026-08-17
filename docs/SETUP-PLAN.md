---
last_verified: 2026-08-17
source: docs/00-PROJECT-CONTEXT.md; docs/01-ARCHITECTURE.md; docs/10-DEVELOPMENT-ROADMAP.md; docs/IMPLEMENTATION-ORDER.md
owner: repository owner / setup owner
---

# NH-Media setup plan — independent client/server foundation

## 1. Goal and status

Create reproducible boundaries for web, Tauri 2 desktop, Go Product API, Go media worker, isolated
Python `nh_media` worker, shared contracts, PostgreSQL, Redis and object storage. The setup contains
no upstream runtime, code, package, image, service or compatibility layer.

**Status:** specification work only; setup has not been executed. Application setup begins only
after owner acceptance of the specification gate and the selected task prerequisites.

## 2. Target layout

```text
apps/
  web/
  desktop/
services/
  api/
  media-worker/
  ml-worker/nh_media/
packages/
  contracts/
  sdk/
infrastructure/
  compose/
  postgres/
  redis/
  object-storage/
  reverse-proxy/
tests/
  unit/
  contract/
  integration/
  reference-behavior/
  e2e/
docs/
tools/
```

Go module: `github.com/nhathao-nguyen/NH-Media`. Python imports begin at `nh_media`.

## 3. Deployment profiles

### Profile A — Local developer

One machine runs clients, API, bounded workers and private disposable data services. Toolchains and
images are pinned; Python is uv-managed; FFmpeg is tested.

### Profile B — Local/LAN production-like

```text
LAN server
  ├── Go Product API
  ├── PostgreSQL / Redis / object storage
  ├── Go media worker
  └── Python nh_media worker

LAN client machines
  ├── browser
  └── Tauri 2 client
```

This is the first required remote-client proof. No VPS, public DNS, CDN or public certificate is
required. Internal services remain private and clients use configured server endpoints.

### Profile C — Internet production

Later phase adds public TLS/reverse proxy, public DNS, canary and managed/private dependencies. It is
not a prerequisite for development or the first functional release candidate.

## 4. Ordered setup stages

| Stage | Scope | Gate |
|---|---|---|
| S0 | accept independent specification and decisions | docs-only gate |
| S1 | pin Go/Python/Node/Rust/FFmpeg toolchains | clean-room version evidence |
| S2 | create repository/service/client boundaries | import/dependency scan |
| S3 | create language-neutral contracts and safe errors/config | schema/redaction tests |
| S4 | provision private pinned PostgreSQL/Redis/object storage | health and negative access |
| S5 | create Go API shell and readiness | OpenAPI/error/async-boundary smoke |
| S6 | create bounded Go/Python worker shells | worker fixture/capability tests |
| S7 | create web/Tauri SDK seams | forbidden direct-access tests |
| S8 | establish CI and independence checks | required checks pass |
| S9 | Local/LAN clean-room certification | remote-client smoke |

## 5. Desktop requirements

- Remote Product API is the default and source of truth.
- No Python, FFmpeg processing, model, PostgreSQL, Redis or server secret in the app.
- Native permissions limited to file picker, selected upload/download and notifications as needed.
- Endpoint configuration is environment-aware; no hard-coded production secret or localhost release.
- OS credential store, deep-link/update origin and signing are separate release gates.

## 6. Verification passes

| Pass | Check | Evidence |
|---|---|---|
| V0 | source/gate | branch, commit, dirty state, accepted architecture |
| V1 | layout | expected boundaries and no forbidden imports |
| V2 | docs/contracts | links, schemas, canonical terms, task/OQ consistency |
| V3 | independence | no upstream source/submodule/dependency/import/image/service |
| V4 | infrastructure | clean start/stop, private ports, least-privilege roles |
| V5 | API | safe errors, health/readiness, no long compute inline |
| V6 | workers | bounded concurrency, capability fixture, no broad secrets |
| V7 | clients | shared SDK, endpoint config, no direct worker/provider/data access |
| V8 | security | path/secret/CORS/presign/dependency/permission checks |
| V9 | Local/LAN | separate client connects, uploads and follows a durable Job |
| V10 | clean-room/review | new checkout reproduces setup; owner handoff |

## 7. Completion

Setup is complete only when V0–V10 pass, Local/LAN separation is proven, private dependencies are
reproducible, clients share contracts, workers are bounded, no secrets/paths leak, and upstream is
absent from all product dependency/package/image graphs.

```text
Setup: independent client/server foundation
Status: complete/blocked
Boundary: web, desktop, API, contracts, workers, infra
Verification: V0–V10
Independence: dependency/import/image scan
Security: secrets/path/presign/sandbox review
Data/rollback: reversible setup and data safety
Open questions: ...
Next task: ...
```
