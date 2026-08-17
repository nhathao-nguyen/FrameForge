# 04 — REST API Contract

## 1. Conventions

- Base path `/api/v1`; JSON UTF-8; UTC ISO-8601; opaque IDs.
- Product authentication required trừ explicitly public health endpoint; every resource lookup scopes Workspace/Project before response.
- List: `limit` default 50/max 100, opaque `cursor`, stable sort `(created_at,id)` hoặc endpoint-defined.
- Mutable aggregate response có `revision` và `ETag: "<revision-or-content-hash>"`; write yêu cầu `If-Match` khi specified.
- Create/command endpoints marked idempotent require `Idempotency-Key`. Same key + same request returns stored response; same key + different hash returns `409 IDEMPOTENCY_CONFLICT`.
- Response không chứa local path, storage credential, provider secret, traceback hoặc raw queue implementation.
- Job/JobStep status dùng canonical states ở `02-DOMAIN-MODEL.md`; NH-Media không trả
  `processing`, `executing`, `succeeded`, `waiting_review` hoặc `dead`.
- Delete Project/Asset/Script/Timeline/ProviderConfiguration là soft-delete/command; Artifact physical deletion không public direct action.
- OpenAPI is the machine-readable source for `/api/v1`; generated/validated client types must not
  introduce a second contract.

### Client compatibility

Web và desktop là hai client của cùng Product API, không phải hai backend khác nhau. Cả hai dùng
`packages/shared-contracts` và `packages/sdk` để chia sẻ ID, error envelope, ETag/If-Match, pagination,
upload-session và event reducer. Client không được gọi `VideoEngine`, worker, provider, PostgreSQL,
Redis hoặc object storage credentials trực tiếp.

- Web chạy browser và có thể được serve cùng reverse proxy hoặc CDN.
- Desktop chạy trên máy người dùng, chỉ giữ endpoint/session/local draft cần thiết; native shell
  không sở hữu Product state.
- Server base URL có thể khác theo dev/staging/production nhưng API version và contract phải giữ
  tương thích; không hard-code localhost trong client release.
- Upload/download bytes dùng presigned URL được cấp theo resource authorization; Product API không
  proxy large media.
- Desktop reconnect/retry dùng cùng snapshot + event replay semantics với web.

### Error envelope

```json
{
  "error": {
    "code": "JOB_INVALID_STATE",
    "message": "Job cannot be resumed from completed state.",
    "details": {"current_status": "completed", "allowed_statuses": ["paused", "waiting_for_review"]},
    "request_id": "req_..."
  }
}
```

HTTP status: `400` malformed, `401` unauthenticated, `403` forbidden, `404` absent/scoped, `409` state/version/idempotency conflict, `412` failed `If-Match`, `413` request/declared upload too large, `415` unsupported declared media type, `422` semantic/schema invalid, `429` quota/rate limit, `500` internal, `503` dependency unavailable.

Validation error không tạo failed Job.

## 2. Authentication and Workspace bootstrap

Initial Local/LAN auth is `LocalAuthProvider` behind `AuthPort`. Installation performs an idempotent,
server-side bootstrap of one local admin User, one default Workspace and one owner membership.
Bootstrap credentials are supplied or generated through installation tooling and must be changed or
acknowledged according to the bootstrap policy; no anonymous LAN mode or `DEV_DISABLE_AUTH` path is
part of the normal architecture.

| Method/path | Contract |
|---|---|
| `POST /auth/login` | Local username/password; rate-limited; returns/sets an opaque server-controlled session without returning credential material. |
| `POST /auth/refresh` | Rotates an unexpired authorized session; old token is revoked. |
| `POST /auth/logout` | Revokes current session idempotently. |
| `GET /auth/session` | Returns redacted User, Workspace memberships and expiry. |
| `GET /workspaces` | Returns only memberships visible to the actor. |
| `GET /workspaces/{workspace_id}` | Workspace metadata scoped by membership. |

Web may use an HttpOnly, SameSite-protected cookie with CSRF controls; Tauri uses an opaque bearer
session stored in the OS credential store. Both map to hashed, revocable server session records and
execute the same Workspace authorization checks. A later OIDCAuthProvider changes authentication,
not User/Workspace/resource contracts.

