# Final Specification Audit

Audit date: 2026-08-17 (Asia/Saigon).

## Overall status

**SPEC READY FOR IMPLEMENTATION**

Owner ratification dated 2026-08-17 is authoritative and T004 is approved. T002 and T003 bootstrap
evidence are now complete. This report certifies specification and pre-code consistency only: no
Product API, schema migration, worker, web/Tauri application or runtime infrastructure was
implemented by this task.

## Final architecture assertions

```text
Product name: NH-Media
Python namespace: nh_media

Independent implementation: YES
Movie Narrator runtime dependency: NO
Movie Narrator build dependency: NO
Movie Narrator deployment dependency: NO
Movie Narrator import dependency: NO
Legacy compatibility service: NO

Local execution supported: YES
LAN testing supported: YES
VPS required for implementation: NO
Public TLS required for functional acceptance: NO

Authentication architecture resolved: YES
Authorization architecture resolved: YES
Workspace model resolved: YES
Secrets architecture resolved: YES
Timeline representation resolved: YES
Timeline command model resolved: YES
Queue technology resolved: YES
Progress mechanism resolved: YES
Retention initial policy resolved: YES

Open architectural questions remaining: 0
Owner decisions blocking implementation: 0
```

## Decisions closed

| Former question | Authoritative resolution |
|---|---|
| OQ-01 authentication | AuthPort with LocalAuthProvider initially; real server sessions; future OIDC adapter is nonblocking. |
| OQ-02 timeline projection | Versioned PostgreSQL JSONB is canonical; optional projections are rebuildable and noncanonical. |
| OQ-03 queue | Redis Streams consumer groups; PostgreSQL is authoritative Job/attempt/DLQ state. |
| OQ-04 progress | SSE primary with reconnect/replay; REST owns commands and authoritative queries. |
| OQ-05 secrets | Workspace-owned credential default; encrypted SecretStore records under a server-owned master key. |
| OQ-06 Workspace | First-class Workspace plus local admin/default Workspace/owner-membership bootstrap. |
| OQ-07 embedding store | Artifact + item-index baseline; later pgvector/external benchmark is deferred nonblocking. |
| OQ-08 timeline editing | Typed domain commands with optimistic concurrency; privileged validated import only. |
| OQ-09 upstream tasks | No upstream task/runtime mapping or compatibility surface. |
| OQ-10 retention | Retain protected source/intermediate/final data until explicit audited delete; scratch cleanup after success. |
| OQ-11 pipeline authoring | Built-in versioned Pipelines initially; later admin declarative authoring is deferred nonblocking; arbitrary user code excluded. |
| OQ-12 identity | NH-Media, `nh_media`, `github.com/nhathao-nguyen/NH-Media`. |
| OQ-13 topology | Go Product API and media worker/FFmpeg; isolated Python ML worker. |
| OQ-14 upstream relation | Movie Narrator is research/reference-only; no runtime/build/deploy/import/migration/rollback dependency. |
| OQ-15 desktop | Tauri 2 thin remote-first Product API client with no server compute or secrets. |

## Local/LAN readiness

- Local profile binds loopback, keeps authentication active and uses LocalAuth/default Workspace
  bootstrap.
- LAN profile is explicit: configurable bind/interface, public API URL and exact CORS origins; no
  fixed IP is embedded.
- Trusted private-LAN HTTP is allowed for Local Functional Acceptance with an insecure-LAN warning;
  Internet exposure is prohibited in that profile.
- PostgreSQL, Redis Streams, MinIO, Product API and both workers remain server-side. Browser and
  Tauri clients call the Product API only.
- Moving later to a VPS/cloud host changes deployment/networking/TLS/secret/storage adapters and
  scale topology, not domain, API, worker, pipeline or Job contracts.

```text
Local development: READY
LAN testing architecture: READY
VPS required: NO
Public TLS required now: NO
```

## Execution and milestone readiness

The first deterministic vertical slice is client/API → PostgreSQL Job/outbox → Redis Streams → Go
media worker → ffprobe/FFmpeg → MinIO Artifact → SSE/result. T323 then proves a minimal Python
`nh_media` task without LLM, Whisper, CUDA, VLM or TTS. T550 certifies `LOCAL FUNCTIONAL ACCEPTANCE`.
T603 is the separate `LOCAL/LAN HARDENED ACCEPTANCE`; T605 is `INTERNET / VPS PRODUCTION`.

Retry, timeout, provider throttling, resource exhaustion, cancellation, invalid input, idempotency,
lease recovery, canonical PostgreSQL DLQ records and checkpoint invalidation are specified. Health
uses minimal `/live` and dependency-aware `/ready`. Large bytes use logical object keys and never
PostgreSQL blobs or durable cross-boundary paths.

## Deferred nonblocking choices

Exact future VPS, public domain, OIDC vendor, KMS/Vault provider, cloud S3 vendor, monitoring SaaS,
public ingress/certificate automation, embedding index optimization and declarative pipeline-authoring
UX are `DEFERRED-NONBLOCKING`. Each has a valid initial baseline and cannot block current coding.

## Gate result

- T004 final documentation/independence gate: **APPROVED / COMPLETE**.
- Open architectural/product questions: **NONE**.
- Owner-decision blockers: **NONE**.
- Application code changed by this task: **NO**.
- T002 toolchain pinning: **COMPLETE**; Windows evidence is recorded in `docs/bootstrap/`.
- T003 reference-fixture policy: **COMPLETE**; policy and empty manifest are recorded in
  `tests/reference-behavior/`.
- Next implementation task: T100 independent repository skeleton.

NEXT EXECUTION TARGET: REPOSITORY AND SERVICE FOUNDATION (T100)

SPEC READY FOR IMPLEMENTATION
NEXT TARGET: REPOSITORY AND SERVICE FOUNDATION (T100)
