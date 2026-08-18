package persistence

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

// WorkerTransfer resolves committed Artifact refs and prepares direct staged
// uploads. Signed URLs exist only in this internal response and never enter a
// Job, checkpoint, event, queue command or durable domain record.
type WorkerTransfer struct {
	SQL       *SQLStore
	Storage   storage.StoragePort
	Workspace string
}

func (t WorkerTransfer) ResolveWorkerArtifact(ctx context.Context, projectRef, artifactRef, expectedSHA string) (storage.SignedRequest, error) {
	if t.SQL == nil || t.Storage == nil || t.Workspace == "" {
		return storage.SignedRequest{}, errors.New("worker transfer dependencies are required")
	}
	projectID, ok := contractUUID("project", projectRef)
	if !ok {
		return storage.SignedRequest{}, ErrNotFound
	}
	artifactID, ok := contractUUID("artifact", artifactRef)
	if !ok {
		return storage.SignedRequest{}, ErrNotFound
	}
	var locator storage.StorageLocator
	var checksum string
	err := t.SQL.DB.QueryRowContext(ctx, `SELECT a.storage_backend,a.object_key,COALESCE(a.object_version,''),a.sha256
		FROM artifacts a JOIN projects p ON p.id=a.project_id
		WHERE p.workspace_id=$1::uuid AND p.id=$2::uuid AND a.id=$3::uuid
		  AND (a.status='committed' OR (a.status='staged' AND EXISTS (
			SELECT 1 FROM job_steps s
			JOIN pipeline_runs r ON r.id=s.pipeline_run_id
			JOIN jobs j ON j.id=r.job_id
			WHERE j.project_id=a.project_id AND j.kind='asset_probe'
			  AND j.status IN ('created','queued','running') AND s.status IN ('pending','ready','queued','running')
			  AND s.input_refs @> jsonb_build_object('artifacts', jsonb_build_array(jsonb_build_object('artifact_id',$4::text,'role','source_original','sha256',$5::text)))
		 )))`, t.Workspace, projectID, artifactID, contractArtifactID(artifactID), expectedSHA).Scan(&locator.Backend, &locator.ObjectKey, &locator.ObjectVersion, &checksum)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (expectedSHA == "" || checksum != expectedSHA) {
		return storage.SignedRequest{}, ErrNotFound
	}
	if err != nil {
		return storage.SignedRequest{}, err
	}
	return t.Storage.PresignDownload(ctx, locator, 5*time.Minute, "attachment")
}

func (t WorkerTransfer) PrepareWorkerStage(ctx context.Context, workspaceRef, projectRef, jobRef, stepRef, messageID, artifactID, contentType string) (storage.SignedRequest, error) {
	transfer, ok := t.Storage.(storage.WorkerTransferPort)
	if t.SQL == nil || !ok || t.Workspace == "" {
		return storage.SignedRequest{}, errors.New("worker staged transfer is unavailable")
	}
	workspaceID, workspaceOK := contractUUID("workspace", workspaceRef)
	projectID, projectOK := contractUUID("project", projectRef)
	jobID, jobOK := contractUUID("job", jobRef)
	stepID, stepOK := contractUUID("step", stepRef)
	if !workspaceOK || !projectOK || !jobOK || !stepOK || workspaceID != t.Workspace {
		return storage.SignedRequest{}, ErrNotFound
	}
	var active bool
	err := t.SQL.DB.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM job_steps s
		JOIN pipeline_runs r ON r.id=s.pipeline_run_id
		JOIN jobs j ON j.id=r.job_id
		JOIN projects p ON p.id=j.project_id
		WHERE p.workspace_id=$1::uuid AND p.id=$2::uuid AND j.id=$3::uuid AND s.id=$4::uuid
		  AND s.status='running' AND EXISTS (SELECT 1 FROM job_step_attempts a WHERE a.job_step_id=s.id AND a.attempt=s.current_attempt AND a.status='running')
	)`, t.Workspace, projectID, jobID, stepID).Scan(&active)
	if err != nil || !active {
		if err != nil {
			return storage.SignedRequest{}, err
		}
		return storage.SignedRequest{}, ErrNotFound
	}
	key, err := storage.WorkerStageKey(workspaceRef, projectRef, messageID, artifactID)
	if err != nil {
		return storage.SignedRequest{}, err
	}
	return transfer.PresignWorkerUpload(ctx, key, 5*time.Minute, contentType)
}

func contractUUID(prefix, value string) (string, bool) {
	prefix += "_"
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	raw := strings.TrimPrefix(value, prefix)
	parts := strings.Split(raw, "_")
	if len(parts) != 5 || len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		return "", false
	}
	for _, part := range parts {
		for _, value := range part {
			if !((value >= '0' && value <= '9') || (value >= 'a' && value <= 'f')) {
				return "", false
			}
		}
	}
	return strings.Join(parts, "-"), true
}
