# Codex Instructions

## 1. Role and source order

Codex works on NH-Media using the specifications in `docs/`. Read in this order:

1. current user instruction and dated owner decisions;
2. `PROJECT_REBUILD_PLAN.md` and canonical master context;
3. `GLOSSARY.md`, `00`–`14`, upstream-reference policy and capability matrix;
4. `OPEN-QUESTIONS.md` and `IMPLEMENTATION-ORDER.md`;
5. upstream audit/provenance only when researching a capability.

Never infer product architecture from upstream source.

## 2. Documentation gate

Until the owner accepts the specification gate:

- do not write application code, migrations, service/client/worker scaffolding or runtime config;
- only analyze/update documentation, evidence and documentation validation tooling;
- do not use a prototype as a reason to bypass the gate.

After approval, perform one ordered task at a time with its explicit prerequisites.

## 3. Independence rule

NH-Media is independently authored. Movie Narrator is research/reference-only.

Do not:

- fork, wrap, embed, import or execute upstream;
- create a legacy adapter, compatibility API/CLI, worker or rollback image;
- vendor/submodule upstream into product/release paths;
- copy implementation modules blindly;
- require upstream in normal CI, build or deployment.

Use the capability matrix classification and turn research into NH-Media behavior specs, interfaces,
tests and code. Optional comparisons use recorded observations or a separate research environment.

## 4. Boundary rules

- Go Product API owns identity, authorization, Workspace, Project, Asset, Job and database.
- Go media workers own FFmpeg/media orchestration; Python `nh_media` workers own AI/ML.
- Workers have no user/billing/session logic and do not own durable product state.
- Frontends call Product API only and hold no provider/server secret.
- Asset/Artifact refs cross boundaries; LocalHandles remain executor-scoped.
- Job/JobStep states use `GLOSSARY.md` exactly.
- Go/Python/client contracts are versioned and language-neutral.
- Product API never executes long-running media/ML work inline.

## 5. Coding rules after approval

1. Use ports/contracts before adapters.
2. Use `github.com/nhathao-nguyen/NH-Media` for Go and `nh_media` for Python.
3. Keep TimelineVersion renderer-authoritative and user overrides immutable/versioned.
4. Make nodes idempotent, checkpointable, observable and provider-neutral.
5. Snapshot provider/model/input/pipeline revisions.
6. Validate every state transition and emit durable audit/progress events.
7. Enforce authorization, idempotency or optimistic concurrency on mutations.
8. Do not hard-code providers, paths or production executables.
9. Never use `shell=True` or auto-load untrusted extensions.
10. Redact secrets, paths, expiring URLs and tracebacks.
11. Bound worker concurrency, resources, leases, timeout, retry and cancellation.

## 6. Decision protocol

If a task depends on an OPEN decision, stop only that dependent work, document context/options/
trade-offs and continue unrelated authorized work. A recommendation is not approval.

## 7. Required verification

Report files/boundary changed, tests run, independence impact, security review, data/rollback impact
and open questions. Documentation work must state that no application code changed. Every
implementation/release task runs an independence scan for upstream imports/dependencies/artifacts.

## 8. Prohibited shortcuts

- application code before the documentation gate;
- direct media upload through Product API;
- Redis or worker disk as sole truth;
- authorization bypass for internal routes;
- arbitrary FFmpeg arguments/executable from user input;
- unbounded goroutine/worker per request;
- upstream source/runtime used as a temporary shortcut;
- unsupported legal conclusions about external source.

## 9. Handoff

```text
Task: Txxx
Status: complete/blocked
Implemented boundary: ...
Tests: ...
Independence: ...
Security: ...
Data/rollback: ...
Open questions: ...
Next task: ...
```
