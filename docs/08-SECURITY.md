# 08 — Security Specification

## 1. Threat model and trust boundaries

Các input không tin cậy: browser/user text, uploaded media, URL/source metadata, imported V1 job config, LLM/VLM output, plugin package và provider response.

```text
Public API boundary
  Internet/browser (untrusted)
    → HTTPS reverse proxy
    → Product API (auth, authorization, upload/command validation)

Trusted orchestration boundary
    → application state machine + PostgreSQL/outbox/queue/storage metadata
    → worker controller with scoped service identity

Untrusted media-processing boundary
    → disposable node executor (restricted filesystem/resources/network)
    → external providers (separate trust/data-processing boundary)
```

Product API không chạy FFmpeg/Pillow/ML model nặng trong request thread. Worker không được coi media, subtitle hay prompt-generated string là trusted command.

## 2. Authentication and authorization

- Browser dùng product identity/session hoặc bearer token do auth layer cấp; không dùng Movie Narrator engine key.
- Mọi request scoped theo `workspace_id`; mọi resource access kiểm tra membership/role và project ownership.
- Roles baseline: `owner`, `admin`, `editor`, `viewer`.
- `viewer` chỉ đọc và nhận signed download URL; `editor` sửa content/submit Job; `admin/owner` quản lý member/provider policy.
- Worker service identity chỉ có quyền đọc input refs, ghi output refs/events cho Job được claim.
- API key chỉ lưu hash, có scope, expiry, rotation/revoke, prefix để audit.
- Health/readiness/metrics public hay protected phải được gateway policy chốt; không trả project data.

Quyền được kiểm tra ở API và trusted orchestration boundary; không tin `workspace_id/project_id` từ queue payload nếu chưa verify DB. Media executor không có quyền tự authorize hoặc mutate product state.

## 3. Input and upload security

- Upload cấp presigned URL sau khi validate declared size/type; server verify actual size, MIME/magic bytes, checksum và ffprobe result.
- Extension không được dùng làm security decision.
- Chặn path traversal, absolute path, drive letter, symlink escape và archive extraction traversal.
- Object key server-generated, scoped theo workspace/project/asset ID; filename chỉ là display metadata.
- Giới hạn file size, duration, resolution, frame rate, number of tracks và decompression ratio.
- Media chưa checksum/magic/probe/security validation hoặc fail scan không được chạy production pipeline.
- Malware scanner là deployment control tại validation worker; nếu không có scanner, policy phải ghi rõ residual risk và vẫn bắt buộc container/codec/probe/resource validation.
- URL ingestion (nếu có) phải có allowlist/egress proxy, timeout, redirect limit và SSRF protection; không cho worker truy cập private metadata endpoints.
- Quarantine asset bất thường và ghi audit event; không cố “soft degrade” đối với security violation.

## 4. Media worker sandbox

Worker controller và media executor phải tách credential/capability. Mỗi media/ML executor:

- chạy non-root trong disposable container/process sandbox;
- filesystem read-only ngoài temp workspace và mounted artifact inputs;
- không mount host socket, Docker socket hoặc workspace root;
- CPU/RAM/GPU/disk/PID/time quota;
- outbound network deny-by-default; chỉ mở provider/object-storage endpoints cần thiết;
- không nhận product database credentials hoặc user API keys nếu node không cần;
- temp directory unique theo Job, cleanup sau terminal/timeout;
- subprocess argv list, không `shell=True`, không shell interpolation từ input;
- FFmpeg executable/image pin version; custom executable bị disable production mặc định;
- kill process tree khi timeout/cancel, sau đó đánh dấu worker lease để reconcile.

Controller chỉ có queue/execution-state credential scoped; executor không có PostgreSQL/Redis credential. Provider credential chỉ inject cho exact node/call, không mount file secret chung cho cả render process.

Pipeline phải có `resource_requirements` để scheduler chọn worker tương thích; worker không tự nâng quyền khi thiếu dependency.

## 5. Plugin security

Third-party Python plugin là arbitrary code và có thể đọc filesystem/env, gọi network hoặc execute process.

- Production mặc định disable third-party plugin.
- Chỉ allowlist package/version/hash đã review.
- Plugin registry không auto-enable entry point lạ.
- Nếu cần plugin, chạy subprocess/container riêng với capability tối thiểu.
- Plugin không được nhận raw secret; credential access qua scoped provider port.
- Giữ upstream plugin behavior trong compatibility nhưng không coi plugin V1 là safe mặc định.

## 6. Provider and secret handling

- Credential lưu secret manager hoặc encrypted config; DB chỉ lưu reference/metadata.
- API không gửi provider key tới browser hoặc render worker không cần.
- Provider request log chỉ metadata redacted; text/media content có retention và privacy policy riêng.
- Timeout, retry budget, circuit breaker và rate limit cho từng provider.
- Validate structured LLM/VLM output trước khi đưa vào domain/timeline.
- LLM output không được trở thành executable path, shell arg, SQL hoặc HTML unsanitized.
- Local/offline mode phải có flag kiểm soát outbound network để user biết media có rời máy hay không.
- Edge-TTS là unofficial/reverse-engineered; chỉ dùng local/test/personal theo master plan, không mặc định production commercial.

## 7. Storage security

### PostgreSQL

