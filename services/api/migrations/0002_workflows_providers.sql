-- T202/T203: built-in graph definitions, SecretStore metadata and RenderProfiles.
CREATE TABLE workflows (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid REFERENCES workspaces(id) ON DELETE RESTRICT,
    workflow_key text NOT NULL,
    name text NOT NULL,
    description text,
    status text NOT NULL CHECK (status IN ('draft','active','deprecated','disabled')),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (workspace_id IS NULL OR created_by IS NOT NULL)
);
CREATE UNIQUE INDEX workflows_system_key_uq ON workflows (workflow_key) WHERE workspace_id IS NULL;
CREATE UNIQUE INDEX workflows_workspace_key_uq ON workflows (workspace_id, workflow_key) WHERE workspace_id IS NOT NULL;
CREATE INDEX workflows_status_idx ON workflows (status, workflow_key);

CREATE TABLE pipelines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id uuid NOT NULL REFERENCES workflows(id) ON DELETE RESTRICT,
    version integer NOT NULL CHECK (version > 0),
    status text NOT NULL CHECK (status IN ('draft','active','deprecated','disabled')),
    schema_version text NOT NULL,
    definition jsonb NOT NULL,
    content_hash char(64) NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
    created_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    activated_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workflow_id, version),
    UNIQUE (workflow_id, content_hash)
);
CREATE INDEX pipelines_status_idx ON pipelines (workflow_id, status);

CREATE TABLE pipeline_nodes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id uuid NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    node_key text NOT NULL,
    node_type text NOT NULL,
    display_name text NOT NULL,
    execution_class text NOT NULL CHECK (execution_class IN ('system','probe','ai','ml','media','render')),
    is_optional boolean NOT NULL DEFAULT false,
    failure_mode text NOT NULL CHECK (failure_mode IN ('hard','soft')),
    review_policy text NOT NULL CHECK (review_policy IN ('none','approval_completes_node','approval_resumes_node')),
    input_schema jsonb,
    output_schema jsonb,
    required_artifact_roles jsonb,
    produced_artifact_roles jsonb,
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    timeout_sec integer NOT NULL CHECK (timeout_sec > 0),
    max_attempts integer NOT NULL CHECK (max_attempts > 0),
    retry_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
    checkpoint_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
    resource_requirements jsonb NOT NULL DEFAULT '{}'::jsonb,
    idempotency_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
    progress_weight numeric(8,4) NOT NULL DEFAULT 1 CHECK (progress_weight >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (pipeline_id, node_key)
);
CREATE INDEX pipeline_nodes_class_idx ON pipeline_nodes (pipeline_id, execution_class);

CREATE TABLE pipeline_node_dependencies (
    pipeline_id uuid NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    node_id uuid NOT NULL REFERENCES pipeline_nodes(id) ON DELETE CASCADE,
    depends_on_node_id uuid NOT NULL REFERENCES pipeline_nodes(id) ON DELETE CASCADE,
    condition jsonb NOT NULL DEFAULT '{}'::jsonb,
    required boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (node_id, depends_on_node_id),
    CHECK (node_id <> depends_on_node_id)
);
CREATE INDEX pipeline_dependencies_parent_idx ON pipeline_node_dependencies (depends_on_node_id);

CREATE TABLE secret_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid REFERENCES workspaces(id) ON DELETE RESTRICT,
    purpose text NOT NULL,
    ciphertext bytea NOT NULL,
    nonce bytea NOT NULL,
    key_version text NOT NULL,
    aad jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL CHECK (status IN ('active','rotated','revoked','deleted')),
    created_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    rotated_from_id uuid REFERENCES secret_records(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX secret_records_scope_idx ON secret_records (workspace_id, purpose, status);

CREATE TABLE provider_configurations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid REFERENCES workspaces(id) ON DELETE RESTRICT,
    kind text NOT NULL CHECK (kind IN ('llm','vlm','tts','asr','embedding')),
    adapter_key text NOT NULL,
    display_name text NOT NULL,
    status text NOT NULL CHECK (status IN ('draft','active','disabled','invalid','deleted')),
    credential_ref text,
    endpoint_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
    model_defaults jsonb NOT NULL DEFAULT '{}'::jsonb,
    capability_overrides jsonb NOT NULL DEFAULT '{}'::jsonb,
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    last_validated_at timestamptz,
    created_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    updated_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE UNIQUE INDEX provider_system_name_uq ON provider_configurations (kind, display_name) WHERE workspace_id IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX provider_workspace_name_uq ON provider_configurations (workspace_id, kind, display_name) WHERE workspace_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX provider_scope_status_idx ON provider_configurations (workspace_id, kind, status);

CREATE TABLE render_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid REFERENCES workspaces(id) ON DELETE RESTRICT,
    profile_key text NOT NULL,
    version integer NOT NULL CHECK (version > 0),
    status text NOT NULL CHECK (status IN ('draft','active','deprecated','disabled')),
    schema_version text NOT NULL,
    document jsonb NOT NULL,
    content_hash char(64) NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
    created_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, profile_key, version),
    UNIQUE (workspace_id, profile_key, content_hash)
);
CREATE INDEX render_profiles_status_idx ON render_profiles (workspace_id, profile_key, status);

CREATE OR REPLACE FUNCTION nh_media_reject_active_definition_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status = 'active' AND (NEW.definition IS DISTINCT FROM OLD.definition OR NEW.content_hash IS DISTINCT FROM OLD.content_hash OR NEW.version IS DISTINCT FROM OLD.version) THEN
        RAISE EXCEPTION 'active pipeline definition is immutable';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER pipelines_active_immutable BEFORE UPDATE ON pipelines FOR EACH ROW EXECUTE FUNCTION nh_media_reject_active_definition_update();

CREATE OR REPLACE FUNCTION nh_media_reject_active_profile_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status = 'active' AND (NEW.document IS DISTINCT FROM OLD.document OR NEW.content_hash IS DISTINCT FROM OLD.content_hash OR NEW.version IS DISTINCT FROM OLD.version) THEN
        RAISE EXCEPTION 'active render profile is immutable';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER render_profiles_active_immutable BEFORE UPDATE ON render_profiles FOR EACH ROW EXECUTE FUNCTION nh_media_reject_active_profile_update();
