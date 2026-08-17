-- T204/T206 foundations: Projects, UploadSessions, Asset and immutable Artifact metadata.
CREATE TABLE projects (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
    name text NOT NULL,
    workflow_id uuid REFERENCES workflows(id) ON DELETE RESTRICT,
    status text NOT NULL CHECK (status IN ('active','archived','deleted')),
    settings jsonb NOT NULL DEFAULT '{}'::jsonb,
    current_script_version_id uuid,
    current_timeline_version_id uuid,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX projects_workspace_updated_idx ON projects (workspace_id, updated_at DESC);
CREATE INDEX projects_workspace_status_idx ON projects (workspace_id, status);
CREATE UNIQUE INDEX projects_live_name_uq ON projects (workspace_id, name) WHERE deleted_at IS NULL;

CREATE TABLE artifacts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    producer_job_id uuid,
    producer_job_step_id uuid,
    kind text NOT NULL,
    role text NOT NULL,
    status text NOT NULL CHECK (status IN ('staged','committed','quarantined','expired','deleted')),
    storage_backend text NOT NULL CHECK (storage_backend IN ('local','s3','minio')),
    object_key text NOT NULL,
    object_version text,
    content_type text,
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
    sha256 char(64) NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    schema_version text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    retention_until timestamptz,
    created_by_user_id uuid REFERENCES users(id) ON DELETE RESTRICT,
    created_by_service text,
    committed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (producer_job_id IS NULL OR producer_job_step_id IS NOT NULL OR created_by_service IS NOT NULL OR created_by_user_id IS NOT NULL)
);
CREATE UNIQUE INDEX artifacts_locator_version_uq ON artifacts (storage_backend, object_key, COALESCE(object_version, ''));
CREATE INDEX artifacts_project_kind_idx ON artifacts (project_id, kind, created_at DESC);
CREATE INDEX artifacts_checksum_idx ON artifacts (sha256);
CREATE INDEX artifacts_status_retention_idx ON artifacts (status, retention_until);

CREATE TABLE assets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    kind text NOT NULL CHECK (kind IN ('video','audio','image','font','music','overlay','other')),
    status text NOT NULL CHECK (status IN ('pending_upload','uploading','uploaded','validating','ready','failed','quarantined','deleted')),
    original_artifact_id uuid REFERENCES artifacts(id) ON DELETE RESTRICT,
    original_filename text,
    declared_mime_type text,
    detected_mime_type text,
    size_bytes bigint CHECK (size_bytes >= 0),
    sha256 char(64) CHECK (sha256 IS NULL OR sha256 ~ '^[0-9a-f]{64}$'),
    duration_sec numeric(14,3) CHECK (duration_sec IS NULL OR duration_sec >= 0),
    width integer CHECK (width IS NULL OR width > 0),
    height integer CHECK (height IS NULL OR height > 0),
    frame_rate_num integer CHECK (frame_rate_num IS NULL OR frame_rate_num > 0),
    frame_rate_den integer CHECK (frame_rate_den IS NULL OR frame_rate_den > 0),
    validation_report jsonb NOT NULL DEFAULT '{}'::jsonb,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX assets_project_status_idx ON assets (project_id, kind, status);
CREATE INDEX assets_project_created_idx ON assets (project_id, created_at DESC);

CREATE TABLE asset_uploads (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id uuid NOT NULL REFERENCES assets(id) ON DELETE RESTRICT,
    storage_backend text NOT NULL CHECK (storage_backend IN ('local','s3','minio')),
    staging_object_key text NOT NULL,
    provider_upload_id text,
    status text NOT NULL CHECK (status IN ('active','completed','aborted','expired')),
    multipart boolean NOT NULL,
    part_size bigint CHECK (part_size IS NULL OR part_size > 0),
    expected_size bigint CHECK (expected_size IS NULL OR expected_size >= 0),
    completed_size bigint CHECK (completed_size IS NULL OR completed_size >= 0),
    expected_sha256 char(64) CHECK (expected_sha256 IS NULL OR expected_sha256 ~ '^[0-9a-f]{64}$'),
    expires_at timestamptz,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX asset_upload_active_uq ON asset_uploads (asset_id) WHERE status = 'active';
CREATE UNIQUE INDEX asset_upload_key_uq ON asset_uploads (storage_backend, staging_object_key);
CREATE INDEX asset_upload_expiry_idx ON asset_uploads (status, expires_at);

CREATE TABLE artifact_variants (
    asset_id uuid NOT NULL REFERENCES assets(id) ON DELETE RESTRICT,
    artifact_id uuid NOT NULL REFERENCES artifacts(id) ON DELETE RESTRICT,
    variant_kind text NOT NULL CHECK (variant_kind IN ('original','proxy','thumbnail','waveform','audio_extract','normalized','transcript','other')),
    is_canonical boolean NOT NULL DEFAULT false,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (asset_id, artifact_id, variant_kind)
);
CREATE UNIQUE INDEX artifact_variant_canonical_uq ON artifact_variants (asset_id, variant_kind) WHERE is_canonical;
CREATE INDEX artifact_variants_asset_idx ON artifact_variants (asset_id, variant_kind);
