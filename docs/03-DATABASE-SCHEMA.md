# 03 — PostgreSQL Schema Specification

## 1. Database rules

- PostgreSQL là source of truth cho product metadata, current state, audit sequence và idempotency. Redis không thay PostgreSQL.
- PostgreSQL không lưu binary media, embedding vector lớn hoặc checkpoint payload lớn; chỉ lưu metadata và Artifact ID.
- Primary key dùng `uuid`; tên dùng `snake_case`; timestamp dùng UTC `timestamptz`.
- Bảng mutable có `created_at`, `updated_at`; aggregate mutable có `revision bigint NOT NULL DEFAULT 1`.
- Soft delete chỉ dùng cho resource product cần restore/audit: Project, Asset, Script, Timeline, ProviderConfiguration; field `deleted_at` nullable.
- Status dùng `text` + named `CHECK` constraint theo canonical states trong `02-DOMAIN-MODEL.md`.
  Không dùng synonym `processing`, `executing`, `success`, `succeeded`, `waiting_review` hoặc `dead`.
- JSONB chỉ dùng cho versioned document, validated snapshot, provider-specific metadata hoặc fields không cần join/filter thường xuyên.
- FK audit/history dùng `ON DELETE RESTRICT`; child lifecycle thuần dùng `CASCADE`. Migration đã apply không được sửa.
- Mọi unique constraint trên soft-deleted resource cần partial index `WHERE deleted_at IS NULL` khi phù hợp.

## 2. Identity and ownership

### `users`

`id uuid PK`, `external_subject text NOT NULL UNIQUE`, `email citext`, `display_name text`, `status text NOT NULL CHECK (status IN ('active','suspended','deleted'))`, `created_at`, `updated_at`.

Indexes: unique `(email) WHERE email IS NOT NULL`, `(status)`. Authentication provider còn mở tại OQ-01; bảng này là local authorization identity, không lưu password nếu dùng OIDC.

### `workspaces`

`id uuid PK`, `slug text NOT NULL`, `name text NOT NULL`, `status text NOT NULL CHECK (status IN ('active','suspended','deleted'))`, `settings jsonb NOT NULL DEFAULT '{}'`, `revision bigint NOT NULL DEFAULT 1`, `created_by uuid FK users`, `created_at`, `updated_at`, `deleted_at`.

Indexes: unique `(slug) WHERE deleted_at IS NULL`, `(status)`. Workspace-first hay single-user bootstrap phải được chốt tại OQ-06; schema không được bỏ ownership scope khỏi repository contract.

### `workspace_members`

`workspace_id uuid FK workspaces`, `user_id uuid FK users`, `role text CHECK (role IN ('owner','admin','editor','viewer'))`, `created_at`, `updated_at`; PK `(workspace_id,user_id)`. Index `(user_id,workspace_id)`.

### `api_keys`

`id uuid PK`, `workspace_id uuid FK workspaces`, `created_by uuid FK users`, `name text`, `key_prefix text`, `secret_hash text`, `scopes jsonb`, `last_used_at`, `expires_at`, `revoked_at`, `created_at`, `updated_at`.

Constraints/indexes: unique `(workspace_id,key_prefix)`, `(workspace_id,revoked_at,expires_at)`. Chỉ hash được persist.

## 3. Workflow and provider catalog

### `workflows`

`id uuid PK`, `workspace_id uuid NULL FK workspaces` (`NULL` = system built-in), `workflow_key text NOT NULL`, `name text NOT NULL`, `description text`, `status text CHECK (status IN ('draft','active','deprecated','disabled'))`, `revision bigint`, `created_by uuid NULL FK users`, `created_at`, `updated_at`.

Indexes: unique `(workflow_key) WHERE workspace_id IS NULL`, unique `(workspace_id,workflow_key) WHERE workspace_id IS NOT NULL`, `(status,workflow_key)`.

### `pipelines`

`id uuid PK`, `workflow_id uuid FK workflows`, `version integer NOT NULL CHECK (version > 0)`, `status text CHECK (status IN ('draft','active','deprecated','disabled'))`, `schema_version text NOT NULL`, `definition jsonb NOT NULL`, `content_hash char(64) NOT NULL`, `created_by uuid NULL FK users`, `activated_at`, `created_at`, `updated_at`.

