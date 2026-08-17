# 07 — Events and Jobs

## 1. Guarantees

- Job command/snapshots commit PostgreSQL before enqueue.
- Queue delivery at-least-once; JobStep execution and Artifact commit idempotent.
- PostgreSQL is source of truth; Redis carries queue/live fan-out only.
- Every state transition is guarded, transactional with durable event/outbox, and uses canonical state vocabulary.
- Worker claim uses lease + heartbeat; media executor memory/local disk is disposable.
- Progress is an estimate; terminal event is emitted only after state/output transaction commits.
- Cancellation is cooperative then bounded kill; API does not claim `cancelled` while state is `cancelling`.

## 2. Execution sequence

```text
API transaction
  INSERT Job(created), PipelineRun(created), JobSteps(pending)
  INSERT job.created + outbox
commit
  ↓
orchestrator resolves ready nodes
  JobStep pending → ready → queued
  Job/PipelineRun created → queued
  ↓
Redis delivery
  ↓
worker controller claim
  Job/PipelineRun queued → running
  JobStep queued → running
  ↓
executor progress/checkpoint/result
  ├─ completed/skipped → unlock dependants
  ├─ paused
  ├─ waiting_for_review
  ├─ retrying → queued
  ├─ failed/blocked
  └─ cancelled
  ↓
aggregate Job transition and committed event
```

Queue message contract is in `13-WORKER-ARCHITECTURE.md`; no media bytes, local paths or secrets.

## 3. State consistency

Canonical Job states:

```text
created | queued | running | paused | waiting_for_review | retrying |
cancelling | completed | failed | dead_lettered | cancelled
```

Canonical JobStep/node execution states:

```text
pending | ready | queued | running | paused | waiting_for_review |
retrying | completed | skipped | failed | cancelled | blocked
```

`02-DOMAIN-MODEL.md` owns transitions. REST response, PostgreSQL `status`, event payload `from_status/to_status` and frontend reducer must use these exact strings.

Upstream task/status names are research observations only. They are not accepted by Product API,
stored in PostgreSQL, emitted in events or mapped by a compatibility gateway.

## 4. Lease protocol

1. Scheduler atomically moves a dependency-satisfied JobStep `pending → ready`, writes event/outbox.
2. Publisher moves `ready → queued` only with queue/outbox precondition; duplicate publish retains same message/event ID.
3. Controller claims `queued → running` with expected attempt, worker ID, token hash and expiry.
4. Heartbeat renews only current token/attempt; stale controller receives conflict and must stop.
5. Executor reports progress under current lease; updates are bounded/coalesced.
6. Output follows stage → validate → promote → DB commit. Duplicate attempt cannot create second canonical role.
7. Expired lease reconciler classifies `worker_lost`, then `running → retrying` or `failed` based on policy.
8. Job/PipelineRun aggregate transition occurs after all required JobSteps have deterministic state.

Lease expiry by itself không xóa output/checkpoint. Reconciler checks committed evidence first.

## 5. Retry, timeout, fallback and DLQ

Retryable categories:

- provider/network timeout, rate-limit and temporary unavailable;
- transient object-storage/queue connection;
- worker lost/lease expired;
- executor process interruption;
- GPU OOM only when node policy permits retry on another compatible worker.

Non-retryable categories:

- schema/semantic input invalid;
- unauthorized, invalid credential or provider content policy;
- quarantined/missing Asset;
- unsupported media/profile/codec under declared requirements;
- deterministic Timeline/render validation;
- plugin/security sandbox violation;
- user cancellation.

JobStep `running → retrying` creates terminal attempt row `failed`, schedules attempt+1 with exponential backoff + jitter, then `retrying → queued`. Timeout is a `failure_category`, not an extra node status.

Provider fallback:

1. adapter translates error taxonomy;
2. node retry budget applies;
3. declared fallback condition may select next ProviderConfiguration;
4. each call writes ProviderRun provenance;
5. if all providers fail, node follows hard/soft failure mode.

Không fallback provider vì auth/policy error trừ khi policy explicitly declares a separately authorized configuration. Không silently mix partial VLM/embedding result without item provenance.

When Job run budget exhausted:

