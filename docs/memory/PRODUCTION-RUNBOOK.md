---
last_verified: 2026-08-16
source: ../08-SECURITY.md; ../10-DEVELOPMENT-ROADMAP.md; ../13-WORKER-ARCHITECTURE.md; ../14-DEVELOPMENT-ENVIRONMENT.md
owner: operations owner / release owner
---

# Production runbook (planned)

**Readiness:** not production-ready. This runbook records required controls and the release order;
commands, image digests, endpoints and credentials must be filled by the authorized implementation
and deployment tasks. Never put secrets or presigned URLs in this file.

## Environments

| Environment | Purpose | Required state |
|---|---|---|
| Local/dev | docs and unit/integration development | uv-managed Python, pinned FFmpeg, private disposable PostgreSQL/Redis/MinIO |
| Staging | production-like acceptance | pinned images, private object storage, sandboxed worker, seeded test fixtures |
| Canary | limited production traffic | feature flag, metrics/alerts, rollback command tested |
| Production | approved release | all phase evidence, backup/restore drill, owner sign-off and compatibility gate |

## Branch and release protection

After CI exists, protect `develop` and release branches with pull requests, required checks,
minimum review, migration/security checks and force-push disabled. Configure this on the repository
host only after the owner confirms the policy; no remote settings were changed by this task.

## Deployment sequence

1. Verify clean worktree, approved commit, CI/security/license scans and release evidence.
2. Snapshot configuration references and confirm no secret values are in logs or artifacts.
3. Apply forward-compatible database migrations; verify schema invariants and migration report.
4. Deploy API/controller and worker versions that understand the same contract.
5. Drain old workers before incompatible state transitions; reconcile leases/checkpoints.
6. Run health, readiness, upload-validation, queue, artifact and three-profile smoke checks.
7. Enable canary/feature flag, watch Job/Run/Step failures, latency, DLQ, storage and provider
   metrics, then expand only after the release owner accepts evidence.

## Rollback

- Stop new traffic/claims with the deployment flag; let safe attempts finish or cancel/drain them.
- Reconcile expired leases and staged artifacts before switching versions.
- Roll back API/worker to the last contract-compatible image; never downgrade blindly across a
  destructive migration.
- Restore PostgreSQL/object storage only under the approved restore procedure, then verify Artifact
  checksum inventory and replay/outbox consistency.
- If V2 behavior is incompatible, route the affected path to `LegacyMovieNarratorAdapter` and the
  frozen V1 environment. Do not delete new artifacts or legacy source during the first rollback.
- Record incident, affected Job IDs (opaque identifiers only), evidence and follow-up migration.

## Worker operations

- Drain rejects new claims, bounds active attempts, kills timed-out process trees and reconciles
  staged output.
- A worker restart waits for lease expiry, verifies committed checkpoints/artifacts and resumes the
  first incomplete compatible boundary.
- Retry transient failures only. Validation, security, user-input and stale-snapshot failures do
  not receive an automatic retry.
- DLQ inspection/replay is an audited command with idempotency and authorization.

## Backup/restore and retention

- Back up PostgreSQL metadata and private object storage according to the owner-approved retention
  policy; OQ-10 is still open.
- Run a clean-environment restore drill before production and after material storage changes.
- Verify that every committed Artifact has the expected checksum and that no protected source,
  approved version or active checkpoint is deleted prematurely.
- Physical deletion requires soft-delete, reference, retention and legal-hold checks.

## Incident response

Capture correlation ID, Job/Run/Step IDs, safe error category, deployment commit, provider/storage
operation and timeline of state transitions. Redact tokens, credentials, prompts containing secrets,
absolute paths and user content. Preserve append-only events and artifact provenance for analysis.

## Health and observability

Liveness must not claim dependency health. Readiness covers required dependencies; deep diagnostics
are authenticated/operational-only. Monitor structured logs, metrics, traces, queue age, lease loss,
retry/DLQ rates, upload quarantine, provider error categories, render QA and storage inventory.