## 3. Project CRUD

| Method/path | Contract | Success |
|---|---|---|
| `POST /projects` | `{name, workflow_key, settings?}`; resolves authorized active Workflow. Idempotent. | `201 Project` |
| `GET /projects` | Filters `status`, pagination. | `200 {items,next_cursor}` |
| `GET /projects/{project_id}` | Summary + current ScriptVersion/TimelineVersion refs/counts. | `200 Project` |
| `PATCH /projects/{project_id}` | `name|settings|status`; requires `If-Match`; ownership not editable. | `200 Project` |
| `DELETE /projects/{project_id}` | Soft-delete command; rejects/coordinates active Jobs. Idempotent. | `202 Project(status=deleted)` |

Create response example:

```json
{
  "id":"proj_...", "workspace_id":"ws_...", "name":"My recap",
  "workflow":{"id":"wf_...","key":"movie_recap"},
  "status":"active", "revision":1, "created_at":"..."
}
```

## 4. Asset upload and metadata

### `POST /projects/{project_id}/assets/upload-sessions`

Idempotent request:

```json
{
  "kind":"video",
  "filename":"source.mp4",
  "content_type":"video/mp4",
  "size_bytes":123456789,
  "sha256":"optional-client-checksum",
  "multipart":true
}
```

Response `201`:

```json
{
  "asset":{"id":"asset_...","status":"uploading","kind":"video","revision":1},
  "upload":{
    "id":"upl_...","status":"active","method":"multipart",
    "part_size":8388608,"expires_at":"...",
    "parts":[{"part_number":1,"method":"PUT","url":"https://...","headers":{}}],
    "more_parts":true
  }
}
```

API kiểm tra declared quota/type/size và tự tạo key; bytes đi Browser → S3/MinIO, không qua API/Redis/worker.

Với upload rất lớn, response chỉ presign batch đầu. `POST /projects/{project_id}/assets/{asset_id}/upload-sessions/{upload_id}/parts` nhận bounded `part_numbers` và trả signed requests; không tạo response hàng nghìn URL một lần.

### `POST /projects/{project_id}/assets/{asset_id}/upload-sessions/{upload_id}/complete`

Request `{ "parts":[{"part_number":1,"etag":"..."}], "sha256":"..." }`. Server verify upload ownership/provider object, chuyển Upload `completed`, Asset `uploaded → validating`, tạo Artifact staged và enqueue probe Job. Response `202`:

```json
{"asset":{"id":"asset_...","status":"validating"},"validation_job_id":"job_probe_..."}
```

Complete idempotent. Checksum/MIME/probe/security failure đưa Asset tới `failed|quarantined`, không `ready`.

### Other Asset endpoints

| Method/path | Contract |
|---|---|
| `POST .../upload-sessions/{upload_id}/abort` | Abort active multipart, Asset về `pending_upload` hoặc deleted theo request; `202`. |
| `GET /projects/{id}/assets?kind=&status=&cursor=` | Metadata list; no signed URL by default. |
| `GET /projects/{id}/assets/{asset_id}` | Probe/validation metadata, original Artifact ref, variants, revision. |
| `PATCH /projects/{id}/assets/{asset_id}` | Display metadata only, `If-Match`; không đổi storage locator/checksum. |
| `POST /projects/{id}/assets/{asset_id}/revalidate` | New asset-probe Job; idempotent; `202`. Quarantine release requires security role/policy. |
| `POST /projects/{id}/assets/{asset_id}/download-url` | `{expires_in_sec}`; exact authorized original/variant; `200`. |
| `DELETE /projects/{id}/assets/{asset_id}` | Soft delete after reference/active-job check; `202`. |

## 5. Scripts and Narrations

Script aggregate and immutable versions are separate resources.

