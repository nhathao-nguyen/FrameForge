-- T207/T208: versioned content, intelligence, canonical TimelineVersion and Render records.
CREATE TABLE scripts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    label text,
    status text NOT NULL CHECK (status IN ('active','archived','deleted')),
    current_version_id uuid,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX scripts_project_idx ON scripts (project_id, status, updated_at DESC);

CREATE TABLE script_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id uuid NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    version integer NOT NULL CHECK (version > 0),
    status text NOT NULL CHECK (status IN ('draft','proposed','approved','superseded')),
    language text NOT NULL,
    origin text NOT NULL CHECK (origin IN ('ai','user','imported','system')),
    content jsonb NOT NULL,
    content_hash char(64) NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
    based_on_version_id uuid REFERENCES script_versions(id) ON DELETE RESTRICT,
    produced_by_job_step_id uuid REFERENCES job_steps(id) ON DELETE RESTRICT,
    created_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    approved_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    approved_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (script_id, version),
    UNIQUE (script_id, content_hash)
);
CREATE INDEX script_versions_status_idx ON script_versions (script_id, status, version DESC);

CREATE TABLE script_reviews (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    script_version_id uuid NOT NULL REFERENCES script_versions(id) ON DELETE RESTRICT,
    decision text NOT NULL CHECK (decision IN ('approved','rejected','changes_requested')),
    actor_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    comment text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE narrations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    script_version_id uuid NOT NULL REFERENCES script_versions(id) ON DELETE RESTRICT,
    status text NOT NULL CHECK (status IN ('created','synthesizing','ready','failed','superseded')),
    provider_configuration_id uuid REFERENCES provider_configurations(id) ON DELETE RESTRICT,
    voice_snapshot jsonb NOT NULL,
    request_snapshot jsonb NOT NULL,
    audio_artifact_id uuid REFERENCES artifacts(id) ON DELETE RESTRICT,
    timing_artifact_id uuid REFERENCES artifacts(id) ON DELETE RESTRICT,
    duration_sec numeric(14,3) CHECK (duration_sec IS NULL OR duration_sec >= 0),
    produced_by_job_step_id uuid REFERENCES job_steps(id) ON DELETE RESTRICT,
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX narrations_script_idx ON narrations (project_id, script_version_id, created_at DESC);

CREATE TABLE scenes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    source_asset_id uuid NOT NULL REFERENCES assets(id) ON DELETE RESTRICT,
    source_artifact_id uuid NOT NULL REFERENCES artifacts(id) ON DELETE RESTRICT,
    detection_revision text NOT NULL,
    scene_index integer NOT NULL CHECK (scene_index >= 0),
    status text NOT NULL CHECK (status IN ('detected','analyzed','confirmed','rejected','superseded')),
    source_start_sec numeric(14,3) NOT NULL CHECK (source_start_sec >= 0),
    source_end_sec numeric(14,3) NOT NULL CHECK (source_end_sec > source_start_sec),
    description text,
    dialogue text,
    location text,
    emotion text,
    actions jsonb,
    quality_score numeric(5,4) CHECK (quality_score IS NULL OR quality_score BETWEEN 0 AND 1),
    embedding_artifact_id uuid REFERENCES artifacts(id) ON DELETE RESTRICT,
    thumbnail_artifact_id uuid REFERENCES artifacts(id) ON DELETE RESTRICT,
    attributes jsonb,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    produced_by_job_step_id uuid REFERENCES job_steps(id) ON DELETE RESTRICT,
    updated_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_artifact_id, detection_revision, scene_index)
);
CREATE INDEX scenes_project_idx ON scenes (project_id, status);

CREATE TABLE characters (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    display_name text,
    status text NOT NULL CHECK (status IN ('unconfirmed','confirmed','rejected','merged')),
    merged_into_character_id uuid REFERENCES characters(id) ON DELETE RESTRICT,
    confidence numeric(5,4) CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1),
    aliases jsonb NOT NULL DEFAULT '[]'::jsonb,
    profile_artifact_id uuid REFERENCES artifacts(id) ON DELETE RESTRICT,
    attributes jsonb,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    produced_by_job_step_id uuid REFERENCES job_steps(id) ON DELETE RESTRICT,
    updated_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX characters_project_idx ON characters (project_id, status);

CREATE TABLE character_appearances (
    character_id uuid NOT NULL REFERENCES characters(id) ON DELETE RESTRICT,
    scene_id uuid NOT NULL REFERENCES scenes(id) ON DELETE RESTRICT,
    confidence numeric(5,4) CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1),
    face_count integer CHECK (face_count IS NULL OR face_count >= 0),
    boxes jsonb,
    embedding_artifact_id uuid REFERENCES artifacts(id) ON DELETE RESTRICT,
    confirmed_by_user_id uuid REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, scene_id)
);

CREATE TABLE timelines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    label text,
    status text NOT NULL CHECK (status IN ('active','archived','deleted')),
    current_version_id uuid,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX timelines_project_idx ON timelines (project_id, status, updated_at DESC);

