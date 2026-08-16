---
last_verified: 2026-08-16
source: user-approved execution plan; AGENTS.md; docs/CODEX-INSTRUCTIONS.md
owner: repository owner / T000 ratifier
---

# Approved decisions

This file records operational decisions approved for the current work. It does not ratify any
recommendation in [`../OPEN-QUESTIONS.md`](../OPEN-QUESTIONS.md). Architecture invariants remain
normative in the specification set and are linked from the project memory.

| ID | Date | Owner | Decision | Alternatives considered | Consequence |
|---|---|---|---|---|---|
| D-MEM-001 | 2026-08-16 | repository owner / task intent | Create local `develop` from `docs/architecture-spec` HEAD `6ca438f...`; do not merge `main` implicitly and do not push without explicit request. | Merge/rebase `main`; push immediately. | Branch provenance is explicit; future integration requires a reviewed operation. |
| D-MEM-002 | 2026-08-16 | repository owner / task intent | Use `AGENTS.md` plus versioned `docs/memory/` as the project-memory model. | Put the full architecture in `AGENTS.md`; use unversioned external notes. | Agents get concise routing while normative docs remain the source of truth. |
| D-MEM-003 | 2026-08-16 | repository rules / T000 ratifier | Keep application implementation blocked until the documentation gate is owner-approved. | Scaffold a temporary V2 prototype before ratification. | Only docs, audits, baselines and consistency tooling may change before T000. |
| D-MEM-004 | 2026-08-16 | repository rules / architecture ratifier | Preserve V1 behind `VideoEngine`/`LegacyMovieNarratorAdapter` and preserve TimelineVersion as renderer source of truth. | Big-bang V1 rewrite; renderer reads AI match output directly. | Compatibility and user overrides remain migration invariants. |
| D-MEM-005 | 2026-08-16 | repository owner / user requirement | Treat web and desktop as first-class Product API clients; remote-server mode is the default and the server remains source of truth. | Web-only client; desktop embedding the engine/local database. | Shared contracts/SDK and API security must serve both clients; desktop shell choice remains OQ-15. |

## Decision protocol

For a new architectural choice, update the root OQ and affected normative specs first. Once the
owner records a dated decision, add it here with alternatives and consequences, then update the
task ledger. A recommendation copied from an OQ is not sufficient evidence.
