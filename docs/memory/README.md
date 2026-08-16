---
last_verified: 2026-08-16
source: AGENTS.md; docs/CODEX-INSTRUCTIONS.md; docs/10-DEVELOPMENT-ROADMAP.md
owner: repository owner / T000 ratifier
---

# Project memory

This directory is the repository's short operational memory. It records the current state,
approved operating decisions, task handoffs, compatibility obligations, production controls and
test evidence. The detailed architecture remains in `docs/00`–`docs/14`, the master context and
the topic specifications.

## Reading order

1. [`PROJECT-MEMORY.md`](PROJECT-MEMORY.md) — scope, boundaries and invariants.
2. [`CURRENT-STATE.md`](CURRENT-STATE.md) — branch, gate and active handoff.
3. [`DECISIONS.md`](DECISIONS.md) — approved decisions only.
4. [`IMPLEMENTATION-STATUS.md`](IMPLEMENTATION-STATUS.md) — T000–T605 ledger.
5. [`../SETUP-PLAN.md`](../SETUP-PLAN.md) — setup gate and V0–V10 verification.
6. [`../PRODUCTION-PLAN.md`](../PRODUCTION-PLAN.md) — full phase/release plan.
7. The task-specific normative specification and [`../OPEN-QUESTIONS.md`](../OPEN-QUESTIONS.md).

## Operating rules

- Memory is versioned with the repository and updated in the same PR as the related change.
- A memory entry has `last_verified`, `source` and `owner` metadata.
- `OPEN` recommendations are not decisions. A decision must update the OQ, affected specs and
  implementation task before dependent work starts.
- Memory never contains secrets, tokens, presigned URLs, user data or durable local filesystem
  paths.
- Run the documentation-only consistency check with:

  `python tools/check_memory.py`

- If the checker reports stale state, refresh the relevant evidence rather than suppressing the
  check.

## Files

| File | Purpose |
|---|---|
| [`PROJECT-MEMORY.md`](PROJECT-MEMORY.md) | Product intent, boundaries, invariants and task intake. |
| [`CURRENT-STATE.md`](CURRENT-STATE.md) | Branch/commit baseline, gate, active work and blockers. |
| [`ARCHITECTURE-MAP.md`](ARCHITECTURE-MAP.md) | Runtime flow, ownership, trust and dependency map. |
| [`DECISIONS.md`](DECISIONS.md) | Approved memory/operating decisions; never OQ recommendations. |
| [`OPEN-QUESTIONS.md`](OPEN-QUESTIONS.md) | Synchronized OQ-01–OQ-15 status and blocking tasks. |
| [`IMPLEMENTATION-STATUS.md`](IMPLEMENTATION-STATUS.md) | Task ledger and handoff fields for T000–T605. |
| [`COMPATIBILITY-MATRIX.md`](COMPATIBILITY-MATRIX.md) | V1 surface, adapter mapping and removal criteria. |
| [`PRODUCTION-RUNBOOK.md`](PRODUCTION-RUNBOOK.md) | Deployment, recovery, rollback and release gates. |
| [`SECURITY-CONTROLS.md`](SECURITY-CONTROLS.md) | Security control inventory and evidence status. |
| [`TEST-EVIDENCE.md`](TEST-EVIDENCE.md) | Commands, environments, results and limitations. |
| [`CHANGELOG.md`](CHANGELOG.md) | Memory-only change history and handoff notes. |
| [`../SETUP-PLAN.md`](../SETUP-PLAN.md) | Client/server foundation setup and certification passes. |
| [`../PRODUCTION-PLAN.md`](../PRODUCTION-PLAN.md) | End-to-end implementation and production release gates. |
| [`../PRODUCTION-EXECUTION-PROMPT.md`](../PRODUCTION-EXECUTION-PROMPT.md) | Copy/paste operating prompt for future execution agents. |
