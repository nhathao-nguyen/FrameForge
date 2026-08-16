# 02 — Domain Model

## 1. Phạm vi và quy ước

Tài liệu này định nghĩa ngôn ngữ domain canonical của V2. Tên trạng thái trong tài liệu này phải được dùng nguyên văn ở PostgreSQL, REST và event payload. Mapping V1 chỉ tồn tại trong compatibility adapter.

- ID là UUID/opaque string; filename, storage key và array index không phải identity.
- Timestamp là UTC `timestamptz`/ISO-8601.
- Thời gian media là số giây không âm, precision tối thiểu 1 ms.
- Aggregate mutable có `revision`; update dùng optimistic concurrency.
- Version content đã commit là immutable. Thay đổi nội dung tạo version mới.
- Artifact đã `committed` là immutable; thay blob tạo Artifact mới.
- Product ownership đi theo `Workspace → Project`; engine chỉ nhận opaque project/reference context, không nhận user, role, billing hoặc subscription.

## 2. Entity catalog

Mỗi hàng dưới đây là một contract bắt buộc. “Persist” chỉ nơi lưu metadata; binary media không được lưu trong PostgreSQL.

| Entity | Responsibility | Identifier và ownership | Lifecycle | Relations | Mutable / immutable | Persistence |
|---|---|---|---|---|---|---|
| **Project** | Aggregate sản xuất video: gom input, content, timeline, jobs và outputs. | `project_id`; thuộc một Workspace, actor được authorize quản lý. | `active → archived → deleted` (soft delete); có thể restore theo policy. | Có nhiều Asset, Script, Scene, Character, Timeline, Job, Render, Artifact. | `name`, settings và current-version pointers mutable qua `revision`; ID/owner immutable. | Bảng `projects`; bắt buộc. |
| **Asset** | Logical media do product quản lý, ví dụ source movie, BGM, image, font. Không đồng nghĩa với blob. | `asset_id`; thuộc đúng một Project. | `pending_upload → uploading → uploaded → validating → ready`; lỗi thành `failed` hoặc `quarantined`; cuối cùng `deleted`. | Trỏ `original_artifact_id`; có nhiều Artifact variant và Scene. | Metadata mô tả/probe, status và current original pointer mutable; một original blob cụ thể immutable qua Artifact. | Bảng `assets`, `asset_uploads`, `artifact_variants`; bắt buộc. |
| **Artifact** | Manifest của một blob immutable do upload hoặc node tạo ra. | `artifact_id`; thuộc Project, optional Asset/Job/JobStep/Render producer. | `staged → committed → expired → deleted`; `quarantined` nếu output bị nghi ngờ. | Có storage locator, checksum, semantic kind/role; được domain objects tham chiếu bằng ID. | Sau `committed`, bytes, checksum, storage locator và producer immutable; retention/status có thể đổi. | Bảng `artifacts`; bytes ở StorageBackend. |
| **Workflow** | Product-level recipe/capability, ví dụ `movie_recap`; movie recap chỉ là workflow đầu tiên. | `workflow_id`, stable `workflow_key`; system-owned hoặc workspace-owned nếu sau này cho custom workflow. | `draft → active → deprecated → disabled`. | Có nhiều Pipeline version. | Name/description mutable khi draft; key immutable sau publish. | Bảng `workflows`; bắt buộc. |
| **Pipeline** | Một version executable của Workflow: DAG, schemas và policies. | `pipeline_id`; thuộc Workflow; `(workflow_id, version)` unique. | `draft → active → deprecated → disabled`. | Có PipelineNode và dependency edges; được PipelineRun snapshot. | Definition mutable khi `draft`; immutable sau `active`; thay đổi tạo Pipeline version mới. | Bảng `pipelines`; bắt buộc. |
| **PipelineNode** | Definition của một node logic trong Pipeline. Không phải process và không phải execution record. | `pipeline_node_id`, stable `node_key` trong Pipeline. | Cùng lifecycle Pipeline; không có runtime state. | Thuộc Pipeline; phụ thuộc node khác; được JobStep instantiate. | Contract/config immutable khi Pipeline active. | `pipeline_nodes`, `pipeline_node_dependencies`; bắt buộc. |
| **Job** | Product command do user/system yêu cầu: chạy workflow, render, probe hoặc migration. | `job_id`; thuộc Project; `requested_by` actor. | State machine ở mục 4. | Snapshot Workflow/Pipeline/input; có một hoặc nhiều PipelineRun, events và Artifact outputs. | Command/input snapshot immutable sau create; status/progress/error mutable bằng guarded transitions. | Bảng `jobs`; bắt buộc. |
| **PipelineRun** | Một engine execution của exact Pipeline snapshot cho một Job. Tách product intent khỏi engine attempt. | `pipeline_run_id`; thuộc một Job và Pipeline. | `created → queued → running`; có thể `paused`/`waiting_for_review`; terminal `completed|failed|cancelled`. | Có ordered/DAG JobSteps và checkpoints. Tối đa một active run cho mỗi Job. | Snapshot immutable; status/timestamps mutable. Resume tiếp tục cùng run; replay tạo run mới theo policy. | Bảng `pipeline_runs`; bắt buộc. |
| **JobStep** | Runtime instance của một PipelineNode trong một PipelineRun; unit của lease, retry, checkpoint và progress. | `job_step_id`; thuộc PipelineRun; unique `(pipeline_run_id, node_key)`. | Node execution state machine ở mục 5. | Tham chiếu PipelineNode, attempts, checkpoint, provider runs và output Artifacts. | Input fingerprint/config snapshot immutable; state/progress/current attempt mutable; attempt history append-only. | `job_steps`, `job_step_attempts`; bắt buộc. |
| **ReviewRequest/Resolution** | Persisted human gate cho một JobStep; ghi proposal, allowed actions và selected replacement version. | `review_id`; thuộc JobStep/Project. | `open → approved|rejected|cancelled|expired`. | Trỏ proposed resource và optional selected ScriptVersion/TimelineVersion; actor resolution. | Request immutable; resolution append/guarded một lần. | Bảng `reviews`; bắt buộc cho studio gate. |
| **Script** | Aggregate narration/content của Project, giữ current version và lifecycle. | `script_id`; thuộc Project. | `active → archived → deleted`. | Có nhiều ScriptVersion; Project có current ScriptVersion pointer. | Label/current pointer/status mutable; ID/project immutable. | Bảng `scripts`; bắt buộc. |
| **ScriptVersion** | Exact immutable script document dùng cho review, TTS và subtitle. | `script_version_id`; `(script_id, version)` unique. | `draft|proposed → approved → superseded`; rejection ghi review decision, không sửa content. | Thuộc Script; dựa trên optional prior version; producer JobStep; Narration tham chiếu exact version. | Content/hash/language immutable sau create; approval metadata/status chỉ chuyển tiến. Edit tạo version mới. | `script_versions`, `script_reviews`; bắt buộc. |
| **Scene** | Đoạn source media được detect/index với semantic metadata. Không phải output clip. | `scene_id`; thuộc Project và source Asset revision. | `detected → analyzed`; có thể `confirmed|rejected`; supersede khi source/detection revision đổi. | Thuộc Asset; có Character appearances; Timeline Clip có thể source Scene. | Source range/model revision immutable; user annotations và confirmation mutable qua `revision`. | Bảng `scenes`; bắt buộc. |
| **Character** | Identity/cluster nhân vật trong Project. | `character_id`; thuộc Project. | `unconfirmed → confirmed|rejected`; có thể `merged` vào Character khác. | Có appearances trong Scenes, representative Artifact và aliases. | AI confidence/cluster metadata có version; user name, alias, confirmation mutable/audited. | `characters`, `character_appearances`; bắt buộc. |
| **Timeline** | Aggregate EDL của Project, giữ current version. Renderer không đọc match ngoài TimelineVersion. | `timeline_id`; thuộc Project. | `active → archived → deleted`. | Có nhiều immutable TimelineVersion; Render tham chiếu exact version. | Label/current pointer/status mutable. | Bảng `timelines`; bắt buộc. |
| **TimelineVersion** | Canonical validated Timeline JSON tại một revision/version. | `timeline_version_id`; `(timeline_id, version)` unique. | `draft|proposed → approved → locked|superseded`. | Chứa Track/Clip/SubtitleTrack value objects; based-on prior version; producer JobStep. | Document/hash/schema version immutable; edit tạo version mới. Approval/lock là guarded metadata transition. | `timeline_versions.document jsonb`; bắt buộc. |
| **Track** | Ordered layer trong TimelineVersion: video, narration, music, SFX, subtitle hoặc overlay. | `track_id` stable trong lineage Timeline, không phải global entity ID. | Sống/chết cùng TimelineVersion. | Chứa Clips; `SubtitleTrack` là specialization bằng `kind=subtitle`. | Immutable trong version; edit tạo TimelineVersion mới. | Embedded trong validated Timeline JSON; không có canonical table riêng. |
| **Clip** | Placement trên output timeline với source range, transform, audio và transitions. | `clip_id` stable trong lineage Timeline, không phải Scene ID. | Sống/chết cùng TimelineVersion. | Thuộc Track; source Asset, Artifact, Scene hoặc generated content. | Immutable trong version; user override tạo version mới và giữ `origin=user`. | Embedded trong Timeline JSON; không có canonical table riêng. |
| **SubtitleTrack** | Track typed chứa subtitle cues/style/language; không đồng nghĩa file SRT/VTT. | Dùng `track_id`, `kind=subtitle`. | Sống/chết cùng TimelineVersion. | Cue là subtitle Clip; export SRT/VTT là Artifact. | Immutable trong version. | Embedded JSON; subtitle export bytes ở Artifact. |
| **Voice** | Provider-neutral voice descriptor/catalog entry dùng để chọn giọng. | Composite `(provider_configuration_id, provider_voice_id, catalog_version)` hoặc built-in key. | `available|deprecated|unavailable`; catalog có thể refresh. | Narration snapshot exact voice descriptor. | Catalog metadata mutable; snapshot trong Narration immutable. | Có thể cache/catalog table; lựa chọn bắt buộc được snapshot trong `narrations.voice_snapshot`. |
| **Narration** | Exact synthesized/recorded voice-over cho một ScriptVersion. | `narration_id`; thuộc Project và ScriptVersion. | `created → synthesizing → ready|failed → superseded`. | Trỏ audio Artifact, timing Artifact, ProviderConfiguration và voice snapshot. | Request snapshot immutable; status/output refs mutable cho tới terminal; re-synthesis tạo Narration mới. | Bảng `narrations`; bắt buộc. |
| **RenderProfile** | Versioned target constraints: aspect ratio, codec, resolution, safe area, subtitle/audio policy. | `render_profile_id`; system hoặc Workspace-owned; `(profile_key, version, owner_scope)` unique. | `draft → active → deprecated → disabled`. | Được Render snapshot; không nằm trong canonical Timeline. | Mutable khi draft; immutable sau active. | Bảng `render_profiles`; bắt buộc. |
| **Render** | Product request/result compile exact TimelineVersion bằng exact RenderProfile. | `render_id`; thuộc Project; có `job_id`. | `created → queued → running → completed|failed|cancelled`. | Tham chiếu TimelineVersion, profile snapshot, Job, optional prior Render lineage và output Artifacts. | Request snapshot immutable; status/QA/output relation mutable tới terminal. Rerender/retry tạo Render mới với `supersedes_render_id`. | `renders`, `render_artifacts`; bắt buộc. |
| **ProviderConfiguration** | Product metadata/policy để resolve một provider adapter và credential reference; không chứa plaintext secret. | `provider_configuration_id`; system- hoặc Workspace-owned. | `draft → active → disabled|invalid → deleted`. | Được Job/JobStep/Narration snapshot; adapter registry resolve theo kind/name. | Kind/adapter key/owner immutable sau use; model defaults/policy/credential ref mutable với revision và audit. | `provider_configurations`; secret ở secret manager. |

