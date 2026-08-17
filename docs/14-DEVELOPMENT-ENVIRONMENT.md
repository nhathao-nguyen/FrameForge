# 14 — Development Environment

## 1. Reproducibility goals

Primary documented development target is Arch Linux; Local/LAN deployment may also be exercised on
other supported hosts. Required toolchain:

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

Do not install ML dependencies into Arch system Python or bypass PEP 668.

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

## 3. FFmpeg

- FFmpeg/ffprobe use the same tested release/build.
- Development package use is allowed; CI/images pin a reproducible source.
- Product/project config cannot choose an arbitrary executable.
- Tests cover actual codecs, filters, probe and render behaviors, not only version output.

## 4. Infrastructure profiles

### Local development

PostgreSQL, Redis and object storage may run native or in pinned containers with private/local
binding and non-default credentials. Mutable data uses task-specific volumes outside repository root.

### Local/LAN production-like

One LAN server runs API, data services and workers; separate machines run web/Tauri clients. This
profile proves endpoint configuration, authentication, upload, Job replay, Artifact download,
worker recovery and server-authoritative state without requiring public DNS/VPS.

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