Constraints/indexes: unique `(workflow_id,version)`, unique `(workflow_id,content_hash)`, `(workflow_id,status)`. Khi `active`, `definition` và node rows là immutable; thay đổi tạo version mới.

### `pipeline_nodes`

`id uuid PK`, `pipeline_id uuid FK pipelines ON DELETE CASCADE`, `node_key text NOT NULL`, `node_type text NOT NULL`, `display_name text NOT NULL`, `execution_class text CHECK (execution_class IN ('system','probe','ai','ml','media','render'))`, `is_optional boolean NOT NULL`, `failure_mode text CHECK (failure_mode IN ('hard','soft'))`, `review_policy text CHECK (review_policy IN ('none','approval_completes_node','approval_resumes_node'))`, `input_schema jsonb`, `output_schema jsonb`, `required_artifact_roles jsonb`, `produced_artifact_roles jsonb`, `config jsonb`, `timeout_sec integer CHECK (timeout_sec > 0)`, `max_attempts integer CHECK (max_attempts > 0)`, `retry_policy jsonb`, `checkpoint_policy jsonb`, `resource_requirements jsonb`, `idempotency_policy jsonb`, `progress_weight numeric(8,4) CHECK (progress_weight >= 0)`, `created_at`, `updated_at`.

Indexes: unique `(pipeline_id,node_key)`, `(pipeline_id,execution_class)`.

### `pipeline_node_dependencies`

`pipeline_id uuid FK pipelines`, `node_id uuid FK pipeline_nodes`, `depends_on_node_id uuid FK pipeline_nodes`, `condition jsonb NOT NULL DEFAULT '{}'`, `required boolean NOT NULL DEFAULT true`, `created_at`; PK `(node_id,depends_on_node_id)`, index `(depends_on_node_id)`. Validator phải bảo đảm cùng Pipeline, không self-loop và graph acyclic.

### `provider_configurations`

`id uuid PK`, `workspace_id uuid NULL FK workspaces` (`NULL` = system/local config), `kind text CHECK (kind IN ('llm','vlm','tts','asr','embedding'))`, `adapter_key text NOT NULL`, `display_name text NOT NULL`, `status text CHECK (status IN ('draft','active','disabled','invalid','deleted'))`, `credential_ref text NULL`, `endpoint_policy jsonb NOT NULL DEFAULT '{}'`, `model_defaults jsonb NOT NULL DEFAULT '{}'`, `capability_overrides jsonb NOT NULL DEFAULT '{}'`, `config jsonb NOT NULL DEFAULT '{}'`, `revision bigint`, `last_validated_at`, `created_by uuid NULL FK users`, `updated_by uuid NULL FK users`, `created_at`, `updated_at`, `deleted_at`.

Indexes: unique `(kind,display_name) WHERE workspace_id IS NULL AND deleted_at IS NULL`; unique `(workspace_id,kind,display_name) WHERE workspace_id IS NOT NULL AND deleted_at IS NULL`; `(workspace_id,kind,status)`, `(adapter_key,kind)`. `credential_ref` là opaque secret-manager reference, không phải plaintext.

### `render_profiles`

`id uuid PK`, `workspace_id uuid NULL FK workspaces`, `profile_key text NOT NULL`, `version integer NOT NULL CHECK (version > 0)`, `status text CHECK (status IN ('draft','active','deprecated','disabled'))`, `schema_version text NOT NULL`, `document jsonb NOT NULL`, `content_hash char(64) NOT NULL`, `created_by uuid NULL FK users`, `created_at`, `updated_at`.

Indexes: unique `(profile_key,version) WHERE workspace_id IS NULL`; unique `(workspace_id,profile_key,version) WHERE workspace_id IS NOT NULL`; unique `(profile_key,content_hash) WHERE workspace_id IS NULL`; unique `(workspace_id,profile_key,content_hash) WHERE workspace_id IS NOT NULL`; `(profile_key,status)`. Active profile document immutable.

## 4. Project, Asset and Artifact

### `projects`

