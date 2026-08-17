---
last_verified: 2026-08-17
source: owner instruction; AGENTS.md; ../OPEN-QUESTIONS.md; ../CODEX-INSTRUCTIONS.md
owner: repository owner / task assignee
---

# Approved decisions

This file records current approved decisions. Recommendations in OPEN questions remain non-binding.

| ID | Date | Owner | Decision | Alternatives considered | Consequence |
|---|---|---|---|---|---|
| D-MEM-001 | 2026-08-17 | repository owner / task intent | Work on `docs/architecture-spec`; preserve unrelated work and perform no push/merge unless requested. | Implicit integration or new branch. | Branch provenance and change boundary remain explicit. |
| D-MEM-002 | 2026-08-17 | repository rules | Keep application code blocked until T004 records approval of the documentation/independence gate. | Scaffold before the gate. | Current work is documentation/evidence only. |
| D-T000-012 | 2026-08-17 | repository owner | Product name is NH-Media; Python namespace is `nh_media`; Go module is `github.com/nhathao-nguyen/NH-Media`. | FrameForge, placeholder namespaces or an upstream namespace. | All new public names and repository layout use NH-Media. |
| D-T000-013 | 2026-08-17 | repository owner | Go Product API/control plane, bounded Go media workers, isolated Python `nh_media` ML/AI workers, language-neutral contracts. | Python Product API, embedded engine or framework-internal transport. | Product API has no Python/FFmpeg/ML dependency and all heavy work is asynchronous. |
| D-T000-014 | 2026-08-17 | repository owner | Movie Narrator is research/reference-only with no runtime, build, deploy, import, compatibility, migration or rollback dependency. | Wrapper, fork, migration source, legacy adapter or frozen runtime. | Capability evidence is classified; implementations are independently authored. |
| D-T000-015 | 2026-08-17 | repository owner | Tauri 2 is the thin remote-first desktop client. | Electron, native-per-OS or embedded backend. | Desktop uses Product API and least-privilege native capabilities only. |
| D-T000-016 | 2026-08-17 | repository owner | Local/LAN validation is a valid release milestone before public internet/VPS production. | Require VPS before product validation. | T603 certifies Local/LAN; T605 owns public deployment decisions. |
| D-T000-017 | 2026-08-17 | repository owner | Candidate race and ReferenceStyleAnalysis are native first-class concepts with persisted provenance and user override. | Hidden provider loop or upstream imitation contract. | Domain/API/pipeline/tasks explicitly model candidates, evaluation, selection and style traits. |

Earlier local notes that described V1 compatibility, `LegacyMovieNarratorAdapter`, FrameForge module
identity or a Python Product API are superseded by D-T000-012 through D-T000-017 and are not current
architecture.

## Decision protocol

Update the authoritative OQ and affected specifications first. Only a dated owner decision can move
an owner question from OPEN. Then update this table, implementation dependencies and memory in the
same reviewed change.
