# NH-Media desktop

Tauri 2 thin remote-first shell. It owns no Product state, provider credential, database, Redis,
object-storage service, FFmpeg process or Python runtime. Its configured endpoint is the Product API.

The webview uses the shared @nh-media/sdk flow for endpoint configuration, LocalAuth/session
revocation, multipart upload, SSE reconnect/replay, durable review commands and Artifact/result
download. A restart reconstructs state from Product API; no local draft is authoritative.

The Rust shell exposes only endpoint validation and a boundary diagnostic command. Its capability
manifest grants core:default and no filesystem, shell, process, database, Redis, storage or
provider permissions. LAN deployments must add the exact configured web/API origins to the staging
CSP/allowlist; this baseline intentionally allows loopback only.

Windows prerequisites: WebView2 Evergreen Runtime, Rust/Tauri 2 toolchain and a running web client.
Tauri does not bundle Go, FFmpeg, Python, models, PostgreSQL, Redis, MinIO or provider secrets.
