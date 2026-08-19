package persistence

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/artifact"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

// WorkerArtifactCommitter keeps result-stream metadata separate from payload
// bytes. Zero-byte deterministic reports use a local empty stage; all other
// outputs must already be verified in the worker staging namespace before the
// committer promotes them to a canonical Artifact.
type WorkerArtifactCommitter struct {
	Storage    storage.StoragePort
	Repository *SQLArtifactRepository
}

func (c WorkerArtifactCommitter) CommitWorkerArtifact(ctx context.Context, workspaceID, projectID, jobID, stepID, messageID string, ref worker.OutputRef) (worker.OutputRef, error) {
	if c.Storage == nil || c.Repository == nil {
		return worker.OutputRef{}, errors.New("worker Artifact commit dependencies are required")
	}
	if workspaceID == "" || projectID == "" || jobID == "" || stepID == "" {
		return worker.OutputRef{}, errors.New("worker Artifact scope is incomplete")
	}
	contentType := strings.TrimSpace(ref.ContentType)
	if contentType == "" {
		contentType = "application/json"
	}
	var staged storage.StagedObject
	var err error
	if ref.SizeBytes == 0 && ref.SHA256 == "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		staged, err = c.Storage.PutStaged(ctx, storage.Scope{WorkspaceID: workspaceID, ProjectID: projectID}, bytes.NewReader(nil), contentType)
	} else {
		transfer, ok := c.Storage.(storage.WorkerTransferPort)
		if !ok {
			return worker.OutputRef{}, errors.New("worker staged Artifact transfer is unavailable")
		}
		stageKey, keyErr := storage.WorkerStageKey(contractID("workspace", workspaceID), contractID("project", projectID), messageID, ref.ArtifactID)
		if keyErr != nil {
			return worker.OutputRef{}, keyErr
		}
		staged, err = transfer.VerifyWorkerStage(ctx, stageKey, ref.SHA256, ref.SizeBytes, contentType)
	}
	if err != nil {
		return worker.OutputRef{}, err
	}
	finalKey := fmt.Sprintf("workspaces/%s/projects/%s/jobs/%s/steps/%s/outputs/%s", safeKey(workspaceID), safeKey(projectID), safeKey(jobID), safeKey(stepID), safeKey(ref.ArtifactID))
	// Worker outputs are immutable per job step, not one canonical role for the
	// entire project.  A project can legitimately contain 16:9, 9:16 and 1:1
	// renders with the same output roles.  The final object key remains scoped
	// to job/step/output and is still promoted with IfNoneMatch, so a replay of
	// the same worker message is idempotent without aliasing a different render
	// to the first project's canonical role.
	committed, err := (artifact.CommitService{Storage: c.Storage, Repository: c.Repository}).Commit(ctx, artifact.CommitRequest{ProjectID: projectID, Kind: ref.Kind, Role: ref.Role, FinalKey: finalKey, ExpectedSHA256: ref.SHA256, Staged: artifact.NewStaged(staged), ContentType: contentType, Metadata: map[string]any{"worker_artifact_id": ref.ArtifactID, "worker_message_id": messageID}, IfNoneMatch: false})
	if err != nil {
		// If the worker result was committed before the controller crashed, the
		// same scoped final key is already authoritative.  Reuse it only after
		// checking project, status and checksum; unrelated commit failures still
		// propagate to the reconciler.
		if existing, foundErr := c.findCommittedWorkerOutput(ctx, projectID, finalKey, ref.SHA256); foundErr == nil {
			committed = existing
			err = nil
		}
	}
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
	return worker.OutputRef{ArtifactID: contractArtifactID(committed.ID), Kind: committed.Kind, Role: committed.Role, SHA256: committed.SHA256, SizeBytes: committed.SizeBytes, ContentType: committed.ContentType}, nil
}

func (c WorkerArtifactCommitter) findCommittedWorkerOutput(ctx context.Context, projectID, objectKey, checksum string) (artifact.Artifact, error) {
	if c.Repository == nil || c.Repository.SQL == nil || c.Repository.SQL.DB == nil {
		return artifact.Artifact{}, errors.New("Artifact repository database is required")
	}
	var value artifact.Artifact
	var objectVersion, contentType sql.NullString
	err := c.Repository.SQL.DB.QueryRowContext(ctx, `SELECT id::text,project_id::text,kind,role,status,storage_backend,object_key,object_version,content_type,size_bytes,sha256 FROM artifacts WHERE project_id=$1::uuid AND object_key=$2 AND status='committed' AND sha256=$3 LIMIT 1`, projectID, objectKey, checksum).Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Role, &value.Status, &value.Locator.Backend, &value.Locator.ObjectKey, &objectVersion, &contentType, &value.SizeBytes, &value.SHA256)
	if err != nil {
		return artifact.Artifact{}, err
	}
	if objectVersion.Valid {
		value.Locator.ObjectVersion = objectVersion.String
	}
	if contentType.Valid {
		value.ContentType = contentType.String
	}
	return value, nil
}