`id uuid PK`, `workspace_id uuid FK workspaces`, `name text NOT NULL`, `workflow_id uuid FK workflows`, `status text CHECK (status IN ('active','archived','deleted'))`, `settings jsonb NOT NULL DEFAULT '{}'`, `current_script_version_id uuid NULL`, `current_timeline_version_id uuid NULL`, `revision bigint`, `created_by uuid FK users`, `updated_by uuid FK users`, `created_at`, `updated_at`, `deleted_at`.

Indexes: `(workspace_id,updated_at DESC)`, `(workspace_id,status)`, `(workspace_id) WHERE deleted_at IS NULL`. FK current version được thêm sau khi version tables tồn tại và phải kiểm tra cùng Project.

### `assets`

`id uuid PK`, `project_id uuid FK projects`, `kind text CHECK (kind IN ('video','audio','image','font','music','overlay','other'))`, `status text CHECK (status IN ('pending_upload','uploading','uploaded','validating','ready','failed','quarantined','deleted'))`, `original_artifact_id uuid NULL`, `original_filename text`, `declared_mime_type text`, `detected_mime_type text`, `size_bytes bigint CHECK (size_bytes >= 0)`, `sha256 char(64)`, `duration_sec numeric(14,3)`, `width integer`, `height integer`, `frame_rate_num integer`, `frame_rate_den integer`, `validation_report jsonb NOT NULL DEFAULT '{}'`, `metadata jsonb NOT NULL DEFAULT '{}'`, `revision bigint`, `created_by uuid FK users`, `updated_by uuid FK users`, `created_at`, `updated_at`, `deleted_at`.

Indexes: `(project_id,kind,status)`, `(project_id,created_at DESC)`, `(sha256)`, `(status,updated_at)`. Positive media dimensions/rate/duration có `CHECK` khi non-null.

### `asset_uploads`

`id uuid PK`, `asset_id uuid FK assets`, `storage_backend text CHECK (storage_backend IN ('local','s3','minio'))`, `staging_object_key text NOT NULL`, `provider_upload_id text`, `status text CHECK (status IN ('active','completed','aborted','expired'))`, `multipart boolean`, `part_size bigint`, `expected_size bigint`, `completed_size bigint`, `expected_sha256 char(64)`, `expires_at timestamptz`, `created_by uuid FK users`, `created_at`, `updated_at`.

Indexes: unique `(asset_id) WHERE status='active'`, unique `(storage_backend,staging_object_key)`, `(status,expires_at)`. Abandoned upload sweeper chỉ xóa staging object sau expiry/reconciliation.

### `artifacts`

`id uuid PK`, `project_id uuid FK projects`, `producer_job_id uuid NULL FK jobs`, `producer_job_step_id uuid NULL FK job_steps`, `kind text NOT NULL`, `role text NOT NULL`, `status text CHECK (status IN ('staged','committed','quarantined','expired','deleted'))`, `storage_backend text CHECK (storage_backend IN ('local','s3','minio'))`, `object_key text NOT NULL`, `object_version text`, `content_type text`, `size_bytes bigint CHECK (size_bytes >= 0)`, `sha256 char(64) NOT NULL`, `schema_version text`, `metadata jsonb NOT NULL DEFAULT '{}'`, `retention_until timestamptz`, `created_by_user_id uuid NULL FK users`, `created_by_service text NULL`, `committed_at`, `created_at`, `updated_at`.

Indexes: unique `(storage_backend,object_key,object_version) NULLS NOT DISTINCT` trên PostgreSQL 15+ (hoặc hai partial unique indexes cho version null/non-null), `(project_id,kind,created_at DESC)`, `(producer_job_id)`, `(producer_job_step_id)`, `(sha256)`, `(status,retention_until)`. Check: đúng một hoặc không actor producer, nhưng provenance Job/Step bắt buộc cho generated outputs.

### `artifact_variants`

`asset_id uuid FK assets`, `artifact_id uuid FK artifacts`, `variant_kind text CHECK (variant_kind IN ('original','proxy','thumbnail','waveform','audio_extract','normalized','transcript','other'))`, `is_canonical boolean`, `metadata jsonb`, `created_at`; PK `(asset_id,artifact_id,variant_kind)`, index `(asset_id,variant_kind)`, unique `(asset_id,variant_kind) WHERE is_canonical`.