CREATE TABLE timeline_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    timeline_id uuid NOT NULL REFERENCES timelines(id) ON DELETE CASCADE,
    version integer NOT NULL CHECK (version > 0),
    status text NOT NULL CHECK (status IN ('draft','proposed','approved','locked','superseded')),
    schema_version text NOT NULL,
    document jsonb NOT NULL,
    content_hash char(64) NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
    origin text NOT NULL CHECK (origin IN ('ai','user','imported','system')),
    based_on_version_id uuid REFERENCES timeline_versions(id) ON DELETE RESTRICT,
    produced_by_job_step_id uuid REFERENCES job_steps(id) ON DELETE RESTRICT,
    created_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    approved_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    approved_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (timeline_id, version),
    UNIQUE (timeline_id, content_hash)
);
CREATE INDEX timeline_versions_status_idx ON timeline_versions (timeline_id, status, version DESC);

CREATE TABLE renders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    timeline_version_id uuid NOT NULL REFERENCES timeline_versions(id) ON DELETE RESTRICT,
    render_profile_id uuid NOT NULL REFERENCES render_profiles(id) ON DELETE RESTRICT,
    supersedes_render_id uuid REFERENCES renders(id) ON DELETE RESTRICT,
    profile_snapshot jsonb NOT NULL,
    status text NOT NULL CHECK (status IN ('created','queued','running','completed','failed','cancelled')),
    job_id uuid UNIQUE,
    request_hash char(64),
    request jsonb NOT NULL,
    qa_summary jsonb,
    error_code text,
    error_message text,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE UNIQUE INDEX renders_completed_dedupe_uq ON renders (timeline_version_id, render_profile_id, request_hash) WHERE status = 'completed';
CREATE INDEX renders_project_idx ON renders (project_id, created_at DESC);

CREATE TABLE render_artifacts (
    render_id uuid NOT NULL REFERENCES renders(id) ON DELETE CASCADE,
    artifact_id uuid NOT NULL REFERENCES artifacts(id) ON DELETE RESTRICT,
    role text NOT NULL CHECK (role IN ('video','audio','subtitle','thumbnail','qa_report','metadata')),
    is_canonical boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (render_id, artifact_id, role)
);
CREATE UNIQUE INDEX render_artifact_canonical_uq ON render_artifacts (render_id, role) WHERE is_canonical;

CREATE TABLE analyses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    kind text NOT NULL,
    status text NOT NULL CHECK (status IN ('created','running','completed','failed','cancelled')),
    input_refs jsonb NOT NULL,
    schema_version text NOT NULL,
    result jsonb,
    result_artifact_id uuid REFERENCES artifacts(id) ON DELETE RESTRICT,
    producer_job_step_id uuid REFERENCES job_steps(id) ON DELETE RESTRICT,
    provenance jsonb NOT NULL,
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE INDEX analyses_project_kind_idx ON analyses (project_id, kind, created_at DESC);

CREATE TABLE candidate_selection_policies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid REFERENCES workspaces(id) ON DELETE RESTRICT,
    policy_key text NOT NULL,
    version integer NOT NULL CHECK (version > 0),
    status text NOT NULL CHECK (status IN ('draft','active','deprecated','disabled')),
    document jsonb NOT NULL,
    content_hash char(64) NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
    created_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, policy_key, version),
    UNIQUE (workspace_id, policy_key, content_hash)
);

CREATE TABLE generation_candidates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    job_step_id uuid NOT NULL REFERENCES job_steps(id) ON DELETE RESTRICT,
    candidate_group_id uuid NOT NULL,
    status text NOT NULL CHECK (status IN ('created','generated','evaluated','selected','rejected','superseded')),
    rank integer CHECK (rank IS NULL OR rank > 0),
    resource_type text NOT NULL,
    resource_id uuid NOT NULL,
    payload jsonb NOT NULL,
    provenance jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX candidates_group_idx ON generation_candidates (project_id, candidate_group_id);

CREATE TABLE evaluation_results (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    candidate_id uuid NOT NULL REFERENCES generation_candidates(id) ON DELETE RESTRICT,
    policy_id uuid REFERENCES candidate_selection_policies(id) ON DELETE RESTRICT,
    status text NOT NULL CHECK (status IN ('created','completed','failed')),
    component_scores jsonb NOT NULL,
    explanation_artifact_id uuid REFERENCES artifacts(id) ON DELETE RESTRICT,
    evaluator_snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE scripts ADD CONSTRAINT scripts_current_version_fk FOREIGN KEY (current_version_id) REFERENCES script_versions(id) ON DELETE RESTRICT;
ALTER TABLE timelines ADD CONSTRAINT timelines_current_version_fk FOREIGN KEY (current_version_id) REFERENCES timeline_versions(id) ON DELETE RESTRICT;
ALTER TABLE projects ADD CONSTRAINT projects_current_script_fk FOREIGN KEY (current_script_version_id) REFERENCES script_versions(id) ON DELETE RESTRICT;
ALTER TABLE projects ADD CONSTRAINT projects_current_timeline_fk FOREIGN KEY (current_timeline_version_id) REFERENCES timeline_versions(id) ON DELETE RESTRICT;
