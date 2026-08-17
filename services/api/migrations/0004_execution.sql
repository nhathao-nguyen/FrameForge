-- T205/T206: canonical Job, PipelineRun, JobStep, review and recovery records.
CREATE TABLE jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    workflow_id uuid NOT NULL REFERENCES workflows(id) ON DELETE RESTRICT,
    pipeline_id uuid NOT NULL REFERENCES pipelines(id) ON DELETE RESTRICT,
    kind text NOT NULL CHECK (kind IN ('pipeline','render','asset_probe','analysis','export')),
    mode text NOT NULL CHECK (mode IN ('automatic','studio','preview')),
    status text NOT NULL CHECK (status IN ('created','queued','running','paused','waiting_for_review','retrying','cancelling','completed','failed','dead_lettered','cancelled')),
    requested_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    supersedes_job_id uuid REFERENCES jobs(id) ON DELETE RESTRICT,
    command jsonb NOT NULL,
    input_snapshot jsonb NOT NULL,
    pipeline_snapshot jsonb NOT NULL,
    provider_bindings_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    start_from text,
    stop_after text,
    priority smallint NOT NULL DEFAULT 5 CHECK (priority BETWEEN 0 AND 20),
    max_runs integer NOT NULL DEFAULT 3 CHECK (max_runs > 0),
    current_run_number integer NOT NULL DEFAULT 0 CHECK (current_run_number >= 0),
    current_pipeline_run_id uuid,
    progress_percent numeric(5,2) CHECK (progress_percent BETWEEN 0 AND 100),
    current_node_key text,
    cancel_requested_at timestamptz,
    error_code text,
    error_message text,
    correlation_id text,
    created_at timestamptz NOT NULL DEFAULT now(),
    queued_at timestamptz,
    started_at timestamptz,
    completed_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX jobs_project_created_idx ON jobs (project_id, created_at DESC);
CREATE INDEX jobs_status_priority_idx ON jobs (status, priority DESC, created_at);
CREATE INDEX jobs_correlation_idx ON jobs (correlation_id);

CREATE TABLE pipeline_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    pipeline_id uuid NOT NULL REFERENCES pipelines(id) ON DELETE RESTRICT,
    run_number integer NOT NULL CHECK (run_number > 0),
    status text NOT NULL CHECK (status IN ('created','queued','running','paused','waiting_for_review','completed','failed','cancelled')),
    executor_kind text NOT NULL CHECK (executor_kind IN ('go_media','python_ml','system')),
    executor_version text NOT NULL,
    contract_version text NOT NULL,
    input_snapshot jsonb NOT NULL,
    config_snapshot jsonb NOT NULL,
    resume_checkpoint_id uuid,
    failure_category text,
    error_code text,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT now(),
    queued_at timestamptz,
    started_at timestamptz,
    completed_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (job_id, run_number)
);
CREATE INDEX pipeline_runs_status_idx ON pipeline_runs (job_id, status);
CREATE UNIQUE INDEX pipeline_runs_one_active_uq ON pipeline_runs (job_id) WHERE status IN ('queued','running','paused','waiting_for_review');

CREATE TABLE job_steps (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_run_id uuid NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    pipeline_node_id uuid NOT NULL REFERENCES pipeline_nodes(id) ON DELETE RESTRICT,
    node_key text NOT NULL,
    status text NOT NULL CHECK (status IN ('pending','ready','queued','running','paused','waiting_for_review','retrying','completed','skipped','failed','cancelled','blocked')),
    current_attempt integer NOT NULL DEFAULT 0 CHECK (current_attempt >= 0),
    input_fingerprint char(64) CHECK (input_fingerprint IS NULL OR input_fingerprint ~ '^[0-9a-f]{64}$'),
    input_refs jsonb NOT NULL DEFAULT '{}'::jsonb,
    output_refs jsonb NOT NULL DEFAULT '{}'::jsonb,
    provider_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    resource_class text,
    progress_percent numeric(5,2) CHECK (progress_percent BETWEEN 0 AND 100),
    checkpoint_id uuid,
    skip_reason text,
    failure_category text,
    error_code text,
    error_message text,
    retryable boolean,
    ready_at timestamptz,
    queued_at timestamptz,
    started_at timestamptz,
    completed_at timestamptz,
    heartbeat_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (pipeline_run_id, node_key)
);
CREATE INDEX job_steps_run_status_idx ON job_steps (pipeline_run_id, status);
CREATE INDEX job_steps_ready_idx ON job_steps (status, ready_at);
CREATE INDEX job_steps_heartbeat_idx ON job_steps (status, heartbeat_at);

