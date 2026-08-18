# NH-Media desktop

Tauri 2 thin remote-first shell. It owns no Product state, provider credential, database, Redis,
object-storage service, FFmpeg process or Python runtime. Its configured endpoint is the Product API.

The webview uses the shared @nh-media/sdk flow for endpoint configuration, LocalAuth/session
revocation, multipart upload, SSE reconnect/replay, durable review commands and Artifact/result
download. A restart reconstructs state from Product API; no local draft is authoritative.

The Rust shell exposes only endpoint validation and a boundary diagnostic command. Its capability
manifest grants core:default and no filesystem, shell, process, database, Redis, storage or
provider permissions. The source baseline intentionally allows loopback only. LAN staging uses
`tools/run-t434-signing.ps1 -LanServerIp <SERVER_LAN_IP>` to merge the exact API origin into the
Tauri CSP and compile the same endpoint into the static frontend; wildcard network origins remain
prohibited.

Windows prerequisites: WebView2 Evergreen Runtime, Rust/Tauri 2 toolchain and a running web client.
Tauri does not bundle Go, FFmpeg, Python, models, PostgreSQL, Redis, MinIO or provider secrets.

The release path builds the static web shell into the Tauri frontend directory, enables the Windows
NSIS bundle, and keeps the loopback CSP as the default. `tools/package-desktop.ps1 -Action validate`
is unsigned development evidence. A release owner must provide the external Tauri signing key only
in the current process; `tools/package-desktop.ps1 -Action package` signs bundle payloads, records
checksums and refuses to stage or roll back an unsigned package. LAN desktop builds must use an
exact external CSP origin overlay for the selected server IP; the package manifest records that API
origin and verification rejects a different requested LAN target.
