# Gate G local dependency preflight — 2026-08-18

This is the actual current Windows-machine preflight for Gate G. It does not reuse a previous
machine's installation claim. Repository pins and the existing T002 toolchain policy remain the
source of truth.

## Host and capability boundary

- Windows host: Lenovo 82XV; Intel i5-13420H (8 cores/12 logical), about 15.7 GiB RAM.
- GPU: NVIDIA GeForce RTX 3050 Laptop 6 GiB; driver 560.94 and CUDA 12.6 visible through
  `nvidia-smi`. No approved NH-Media model runtime or provider credentials were present, so Gate G
  uses deterministic fake/local providers and does not claim external ML quality.
- Storage headroom: approximately 282 GiB on C: and 248 GiB on D: at preflight.
- No native PostgreSQL, Redis or MinIO service was assumed. Docker Desktop was started through its
  installed per-user executable, became ready, and the repository local Compose profile reported
  healthy PostgreSQL 16.10, Redis 7.4.5 and the pinned MinIO service.

## Required tool status and action

| Dependency | Repository pin | Current-machine result | Action/evidence |
|---|---|---|---|
| Go | 1.26.5 | present, exact version | `go1.26.5`; `go mod download`, `go vet`, and `go test ./...` pass |
| Rust/Cargo | 1.97.1 | present, exact version | `rustc/cargo 1.97.1`; `cargo check` pass |
| Tauri CLI | 2.11.4 | initially missing | installed with locked Cargo command; `cargo-tauri 2.11.4` verified |
| Python | 3.13.14 | system Python 3.14.6 is not the project runtime | exact 3.13.14 selected by uv; project sync/tests pass |
| uv | 0.11.28 | PATH had 0.11.32 | official pinned 0.11.28 installed and used by acceptance |
| Node.js | 24.14.1 | PATH had a different Node | official pinned 24.14.1 installed and used directly by acceptance |
| pnpm | 11.19.0 | present via project/tooling shim | exact pinned Node invokes the repository pnpm.mjs; recursive typecheck passes |
| FFmpeg/ffprobe | 2026-03-12 git `9dc44b43b2` essentials | initially missing; old T002 path absent | official reviewed GyanD release installed; executable hashes match T002 and media filters/codecs smoke pass |
| Docker/Compose | Docker Desktop policy | daemon initially stopped | Docker Desktop started; `docker 29.7.2`, Compose `v5.4.0`, local profile healthy |

The Node and uv installs are project toolchain installations outside the repository; no lockfile pin
was changed. FFmpeg/ffprobe were obtained from the reviewed official release source, not a random
binary mirror. No credentials, tokens, absolute machine paths, or local service volumes are part of
the repository change.

## Gate G disposition

All installable local prerequisites required by Gate G are now available and verified by their actual
version commands. External provider credentials, approved model runtimes and a production/VPS
environment remain unavailable by scope; they are not required for the deterministic local Gate G
acceptance and are not silently substituted.
