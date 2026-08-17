package persistence

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/artifact"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

// SQLArtifactRepository persists only immutable Artifact metadata. Bytes stay
// in StoragePort and the repository never accepts or returns a local path.
type SQLArtifactRepository struct {
	SQL       *SQLStore
	UserID    string
	Workspace string
}

func NewSQLArtifactRepository(sqlStore *SQLStore, userID, workspaceID string) (*SQLArtifactRepository, error) {
	if sqlStore == nil || userID == "" || workspaceID == "" {
		return nil, errors.New("artifact repository requires SQL store, user and Workspace")
	}
	return &SQLArtifactRepository{SQL: sqlStore, UserID: userID, Workspace: workspaceID}, nil
}

func (r *SQLArtifactRepository) FindCanonicalRole(ctx context.Context, projectID, role string) (artifact.Artifact, bool, error) {
	if _, err := r.SQL.GetProject(ctx, r.UserID, r.Workspace, projectID); err != nil {
		return artifact.Artifact{}, false, err
	}
	var value artifact.Artifact
	var backend, objectKey string
	var objectVersion, contentType sql.NullString
	var metadata []byte
	var size int64
	var committedAtValue sql.NullTime
	err := r.SQL.DB.QueryRowContext(ctx, `SELECT id::text,project_id::text,kind,role,status,storage_backend,object_key,COALESCE(object_version,''),COALESCE(content_type,''),size_bytes,sha256,metadata,committed_at FROM artifacts WHERE project_id=$1 AND role=$2 AND status='committed' ORDER BY committed_at DESC NULLS LAST,created_at DESC LIMIT 1`, projectID, role).Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Role, &value.Status, &backend, &objectKey, &objectVersion, &contentType, &size, &value.SHA256, &metadata, &committedAtValue)
	if errors.Is(err, sql.ErrNoRows) {
		return artifact.Artifact{}, false, nil
	}
	if err != nil {
		return artifact.Artifact{}, false, err
	}
	value.Locator = storage.StorageLocator{Backend: backend, ObjectKey: objectKey}
	if objectVersion.Valid {
		value.Locator.ObjectVersion = objectVersion.String
	}
	value.SizeBytes = size
	if contentType.Valid {
		value.ContentType = contentType.String
	}
	value.Metadata = mapFromJSON(metadata)
	if committedAtValue.Valid {
		committedAt := committedAtValue.Time
		value.CommittedAt = &committedAt
	}
	return value, true, nil
}

func (r *SQLArtifactRepository) PublishCommitted(ctx context.Context, input artifact.Artifact) (artifact.Artifact, error) {
	if _, err := r.SQL.GetProject(ctx, r.UserID, r.Workspace, input.ProjectID); err != nil {
		return artifact.Artifact{}, err
	}
	if input.Status != "committed" || input.Locator.ObjectKey == "" || input.SHA256 == "" {
		return artifact.Artifact{}, errors.New("committed artifact metadata is incomplete")
	}
	metadata := jsonBytes(input.Metadata)
	var value artifact.Artifact
	var objectVersion, contentType sql.NullString
	var metadataJSON []byte
	var committedAt sql.NullTime
	err := r.SQL.DB.QueryRowContext(ctx, `INSERT INTO artifacts(project_id,kind,role,status,storage_backend,object_key,object_version,content_type,size_bytes,sha256,metadata,created_by_user_id,committed_at) VALUES($1,$2,$3,'committed',$4,$5,NULLIF($6,''),NULLIF($7,''),$8,$9,$10,$11,now()) RETURNING id::text,project_id::text,kind,role,status,storage_backend,object_key,object_version,content_type,size_bytes,sha256,metadata,committed_at`, input.ProjectID, input.Kind, input.Role, input.Locator.Backend, input.Locator.ObjectKey, input.Locator.ObjectVersion, input.ContentType, input.SizeBytes, input.SHA256, metadata, r.UserID).Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Role, &value.Status, &value.Locator.Backend, &value.Locator.ObjectKey, &objectVersion, &contentType, &value.SizeBytes, &value.SHA256, &metadataJSON, &committedAt)
	if err != nil {
		return artifact.Artifact{}, err
	}
	if objectVersion.Valid {
		value.Locator.ObjectVersion = objectVersion.String
	}
	if contentType.Valid {
		value.ContentType = contentType.String
	}
	value.Metadata = mapFromJSON(metadataJSON)
	if committedAt.Valid {
		committed := committedAt.Time
		value.CommittedAt = &committed
	}
	return value, nil
}

func (r *SQLArtifactRepository) MarkOrphan(ctx context.Context, locator storage.StorageLocator, reason string) error {
	_, err := r.SQL.DB.ExecContext(ctx, `INSERT INTO artifact_reconciliation(storage_backend,object_key,object_version,reason,status) VALUES($1,$2,NULLIF($3,''),$4,'pending')`, locator.Backend, locator.ObjectKey, locator.ObjectVersion, reason)
	return err
}