Sau khi Artifact original commit, transaction tạo relation `variant_kind='original'`, set `assets.original_artifact_id`, verify cùng Project và chuyển Asset sang `uploaded`/`validating`. `assets.original_artifact_id` FK tới `artifacts.id` được thêm sau table creation.

## 5. Jobs and execution

### `jobs`

`id uuid PK`, `project_id uuid FK projects`, `workflow_id uuid FK workflows`, `pipeline_id uuid FK pipelines`, `kind text CHECK (kind IN ('pipeline','render','asset_probe','analysis','export'))`, `mode text CHECK (mode IN ('automatic','studio','preview'))`, `status text CHECK (status IN ('created','queued','running','paused','waiting_for_review','retrying','cancelling','completed','failed','dead_lettered','cancelled'))`, `requested_by uuid NULL FK users`, `supersedes_job_id uuid NULL FK jobs`, `command jsonb NOT NULL`, `input_snapshot jsonb NOT NULL`, `pipeline_snapshot jsonb NOT NULL`, `provider_bindings_snapshot jsonb NOT NULL DEFAULT '{}'`, `start_from text`, `stop_after text`, `priority smallint CHECK (priority BETWEEN 0 AND 20)`, `max_runs integer CHECK (max_runs > 0)`, `current_run_number integer NOT NULL DEFAULT 0`, `current_pipeline_run_id uuid NULL`, `progress_percent numeric(5,2) CHECK (progress_percent BETWEEN 0 AND 100)`, `current_node_key text`, `cancel_requested_at`, `error_code`, `error_message`, `correlation_id`, `created_at`, `queued_at`, `started_at`, `completed_at`, `updated_at`.

Indexes: `(project_id,created_at DESC)`, `(status,priority DESC,created_at)`, `(correlation_id)`, `(supersedes_job_id)`. Command/snapshots immutable after insert. FK `current_pipeline_run_id → pipeline_runs.id` được thêm sau `pipeline_runs`, với guard run thuộc cùng Job.

### `pipeline_runs`

`id uuid PK`, `job_id uuid FK jobs ON DELETE CASCADE`, `pipeline_id uuid FK pipelines`, `run_number integer CHECK (run_number > 0)`, `status text CHECK (status IN ('created','queued','running','paused','waiting_for_review','completed','failed','cancelled'))`, `executor_kind text CHECK (executor_kind IN ('go_media','python_ml','system'))`, `executor_version text NOT NULL`, `contract_version text NOT NULL`, `input_snapshot jsonb NOT NULL`, `config_snapshot jsonb NOT NULL`, `resume_checkpoint_id uuid NULL`, `failure_category text`, `error_code`, `error_message`, `created_at`, `queued_at`, `started_at`, `completed_at`, `updated_at`.

Indexes: unique `(job_id,run_number)`, `(job_id,status)`, `(status,queued_at)`, partial unique `(job_id) WHERE status IN ('queued','running','paused','waiting_for_review')`.

### `job_steps`

`id uuid PK`, `pipeline_run_id uuid FK pipeline_runs ON DELETE CASCADE`, `pipeline_node_id uuid FK pipeline_nodes`, `node_key text NOT NULL`, `status text CHECK (status IN ('pending','ready','queued','running','paused','waiting_for_review','retrying','completed','skipped','failed','cancelled','blocked'))`, `current_attempt integer NOT NULL DEFAULT 0`, `input_fingerprint char(64)`, `input_refs jsonb NOT NULL DEFAULT '{}'`, `output_refs jsonb NOT NULL DEFAULT '{}'`, `provider_snapshot jsonb NOT NULL DEFAULT '{}'`, `resource_class text`, `progress_percent numeric(5,2)`, `checkpoint_id uuid NULL`, `skip_reason text`, `failure_category text`, `error_code`, `error_message`, `retryable boolean`, `ready_at`, `queued_at`, `started_at`, `completed_at`, `heartbeat_at`, `created_at`, `updated_at`.

Indexes: unique `(pipeline_run_id,node_key)`, `(pipeline_run_id,status)`, `(status,ready_at)`, `(status,heartbeat_at)`, `(pipeline_node_id,status)`. `node_key` phải khớp referenced PipelineNode.

