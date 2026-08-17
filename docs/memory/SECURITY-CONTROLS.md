---
last_verified: 2026-08-17
source: ../08-SECURITY.md; ../12-STORAGE-ARCHITECTURE.md; ../13-WORKER-ARCHITECTURE.md
owner: security owner / release owner
---

# Security controls

All controls are specified; implementation evidence is pending.

| Control | Required behavior | Evidence/status |
|---|---|---|
| Upstream independence | normal build/test/runtime/deploy succeeds with no upstream checkout/package/image | T004 spec pass; T108 runtime gate pending |
| Upload quarantine | staged bytes are checksum/MIME/magic/probe/scan validated before ready | T224/T600 pending |
| Direct upload | clients use short-lived scoped upload URLs; API does not proxy large media | T223 pending |
| Path/symlink guards | normalize/contain paths and reject traversal/archive escapes | T224/T600 pending |
| Sandbox | non-root, read-only, isolated workspace with CPU/RAM/PID/disk/time bounds | T313/T600 pending |
| Process control | argv-list wrapper, allowlisted tools/options, process-tree kill, no `shell=True` | T313/T600 pending |
| Egress | deny by default; allow only declared storage/provider endpoints | T600 pending |
| Secret scope | encrypted SecretStore refs under server master key; exact capability only; no DB/Redis/user secrets in executor | decision resolved; T102/T500/T600 pending |
| API authz | LocalAuthProvider plus Workspace/Project scope at repository boundary with negative tests | decision resolved; T201/T210 pending |
| Safe errors/redaction | no traceback, secret, absolute path or sensitive payload in public/telemetry contracts | T102/T601 pending |
| Extension policy | no unrestricted plugin auto-load; reviewed manifests and isolation only | later explicit task/evidence |
| Dependency/license scans | pinned inputs, reviewed licenses, scoped expiring exceptions | T108/T600 pending |
| Backup/restore | manual/on-demand initial DB backup; clean restore plus Artifact inventory checksum | default resolved; T602 evidence pending |

Before release, require malicious-media, authz-negative, secret/redaction, subprocess, dependency,
license, crash/drain and restore evidence. Compilation alone is not sufficient.
