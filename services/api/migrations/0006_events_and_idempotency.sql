-- T209 and immutable event/outbox/idempotency records.
CREATE TABLE provider_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id uuid NOT NULL REFERENCES jobs(id) ON DELETE RESTRICT,
    job_step_id uuid NOT NULL REFERENCES job_steps(id) ON DELETE RESTRICT,
    provider_configuration_id uuid REFERENCES provider_configurations(id) ON DELETE RESTRICT,
    provider_kind text NOT NULL CHECK (provider_kind IN ('llm','vlm','tts','asr','embedding')),
    adapter_key text NOT NULL,
    model text,
    request_hash char(64),
    status text NOT NULL CHECK (status IN ('started','completed','failed','cancelled')),
    input_units bigint,
    output_units bigint,
    estimated_cost numeric(18,8),
    latency_ms integer,
    provider_request_id text,
    error_code text,
    redacted_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE INDEX provider_runs_job_idx ON provider_runs (job_id, created_at);

CREATE TABLE job_events (
    event_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    schema_version text NOT NULL,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    job_id uuid NOT NULL REFERENCES jobs(id) ON DELETE RESTRICT,
    pipeline_run_id uuid REFERENCES pipeline_runs(id) ON DELETE RESTRICT,
    job_step_id uuid REFERENCES job_steps(id) ON DELETE RESTRICT,
    pipeline_node_id uuid REFERENCES pipeline_nodes(id) ON DELETE RESTRICT,
    node_key text,
    sequence bigint NOT NULL CHECK (sequence > 0),
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    correlation_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (job_id, sequence)
);
CREATE INDEX job_events_project_idx ON job_events (project_id, occurred_at);
CREATE INDEX job_events_type_idx ON job_events (event_type, occurred_at);

CREATE TABLE outbox_events (
    id uuid PRIMARY KEY,
    aggregate_type text NOT NULL,
    aggregate_id uuid NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    status text NOT NULL CHECK (status IN ('pending','published','failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX outbox_ready_idx ON outbox_events (status, available_at);
CREATE INDEX outbox_aggregate_idx ON outbox_events (aggregate_type, aggregate_id);

CREATE TABLE idempotency_keys (
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
    key text NOT NULL,
    request_hash char(64) NOT NULL,
    response_status smallint NOT NULL,
    response_body jsonb NOT NULL,
    resource_type text,
    resource_id uuid,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, key)
);
CREATE INDEX idempotency_expiry_idx ON idempotency_keys (expires_at);

CREATE OR REPLACE FUNCTION nh_media_reject_event_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'job_events are append-only';
END;
$$;
CREATE TRIGGER job_events_append_only BEFORE UPDATE OR DELETE ON job_events FOR EACH ROW EXECUTE FUNCTION nh_media_reject_event_update();
