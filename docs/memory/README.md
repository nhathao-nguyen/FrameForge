---
last_verified: 2026-08-17
source: AGENTS.md; docs/CODEX-INSTRUCTIONS.md; docs/10-DEVELOPMENT-ROADMAP.md
owner: repository owner / task assignee
---

# Project memory

This directory is a concise operational index for the independent NH-Media project. Normative
architecture remains in the master context and `docs/00`–`docs/14`; memory cannot override it.

## Reading order

1. [`PROJECT-MEMORY.md`](PROJECT-MEMORY.md)
2. [`CURRENT-STATE.md`](CURRENT-STATE.md)
3. [`DECISIONS.md`](DECISIONS.md)
4. [`IMPLEMENTATION-STATUS.md`](IMPLEMENTATION-STATUS.md)
5. [`../OPEN-QUESTIONS.md`](../OPEN-QUESTIONS.md)
6. The task-specific normative specification.

Run `python tools/check_memory.py --max-age 0` after changing specifications or memory.

## Operating rules

- Update related memory/evidence in the same change.
- Never store secrets, presigned URLs, user data or durable local paths here.
- An OPEN recommendation is not an approved choice.
- Movie Narrator facts are research provenance only; memory must not turn them into an operational
  dependency, compatibility contract or rollback target.
- Record commands as evidence only after they run.

## Files

| File | Purpose |
|---|---|
| [`PROJECT-MEMORY.md`](PROJECT-MEMORY.md) | Product intent and invariants. |
| [`CURRENT-STATE.md`](CURRENT-STATE.md) | Branch, gate, active task and blockers. |
| [`ARCHITECTURE-MAP.md`](ARCHITECTURE-MAP.md) | Ownership, flow and trust map. |
| [`DECISIONS.md`](DECISIONS.md) | Approved decisions. |
| [`OPEN-QUESTIONS.md`](OPEN-QUESTIONS.md) | Synchronized OQ status. |
| [`IMPLEMENTATION-STATUS.md`](IMPLEMENTATION-STATUS.md) | T000–T605 ledger. |
| [`COMPATIBILITY-MATRIX.md`](COMPATIBILITY-MATRIX.md) | Historical filename; records absence of an upstream compatibility surface. |
| [`PRODUCTION-RUNBOOK.md`](PRODUCTION-RUNBOOK.md) | Planned Local/LAN and later production operations. |
| [`SECURITY-CONTROLS.md`](SECURITY-CONTROLS.md) | Control inventory. |
| [`TEST-EVIDENCE.md`](TEST-EVIDENCE.md) | Commands and limitations. |
| [`CHANGELOG.md`](CHANGELOG.md) | Memory change history. |
