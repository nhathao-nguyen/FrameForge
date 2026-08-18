---
last_verified: 2026-08-18
source: owner instruction; AGENTS.md; ../OPEN-QUESTIONS.md; ../CODEX-INSTRUCTIONS.md
owner: repository owner / task assignee
---

# Approved decisions

This file records current approved decisions. Recommendations in OPEN questions remain non-binding.

| ID | Date | Owner | Decision | Alternatives considered | Consequence |
|---|---|---|---|---|---|
| D-MEM-001 | 2026-08-17 | repository owner / task intent | Work on `docs/architecture-spec`; preserve unrelated work and perform no push/merge unless requested. | Implicit integration or new branch. | Branch provenance and change boundary remain explicit. |
| D-MEM-002 | 2026-08-17 | repository rules | Keep application code blocked until T004 records approval of the documentation/independence gate. | Scaffold before the gate. | T004 is approved; later implementation tasks may proceed in dependency order, while T300 remains out of scope for Gate C+D. |
| D-T000-012 | 2026-08-17 | repository owner | Product name is NH-Media; Python namespace is `nh_media`; Go module is `github.com/nhathao-nguyen/NH-Media`. | FrameForge, placeholder namespaces or an upstream namespace. | All new public names and repository layout use NH-Media. |
| D-T000-013 | 2026-08-17 | repository owner | Go Product API/control plane, bounded Go media workers, isolated Python `nh_media` ML/AI workers, language-neutral contracts. | Python Product API, embedded engine or framework-internal transport. | Product API has no Python/FFmpeg/ML dependency and all heavy work is asynchronous. |
| D-T000-014 | 2026-08-17 | repository owner | Movie Narrator is research/reference-only with no runtime, build, deploy, import, compatibility, migration or rollback dependency. | Wrapper, fork, migration source, legacy adapter or frozen runtime. | Capability evidence is classified; implementations are independently authored. |
| D-T000-015 | 2026-08-17 | repository owner | Tauri 2 is the thin remote-first desktop client. | Electron, native-per-OS or embedded backend. | Desktop uses Product API and least-privilege native capabilities only. |
| D-T000-016 | 2026-08-17 | repository owner | Local/LAN validation is a valid release milestone before public internet/VPS production. | Require VPS before product validation. | T603 certifies Local/LAN; T605 owns public deployment decisions. |
| D-T000-017 | 2026-08-17 | repository owner | Candidate race and ReferenceStyleAnalysis are native first-class concepts with persisted provenance and user override. | Hidden provider loop or upstream imitation contract. | Domain/API/pipeline/tasks explicitly model candidates, evaluation, selection and style traits. |
| D-T004-001 | 2026-08-17 | repository owner | AuthPort uses LocalAuthProvider initially; bootstrap creates one admin User, one default Workspace and one owner membership; later OIDC is an adapter. | External OIDC requirement, anonymous LAN or deferred Workspace. | Real auth and Workspace authorization are exercised from the first local slice. |
| D-T004-002 | 2026-08-17 | repository owner | SecretStore initially uses encrypted server-side records under a server-owned master key; Workspace credentials are the product default. | Plaintext client/server bundles or required Vault/KMS. | Local development can proceed and secret backend replacement does not change domain contracts. |
| D-T004-003 | 2026-08-17 | repository owner | TimelineVersion is canonical versioned PostgreSQL JSONB edited through typed domain commands with optimistic concurrency. | Normalized canonical Clip tables, unrestricted replace or array-index JSON Patch. | Renderer truth remains atomic and user edits cannot be silently overwritten. |
| D-T004-004 | 2026-08-17 | repository owner | Redis Streams is execution transport; PostgreSQL is canonical Job/DLQ state; SSE is primary progress and REST owns commands/queries. | Lists, broker left open, WebSocket-first or Redis truth. | Queue/recovery/reconnect contracts are implementation-ready. |
| D-T004-005 | 2026-08-17 | repository owner | MinIO is the initial S3-compatible object store; Local/LAN defaults retain protected Artifacts until explicit delete and clean executor scratch after success. | Large PostgreSQL blobs, machine paths or unresolved retention. | Storage migration and resumability do not require redesign. |
| D-T004-006 | 2026-08-17 | repository owner | Local Functional Acceptance precedes Local/LAN Hardened Acceptance and Internet/VPS Production; trusted private-LAN HTTP is allowed only in explicit insecure LAN mode. | VPS/public TLS as a coding prerequisite. | The owner machine and LAN can prove the full functional system first. |
| D-T004-007 | 2026-08-17 | repository owner | T004 is approved by this owner ratification after documentation consistency checks; no further owner gate applies to these decisions. | Another approval round for the same architecture. | T002/T003 are evidence prerequisites, not architecture blockers; T100 follows them. |
| D-T500-001 | 2026-08-18 | task implementation audit | Provider capabilities use typed LLM/VLM/TTS/ASR/embedding contracts with normalized safe errors and explicit allowlisted fallback; domain nodes do not depend on a universal untyped call method. | Provider SDK objects in domain code or a generic request/response port. | Adapter provenance, credential redaction and capability-specific request/response validation remain auditable. |
| D-T500-002 | 2026-08-18 | task contract repair | Provider transient retry is bounded by node timeout/retry budgets, uses exponential backoff plus jitter and honors bounded Retry-After; circuit state is isolated by provider configuration, endpoint and capability. Authentication, policy, permanent and cancellation failures do not retry or fall through to another provider. | Immediate fallback-only resolution or one global provider breaker. | T500 resilience cannot hide credential/policy faults or let one endpoint poison unrelated provider routes. |
| D-T530-002 | 2026-08-18 | task contract repair | Canonical Timeline `source_ref` permits bounded `rights_status` and non-empty `rights_metadata`; music clips require `owned` or `cleared` rights before renderer compilation. | Renderer-only fields outside the strict Timeline schema or accepting unproven music rights. | Product API validation and renderer preconditions now accept and reject the same document contract. |
| D-T531-001 | 2026-08-18 | task implementation audit | Renderer truth is the canonical TimelineVersion; Go compiles bounded multi-clip media/audio/subtitle plans and performs real ffprobe deliverable QA before Artifact commit. | Caller-supplied FFmpeg argv, synthetic narration bytes or commit-before-QA. | Profile dimensions/codecs, duration, required audio, sandbox paths and post-QA checks are explicit acceptance gates. |
| D-T531-002 | 2026-08-18 | task contract repair | Worker commands/results carry scoped identity and Artifact refs only. Workers request lease/step-scoped transfer grants from internal authenticated Product API endpoints and move bytes directly to/from object storage; Redis and Product API do not proxy media bytes. | Durable local paths, signed URLs on Redis or large-media Product API proxying. | Python and Go capability workers can execute the built-in DAG without crossing the storage/security boundary. |

Earlier local notes that described V1 compatibility, `LegacyMovieNarratorAdapter`, FrameForge module
identity or a Python Product API are superseded by D-T000-012 through D-T000-017 and are not current
architecture.

## Decision protocol

Update the authoritative decision register and affected specifications first. A new dated owner
decision is required to reopen or supersede a resolved architecture choice. Then update this table,
implementation dependencies and memory in the same reviewed change.