CREATE TABLE job_step_attempts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_step_id uuid NOT NULL REFERENCES job_steps(id) ON DELETE CASCADE,
    attempt integer NOT NULL CHECK (attempt > 0),
    status text NOT NULL CHECK (status IN ('running','completed','failed','cancelled')),
    worker_id text,
    lease_token_hash char(64),
    lease_expires_at timestamptz,
    input_fingerprint char(64),
    progress_percent numeric(5,2) CHECK (progress_percent BETWEEN 0 AND 100),
    provider_snapshot jsonb,
    output_refs jsonb,
    failure_category text,
    error_code text,
    error_message text,
    retryable boolean,
    started_at timestamptz,
    completed_at timestamptz,
    heartbeat_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (job_step_id, attempt)
);
CREATE INDEX job_attempts_lease_idx ON job_step_attempts (status, lease_expires_at);
CREATE INDEX job_attempts_worker_idx ON job_step_attempts (worker_id, status);

CREATE TABLE job_checkpoints (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_run_id uuid NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    job_step_id uuid NOT NULL REFERENCES job_steps(id) ON DELETE CASCADE,
    completed_node_key text NOT NULL,
    context_artifact_id uuid REFERENCES artifacts(id) ON DELETE RESTRICT,
    state_hash char(64),
    schema_version text,
    input_fingerprint char(64),
    is_valid boolean NOT NULL DEFAULT true,
    invalidated_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz,
    UNIQUE (pipeline_run_id, completed_node_key, state_hash)
);
CREATE INDEX job_checkpoints_recent_idx ON job_checkpoints (pipeline_run_id, created_at DESC);
CREATE INDEX job_checkpoints_expiry_idx ON job_checkpoints (is_valid, expires_at);

CREATE TABLE dead_letters (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id uuid NOT NULL REFERENCES jobs(id) ON DELETE RESTRICT,
    pipeline_run_id uuid NOT NULL REFERENCES pipeline_runs(id) ON DELETE RESTRICT,
    failure_event_id uuid,
    command_snapshot jsonb NOT NULL,
    safe_error jsonb NOT NULL,
    replayed_as_job_id uuid REFERENCES jobs(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    replayed_at timestamptz,
    UNIQUE (job_id)
);
CREATE INDEX dead_letters_created_idx ON dead_letters (created_at DESC);

CREATE TABLE reviews (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    job_id uuid NOT NULL REFERENCES jobs(id) ON DELETE RESTRICT,
    pipeline_run_id uuid NOT NULL REFERENCES pipeline_runs(id) ON DELETE RESTRICT,
    job_step_id uuid NOT NULL UNIQUE REFERENCES job_steps(id) ON DELETE RESTRICT,
    review_type text NOT NULL CHECK (review_type IN ('script','timeline','scene_match','subtitle','voice')),
    status text NOT NULL CHECK (status IN ('open','approved','rejected','cancelled','expired')),
    proposed_resource_type text NOT NULL,
    proposed_resource_id uuid NOT NULL,
    proposed_resource_revision bigint,
    allowed_actions jsonb NOT NULL,
    selected_resource_type text,
    selected_resource_id uuid,
    selected_resource_revision bigint,
    decision text,
    decision_comment text,
    resolved_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    opened_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX reviews_status_idx ON reviews (project_id, status, opened_at);

ALTER TABLE jobs ADD CONSTRAINT jobs_current_run_fk FOREIGN KEY (current_pipeline_run_id) REFERENCES pipeline_runs(id) ON DELETE RESTRICT;
ALTER TABLE pipeline_runs ADD CONSTRAINT pipeline_runs_resume_checkpoint_fk FOREIGN KEY (resume_checkpoint_id) REFERENCES job_checkpoints(id) ON DELETE RESTRICT;
ALTER TABLE job_steps ADD CONSTRAINT job_steps_checkpoint_fk FOREIGN KEY (checkpoint_id) REFERENCES job_checkpoints(id) ON DELETE RESTRICT;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_producer_job_fk FOREIGN KEY (producer_job_id) REFERENCES jobs(id) ON DELETE RESTRICT;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_producer_step_fk FOREIGN KEY (producer_job_step_id) REFERENCES job_steps(id) ON DELETE RESTRICT;
