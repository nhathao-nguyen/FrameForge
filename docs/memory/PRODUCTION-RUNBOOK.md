---
last_verified: 2026-08-17
source: ../08-SECURITY.md; ../10-DEVELOPMENT-ROADMAP.md; ../13-WORKER-ARCHITECTURE.md; ../PRODUCTION-PLAN.md
owner: operations owner / release owner
---

# Production runbook (planned)

**Readiness:** no runtime or production release exists. This file records required operational
behavior; exact commands, image digests, endpoints and secret references belong to later tasks.

## Release progression

| Stage | Exit condition |
|---|---|
| Local development | pinned toolchains and isolated disposable dependencies |
| Local/LAN release | T603 functional, security, failure and desktop/web evidence |
| Staging/public candidate | owner-selected DNS/TLS/services plus production-like drills |
| Internet production | T605 sign-off, canary and rollback evidence |

## Deployment sequence

1. Verify approved commit, dependency/license/security scans and absence of upstream runtime/build
   dependencies.
2. Apply forward-compatible database migrations and verify schema/inventory.
3. Deploy contract-compatible Go API, Go media and Python `nh_media` worker versions.
4. Run readiness, upload quarantine, queue/lease, Artifact, client and render smoke tests.
5. For public production, enable bounded canary traffic and inspect SLO/queue/DLQ/provider/storage
   evidence before expansion.

## Rollback

- Stop new claims, drain bounded attempts and reconcile leases/staged Artifacts.
- Roll back to the last contract-compatible **NH-Media** release; never to Movie Narrator.
- Do not blindly downgrade across destructive schema changes.
- Restore PostgreSQL/object storage only through the tested procedure and verify Artifact checksums,
  outbox/event continuity and authorization.

## Operations

- Executor restarts resume from committed compatible checkpoints, not Redis/local disk.
- Retry only declared transient failures; security/validation/stale-snapshot failures are not
  automatically retried.
- DLQ replay is authorized, audited and idempotent.
- Retention/deletion remains blocked on OQ-10; preserve referenced/protected content.
- Capture opaque correlation and Job/Run/Step IDs, deployment identity and safe error category;
  redact tokens, credentials, paths and user content.