| Method/path | Contract |
|---|---|
| `POST /projects/{id}/scripts` | Create Script + version 1 from user/import; idempotent; `201`. |
| `GET /projects/{id}/scripts` | Aggregate list + current version summary. |
| `GET /projects/{id}/scripts/{script_id}` | Aggregate/current pointer. |
| `GET /projects/{id}/scripts/{script_id}/versions` | Version history. |
| `GET /projects/{id}/scripts/{script_id}/versions/{version_id}` | Exact immutable content. |
| `POST /projects/{id}/scripts/{script_id}/versions` | Create next version from `based_on_version_id`; requires `If-Match` on Script aggregate. |
| `POST .../versions/{version_id}/approve` | Audit decision, update current pointer/status; idempotent. |
| `POST .../versions/{version_id}/reject` | `{comment, action}`; action must follow pipeline/review policy. |
| `DELETE /projects/{id}/scripts/{script_id}` | Soft delete aggregate; preserves referenced versions. |

Version request:

```json
{
  "based_on_version_id":"scriptv_03",
  "language":"vi",
  "origin":"user",
  "content":{"segments":[{"id":"seg_1","text":"...","beat":"hook"}]}
}
```

Narration endpoints:

- `GET /projects/{id}/narrations?script_version_id=&status=`
- `GET /projects/{id}/narrations/{narration_id}`
- `POST /projects/{id}/narrations` creates a TTS Job from exact ScriptVersion + ProviderConfiguration/Voice descriptor; `202` with `narration_id`, `job_id`.

Client không upload provider response path; generated audio exposed through Artifact action.

## 6. Scenes and Characters

| Method/path | Contract |
|---|---|
| `GET /projects/{id}/scenes?asset_id=&status=&cursor=` | Scene list/source ranges/analysis summary. |
| `GET /projects/{id}/scenes/{scene_id}` | Exact Scene + Artifact/Character refs. |
| `PATCH /projects/{id}/scenes/{scene_id}` | User annotation/confirm/reject only, `If-Match`; source range/model ref protected. |
| `GET /projects/{id}/characters?status=&cursor=` | Character list. |
| `POST /projects/{id}/characters` | User-created identity/label where policy permits. |
| `PATCH /projects/{id}/characters/{character_id}` | Label/alias/confirm/reject, `If-Match`. |
| `POST /projects/{id}/characters/{character_id}/merge` | `{target_character_id}`; audited/idempotent, preserves lineage. |
| `GET /projects/{id}/characters/{character_id}/appearances` | Paginated Scene appearances. |

Client không tự set embedding Artifact, model revision hoặc cross-project source.

## 7. Timelines

| Method/path | Contract |
|---|---|
| `POST /projects/{id}/timelines` | Create Timeline aggregate + initial version from proposal/import/user document; `201`. |
| `GET /projects/{id}/timelines` | Aggregates/current version summaries. |
| `GET /projects/{id}/timelines/{timeline_id}` | Aggregate/current pointer. |
| `GET /projects/{id}/timelines/{timeline_id}/versions` | Version history. |
| `GET /projects/{id}/timelines/{timeline_id}/versions/{version_id}` | Exact canonical JSON document. |
| `POST /projects/{id}/timelines/{timeline_id}/commands` | Execute one typed domain command against exact `based_on_version_id`/`expected_version`; requires `If-Match`; returns a new immutable version. |
| `POST .../versions/{version_id}/validate` | Validate schema/cross-refs without changing content; `200 ValidationReport`. |
| `POST .../versions/{version_id}/approve` | Approval command; exact validated version; idempotent. |
| `POST .../versions/{version_id}/lock` | Prevent further pointer mutation/edit; admin/editor policy. |
| `POST .../versions/{version_id}/duplicate` | New version preserving IDs/origin lineage. |
| `DELETE /projects/{id}/timelines/{timeline_id}` | Soft delete aggregate if policy allows. |

Edit never merges Clips by array index. Conflict returns `412` with current ETag/version and diff metadata; user override semantics follow `06-TIMELINE-SPEC.md`.

Initial command kinds are `AddClip`, `RemoveClip`, `MoveClip`, `TrimClip`, `UpdateSubtitle`,
`ReplaceNarration`, `ChangeTrackOrder` and `UpdateScene`. Each payload has a versioned schema,
validates ownership/ranges/invariants and declares downstream invalidation. Full-document input is
accepted only by a separate privileged create/import command, is validated as untrusted input and
is not the normal editor mutation surface.

