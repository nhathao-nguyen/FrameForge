# Repository Agent Rules

Read before changing this repository:

1. `PROJECT_REBUILD_PLAN.md` points to the complete architecture master context.
2. `docs/00-PROJECT-CONTEXT.md` and `docs/GLOSSARY.md` define scope/terminology.
3. Use the topic specification in `docs/01`–`docs/14` and `docs/OPEN-QUESTIONS.md`.
4. Follow `docs/IMPLEMENTATION-ORDER.md`; an OPEN recommendation is not an approved decision.
5. `references/movie-narrator` is frozen legacy/reference behavior, not V2 architecture.
6. Before starting work, read `docs/memory/PROJECT-MEMORY.md`, `docs/memory/CURRENT-STATE.md`,
   `docs/memory/DECISIONS.md`, and `docs/memory/IMPLEMENTATION-STATUS.md`.
7. For setup or release work, also read `docs/SETUP-PLAN.md` and `docs/PRODUCTION-PLAN.md`.
8. After each task, update the relevant memory/evidence entry in the same change.

Project-wide invariants:

- Do not implement application code until the documentation gate is explicitly approved.
- Keep Movie Narrator V1 behind `VideoEngine`/legacy adapter; no wholesale rewrite or namespace rename.
- Product backend owns identity/authorization/Project/Job/database; engine owns media/AI execution and has no user/billing/session logic.
- TimelineVersion is renderer source of truth; AI creates proposals and user overrides are preserved.
- Use Asset/Artifact refs across boundaries; local paths exist only inside adapter/executor sandbox.
- Job/JobStep states must use canonical terms in `docs/GLOSSARY.md` across DB/API/events.
- Providers, storage, queue and media processes use ports/adapters; no provider branches scattered in pipeline.
- Worker executors are disposable, least-privilege and checkpoint/retry/idempotency aware.
- Never use `shell=True`, auto-load untrusted plugins, expose secrets/tracebacks/paths, or proxy large media through Product API.
- Preserve upstream license/attribution and compatibility behavior until documented removal criteria pass.

Detailed coding/handoff rules are in `docs/CODEX-INSTRUCTIONS.md`; do not duplicate the full specification here.

Project memory is a concise, versioned operational index. It does not replace the normative
specifications. If memory conflicts with a specification, the specification wins; record the
discrepancy and repair the memory before implementation. Do not store secrets, presigned URLs,
user data, or durable local paths in memory.
