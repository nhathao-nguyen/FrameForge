# 13 — Worker Architecture

## 1. Initial versus scale-out topology

Initial architecture (Phase 1–3):

```text
Product API / trusted orchestration
            ↓ DB outbox
         Redis queue
            ↓
   Engine Worker replicas
     ├─ controller
     └─ disposable node executor
```

Một Engine Worker image/class xử lý mọi capability được cài đặt. Có thể chạy nhiều replica nhưng không tách AI/ML/Render service, không yêu cầu Kubernetes và không có distributed scheduler phức tạp.

Scale-out chỉ sau benchmark:

```text
capability queues
  ├─ ai     → AI workers
  ├─ ml     → GPU/ML workers
  └─ render → Render workers
```

Pipeline contract, JobStep state và event contract không đổi khi tách pool.

## 2. Trust boundary

Worker gồm hai trust levels:

- **Controller**: trusted service code, có queue credential và execution-state service identity giới hạn. Controller claim lease, resolve manifests, launch executor, heartbeat và report result.
- **Executor sandbox**: xử lý untrusted media/node. Chỉ nhận input files/manifest, output staging location, cancellation channel và đúng provider credential nếu cần. Không có product DB, broad Redis, user session, billing hoặc unrelated provider secrets.

Trusted orchestration là owner của transition. Controller không được `UPDATE jobs` tùy ý; mọi report đi qua guarded `ExecutionStatePort` để validate current state, lease token và expected revision. Baseline adapter của port dùng cùng application transition service + PostgreSQL repository với một DB role giới hạn cho controller; nếu sau này chuyển sang internal HTTP/gRPC, contract/state machine không đổi. Media executor không bao giờ nhận DB role đó.

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
  "available_at":"2026-08-15T10:00:00Z",
  "enqueued_at":"2026-08-15T10:00:00Z"
}
```

Không chứa bytes, local path, plaintext secret, user token hoặc mutable full context. Controller load exact snapshots sau successful atomic claim.

Claim protocol:

1. Consume message at-least-once.
2. Call `claim(job_step_id, attempt, worker_id, expected_status=queued)`.
3. Orchestrator atomically creates/updates attempt `running`, lease token hash/expiry và `node.started` event.
4. Duplicate/stale message không claim được thì ack/drop sau state inspection.
5. Controller materialize inputs, launch executor và heartbeat dưới lease token.

## 4. Lease, heartbeat and progress

- Lease duration runtime-configured; heartbeat interval nhỏ hơn một phần ba lease.
- Heartbeat độc lập với frame/provider progress.
- Progress được throttle/coalesce nhưng không vượt 0–100; terminal commit không phụ thuộc event fan-out.
- Lease token opaque, scoped one attempt, không log.
- Reconciler xử lý expired lease: inspect output commit/checkpoint, mark attempt failed `worker_lost`, rồi JobStep `retrying` hoặc terminal theo policy.
- Hai attempt không được cùng promote canonical output; storage/DB precondition bảo vệ split-brain.

## 5. Executor lifecycle

```text
prepare isolated workspace
  → materialize/checksum declared inputs
  → resolve only required provider binding
  → execute node with timeout/cancel token
  → validate declared outputs
  → stage outputs and return manifest
  → controller commits through orchestration
  → terminate process tree and clean workspace
```

Executor result contract:

```text
outcome: completed | skipped | waiting_for_review | paused | failed | cancelled
outputs: ProducedBlob/DomainProposal manifests
checkpoint_payload?
progress_summary, warnings, metrics, provider_usage
error {code, category, retryable, safe_message}?
```

Executor không phát trực tiếp browser event và không quyết định Job aggregate terminal state.

## 6. Resource and sandbox policy

Mỗi node definition khai báo execution class và resource requirements. Controller enforce:

- non-root UID/GID, no-new-privileges, read-only root;
- isolated writable temp/output mounts;
- CPU, RAM, GPU, disk, PID and wall-time quota;
- process-group kill khi timeout/cancel;
- deny host/Docker socket và workspace root mount;
- network deny-by-default; allow exact object storage/provider endpoints;
- pinned FFmpeg and dependencies; no project-level executable override in production;
- subprocess argv list only, never `shell=True`;
- redacted stdout/stderr size limits.

Production plugin/custom executable disabled unless allowlisted and isolated under stricter profile.

## 7. Cancellation, pause and review

- Cancel sets Job `cancelling`; controller forwards token; long node checks at documented safe points.
- Nếu graceful timeout hết, controller kills executor tree, reconciles staged output và confirms `cancelled` only after cleanup policy.
- Pause chỉ thành công tại checkpoint-safe boundary. Node không checkpointable hoàn tất current atomic unit hoặc trả capability error theo policy.
- `stop_after` marks completed target, commits checkpoint, sets run/job `paused`; không gọi đây là review.
- Human gate commits proposal/checkpoint, sets step/run/job `waiting_for_review`; browser disconnect không ảnh hưởng.
- Approval/rejection là Product API command có actor/audit, không là worker message tùy ý.

## 8. Statelessness and recovery

Durable state nằm ở PostgreSQL/object storage. Redis message, controller memory, executor temp disk và local cache đều reconstructable.

Worker restart flow:

1. active attempts ngừng heartbeat;
2. reconciler chờ lease expiry, không assume failure ngay;
3. verify committed checkpoint/output;
4. retry same JobStep attempt+1 từ first non-committed boundary;
5. idempotency fingerprint reuse committed artifacts;
6. no retry khi input/pipeline/provider snapshot stale hoặc security failure.

V1 adapter maps V1 checkpoint JSON/path context to this protocol; V1 process crash behavior không được xem là đủ reliability cho product path.

## 9. Capability routing evolution

Baseline worker advertises installed capabilities (`probe`, `ai`, `ml`, `media`, `render`) and resource limits. Scheduler chỉ queue node cho compatible worker. Khi scale:

- route by execution class/capability, not hard-coded workflow;
- retain one shared state machine and conformance suite;
- provider/network secrets scoped to AI/ML pool;
- Render pool receives no LLM/TTS credentials;
- fallback to local render/distributed behavior phải là explicit node policy.

Upstream `cloud/distributed.py` là best-effort remote render dispatch với local fallback, không phải shared durable scheduler và input sharing còn deployment-specific. Không dùng nó làm bằng chứng rằng V1 đã có production distributed queue.

## 10. Worker acceptance tests

- duplicate delivery and competing workers cannot duplicate canonical commit;
- expired lease resumes from valid checkpoint;
- worker death between TTS and scene detection resumes at first incomplete node;
- cancel/timeout kills process tree and cleans staged output;
- executor cannot access DB/Redis/unrelated secret/host path;
- oversized/malicious media is contained by quotas and quarantine flow;
- one baseline Engine Worker runs full compatibility pipeline;
- capability split later runs same pipeline/event contract without migration;
- graceful drain rejects claims, lets bounded attempts finish, then reconciles remainder.
