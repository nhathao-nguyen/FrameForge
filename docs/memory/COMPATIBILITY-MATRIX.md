---
last_verified: 2026-08-17
source: ../UPSTREAM-REFERENCE-POLICY.md; ../UPSTREAM-CAPABILITY-MATRIX.md; ../09-INDEPENDENT-IMPLEMENTATION-FROM-REFERENCE.md
owner: architecture owner / task assignee
---

# Upstream compatibility boundary

The filename is retained so existing memory tooling and links remain stable. Its current assertion
is that **NH-Media has no Movie Narrator compatibility contract**.

| Surface | NH-Media obligation | Evidence |
|---|---|---|
| Python import/package | none | `nh_media` is independent; `movie_narrator` is provenance text only |
| CLI and REST routes | none | native `/api/v1` Product resources |
| Task/status/event vocabulary | none | canonical Job/JobStep terms in Glossary |
| JSON task/checkpoint/config formats | none | native durable schema and versioned contracts |
| Runtime/container/image | none | pinned NH-Media Go/Python/FFmpeg environments |
| Build/test/deployment checkout | none | independence gate must pass with upstream absent |
| Rollback target | none | rollback uses the last compatible NH-Media release |
| Capability research | allowed under policy | dedicated capability matrix and module audit |

Classification is limited to `REIMPLEMENT`, `ADOPT-CONCEPT`, `IMPROVE`, `REFERENCE`, `DEFER` and
`IGNORE`. The detailed, normative disposition lives in
[`../UPSTREAM-CAPABILITY-MATRIX.md`](../UPSTREAM-CAPABILITY-MATRIX.md).