- DLQ enabled: Job `dead_lettered`, `dead_letters` row and `job.dead_lettered` event commit together;
- DLQ disabled: Job `failed`;
- replay creates new Job/PipelineRun IDs and `supersedes_job_id`; old history immutable.

## 6. Checkpoint, partial execution and review

Checkpoint commits after every JobStep `completed|skipped`, before `node.completed|node.skipped`; chunk checkpoint is allowed only by node contract. It stores references/hashes, never local path/secret.

`start_from`:

- validates declared prior outputs/checkpoint/input fingerprints;
- compatible prior nodes become `skipped` with `skip_reason=reused_checkpoint|partial_boundary` and retained refs;
- missing/stale required output returns `422`, not a hacked execution.

`stop_after`:

- target completes and checkpoint commits;
- active PipelineRun/Job becomes `paused`;
- resume queues first not-completed compatible JobStep.

Human review payload:

```json
{
  "review_id":"review_...",
  "review_type":"script|timeline|scene_match|subtitle|voice",
  "resource_type":"script_version",
  "resource_id":"scriptv_...",
  "resource_revision":4,
  "actions":["approve","reject","edit_then_resume"],
  "expires_at":null
}
```

Review is durable. `review.required` commits with Job/Run/Step `waiting_for_review`. Approve/reject requires actor + exact resource version and emits `review.approved|review.rejected`; browser disconnect has no effect.

Approve payload records both `proposed_resource_*` and `selected_resource_*`. Selected version may be a user-edited descendant of proposal. Orchestrator completes/requeues the review node according to policy, writes a new checkpoint and invalidates only downstream fingerprints; immutable Job command/input snapshot remains unchanged.

## 7. Event envelope

Every durable/live domain progress event uses:

```json
{
  "event_id":"evt_01",
  "event_type":"node.progress",
  "schema_version":"1.0",
  "sequence":27,
  "occurred_at":"2026-08-15T10:01:05.123Z",
  "workspace_id":"ws_01",
  "project_id":"proj_01",
  "job_id":"job_01",
  "pipeline_run_id":"run_01",
  "job_step_id":"step_01",
  "pipeline_node_id":"pnode_01",
  "node_key":"detect_scenes",
  "correlation_id":"corr_01",
  "payload":{
    "node_status":"running",
    "percent":41.2,
    "message":"Analyzing frames",
    "units":{"completed":8200,"total":20000,"name":"frames"},
    "estimated_remaining_sec":132
  }
}
```

Required every event: `event_id`, `event_type`, `schema_version`, `sequence`, `occurred_at`, `workspace_id`, `project_id`, `job_id`, `correlation_id`, `payload`. Node events additionally require `pipeline_run_id`, `job_step_id`, `pipeline_node_id`, `node_key`.

Rules:

- `sequence` strictly increases per Job and is allocated transactionally.
- Consumer dedupes `event_id`; ordered reducer uses sequence.
- Payload schema is versioned per `event_type`; additive compatible change keeps major, breaking payload creates new schema major/event version.
- `message` is safe/localizable hint, never state source.
- Error payload has safe code/category/retryable, no traceback/path/secret.

## 8. Event catalog

