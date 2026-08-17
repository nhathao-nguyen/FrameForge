---
last_verified: 2026-08-17
source: ../OPEN-QUESTIONS.md; ../IMPLEMENTATION-ORDER.md
owner: repository owner / decision owners to be assigned
---

# Open questions mirror

[`../OPEN-QUESTIONS.md`](../OPEN-QUESTIONS.md) is authoritative.

| ID | Status | Category | Blocking work | Owner | Deadline |
|---|---|---|---|---|---|
| OQ-01 | OPEN | owner identity decision | T106, T201, T210, T430 | project owner | not set |
| OQ-05 | OPEN | owner credential/secret decision | T102, T203, T235, T500 | project owner | not set |
| OQ-06 | OPEN | owner Workspace decision | T201, T210 and scoped resources | project owner | not set |
| OQ-10 | OPEN | owner retention/privacy decision | T602 and deletion policy | project owner | not set |
| OQ-02 | OPEN | implementation projection choice | projection-specific T208 | task owner | not set |
| OQ-03 | OPEN | implementation queue primitive | T310-T312 | task owner | not set |
| OQ-04 | OPEN | implementation progress transport | T340 | task owner | not set |
| OQ-08 | OPEN | implementation edit transport | T233, T432 | task owner | not set |
| OQ-07 | DEFERRED | later embedding storage | persistent part of T523 | project owner | later phase |
| OQ-11 | DEFERRED | later pipeline authoring | public/admin authoring only | project owner | later phase |
| OQ-09 | DECIDED | no upstream task mapping | none | repository owner | 2026-08-17 |
| OQ-12 | DECIDED | NH-Media and `nh_media` names | none | repository owner | 2026-08-17 |
| OQ-13 | DECIDED | Go API/media plus isolated Python ML | none | repository owner | 2026-08-17 |
| OQ-14 | DECIDED | upstream is research/reference only | none | repository owner | 2026-08-17 |
| OQ-15 | DECIDED | Tauri 2 thin remote client | T434 evidence only | repository owner | 2026-08-17 |

When a question changes, update the authoritative OQ, affected specs, task dependencies, decisions
and this mirror in one reviewed change.
