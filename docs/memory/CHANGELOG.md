---
last_verified: 2026-08-17
source: owner instruction; git history; docs/memory/
owner: repository owner / task assignee
---

# Memory changelog

## 2026-08-17 — T002/T003 pre-code bootstrap evidence

- Pinned the validated Windows baseline with `.go-version`, `.python-version`, `.node-version`,
  `package.json` package-manager metadata and `rust-toolchain.toml`.
- Recorded exact Go/Python/uv/Node/pnpm/Rust/Tauri/FFmpeg/ffprobe/Docker evidence and smoke checks
  in `docs/bootstrap/T002-TOOLCHAIN-MATRIX-WINDOWS.md`.
- Added the T003 reference-behavior policy and empty manifest under `tests/reference-behavior/`.
- Reconciled current branch/status memory and made the branch validator compare against the recorded
  branch rather than a stale hard-coded branch.
- No application code, database schema, service/worker/client runtime or upstream dependency was added.

## 2026-08-17 — Final Local/LAN decision closure

- Recorded owner ratification of LocalAuthProvider, Workspace-first authorization, encrypted
  SecretStore records, Timeline JSONB/domain commands, Redis Streams, SSE and MinIO/S3 semantics.
- Closed all 15 former OQs; OQ-07/OQ-11 are deferred nonblocking with concrete baselines.
- Marked T004 complete, added T323 Python worker proof and T550 Local Functional Acceptance, and
  separated T603 Local/LAN Hardened Acceptance from T605 Internet/VPS Production.
- Updated Local/LAN HTTP/CORS/binding, retention/delete, health and canonical Windows startup targets.
- No application code, migration, service/client/worker scaffold or runtime configuration was added.

## 2026-08-17 — Independent specification refactor

- Replaced the former wrapper/migration/compatibility architecture with an independent NH-Media
  architecture.
- Corrected names and topology: NH-Media, `nh_media`, `github.com/nhathao-nguyen/NH-Media`, Go
  Product API, Go media workers, isolated Python ML/AI workers, Next.js web and Tauri 2 client.
- Added upstream reference policy and detailed capability matrix; converted the module audit and
  prior source baseline into non-normative research evidence.
- Added native candidate evaluation/selection, ReferenceStyleAnalysis, scheduling/distributed
  evolution and safer extension direction.
- Replaced migration phases with independent implementation order and a first native vertical slice.
- Made Local/LAN certification a release milestone before public production.
- Updated the memory model and OQ/task mirrors. No application code, runtime, migration or
  deployment implementation was added.

Final documentation/link/table/JSON/task/OQ/terminology/change-boundary checks are recorded in
[`TEST-EVIDENCE.md`](TEST-EVIDENCE.md). T002/T003 then T004 are the next evidence gates.
