---
last_verified: 2026-08-16
source: docs/10-DEVELOPMENT-ROADMAP.md; docs/IMPLEMENTATION-ORDER.md; docs/08-SECURITY.md; docs/13-WORKER-ARCHITECTURE.md; docs/SETUP-PLAN.md
owner: repository owner / delivery owner
---

# NH-Media production completion plan

## 1. Objective

Complete NH-Media from the documentation baseline to a production release supporting:

- browser web client and first-class desktop client;
- Product API on a server/VPS separate from user devices;
- durable PostgreSQL product state, Redis coordination and private object storage;
- Go Product API/control plane with bounded Go media workers and isolated Python ML/V1 workers;
- sandboxed media execution;
- frozen Movie Narrator V1 compatibility through `VideoEngine`;
- native V2 pipeline, editable Script/Timeline, review workflow and multi-profile render;
- observable, recoverable, backed-up and rollback-capable production operations.

This plan is an execution contract. It does not authorize work around the documentation gate or
turn an OQ recommendation into a decision. The exact task definition and DoD remain in
[`IMPLEMENTATION-ORDER.md`](IMPLEMENTATION-ORDER.md).

## 2. Completion policy

Work proceeds one task/PR at a time. A task can be marked complete only when implementation, tests,
compatibility, security, migration/rollback, memory and handoff evidence are present. A phase cannot
pass because code compiles; state durability, behavior, boundaries and recovery must be demonstrated.

No compatibility removal, provider lock-in, desktop update channel or production migration is
accepted without an owner-approved decision and rollback path.

## 3. Target production topology

```text
Browser / signed Tauri 2 desktop client
          │ HTTPS only
          ▼
Reverse proxy / TLS / rate limit / origin policy
          ├── web delivery (browser)
          └── Go Product API replicas (no Python/media runtime)
                    ├── PostgreSQL: source of durable product state
                    ├── Redis: queue/lease/event fan-out, not audit source
                    ├── private S3/MinIO: source media and Artifacts
                    └── versioned worker/job contract
                          ├── bounded Go media worker ──> FFmpeg
                          ├── isolated Python ML worker ──> `nh_media`
                          └── frozen V1 compatibility ──> `movie_narrator`
```

Desktop is distributed separately and connects to the same versioned Product API. It never opens
database/Redis/engine ports. Workers can move to a second VPS/GPU host without changing contracts.

## 4. Phase gates

### Phase -1 — Specification and decision gate

Tasks: T000.

T000 has recorded namespace policy `nh_media`/`movie_narrator` plus Go module conventions (OQ-12),
the superseding Go Product/control-plane and isolated Python ML/V1 topology (OQ-13), all verified
V1 compatibility surfaces (OQ-14) and Tauri 2 desktop runtime (OQ-15). Auth/Workspace/queue/provider
choices remain required when their dependent tasks begin. The earlier Python Product/API decision is
preserved in history but is not normative.
The ratification update must remain synchronized across every affected specification, this plan,
memory and implementation order.

Exit evidence: owner sign-off, consistency audit, no undocumented contradiction, explicit list of
remaining non-blocking OQs, and confirmation that no application code was added early.

### Phase 0 — V1 freeze and reproducibility

Tasks: T001–T005.

Record the immutable upstream remote/tag/peeled commit, pinned Go + isolated Python/FFmpeg/environment matrix, V1
CLI/REST/config/status/output profile, golden media outputs and a digest-pinned non-root rollback
image. Run the complete V1 unit/integration/security/media evidence selected by OQ-14.

Exit evidence: another agent can reproduce V1 and detect drift; rollback is executable; license and
dependency exceptions are documented; V1 source is unchanged.

### Phase 1 — Foundation and setup certification

Tasks: T100–T108 plus the verification passes in [`SETUP-PLAN.md`](SETUP-PLAN.md).

Create package boundaries for web, desktop, Go Product API/control plane, Go media worker, isolated
Python ML/V1 workers, contracts, SDK, shared tooling, infra and tests. Add safe ID/time/error
contracts, configuration/redaction, private PostgreSQL, Redis/MinIO, Go API shell, health/readiness
and CI. Client behavior remains a later Phase 5 task; this phase proves the package/contract boundary,
bounded asynchronous execution and no direct engine/provider access.

