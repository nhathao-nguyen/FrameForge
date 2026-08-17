---
last_verified: 2026-08-17
source: repository bootstrap inspection; Windows PowerShell command evidence
owner: repository owner / task assignee
---

# T002 — Windows toolchain matrix

## Status

**COMPLETE** for the pre-code/bootstrap phase on 2026-08-17. This record pins the native
toolchain baseline without creating an application module, service, client or worker.

The selected versions are the versions actually validated on the current Windows machine. A
future clean checkout must verify the same pins before installing dependencies. Tool-managed
environments and lockfiles for application packages are intentionally deferred to T100 and later
service/package tasks; no system `pip` workflow is introduced here.

## Reproducibility pins

| Tool | Detected | Selected | Pin mechanism | Validation result |
|---|---|---|---|---|
| Windows | Windows 11 Pro `10.0.26200`, build `26200` | same | host prerequisite evidence | pass |
| PowerShell | `7.6.4` | same | host prerequisite evidence | pass |
| Git | `2.53.0.windows.3` | same | host prerequisite evidence | `git --version` pass |
| Go | `go1.26.5 windows/amd64` | `1.26.5` | root `.go-version`; future module must use this exact toolchain | `go version` and `go env GOVERSION` pass |
| Python | `3.13.14` | `3.13.14` | root `.python-version`; Python dependencies must be uv-managed | `python --version` and `uv python find 3.13.14` pass |
| uv | `0.11.28` | `0.11.28` | bootstrap evidence; future Python project must use uv lock/install commands | `uv --version` pass |
| Node.js | `v24.14.1` | `24.14.1` | root `.node-version`; root `package.json` engine | `node --version` pass |
| pnpm | `11.19.0` | `11.19.0` | root `package.json` `packageManager` and engine | `pnpm --version` pass |
| Rust | `1.97.1 (8bab26f4f)`, `x86_64-pc-windows-msvc` | `1.97.1` | root `rust-toolchain.toml`; exact rustup toolchain | `rustc --version --verbose` pass |
| Tauri CLI | absent initially; installed during T002 | `2.11.4` | exact `cargo install tauri-cli --version 2.11.4 --locked` | `cargo-tauri --version` pass |
| FFmpeg | PATH first resolved to `8.1.2` from a different install | `2026-03-12-git-9dc44b43b2-essentials_build` | configure the future worker to use a reviewed path/artifact matching both hashes below; PATH is not accepted as the product pin | paired version/build, codec/filter and encode smoke pass |
| ffprobe | PATH resolved to `2026-03-12-git-9dc44b43b2-essentials_build` | same build as FFmpeg | same reviewed artifact/build ID and hash verification | paired version/build and probe smoke pass |
| Docker Desktop | `4.82.0 (233772)` | same | local developer prerequisite; future infrastructure pins image digests separately | engine smoke pass |
| Docker Engine | client/server `29.6.1`, context `desktop-linux` | `29.6.1` | Docker Desktop installation baseline; compose files/images are later T100/T103–T105 inputs | `docker version` pass |
| Docker Compose | `v5.3.0` | `v5.3.0` | Docker Compose CLI bundled with selected Docker Desktop | `docker compose version` pass |

## Windows/Tauri prerequisites

| Prerequisite | Detected evidence | Result |
|---|---|---|
| Visual Studio Build Tools | `17.14.37`; MSVC `14.44.35207`; `cl.exe` x64 compiler `19.44.35228` | pass through `VsDevCmd.bat -arch=amd64 -host_arch=amd64`, `where cl`, `cl` |
| Rust target | `x86_64-pc-windows-msvc` installed | pass via `rustup target list --installed` |
| Rust components | `rustfmt`, `clippy`, `rust-std`, `rustc`, `cargo` for `1.97.1-x86_64-pc-windows-msvc` | pass via `rustup component list --toolchain 1.97.1-x86_64-pc-windows-msvc --installed` |
| WebView2 | Microsoft Edge WebView2 Runtime `151.0.4129.86` | pass from EdgeUpdate registry and installed application directory |
| Tauri CLI | `tauri-cli 2.11.4` | pass; no Tauri application was built in this phase |

## FFmpeg artifact evidence

The initial PATH inspection exposed a non-reproducible mismatch: `ffmpeg` resolved to the
PySceneDetect `8.1.2` binary while `ffprobe` resolved to a different 2026-03-12 git build. The
selected pair is the matching build under:

```text
D:\New folder\ffmpeg-2026-03-12-git-9dc44b43b2-essentials_build\bin
```

The path is machine evidence, not a durable application configuration. Future media-worker setup
must locate or provision this reviewed build through an explicit configuration/installer and
verify both version/build ID and SHA-256 before use.

| Binary | Version/build | SHA-256 |
|---|---|---|
| `ffmpeg.exe` | `2026-03-12-git-9dc44b43b2-essentials_build-www.gyan.dev` | `AF9E7AF850346AE908745F6191CDCF9581889E915F8FE2DD6AB8CABEF21161D9` |
| `ffprobe.exe` | `2026-03-12-git-9dc44b43b2-essentials_build-www.gyan.dev` | `6D2B7AC8CD07DA82F066994BE8BCF0C74AB1ABA7F248BB75025FCC543711181A` |

Validated capabilities include `libx264`, `aac`, `scale`, `aresample` and `subtitles`. A temporary
320x180, one-second H.264/AAC MP4 was encoded and then probed successfully; the temporary file
was removed after the check. No media asset or generated file was added to the repository.

## Commands and results

The following Windows/PowerShell-compatible checks passed on 2026-08-17:

```text
git --version
go version
go env GOVERSION GOOS GOARCH
python --version
uv --version
uv python find 3.13.14
node --version
pnpm --version
rustc --version --verbose
rustup show active-toolchain
rustup target list --installed
rustfmt --version
cargo clippy --version
cargo-tauri --version
ffmpeg -version       # selected pair was invoked by explicit reviewed path
ffprobe -version      # selected pair was invoked by explicit reviewed path
docker version
docker compose version
```

Docker Desktop was started only to validate the installed local development prerequisite. No
containers, volumes, production services or infrastructure files were created by T002.

## Boundary and limitations

- `.go-version`, `.python-version`, `.node-version`, `package.json` and `rust-toolchain.toml` are
  metadata pins only; they do not claim that application modules exist.
- No `go.mod`, Python project, `uv.lock`, web package graph, Tauri application or Docker Compose
  stack is fabricated to hold versions. Those files belong to their owning T100+ task.
- The selected FFmpeg build is validated for the current Windows machine; a later clean-room
  task must add immutable download/source checksums or an approved internal artifact source.
- No ML/GPU environment was installed. Python 3.13 and uv are validated without heavyweight model
  dependencies, as required for this bootstrap phase.
- This evidence does not claim API, worker, render-pipeline, database, queue or Local/LAN product
  acceptance.

## Handoff

```text
Task: T002
Status: complete
Implemented boundary: root toolchain metadata and Windows bootstrap evidence only
Tests: native version checks, Tauri prerequisite checks, paired FFmpeg/ffprobe codec/render smoke, Docker CLI/engine checks
Independence: no Movie Narrator checkout/package/image/import used
Security: no secrets or user data recorded; FFmpeg executable is explicitly reviewed/pinned by build and hash
Data/rollback: no application data or runtime state changed; Docker Desktop was started for inspection
Open questions: none; future artifact distribution is an implementation/configuration task
Next task: T003 reference-behavior fixture policy
```
