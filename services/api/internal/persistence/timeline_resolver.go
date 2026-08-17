package persistence

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/product"
)

// DurableTimelineReferenceResolver resolves every Timeline reference from the
// PostgreSQL source of truth. IDs supplied by a client are never accepted
// without checking Workspace/Project ownership and the referenced lifecycle.
type DurableTimelineReferenceResolver struct {
	SQL       *SQLStore
	Workspace string
}

func (r DurableTimelineReferenceResolver) ResolveAssetArtifact(assetID, artifactID, projectID string) (bool, float64, int, int, error) {
	if r.SQL == nil || r.SQL.DB == nil {
		return false, 0, 0, 0, product.ErrNotFound
	}
	var duration float64
	var width, height int
	err := r.SQL.DB.QueryRowContext(context.Background(), `
SELECT COALESCE(a.duration_sec, 0)::double precision, COALESCE(a.width, 0), COALESCE(a.height, 0)
FROM assets a
JOIN projects p ON p.id = a.project_id
JOIN artifacts ar ON ar.id = a.original_artifact_id
WHERE a.id = $1 AND a.project_id = $2 AND p.workspace_id = $3
  AND a.original_artifact_id = $4 AND a.status = 'ready' AND ar.project_id = a.project_id
  AND ar.status = 'committed'`, assetID, projectID, r.Workspace, artifactID).Scan(&duration, &width, &height)
	if errors.Is(err, sql.ErrNoRows) {
		return false, 0, 0, 0, nil
	}
	if err != nil {
		return false, 0, 0, 0, err
	}
	return true, duration, width, height, nil
}

func (r DurableTimelineReferenceResolver) ResolveScene(sceneID, assetID, artifactID, projectID string) (bool, float64, float64, error) {
	if r.SQL == nil || r.SQL.DB == nil {
		return false, 0, 0, product.ErrNotFound
	}
	var start, end float64
	err := r.SQL.DB.QueryRowContext(context.Background(), `
SELECT s.source_start_sec::double precision, s.source_end_sec::double precision
FROM scenes s
JOIN projects p ON p.id = s.project_id
JOIN assets a ON a.id = s.source_asset_id
JOIN artifacts ar ON ar.id = s.source_artifact_id
WHERE s.id = $1 AND s.project_id = $2 AND p.workspace_id = $3
  AND s.source_asset_id = $4 AND s.source_artifact_id = $5
  AND s.status IN ('detected', 'analyzed', 'confirmed')
  AND a.status = 'ready' AND ar.status = 'committed'`, sceneID, projectID, r.Workspace, assetID, artifactID).Scan(&start, &end)
	if errors.Is(err, sql.ErrNoRows) {
		return false, 0, 0, nil
	}
	if err != nil {
		return false, 0, 0, err
	}
	return true, start, end, nil
}

func (r DurableTimelineReferenceResolver) ResolveArtifact(artifactID, projectID string) (bool, error) {
	if r.SQL == nil || r.SQL.DB == nil {
		return false, product.ErrNotFound
	}
	var present int
	err := r.SQL.DB.QueryRowContext(context.Background(), `
SELECT 1
FROM artifacts ar
JOIN projects p ON p.id = ar.project_id
WHERE ar.id = $1 AND ar.project_id = $2 AND p.workspace_id = $3 AND ar.status = 'committed'`, artifactID, projectID, r.Workspace).Scan(&present)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return present == 1, nil
}

func (r DurableTimelineReferenceResolver) ResolveNarration(narrationID, scriptVersionID, projectID string) (bool, error) {
	if r.SQL == nil || r.SQL.DB == nil {
		return false, product.ErrNotFound
	}
	var present int
	err := r.SQL.DB.QueryRowContext(context.Background(), `
SELECT 1
FROM narrations n
JOIN projects p ON p.id = n.project_id
JOIN script_versions sv ON sv.id = n.script_version_id
JOIN artifacts ar ON ar.id = n.audio_artifact_id
WHERE n.id = $1 AND n.project_id = $2 AND p.workspace_id = $3
  AND n.script_version_id = $4 AND n.status = 'ready' AND sv.project_id = n.project_id
  AND sv.status = 'approved' AND ar.project_id = n.project_id AND ar.status = 'committed'`, narrationID, projectID, r.Workspace, scriptVersionID).Scan(&present)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return present == 1, nil
}

func (b *DurableBackend) durableTimelineResolver() DurableTimelineReferenceResolver {
	return DurableTimelineReferenceResolver{SQL: b.SQL, Workspace: b.Workspace}
}

func (b *DurableBackend) ResolveAssetArtifact(assetID, artifactID, projectID string) (bool, float64, int, int, error) {
	return b.durableTimelineResolver().ResolveAssetArtifact(assetID, artifactID, projectID)
}

func (b *DurableBackend) ResolveScene(sceneID, assetID, artifactID, projectID string) (bool, float64, float64, error) {
	return b.durableTimelineResolver().ResolveScene(sceneID, assetID, artifactID, projectID)
}

func (b *DurableBackend) ResolveArtifact(artifactID, projectID string) (bool, error) {
	return b.durableTimelineResolver().ResolveArtifact(artifactID, projectID)
}

func (b *DurableBackend) ResolveNarration(narrationID, scriptVersionID, projectID string) (bool, error) {
	return b.durableTimelineResolver().ResolveNarration(narrationID, scriptVersionID, projectID)
}
