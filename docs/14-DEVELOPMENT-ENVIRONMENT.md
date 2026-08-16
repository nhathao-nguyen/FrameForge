# 14 — Development Environment (Arch Linux)

## 1. Reproducibility goals

Development target là Arch Linux nhưng project không phụ thuộc system Python packages. Required toolchain:

```text
Git
uv
Python 3.13 managed by uv
FFmpeg/ffprobe (pinned/tested version recorded in baseline)
PostgreSQL
Redis
MinIO or S3-compatible storage
Docker or Podman optional for infrastructure/sandbox
Ollama optional for local LLM/VLM
Node.js/pnpm (web and shared TypeScript client tooling)
Rust stable/pinned targets and Tauri 2 tooling for the desktop client
```

Không `pip install` vào Python system của Arch, không dùng PEP 668 override và không ghi dependency vào global site-packages.

## 2. Python policy

Master plan chọn `uv + Python 3.13 + project virtualenv` cho development. Upstream V1 `pyproject.toml` khai báo Python 3.10–3.13 và CI test cả bốn version; do đó core V1 phải được baseline trên Python 3.13.

Upstream Dockerfile hiện dùng Python 3.12 có chủ ý vì compatibility wheel của ML stack. OQ-13 đã
được quyết định: Product/API/core dùng Python 3.13; frozen legacy/ML image tạm dùng Python 3.12
với lock/image riêng. Chỉ hợp nhất sau khi Phase 0 chứng minh dependency/ML/CUDA parity trên 3.13.
Product/core và frozen legacy/ML phải có lock/report/image metadata riêng; không dùng một lockfile
chung để che khác biệt runtime.

Expected setup contract sau Phase 0 (lệnh cụ thể sẽ được lock bởi task implementation):

```text
uv python install 3.13
uv sync --frozen --all-groups (hoặc groups được project định nghĩa)
uv run python --version
uv run pytest ...
```

Lockfile là required source; dependency change phải update lock và CI evidence.

## 3. FFmpeg policy

- `ffmpeg` và `ffprobe` phải cùng tested release/build; lưu output version/config trong baseline report.
- Development có thể dùng Arch package; CI/container pin image/package snapshot để reproducible.
- Production không nhận executable path từ project `.env`/payload.
- Upstream `MN_FFMPEG_BIN` compatibility chỉ hoạt động trong legacy/local mode hoặc explicit allowlist.
- Test phải cover codec/probe/render features thật sự dùng, không chỉ `ffmpeg -version`.

## 4. Infrastructure modes

### Native Arch

PostgreSQL, Redis và MinIO có thể chạy native dưới local-only bind với non-default credentials. Dữ liệu dev đặt ở task-specific directories/volumes, không dùng repository root làm mutable database volume.

### Containerized infrastructure

Docker Compose hoặc Podman Compose có thể chạy PostgreSQL/Redis/MinIO. Pin image versions, health checks và private network. Không bắt buộc containerize editor/API trong Phase 0.

### Local adapters

Unit tests có thể dùng fake/LocalQueue/LocalStorage nhưng integration gate phải chạy PostgreSQL + Redis + S3-compatible adapter để tránh chỉ chứng minh in-memory path.

### Web and desktop clients

Web và desktop phải dùng chung API contract/SDK. Desktop remote-server mode không yêu cầu Python,
FFmpeg, model hoặc database trên máy người dùng; các native capability như file picker/download/
notification phải nằm sau một adapter nhỏ và có permission review. Desktop shell đã được owner
chốt là Tauri 2. Tauri chỉ là remote-first Product API client nhẹ; không bundle
Python/FFmpeg/ML/database/Redis/engine/provider secret. Capability phải least-privilege và
signing/update policy phải có proof trước production.

Client profiles phải kiểm thử ít nhất: server URL config theo environment, login/session, direct
multipart upload, reconnect event stream, download artifact, safe error display và logout/token
revocation. Không commit desktop signing key hoặc local secret.

## 5. Optional ML/local providers

- Ollama là optional provider adapter; absence không làm API/core test fail.
- WhisperX/faster-whisper/FunASR, PySceneDetect/OpenCV và GPU extras ở dependency groups riêng.
- GPU/CUDA/ROCm matrix phải tách khỏi CPU baseline; không buộc mọi developer tải model lớn.
- Model downloads/cache đặt ngoài repo và phải có checksum/version metadata khi dùng golden tests.

## 6. Environment and secrets

- Commit `.env.example` chỉ chứa placeholder/non-secret defaults; `.env` local không commit.
- Không trust `.env` từ imported Project/V1 output.
- Product config, legacy compatibility config và worker sandbox config là namespaces riêng.
- Dev credentials không dùng default production-known values; MinIO `minioadmin` chỉ có thể xuất hiện trong explicit disposable example và phải có warning, recommendation là random local secret.
- Redaction tests kiểm tra logs/events/checkpoints.

## 7. Phase 0 verification matrix

| Check | Required evidence |
|---|---|
| Git baseline | remote URL, local HEAD, remote main, tag object/peeled commit, dirty status |
| Python | uv-managed 3.13 version; optional 3.12 ML/container result |
| Dependencies | lock hash, install command, optional-group matrix |
| FFmpeg | version/build flags, ffprobe, required codec/filter checks |
| V1 unit | upstream test command/result and coverage gate |
| V1 integration | media/FFmpeg/PySceneDetect path and sample artifacts |
| CLI | `mn --help`, `mn version`, create/config/resume behavior |
| REST | serve/submit/status/cancel/result/artifact/auth behavior |
| Storage | Local + S3/MinIO conformance/security tests |
| Containers | non-root UID, health/readiness, mounted paths, CPU/GPU variant |

If a check cannot pass on Arch, baseline report records exact blocker; không thay expectation bằng unverified claim.

## 8. CI expectations

V2 CI tối thiểu:

- lint/type/unit on pinned primary Python;
- compatibility tests against V1 baseline;
- PostgreSQL migration clean bootstrap;
- Redis queue duplicate/reclaim tests;
- Local/S3-compatible storage contract tests;
- Timeline JSON Schema examples/negative corpus;
- API contract/OpenAPI compatibility;
- security checks for subprocess, dependencies, path/upload and secrets;
- integration smoke with pinned FFmpeg;
- optional ML/GPU jobs không được che lỗi core nhưng có status rõ.

Upstream CI hiện test Python 3.10–3.13, coverage 90% trên 3.11, separate integration/media/plugin/security jobs. Bandit config bỏ qua B404/B603 và `pip-audit` đang ignore một nhóm Pillow advisories do MoviePy constraint; V2 không copy các exception này mà thiếu ticket, scope và expiry.