## 3. Chuẩn hóa các concept dễ trùng

### Asset và Artifact

- Asset là logical input mà user nhìn thấy và có lifecycle upload/validation.
- Artifact là một immutable blob manifest.
- Original upload là Artifact role `original` được Asset trỏ tới. Proxy, thumbnail, waveform và extracted audio là Artifact variants của cùng Asset.
- Asset không dùng `object_key` làm identity; Artifact không thay Project ownership của Asset.

### PipelineNode, JobStep và PipelineRun

- PipelineNode là definition bất biến trong một Pipeline version.
- JobStep là runtime instance của PipelineNode trong một PipelineRun.
- PipelineRun là toàn bộ execution graph; Job là product command/aggregate bên ngoài.
- Thuật ngữ `stage` chỉ được dùng khi mô tả V1. V2 dùng `node` (definition) và `job step` (execution).

### Render và Artifact

Render là request/execution metadata. File MP4, audio, thumbnail và QA report do Render tạo là Artifacts. Render `completed` chỉ sau khi required Artifacts đã commit và QA policy pass.

### Scene match và TimelineClip

Scene match là proposal/score output của node `match_clips`, lưu như Artifact/domain proposal. TimelineClip là quyết định edit canonical. Renderer không đọc `matches.json` hoặc rerun matching để thay TimelineClip.

