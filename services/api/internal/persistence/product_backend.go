package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/product"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/queue"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

// DurableBackend is the Product API adapter for PostgreSQL-backed resources.
// The embedded Store remains only as an explicit fallback for resource methods
// that have not yet been moved to SQL; Project, Asset and upload lifecycles are
// never routed through that fallback when this adapter is installed.
type DurableBackend struct {
	*product.Store
	SQL          *SQLStore
	UserID       string
	Workspace    string
	Storage      storage.StoragePort
	Queue        queue.QueuePort
	providerMu   sync.Mutex
	providerData map[string]storage.UploadSession
}

// SetQueue attaches the optional Redis transport after durable bootstrap. The
// database remains authoritative when it is nil or temporarily unavailable;
// queued steps can be reconciled after a transport outage.
func (b *DurableBackend) SetQueue(value queue.QueuePort) { b.Queue = value }

func NewDurableBackend(sqlStore *SQLStore, userID, workspaceID string, backend storage.StoragePort) (*DurableBackend, error) {
	if sqlStore == nil || userID == "" || workspaceID == "" {
		return nil, errors.New("durable Product backend requires SQL store, user and Workspace")
	}
	return &DurableBackend{
		Store:        product.NewStoreWithStorage(workspaceID, backend),
		SQL:          sqlStore,
		UserID:       userID,
		Workspace:    workspaceID,
		Storage:      backend,
		providerData: map[string]storage.UploadSession{},
	}, nil
}

func (b *DurableBackend) WorkspaceID() string { return b.Workspace }

// BeginHTTPIdempotency/FinalizeHTTPIdempotency are deliberately narrow HTTP
// adapters around the durable idempotency table. They keep request hashes and
// replay bodies in PostgreSQL without exposing the SQL repository to handlers.
func (b *DurableBackend) BeginHTTPIdempotency(ctx context.Context, workspaceID, key, request string) (json.RawMessage, int, bool, error) {
	if workspaceID != b.Workspace {
		return nil, 0, false, product.ErrNotFound
	}
	body, replay, err := b.SQL.ClaimIdempotency(ctx, workspaceID, key, request, 0, json.RawMessage(`{"_pending":true}`), "http", "")
	if errors.Is(err, ErrVersionConflict) {
		return nil, 0, false, product.ErrIdempotencyConflict
	}
	if err != nil || !replay {
		return body, 0, replay, err
	}
	status, err := b.SQL.IdempotencyStatus(ctx, workspaceID, key)
	return body, status, replay, err
}

func (b *DurableBackend) FinalizeHTTPIdempotency(ctx context.Context, workspaceID, key, request string, status int, body json.RawMessage) error {
	if workspaceID != b.Workspace {
		return product.ErrNotFound
	}
	return b.SQL.FinalizeIdempotency(ctx, workspaceID, key, request, status, body)
}

func (b *DurableBackend) CreateProject(_ string, name, workflowKey string, settings map[string]any) (*product.Project, error) {
	record, err := b.SQL.CreateProject(context.Background(), b.UserID, b.Workspace, name, workflowKey, jsonBytes(settings))
	if err != nil {
		return nil, err
	}
	return projectFromRecord(record), nil
}
func (b *DurableBackend) ListProjects(_ string, limit int) ([]product.Project, error) {
	records, err := b.SQL.ListProjects(context.Background(), b.UserID, b.Workspace, limit)
	if err != nil {
		return nil, err
	}
	result := make([]product.Project, 0, len(records))
	for _, record := range records {
		result = append(result, *projectFromRecord(record))
	}
	return result, nil
}
func (b *DurableBackend) GetProject(_ string, id string) (*product.Project, error) {
	record, err := b.SQL.GetProject(context.Background(), b.UserID, b.Workspace, id)
	if err != nil {
		return nil, err
	}
	return projectFromRecord(record), nil
}
func (b *DurableBackend) UpdateProject(_ string, id string, revision int64, name, status *string, settings map[string]any) (*product.Project, error) {
	record, err := b.SQL.UpdateProject(context.Background(), b.UserID, b.Workspace, id, revision, name, status, jsonBytes(settings))
	if err != nil {
		return nil, err
	}
	return projectFromRecord(record), nil
}

func (b *DurableBackend) CreateAsset(_ string, projectID, kind, filename, contentType string, size int64, checksum string) (*product.Asset, error) {
	record, err := b.SQL.CreateAsset(context.Background(), b.UserID, b.Workspace, projectID, kind, filename, contentType, checksum, size)
	if err != nil {
		return nil, err
	}
	return assetFromRecord(record), nil
}
func (b *DurableBackend) ListAssets(_ string, projectID string, limit int) ([]product.Asset, error) {
	records, err := b.SQL.ListAssets(context.Background(), b.UserID, b.Workspace, projectID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]product.Asset, 0, len(records))
	for _, record := range records {
		result = append(result, *assetFromRecord(record))
	}
	return result, nil
}
func (b *DurableBackend) GetAsset(_ string, projectID, assetID string) (*product.Asset, error) {
	record, err := b.SQL.GetAsset(context.Background(), b.UserID, b.Workspace, projectID, assetID)
	if err != nil {
		return nil, err
	}
	return assetFromRecord(record), nil
}
func (b *DurableBackend) UpdateAsset(_ string, projectID, assetID string, revision int64, metadata map[string]any) (*product.Asset, error) {
	record, err := b.SQL.UpdateAsset(context.Background(), b.UserID, b.Workspace, projectID, assetID, revision, jsonBytes(metadata))
	if err != nil {
		return nil, err
	}
	return assetFromRecord(record), nil
}
func (b *DurableBackend) SetAssetStatus(_ string, projectID, assetID, status, artifactID string) (*product.Asset, error) {
	record, err := b.SQL.SetAssetStatus(context.Background(), b.UserID, b.Workspace, projectID, assetID, status, artifactID)
	if err != nil {
		return nil, err
	}
	return assetFromRecord(record), nil
}

