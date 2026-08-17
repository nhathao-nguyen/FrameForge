# Repository Agent Rules

Read before changing this repository:

1. `PROJECT_REBUILD_PLAN.md` points to the canonical architecture context.
2. `docs/00-PROJECT-CONTEXT.md` and `docs/GLOSSARY.md` define scope and terminology.
3. Use the topic specification in `docs/01`–`docs/14`, `docs/UPSTREAM-REFERENCE-POLICY.md`,
   `docs/UPSTREAM-CAPABILITY-MATRIX.md` and `docs/OPEN-QUESTIONS.md`.
4. Follow `docs/IMPLEMENTATION-ORDER.md`; an OPEN recommendation is not an approved decision.
5. Movie Narrator is research/reference-only. It is not an NH-Media runtime, build, deployment,
   compatibility or rollback dependency.
6. Before starting work, read `docs/memory/PROJECT-MEMORY.md`, `docs/memory/CURRENT-STATE.md`,
   `docs/memory/DECISIONS.md` and `docs/memory/IMPLEMENTATION-STATUS.md`.
7. For setup or release work, also read `docs/SETUP-PLAN.md` and `docs/PRODUCTION-PLAN.md`.
8. After each task, update the relevant memory/evidence entry in the same change.

Project-wide invariants:

- Do not implement application code until the documentation gate is explicitly approved.
- NH-Media is independently designed and independently implemented. Do not fork, wrap, embed,
  import, execute, vendor or incrementally migrate the Movie Narrator runtime.
- Product backend owns identity/authorization/Project/Job/database; compute workers own media/AI
  execution and have no user/billing/session logic.
- Product API/control plane is Go and must not depend on Python, FFmpeg or ML runtimes. The Go
  module starts at `github.com/nhathao-nguyen/NH-Media` with domain-oriented `internal/*` packages.
- Go media workers own FFmpeg/media orchestration where applicable; isolated Python workers own
  ML/AI workloads under the `nh_media` namespace.
- Go/Python communication uses versioned language-neutral contracts; never use pickle, gob, ORM
  objects or framework-internal types as durable/public contracts.
- TimelineVersion is renderer source of truth; AI creates proposals and user overrides are preserved.
- Use Asset/Artifact refs across boundaries; local paths exist only inside executor sandboxes.
- Job/JobStep states use canonical terms in `docs/GLOSSARY.md` across DB/API/events.
- Providers, storage, queue and media processes use ports/adapters; provider branches do not scatter
  through pipelines.
- Worker executors are disposable, least-privilege and checkpoint/retry/idempotency aware.
- Never use `shell=True`, auto-load untrusted extensions, expose secrets/tracebacks/paths, or proxy
  large media through Product API.
- Preserve upstream provenance and license evidence. Upstream source, images, modules and services
  must not enter NH-Media production dependency graphs or packages.
- Product API enqueues bounded asynchronous work; it never executes long-running render, FFmpeg,
  transcription or ML inline or creates unbounded goroutine/worker-per-request behavior.

Detailed coding and handoff rules are in `docs/CODEX-INSTRUCTIONS.md`.

Project memory is a concise, versioned operational index. It does not replace the normative
specifications. If memory conflicts with a specification, the specification wins; record and repair
the discrepancy before implementation. Do not store secrets, presigned URLs or user data in memory.
