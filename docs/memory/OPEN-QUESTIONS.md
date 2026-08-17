---
last_verified: 2026-08-17
source: ../OPEN-QUESTIONS.md; ../IMPLEMENTATION-ORDER.md
owner: repository owner / task assignee
---

# Decision closure mirror

[`../OPEN-QUESTIONS.md`](../OPEN-QUESTIONS.md) is authoritative. No architecture/product question
remains open or blocks implementation.

| ID | Status | Final decision | Blocking work | Owner/date |
|---|---|---|---|---|
| OQ-01 | RESOLVED | AuthPort with LocalAuthProvider; future OIDC adapter | none | repository owner / 2026-08-17 |
| OQ-02 | RESOLVED | canonical versioned Timeline JSONB; optional rebuildable projections later | none | repository owner / 2026-08-17 |
| OQ-03 | RESOLVED | Redis Streams consumer groups; PostgreSQL canonical | none | repository owner / 2026-08-17 |
| OQ-04 | RESOLVED | SSE primary; REST commands/authoritative queries | none | repository owner / 2026-08-17 |
| OQ-05 | RESOLVED | encrypted SecretStore records with server-owned master key | none | repository owner / 2026-08-17 |
| OQ-06 | RESOLVED | Workspace-first plus local admin/default Workspace bootstrap | none | repository owner / 2026-08-17 |
| OQ-07 | DEFERRED-NONBLOCKING | embedding Artifact + item-index baseline; benchmark later stores | none | repository owner / 2026-08-17 |
| OQ-08 | RESOLVED | typed Timeline domain commands and optimistic concurrency | none | repository owner / 2026-08-17 |
| OQ-09 | RESOLVED | no upstream task/runtime mapping | none | repository owner / 2026-08-17 |
| OQ-10 | RESOLVED | retain protected data until explicit delete; scratch cleanup after success | none | repository owner / 2026-08-17 |
| OQ-11 | DEFERRED-NONBLOCKING | built-in Pipelines initially; later declarative admin authoring | none | repository owner / 2026-08-17 |
| OQ-12 | RESOLVED | NH-Media, `nh_media`, canonical Go module | none | repository owner / 2026-08-17 |
| OQ-13 | RESOLVED | Go API/media worker and isolated Python ML worker | none | repository owner / 2026-08-17 |
| OQ-14 | RESOLVED | Movie Narrator research/reference only | none | repository owner / 2026-08-17 |
| OQ-15 | RESOLVED | Tauri 2 thin remote-first client | none | repository owner / 2026-08-17 |

Future VPS/domain/OIDC vendor/KMS/cloud storage/monitoring choices are deployment configuration and
`DEFERRED-NONBLOCKING`; they cannot block Local Functional Acceptance.
