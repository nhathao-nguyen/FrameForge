-- T206/T222: durable record for objects promoted before a DB publish failure.
CREATE TABLE artifact_reconciliation (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    storage_backend text NOT NULL CHECK (storage_backend IN ('local','s3','minio')),
    object_key text NOT NULL,
    object_version text,
    reason text NOT NULL,
    status text NOT NULL CHECK (status IN ('pending','reconciled','retained','failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    reconciled_at timestamptz
);
CREATE INDEX artifact_reconciliation_ready_idx ON artifact_reconciliation (status, created_at);
CREATE UNIQUE INDEX artifact_reconciliation_locator_uq ON artifact_reconciliation (storage_backend, object_key, COALESCE(object_version, ''));
