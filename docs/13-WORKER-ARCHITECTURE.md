# 13 — Worker Architecture

## 1. Initial topology

```text
Go Product API / trusted orchestration
            ↓ durable outbox
         Redis queue
            ↓ versioned worker contracts
       bounded worker replicas
         ├─ Go media worker → FFmpeg/I/O
         ├─ Python ML worker → nh_media/models
         └─ disposable node executors
```

There is no upstream or compatibility worker. Initial deployment may run one instance of each
worker class on the same LAN server. Every class has bounded concurrency and advertised capability.
Product API never executes compute inline.

## 2. Trust boundary

- **Controller:** trusted service code with queue credential and a scoped execution-state identity.
  It claims leases, resolves manifests, launches executors, heartbeats and reports results.
- **Executor sandbox:** handles untrusted media/model work. It receives only declared input files,
  output staging location, cancellation and the exact provider credential required for the node.

Trusted orchestration owns transitions. A worker report goes through `ExecutionStatePort`, which
validates current state, lease token, attempt and revision. Executors have no broad Product DB,
Redis, user/session, billing or unrelated provider credentials.

## 3. Queue message and claim

```json
{
  "schema_version":"1.0",
  "message_id":"msg_...",
  "job_id":"job_...",
  "pipeline_run_id":"run_...",
  "job_step_id":"step_...",
  "node_key":"detect_scenes",
  "attempt":1,
  "priority":5,
  "available_at":"2026-08-17T10:00:00Z",
  "enqueued_at":"2026-08-17T10:00:00Z"
}
```

Messages contain no media bytes, local paths, plaintext secret, user token or mutable full context.

Claim sequence:

1. Consume at least once.
2. Call `claim(job_step_id, attempt, worker_id, expected_status=queued)`.
3. Orchestrator atomically records `running`, lease hash/expiry and `node.started`.
4. Duplicate/stale messages fail claim and are acknowledged after state inspection.
5. Controller loads exact snapshots, verifies capability and launches the executor.

## 4. Lease and progress

- Heartbeat interval is less than one third of lease duration.
- Heartbeat is independent from media/provider progress.
- Progress is throttled/coalesced and bounded 0–100.
- Lease tokens are opaque, attempt-scoped and never logged.
- Reconciler checks committed evidence before classifying expired work as `worker_lost`.
- Storage and DB preconditions prevent two attempts from committing the same canonical output.

## 5. Executor lifecycle

```text
prepare isolated workspace
→ materialize and checksum declared Artifact inputs
→ resolve only required provider binding
→ execute node with timeout/cancellation
→ validate declared outputs
→ stage output manifests
→ controller commits via trusted orchestration
→ terminate process tree and clean workspace
```

Worker result:

```text
outcome: completed | skipped | waiting_for_review | paused | failed | cancelled
outputs: ProducedBlob/DomainProposal manifests
checkpoint_payload?
progress_summary, warnings, metrics, provider_usage
error {code, category, retryable, safe_message}?
```

Workers do not emit browser events directly or decide aggregate terminal state.

## 6. Resource and sandbox policy

Controller enforces per node:

- non-root, no-new-privileges and read-only root;
- isolated writable temp/output mounts;
- CPU/RAM/GPU/disk/PID/wall-time limits;
- process-group kill on timeout/cancel;
- no host/Docker socket or repository-root mount;
- network deny-by-default with exact storage/provider allowlist;
- pinned FFmpeg/dependencies and no project executable override;
- subprocess argv lists only, never `shell=True`;
- bounded/redacted stdout/stderr.

Extensions/custom executables are disabled unless reviewed, allowlisted and isolated.

## 7. Cancellation, pause and review

- Cancel moves Job to `cancelling`, forwards a token, then confirms terminal state after cleanup.
- Pause succeeds only at a declared checkpoint-safe boundary.
- `stop_after` completes the target, commits a checkpoint and pauses the run/job.
- Human review commits proposal/checkpoint and sets `waiting_for_review`; browser disconnect has no
  effect.
- Approval/rejection is an authorized Product API command, not an arbitrary worker message.

## 8. Statelessness and recovery

Durable state is in PostgreSQL/object storage. Redis messages, controller memory, executor temp
disk and caches are reconstructable.

Recovery:

1. heartbeat stops;
2. reconciler waits for lease expiry;
3. committed checkpoints/outputs are verified;
4. first incomplete compatible JobStep is scheduled with a new attempt;
5. exact input fingerprints reuse compatible committed Artifacts;
6. stale/security failures do not auto-retry.

## 9. Capability routing and distribution

Workers advertise `probe`, `ai`, `ml`, `media`, `render` and resource limits. Scheduler routes by
capability, not workflow name. Later multi-host pools retain the same state/event/Artifact contract.
Render pools receive no LLM/TTS secrets. A complex distributed scheduler is deferred until a
single-host Local/LAN profile passes reliability and load evidence.

## 10. Acceptance

- competing workers cannot duplicate a canonical commit;
- expired leases resume from valid checkpoints;
- worker death resumes the first incomplete node;
- cancel/timeout kills the process tree and reconciles staged output;
- executor cannot access DB/Redis/unrelated secrets/host paths;
- malicious media remains contained;
- bounded Go and Python workers complete the independent NH-Media vertical slice;
- moving a capability across worker hosts preserves the same contracts;
- release scans find no `movie_narrator` import, service, image or execution path.