### Voice, Narration và audio Artifact

Voice là descriptor lựa chọn; Narration là synthesis instance; audio bytes là Artifact. TTS provider không trực tiếp tạo Render và không sửa ScriptVersion.

## 4. Job state machine canonical

Canonical states:

```text
created → queued → running ───────────────→ completed
   │         │        │
   │         │        ├→ paused ─resume──→ queued
   │         │        ├→ waiting_for_review ─approve/resume→ queued
   │         │        ├→ retrying ─backoff/new run→ queued
   │         │        ├→ cancelling ──────────────→ cancelled
   │         │        └→ failed
   │         └────────────cancel──────────────────→ cancelled
   └──────────────────────cancel──────────────────→ cancelled

retrying ─exhausted, DLQ on→ dead_lettered
retrying ─exhausted, DLQ off→ failed
waiting_for_review ─reject policy→ failed|cancelled
```

| State | Meaning | Allowed next states |
|---|---|---|
| `created` | Command và snapshots đã commit; outbox chưa xác nhận queue-ready. | `queued`, `cancelled` |
| `queued` | Có work item sẵn sàng, chưa có active execution lease. | `running`, `cancelling`, `cancelled` |
| `running` | PipelineRun/JobStep đang có lease hoặc đang điều phối graph. | `paused`, `waiting_for_review`, `retrying`, `cancelling`, `completed`, `failed` |
| `paused` | Đã dừng tại safe checkpoint theo lệnh/`stop_after`. | `queued`, `cancelling`, `cancelled` |
| `waiting_for_review` | Persisted human gate; không phải failure. | `queued`, `cancelling`, `failed`, `cancelled` |
| `retrying` | Đang chờ backoff/recovery; chưa terminal. | `queued`, `cancelling`, `failed`, `dead_lettered`, `cancelled` |
| `cancelling` | Cancel đã được chấp nhận; chờ executor xác nhận safe stop/kill. | `cancelled`, `failed` nếu cleanup integrity fail |
| `completed` | Required graph outputs đã commit. | none |
| `failed` | Non-retryable hoặc retry budget kết thúc khi DLQ off. | none; explicit retry tạo Job mới |
| `dead_lettered` | Retry budget cạn và record DLQ đã commit. | none; replay tạo Job mới |
| `cancelled` | Cancel terminal đã được xác nhận. | none |

