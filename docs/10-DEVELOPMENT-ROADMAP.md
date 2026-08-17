# 10 — Development Roadmap

## 1. Delivery rules

- NH-Media starts from an empty/new independent implementation.
- A phase is a gate, not one PR; task detail is in `IMPLEMENTATION-ORDER.md`.
- No phase starts before prerequisites and acceptance evidence pass.
- Reference research informs specs/tests only; upstream is absent from runtime/build/deployment.
- No application code is authorized by this documentation refactor itself.

## 2. Phase 0 — Specification ratification

**Scope:** normalize product identity, architecture, upstream policy, capability coverage, contracts,
task order and open decisions.

**Deliverables:** docs 00–14; reference policy/capability/module audits; domain/DB/API/pipeline/
Timeline/event contracts; implementation order; Local/LAN plan; final audit.

**Acceptance:** independence assertions pass; every capability has disposition; no upstream runtime,
adapter, compatibility service, image or import target; links/terms consistent; no application diff.

## 3. Phase 1 — Repository and toolchain foundation

**Scope:** bootstrap NH-Media packages and reproducible environments without product features.

**Deliverables:** `apps/web`, `apps/desktop`, Go API/media-worker boundaries, Python
`services/ml-worker/nh_media`, contracts/SDK, pinned toolchains, private PostgreSQL/Redis/object
storage dev profile, API health/error shell and CI.

**Acceptance:** dependency boundaries pass; no upstream source/dependency; Go API imports no Python/
FFmpeg; Tauri needs no server compute; clean setup reproducible.

## 4. Phase 2 — Domain, persistence, storage and Job foundation

**Scope:** implement NH-Media-native aggregates, migrations, uploads, events and worker protocol.

**Deliverables:** Workspace/Project/Asset/Artifact/Workflow/Pipeline/Job/Script/Timeline/Render tables;
StoragePort; multipart upload/probe/quarantine; outbox/Redis QueuePort; leases/checkpoints/retry/DLQ;
versioned worker contracts and fake-node conformance.

**Acceptance:** state vocabulary agrees across DB/API/events; bytes bypass API; workers are bounded;
crash/retry/idempotency tests pass; no paths/secrets cross boundaries.

## 5. Phase 3 — First independent vertical slice

**Scope:** prove NH-Media itself works before implementing the full video workflow.

```text
web or desktop request
→ Go Product API
→ persistent Job/JobStep/outbox
→ Redis delivery + worker lease
→ Go media worker independently probes source and generates thumbnail/proxy
→ Artifact stage/verify/commit
→ durable completion event
→ client displays/downloads result
```

**Acceptance:** works on Local/LAN, survives client disconnect and worker restart, uses no upstream
runtime/build/import/data directory, and returns safe native Product API resources.

## 6. Phase 4 — Native pipeline and studio foundations

**Scope:** establish Pipeline DAG, review/versioning and Timeline-driven render boundary.

**Deliverables:** graph validator/runtime; built-in movie-recap graph; Script/Scene/Analysis/
Narration/Timeline APIs; durable reviews; `build_timeline`; web and desktop project/review slices.

**Acceptance:** automatic/studio modes use declared policies; user versions survive rerun; renderer
input is exact TimelineVersion; candidate-group domain is representable even if evaluation is deferred.

## 7. Phase 5 — Core media/AI capability implementation

**Scope:** independently implement one capability at a time.

**Deliverables:** provider ports; research/script; TTS/Narration; ASR/alignment; scene detection;
scene analysis; subtitle/translation; match proposals; Timeline compiler; audio mix; render/QA/export.

**Method per capability:** research → behavior spec → NH-Media interface → implementation → unit/
integration/real-media tests → optional reference comparison → acceptance.

**Acceptance:** each node has provenance, timeout/retry/checkpoint/idempotency policy; providers stay
behind adapters; a complete recap renders from NH-Media-owned code only.

## 8. Phase 6 — Intelligence and multi-output improvements

**Scope:** add quality/intelligence beyond the initial reference feature set.

**Deliverables:** GenerationCandidate/EvaluationResult/SelectionPolicy; multimodal embeddings;
Character/Appearance; coverage feedback; ReferenceStyleAnalysis; auto-reframe; 16:9/9:16/1:1 reuse;
advanced subtitle/audio quality.

**Acceptance:** candidate selection is auditable; reference style contains abstract metrics, not
copied footage; profile-only renders do not rerun unrelated AI; quality benchmarks pass.

## 9. Phase 7 — Local/LAN release and operations

**Scope:** production-like single-server/LAN validation without requiring a VPS.

**Deliverables:** reproducible LAN server profile; private data services; auth; multi-client flows;
worker drain/recovery; backups/restores; Artifact inventory; observability; signed staging desktop.

**Acceptance:** remote web/Tauri clients, persistence, restart, recovery, security and restore drills
pass on LAN; upstream absence is proven in images/packages/dependency graphs.

## 10. Phase 8 — Internet production and later capabilities

**Scope:** public ingress/deployment after functional architecture is proven.

**Deliverables:** public TLS/DNS/CDN as needed, canary, production SLO/alerts, release rollback;
later scheduling, batch, distributed workers, semantic search, OCR/tracking/inpainting/removal where
approved.

**Acceptance:** public threat controls, canary and restore/rollback pass; every production-affecting
decision is approved. Kubernetes remains optional and evidence-driven.

## 11. Phase evidence

```text
Phase/date/owner
Prerequisites
Deliverables/versions
Commands/tests/reports
Acceptance evidence
Independence scan
Security/data review
Known limitations/open decisions
Decision: pass | fail
```
