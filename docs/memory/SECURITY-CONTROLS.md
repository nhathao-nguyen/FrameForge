---
last_verified: 2026-08-19
source: ../08-SECURITY.md; ../12-STORAGE-ARCHITECTURE.md; ../13-WORKER-ARCHITECTURE.md
owner: security owner / release owner
---

# Security controls

Gate H implementation evidence is recorded in `../evidence/gate-h-implementation-20260818.md`.

| Control | Required behavior | Evidence/status |
|---|---|---|
| Upstream independence | normal build/test/runtime/deploy succeeds with no upstream checkout/package/image | T004 spec pass; T108 runtime gate pending |
| Upload quarantine | staged bytes are checksum/MIME/magic/probe/scan validated before ready | T224 deterministic probe/quarantine corpus pass; worker scan/orchestration remains T300/T600 |
| Direct upload | clients use short-lived scoped upload URLs; API does not proxy large media | T223 pending |
| Path/symlink guards | normalize/contain paths and reject traversal/archive escapes | T224 + T600 policy/process tests pass |
| Sandbox | non-root, read-only, isolated workspace with CPU/RAM/PID/disk/time bounds | T600 process/input/output/time bounds pass; OS/container enforcement remains residual |
| Process control | argv-list wrapper, allowlisted tools/options, process-tree kill, no `shell=True` | T313 + T600 direct argv/tree-kill implementation pass |
| Egress | deny by default; allow only declared storage/provider endpoints | T600 exact endpoint/SSRF policy tests pass |
| Secret scope | encrypted SecretStore refs under server master key; exact capability only; no DB/Redis/user secrets in executor | T102/T500 + T600 capability/redaction tests pass |
| API authz | LocalAuthProvider plus Workspace/Project scope at repository boundary with negative tests | decision resolved; T201/T210 pending |
| Safe errors/redaction | no traceback, secret, absolute path or sensitive payload in public/telemetry contracts | T102/T601 + T600 redaction tests pass |
| Extension policy | no unrestricted plugin auto-load; reviewed manifests and isolation only | T600 default-deny manifest policy pass |
| Dependency/license scans | pinned inputs, reviewed licenses, scoped expiring exceptions | prior T108/Gate G scans pass; current Gate H scan is in runner |
| Backup/restore | manual/on-demand initial DB backup; clean restore plus Artifact inventory checksum | T602 live dump/manifest, clean separate-target restore/migration, lineage/API read-through and negative tests pass; orphan report is retained for owner review |

Before release, require malicious-media, authz-negative, secret/redaction, subprocess, dependency,
license, crash/drain and restore evidence. Compilation alone is not sufficient.
