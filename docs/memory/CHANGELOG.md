---
last_verified: 2026-08-16
source: user-approved execution plan; git history; docs/memory/
owner: repository owner / task assignee
---

# Memory changelog

## 2026-08-16

- Created local `develop` from `docs/architecture-spec` at `6ca438f849a645a6c910d722776dcf448064657a`.
- Preserved `main` and `docs/architecture-spec`; recorded that they have no merge-base.
- Added `AGENTS.md` routing for the versioned project memory.
- Added memory index, current state, architecture map, approved operating decisions, OQ mirror,
  T000–T605 status ledger, compatibility matrix, production runbook, security controls, test
  evidence and this changelog.
- Added `tools/check_memory.py` as documentation consistency tooling only.
- Extended the architecture for first-class web + desktop clients, remote VPS deployment and shared
  SDK/contracts; added OQ-15 for desktop shell/runtime choice.
- Added [`../SETUP-PLAN.md`](../SETUP-PLAN.md), [`../PRODUCTION-PLAN.md`](../PRODUCTION-PLAN.md)
  and [`../PRODUCTION-EXECUTION-PROMPT.md`](../PRODUCTION-EXECUTION-PROMPT.md), with multi-pass
  setup certification and production release gates.
- Recorded owner approval for T000: `nh_media` namespace, Python 3.13 core + temporary 3.12
  legacy/ML split, full verified V1 compatibility and Tauri 2 desktop client. T001 is now next;
  application code remains blocked until T001–T005 pass.
- Completed T001 with an immutable upstream baseline for `references/movie-narrator`: exact
  commit/tree/tag/remote/license/runtime/tracked-inventory evidence is in
  [`../baselines/upstream-movie-narrator-v1.1.0.md`](../baselines/upstream-movie-narrator-v1.1.0.md).
  The upstream worktree and index were clean and no V1 file was changed.
- Recorded the owner amendment to T000: the earlier Python Product/API 3.13 decision is retained
  as superseded history; Go now owns the V2 Product API/control plane, Go is preferred for media/
  FFmpeg workers, and Python is isolated to ML/AI plus frozen V1 compatibility. Language-neutral
  versioned contracts, bounded asynchronous workers and migration/rollback safeguards are normative.
- No application code, V1 source, migration, runtime, API, frontend or worker implementation was
  added.

## Next update

T000 is ratified and amended; T001 is already complete and was not restarted. T002 is the next
eligible task only after this amendment is accepted; T002–T005/Phase 0 evidence must pass before any
application task is marked eligible.