- TLS in transit, encrypted disks/backups, least-privilege DB roles.
- API role không có schema migration privilege trong production.
- Tenant scoping test bắt buộc; có thể dùng RLS sau khi identity/transaction context chốt.
- Backup/restore được kiểm tra định kỳ; audit/event tables append-only.

### Redis

- private network, ACL/password/TLS tùy deployment; không expose public.
- Không coi Redis là durable audit; stream/pubsub payload không chứa secret.
- TTL cho ephemeral queue/control data và bounded event retention.

### S3/MinIO

- bucket private, block public ACL, encryption at rest, versioning khi cần.
- Presigned URL TTL ngắn, scope exact key/method/content length.
- MinIO không public với default credential; image pin version.
- Verify checksum sau upload/download; lifecycle policy cho temp/proxy/artifact.
- Artifact key normalization phải từ chối absolute/`..`/symlink escape, tương thích guard V1.

## 8. API/web security

- HTTPS qua Caddy/Nginx/Traefik; không truyền API key qua plain HTTP Internet.
- CORS allowlist exact web origins; CSRF protection cho cookie session; SameSite/secure flags.
- Request body/JSON nesting/string limits; pagination bounded.
- Rate limit theo user/workspace/IP; upload initiation và Job submission có quota.
- Idempotency key chống duplicate upload/job.
- ETag/If-Match chống lost update.
- Error response safe, không traceback/path/provider key.
- Content-Security-Policy, X-Frame-Options, secure headers cho frontend.
- Download `Content-Disposition` và content type không được cho phép response header injection.

## 9. Pipeline and timeline safety

- DAG validator chống cycle/resource amplification.
- Timeline validator chống negative time, out-of-range source, clip explosion, unbounded text/track count.
- Render profile allowlist; không nhận arbitrary FFmpeg flags từ user.
- Subtitle/text overlay escape đúng renderer; không render HTML/script.
- LLM-generated source refs phải resolve qua authorization; không được tự tạo path/artifact ID ngoài project.
- QA fail-closed với missing required audio/video/unsafe media; soft degrade chỉ dùng cho capability không an toàn.

## 10. Dependency and supply chain

- Pin/lock dependency và container image versions; scan `pip-audit`/Bandit/Ruff/mypy phù hợp.
- Theo dõi Pillow advisory và MoviePy constraint; không bỏ qua advisory toàn cục mà không có ticket/rationale.
- Target V2 phải funnel subprocess qua một reviewed execution port/wrapper; migration inventory phải theo dõi mọi V1 callsite cho tới khi port xong.
- Verify license/attribution upstream AGPL-3.0-or-later; không xóa LICENSE/attribution khi modify/redistribute.
- CI giữ test matrix/regression V1 và security tests input sanitization.

Upstream evidence: current V1 subprocess calls dùng argv list và audit không tìm thấy `shell=True`/`os.system`, nhưng callsites còn phân tán; Bandit CI bỏ qua B404/B603, vì vậy đây chưa phải proof đầy đủ. Upstream plugin loader auto-load Python entry points; production V2 phải disable/allowlist như mục 5. Upstream Docker chạy non-root UID 10001; Compose MinIO optional dùng image `latest`/default credentials trong dev examples và không được copy sang production.

### Security verification matrix

| Threat | Required control/test |
|---|---|
| Path traversal/symlink | canonical key validation, resolved-root check, encoded/backslash/TOCTOU tests |
| Archive extraction | disabled unless needed; entry count/size/ratio/path/symlink limits |
| SSRF | no arbitrary URL ingestion by default; DNS/IP recheck, redirect/port/private-range deny, egress proxy |
| FFmpeg/subprocess | pinned binary, argv only, option allowlist, timeout/process-tree kill, no input-derived executable |
| Malicious media | disposable non-root executor, resource quotas, probe/scan quarantine, patched codec stack |
| Presigned URL | exact key/method/TTL, content constraints, authorization before sign, no URL logging |
| Object storage | private bucket, non-default credentials, TLS/encryption/version/lifecycle policy |
| Provider secrets | secret references, short-lived injection, pool/node scope, log/event/checkpoint redaction |
| Plugin | disabled by default, allowlist package/version/hash, isolated process/container |
| Upload exhaustion | declared + actual quota, multipart expiry/abort, rate limits, staging sweeper |
| Dependency/supply chain | lock, hashes/images pinned, SBOM/audit, scoped advisory exception with owner/expiry |
| Tenant leakage | repository/API/presign/event stream cross-workspace negative tests |

## 11. Audit, privacy and retention

Audit các action: login/member change, asset upload/delete, script/timeline edit/approve, Job start/pause/resume/cancel, provider config change, artifact download, plugin enable.

Audit event chứa actor/resource/action/result/request ID, không chứa media bytes hoặc secret. Xác định retention cho source media, generated media, prompt/log và event trong `OPEN-QUESTIONS.md` trước production policy.

## 12. Incident controls

- revoke/rotate credential không cần redeploy worker;
- disable provider/plugin/pipeline version nhanh;
- quarantine asset/project;
- stop accepting new jobs trong graceful drain;
- replay từ checkpoint sau worker crash;
- preserve audit/artifact checksum khi điều tra;
- có runbook cho leaked URL, malicious media, provider outage và DB restore.