Rules:

1. Invalid request bị trả `4xx` trước khi tạo Job; không tạo Job `failed` chỉ để biểu diễn validation HTTP.
2. Terminal state không chuyển ngược. `retry`/DLQ replay tạo Job ID mới với `supersedes_job_id`.
3. State transition, `job_events` và outbox write cùng transaction.
4. Job status là product-facing aggregate. PipelineRun status phải khớp invariant ở mục 6; API không tự phát minh status `processing` hoặc `executing`.

## 5. Pipeline Node execution state machine

PipelineNode definition không có runtime state. State dưới đây thuộc JobStep, tức execution của node:

```text
pending ─dependencies satisfied→ ready ─enqueue→ queued ─claim→ running
   │                                  │                 │
   └─dependency terminal failure────→ blocked          ├→ completed
                                                        ├→ skipped
                                                        ├→ paused ─resume→ queued
                                                        ├→ waiting_for_review ─approve→ completed
                                                        ├→ retrying ─backoff→ queued
                                                        ├→ failed
                                                        └→ cancelled
```

| State | Meaning | Terminal? |
|---|---|---:|
| `pending` | Chưa đủ dependency hoặc nằm ngoài execution frontier. | no |
| `ready` | Dependency/input đã hợp lệ, chưa publish queue message. | no |
| `queued` | Work item đã publish, chờ claim. | no |
| `running` | Active lease; attempt đang execute. | no |
| `paused` | Node dừng tại safe checkpoint, có thể resume. | no |
| `waiting_for_review` | Proposal/checkpoint đã commit, chờ review command. | no |
| `retrying` | Attempt lỗi retryable, chờ backoff/attempt mới. | no |
| `completed` | Required outputs đã commit và validate. | yes |
| `skipped` | Disabled, ngoài partial boundary hoặc soft degradation có reason/consequence. | yes |
| `failed` | Non-retryable hoặc node attempt budget cạn. | yes |
| `cancelled` | Bị cancel trước/khi execute. | yes |
| `blocked` | Không thể chạy vì required dependency terminal fail/cancel. | yes |

