---
last_verified: 2026-08-16
source: user-approved execution plan; AGENTS.md; docs/CODEX-INSTRUCTIONS.md
owner: repository owner / T000 ratifier
---

# Approved decisions

This file records operational decisions approved for the current work. It does not ratify any
recommendation in [`../OPEN-QUESTIONS.md`](../OPEN-QUESTIONS.md). Architecture invariants remain
normative in the specification set and are linked from the project memory.

| ID | Date | Owner | Decision | Alternatives considered | Consequence |
|---|---|---|---|---|---|
| D-MEM-001 | 2026-08-16 | repository owner / task intent | Create local `develop` from `docs/architecture-spec` HEAD `6ca438f...`; do not merge `main` implicitly and do not push without explicit request. | Merge/rebase `main`; push immediately. | Branch provenance is explicit; future integration requires a reviewed operation. |
| D-MEM-002 | 2026-08-16 | repository owner / task intent | Use `AGENTS.md` plus versioned `docs/memory/` as the project-memory model. | Put the full architecture in `AGENTS.md`; use unversioned external notes. | Agents get concise routing while normative docs remain the source of truth. |
| D-MEM-003 | 2026-08-16 | repository rules / T000 ratifier | Keep application implementation blocked until the documentation gate is owner-approved. | Scaffold a temporary V2 prototype before ratification. | Only docs, audits, baselines and consistency tooling may change before T000. |
| D-MEM-004 | 2026-08-16 | repository rules / architecture ratifier | Preserve V1 behind `VideoEngine`/`LegacyMovieNarratorAdapter` and preserve TimelineVersion as renderer source of truth. | Big-bang V1 rewrite; renderer reads AI match output directly. | Compatibility and user overrides remain migration invariants. |
| D-MEM-005 | 2026-08-16 | repository owner / user requirement | Treat web and desktop as first-class Product API clients; remote-server mode is the default and the server remains source of truth. | Web-only client; desktop embedding the engine/local database. | Shared contracts/SDK and API security must serve both clients; Tauri 2 is recorded in D-T000-015. |
| D-T000-012 | 2026-08-16 | repository owner | Use `nh_media` for new Python-side V2 compute/ML code; reject `your_engine`, `video_engine` and `frameforge` as Python public namespaces; preserve `movie_narrator`. Go uses `github.com/nhathao-nguyen/FrameForge` module conventions and internal domain packages; contracts stay language-neutral. | Placeholder/generic/product-name namespaces. | T100 and all new Python imports use `nh_media`; legacy adapter keeps its upstream namespace; Go does not expose a global public `frameforge` package. |
| D-T000-013 | 2026-08-16 | repository owner | **SUPERSEDED by D-T000-013-SUPERSEDING.** Historical T000 decision: Product/API/core use Python 3.13; frozen legacy/ML image uses Python 3.12 temporarily with separate locks/images. | 3.13 everywhere; 3.12 everywhere. | Retained for audit history only; it is not normative for the V2 Product API. |
| D-T000-013-SUPERSEDING | 2026-08-16 | repository owner | V2 Product API/control plane is Go and must not depend on Python. Prefer Go for media/FFmpeg workers; isolate Python 3.12 initially for ML/AI and frozen V1 compatibility. Go↔Python uses versioned language-neutral contracts; implementation language is replaceable; Product API work is bounded asynchronous via durable Job/outbox/worker/result flow. | Python Product API; big-bang Go rewrite; framework-specific cross-language serialization. | T002 pins supported Go + isolated Python/FFmpeg environments; T100/T106 use Go control-plane boundaries; parity, contract and rollback evidence are mandatory. |
| D-T000-014 | 2026-08-16 | repository owner | Preserve all verified public V1 CLI/REST behavior in the frozen baseline, including batch, schedule, DLQ and distributed surfaces. | Preserve only core task routes; remove surfaces for convenience. | T003 profile and T360–T362 compatibility must cover all verified behavior; later removal needs evidence/window/approval. |
| D-T000-015 | 2026-08-16 | repository owner | Use Tauri 2 for the lightweight remote-first desktop Product API client; no bundled server/media/AI infrastructure or provider secrets. | Electron; native per OS; desktop embedding engine. | T433/T434 implement least-privilege capabilities and prove signing/update/rollback before release. |

## Decision protocol

For a new architectural choice, update the root OQ and affected normative specs first. Once the
owner records a dated decision, add it here with alternatives and consequences, then update the
task ledger. A recommendation copied from an OQ is not sufficient evidence.
