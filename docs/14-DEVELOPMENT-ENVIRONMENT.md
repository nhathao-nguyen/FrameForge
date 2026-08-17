# 14 — Development Environment

## 1. Reproducibility goals

Primary documented development and first Local/LAN acceptance target is the owner's current Windows
machine using PowerShell, Docker-compatible infrastructure and pinned native/tool-managed runtimes.
Linux remains a supported CI/deployment target once validated. Required toolchain:

```text
Git
pinned supported Go toolchain
uv for isolated Python worker environments
Python version selected by the ML/provider dependency matrix
FFmpeg/ffprobe pinned and probed
PostgreSQL
Redis
MinIO or another S3-compatible store
Node.js/pnpm for web/SDK
Rust + Tauri 2 tooling for desktop
Docker or Podman when needed for infrastructure/sandbox
```

Do not install ML dependencies into a system Python or bypass environment isolation/PEP 668.

## 2. Runtime policy

- Product API/control plane is Go and does not depend on Python.
- Go module path is `github.com/nhathao-nguyen/NH-Media`.
- Python is isolated to `services/ml-worker/nh_media`; target Python 3.13 where the selected
  ML/provider stack supports it, and pin a separate lower version only with dependency/parity
  evidence. Do not assume one version for every CUDA/model stack.
- Go, Python, web and desktop share versioned JSON/Protobuf/schema contracts, not implementation
  classes or ORM objects.
- No upstream checkout or `movie_narrator` package is installed by standard setup.

Expected bootstrap contract after implementation:

```text
go version
go test ./...
uv sync --frozen --project services/ml-worker
uv run --project services/ml-worker python --version
uv run --project services/ml-worker pytest
ffmpeg -version
ffprobe -version
```

Exact versions and lock hashes are selected by T002 and stored in clean-room evidence.
T002 selects currently supported compatible Go, Node/pnpm, Rust/Tauri, Python/uv and FFmpeg
versions at task execution time. Exact version selection is a bootstrap operation, not an Open
Question and does not reopen the architecture.

## 3. FFmpeg

- FFmpeg/ffprobe use the same tested release/build.
- Development package use is allowed; CI/images pin a reproducible source.
- Product/project config cannot choose an arbitrary executable.
- Tests cover actual codecs, filters, probe and render behaviors, not only version output.

## 4. Infrastructure profiles

### Local development

PostgreSQL, Redis and object storage may run native or in pinned containers with private/local
binding and non-default credentials. Mutable data uses task-specific volumes outside repository root.

The canonical target developer experience is one documented orchestration entry point (for example
`make dev`, `just dev` or an equivalent PowerShell-friendly command) which starts/verifies pinned
PostgreSQL, Redis and MinIO, then Product API, Go media worker, Python worker, web and optional Tauri
development client. Until that entry point is implemented, the conceptual order is:

```text
docker compose up -d postgres redis minio
Product API → Go media worker → Python nh_media worker → web → desktop dev
```

Local mode binds loopback, runs LocalAuthProvider and idempotently bootstraps one admin User, one
default Workspace and one owner membership.

### Local/LAN production-like

One LAN server runs API, data services and workers; separate machines run web/Tauri clients. This
profile proves endpoint configuration, authentication, upload, Job replay, Artifact download,
worker recovery and server-authoritative state without requiring public DNS/VPS.

Conceptual LAN configuration (the actual address is environment-specific):

```text
PROFILE=lan
API_BIND=0.0.0.0:8080
PUBLIC_API_URL=http://<server-lan-ip>:8080
WEB_BIND=0.0.0.0:3000
ALLOWED_ORIGINS=http://<server-lan-ip>:3000
```

LAN exposure is never automatic. Authentication and Workspace authorization remain active; clients
configure the Product API endpoint; PostgreSQL, Redis, MinIO and workers stay server-side. Trusted
private-LAN HTTP is allowed for Local Functional Acceptance and clearly marked insecure for Internet
use.

### Internet production

Public TLS/DNS/CDN/ingress and canary are a later deployment profile. They must not be prerequisites
for functional development.

## 5. Clients

Web and desktop use one SDK/contract set. Tauri remote mode does not require Python, FFmpeg, models,
PostgreSQL or Redis on the client. Native capabilities are minimal and permission-reviewed. Desktop
signing keys and secrets stay outside the repository.

Client test profiles cover server URL configuration, login/session, multipart upload, event
reconnect, Artifact download, safe errors and logout/revocation.

## 6. Optional providers/models

- Local/remote providers are optional adapters; absence does not fail core control-plane tests.
- ASR/VLM/scene/GPU extras live in separate dependency groups/images.
- GPU/CUDA/ROCm evidence is separate from CPU baseline.
- Model caches live outside repo and record model/version/checksum provenance for test fixtures.

## 7. Secrets and configuration

- `.env.example` contains placeholders only; local `.env` is not committed.
- Imported Project data cannot override executable or secret policy.
- Product, media worker, ML worker and provider configuration use separate namespaces.
- Logs/events/checkpoints pass redaction tests.

## 8. Verification matrix

| Check | Required evidence |
|---|---|
| Git | branch/commit/dirty status and intended diff |
| Independence | no upstream source/submodule/import/package/image/route in build graph |
| Go | pinned version, module path and clean test |
| Python | isolated `nh_media` lock/image and dependency groups |
| FFmpeg | version/build flags plus required codecs/filters |
| Contracts | same fixtures pass Go/Python/client validators |
| Storage | Local and S3-compatible conformance/security |
| Infrastructure | private services, health, restart and clean bootstrap |
| Local/LAN | remote client upload → Job → worker → Artifact → result |
| Desktop | thin-client package, permissions and no bundled server compute |

## 9. CI

- Go format/vet/test/static/security on pinned toolchain;
- Python lint/type/unit/security in isolated `nh_media` scope;
- PostgreSQL migration bootstrap;
- Redis duplicate/reclaim tests;
- storage contract tests;
- Timeline schema positive/negative corpus;
- Product API/OpenAPI contract tests;
- subprocess, upload/path and secret security checks;
- pinned FFmpeg integration smoke;
- dependency/image/SBOM/license review;
- independence scan proving upstream is absent from product dependency and artifact graphs.

Recorded upstream CI/dependency behavior is research evidence only; NH-Media does not copy advisory
exceptions without its own scoped ticket, owner and expiry.