func (b *DurableBackend) CreateUpload(ctx context.Context, _ string, projectID, assetID string, expectedSize int64, checksum, contentType string, requested []product.UploadPart, _ string) (*product.UploadSession, error) {
	if b.Storage == nil {
		return nil, errors.New("durable upload requires StoragePort")
	}
	provider, err := b.Storage.InitiateUpload(ctx, storage.Scope{WorkspaceID: b.Workspace, ProjectID: projectID}, storage.UploadConstraints{ExpectedSize: expectedSize, ContentType: contentType, PartSize: 8 << 20, Multipart: true, ExpiresAt: time.Now().UTC().Add(2 * time.Hour)})
	if err != nil {
		return nil, fmt.Errorf("initiate storage upload: %w", err)
	}
	record, err := b.SQL.CreateUpload(ctx, b.UserID, b.Workspace, projectID, assetID, provider.Locator.Backend, provider.Locator.ObjectKey, provider.ProviderID, provider.Multipart, provider.PartSize, expectedSize, checksum, provider.ExpiresAt)
	if err != nil {
		_ = b.Storage.AbortUpload(ctx, provider)
		return nil, err
	}
	if len(requested) == 0 {
		requested = []product.UploadPart{{PartNumber: 1}}
	}
	parts, err := b.presign(ctx, provider, requested)
	if err != nil {
		_ = b.SQL.AbortUpload(ctx, b.UserID, b.Workspace, projectID, assetID, record.ID)
		_ = b.Storage.AbortUpload(ctx, provider)
		return nil, err
	}
	b.providerMu.Lock()
	b.providerData[record.ID] = provider
	b.providerMu.Unlock()
	return uploadFromRecord(record, parts), nil
}

func (b *DurableBackend) GetUpload(_ string, projectID, assetID, uploadID string) (*product.UploadSession, error) {
	record, err := b.SQL.GetUpload(context.Background(), b.UserID, b.Workspace, projectID, assetID, uploadID)
	if err != nil {
		return nil, err
	}
	b.providerMu.Lock()
	provider, ok := b.providerData[record.ID]
	b.providerMu.Unlock()
	var parts []product.UploadPart
	if ok {
		parts, _ = b.presign(context.Background(), provider, []product.UploadPart{{PartNumber: 1}})
	}
	return uploadFromRecord(record, parts), nil
}

func (b *DurableBackend) PresignUploadParts(ctx context.Context, _ string, projectID, assetID, uploadID string, partNumbers []int) (*product.UploadSession, error) {
	record, err := b.SQL.GetUpload(ctx, b.UserID, b.Workspace, projectID, assetID, uploadID)
	if err != nil {
		return nil, err
	}
	if len(partNumbers) == 0 {
		return nil, errors.New("at least one part number is required")
	}
	b.providerMu.Lock()
	provider, ok := b.providerData[record.ID]
	b.providerMu.Unlock()
	if !ok {
		provider = providerFromRecord(record)
	}
	requested := make([]product.UploadPart, 0, len(partNumbers))
	for _, number := range partNumbers {
		requested = append(requested, product.UploadPart{PartNumber: number})
	}
	parts, err := b.presign(ctx, provider, requested)
	if err != nil {
		return nil, err
	}
	return uploadFromRecord(record, parts), nil
}

func (b *DurableBackend) CompleteUpload(ctx context.Context, _ string, projectID, assetID, uploadID, checksum string, parts []product.UploadPart) (*product.UploadSession, *product.Job, error) {
	record, err := b.SQL.GetUpload(ctx, b.UserID, b.Workspace, projectID, assetID, uploadID)
	if err != nil {
		return nil, nil, err
	}
	if record.Status == "completed" {
		durableJob, jobErr := b.SQL.GetAssetProbeJob(ctx, b.UserID, b.Workspace, projectID, assetID)
		if jobErr != nil {
			return nil, nil, jobErr
		}
		return uploadFromRecord(record, parts), &product.Job{ID: durableJob.ID, ProjectID: durableJob.ProjectID, Kind: durableJob.Kind, Status: domain.JobCreated, Command: mapFromJSON(durableJob.Command), CreatedAt: durableJob.CreatedAt}, nil
	}
	if record.Status != "active" {
		return nil, nil, product.ErrConflict
	}
	if record.ExpectedSHA256 != "" && record.ExpectedSHA256 != checksum {
		return nil, nil, errors.New("completed upload checksum does not match the declared checksum")
	}
	b.providerMu.Lock()
	provider, ok := b.providerData[record.ID]
	b.providerMu.Unlock()
	if !ok {
		provider = providerFromRecord(record)
	}
	providerParts := make([]storage.UploadedPart, 0, len(parts))
	for _, part := range parts {
		providerParts = append(providerParts, storage.UploadedPart{PartNumber: part.PartNumber, ETag: part.ETag})
	}
	staged, err := b.Storage.CompleteUpload(ctx, provider, providerParts)
	if err != nil {
		return nil, nil, fmt.Errorf("complete storage upload: %w", err)
	}
	command := jsonBytes(map[string]any{"asset_id": assetID, "purpose": "validation"})
	completed, durableJob, err := b.SQL.CompleteUploadAndCreateProbeJob(ctx, b.UserID, b.Workspace, projectID, assetID, uploadID, staged.SizeBytes, command, command)
	if err != nil {
		// Leave the staging locator for reconciliation; it is not published as
		// a committed Artifact and is never returned as ready media.
		return nil, nil, err
	}
	b.providerMu.Lock()
	delete(b.providerData, record.ID)
	b.providerMu.Unlock()
	// T300 owns Job transitions/orchestration. This Gate C+D boundary only
	// creates the durable immutable intent and leaves Asset non-ready.
	job := &product.Job{ID: durableJob.ID, ProjectID: durableJob.ProjectID, Kind: durableJob.Kind, Status: domain.JobCreated, Command: mapFromJSON(durableJob.Command), CreatedAt: durableJob.CreatedAt}
	return uploadFromRecord(completed, parts), job, nil
}

func (b *DurableBackend) AbortUpload(ctx context.Context, _ string, projectID, assetID, uploadID string) error {
	record, err := b.SQL.GetUpload(ctx, b.UserID, b.Workspace, projectID, assetID, uploadID)
	if err != nil {
		return err
	}
	if record.Status == "aborted" {
		return nil
	}
	if record.Status != "active" {
		return product.ErrConflict
	}
	b.providerMu.Lock()
	provider, ok := b.providerData[record.ID]
	b.providerMu.Unlock()
	if ok {
		if err := b.Storage.AbortUpload(ctx, provider); err != nil {
			return err
		}
	}
	if err := b.SQL.AbortUpload(ctx, b.UserID, b.Workspace, projectID, assetID, uploadID); err != nil {
		return err
	}
	b.providerMu.Lock()
	delete(b.providerData, record.ID)
	b.providerMu.Unlock()
	return nil
}