## 8. Jobs, PipelineRuns and JobSteps

### `POST /projects/{project_id}/jobs`

Idempotent request:

```json
{
  "kind":"pipeline",
  "workflow_key":"movie_recap",
  "pipeline_version":1,
  "mode":"studio",
  "input":{
    "asset_ids":["asset_..."],
    "script_version_id":null,
    "timeline_version_id":null
  },
  "start_from":"generate_script",
  "stop_after":"generate_script",
  "params":{"duration_sec":60,"language":"vi"},
  "priority":5,
  "max_runs":3,
  "enable_dlq":true,
  "auto_start":true
}
```

API resolves exact Pipeline/inputs/provider bindings and atomically creates Job + first PipelineRun + JobSteps + outbox. `auto_start=false` returns `201 status=created`; `auto_start=true` returns `202 status=created|queued` depending outbox publication timing. Client must accept both and follow status endpoint/events.

### Job resource/commands

| Method/path | Contract |
|---|---|
| `GET /projects/{id}/jobs?status=&kind=&cursor=` | Job summaries. |
| `GET /projects/{id}/jobs/{job_id}` | Canonical state/progress/current run/input snapshot/safe error/links. |
| `POST .../jobs/{job_id}/start` | `created → queued`; idempotent, `202`. |
| `POST .../jobs/{job_id}/pause` | Cooperative pause at safe checkpoint; `running → paused`; `202`. |
| `POST .../jobs/{job_id}/resume` | From `paused|waiting_for_review`, optional checkpoint/review revision; validate stale dependencies; `202`. |
| `POST .../jobs/{job_id}/cancel` | Sets `cancelling` when work active; terminal only after confirm; `202`. |
| `POST .../jobs/{job_id}/retry` | Creates new Job with `supersedes_job_id`; explicit input-version choice; `201/202`. |
| `POST .../jobs/{job_id}/reviews/{job_step_id}/approve` | `{selected_resource_type, selected_resource_id, selected_resource_revision}`; follows node review policy; `202`. |
| `POST .../jobs/{job_id}/reviews/{job_step_id}/reject` | `{reason, action}` allowed by graph; `202`. |
| `GET .../jobs/{job_id}/runs` | PipelineRun history. |
| `GET .../jobs/{job_id}/runs/{pipeline_run_id}` | Exact run status/engine/checkpoint. |
| `GET .../jobs/{job_id}/steps` | JobSteps for current/all run via filter, ordered graph summary. |
| `GET .../jobs/{job_id}/steps/{job_step_id}/attempts` | Attempt history, no secret/traceback. |
| `GET .../jobs/{job_id}/artifacts` | Artifact metadata/roles. |
| `GET .../jobs/{job_id}/events?after_sequence=&limit=` | Durable replay. |
| `GET .../jobs/{job_id}/events/stream` | SSE contract from `07`. |

Resume không rerun committed compatible nodes. Retry/replay không mutate terminal Job history.

Approve có thể chọn version user vừa tạo dựa trên proposal. Đây là ReviewResolution/JobStep output mới, không sửa immutable Job command. Response/event trả cả proposed và selected refs; downstream checkpoints/fingerprints dùng selected ref.

## 9. Renders and Artifacts

### `POST /projects/{project_id}/renders`

```json
{
  "timeline_version_id":"tlv_...",
  "render_profile":{"profile_key":"youtube_16_9","version":1},
  "preview":false,
  "overrides":{}
}
```

Validates exact TimelineVersion/Profile, creates Render `created` + Job `kind=render`, snapshots profile and returns `202 {render_id,job_id,status}`. Deliverable requires approved/locked timeline; preview draft must be explicit.

| Method/path | Contract |
|---|---|
| `GET /projects/{id}/renders?status=&timeline_version_id=&cursor=` | Render summaries. |
| `GET /projects/{id}/renders/{render_id}` | Request snapshot, Job, QA and Artifact roles. |
| `POST /projects/{id}/renders/{render_id}/cancel` | Delegates Job cancel; does not delete source. |
| `POST /projects/{id}/renders/{render_id}/retry` | New Render/Job with `supersedes_render_id`; may return an already completed exact-hash Render under documented dedupe policy. |
| `GET /projects/{id}/artifacts?kind=&role=&job_id=&cursor=` | Project Artifact list. |
| `GET /projects/{id}/artifacts/{artifact_id}` | Metadata/provenance/checksum. |
| `POST /projects/{id}/artifacts/{artifact_id}/download-url` | Short-lived signed URL after ACL. |