### `job_step_attempts`

`id uuid PK`, `job_step_id uuid FK job_steps ON DELETE CASCADE`, `attempt integer CHECK (attempt > 0)`, `status text CHECK (status IN ('running','completed','failed','cancelled'))`, `worker_id text`, `lease_token_hash char(64)`, `lease_expires_at`, `input_fingerprint char(64)`, `progress_percent numeric(5,2)`, `provider_snapshot jsonb`, `output_refs jsonb`, `failure_category text`, `error_code`, `error_message`, `retryable boolean`, `started_at`, `completed_at`, `heartbeat_at`, `created_at`, `updated_at`.

Indexes: unique `(job_step_id,attempt)`, `(job_step_id,attempt DESC)`, `(status,lease_expires_at)`, `(worker_id,status)`. Attempt row append-only sau terminal; timeout/worker lost nằm trong `failure_category`, không tạo status synonym.

### `job_checkpoints`

`id uuid PK`, `pipeline_run_id uuid FK pipeline_runs`, `job_step_id uuid FK job_steps`, `completed_node_key text NOT NULL`, `context_artifact_id uuid FK artifacts`, `state_hash char(64)`, `schema_version text`, `input_fingerprint char(64)`, `is_valid boolean NOT NULL DEFAULT true`, `invalidated_reason text`, `created_at`, `updated_at`, `expires_at`.

Indexes: unique `(pipeline_run_id,completed_node_key,state_hash)`, `(pipeline_run_id,created_at DESC)`, `(is_valid,expires_at)`. `pipeline_runs.resume_checkpoint_id` và `job_steps.checkpoint_id` FK được thêm sau table creation.

### `dead_letters`

`id uuid PK`, `job_id uuid FK jobs`, `pipeline_run_id uuid FK pipeline_runs`, `failure_event_id uuid`, `command_snapshot jsonb NOT NULL`, `safe_error jsonb NOT NULL`, `replayed_as_job_id uuid NULL FK jobs`, `created_at`, `updated_at`, `replayed_at`.

Indexes: unique `(job_id)`, `(created_at DESC)`, `(replayed_as_job_id)`.

### `reviews`

`id uuid PK`, `project_id uuid FK projects`, `job_id uuid FK jobs`, `pipeline_run_id uuid FK pipeline_runs`, `job_step_id uuid UNIQUE FK job_steps`, `review_type text CHECK (review_type IN ('script','timeline','scene_match','subtitle','voice'))`, `status text CHECK (status IN ('open','approved','rejected','cancelled','expired'))`, `proposed_resource_type text NOT NULL`, `proposed_resource_id uuid NOT NULL`, `proposed_resource_revision bigint`, `allowed_actions jsonb NOT NULL`, `selected_resource_type text NULL`, `selected_resource_id uuid NULL`, `selected_resource_revision bigint NULL`, `decision text NULL`, `decision_comment text NULL`, `resolved_by uuid NULL FK users`, `opened_at timestamptz`, `resolved_at timestamptz`, `expires_at timestamptz`, `created_at`, `updated_at`.

Indexes: `(job_id,status)`, `(project_id,status,opened_at)`, `(status,expires_at)`. Polymorphic resource IDs được application validator resolve/scope; approved resolution bắt buộc selected resource cùng Project, đúng allowed type và dựa trên proposal/allowed lineage. Một review chỉ resolve terminal một lần.

## 6. Content and intelligence

### `scripts`

`id uuid PK`, `project_id uuid FK projects`, `label text`, `status text CHECK (status IN ('active','archived','deleted'))`, `current_version_id uuid NULL`, `revision bigint`, `created_by uuid FK users`, `updated_by uuid FK users`, `created_at`, `updated_at`, `deleted_at`.

Indexes: `(project_id,status)`, `(project_id,updated_at DESC)`.

### `script_versions`

`id uuid PK`, `script_id uuid FK scripts ON DELETE CASCADE`, `version integer CHECK (version > 0)`, `status text CHECK (status IN ('draft','proposed','approved','superseded'))`, `language text NOT NULL`, `origin text CHECK (origin IN ('ai','user','imported','system'))`, `content jsonb NOT NULL`, `content_hash char(64) NOT NULL`, `based_on_version_id uuid NULL FK script_versions`, `produced_by_job_step_id uuid NULL FK job_steps`, `created_by uuid NULL FK users`, `approved_by uuid NULL FK users`, `approved_at`, `created_at`, `updated_at`.