func (b *DurableBackend) CreateScript(_ string, projectID, language, origin string, content map[string]any) (*product.Script, *product.ScriptVersion, error) {
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, nil, mapPersistenceError(err)
	}
	contentJSON := jsonBytes(content)
	hash := jsonHash(contentJSON)
	tx, err := b.SQL.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var scriptID string
	if err := tx.QueryRowContext(ctx, `INSERT INTO scripts(project_id,status,created_by,updated_by) VALUES($1,'active',$2,$2) RETURNING id::text`, projectID, b.UserID).Scan(&scriptID); err != nil {
		return nil, nil, err
	}
	var versionID string
	var createdAt time.Time
	if err := tx.QueryRowContext(ctx, `INSERT INTO script_versions(script_id,version,status,language,origin,content,content_hash,created_by) VALUES($1,1,'draft',$2,$3,$4,$5,$6) RETURNING id::text,created_at`, scriptID, language, origin, contentJSON, hash, b.UserID).Scan(&versionID, &createdAt); err != nil {
		return nil, nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE scripts SET current_version_id=$2,updated_at=now() WHERE id=$1`, scriptID, versionID); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	script := &product.Script{ID: scriptID, ProjectID: projectID, Status: "active", CurrentVersionID: versionID, Revision: 1, Versions: []string{versionID}, CreatedAt: createdAt}
	version := &product.ScriptVersion{ID: versionID, ScriptID: scriptID, ProjectID: projectID, Version: 1, Status: "draft", Language: language, Origin: origin, Content: cloneMap(content), ContentHash: hash, CreatedAt: createdAt}
	return script, version, nil
}

func (b *DurableBackend) GetScript(_ string, projectID, scriptID string) (*product.Script, error) {
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	var script product.Script
	if err := b.SQL.DB.QueryRowContext(ctx, `SELECT id::text,project_id::text,status,COALESCE(current_version_id::text,''),revision,created_at FROM scripts WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, scriptID, projectID).Scan(&script.ID, &script.ProjectID, &script.Status, &script.CurrentVersionID, &script.Revision, &script.CreatedAt); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		return nil, mapPersistenceError(err)
	}
	rows, err := b.SQL.DB.QueryContext(ctx, `SELECT id::text FROM script_versions WHERE script_id=$1 ORDER BY version`, scriptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		script.Versions = append(script.Versions, id)
	}
	return &script, rows.Err()
}

func (b *DurableBackend) CreateScriptVersion(_ string, projectID, scriptID, basedOn, language, origin string, content map[string]any, expectedRevision int64) (*product.ScriptVersion, error) {
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	contentJSON := jsonBytes(content)
	hash := jsonHash(contentJSON)
	tx, err := b.SQL.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var nextVersion int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0)+1 FROM script_versions WHERE script_id=$1`, scriptID).Scan(&nextVersion); err != nil {
		return nil, err
	}
	var currentRevision int64
	if err := tx.QueryRowContext(ctx, `SELECT revision FROM scripts WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`, scriptID, projectID).Scan(&currentRevision); err != nil {
		return nil, mapPersistenceError(err)
	}
	if currentRevision != expectedRevision {
		return nil, product.ErrConflict
	}
	var versionID string
	var createdAt time.Time
	if err := tx.QueryRowContext(ctx, `INSERT INTO script_versions(script_id,version,status,language,origin,content,content_hash,based_on_version_id,created_by) VALUES($1,$2,'draft',$3,$4,$5,$6,NULLIF($7,'')::uuid,$8) RETURNING id::text,created_at`, scriptID, nextVersion, language, origin, contentJSON, hash, basedOn, b.UserID).Scan(&versionID, &createdAt); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE scripts SET current_version_id=$2,revision=revision+1,updated_by=$3,updated_at=now() WHERE id=$1`, scriptID, versionID, b.UserID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &product.ScriptVersion{ID: versionID, ScriptID: scriptID, ProjectID: projectID, Version: nextVersion, Status: "draft", Language: language, Origin: origin, Content: cloneMap(content), ContentHash: hash, BasedOnVersionID: basedOn, CreatedAt: createdAt}, nil
}

func (b *DurableBackend) GetScriptVersion(_ string, projectID, versionID string) (*product.ScriptVersion, error) {
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	var value product.ScriptVersion
	var content []byte
	if err := b.SQL.DB.QueryRowContext(ctx, `SELECT v.id::text,v.script_id::text,s.project_id::text,v.version,v.status,v.language,v.origin,v.content,v.content_hash,COALESCE(v.based_on_version_id::text,''),v.created_at FROM script_versions v JOIN scripts s ON s.id=v.script_id WHERE v.id=$1 AND s.project_id=$2`, versionID, projectID).Scan(&value.ID, &value.ScriptID, &value.ProjectID, &value.Version, &value.Status, &value.Language, &value.Origin, &content, &value.ContentHash, &value.BasedOnVersionID, &value.CreatedAt); err != nil {
		return nil, mapPersistenceError(err)
	}
	value.Content = cloneMap(mapFromJSON(content))
	return &value, nil
}

