---
last_verified: 2026-08-16
source: ../08-SECURITY.md; ../12-STORAGE-ARCHITECTURE.md; ../13-WORKER-ARCHITECTURE.md; ../14-DEVELOPMENT-ENVIRONMENT.md
owner: security owner / release owner
---

# Security controls

Status values below describe the current specification/evidence state, not production completion.
Most controls are `specified / implementation evidence pending` because the documentation gate is
not ratified.

| Control | Required behavior | Evidence/status |
|---|---|---|
| Upload quarantine | staged bytes are checksum/MIME/magic/probe/scan validated before `Asset=ready` | specified; T224/T600 pending |
| Direct upload | browser uses short-lived scoped multipart URLs; Product API does not proxy large media | specified; T223 pending |
| Path and symlink guards | normalize/contain paths, reject traversal/symlink/archive escapes | specified; T224/T600 pending |
| Sandbox | non-root, read-only root, no-new-privileges, isolated workspace, CPU/RAM/PID/disk/time quotas | specified; T313/T600 pending |
| Process control | argv-list subprocess wrapper, process-tree kill, no `shell=True` | invariant; security test pending |
| Egress | deny by default; allow only declared object-storage/provider endpoints | specified; T600 pending |
| Secret scope | secret-manager references only; executor receives exact required provider secret, never broad DB/Redis/user secrets | specified; T102/T235/T500/T600 pending |
| API authz | Workspace/Project ownership is checked at repository boundary; negative cross-owner tests | blocked by OQ-01/OQ-06; T210 pending |
| Safe errors | public errors expose stable safe code/message, not traceback, secret or absolute path | specified; T101/T106 pending |
| Presigned URLs | short TTL, exact key/method/constraints, never stored in events/checkpoints | specified; T223 pending |
| Plugin policy | third-party plugins disabled/allowlisted and isolated; no auto-load | specified; T600 pending |
| Dependency/image scan | pinned dependencies/images, license review, scoped expiring exceptions | Phase 0/T108/T600 pending |
| Observability redaction | structured logs/events/checkpoints redact tokens, paths and sensitive user/provider data | T102/T601 pending |
| Backup/restore | PostgreSQL/object backup and clean restore with Artifact inventory checksum | T602 pending |

## Review checklist

Before release, confirm malicious-media suite, cross-Workspace negative tests, secret/redaction
tests, subprocess audit, dependency/image/license scans, worker crash/kill/drain drill, restore
drill and no raw path/secret/traceback leakage. A green compile or unit test is not sufficient.