| Event type | Trigger | Required payload |
|---|---|---|
| `job.created` | Job/first run committed | kind, mode, pipeline ID/version |
| `job.queued` | first executable work queue-ready | `from_status`, `to_status=queued`, priority |
| `job.started` | first active lease | `from_status`, `to_status=running` |
| `job.paused` | safe checkpoint pause/stop_after | checkpoint ID, resume node |
| `job.waiting_for_review` | review gate aggregate transition | review ID/type/resource |
| `job.retry_scheduled` | run/node recovery causes aggregate backoff | run number, delay, safe error |
| `job.cancellation_requested` | cancel accepted | prior status, actor/reason |
| `job.completed` | required graph/artifacts committed | duration, artifact IDs/roles |
| `job.failed` | terminal non-DLQ failure | failed node, safe error |
| `job.dead_lettered` | DLQ row committed | dead letter ID, safe error |
| `job.cancelled` | cancel confirmed | cancelled step/reason |
| `pipeline_run.created` | engine run snapshot committed | run number, engine/contract version |
| `pipeline_run.started` | run active | run number |
| `pipeline_run.completed` | run terminal success | duration, checkpoint summary |
| `pipeline_run.failed` | run terminal failure | safe error/retryability |
| `node.ready` | dependencies/inputs satisfied | node key, attempt |
| `node.queued` | work message ready | node key, attempt, execution class |
| `node.started` | lease claim committed | node key, attempt, worker/capability |
| `node.progress` | bounded progress update | node status, percent, units/message |
| `node.checkpointed` | checkpoint committed | checkpoint ID, input/state hash |
| `node.paused` | node safe pause | checkpoint ID, resume support |
| `node.completed` | required outputs committed | output refs, duration, attempt |
| `node.skipped` | optional/boundary/degraded skip | reason, consequence, reused refs |
| `node.retry_scheduled` | retry backoff committed | next attempt, delay, safe error |
| `node.failed` | terminal node failure | safe error, attempt, retryable=false |
| `node.cancelled` | node cancel confirmed | attempt/reason |
| `node.blocked` | dependency terminal prevents run | dependency IDs/reason |
| `artifact.created` | staged Artifact metadata exists | artifact ID, kind, role |
| `artifact.committed` | immutable blob visible | artifact ID, role, size, checksum |
| `artifact.quarantined` | validation/security quarantine | artifact ID, safe reason |
| `review.required` | review request + checkpoint committed | review payload |
| `review.approved` | authorized approval command | review/resource/actor IDs |
| `review.rejected` | authorized reject command | review ID, action, safe reason |
| `render.created` | Render/Job committed | render ID, timeline version, profile |
| `render.started` | render node starts | render ID, profile |
| `render.completed` | QA + canonical outputs committed | render ID, artifact roles, QA summary |
| `render.failed` | render terminal failure | render ID, safe error |

No generic `job.status_changed` is required for clients; explicit event types above carry `from_status/to_status` where applicable. This prevents each producer inventing state aliases.

## 9. SSE

```text
GET /api/v1/projects/{project_id}/jobs/{job_id}/events/stream
Accept: text/event-stream
Last-Event-ID: 26
```

Frame:

```text
id: 27
event: node.progress
retry: 3000
data: {"event_id":"evt_01","event_type":"node.progress",...}

```

Behavior:

- No `Last-Event-ID`: emit `stream.snapshot` containing canonical Job/current steps refs, then live events.
- With ID: replay durable `sequence > ID` before live subscribe.
- Retention gap: emit `stream.reset {snapshot_url,last_available_sequence}`; client refetches and reconnects.
- Keepalive comment at most every 20 seconds of silence.
- Close after terminal event plus short flush grace; disconnect is not failure/cancel.
- Auth rechecked at connect and bounded stream lifetime/refresh policy.

## 10. WebSocket

Optional endpoint `/api/v1/ws/jobs/{job_id}/events` uses same envelope/order/replay.

Client:

```json
{"type":"subscribe","after_sequence":26}
```

Server control messages:

```jsonl
{"type":"subscribed","job_id":"job_01","from_sequence":27}
{"type":"snapshot_required","reason":"retention_gap","snapshot_url":"/api/v1/.../jobs/job_01"}
{"type":"error","code":"FORBIDDEN","message":"..."}
```

Client cannot send state/event mutations over WebSocket. Commands remain authorized REST requests.

## 11. Progress aggregation

Job `progress_percent` is materialized from Pipeline `progress_weight`:

- `completed|skipped` JobStep contributes 100% of its weight;
- `running` contributes reported percent;
- `pending|ready|queued|paused|waiting_for_review|retrying` retains last committed percent, never fabricates completion;
- `blocked|failed|cancelled` stops aggregate and terminal/paused UX follows Job state;
- soft skip surfaces warning/consequence.

Frontend reducers use snapshot + sequence, not infer state from event names alone. Artifact download appears only after `artifact.committed` or REST confirms committed.

## 12. Outbox and observability

State/event/outbox rows commit together. Publisher retries with same event ID; live duplicate is harmless. Event replay uses `job_events`, not Redis retention.

Structured logs/traces share request/correlation/job/run/step IDs. Heartbeats and high-frequency encoder telemetry go to observability channel, not durable browser event by default. Provider content/credentials, media bytes, filesystem paths and Python tracebacks are redacted; usage/cost/latency use `provider_runs`.