func (b *DurableBackend) SetScriptVersionStatus(_ string, projectID, scriptID, versionID, status string) (*product.ScriptVersion, error) {
	if status != "approved" && status != "superseded" && status != "draft" {
		return nil, errors.New("invalid script version status")
	}
	ctx := context.Background()
	tx, err := b.SQL.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var approvedAt any
	if status == "approved" {
		approvedAt = time.Now().UTC()
	}
	result, err := tx.ExecContext(ctx, `UPDATE script_versions SET status=$3,approved_by=CASE WHEN $3='approved' THEN $4 ELSE approved_by END,approved_at=CASE WHEN $3='approved' THEN now() ELSE approved_at END,updated_at=now() WHERE id=$1 AND script_id=$2`, versionID, scriptID, status, b.UserID)
	if err != nil {
		return nil, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return nil, product.ErrNotFound
	}
	if status == "approved" {
		if _, err := tx.ExecContext(ctx, `UPDATE scripts SET current_version_id=$2,revision=revision+1,updated_by=$3,updated_at=now() WHERE id=$1 AND project_id=$4`, scriptID, versionID, b.UserID, projectID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	_ = approvedAt
	return b.GetScriptVersion("", projectID, versionID)
}

func (b *DurableBackend) CreateNarration(_ string, projectID, scriptVersionID, providerID string, voice, request map[string]any) (*product.Narration, error) {
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	var value product.Narration
	var voiceJSON, requestJSON []byte
	err := b.SQL.DB.QueryRowContext(ctx, `INSERT INTO narrations(project_id,script_version_id,status,provider_configuration_id,voice_snapshot,request_snapshot) SELECT $1,$2,'created',NULLIF($3,'')::uuid,$4,$5 FROM script_versions v JOIN scripts s ON s.id=v.script_id WHERE v.id=$2 AND s.project_id=$1 AND v.status='approved' RETURNING id::text,project_id::text,script_version_id::text,status,COALESCE(provider_configuration_id::text,''),voice_snapshot,request_snapshot,created_at`, projectID, scriptVersionID, providerID, jsonBytes(voice), jsonBytes(request)).Scan(&value.ID, &value.ProjectID, &value.ScriptVersionID, &value.Status, &value.ProviderConfigID, &voiceJSON, &requestJSON, &value.CreatedAt)
	if err != nil {
		return nil, product.ErrConflict
	}
	value.VoiceSnapshot = mapFromJSON(voiceJSON)
	value.RequestSnapshot = mapFromJSON(requestJSON)
	job, jobErr := b.SQL.CreateProjectJob(ctx, b.UserID, b.Workspace, projectID, "pipeline", jsonBytes(map[string]any{"narration_id": value.ID, "script_version_id": scriptVersionID}), jsonBytes(map[string]any{"narration_id": value.ID, "script_version_id": scriptVersionID}))
	if jobErr != nil {
		return nil, jobErr
	}
	value.JobID = job.ID
	return &value, nil
}

func (b *DurableBackend) GetNarration(_ string, projectID, narrationID string) (*product.Narration, error) {
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	var value product.Narration
	var voiceJSON, requestJSON []byte
	err := b.SQL.DB.QueryRowContext(ctx, `SELECT id::text,project_id::text,script_version_id::text,status,COALESCE(provider_configuration_id::text,''),voice_snapshot,request_snapshot,COALESCE(audio_artifact_id::text,''),COALESCE(timing_artifact_id::text,''),COALESCE(duration_sec,0),created_at FROM narrations WHERE id=$1 AND project_id=$2`, narrationID, projectID).Scan(&value.ID, &value.ProjectID, &value.ScriptVersionID, &value.Status, &value.ProviderConfigID, &voiceJSON, &requestJSON, &value.AudioArtifactID, &value.TimingArtifactID, &value.DurationSec, &value.CreatedAt)
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	value.VoiceSnapshot = mapFromJSON(voiceJSON)
	value.RequestSnapshot = mapFromJSON(requestJSON)
	_ = b.SQL.DB.QueryRowContext(ctx, `SELECT id::text FROM jobs WHERE project_id=$1 AND kind='pipeline' AND command->>'narration_id'=$2 ORDER BY created_at DESC,id DESC LIMIT 1`, projectID, narrationID).Scan(&value.JobID)
	return &value, nil
}

func (b *DurableBackend) CreateTimeline(_ string, projectID, origin string, document json.RawMessage) (*product.Timeline, *product.TimelineVersion, error) {
	hash, err := domain.ValidateTimeline(document, domain.TimelineValidationOptions{Resolver: b})
	if err != nil {
		return nil, nil, err
	}
	if value := timelineProject(document); value != projectID {
		return nil, nil, errors.New("timeline document belongs to another project")
	}
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, nil, mapPersistenceError(err)
	}
	tx, err := b.SQL.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var timelineID string
	var createdAt time.Time
	if err := tx.QueryRowContext(ctx, `INSERT INTO timelines(project_id,status,created_by,updated_by) VALUES($1,'active',$2,$2) RETURNING id::text,created_at`, projectID, b.UserID).Scan(&timelineID, &createdAt); err != nil {
		return nil, nil, err
	}
	var versionID string
	if err := tx.QueryRowContext(ctx, `INSERT INTO timeline_versions(timeline_id,version,status,schema_version,document,content_hash,origin,created_by) VALUES($1,1,'draft','1.0',$2,$3,$4,$5) RETURNING id::text`, timelineID, document, hash, origin, b.UserID).Scan(&versionID); err != nil {
		return nil, nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE timelines SET current_version_id=$2,updated_at=now() WHERE id=$1`, timelineID, versionID); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	timeline := &product.Timeline{ID: timelineID, ProjectID: projectID, Status: "active", CurrentVersionID: versionID, Revision: 1, Versions: []string{versionID}, CreatedAt: createdAt}
	version := &product.TimelineVersion{ID: versionID, TimelineID: timelineID, ProjectID: projectID, Version: 1, Status: "draft", SchemaVersion: domain.TimelineSchemaVersion, Document: append(json.RawMessage(nil), document...), ContentHash: hash, Origin: origin, CreatedAt: createdAt}
	return timeline, version, nil
}

func (b *DurableBackend) GetTimeline(_ string, projectID, timelineID string) (*product.Timeline, error) {
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	var value product.Timeline
	if err := b.SQL.DB.QueryRowContext(ctx, `SELECT id::text,project_id::text,status,COALESCE(current_version_id::text,''),revision,created_at FROM timelines WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, timelineID, projectID).Scan(&value.ID, &value.ProjectID, &value.Status, &value.CurrentVersionID, &value.Revision, &value.CreatedAt); err != nil {
		return nil, mapPersistenceError(err)
	}
	rows, err := b.SQL.DB.QueryContext(ctx, `SELECT id::text FROM timeline_versions WHERE timeline_id=$1 ORDER BY version`, timelineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		value.Versions = append(value.Versions, id)
	}
	return &value, rows.Err()
}

func (b *DurableBackend) GetTimelineVersion(_ string, projectID, versionID string) (*product.TimelineVersion, error) {
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	var value product.TimelineVersion
	if err := b.SQL.DB.QueryRowContext(ctx, `SELECT v.id::text,v.timeline_id::text,t.project_id::text,v.version,v.status,v.schema_version,v.document,v.content_hash,v.origin,COALESCE(v.based_on_version_id::text,''),v.created_at FROM timeline_versions v JOIN timelines t ON t.id=v.timeline_id WHERE v.id=$1 AND t.project_id=$2`, versionID, projectID).Scan(&value.ID, &value.TimelineID, &value.ProjectID, &value.Version, &value.Status, &value.SchemaVersion, &value.Document, &value.ContentHash, &value.Origin, &value.BasedOnVersionID, &value.CreatedAt); err != nil {
		return nil, mapPersistenceError(err)
	}
	return &value, nil
}

func (b *DurableBackend) AddTimelineVersion(_ string, projectID, timelineID, basedOn, origin string, document json.RawMessage, expectedRevision int64) (*product.TimelineVersion, error) {
	hash, err := domain.ValidateTimeline(document, domain.TimelineValidationOptions{Resolver: b})
	if err != nil {
		return nil, err
	}
	if timelineProject(document) != projectID {
		return nil, errors.New("timeline document belongs to another project")
	}
	ctx := context.Background()
	tx, err := b.SQL.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var revision int64
	if err := tx.QueryRowContext(ctx, `SELECT revision FROM timelines WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`, timelineID, projectID).Scan(&revision); err != nil {
		return nil, mapPersistenceError(err)
	}
	if revision != expectedRevision {
		return nil, product.ErrConflict
	}
	var versionNumber int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0)+1 FROM timeline_versions WHERE timeline_id=$1`, timelineID).Scan(&versionNumber); err != nil {
		return nil, err
	}
	var versionID string
	var createdAt time.Time
	if err := tx.QueryRowContext(ctx, `INSERT INTO timeline_versions(timeline_id,version,status,schema_version,document,content_hash,origin,based_on_version_id,created_by) VALUES($1,$2,'draft','1.0',$3,$4,$5,NULLIF($6,'')::uuid,$7) RETURNING id::text,created_at`, timelineID, versionNumber, document, hash, origin, basedOn, b.UserID).Scan(&versionID, &createdAt); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE timelines SET current_version_id=$2,revision=revision+1,updated_by=$3,updated_at=now() WHERE id=$1`, timelineID, versionID, b.UserID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &product.TimelineVersion{ID: versionID, TimelineID: timelineID, ProjectID: projectID, Version: versionNumber, Status: "draft", SchemaVersion: domain.TimelineSchemaVersion, Document: append(json.RawMessage(nil), document...), ContentHash: hash, Origin: origin, BasedOnVersionID: basedOn, CreatedAt: createdAt}, nil
}

