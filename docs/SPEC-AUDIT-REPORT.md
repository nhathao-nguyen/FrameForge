# Specification Audit

Audit date: 2026-08-17 (Asia/Saigon).

## Overall status

**SPEC READY FOR IMPLEMENTATION**

This means the independent architecture is specified consistently and decomposed into reviewable
tasks. It does not authorize application code by itself: T004 must record owner approval of the
documentation/independence gate, and every later task must satisfy its dependencies and OPEN
question blocks.

## Critical findings resolved

1. Removed the former premise that NH-Media wraps, migrates, preserves or can roll back to a Movie
   Narrator runtime.
2. Removed `LegacyMovieNarratorAdapter`, legacy compatibility workers, upstream public route/state
   mappings and compatibility-removal gates from the target architecture.
3. Corrected product identity to NH-Media, Python namespace to `nh_media`, and Go module path to
   `github.com/nhathao-nguyen/NH-Media`.
4. Established one target topology: Go Product API, bounded Go media workers, isolated Python ML/AI
   workers, Next.js/React web and a Tauri 2 thin remote client.
5. Replaced migration planning with independent implementation phases and an explicit research
   policy/capability matrix.
6. Added native Candidate/Evaluation/Selection and ReferenceStyleAnalysis domain, persistence,
   API, pipeline and task coverage.
7. Made Local/LAN certification a valid release milestone before VPS/public production.

## Cross-document result

| Area | Result |
|---|---|
| Product/context/architecture | Independent NH-Media identity and topology are consistent. |
| Domain/database/API | Native analysis, candidate and reference-style contracts align. |
| Pipeline/timeline/events/jobs | Canonical states, proposal ownership and renderer input align. |
| Security/storage/workers/providers | Ports, trust boundaries, least privilege and Artifact refs align. |
| Roadmap/implementation order | First native vertical slice precedes advanced feature work. |
| Setup/production | Local/LAN proof precedes internet production. |
| Upstream research | Policy, provenance, module audit and capability classification are isolated. |
| Memory/evidence | Mirrors the normative docs and records no application implementation. |

## Upstream independence assertions

```text
Independent implementation: YES
Movie Narrator runtime dependency: NO
Movie Narrator build dependency: NO
Movie Narrator deployment dependency: NO
Movie Narrator import dependency: NO
Legacy compatibility service: NO
```

- Upstream repository/package/source is not required to build, test, run or deploy NH-Media.
- `movie_narrator` is not an NH-Media runtime namespace.
- No upstream CLI, REST route, status, JSON task store, checkpoint, plugin or configuration format is
  a compatibility contract.
- Upstream observations can inform independently authored acceptance criteria only through the
  reference policy and capability matrix.
- A research refresh cannot silently change target architecture or implementation order.

## Knowledge retention, naming and LAN readiness

- The capability matrix retains all meaningful observations from the prior module audit, including
  media research, speech/subtitles, scene/VLM/matching, composition/rendering, reliability,
  scheduling, distributed work, candidate race and reference-style concepts.
- Deferred/ignored items have an explicit reason and acceptance boundary.
- Product: `NH-Media`; Python namespace: `nh_media`; upstream reference: `Movie Narrator`.
- Next.js/React and Tauri call the Go Product API; Go media and Python ML workers remain isolated;
  PostgreSQL/Redis/object storage are server-side.
- Local/LAN production-like validation remains fully supported at T603. Public DNS/TLS/VPS/canary
  work is later T605 scope and does not block functional implementation.

## Security findings

The specifications require direct staged uploads, quarantine, strict path containment, bounded
argv-list subprocesses, process-tree termination, resource/time/disk/PID limits, deny-by-default
egress, scoped secret references, safe public errors, redacted telemetry and audited dependency/
license handling. Unrestricted Python plugin discovery is explicitly excluded.

## Remaining decisions

Owner decisions still OPEN:

- OQ-01: identity provider;
- OQ-05: provider credential ownership/secret backend;
- OQ-06: Workspace scope;
- OQ-10: retention/privacy defaults.

Implementation decisions OQ-02, OQ-03, OQ-04 and OQ-08 block only their listed tasks. OQ-07 and
OQ-11 are deferred. OQ-09 and OQ-12–OQ-15 are resolved by the independent architecture.

## Implementation blockers

- T004 documentation/independence certification and owner approval are required before T100.
- Exact toolchain versions and clean-room fixture policy are evidence tasks T002/T003.
- Later application tasks remain bounded by their explicit task and OQ dependencies.
- No setup, runtime, application, release or deployment evidence is claimed by this audit.

## Audit conclusion

The specification set is internally ready for the implementation gate. It is not a release claim
and does not treat Movie Narrator as a fallback, migration source or operational dependency.

SPEC READY FOR IMPLEMENTATION