Exit evidence: setup V0–V10 pass, local and clean VPS profiles reproduce, clients cannot bypass API,
and setup rollback is documented.

### Phase 2 — Product domain, persistence, storage and upload

Tasks: T200–T235.

Implement migrations and scoped repositories for identity, Workspace, Workflow/Pipeline, provider
metadata, Project, Asset, Job/Run/Step, Artifact/Checkpoint/DLQ, Script/Scene/Character,
Timeline/Render, events/outbox and idempotency. Implement StoragePort, presigned multipart upload,
quarantine/probe validation and immutable ScriptVersion/TimelineVersion.

Exit evidence: clean/upgrade migration tests, cross-Workspace negative tests, storage conformance,
malicious upload corpus, no binary in PostgreSQL, no raw path in API/domain/events and optimistic
concurrency evidence.

### Phase 3 — Legacy vertical slice through the V2 control plane

Tasks: T300–T361.

Implement canonical Go state transitions, durable events/replay, QueuePort, scheduler, leases,
sandboxed MediaProcessPort, `VideoEngine`, `LegacyMovieNarratorAdapter`, V1 DTO/status/path mapping,
Go/Python worker protocol,
Artifact commits, checkpoint/crash resume, pause/review/cancel/retry/DLQ, SSE and approved V1
compatibility gateway.

Exit evidence: frozen 16-step V1 runs as a Product Job from web/API and desktop/API flows; duplicate
delivery, worker death, stale lease, cancel/kill/drain, event replay, soft/strict behavior and
rollback all pass. No Product API exposes V1 implementation internals.

### Phase 4 — Native V2 pipeline and studio backend

Tasks: T400–T420.

Add DAG validator/activation, typed node runtime, built-in movie recap graph, durable review
orchestration and `build_timeline`. The same graph supports automatic and studio policies. AI output
is a proposal; user-selected ScriptVersion/TimelineVersion is persisted and preserved.

Exit evidence: graph negative corpus, all node lifecycle paths, review resume, partial execution,
version conflict and override-rerun tests pass. Renderer input is always exact TimelineVersion.

### Phase 5 — Web and desktop clients

Tasks: T430–T434, including T433 desktop shell/remote-session integration and T434 desktop
packaging/security baseline.

Build shared SDK/client reducers first, then web navigation/editor slices and the desktop shell.
Desktop must support remote login, endpoint configuration, direct upload, Job/event reconnect,
review, Artifact download and safe logout. Add desktop build/package/sign/update policy only after
Tauri 2 decision; signing keys never enter the repo.

Exit evidence: web and desktop both pass authorization, upload, reconnect, optimistic concurrency,
review, edit→resume→render and role-denial e2e tests. Desktop binaries pass clean build, tamper/update
verification, OS permission review, endpoint allowlist and rollback-to-previous-build tests.

### Phase 6 — Providers and AI intelligence

Tasks: T500–T524.

Add provider ports/resolver/conformance and port nodes one at a time: LLM Script, TTS/Narration,
ASR/alignment, scene features, VLM caption, Character/Appearance, Embedding and multimodal matching.
Record provider/model/prompt/schema/input fingerprint/cost provenance. Nodes must define timeout,
retry, circuit breaker, fallback, soft/hard behavior, checkpoint and rollback to V1.

Exit evidence: fake/local/approved remote conformance, provider error matrix, privacy/egress review,
quality/golden comparison, deterministic scoring and user-override preservation.

### Phase 7 — Renderer, profiles and production hardening

Tasks: T530–T532 and T600–T605.

Compile only from TimelineVersion; implement reviewed media process adapters, native render/QA,
16:9/9:16/1:1 profile reuse, audio/subtitle/reframe, sandbox/secret/egress controls, observability,
drain/recovery, backup/restore, Artifact inventory, legacy importer, upstream update drill and
compatibility deprecation gate.

Exit evidence: three-profile acceptance, no AI rerun on profile-only output, malicious-media suite,
worker crash/retry/DLQ/replay drills, clean restore/inventory, load/failure bounds and compatibility
removal review.

## 5. Cross-cutting requirements for every phase

### Contracts and ownership

- API, events, storage references and desktop/web SDK are versioned.
- Product owns authorization and durable state; Engine/Worker never infer ownership.
- `Job`/`JobStep` states use Glossary vocabulary across DB/API/events.
- IDs/references cross boundaries; local paths exist only in adapter/executor sandbox.

