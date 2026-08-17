---
last_verified: 2026-08-17
source: ../SPEC-AUDIT-REPORT.md; ../IMPLEMENTATION-ORDER.md; git metadata
owner: task assignee / release owner
---

# Test evidence

Evidence is append-only where practical and must state limitations.

| Date | Task/check | Command/evidence | Environment/commit | Result | Limitation |
|---|---|---|---|---|---|
| 2026-08-16 | Upstream research provenance | recorded git identity/tag/tree/inventory inspection | external checkout at `bc2d276cf477fe3ce1a16f9679dcb1d2978e3a74` | pass at inspection time | not reverified; not runtime/parity/rollback evidence |
| 2026-08-17 | Starting repository boundary | branch/status/HEAD inspection | `docs/architecture-spec` at `1cab87bbfd2801aa552948f8e8f8db9b3e2ffb0c` | clean before refactor | proves starting state only |
| 2026-08-17 | T000 documentation consistency | `python tools/check_memory.py --max-age 0` | current working tree | pass: 12 memory files, 15 OQs, 81 tasks; all internal links, Markdown tables and JSON/JSONL fences valid | documentation structure only; no runtime behavior |
| 2026-08-17 | Capability/terminology audit | repository-wide `rg` classifications plus matrix class counts | current working tree | pass: 25 REIMPLEMENT, 6 ADOPT-CONCEPT, 11 IMPROVE, 1 REFERENCE, 10 DEFER, 3 IGNORE; remaining legacy/version terms are negative, provenance or generic native concepts | semantic review, not a source-license opinion |
| 2026-08-17 | Change hygiene | `git diff --check`; trailing-whitespace and secret-assignment scans; status/file-boundary/reference-tree checks | current working tree on `docs/architecture-spec` | pass: docs plus validator only; no vendored upstream tree or application source | Git reports expected LF-to-CRLF working-copy warnings only; no commit/push/merge performed |
| 2026-08-17 | T004 final specification consistency | `python tools/check_memory.py --max-age 0` plus validator syntax compile | current working tree | pass: 12 memory files, 15 closed OQs, 83 mirrored tasks; links/tables/JSON fences/dependencies valid | documentation and validation tooling only; not runtime acceptance |
| 2026-08-17 | Decision closure and final gate | unresolved-status/false-blocker `rg` scan; final-status assertion scan; OQ mirror count | current working tree | pass: no OPEN/BLOCKED owner/spec status, 15 allowed OQ statuses, required SPEC/NEXT assertions present | historical/negative upstream and canonical JobStep `blocked` terms remain intentionally |
| 2026-08-17 | Final change boundary | `git diff --check`; changed-path allowlist; upstream-runtime name scan outside documentation | current working tree on `docs/architecture-spec` | pass: Markdown plus `tools/check_memory.py` only; no application/runtime/upstream source | expected Git LF-to-CRLF warnings only; no commit/push/merge performed |
| 2026-08-17 | T002 Windows toolchain bootstrap | `git/go/python/uv/node/pnpm/rust/tauri/ffmpeg/ffprobe/docker` version checks; MSVC/WebView2 checks; paired FFmpeg encode/probe smoke | `implementation/bootstrap` at `5df7308`, Windows 11 Pro PowerShell 7.6.4 | pass: selected pins and hashes recorded in `docs/bootstrap/T002-TOOLCHAIN-MATRIX-WINDOWS.md` | no application module, lockfile, ML environment or compose stack created |
| 2026-08-17 | T003 reference-behavior policy | JSON manifest parse/schema/empty-fixture assertion; absent `references` checkout check; policy review | `implementation/bootstrap` at `5df7308` | pass: optional recorded research only; normal upstream execution prohibited | no fixture comparison is claimed; manifest intentionally empty |
| 2026-08-17 | T004 final pre-code certification | memory/link/table/JSON checks; task/OQ/status parity; forbidden upstream/source scan; `git diff --check` | current working tree on `implementation/bootstrap` | pass: T002/T003 complete, T100 eligible, no application code | Docker and Tauri prerequisites are environment evidence, not runtime acceptance |

No runtime, API/worker/client, fixture comparison, malicious-media, load, backup/restore, Local/LAN
or public production evidence is claimed; T002 is limited to toolchain prerequisites and T003 has
an intentionally empty manifest.
