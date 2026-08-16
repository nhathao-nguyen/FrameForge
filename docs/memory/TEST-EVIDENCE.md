---
last_verified: 2026-08-16
source: ../SPEC-AUDIT-REPORT.md; ../10-DEVELOPMENT-ROADMAP.md; ../IMPLEMENTATION-ORDER.md; git metadata
owner: task assignee / release owner
---

# Test evidence

Evidence is append-only by task where practical. A row must identify the command, environment,
commit, report/artifact location, result, limitation and acceptance criterion it proves.

| Date | Task/check | Command | Environment/commit | Report or artifact | Result | Limitation / acceptance criterion |
|---|---|---|---|---|---|---|
| 2026-08-16 | Git baseline | `git status --short --branch`; `git log -1`; branch/merge-base checks | repository; `6ca438f849a645a6c910d722776dcf448064657a` | terminal evidence; [`CURRENT-STATE.md`](CURRENT-STATE.md) | pass | proves clean source before branch creation, local `develop`, no merge-base with `main` |
| 2026-08-16 | Documentation audit/T000 | existing audit report, owner approval and consistency review | repository; baseline commit | [`../SPEC-AUDIT-REPORT.md`](../SPEC-AUDIT-REPORT.md), OQ/spec/memory diff | pass — T000 ratified | OQ-12–OQ-15 decisions recorded; application code remains blocked until T001–T005 |
| 2026-08-16 | Memory/plan consistency | `python tools/check_memory.py --max-age 30` | repository; `develop` at baseline commit | this directory; terminal output | pass | checks memory/plan links, OQ mirror (including OQ-15), task coverage/dependencies (including T433/T434), canonical-term duplicates, metadata freshness, branch/commit and sensitive data |
| 2026-08-16 | Upstream immutable baseline/T001 | `git -C references/movie-narrator status --short --branch`; `show-ref --tags`; `rev-parse HEAD^{tree}`; `ls-files`; `diff --quiet`; `ls-remote` | frozen reference checkout; HEAD `bc2d276cf477fe3ce1a16f9679dcb1d2978e3a74` | [`../baselines/upstream-movie-narrator-v1.1.0.md`](../baselines/upstream-movie-narrator-v1.1.0.md) | pass — snapshot recorded | proves source identity, tag/remote agreement, clean V1 worktree and inventory; does not yet prove behavioral parity or golden output |
| 2026-08-16 | T000 architecture amendment | documentation diff review; `rg` language-topology audit; no-application-source check; `python tools/check_memory.py --max-age 0` | repository; `develop`; amendment commit recorded in git history | affected docs, [`DECISIONS.md`](DECISIONS.md), [`CURRENT-STATE.md`](CURRENT-STATE.md) | pass — superseding owner decision recorded | confirms Go control plane, bounded worker flow, isolated Python ML/V1 and language-neutral contracts; no T001 restart and no application code |

## Not yet run

The following are deliberately not claimed: full V1 tests, sample-media/golden outputs, Go/Python/
FFmpeg matrix, rollback image, migrations, API/worker/frontend tests, malicious-media suite, backup
restore, load/failure drills or production canary. They are owned by T002–T605 in the order
specified by the roadmap and remain blocked by the documentation gate or open decisions.

## Evidence format for future tasks

```text
Task: Txxx
Status: complete/blocked
Implemented boundary: ...
Tests: command + environment + commit + report/artifact
Compatibility: old contract and mapping result
Security: controls and scan result
Migration/rollback: forward, downgrade/rollback and data safety
Open questions: IDs or none
Next task: Txxx
```
