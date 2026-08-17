---
baseline: upstream-movie-narrator-v1.1.0
task: T001
status: research-provenance-recorded
recorded: 2026-08-16
source: external research checkout present at inspection time
owner: repository owner / task assignee
---

# Movie Narrator v1.1.0 research provenance

This is an immutable observation record. It does not freeze a compatibility target, create a
rollback path or authorize source reuse. The local checkout used during the earlier inspection is
not present in the current NH-Media repository and is not required.

## Source identity observed

| Field | Recorded value |
|---|---|
| Repository | `https://github.com/zcbacxc/movie-narrator.git` |
| Branch inspected | `main` |
| Commit | `bc2d276cf477fe3ce1a16f9679dcb1d2978e3a74` |
| Tree | `8c3fa095a836fee2ca02231bd9a8dfc17fd22b98` |
| Commit date | `2026-08-11T20:40:49+08:00` |
| Subject | `v1.1.0 community and polish (code changes) (#156)` |
| Tracked file count | `333` |
| Tracked-file-list SHA-256 | `da87e626eadacbd72eac6edfe44b2bad57de1ea4e147f66b2bac292e6ef78e47` |

Recorded tag evidence:

| Ref | Tag object | Peeled commit |
|---|---|---|
| `v1.0.0` | `27e866a7a0743a441d031e5a0dec319719bd3804` | `121b8833d494f0a42a094371db9ff5e98a01d642` |
| `v1.1.0` | `c6f30d85e2abd1bcb60c16fda9e660b26b336ede` | `bc2d276cf477fe3ce1a16f9679dcb1d2978e3a74` |

## Package and license observations

- Distribution/package name recorded as `movie-narrator`, version `1.1.0`.
- Python import namespace recorded as `movie_narrator` and CLI entry point as `mn`.
- License metadata recorded as `AGPL-3.0-or-later`; the checkout contained the GNU Affero General
  Public License version 3.
- The container recorded Python 3.12; project metadata advertised Python 3.10–3.13 support.

These are facts about the inspected source. They are not NH-Media product/runtime choices and are
not a legal conclusion. Any future source reuse requires a separate legal and owner review; the
current architecture does not require or plan it.

## Inspection commands recorded

```text
git status --short --branch
git show-ref --tags
git show -s --format=...
git rev-parse HEAD^{tree}
git ls-files
git diff --quiet
git diff --cached --quiet
git ls-remote --heads origin main
git ls-remote --tags origin ...
```

The earlier evidence stated that the checkout was clean, `main`/`origin/main` and the `v1.1.0`
peeled commit agreed, and 333 tracked files were inventoried. This turn did not re-verify the remote
or absent checkout; the record intentionally preserves the dated observation.

## NH-Media boundary

- No upstream source, package, image, process or namespace enters normal NH-Media build/test/runtime.
- No upstream CLI/REST/status/config/checkpoint/task database becomes a public compatibility surface.
- Capability observations are routed through
  [`../UPSTREAM-REFERENCE-POLICY.md`](../UPSTREAM-REFERENCE-POLICY.md) and
  [`../UPSTREAM-CAPABILITY-MATRIX.md`](../UPSTREAM-CAPABILITY-MATRIX.md).
- A later research refresh creates a new dated record; it does not mutate NH-Media contracts or
  provide an operational rollback target.