Indexes: unique `(script_id,version)`, unique `(script_id,content_hash)`, `(script_id,status,version DESC)`. Content immutable; `updated_at` chỉ phản ánh approval/status metadata.

### `script_reviews`

`id uuid PK`, `script_version_id uuid FK script_versions`, `decision text CHECK (decision IN ('approved','rejected','changes_requested'))`, `actor_id uuid FK users`, `comment text`, `created_at`; index `(script_version_id,created_at)`.

### `narrations`

`id uuid PK`, `project_id uuid FK projects`, `script_version_id uuid FK script_versions`, `status text CHECK (status IN ('created','synthesizing','ready','failed','superseded'))`, `provider_configuration_id uuid NULL FK provider_configurations`, `voice_snapshot jsonb NOT NULL`, `request_snapshot jsonb NOT NULL`, `audio_artifact_id uuid NULL FK artifacts`, `timing_artifact_id uuid NULL FK artifacts`, `duration_sec numeric(14,3)`, `produced_by_job_step_id uuid NULL FK job_steps`, `error_code`, `created_at`, `updated_at`.

Indexes: `(project_id,script_version_id,created_at DESC)`, `(status)`, `(audio_artifact_id)`.

### `scenes`

`id uuid PK`, `project_id uuid FK projects`, `source_asset_id uuid FK assets`, `source_artifact_id uuid FK artifacts`, `detection_revision text NOT NULL`, `scene_index integer CHECK (scene_index >= 0)`, `status text CHECK (status IN ('detected','analyzed','confirmed','rejected','superseded'))`, `source_start_sec numeric(14,3)`, `source_end_sec numeric(14,3)`, `description`, `dialogue`, `location`, `emotion`, `actions jsonb`, `quality_score numeric(5,4)`, `embedding_artifact_id uuid NULL FK artifacts`, `thumbnail_artifact_id uuid NULL FK artifacts`, `attributes jsonb`, `revision bigint`, `produced_by_job_step_id uuid NULL FK job_steps`, `updated_by uuid NULL FK users`, `created_at`, `updated_at`.

Indexes: unique `(source_artifact_id,detection_revision,scene_index)`, `(project_id,source_asset_id,source_start_sec)`, `(project_id,status)`. Check `0 <= source_start_sec < source_end_sec`.

### `characters`

`id uuid PK`, `project_id uuid FK projects`, `display_name text`, `status text CHECK (status IN ('unconfirmed','confirmed','rejected','merged'))`, `merged_into_character_id uuid NULL FK characters`, `confidence numeric(5,4)`, `aliases jsonb`, `profile_artifact_id uuid NULL FK artifacts`, `attributes jsonb`, `revision bigint`, `produced_by_job_step_id uuid NULL FK job_steps`, `updated_by uuid NULL FK users`, `created_at`, `updated_at`.

Indexes: `(project_id,status)`, `(merged_into_character_id)`.

### `character_appearances`

`character_id uuid FK characters`, `scene_id uuid FK scenes`, `confidence numeric(5,4)`, `face_count integer`, `boxes jsonb`, `embedding_artifact_id uuid NULL FK artifacts`, `confirmed_by_user_id uuid NULL FK users`, `created_at`, `updated_at`; PK `(character_id,scene_id)`, index `(scene_id,confidence DESC)`.

### `timelines`

`id uuid PK`, `project_id uuid FK projects`, `label text`, `status text CHECK (status IN ('active','archived','deleted'))`, `current_version_id uuid NULL`, `revision bigint`, `created_by uuid FK users`, `updated_by uuid FK users`, `created_at`, `updated_at`, `deleted_at`.

Indexes: `(project_id,status)`, `(project_id,updated_at DESC)`.

### `timeline_versions`

