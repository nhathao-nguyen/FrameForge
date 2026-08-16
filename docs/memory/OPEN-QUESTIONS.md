---
last_verified: 2026-08-16
source: ../OPEN-QUESTIONS.md; ../IMPLEMENTATION-ORDER.md
owner: repository owner / decision owners to be assigned
---

# Open questions mirror

The root [`../OPEN-QUESTIONS.md`](../OPEN-QUESTIONS.md) is authoritative. All fifteen questions
are still `OPEN`; this mirror exists so task handoffs expose blockers without changing the source.
No deadline has been assigned in the source, so the deadline column deliberately says `not set`.

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
| OQ-12 | OPEN | V2 package/product namespace | first application scaffold T100; Phase 0 baseline is not blocked | project owner | not set |
| OQ-13 | OPEN | Python version split for legacy ML/container | environment lock/image policy; no documentation audit block | project owner | not set |
| OQ-14 | OPEN | Frozen V1 compatibility breadth | Phase 0 compatibility profile and T360–T362 | project owner | not set |
| OQ-15 | OPEN | Desktop client shell/runtime | T433–T434; API/contracts and web are not blocked | project owner | not set |

## Update protocol

When an owner decides an OQ, update the root question, all affected specs, implementation order and
this mirror in the same reviewed change. Record the decision in [`DECISIONS.md`](DECISIONS.md),
remove the `OPEN` status only with date/owner/evidence, and do not silently fill a default in code.
