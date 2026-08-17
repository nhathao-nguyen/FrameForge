# 00 — Project Context

## Status and authority

This specification implements the owner-confirmed 2026-08-17 direction and final Local/LAN-first
ratification: NH-Media is an independent
product; Movie Narrator is research/reference-only. This decision supersedes every prior target
that required an upstream runtime, adapter, compatibility service, migration path or rollback image.
Historical research facts remain evidence, not architecture.

This change completes the specification phase without adding application code. The ratified gate
authorizes the ordered bootstrap/implementation tasks after their explicit evidence prerequisites.

## Product identity

- Product: **NH-Media**.
- Go module: `github.com/nhathao-nguyen/NH-Media`.
- Python ML/AI namespace: `nh_media`.
- `movie_narrator`: upstream research identifier only, never an NH-Media public/runtime namespace.

## Goal

Build an independent AI Video Production System with:

- web and Tauri 2 desktop clients using one Product API;
- versioned Projects, Scripts, Timelines and Renders;
- durable asynchronous pipelines with checkpoint, pause/resume, review and retry;
- TimelineVersion as renderer source of truth;
- provider abstractions for LLM, VLM, TTS, ASR and Embedding;
- bounded Go media workers and isolated Python `nh_media` workers;
- PostgreSQL metadata, Redis Streams coordination and private MinIO/S3-compatible object storage;
- multiple output profiles without rerunning unrelated AI work;
- future distributed execution without requiring it initially.

Movie recap is the first workflow, not the product boundary.

## System boundary

```text
Web / Tauri 2 clients
          │ versioned Product API + shared SDK
          ▼
Go Product API / control plane
  ├── PostgreSQL: durable product and job state
  ├── Redis Streams: consumer-group execution transport and coordination
  └── MinIO/S3-compatible object storage: source media and Artifacts
          │ versioned worker contract
          ├── Go media worker → FFmpeg/ffprobe
          └── Python ML worker → nh_media / models
```

There is no upstream runtime branch in this topology.

### Product backend

Owns identity integration, Workspace/Project authorization, Asset/Artifact registration, Job
commands, database transactions, uploads, idempotency and public API/events. It does not contain
media/AI algorithms or execute long work inline.

### Compute workers

Own media and AI execution through declared node contracts. Workers have no product user, billing,
session or UI responsibility. Executors are disposable and least-privilege.

### Web and desktop

Both clients call Product API only. Tauri remains thin and remote-first; it does not bundle Python,
FFmpeg processing, models, PostgreSQL, Redis, server secrets or worker services.

## Operating profiles

### Local/LAN production-like validation

A server on the LAN runs API, PostgreSQL, Redis Streams, MinIO and workers. Other LAN machines use the
web or desktop client. This is sufficient to prove architecture, jobs, rendering, recovery,
persistence and multi-client behavior. Lack of a VPS does not block functional implementation.
Local-only binding is the default; LAN binding, API URL and allowed origins are explicit profile
configuration. Authentication remains active. Trusted-private-LAN HTTP is permitted for the first
functional milestone and must be labelled as unsuitable for Internet exposure.

### Internet production

Public DNS, managed TLS, public ingress, CDN, canary and internet threat hardening are later release
concerns. Internal services remain private in every profile.

## Core invariants

1. NH-Media builds, tests, deploys and runs without Movie Narrator.
2. Product API/control plane is Go and uses `github.com/nhathao-nguyen/NH-Media`.
3. Python AI/ML code uses `nh_media`; `movie_narrator` is never imported by product code.
4. Web and desktop never call workers/providers directly.
5. PostgreSQL is source of durable product state; Redis is not.
6. Asset/Artifact refs cross boundaries; durable raw paths do not.
7. TimelineVersion is renderer source of truth; AI proposals never silently replace user edits.
8. Job/JobStep states use `GLOSSARY.md` exactly.
9. Go↔Python uses versioned language-neutral contracts.
10. Worker concurrency, retries, leases, cancellation and resources are bounded.
11. No `shell=True`, untrusted extension auto-load or secret/path/traceback exposure.
12. Upstream code, containers and dependency graphs remain outside product/runtime/release packages.
13. LocalAuthProvider behind AuthPort bootstraps one local admin, one Workspace and one membership.
14. Provider secrets use encrypted server-side records protected by a server-owned master key.
15. TimelineVersion uses versioned PostgreSQL JSONB and changes only through validated domain commands.
16. SSE is the primary progress transport; REST/database state remains authoritative.

## Upstream reference boundary

Useful upstream knowledge is retained in:

- [`UPSTREAM-REFERENCE-POLICY.md`](UPSTREAM-REFERENCE-POLICY.md);
- [`UPSTREAM-CAPABILITY-MATRIX.md`](UPSTREAM-CAPABILITY-MATRIX.md);
- [`UPSTREAM-MODULE-AUDIT.md`](UPSTREAM-MODULE-AUDIT.md);
- [`baselines/upstream-movie-narrator-v1.1.0.md`](baselines/upstream-movie-narrator-v1.1.0.md).

These are research/provenance records. Normal development and CI use NH-Media-owned tests. Optional
comparison fixtures do not make upstream behavior or implementation a permanent compatibility
contract.

## Document map

| File | Purpose |
|---|---|
| `01-ARCHITECTURE.md` | component ownership, dependency direction, deployment |
| `02-DOMAIN-MODEL.md` | NH-Media-native entities and lifecycle |
| `03-DATABASE-SCHEMA.md` | PostgreSQL persistence contract |
| `04-API-CONTRACT.md` | NH-Media-native Product API |
| `05-PIPELINE-SPEC.md` | DAG and independent node catalog |
| `06-TIMELINE-SPEC.md` | canonical TimelineVersion contract |
| `07-EVENTS-AND-JOBS.md` | job states, events, retry, replay |
| `08-SECURITY.md` | trust boundaries and controls |
| `09-INDEPENDENT-IMPLEMENTATION-FROM-REFERENCE.md` | reference-to-implementation method |
| `10-DEVELOPMENT-ROADMAP.md` | phase gates and first vertical slice |
| `11-PROVIDER-ARCHITECTURE.md` | provider ports and policy |
| `12-STORAGE-ARCHITECTURE.md` | Asset/Artifact/storage/upload |
| `13-WORKER-ARCHITECTURE.md` | Go/Python worker protocol and scaling |
| `14-DEVELOPMENT-ENVIRONMENT.md` | reproducible toolchains and Local/LAN setup |
| `UPSTREAM-CAPABILITY-MATRIX.md` | capability-level disposition and acceptance |
| `UPSTREAM-REFERENCE-POLICY.md` | provenance, license and allowed research use |
| `IMPLEMENTATION-ORDER.md` | task order and definitions of done |
| `OPEN-QUESTIONS.md` | closed decisions and deferred nonblocking operational choices |

## Specification completion

The specification gate is approved when independence assertions pass, every meaningful upstream
capability has a documented disposition, links/terms are consistent, the independent first slice is
defined and no unresolved architecture/product decision remains. Readiness does not mean application
code was written or runtime acceptance has already passed.
