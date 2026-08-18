package persistence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/artifact"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

// WorkerArtifactCommitter is intentionally a narrow first-slice adapter. It
// accepts only the zero-byte deterministic analysis report; media workers must
// stage real bytes through a storage-aware executor and never put them on the
// Redis result stream.
type WorkerArtifactCommitter struct {
	Storage    storage.StoragePort
	Repository *SQLArtifactRepository
}

func (c WorkerArtifactCommitter) CommitWorkerArtifact(ctx context.Context, workspaceID, projectID, jobID, stepID string, ref worker.OutputRef) (worker.OutputRef, error) {
	if c.Storage == nil || c.Repository == nil {
		return worker.OutputRef{}, errors.New("worker Artifact commit dependencies are required")
	}
	if ref.SizeBytes != 0 || ref.SHA256 != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		return worker.OutputRef{}, errors.New("worker first-slice Artifact must be the empty deterministic report")
	}
	if workspaceID == "" || projectID == "" || jobID == "" || stepID == "" {
		return worker.OutputRef{}, errors.New("worker Artifact scope is incomplete")
	}
	finalKey := fmt.Sprintf("jobs/%s/steps/%s/outputs/%s", safeKey(jobID), safeKey(stepID), safeKey(ref.ArtifactID))
	staged, err := c.Storage.PutStaged(ctx, storage.Scope{WorkspaceID: workspaceID, ProjectID: projectID}, bytes.NewReader(nil), "application/json")
	if err != nil {
		return worker.OutputRef{}, err
	}
	committed, err := (artifact.CommitService{Storage: c.Storage, Repository: c.Repository}).Commit(ctx, artifact.CommitRequest{ProjectID: projectID, Kind: ref.Kind, Role: ref.Role, FinalKey: finalKey, ExpectedSHA256: ref.SHA256, Staged: artifact.NewStaged(staged), ContentType: "application/json", Metadata: map[string]any{"worker_artifact_id": ref.ArtifactID}, IfNoneMatch: true})
	if errors.Is(err, artifact.ErrDuplicateCanonicalRole) {
		if committed.ID == "" {
			return worker.OutputRef{}, err
		}
	} else if err != nil {
		return worker.OutputRef{}, err
	}
	if err := c.Repository.LinkProducer(ctx, committed.ID, projectID, jobID, stepID); err != nil {
		return worker.OutputRef{}, err
	}
	return worker.OutputRef{ArtifactID: contractArtifactID(committed.ID), Kind: committed.Kind, Role: committed.Role, SHA256: committed.SHA256, SizeBytes: committed.SizeBytes}, nil
}

func (r *SQLArtifactRepository) LinkProducer(ctx context.Context, artifactID, projectID, jobID, stepID string) error {
	if r == nil || r.SQL == nil || r.SQL.DB == nil {
		return errors.New("Artifact repository database is required")
	}
	result, err := r.SQL.DB.ExecContext(ctx, `UPDATE artifacts SET producer_job_id=$3::uuid,producer_job_step_id=$4::uuid,created_by_service=NULL,updated_at=now() WHERE id=$1::uuid AND project_id=$2::uuid AND status='committed'`, artifactID, projectID, jobID, stepID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return ErrNotFound
	}
	return nil
}

func contractArtifactID(value string) string {
	return "artifact_" + strings.ReplaceAll(strings.ReplaceAll(value, "-", "_"), ".", "_")
}

func safeKey(value string) string {
	value = strings.TrimSpace(value)
	value = strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
