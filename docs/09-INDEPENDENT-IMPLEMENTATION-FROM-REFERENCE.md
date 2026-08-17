# 09 — Independent Implementation from Reference

## 1. Strategy

NH-Media converts external research into independently implemented product capabilities. It does
not migrate an upstream codebase and does not preserve an upstream runtime.

```text
reference inventory → behavior specification → NH-Media domain/interface
→ independent implementation → NH-Media tests
→ optional behavior comparison → NH-Media acceptance
```

Behavioral comparison is not source/API/CLI compatibility.

## 2. Research phases

### R0 — Capability inventory

Inspect recorded upstream evidence and list every meaningful capability, including hidden or
later-phase features. Update `UPSTREAM-CAPABILITY-MATRIX.md` without changing product architecture.

### R1 — Behavioral specification extraction

Describe inputs, outputs, sequence, failure modes, quality expectations and known limitations. Mark
facts versus proposals. Preserve useful fixtures/observations with provenance when appropriate.

### R2 — NH-Media design

Define NH-Media-native entities, contracts, ownership, service/module, security policy and
acceptance criteria. Do not expose upstream module names or path-based DTOs.

### R3 — Independent minimum implementation

Write new code under NH-Media namespaces and boundaries. The implementation must run without an
upstream checkout, import, container, service or data directory.

### R4 — Behavioral/quality evaluation

Run NH-Media unit, contract, integration and real-media tests. Optional upstream comparisons use
recorded outputs or a separate research environment, never the production graph.

### R5 — NH-Media improvements

Improve quality, editability, security, observability and scalability while maintaining NH-Media
contracts. Document intentional divergence from reference observations.

### R6 — Reference independence check

Prove normal development, CI, build, deployment and operations require no upstream repository.
Reference material may remain as documentation/provenance only.

## 3. Classification

| Classification | Meaning |
|---|---|
| `REIMPLEMENT` | Implement the product capability independently. |
| `ADOPT-CONCEPT` | Use an architectural/algorithmic concept, not source code. |
| `IMPROVE` | Build an NH-Media-native version with intentional improvements. |
| `REFERENCE` | Retain information/fixtures only for research. |
| `DEFER` | Valid capability scheduled later with rationale. |
| `IGNORE` | Deliberately outside scope with explicit reason. |

## 4. High-risk lost capabilities

### Candidate race

Represent multiple generated alternatives generically:

```text
GenerationCandidate → EvaluationResult → CandidateSelectionPolicy → selected output
```

This is `IMPROVE` and may be later-phase, but Pipeline/domain/storage schemas cannot hard-code one
candidate forever.

### Reference-video style analysis

`ReferenceStyleAnalysis` extracts abstract production characteristics such as pacing, average shot
duration, narration density, subtitle style, framing, transition frequency, music intensity,
structural rhythm and scene categories. It does not copy source footage. This capability is
`IMPROVE` and scheduled after core media analysis.

### Scheduling

Scheduled Job submission is `DEFER`. It must reuse Product authorization, idempotency and durable
Job commands rather than reproduce an upstream local scheduler.

### Distributed execution

Capability-based leases, Artifact refs and stateless workers make future distribution possible.
Multi-host deployment is `ADOPT-CONCEPT`; a complex scheduler is deferred until load evidence.

### Extensions

Use built-in/provider adapter registries and reviewed, allowlisted isolation. Unrestricted Python
entry-point loading is `IGNORE` for product deployments because it violates the trust boundary.

## 5. Golden/reference tests

Golden artifacts describe a desired NH-Media behavior or quality threshold. They do not freeze
undocumented upstream quirks. Every test records fixture provenance, deterministic fields,
tolerances and intentional divergence. Normal test commands must not execute upstream.

## 6. Upstream refresh procedure

1. Record old/new URL, tag/commit and license metadata.
2. Compare affected capabilities, contracts, dependencies and security behavior.
3. Update research observations and capability classifications.
4. Decide independently whether NH-Media should reimplement, improve, defer or ignore the change.
5. Add/update NH-Media acceptance tests before implementation.
6. Confirm no wholesale merge, source copy or runtime dependency was introduced.

## 7. Acceptance

- Every meaningful upstream capability has a disposition, rationale, target and acceptance criteria.
- Important research knowledge is retained without keeping upstream runtime requirements.
- No `LegacyMovieNarratorAdapter`, V1 engine, compatibility gateway or upstream worker/image exists
  in target architecture.
- NH-Media-native API/domain terms replace upstream CLI/REST/module terms.
- Product build/test/deployment and first vertical slice work without upstream.