`id uuid PK`, `timeline_id uuid FK timelines ON DELETE CASCADE`, `version integer CHECK (version > 0)`, `status text CHECK (status IN ('draft','proposed','approved','locked','superseded'))`, `schema_version text NOT NULL`, `document jsonb NOT NULL`, `content_hash char(64) NOT NULL`, `origin text CHECK (origin IN ('ai','user','imported','system'))`, `based_on_version_id uuid NULL FK timeline_versions`, `produced_by_job_step_id uuid NULL FK job_steps`, `created_by uuid NULL FK users`, `approved_by uuid NULL FK users`, `approved_at`, `created_at`, `updated_at`.

Indexes: unique `(timeline_id,version)`, unique `(timeline_id,content_hash)`, `(timeline_id,status,version DESC)`, `(content_hash)`. Canonical Track, Clip và SubtitleTrack nằm trong `document`; projection table nếu có chỉ là rebuildable read model, không là source of truth.

Sau table creation, thêm FK cho `scripts.current_version_id`, `timelines.current_version_id`, `projects.current_script_version_id`, `projects.current_timeline_version_id`; trigger/application guard phải xác nhận cùng aggregate/Project.

## 7. Render

### `renders`

`id uuid PK`, `project_id uuid FK projects`, `timeline_version_id uuid FK timeline_versions`, `render_profile_id uuid FK render_profiles`, `supersedes_render_id uuid NULL FK renders`, `profile_snapshot jsonb NOT NULL`, `status text CHECK (status IN ('created','queued','running','completed','failed','cancelled'))`, `job_id uuid UNIQUE FK jobs`, `request_hash char(64)`, `request jsonb`, `qa_summary jsonb`, `error_code`, `error_message`, `created_by uuid FK users`, `created_at`, `updated_at`, `completed_at`.

Indexes: `(timeline_version_id,render_profile_id,request_hash)`, unique `(timeline_version_id,render_profile_id,request_hash) WHERE status='completed'`, `(supersedes_render_id)`, `(project_id,created_at DESC)`, `(status,created_at)`. Retry/rerender tạo row mới; partial unique bảo đảm chỉ một canonical completed result cho exact request hash, còn application có thể reuse row đó.

### `render_artifacts`

`render_id uuid FK renders`, `artifact_id uuid FK artifacts`, `role text CHECK (role IN ('video','audio','subtitle','thumbnail','qa_report','metadata'))`, `is_canonical boolean`, `created_at`; PK `(render_id,artifact_id,role)`, unique `(render_id,role) WHERE is_canonical`, index `(artifact_id)`.

## 8. Analysis and candidate evaluation

### `analyses`

`id uuid PK`, `project_id uuid FK projects`, `kind text NOT NULL`, `status text CHECK (status IN
('created','running','completed','failed','cancelled'))`, `input_refs jsonb NOT NULL`,
`schema_version text NOT NULL`, `result jsonb`, `result_artifact_id uuid NULL FK artifacts`,
`producer_job_step_id uuid NULL FK job_steps`, `provenance jsonb NOT NULL`, `error_code text`,
`created_at`, `updated_at`, `completed_at`.

Indexes: `(project_id,kind,created_at DESC)`, `(producer_job_step_id)`. `kind=reference_style` stores
abstract metrics and never copied source footage as its semantic result.

Candidate tables are introduced when candidate workflows are enabled:

- `candidate_selection_policies`: versioned immutable active policy documents.
- `generation_candidates`: candidate group, producer JobStep, resource/Artifact ref, status and rank.
- `evaluation_results`: candidate, evaluator/policy/model snapshot, component scores and evidence ref.

These tables use Project ownership and immutable terminal results. They do not add Job state aliases.

## 9. Provider execution, events and idempotency

### `provider_runs`

`id uuid PK`, `job_id uuid FK jobs`, `job_step_id uuid FK job_steps`, `provider_configuration_id uuid NULL FK provider_configurations`, `provider_kind text CHECK (provider_kind IN ('llm','vlm','tts','asr','embedding'))`, `adapter_key text`, `model text`, `request_hash char(64)`, `status text CHECK (status IN ('started','completed','failed','cancelled'))`, `input_units bigint`, `output_units bigint`, `estimated_cost numeric(18,8)`, `latency_ms integer`, `provider_request_id text`, `error_code text`, `redacted_metadata jsonb`, `created_at`, `updated_at`, `completed_at`.