func (b *DurableBackend) SetTimelineVersionStatus(_ string, projectID, timelineID, versionID, status string) (*product.TimelineVersion, error) {
	if status != "approved" && status != "locked" && status != "superseded" && status != "draft" {
		return nil, errors.New("invalid timeline version status")
	}
	if _, err := b.SQL.GetProject(context.Background(), b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	if status == "approved" || status == "locked" {
		if _, err := b.ValidateTimeline("", projectID, versionID); err != nil {
			return nil, err
		}
	}
	ctx := context.Background()
	tx, err := b.SQL.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE timeline_versions SET status=$3,approved_by=CASE WHEN $3 IN ('approved','locked') THEN $4 ELSE approved_by END,approved_at=CASE WHEN $3 IN ('approved','locked') THEN now() ELSE approved_at END,updated_at=now() WHERE id=$1 AND timeline_id=$2 AND EXISTS (SELECT 1 FROM timelines scoped_timeline WHERE scoped_timeline.id=$2 AND scoped_timeline.project_id=$5)`, versionID, timelineID, status, b.UserID, projectID)
	if err != nil {
		return nil, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return nil, product.ErrNotFound
	}
	if status == "approved" || status == "locked" {
		if _, err := tx.ExecContext(ctx, `UPDATE timelines SET current_version_id=$2,revision=revision+1,updated_by=$3,updated_at=now() WHERE id=$1 AND project_id=$4`, timelineID, versionID, b.UserID, projectID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return b.GetTimelineVersion("", projectID, versionID)
}

func (b *DurableBackend) ValidateTimeline(_ string, projectID, versionID string) (string, error) {
	value, err := b.GetTimelineVersion("", projectID, versionID)
	if err != nil {
		return "", err
	}
	return domain.ValidateTimeline(value.Document, domain.TimelineValidationOptions{Resolver: b})
}

func (b *DurableBackend) CreateRenderProfile(_ string, profileKey string, version int, document map[string]any) (*product.RenderProfile, error) {
	if profileKey == "" || version < 1 || len(document) == 0 {
		return nil, errors.New("render profile key, positive version and document are required")
	}
	ctx := context.Background()
	profileJSON := jsonBytes(document)
	hash := jsonHash(profileJSON)
	var value product.RenderProfile
	err := b.SQL.DB.QueryRowContext(ctx, `INSERT INTO render_profiles(workspace_id,profile_key,version,status,schema_version,document,content_hash,created_by) VALUES($1,$2,$3,'draft','1.0',$4,$5,$6) RETURNING id::text,workspace_id::text,profile_key,version,status,schema_version,document,content_hash,created_at,updated_at`, b.Workspace, profileKey, version, profileJSON, hash, b.UserID).Scan(&value.ID, &value.WorkspaceID, &value.ProfileKey, &value.Version, &value.Status, &value.SchemaVersion, &profileJSON, &value.ContentHash, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return nil, err
	}
	value.Document = mapFromJSON(profileJSON)
	return &value, nil
}

func (b *DurableBackend) GetRenderProfile(_ string, profileKey string, version int) (*product.RenderProfile, error) {
	ctx := context.Background()
	var value product.RenderProfile
	var document []byte
	err := b.SQL.DB.QueryRowContext(ctx, `SELECT id::text,workspace_id::text,profile_key,version,status,schema_version,document,content_hash,created_at,updated_at FROM render_profiles WHERE workspace_id=$1 AND profile_key=$2 AND version=$3`, b.Workspace, profileKey, version).Scan(&value.ID, &value.WorkspaceID, &value.ProfileKey, &value.Version, &value.Status, &value.SchemaVersion, &document, &value.ContentHash, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, product.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	value.Document = mapFromJSON(document)
	return &value, nil
}

func (b *DurableBackend) UpdateRenderProfile(_ string, profileKey string, version int, document map[string]any) (*product.RenderProfile, error) {
	current, err := b.GetRenderProfile("", profileKey, version)
	if err != nil {
		return nil, err
	}
	if current.Status != "draft" {
		return nil, product.ErrConflict
	}
	if len(document) == 0 {
		return nil, errors.New("render profile document is required")
	}
	ctx := context.Background()
	profileJSON := jsonBytes(document)
	hash := jsonHash(profileJSON)
	var value product.RenderProfile
	err = b.SQL.DB.QueryRowContext(ctx, `UPDATE render_profiles SET document=$4,content_hash=$5,updated_at=now() WHERE workspace_id=$1 AND profile_key=$2 AND version=$3 AND status='draft' RETURNING id::text,workspace_id::text,profile_key,version,status,schema_version,document,content_hash,created_at,updated_at`, b.Workspace, profileKey, version, profileJSON, hash).Scan(&value.ID, &value.WorkspaceID, &value.ProfileKey, &value.Version, &value.Status, &value.SchemaVersion, &profileJSON, &value.ContentHash, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, product.ErrConflict
	}
	if err != nil {
		return nil, err
	}
	value.Document = mapFromJSON(profileJSON)
	return &value, nil
}

func (b *DurableBackend) SetRenderProfileStatus(_ string, profileKey string, version int, status string) (*product.RenderProfile, error) {
	current, err := b.GetRenderProfile("", profileKey, version)
	if err != nil {
		return nil, err
	}
	if err := product.ValidateRenderProfileTransition(current.Status, status); err != nil {
		return nil, product.ErrConflict
	}
	ctx := context.Background()
	var value product.RenderProfile
	var document []byte
	err = b.SQL.DB.QueryRowContext(ctx, `UPDATE render_profiles SET status=$4,updated_at=now() WHERE workspace_id=$1 AND profile_key=$2 AND version=$3 AND status=$5 RETURNING id::text,workspace_id::text,profile_key,version,status,schema_version,document,content_hash,created_at,updated_at`, b.Workspace, profileKey, version, status, current.Status).Scan(&value.ID, &value.WorkspaceID, &value.ProfileKey, &value.Version, &value.Status, &value.SchemaVersion, &document, &value.ContentHash, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, product.ErrConflict
	}
	if err != nil {
		return nil, err
	}
	value.Document = mapFromJSON(document)
	return &value, nil
}

func (b *DurableBackend) CreateRender(_ string, projectID, timelineVersionID, profileKey string, profileVersion int, request map[string]any) (*product.Render, error) {
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	if _, err := b.ValidateTimeline("", projectID, timelineVersionID); err != nil {
		return nil, err
	}
	var timelineStatus string
	if err := b.SQL.DB.QueryRowContext(ctx, `SELECT status FROM timeline_versions v JOIN timelines t ON t.id=v.timeline_id WHERE v.id=$1 AND t.project_id=$2`, timelineVersionID, projectID).Scan(&timelineStatus); err != nil {
		return nil, mapPersistenceError(err)
	}
	if timelineStatus != "approved" && timelineStatus != "locked" {
		return nil, product.ErrConflict
	}
	var profileID, profileStatus, profileHash string
	var profileDocument []byte
	err := b.SQL.DB.QueryRowContext(ctx, `SELECT id::text,status,document,content_hash FROM render_profiles WHERE (workspace_id=$1 OR workspace_id IS NULL) AND profile_key=$2 AND version=$3 ORDER BY CASE WHEN workspace_id=$1 THEN 0 ELSE 1 END LIMIT 1`, b.Workspace, profileKey, profileVersion).Scan(&profileID, &profileStatus, &profileDocument, &profileHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, product.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if profileStatus != "active" {
		return nil, product.ErrConflict
	}
	requestJSON := jsonBytes(request)
	var value product.Render
	requestHash := jsonHash(requestJSON)
	if err := b.SQL.DB.QueryRowContext(ctx, `INSERT INTO renders(project_id,timeline_version_id,render_profile_id,profile_snapshot,status,request_hash,request,created_by) VALUES($1,$2,$3,$4,'created',$5,$6,$7) RETURNING id::text,project_id::text,timeline_version_id::text,status,created_at`, projectID, timelineVersionID, profileID, profileDocument, requestHash, requestJSON, b.UserID).Scan(&value.ID, &value.ProjectID, &value.TimelineVersionID, &value.Status, &value.CreatedAt); err != nil {
		return nil, err
	}
	value.ProfileKey = profileKey
	value.ProfileVersion = profileVersion
	value.ProfileID = profileID
	value.ProfileSnapshot = mapFromJSON(profileDocument)
	jobCommand := cloneMap(request)
	jobCommand["render_id"] = value.ID
	jobRequest := jsonBytes(jobCommand)
	job, jobErr := b.SQL.CreateProjectJob(ctx, b.UserID, b.Workspace, projectID, "render", jobRequest, jobRequest)
	if jobErr != nil {
		return nil, jobErr
	}
	if _, err := b.SQL.DB.ExecContext(ctx, `UPDATE renders SET job_id=$2,updated_at=now() WHERE id=$1`, value.ID, job.ID); err != nil {
		return nil, err
	}
	value.JobID = job.ID
	return &value, nil
}

func (b *DurableBackend) CreateAnalysis(_ string, projectID, kind string, inputRefs, provenance map[string]any) (*product.Analysis, error) {
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	if kind == "reference_style" {
		provenance = cloneMap(provenance)
		provenance["output_policy"] = "abstract_metrics_only"
	}
	var value product.Analysis
	var inputJSON, provenanceJSON []byte
	err := b.SQL.DB.QueryRowContext(ctx, `INSERT INTO analyses(project_id,kind,status,input_refs,schema_version,provenance) VALUES($1,$2,'created',$3,'1.0',$4) RETURNING id::text,project_id::text,kind,status,input_refs,provenance,created_at`, projectID, kind, jsonBytes(inputRefs), jsonBytes(provenance)).Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Status, &inputJSON, &provenanceJSON, &value.CreatedAt)
	if err != nil {
		return nil, err
	}
	value.InputRefs = mapFromJSON(inputJSON)
	value.Provenance = mapFromJSON(provenanceJSON)
	return &value, nil
}
func (b *DurableBackend) GetAnalysis(_ string, projectID, analysisID string) (*product.Analysis, error) {
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	var value product.Analysis
	var inputJSON, resultJSON, provenanceJSON []byte
	err := b.SQL.DB.QueryRowContext(ctx, `SELECT id::text,project_id::text,kind,status,input_refs,COALESCE(result,'{}'::jsonb),provenance,created_at FROM analyses WHERE id=$1 AND project_id=$2`, analysisID, projectID).Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Status, &inputJSON, &resultJSON, &provenanceJSON, &value.CreatedAt)
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	value.InputRefs, value.Result, value.Provenance = mapFromJSON(inputJSON), mapFromJSON(resultJSON), mapFromJSON(provenanceJSON)
	return &value, nil
}
func (b *DurableBackend) ListAnalyses(_ string, projectID string) ([]product.Analysis, error) {
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	rows, err := b.SQL.DB.QueryContext(ctx, `SELECT id::text,project_id::text,kind,status,input_refs,COALESCE(result,'{}'::jsonb),provenance,created_at FROM analyses WHERE project_id=$1 ORDER BY created_at,id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []product.Analysis{}
	for rows.Next() {
		var value product.Analysis
		var inputJSON, resultJSON, provenanceJSON []byte
		if err := rows.Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Status, &inputJSON, &resultJSON, &provenanceJSON, &value.CreatedAt); err != nil {
			return nil, err
		}
		value.InputRefs, value.Result, value.Provenance = mapFromJSON(inputJSON), mapFromJSON(resultJSON), mapFromJSON(provenanceJSON)
		result = append(result, value)
	}
	return result, rows.Err()
}