Một TimelineVersion có thể tạo 16:9, 9:16 và 1:1 Render mà không tạo lại research/script/scenes/matches.

## 10. Workflow, Pipeline, profiles and providers

Read catalog:

- `GET /workflows?status=`
- `GET /workflows/{workflow_id}/pipelines?status=`
- `GET /pipelines/{pipeline_id}` — definition/contract, no secret.
- `GET /render-profiles?profile_key=&status=`
- `GET /providers/catalog?kind=` — descriptors/models/capabilities only.

ProviderConfiguration product API (admin/owner scope, backed by server-side SecretStore):

| Method/path | Contract |
|---|---|
| `GET /provider-configurations?kind=&status=` | Redacted metadata. |
| `POST /provider-configurations` | Create config with secret-manager credential operation/reference; plaintext never returned/persisted. |
| `GET /provider-configurations/{id}` | Redacted config/revision/last validation. |
| `PATCH /provider-configurations/{id}` | `If-Match`, allowlisted non-secret fields or new credential reference. |
| `POST /provider-configurations/{id}/validate` | Async/sync safe connectivity/capability validation; no credential echo. |
| `POST /provider-configurations/{id}/disable` | Prevent new Jobs; existing snapshot policy explicit. |
| `DELETE /provider-configurations/{id}` | Soft delete only if references/policy allow. |

Pipeline authoring mutation không public trong baseline; only built-in versioned definitions are
activated initially. Later admin declarative authoring is deferred nonblocking.

### Analyses and candidates

- `POST /projects/{id}/analyses` creates a typed analysis Job from exact Asset/Artifact refs.
- `GET /projects/{id}/analyses?kind=&status=` lists Project-scoped results and provenance.
- `GET /projects/{id}/analyses/{analysis_id}` returns typed result metadata and Artifact refs.
- `POST /projects/{id}/candidate-groups` requests multiple GenerationCandidates when the selected
  Workflow/Policy supports it; this endpoint is later-phase and must not be simulated with one
  overwritten result.
- `GET /projects/{id}/candidate-groups/{group_id}` returns candidates, EvaluationResults and the
  audited selection. Selection never mutates candidate payloads.

`ReferenceStyleAnalysis` accepts a user-authorized reference Asset and returns abstract metrics. It
does not expose copied footage or an upstream workflow name.

## 11. Event transport and health

- SSE canonical endpoint là nested Job stream ở mục 8.
- State-changing commands luôn REST; clients cannot emit domain events over the progress stream.
- Snapshot is REST state, event stream is ordered change feed. Disconnect không đổi Job state.
- `GET /live` reports minimal process liveness and does not expose dependency/tenant details.
- `GET /ready` reports whether required enabled dependencies are reachable: PostgreSQL, Redis Streams
  and object storage. Detailed diagnostics are protected.
- WebSocket is not part of the initial contract. A future bidirectional requirement must justify a
  separate versioned transport decision; progress alone is not sufficient justification.

## 12. Independence and API contract tests

- OpenAPI schema snapshots and request/response examples;
- Project/Asset/ScriptVersion/TimelineVersion CRUD + ETag conflict;
- LocalAuth login/rotation/revocation, default Workspace bootstrap and cross-Workspace denial;
- Timeline domain command positive/negative/version/invalidation corpus;
- direct multipart upload proves no media body through API;
- all Job commands across valid/invalid canonical states;
- REST status equals DB/event state after transitions;
- idempotency same/different hash and concurrent request;
- cursor stability;
- cross-workspace 404/403 policy and signed URL ACL;
- secret/path/traceback redaction;
- analysis/candidate authorization, versioning and provenance;
- dependency/package/route scan proving no Movie Narrator CLI, REST, import or compatibility route;
- web and desktop clients complete the same native Product API flows.
