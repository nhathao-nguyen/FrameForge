package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/product"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

// PresignArtifactDownload authorizes the project-owned committed Artifact and
// delegates byte delivery to StoragePort. Worker paths and raw object keys are
// never returned to the caller.
func (b *DurableBackend) PresignArtifactDownload(ctx context.Context, workspaceID, projectID, artifactID string) (product.ArtifactDownload, error) {
	if workspaceID != b.Workspace || b.Storage == nil {
		return product.ArtifactDownload{}, product.ErrNotFound
	}
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return product.ArtifactDownload{}, err
	}
	var locator storage.StorageLocator
	if err := b.SQL.DB.QueryRowContext(ctx, `SELECT storage_backend,object_key,COALESCE(object_version,'') FROM artifacts WHERE id=$1::uuid AND project_id=$2::uuid AND status='committed'`, artifactID, projectID).Scan(&locator.Backend, &locator.ObjectKey, &locator.ObjectVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return product.ArtifactDownload{}, product.ErrNotFound
		}
		return product.ArtifactDownload{}, err
	}
	signed, err := b.Storage.PresignDownload(ctx, locator, 5*time.Minute, "attachment")
	if err != nil {
		return product.ArtifactDownload{}, fmt.Errorf("presign artifact download: %w", err)
	}
	if signed.URL == "" || signed.ExpiresAt.IsZero() {
		return product.ArtifactDownload{}, errors.New("storage returned an incomplete signed download")
	}
	return product.ArtifactDownload{ArtifactID: artifactID, URL: signed.URL, ExpiresAt: signed.ExpiresAt}, nil
}