### Desktop client

- Remote-server mode is default; desktop must not install or run server services for normal use.
- No secret, engine key, provider key, DB/Redis credential or raw presigned URL persistence.
- OS credential store, least native permissions, signed builds and verified update/deep-link origins.
- Local drafts/cache are disposable and cannot override server state.
- Desktop release support matrix, crash reporting and rollback version are recorded before release.

### Compatibility and migration

- V1 is frozen and invoked only behind the legacy adapter.
- Copy → checksum/verify → switch → retain; never delete legacy bytes/state in one migration step.
- Approved Script/Timeline versions are immutable; AI rerun cannot erase user origin.
- Every migrated surface has old-contract test, V2 mapping, removal criteria and rollback route.

### Security and operations

- Upload quarantine, sandbox, egress deny-by-default, scoped secrets, path/archive/symlink guards.
- Safe errors and redacted logs/events/checkpoints; no traceback/secret/path leakage.
- CI scans dependencies/images/licenses and runs V1 regression plus contract/security suites.
- PostgreSQL/object storage backups and clean restore are release prerequisites.

## 6. Staging, canary and production release sequence

1. Freeze a release commit and confirm all task/phase evidence, decisions and memory are current.
2. Build/pin API, worker, web and desktop artifacts; produce SBOM, license/security reports and
   desktop signatures without exposing signing secrets.
3. Apply forward-compatible migrations in staging; run upgrade/rollback/restore drills.
4. Run V1 compatibility, V2 contract/integration/e2e, malicious media and authorization-negative
   suites on staging.
5. Deploy API/worker/infra to staging, execute representative web and desktop flows, then drain and
   restart workers while checking replay/checkpoint/artifact invariants.
6. Deploy canary with feature flag and bounded traffic; monitor API errors, queue age, lease loss,
   DLQ, storage, provider failures, render QA, client crash/update metrics.
7. Expand production traffic only after release owner accepts the canary report.
8. Keep the previous API/worker/client versions and LegacyMovieNarratorAdapter rollback path until
   the deprecation window and usage evidence complete.

## 7. Production acceptance checklist

Production is approved only when every item is evidenced:

- all phase gates and owner sign-offs complete;
- all production-affecting OQs are `DECIDED`, including OQ-15;
- V1 golden regression and compatibility profile pass;
- V2 contract/integration/e2e and three-profile render pass;
- upload/quarantine/malicious-media and cross-Workspace negative suites pass;
- migrations upgrade/downgrade/rollback and clean restore pass;
- worker crash/retry/DLQ/replay/cancel/drain drills pass;
- no raw path, secret, token, presigned URL or traceback leakage;
- dependency/image/license/security scan pass with no unexplained exception;
- API/web/desktop observability, alerts and SLOs are configured;
- desktop package signature/update/tamper/rollback tests pass;
- runbook has been rehearsed by someone other than its author;
- canary and rollback command have been tested on the target server profile;
- compatibility deprecation/removal has usage evidence, replacement parity, announced window and
  owner approval.

## 8. Failure and rollback rules

- Stop new claims/traffic first; do not delete data to make a release appear healthy.
- Reconcile leases, checkpoints, staged objects and outbox before version switching.
- Roll back to the last contract-compatible API/worker and previous desktop build.
- If V2 behavior regresses, route the affected execution to `LegacyMovieNarratorAdapter`.
- Restore database/object data only through the rehearsed restore procedure and verify Artifact
  checksum inventory afterward.
- Keep incident evidence redacted and append the result to `TEST-EVIDENCE.md`/runbook.

## 9. Final handoff

```text
Release: NH-Media production
Status: pass/fail/blocked
Commit/artifacts: API, worker, web, desktop, infra and SBOM references
Phase evidence: -1 through 7 reports
Tests: commands, environments, reports and known limitations
Compatibility: V1 profile, mapping, parity and deprecation status
Security: scans, threat suites, redaction and secret review
Migration/rollback: migration result, restore drill, canary and rollback result
Open questions: none for production, or explicit blocker IDs
Operations: dashboard, alerts, SLO, runbook rehearsal
Next task: post-release follow-up or compatibility review
```
