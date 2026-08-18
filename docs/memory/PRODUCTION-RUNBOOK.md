---
last_verified: 2026-08-18
source: ../08-SECURITY.md; ../10-DEVELOPMENT-ROADMAP.md; ../13-WORKER-ARCHITECTURE.md; ../PRODUCTION-PLAN.md
owner: operations owner / release owner
---

# Production runbook (planned)

**Readiness:** Gate H controls and local operational commands are implemented, but no public
production release exists. Exact owner/LAN endpoints and secret references stay outside the repo.

## Release progression

| Stage | Exit condition |
|---|---|
| Local development | pinned toolchains and isolated disposable dependencies |
| Local Functional Acceptance | T550 functional/auth/two-worker/client/LAN evidence |
| Local/LAN Hardened Acceptance | T603 security, failure, restore, stability and packaged-client evidence |
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
- Initial retention preserves source/final/resume-required content until explicit audited deletion;
  executor scratch may be cleaned after success. T602 tooling covers backup/restore and inventory;
  the 2026-08-18 local rehearsal proved clean separate-target restore and post-restore lineage/API
  read-through; physical second-device and public deployment remain separate gates.
- Capture opaque correlation and Job/Run/Step IDs, deployment identity and safe error category;
  redact tokens, credentials, paths and user content.

## Gate H local commands

- `tools/reconcile-local.ps1` calls the authenticated recovery operation; it requeues only
  PostgreSQL-authorized ready/retrying frontiers.
- `tools/backup-local.ps1` writes a custom PostgreSQL dump and checksum manifest outside the repo.
- `tools/restore-local.ps1` restores only into a separately named empty `restore/recovery/rehearsal`
  target and never uses `--clean`.
- `tools/inventory-artifacts.ps1` reports missing, orphan and checksum/size-corrupt objects; it
  never deletes objects.
- `tools/verify-gate-h.ps1` runs the deterministic T600–T602 suites and parser/independence checks;
  `-IncludeLive` runs the reviewed-FFmpeg Product API flow and writes temporary evidence.