// FinalizeValidatedUpload promotes the staged source Artifact only after the
// probe has completed successfully. The worker receives a staged ref through
// the lease-scoped command, while Product state exposes the source only after
// this promotion and the Asset pointer update succeed.
func (c WorkerArtifactCommitter) FinalizeValidatedUpload(ctx context.Context, workspaceID, projectID, jobID string, command worker.Command, result worker.Result) error {
	if c.Storage == nil || c.Repository == nil || c.Repository.SQL == nil || c.Repository.SQL.DB == nil {
		return errors.New("validated upload commit dependencies are required")
	}
	var ref worker.ArtifactRef
	for _, candidate := range command.InputRefs {
		if candidate.Role == "source_original" || candidate.Role == "source" {
			ref = candidate
			break
		}
	}
	artifactID, ok := contractUUID("artifact", ref.ArtifactID)
	if !ok || ref.SHA256 == "" {
		return errors.New("validated upload source Artifact ref is invalid")
	}
	var assetID string
	if err := c.Repository.SQL.DB.QueryRowContext(ctx, `SELECT command->>'asset_id' FROM jobs WHERE id=$1::uuid AND project_id=$2::uuid`, jobID, projectID).Scan(&assetID); err != nil {
		return err
	}
	var staged storage.StagedObject
	var kind, role, contentType string
	var size int64
	var checksum string
	if err := c.Repository.SQL.DB.QueryRowContext(ctx, `SELECT a.kind,a.role,a.storage_backend,a.object_key,COALESCE(a.object_version,''),COALESCE(a.content_type,''),a.size_bytes,a.sha256 FROM artifacts a JOIN projects p ON p.id=a.project_id WHERE p.workspace_id=$1::uuid AND a.project_id=$2::uuid AND a.id=$3::uuid AND a.status='staged'`, workspaceID, projectID, artifactID).Scan(&kind, &role, &staged.Locator.Backend, &staged.Locator.ObjectKey, &staged.Locator.ObjectVersion, &contentType, &size, &checksum); err != nil {
		return err
	}
	if checksum != ref.SHA256 {
		return errors.New("validated upload source checksum does not match the command")
	}
	staged.SizeBytes, staged.SHA256, staged.ContentType = size, checksum, contentType
	finalKey := fmt.Sprintf("workspaces/%s/projects/%s/artifacts/source_original_%s", safeKey(workspaceID), safeKey(projectID), safeKey(assetID))
	committed, err := (artifact.CommitService{Storage: c.Storage, Repository: c.Repository}).Commit(ctx, artifact.CommitRequest{
		ProjectID: projectID, Kind: kind, Role: role, FinalKey: finalKey,
		ExpectedSHA256: checksum, Staged: artifact.NewStaged(staged), ContentType: contentType,
		Metadata: map[string]any{"source": "upload_validation", "validation_job_id": jobID}, IfNoneMatch: true,
	})
	if errors.Is(err, artifact.ErrDuplicateCanonicalRole) {
		if committed.ID == "" {
			return err
		}
	} else if err != nil {
		return err
	}
	validationReport, err := json.Marshal(map[string]any{"status": "validated", "probe_output_refs": result.OutputRefs})
	if err != nil {
		return err
	}
	tx, err := c.Repository.SQL.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	updated, err := tx.ExecContext(ctx, `UPDATE assets SET status='ready',original_artifact_id=$3::uuid,detected_mime_type=NULLIF($4,''),sha256=$5,size_bytes=$6,validation_report=$7,revision=revision+1,updated_at=now() WHERE id=$1::uuid AND project_id=$2::uuid AND status='validating'`, assetID, projectID, committed.ID, contentType, checksum, size, validationReport)
	if err != nil {
		return err
	}
	if count, err := updated.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO artifact_variants(asset_id,artifact_id,variant_kind,is_canonical,metadata) VALUES($1::uuid,$2::uuid,'original',true,'{"source":"upload_validation"}'::jsonb) ON CONFLICT (asset_id,artifact_id,variant_kind) DO UPDATE SET is_canonical=true`, assetID, committed.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE artifacts SET status='expired',updated_at=now() WHERE id=$1::uuid AND status='staged'`, artifactID); err != nil {
		return err
	}
	return tx.Commit()
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