func (b *DurableBackend) CreateScene(_ string, projectID, assetID, artifactID string, start, end float64, description string) (*product.Scene, error) {
	if artifactID == "" || start < 0 || end <= start {
		return nil, errors.New("durable scene requires committed artifact and valid range")
	}
	ctx := context.Background()
	if _, err := b.SQL.GetAsset(ctx, b.UserID, b.Workspace, projectID, assetID); err != nil {
		return nil, mapPersistenceError(err)
	}
	var scene product.Scene
	err := b.SQL.DB.QueryRowContext(ctx, `INSERT INTO scenes(project_id,source_asset_id,source_artifact_id,detection_revision,scene_index,status,source_start_sec,source_end_sec,description,updated_by) SELECT $1,$2,$3,'api',COALESCE(MAX(scene_index),-1)+1,'detected',$4,$5,$6,$7 FROM scenes WHERE project_id=$1 AND source_artifact_id=$3 RETURNING id::text,project_id::text,source_asset_id::text,source_artifact_id::text,status,source_start_sec,source_end_sec,COALESCE(description,''),revision`, projectID, assetID, artifactID, start, end, description, b.UserID).Scan(&scene.ID, &scene.ProjectID, &scene.AssetID, &scene.ArtifactID, &scene.Status, &scene.StartSec, &scene.EndSec, &scene.Description, &scene.Revision)
	if err != nil {
		return nil, err
	}
	return &scene, nil
}
func (b *DurableBackend) ListScenes(_ string, projectID string) ([]product.Scene, error) {
	if _, err := b.SQL.GetProject(context.Background(), b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	rows, err := b.SQL.DB.QueryContext(context.Background(), `SELECT id::text,project_id::text,source_asset_id::text,source_artifact_id::text,status,source_start_sec,source_end_sec,COALESCE(description,''),revision FROM scenes WHERE project_id=$1 ORDER BY scene_index,id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []product.Scene{}
	for rows.Next() {
		var value product.Scene
		if err := rows.Scan(&value.ID, &value.ProjectID, &value.AssetID, &value.ArtifactID, &value.Status, &value.StartSec, &value.EndSec, &value.Description, &value.Revision); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
func (b *DurableBackend) GetScene(_ string, projectID, sceneID string) (*product.Scene, error) {
	if _, err := b.SQL.GetProject(context.Background(), b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	var value product.Scene
	err := b.SQL.DB.QueryRowContext(context.Background(), `SELECT id::text,project_id::text,source_asset_id::text,source_artifact_id::text,status,source_start_sec,source_end_sec,COALESCE(description,''),revision FROM scenes WHERE id=$1 AND project_id=$2`, sceneID, projectID).Scan(&value.ID, &value.ProjectID, &value.AssetID, &value.ArtifactID, &value.Status, &value.StartSec, &value.EndSec, &value.Description, &value.Revision)
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	return &value, nil
}

func (b *DurableBackend) CreateCandidateGroup(_ string, projectID string, candidates []product.Candidate) (*product.CandidateGroup, error) {
	if len(candidates) == 0 {
		return nil, errors.New("candidate group requires candidates")
	}
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	var groupID string
	if err := b.SQL.DB.QueryRowContext(ctx, `SELECT gen_random_uuid()::text`).Scan(&groupID); err != nil {
		return nil, err
	}
	result := &product.CandidateGroup{ID: groupID, ProjectID: projectID, Candidates: make([]product.Candidate, 0, len(candidates))}
	for index, candidate := range candidates {
		if candidate.JobStepID == "" || candidate.ResourceID == "" {
			return nil, errors.New("durable candidate requires job_step_id and resource_id")
		}
		status := candidate.Status
		if status == "" {
			status = "created"
		}
		rank := index + 1
		var candidateID string
		if err := b.SQL.DB.QueryRowContext(ctx, `INSERT INTO generation_candidates(project_id,job_step_id,candidate_group_id,status,rank,resource_type,resource_id,payload,provenance) VALUES($1,$2,$3,$4,$5,$6,$7::uuid,$8,$9) RETURNING id::text`, projectID, candidate.JobStepID, groupID, status, rank, candidate.ResourceType, candidate.ResourceID, jsonBytes(candidate.Payload), jsonBytes(candidate.Provenance)).Scan(&candidateID); err != nil {
			return nil, err
		}
		candidate.ID, candidate.ProjectID, candidate.GroupID, candidate.Status = candidateID, projectID, groupID, status
		result.Candidates = append(result.Candidates, candidate)
	}
	return result, nil
}
func (b *DurableBackend) GetCandidateGroup(_ string, projectID, groupID string) (*product.CandidateGroup, error) {
	ctx := context.Background()
	if _, err := b.SQL.GetProject(ctx, b.UserID, b.Workspace, projectID); err != nil {
		return nil, mapPersistenceError(err)
	}
	rows, err := b.SQL.DB.QueryContext(ctx, `SELECT id::text,project_id::text,job_step_id::text,candidate_group_id::text,status,resource_type,resource_id::text,payload,provenance FROM generation_candidates WHERE project_id=$1 AND candidate_group_id=$2 ORDER BY rank,id`, projectID, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := &product.CandidateGroup{ID: groupID, ProjectID: projectID, Candidates: []product.Candidate{}}
	for rows.Next() {
		var candidate product.Candidate
		var payload, provenance []byte
		if err := rows.Scan(&candidate.ID, &candidate.ProjectID, &candidate.JobStepID, &candidate.GroupID, &candidate.Status, &candidate.ResourceType, &candidate.ResourceID, &payload, &provenance); err != nil {
			return nil, err
		}
		candidate.Payload, candidate.Provenance = mapFromJSON(payload), mapFromJSON(provenance)
		result.Candidates = append(result.Candidates, candidate)
	}
	if len(result.Candidates) == 0 {
		return nil, product.ErrNotFound
	}
	return result, rows.Err()
}

func (b *DurableBackend) ListProviders(_ string) ([]product.ProviderConfiguration, error) {
	rows, err := b.SQL.DB.QueryContext(context.Background(), `SELECT id::text,workspace_id::text,kind,adapter_key,display_name,status,COALESCE(credential_ref,''),model_defaults,config,revision,last_validated_at FROM provider_configurations WHERE workspace_id=$1 AND deleted_at IS NULL ORDER BY display_name,id`, b.Workspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []product.ProviderConfiguration{}
	for rows.Next() {
		value, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
func (b *DurableBackend) CreateProvider(_ string, kind, adapter, display string, config, models map[string]any, credentialRef string) (*product.ProviderConfiguration, error) {
	ctx := context.Background()
	var value product.ProviderConfiguration
	var modelJSON, configJSON []byte
	err := b.SQL.DB.QueryRowContext(ctx, `INSERT INTO provider_configurations(workspace_id,kind,adapter_key,display_name,status,credential_ref,model_defaults,config,created_by,updated_by) VALUES($1,$2,$3,$4,'draft',NULLIF($5,''),$6,$7,$8,$8) RETURNING id::text,workspace_id::text,kind,adapter_key,display_name,status,COALESCE(credential_ref,''),model_defaults,config,revision,last_validated_at`, b.Workspace, kind, adapter, display, credentialRef, jsonBytes(models), jsonBytes(config), b.UserID).Scan(&value.ID, &value.WorkspaceID, &value.Kind, &value.AdapterKey, &value.DisplayName, &value.Status, &value.CredentialRef, &modelJSON, &configJSON, &value.Revision, &value.LastValidatedAt)
	if err != nil {
		return nil, err
	}
	value.ModelDefaults, value.Config = mapFromJSON(modelJSON), mapFromJSON(configJSON)
	return &value, nil
}
func (b *DurableBackend) GetProvider(_ string, providerID string) (*product.ProviderConfiguration, error) {
	var value product.ProviderConfiguration
	var modelJSON, configJSON []byte
	err := b.SQL.DB.QueryRowContext(context.Background(), `SELECT id::text,workspace_id::text,kind,adapter_key,display_name,status,COALESCE(credential_ref,''),model_defaults,config,revision,last_validated_at FROM provider_configurations WHERE id=$1 AND workspace_id=$2 AND deleted_at IS NULL`, providerID, b.Workspace).Scan(&value.ID, &value.WorkspaceID, &value.Kind, &value.AdapterKey, &value.DisplayName, &value.Status, &value.CredentialRef, &modelJSON, &configJSON, &value.Revision, &value.LastValidatedAt)
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	value.ModelDefaults, value.Config = mapFromJSON(modelJSON), mapFromJSON(configJSON)
	return &value, nil
}
func (b *DurableBackend) UpdateProvider(_ string, providerID string, revision int64, status, adapter, display string, config, models map[string]any, credentialRef *string) (*product.ProviderConfiguration, error) {
	current, err := b.GetProvider("", providerID)
	if err != nil {
		return nil, err
	}
	if status == "" {
		status = current.Status
	}
	if adapter == "" {
		adapter = current.AdapterKey
	}
	if display == "" {
		display = current.DisplayName
	}
	if config == nil {
		config = current.Config
	}
	if models == nil {
		models = current.ModelDefaults
	}
	credential := current.CredentialRef
	if credentialRef != nil {
		credential = *credentialRef
	}
	var value product.ProviderConfiguration
	var modelJSON, configJSON []byte
	err = b.SQL.DB.QueryRowContext(context.Background(), `UPDATE provider_configurations SET status=$3,adapter_key=$4,display_name=$5,credential_ref=NULLIF($6,''),model_defaults=$7,config=$8,revision=revision+1,updated_by=$9,updated_at=now(),deleted_at=CASE WHEN $3='deleted' THEN now() ELSE deleted_at END WHERE id=$1 AND workspace_id=$2 AND revision=$10 AND deleted_at IS NULL RETURNING id::text,workspace_id::text,kind,adapter_key,display_name,status,COALESCE(credential_ref,''),model_defaults,config,revision,last_validated_at`, providerID, b.Workspace, status, adapter, display, credential, jsonBytes(models), jsonBytes(config), b.UserID, revision).Scan(&value.ID, &value.WorkspaceID, &value.Kind, &value.AdapterKey, &value.DisplayName, &value.Status, &value.CredentialRef, &modelJSON, &configJSON, &value.Revision, &value.LastValidatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, product.ErrConflict
	}
	if err != nil {
		return nil, err
	}
	value.ModelDefaults, value.Config = mapFromJSON(modelJSON), mapFromJSON(configJSON)
	return &value, nil
}

func scanProvider(rows interface{ Scan(...any) error }) (product.ProviderConfiguration, error) {
	var value product.ProviderConfiguration
	var modelJSON, configJSON []byte
	err := rows.Scan(&value.ID, &value.WorkspaceID, &value.Kind, &value.AdapterKey, &value.DisplayName, &value.Status, &value.CredentialRef, &modelJSON, &configJSON, &value.Revision, &value.LastValidatedAt)
	value.ModelDefaults, value.Config = mapFromJSON(modelJSON), mapFromJSON(configJSON)
	return value, err
}

func (b *DurableBackend) presign(ctx context.Context, provider storage.UploadSession, requested []product.UploadPart) ([]product.UploadPart, error) {
	result := make([]product.UploadPart, 0, len(requested))
	for _, part := range requested {
		if part.PartNumber < 1 {
			return nil, errors.New("part number must be positive")
		}
		signed, err := b.Storage.PresignPart(ctx, provider, part.PartNumber)
		if err != nil {
			return nil, err
		}
		result = append(result, product.UploadPart{PartNumber: part.PartNumber, Method: signed.Method, URL: signed.URL, Headers: signed.Headers})
	}
	return result, nil
}

func projectFromRecord(record ProjectRecord) *product.Project {
	return &product.Project{ID: record.ID, WorkspaceID: record.WorkspaceID, Name: record.Name, WorkflowKey: record.WorkflowKey, Status: record.Status, Settings: mapFromJSON(record.Settings), Revision: record.Revision, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}
func assetFromRecord(record AssetRecord) *product.Asset {
	return &product.Asset{ID: record.ID, ProjectID: record.ProjectID, Kind: record.Kind, Status: record.Status, OriginalFilename: record.OriginalFilename, DeclaredMIME: record.DeclaredMIME, DetectedMIME: record.DetectedMIME, SizeBytes: record.SizeBytes, SHA256: record.SHA256, OriginalArtifactID: record.OriginalArtifactID, Revision: record.Revision, Metadata: mapFromJSON(record.Metadata), CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}
func uploadFromRecord(record UploadRecord, parts []product.UploadPart) *product.UploadSession {
	return &product.UploadSession{ID: record.ID, AssetID: record.AssetID, Status: record.Status, Method: "multipart", PartSize: record.PartSize, ExpiresAt: record.ExpiresAt, Parts: parts, ExpectedSize: record.ExpectedSize, ExpectedSHA256: record.ExpectedSHA256}
}
func providerFromRecord(record UploadRecord) storage.UploadSession {
	return storage.UploadSession{ID: record.ID, Locator: storage.StorageLocator{Backend: record.Backend, ObjectKey: record.StagingObjectKey}, ProviderID: record.ProviderUploadID, Multipart: record.Multipart, PartSize: record.PartSize, ExpiresAt: record.ExpiresAt}
}
func jsonBytes(value map[string]any) json.RawMessage {
	if value == nil {
		return json.RawMessage(`{}`)
	}
	encoded, _ := json.Marshal(value)
	return encoded
}
func mapFromJSON(value json.RawMessage) map[string]any {
	if len(value) == 0 {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal(value, &result); err != nil || result == nil {
		return map[string]any{}
	}
	return result
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	encoded, _ := json.Marshal(value)
	return mapFromJSON(encoded)
}

func jsonHash(value json.RawMessage) string {
	canonical, err := canonicalJSON(value)
	if err != nil {
		canonical = value
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
}

func timelineProject(document json.RawMessage) string {
	var value struct {
		ProjectID string `json:"project_id"`
	}
	_ = json.Unmarshal(document, &value)
	return value.ProjectID
}

func mapPersistenceError(err error) error {
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrScopeDenied):
		return product.ErrNotFound
	case errors.Is(err, ErrVersionConflict):
		return product.ErrConflict
	case errors.Is(err, ErrTransitionConflict):
		return product.ErrConflict
	default:
		return err
	}
}
