---
last_verified: 2026-08-17
source: docs/10-DEVELOPMENT-ROADMAP.md; docs/IMPLEMENTATION-ORDER.md; docs/08-SECURITY.md; docs/13-WORKER-ARCHITECTURE.md; docs/SETUP-PLAN.md
owner: repository owner / delivery owner
---

# NH-Media production completion plan

## 1. Objective

Deliver NH-Media as an independent product with browser and Tauri clients, Go Product API, durable
PostgreSQL/Redis/object storage, bounded Go media workers, isolated Python `nh_media` workers,
editable TimelineVersion workflows, safe rendering and recoverable operations.

Movie Narrator remains research/provenance only. Production does not ship or call upstream and has
no compatibility or rollback dependency on it.

## 2. Completion policy

Work follows `IMPLEMENTATION-ORDER.md` one reviewable task at a time. Completion requires tests,
independence, security, migration/data safety, memory and handoff evidence. Compilation alone is not
acceptance.

## 3. Production topology

```text
Browser / signed Tauri 2 client
          │ HTTPS
          ▼
reverse proxy / origin / rate controls
          ├── web delivery
          └── Go Product API replicas
                ├── PostgreSQL
                ├── Redis Streams
                ├── private S3/MinIO
                └── versioned worker contracts
                     ├── bounded Go media workers → FFmpeg
                     └── isolated Python ML workers → nh_media
```

The same topology can run on one Local/LAN server before services are moved to public/managed hosts.

## 4. Release phases

### Phase 0 — Specification

Independent architecture, reference policy, capability matrix, contracts, tasks and decisions pass
the final audit. No application code is added.

### Phase 1 — Foundation

Complete `SETUP-PLAN.md`: pinned toolchains, boundaries, private data services, Go API shell,
workers, clients, shared contracts, CI and independence scan.

### Phase 2 — Product state and execution

Implement domain/schema, storage/uploads, Job/events/outbox/queue, worker leases/checkpoints/retry/
DLQ and fake-node conformance.

### Phase 3 — Independent vertical slice

Prove remote client → API → durable Job → Go media worker → Artifact → event/result on Local/LAN.
No upstream checkout/package/image is present.

### Milestone — LOCAL FUNCTIONAL ACCEPTANCE

Start PostgreSQL, Redis Streams, MinIO, Product API, Go media worker, Python `nh_media` worker, web
and Tauri development client on the owner machine. Prove LocalAuth/default Workspace bootstrap,
Project/upload/Job, deterministic FFmpeg Artifact, one minimal Python worker task, SSE result,
restart recovery and an explicit second LAN client. VPS, public DNS and public TLS are not required.

### Phase 4 — Pipeline/studio

Implement native DAG, Script/Scene/Analysis/Timeline/reviews, web/Tauri review slices and canonical
Timeline compiler boundary.

### Phase 5 — Core recap capability

Independently implement providers, script, narration, alignment, scene analysis, subtitles,
matching, audio mix, render, QA and exports.

### Phase 6 — Intelligence and multi-output

Add candidates/evaluation/selection, Character, embeddings, ReferenceStyleAnalysis, coverage,
auto-reframe and reusable 16:9/9:16/1:1 outputs.

### Phase 7 — LOCAL/LAN HARDENED ACCEPTANCE

Prove auth, multi-client operation, backups/restores, worker failure recovery, observability,
malicious-media containment and signed staging desktop on a LAN server.

### Phase 8 — INTERNET / VPS PRODUCTION

Add public TLS/DNS/ingress, canary, SLO/alerts and rollback only after the functional Local/LAN
release. VPS/public hosting is not a blocker for earlier phases.

## 5. Cross-cutting requirements

- Product API owns authorization/durable state; workers never infer ownership.
- TimelineVersion is renderer input and user versions survive AI reruns.
- IDs/Artifact refs cross boundaries; paths are sandbox-only.
- Jobs are bounded asynchronous; PostgreSQL remains authoritative.
- Desktop remains thin and contains no server compute/secrets.
- Provider/storage/queue/media adapters are explicit and allowlisted.
- Every release scan proves no `movie_narrator` import, package, image, service, route or vendored
  source in product artifacts.

## 6. Local/LAN release sequence

1. Freeze a candidate commit and run CI/security/license/independence scans.
2. Build pinned API/worker/web/desktop artifacts and SBOMs.
3. Apply migrations on a clean LAN server and run backup/restore rehearsal.
4. Connect browser and desktop from separate LAN clients.
5. Run upload → Job → worker → review → Timeline → render → download flows.
6. Kill/restart workers and dependencies; verify lease/checkpoint/event/Artifact recovery.
7. Run malicious-media, cross-Workspace and secret/path negative suites.
8. Record owner acceptance or concrete blockers.

This sequence is hardening/release evidence after Local Functional Acceptance; it does not delay the
first functional system.

## 7. Internet release sequence

After Local/LAN pass: deploy staging, rehearse migrations/restore, build signed clients, run full
e2e/security/load tests, deploy a bounded canary, monitor queue/lease/provider/storage/render/client
metrics, then expand with an already tested rollback to the last contract-compatible NH-Media build.

## 8. Acceptance checklist

- all phase/task evidence and production-affecting decisions complete;
- independent vertical slice and full recap flow pass;
- three-profile reuse and user-override preservation pass;
- upload/quarantine/malicious-media and authorization-negative suites pass;
- migration and clean restore pass;
- worker crash/retry/DLQ/cancel/drain/replay pass;
- no secret, path, presigned URL or traceback leakage;
- dependency/image/license/SBOM and upstream-independence scans pass;
- web/desktop observability and desktop signature/update/rollback pass;
- Local/LAN runbook rehearsed; internet canary required only for public release.

## 9. Failure and rollback

Stop new claims/traffic, reconcile leases/outbox/staged objects, and roll back to the last
contract-compatible NH-Media API/worker/client build. Database/object restore follows a rehearsed
procedure and checksum inventory. Never delete data or introduce an upstream runtime to recover a
release.

## 10. Final handoff

```text
Release: NH-Media
Status: pass/fail/blocked
Artifacts: API, workers, web, desktop, infra, SBOM
Phase evidence: 0–8 as applicable
Tests: commands/environments/reports/limitations
Independence: upstream absence proof
Security: scans/threat suites/redaction
Migration/data rollback: result
Open decisions: none for selected release profile, or blockers
Operations: dashboards/alerts/runbook
```
