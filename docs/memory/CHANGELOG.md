---
last_verified: 2026-08-17
source: owner instruction; git history; docs/memory/
owner: repository owner / task assignee
---

# Memory changelog

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