Indexes: `(job_id,created_at)`, `(job_step_id)`, `(provider_configuration_id,created_at)`, `(status,created_at)`.

### `job_events`

`event_id uuid PK`, `schema_version text NOT NULL`, `workspace_id uuid FK workspaces`, `project_id uuid FK projects`, `job_id uuid FK jobs`, `pipeline_run_id uuid NULL FK pipeline_runs`, `job_step_id uuid NULL FK job_steps`, `pipeline_node_id uuid NULL FK pipeline_nodes`, `sequence bigint CHECK (sequence > 0)`, `event_type text NOT NULL`, `payload jsonb NOT NULL`, `correlation_id text`, `occurred_at timestamptz`, `created_at timestamptz`.

Indexes: unique `(job_id,sequence)`, `(job_id,sequence)`, `(project_id,occurred_at)`, `(event_type,occurred_at)`. Append-only; không có `updated_at` vì event không mutate.

### `outbox_events`

`id uuid PK`, `aggregate_type text`, `aggregate_id uuid`, `event_type text`, `payload jsonb`, `status text CHECK (status IN ('pending','published','failed'))`, `attempts integer`, `available_at`, `published_at`, `last_error`, `created_at`, `updated_at`.

Indexes: `(status,available_at)`, `(aggregate_type,aggregate_id)`. Payload dùng cùng event ID để publish retry idempotent.

### `idempotency_keys`

`workspace_id uuid FK workspaces`, `key text`, `request_hash char(64)`, `response_status smallint`, `response_body jsonb`, `resource_type text`, `resource_id uuid`, `expires_at`, `created_at`, `updated_at`; PK `(workspace_id,key)`, index `(expires_at)`.

## 10. Cross-table constraints

1. Mọi repository query product scope qua Workspace/Project; authorization không dựa vào ID trong request body.
2. `pipelines.workflow_id`, `jobs.workflow_id` và `jobs.pipeline_id` phải cùng lineage.
3. PipelineRun pipeline snapshot phải bằng Job snapshot; JobStep node phải thuộc Pipeline đó.
4. Mỗi Job chỉ có tối đa một active PipelineRun; state mapping phải thỏa mục 6 của domain model.
5. Artifact/Asset/Scene/Script/Timeline/Render refs phải cùng Project, trừ explicitly shared system Artifact/Profile.
6. Worker commit Artifact + JobStep terminal state + checkpoint/event trong một orchestration transaction; blob dùng stage → verify → commit.
7. `Render.completed` yêu cầu Job `completed`, required `render_artifacts` committed và QA policy pass.
8. `Asset.ready` yêu cầu original Artifact `committed`, checksum/probe pass và không quarantine.
9. JSONB được validate tại API boundary và worker boundary bằng versioned JSON Schema; DB `CHECK` bảo vệ invariant đơn giản.
10. PostgreSQL migration tests phải xác minh mọi FK, check, partial unique index và clean bootstrap.

## 11. Intentionally non-tabular objects

| Object | Persistence decision | Reason |
|---|---|---|
| Track, Clip, SubtitleTrack | Embedded trong immutable `timeline_versions.document`. | Atomic edit/version/hash và renderer input canonical; tránh hai source of truth. |
| Voice catalog entry | Không bắt buộc table ở baseline; cache tùy adapter. Exact selection nằm trong Narration snapshot. | Provider catalog thay đổi và provider-specific. |
| Embedding vectors | Artifact hoặc future vector-store port; DB giữ Artifact ref/model metadata. | Không biến PostgreSQL thành blob store; pgvector quyết định sau benchmark/OQ-07. |
| Media bytes/checkpoint payload | Object storage Artifact. | Kích thước lớn, cần checksum/version/lifecycle. |

## 12. Retention and external imports

- External/user media imports use the same staged upload, checksum, probe and Artifact commit path.
- Research fixtures are not product state unless explicitly authorized as Project data.
- DB audit records follow retention policy even when a blob becomes `expired|deleted`; sweepers do
  not delete blobs protected by active Jobs, checkpoints or approved/current versions.
- No upstream task database, status or raw path is mapped into NH-Media product state.
