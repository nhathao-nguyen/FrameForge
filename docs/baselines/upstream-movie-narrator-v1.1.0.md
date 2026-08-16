---
baseline: upstream-movie-narrator-v1.1.0
task: T001
status: immutable-snapshot-recorded
recorded: 2026-08-16
source: references/movie-narrator
owner: repository owner / task assignee
---

# Frozen upstream baseline: Movie Narrator V1

This record freezes the upstream legacy/reference input for Phase 0. It is an
evidence record, not an instruction to rewrite, rename or merge the upstream
repository into the V2 architecture.

## Source identity

| Field | Value |
|---|---|
| Repository | `https://github.com/zcbacxc/movie-narrator.git` |
| Local reference | `references/movie-narrator` |
| Branch inspected | `main` |
| Local HEAD | `bc2d276cf477fe3ce1a16f9679dcb1d2978e3a74` |
| Remote `origin/main` | `bc2d276cf477fe3ce1a16f9679dcb1d2978e3a74` |
| HEAD tree | `8c3fa095a836fee2ca02231bd9a8dfc17fd22b98` |
| HEAD commit date | `2026-08-11T20:40:49+08:00` |
| HEAD subject | `v1.1.0 community and polish (code changes) (#156)` |
| Tracked file count | `333` |
| Tracked-file-list SHA-256 | `da87e626eadacbd72eac6edfe44b2bad57de1ea4e147f66b2bac292e6ef78e47` |
| Worktree/index status | clean at inspection time |

The tracked-file-list hash is calculated from the newline-delimited output of
`git ls-files`. The commit and tree hashes remain the authoritative content
identity; the file-list hash is an additional inventory check.

## Tag and release evidence

The local annotated tag objects and peeled commits were inspected as follows:

| Ref | Tag object | Peeled commit |
|---|---|---|
| `v1.0.0` | `27e866a7a0743a441d031e5a0dec319719bd3804` | `121b8833d494f0a42a094371db9ff5e98a01d642` |
| `v1.1.0` | `c6f30d85e2abd1bcb60c16fda9e660b26b336ede` | `bc2d276cf477fe3ce1a16f9679dcb1d2978e3a74` |

The remote advertised the same release refs during this inspection. The
frozen compatibility target for this project is the peeled `v1.1.0` commit
shown above, unless a later owner-approved baseline explicitly supersedes it.

## Package, runtime and license evidence

At the frozen commit:

- Python package name: `movie-narrator`.
- Package version: `1.1.0`.
- Public Python package namespace and CLI entry point: `movie_narrator` and
  `mn = movie_narrator.cli:app`.
- License metadata: `AGPL-3.0-or-later`.
- `LICENSE` is the GNU Affero General Public License, version 3, and must be
  retained with attribution in any compatible distribution or derivative
  handling covered by the license.
- The legacy container declares `PYTHON_VERSION=3.12`; this is the frozen V1
  legacy/ML runtime recorded at the baseline commit.
- **Historical decision note:** At the time T001 was originally recorded, the
  then-current OQ-13 decision assigned Python 3.13 to Product/API/core.
- That decision was subsequently superseded on 2026-08-16 by
  `D-T000-013-SUPERSEDING`.
- **Current normative topology:** Product API/control plane is Go; Python 3.12
  is isolated to ML/AI and frozen V1 compatibility; the V1 runtime/dependency
  evidence in this baseline remains unchanged.
- The V1 repository advertises Python `>=3.10` and includes the legacy
  `movie_narrator*` package namespace. V2 must keep that namespace behind the
  legacy adapter rather than exposing it as the new product namespace.

The historical note above does not change the frozen V1 evidence, commit/tree/tag identity,
runtime evidence or rollback baseline recorded in this document.

## Verification performed

Commands run from the NH-Media repository on 2026-08-16:

```text
git -C references/movie-narrator status --short --branch
git -C references/movie-narrator show-ref --tags
git -C references/movie-narrator show -s --format='HEAD=%H%nDATE=%cI%nSUBJECT=%s' HEAD
git -C references/movie-narrator rev-parse HEAD^{tree}
git -C references/movie-narrator ls-files | wc -l
git -C references/movie-narrator ls-files | sha256sum
git -C references/movie-narrator diff --quiet
git -C references/movie-narrator diff --cached --quiet
git -C references/movie-narrator ls-remote --heads origin main
git -C references/movie-narrator ls-remote --tags origin 'v1.0.0*' 'v1.1.0*'
```

Acceptance results:

- local worktree and index were clean;
- local `main` and remote `origin/main` resolved to the frozen HEAD;
- `v1.1.0` peeled to the frozen HEAD;
- the tracked inventory contained 333 files;
- no V1 file was modified by T001;
- license, namespace, CLI entry point and Python 3.12 container evidence were
  recorded for later compatibility and rollback work.

## Compatibility and rollback boundary

This baseline preserves the verified V1 CLI/REST behavior required by OQ-14,
including batch, schedule, DLQ and distributed behavior when those surfaces
are verified in T003. T001 does not claim behavioral parity; it only freezes
the upstream input from which T003–T005 will produce the compatibility profile,
golden outputs and rollback image.

Future V2 execution must use `LegacyMovieNarratorAdapter`/`VideoEngine` as the
boundary. It must not wholesale-merge this repository, rename
`movie_narrator`, or remove its license and attribution. Any upstream update
must go through the comparison drill in T604 and an owner-approved baseline
change.

## Reproduction and change control

To reproduce this snapshot, verify the local reference checkout resolves to
the HEAD, tree and remote ref values in this document, then rerun the commands
above. A changed commit, tree, tag, remote ref, worktree or tracked inventory
invalidates this record and requires a new reviewed baseline record; the
existing record must remain available for rollback and comparison.
