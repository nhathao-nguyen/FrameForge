---
last_verified: 2026-08-16
source: ../OPEN-QUESTIONS.md; ../IMPLEMENTATION-ORDER.md
owner: repository owner / decision owners to be assigned
---

# Open questions mirror

The root [`../OPEN-QUESTIONS.md`](../OPEN-QUESTIONS.md) is authoritative. OQ-01–OQ-11 remain
`OPEN`; OQ-12–OQ-15 were decided by the owner on 2026-08-16. This mirror exists so task handoffs
expose blockers without changing the source. No deadline has been assigned in the source, so the
deadline column deliberately says `not set`.

| ID | Status | Decision area | Blocking work | Owner | Deadline |
|---|---|---|---|---|---|
| OQ-01 | OPEN | Product identity provider | T110, T120, T210 | project owner | not set |
| OQ-02 | OPEN | Timeline read projections | projection-specific schema only; canonical TimelineVersion is not blocked | project owner | not set |
| OQ-03 | OPEN | Redis work-queue primitive | T310–T312 | project owner | not set |
| OQ-04 | OPEN | Primary browser progress transport | T340 transport choice; durable event API is not blocked | project owner | not set |
| OQ-05 | OPEN | Provider credentials and secret backend | T112, T217, T500 and production provider configuration | project owner | not set |
| OQ-06 | OPEN | Workspace scope in MVP | T120, T200–T210 and legacy ownership mapping | project owner | not set |
| OQ-07 | OPEN | Persistent embedding storage | persistent index portion of T523 | project owner | not set |
| OQ-08 | OPEN | Timeline edit transport | T421/T530 and editor mutation client | project owner | not set |
| OQ-09 | OPEN | Legacy Task ownership mapping | T360/T361 and legacy importer | project owner | not set |
| OQ-10 | OPEN | Retention/privacy defaults | T224/T610 and production policy; immutable Artifact model is not blocked | project owner | not set |
| OQ-11 | OPEN | Pipeline authoring scope | pipeline mutation/admin API; built-in graph needs owner confirmation | project owner | not set |
| OQ-12 | DECIDED | V2 package/product namespace = `nh_media` | resolved for T100; legacy remains `movie_narrator` | repository owner | 2026-08-16 |
| OQ-13 | DECIDED | Product/core 3.13; frozen legacy/ML 3.12 with separate locks/images | resolved for T002; Phase 0 parity evidence required | repository owner | 2026-08-16 |
| OQ-14 | DECIDED | Preserve all verified V1 CLI/REST surfaces, including batch/schedule/DLQ/distributed | resolved for T003/T360–T362; deprecation needs evidence/window/approval | repository owner | 2026-08-16 |
| OQ-15 | DECIDED | Tauri 2 lightweight remote-first desktop client | resolved for T433–T434; signing/update/least-privilege proof remains required | repository owner | 2026-08-16 |

## Update protocol

When an owner decides an OQ, update the root question, all affected specs, implementation order and
this mirror in the same reviewed change. Record the decision in [`DECISIONS.md`](DECISIONS.md),
remove the `OPEN` status only with date/owner/evidence, and do not silently fill a default in code.