Retry tạo `job_step_attempts.attempt = n + 1` nhưng giữ cùng JobStep. `timeout` được phân loại retryable theo node policy. Hai worker không được cùng commit output cho `(job_step_id, attempt, input_fingerprint)`.

Manual approval có hai dạng được khai báo trong PipelineNode contract:

- `approval_completes_node`: approval chuyển `waiting_for_review → completed` và unlock downstream;
- `approval_resumes_node`: approval chuyển `waiting_for_review → queued` để node hoàn tất phần sau review.

Reject outcome phải là `failed`, `cancelled` hoặc một explicit correction branch trong Pipeline definition; không có transition ngầm.

Khi user edit proposal trong lúc review, Job command không bị mutate. ReviewResolution chọn exact replacement ScriptVersion/TimelineVersion; review JobStep output ref trở thành version đã chọn, checkpoint mới ghi ref đó và chỉ downstream fingerprints bị invalidate.

## 6. Job và PipelineRun consistency

| PipelineRun condition | Job state bắt buộc |
|---|---|
| Run đầu đã tạo, outbox chưa queue | `created` |
| Active run `queued` | `queued` |
| Active run `running` | `running` hoặc `retrying` khi một JobStep backoff |
| Active run `paused` | `paused` |
| Active run `waiting_for_review` | `waiting_for_review` |
| Active run `completed` | `completed` |
| Run failed còn retry budget | `retrying`; run mới chỉ tạo sau backoff |
| Run failed hết budget | `failed` hoặc `dead_lettered` |
| Cancel đang propagate | Job `cancelling`; active run chưa terminal |
| Active run `cancelled` | Job `cancelled` |

Không được có hai PipelineRun active (`queued|running|paused|waiting_for_review`) cho cùng Job. Resume dùng cùng PipelineRun; explicit retry/replay mặc định tạo Job mới, do đó cũng tạo PipelineRun mới dưới Job mới.

## 7. Cross-object invariants

1. Asset chỉ được dùng làm production input khi `ready`; `quarantined` fail closed.
2. Job snapshot exact Pipeline, ScriptVersion, TimelineVersion, Asset original Artifact và ProviderConfiguration revisions trước queue.
3. JobStep chỉ đọc declared inputs và chỉ publish declared output roles.
4. Artifact output chỉ visible sau checksum, metadata và state transaction thành công.
5. Scene range phải nằm trong duration của exact source Artifact revision.
6. Character appearance và Scene phải cùng Project; merge Character giữ alias/audit lineage.
7. TimelineVersion Clip source phải resolve cùng Project hoặc shared system artifact đã authorize.
8. Approved ScriptVersion/TimelineVersion không bị sửa tại chỗ.
9. Render đọc exact TimelineVersion và profile snapshot; không gọi scene matching ngoài graph đã khai báo.
10. Nhiều Render profile reuse Script/Narration/Scene/Timeline artifacts khi input fingerprint không đổi.
11. Worker-local path chỉ là lease-scoped cache handle; không xuất hiện trong domain/API/event/checkpoint.
12. AI output có `origin`, provenance, model/provider snapshot và confidence khi có; user override không bị overwrite nếu không có explicit replace command.
