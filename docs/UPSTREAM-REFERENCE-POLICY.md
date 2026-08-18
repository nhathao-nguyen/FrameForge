# Upstream Reference Policy

## Purpose

Movie Narrator is an external research/reference repository:

- URL: `https://github.com/zcbacxc/movie-narrator.git`
- Research snapshot recorded by the existing audit: commit
  `b332cf9411a09cd8452c7dd84bfffb380be9c621` (refreshed 2026-08-18; prior audit snapshot is
  retained in the historical evidence)
- Recorded upstream license identifier: `AGPL-3.0-or-later`

These fields are engineering provenance. This document does not make a legal conclusion or replace
license review.

## Boundary

The upstream repository is `development/reference-only`. It is not owned by NH-Media and is not a
runtime, build, test, deployment, packaging or rollback dependency.

NH-Media must not:

- import or execute `movie_narrator` in product code;
- call its CLI or REST API;
- ship a vendored copy, production submodule or upstream execution container;
- create a compatibility service or preserve upstream public APIs as NH-Media contracts;
- instruct developers/agents to copy modules blindly;
- include upstream source in web/desktop/server/worker artifacts;
- require an upstream checkout for normal CI or development.

## Allowed research use

Developers may inspect upstream source and recorded outputs to understand:

- capabilities and workflow sequencing;
- algorithms and processing concepts;
- interface and provider ideas;
- failure modes, security concerns and operational behavior;
- test cases and observable acceptance criteria.

Research produces independently written NH-Media specifications, interfaces, tests and code. If
source is temporarily fetched, use a separate disposable research workspace, record URL/commit and
license, and keep it outside production dependency graphs.

## Capability classification

Every meaningful observed capability receives one classification in
[`UPSTREAM-CAPABILITY-MATRIX.md`](UPSTREAM-CAPABILITY-MATRIX.md):

`REIMPLEMENT`, `ADOPT-CONCEPT`, `IMPROVE`, `REFERENCE`, `DEFER` or `IGNORE`.

`IGNORE` requires a reason. `DEFER` requires a target phase or decision trigger. Classification is
about product capability, never permission to copy source.

## Reference behavior tests

Recorded fixtures/outputs may be used when lawful and useful. The repository policy and manifest
contract live in [`tests/reference-behavior/README.md`](../tests/reference-behavior/README.md), with
the current manifest intentionally empty. Normal NH-Media tests run NH-Media only. Optional
comparison:

```text
same input fixture
  ├── recorded upstream observation
  └── NH-Media implementation output
          → contract and quality comparison
```

Upstream quirks are not permanent contracts. An NH-Media result may intentionally diverge if its
documented contract, deterministic requirements and quality thresholds pass. Record the reason and
acceptance evidence.

## Provenance checklist

For each research refresh record:

- repository URL and exact commit/tag;
- inspection date and source files/modules;
- recorded license identifier/notices;
- capability or behavior learned;
- NH-Media classification and independently authored target;
- fixtures/observations retained and their storage/license review;
- confirmation that no upstream source entered product artifacts.

## Release gate

Release validation must prove:

```text
Movie Narrator runtime dependency: NO
Movie Narrator build dependency: NO
Movie Narrator deployment dependency: NO
Movie Narrator import dependency: NO
Movie Narrator source in packages/images: NO
```
