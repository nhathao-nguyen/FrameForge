package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/auth"
)

var ErrNotFound = errors.New("resource not found")
var ErrScopeDenied = errors.New("workspace scope denied")
var ErrVersionConflict = errors.New("version conflict")

type SQLStore struct{ DB *sql.DB }

func (s *SQLStore) CreateSession(ctx context.Context, userID, tokenHash, clientKind string, expiresAt time.Time) error {
	if userID == "" || tokenHash == "" || clientKind == "" || expiresAt.IsZero() {
		return errors.New("session metadata is incomplete")
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO auth_sessions(user_id,token_hash,client_kind,expires_at) VALUES($1,$2,$3,$4)`, userID, tokenHash, clientKind, expiresAt)
	return err
}

func (s *SQLStore) GetSession(ctx context.Context, tokenHash string) (auth.Principal, time.Time, error) {
	var principal auth.Principal
	var expiresAt time.Time
	err := s.DB.QueryRowContext(ctx, `SELECT ai.subject,u.id::text,wm.workspace_id::text,wm.role,s.expires_at FROM auth_sessions s JOIN users u ON u.id=s.user_id JOIN auth_identities ai ON ai.user_id=u.id AND ai.provider_kind='local' JOIN workspace_members wm ON wm.user_id=u.id JOIN workspaces w ON w.id=wm.workspace_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at > now() AND u.status='active' AND u.deleted_at IS NULL AND w.status='active' AND w.deleted_at IS NULL ORDER BY wm.created_at,wm.workspace_id LIMIT 1`, tokenHash).Scan(&principal.Subject, &principal.UserID, &principal.WorkspaceID, &principal.Role, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.Principal{}, time.Time{}, auth.ErrSessionNotFound
	}
	if err != nil {
		return auth.Principal{}, time.Time{}, err
	}
	principal.Scopes = []string{"*"}
	return principal, expiresAt, nil
}

func (s *SQLStore) RevokeSession(ctx context.Context, tokenHash string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now()),last_seen_at=now() WHERE token_hash=$1`, tokenHash)
	return err
}

func NewSQLStore(db *sql.DB) (*SQLStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &SQLStore{DB: db}, nil
}

type BootstrapInput struct{ Username, DisplayName, PasswordHash, WorkspaceSlug, WorkspaceName string }
type BootstrapResult struct{ UserID, WorkspaceID string }

// BootstrapLocalInstallation is safe to call repeatedly. It persists only a
// password verifier; the caller owns password KDF policy and never passes the
// plaintext credential to this repository.
func (s *SQLStore) BootstrapLocalInstallation(ctx context.Context, input BootstrapInput) (BootstrapResult, error) {
	if input.Username == "" || input.PasswordHash == "" || input.WorkspaceSlug == "" {
		return BootstrapResult{}, errors.New("local bootstrap requires username, password hash and workspace slug")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return BootstrapResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var userID, workspaceID string
	err = tx.QueryRowContext(ctx, `SELECT user_id::text FROM auth_identities WHERE provider_kind='local' AND subject=$1 FOR UPDATE`, input.Username).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.QueryRowContext(ctx, `INSERT INTO users(display_name,status) VALUES($1,'active') RETURNING id::text`, firstNonEmpty(input.DisplayName, input.Username)).Scan(&userID); err != nil {
			return BootstrapResult{}, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO auth_identities(user_id,provider_kind,subject,password_hash,password_changed_at) VALUES($1,'local',$2,$3,now())`, userID, input.Username, input.PasswordHash); err != nil {
			return BootstrapResult{}, err
		}
	} else if err != nil {
		return BootstrapResult{}, err
	}
	err = tx.QueryRowContext(ctx, `SELECT id::text FROM workspaces WHERE slug=$1 AND deleted_at IS NULL FOR UPDATE`, input.WorkspaceSlug).Scan(&workspaceID)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.QueryRowContext(ctx, `INSERT INTO workspaces(slug,name,status,created_by) VALUES($1,$2,'active',$3) RETURNING id::text`, input.WorkspaceSlug, firstNonEmpty(input.WorkspaceName, input.WorkspaceSlug), userID).Scan(&workspaceID); err != nil {
			return BootstrapResult{}, err
		}
	} else if err != nil {
		return BootstrapResult{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO workspace_members(workspace_id,user_id,role) VALUES($1,$2,'owner') ON CONFLICT (workspace_id,user_id) DO UPDATE SET role='owner', updated_at=now()`, workspaceID, userID); err != nil {
		return BootstrapResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return BootstrapResult{}, err
	}
	return BootstrapResult{UserID: userID, WorkspaceID: workspaceID}, nil
}

// EnsureDefaultWorkflow seeds the native, non-executing workflow and the
// declarative validation pipeline required for upload completion to enqueue a
// durable asset-probe Job. It does not execute nodes or transition Job state;
// those concerns remain Gate E/T300.
func (s *SQLStore) EnsureDefaultWorkflow(ctx context.Context, userID string) error {
	if userID == "" {
		return errors.New("workflow seed requires creator")
	}
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO workflows(workflow_key,name,description,status,created_by) VALUES('movie_recap','Movie recap','NH-Media native workflow definition','active',$1) ON CONFLICT DO NOTHING`, userID); err != nil {
		return err
	}
	_, err := s.EnsurePipelineDefinition(ctx, PipelineDefinitionInput{
		WorkflowKey:   "movie_recap",
		WorkflowName:  "Movie recap",
		Description:   "Native validation graph definition",
		SchemaVersion: "1.0",
		Definition:    json.RawMessage(`{"kind":"asset_validation","nodes":["asset_probe"]}`),
		Version:       1,
		Status:        "active",
		CreatedBy:     userID,
		Nodes: []PipelineNodeInput{{
			NodeKey:               "asset_probe",
			NodeType:              "asset_probe",
			DisplayName:           "Validate uploaded asset",
			ExecutionClass:        "probe",
			FailureMode:           "hard",
			ReviewPolicy:          "none",
			InputSchema:           json.RawMessage(`{"type":"object"}`),
			OutputSchema:          json.RawMessage(`{"type":"object"}`),
			ProducedArtifactRoles: json.RawMessage(`[]`),
			Config:                json.RawMessage(`{"purpose":"upload_validation"}`),
			TimeoutSec:            900,
			MaxAttempts:           1,
			RetryPolicy:           json.RawMessage(`{"retryable":false}`),
			CheckpointPolicy:      json.RawMessage(`{"enabled":false}`),
			ResourceRequirements:  json.RawMessage(`{"network":"none"}`),
			IdempotencyPolicy:     json.RawMessage(`{"key":"asset_id"}`),
			ProgressWeight:        1,
		}},
	})
	return err
}

func (s *SQLStore) RequireWorkspaceRole(ctx context.Context, userID, workspaceID string, allowed ...string) (string, error) {
	if userID == "" || workspaceID == "" {
		return "", ErrScopeDenied
	}
	args := make([]any, 0, len(allowed)+2)
	args = append(args, workspaceID, userID)
	query := `SELECT wm.role FROM workspace_members wm JOIN users u ON u.id=wm.user_id JOIN workspaces w ON w.id=wm.workspace_id WHERE wm.workspace_id=$1 AND wm.user_id=$2 AND u.status='active' AND u.deleted_at IS NULL AND w.status='active' AND w.deleted_at IS NULL`
	if len(allowed) > 0 {
		placeholders := make([]string, len(allowed))
		for i, role := range allowed {
			placeholders[i] = fmt.Sprintf("$%d", i+3)
			args = append(args, role)
		}
		query += " AND role IN (" + strings.Join(placeholders, ",") + ")"
	}
	var role string
	if err := s.DB.QueryRowContext(ctx, query, args...).Scan(&role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrScopeDenied
		}
		return "", err
	}
	return role, nil
}

type ProjectRecord struct {
	ID, WorkspaceID, Name, WorkflowKey, Status string
	Settings                                   json.RawMessage
	Revision                                   int64
	CreatedAt, UpdatedAt                       time.Time
}

func (s *SQLStore) CreateProject(ctx context.Context, userID, workspaceID, name, workflowKey string, settings json.RawMessage) (ProjectRecord, error) {
	if _, err := s.RequireWorkspaceRole(ctx, userID, workspaceID, "owner", "admin", "editor"); err != nil {
		return ProjectRecord{}, err
	}
	if len(settings) == 0 {
		settings = []byte(`{}`)
	}
	var result ProjectRecord
	err := s.DB.QueryRowContext(ctx, `INSERT INTO projects(workspace_id,name,workflow_id,status,settings,created_by,updated_by) SELECT $1,$2,w.id,'active',$3,$4,$4 FROM workflows w WHERE w.workflow_key=$5 AND w.status='active' AND (w.workspace_id IS NULL OR w.workspace_id=$1) ORDER BY (w.workspace_id IS NULL) DESC LIMIT 1 RETURNING id::text,workspace_id::text,name,(SELECT workflow_key FROM workflows WHERE id=projects.workflow_id),status,settings,revision,created_at,updated_at`, workspaceID, name, settings, userID, workflowKey).Scan(&result.ID, &result.WorkspaceID, &result.Name, &result.WorkflowKey, &result.Status, &result.Settings, &result.Revision, &result.CreatedAt, &result.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectRecord{}, ErrNotFound
	}
	return result, err
}
func (s *SQLStore) GetProject(ctx context.Context, userID, workspaceID, projectID string) (ProjectRecord, error) {
	if _, err := s.RequireWorkspaceRole(ctx, userID, workspaceID); err != nil {
		return ProjectRecord{}, ErrNotFound
	}
	var result ProjectRecord
	err := s.DB.QueryRowContext(ctx, `SELECT id::text,workspace_id::text,name,(SELECT workflow_key FROM workflows WHERE id=projects.workflow_id),status,settings,revision,created_at,updated_at FROM projects WHERE id=$1 AND workspace_id=$2 AND deleted_at IS NULL`, projectID, workspaceID).Scan(&result.ID, &result.WorkspaceID, &result.Name, &result.WorkflowKey, &result.Status, &result.Settings, &result.Revision, &result.CreatedAt, &result.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectRecord{}, ErrNotFound
	}
	return result, err
}
func (s *SQLStore) ListProjects(ctx context.Context, userID, workspaceID string, limit int) ([]ProjectRecord, error) {
	if _, err := s.RequireWorkspaceRole(ctx, userID, workspaceID); err != nil {
		return nil, ErrNotFound
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id::text,workspace_id::text,name,(SELECT workflow_key FROM workflows WHERE id=projects.workflow_id),status,settings,revision,created_at,updated_at FROM projects WHERE workspace_id=$1 AND deleted_at IS NULL ORDER BY created_at,id LIMIT $2`, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ProjectRecord{}
	for rows.Next() {
		var value ProjectRecord
		if err := rows.Scan(&value.ID, &value.WorkspaceID, &value.Name, &value.WorkflowKey, &value.Status, &value.Settings, &value.Revision, &value.CreatedAt, &value.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
func (s *SQLStore) UpdateProject(ctx context.Context, userID, workspaceID, projectID string, expectedRevision int64, name, status *string, settings json.RawMessage) (ProjectRecord, error) {
	if _, err := s.RequireWorkspaceRole(ctx, userID, workspaceID, "owner", "admin", "editor"); err != nil {
		return ProjectRecord{}, ErrNotFound
	}
	var result ProjectRecord
	err := s.DB.QueryRowContext(ctx, `UPDATE projects SET name=COALESCE($3,name),status=COALESCE($4,status),settings=COALESCE($5,settings),revision=revision+1,updated_by=$6,updated_at=now() WHERE id=$1 AND workspace_id=$2 AND revision=$7 AND deleted_at IS NULL RETURNING id::text,workspace_id::text,name,(SELECT workflow_key FROM workflows WHERE id=projects.workflow_id),status,settings,revision,created_at,updated_at`, projectID, workspaceID, nullableString(name), nullableString(status), nullableJSON(settings), userID, expectedRevision).Scan(&result.ID, &result.WorkspaceID, &result.Name, &result.WorkflowKey, &result.Status, &result.Settings, &result.Revision, &result.CreatedAt, &result.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectRecord{}, ErrVersionConflict
	}
	return result, err
}

type PipelineNodeInput struct {
	NodeKey, NodeType, DisplayName, ExecutionClass string
	Optional                                       bool
	FailureMode, ReviewPolicy                      string
	InputSchema, OutputSchema                      json.RawMessage
	RequiredArtifactRoles, ProducedArtifactRoles   json.RawMessage
	Config, RetryPolicy, CheckpointPolicy          json.RawMessage
	ResourceRequirements, IdempotencyPolicy        json.RawMessage
	TimeoutSec, MaxAttempts                        int
	ProgressWeight                                 float64
}

type PipelineDependencyInput struct {
	NodeKey, DependsOnNodeKey string
	Condition                 json.RawMessage
	Required                  bool
}

type PipelineDefinitionInput struct {
	WorkflowKey, WorkflowName, Description string
	SchemaVersion                          string
	Definition                             json.RawMessage
	Version                                int
	Status                                 string
	CreatedBy                              string
	Nodes                                  []PipelineNodeInput
	Dependencies                           []PipelineDependencyInput
}

// EnsurePipelineDefinition stores a native, versioned graph definition. It
// never executes a node and returns an existing version only when its content
// hash is identical. Active rows are protected again by the database trigger.
func (s *SQLStore) EnsurePipelineDefinition(ctx context.Context, input PipelineDefinitionInput) (string, error) {
	if input.WorkflowKey == "" || input.WorkflowName == "" || input.SchemaVersion == "" || input.CreatedBy == "" || input.Version < 1 {
		return "", errors.New("workflow and pipeline identity are required")
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.Status != "draft" && input.Status != "active" && input.Status != "deprecated" && input.Status != "disabled" {
		return "", errors.New("invalid pipeline status")
	}
	definition, err := canonicalJSON(input.Definition)
	if err != nil {
		return "", fmt.Errorf("pipeline definition: %w", err)
	}
	digest := sha256.Sum256(definition)
	contentHash := hex.EncodeToString(digest[:])
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	var workflowID string
	err = tx.QueryRowContext(ctx, `SELECT id::text FROM workflows WHERE workspace_id IS NULL AND workflow_key=$1 FOR UPDATE`, input.WorkflowKey).Scan(&workflowID)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `INSERT INTO workflows(workflow_key,name,description,status,created_by) VALUES($1,$2,NULLIF($3,''),$4,$5) RETURNING id::text`, input.WorkflowKey, input.WorkflowName, input.Description, input.Status, input.CreatedBy).Scan(&workflowID)
	}
	if err != nil {
		return "", err
	}
	var pipelineID, existingHash string
	err = tx.QueryRowContext(ctx, `SELECT id::text,content_hash FROM pipelines WHERE workflow_id=$1 AND version=$2`, workflowID, input.Version).Scan(&pipelineID, &existingHash)
	if err == nil {
		if strings.TrimSpace(existingHash) != contentHash {
			return "", fmt.Errorf("pipeline version %d already contains different content", input.Version)
		}
		if err := tx.Commit(); err != nil {
			return "", err
		}
		return pipelineID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if err = tx.QueryRowContext(ctx, `INSERT INTO pipelines(workflow_id,version,status,schema_version,definition,content_hash,created_by,activated_at) VALUES($1,$2,$3,$4,$5,$6,$7,CASE WHEN $3='active' THEN now() END) RETURNING id::text`, workflowID, input.Version, input.Status, input.SchemaVersion, definition, contentHash, input.CreatedBy).Scan(&pipelineID); err != nil {
		return "", err
	}
	nodeIDs := make(map[string]string, len(input.Nodes))
	for _, node := range input.Nodes {
		if node.NodeKey == "" || node.NodeType == "" || node.DisplayName == "" || node.ExecutionClass == "" || node.FailureMode == "" || node.ReviewPolicy == "" || node.TimeoutSec < 1 || node.MaxAttempts < 1 {
			return "", errors.New("pipeline node is incomplete")
		}
		var nodeID string
		if err := tx.QueryRowContext(ctx, `INSERT INTO pipeline_nodes(pipeline_id,node_key,node_type,display_name,execution_class,is_optional,failure_mode,review_policy,input_schema,output_schema,required_artifact_roles,produced_artifact_roles,config,timeout_sec,max_attempts,retry_policy,checkpoint_policy,resource_requirements,idempotency_policy,progress_weight) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20) RETURNING id::text`, pipelineID, node.NodeKey, node.NodeType, node.DisplayName, node.ExecutionClass, node.Optional, node.FailureMode, node.ReviewPolicy, nullableJSON(node.InputSchema), nullableJSON(node.OutputSchema), nullableJSON(node.RequiredArtifactRoles), nullableJSON(node.ProducedArtifactRoles), jsonOrEmpty(node.Config), node.TimeoutSec, node.MaxAttempts, jsonOrEmpty(node.RetryPolicy), jsonOrEmpty(node.CheckpointPolicy), jsonOrEmpty(node.ResourceRequirements), jsonOrEmpty(node.IdempotencyPolicy), node.ProgressWeight).Scan(&nodeID); err != nil {
			return "", err
		}
		nodeIDs[node.NodeKey] = nodeID
	}
	for _, dependency := range input.Dependencies {
		nodeID, nodeOK := nodeIDs[dependency.NodeKey]
		dependsOnID, parentOK := nodeIDs[dependency.DependsOnNodeKey]
		if !nodeOK || !parentOK || dependency.NodeKey == dependency.DependsOnNodeKey {
			return "", errors.New("pipeline dependency references an invalid node")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO pipeline_node_dependencies(pipeline_id,node_id,depends_on_node_id,condition,required) VALUES($1,$2,$3,$4,$5)`, pipelineID, nodeID, dependsOnID, jsonOrEmpty(dependency.Condition), dependency.Required); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return pipelineID, nil
}

type SecretMetadata struct {
	ID, WorkspaceID, Purpose, KeyVersion, Status string
	AAD                                          json.RawMessage
	CreatedAt, UpdatedAt                         time.Time
}

// StoreSecret stores ciphertext produced by the application SecretStore. The
// repository intentionally has no plaintext-secret parameter or read method.
func (s *SQLStore) StoreSecret(ctx context.Context, workspaceID, purpose string, ciphertext, nonce []byte, keyVersion string, aad json.RawMessage, createdBy string) (SecretMetadata, error) {
	if workspaceID == "" || purpose == "" || len(ciphertext) == 0 || len(nonce) == 0 || keyVersion == "" || createdBy == "" {
		return SecretMetadata{}, errors.New("encrypted secret metadata is incomplete")
	}
	if len(aad) == 0 {
		aad = []byte(`{}`)
	}
	var result SecretMetadata
	err := s.DB.QueryRowContext(ctx, `INSERT INTO secret_records(workspace_id,purpose,ciphertext,nonce,key_version,aad,status,created_by) VALUES($1,$2,$3,$4,$5,$6,'active',$7) RETURNING id::text,workspace_id::text,purpose,key_version,status,aad,created_at,updated_at`, workspaceID, purpose, ciphertext, nonce, keyVersion, aad, createdBy).Scan(&result.ID, &result.WorkspaceID, &result.Purpose, &result.KeyVersion, &result.Status, &result.AAD, &result.CreatedAt, &result.UpdatedAt)
	return result, err
}

func (s *SQLStore) GetSecretMetadata(ctx context.Context, userID, workspaceID, secretID string) (SecretMetadata, error) {
	if _, err := s.RequireWorkspaceRole(ctx, userID, workspaceID); err != nil {
		return SecretMetadata{}, ErrScopeDenied
	}
	var result SecretMetadata
	err := s.DB.QueryRowContext(ctx, `SELECT id::text,workspace_id::text,purpose,key_version,status,aad,created_at,updated_at FROM secret_records WHERE id=$1 AND workspace_id=$2 AND deleted_at IS NULL`, secretID, workspaceID).Scan(&result.ID, &result.WorkspaceID, &result.Purpose, &result.KeyVersion, &result.Status, &result.AAD, &result.CreatedAt, &result.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SecretMetadata{}, ErrNotFound
	}
	return result, err
}

type AssetRecord struct {
	ID, ProjectID, Kind, Status, OriginalFilename, DeclaredMIME, DetectedMIME, OriginalArtifactID, SHA256 string
	SizeBytes                                                                                             int64
	Revision                                                                                              int64
	Metadata, ValidationReport                                                                            json.RawMessage
	CreatedAt, UpdatedAt                                                                                  time.Time
}

func (s *SQLStore) CreateAsset(ctx context.Context, userID, workspaceID, projectID, kind, filename, mime, checksum string, size int64) (AssetRecord, error) {
	if _, err := s.GetProject(ctx, userID, workspaceID, projectID); err != nil {
		return AssetRecord{}, err
	}
	var value AssetRecord
	err := s.DB.QueryRowContext(ctx, `INSERT INTO assets(project_id,kind,status,original_filename,declared_mime_type,size_bytes,sha256,created_by,updated_by) SELECT $1,$2,'uploading',$3,$4,$5,NULLIF($6,''),$7,$7 FROM projects WHERE id=$1 AND workspace_id=$8 RETURNING id::text,project_id::text,kind,status,COALESCE(original_filename,''),COALESCE(declared_mime_type,''),COALESCE(detected_mime_type,''),COALESCE(original_artifact_id::text,''),COALESCE(sha256,''),COALESCE(size_bytes,0),revision,metadata,validation_report,created_at,updated_at`, projectID, kind, filename, mime, size, checksum, userID, workspaceID).Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Status, &value.OriginalFilename, &value.DeclaredMIME, &value.DetectedMIME, &value.OriginalArtifactID, &value.SHA256, &value.SizeBytes, &value.Revision, &value.Metadata, &value.ValidationReport, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}
func (s *SQLStore) GetAsset(ctx context.Context, userID, workspaceID, projectID, assetID string) (AssetRecord, error) {
	if _, err := s.GetProject(ctx, userID, workspaceID, projectID); err != nil {
		return AssetRecord{}, ErrNotFound
	}
	var value AssetRecord
	err := s.DB.QueryRowContext(ctx, `SELECT id::text,project_id::text,kind,status,COALESCE(original_filename,''),COALESCE(declared_mime_type,''),COALESCE(detected_mime_type,''),COALESCE(original_artifact_id::text,''),COALESCE(sha256,''),COALESCE(size_bytes,0),revision,metadata,validation_report,created_at,updated_at FROM assets WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, assetID, projectID).Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Status, &value.OriginalFilename, &value.DeclaredMIME, &value.DetectedMIME, &value.OriginalArtifactID, &value.SHA256, &value.SizeBytes, &value.Revision, &value.Metadata, &value.ValidationReport, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AssetRecord{}, ErrNotFound
	}
	return value, err
}

func (s *SQLStore) ListAssets(ctx context.Context, userID, workspaceID, projectID string, limit int) ([]AssetRecord, error) {
	if _, err := s.GetProject(ctx, userID, workspaceID, projectID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id::text,project_id::text,kind,status,COALESCE(original_filename,''),COALESCE(declared_mime_type,''),COALESCE(detected_mime_type,''),COALESCE(original_artifact_id::text,''),COALESCE(sha256,''),COALESCE(size_bytes,0),revision,metadata,validation_report,created_at,updated_at FROM assets WHERE project_id=$1 AND deleted_at IS NULL ORDER BY created_at,id LIMIT $2`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []AssetRecord{}
	for rows.Next() {
		var value AssetRecord
		if err := rows.Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Status, &value.OriginalFilename, &value.DeclaredMIME, &value.DetectedMIME, &value.OriginalArtifactID, &value.SHA256, &value.SizeBytes, &value.Revision, &value.Metadata, &value.ValidationReport, &value.CreatedAt, &value.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *SQLStore) UpdateAsset(ctx context.Context, userID, workspaceID, projectID, assetID string, expectedRevision int64, metadata json.RawMessage) (AssetRecord, error) {
	if _, err := s.GetProject(ctx, userID, workspaceID, projectID); err != nil {
		return AssetRecord{}, err
	}
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	var value AssetRecord
	err := s.DB.QueryRowContext(ctx, `UPDATE assets SET metadata=$4,revision=revision+1,updated_by=$5,updated_at=now() WHERE id=$1 AND project_id=$2 AND revision=$3 AND deleted_at IS NULL RETURNING id::text,project_id::text,kind,status,COALESCE(original_filename,''),COALESCE(declared_mime_type,''),COALESCE(detected_mime_type,''),COALESCE(original_artifact_id::text,''),COALESCE(sha256,''),COALESCE(size_bytes,0),revision,metadata,validation_report,created_at,updated_at`, assetID, projectID, expectedRevision, metadata, userID).Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Status, &value.OriginalFilename, &value.DeclaredMIME, &value.DetectedMIME, &value.OriginalArtifactID, &value.SHA256, &value.SizeBytes, &value.Revision, &value.Metadata, &value.ValidationReport, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AssetRecord{}, ErrVersionConflict
	}
	return value, err
}

func (s *SQLStore) SetAssetStatus(ctx context.Context, userID, workspaceID, projectID, assetID, status, artifactID string) (AssetRecord, error) {
	if _, err := s.GetProject(ctx, userID, workspaceID, projectID); err != nil {
		return AssetRecord{}, err
	}
	var value AssetRecord
	err := s.DB.QueryRowContext(ctx, `UPDATE assets SET status=$3,original_artifact_id=NULLIF($4,'')::uuid,revision=revision+1,updated_by=$5,updated_at=now(),deleted_at=CASE WHEN $3='deleted' THEN now() ELSE deleted_at END WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL RETURNING id::text,project_id::text,kind,status,COALESCE(original_filename,''),COALESCE(declared_mime_type,''),COALESCE(detected_mime_type,''),COALESCE(original_artifact_id::text,''),COALESCE(sha256,''),COALESCE(size_bytes,0),revision,metadata,validation_report,created_at,updated_at`, assetID, projectID, status, artifactID, userID).Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Status, &value.OriginalFilename, &value.DeclaredMIME, &value.DetectedMIME, &value.OriginalArtifactID, &value.SHA256, &value.SizeBytes, &value.Revision, &value.Metadata, &value.ValidationReport, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AssetRecord{}, ErrNotFound
	}
	return value, err
}

type UploadRecord struct {
	ID, AssetID, Backend, StagingObjectKey, ProviderUploadID, Status string
	Multipart                                                        bool
	PartSize, ExpectedSize, CompletedSize                            int64
	ExpectedSHA256                                                   string
	ExpiresAt, CreatedAt, UpdatedAt                                  time.Time
}

type JobRecord struct {
	ID, ProjectID, Kind, Status string
	Command                     json.RawMessage
	CreatedAt                   time.Time
}

func (s *SQLStore) GetAssetProbeJob(ctx context.Context, userID, workspaceID, projectID, assetID string) (JobRecord, error) {
	if _, err := s.GetProject(ctx, userID, workspaceID, projectID); err != nil {
		return JobRecord{}, err
	}
	var value JobRecord
	err := s.DB.QueryRowContext(ctx, `SELECT id::text,project_id::text,kind,status,command,created_at FROM jobs WHERE project_id=$1 AND kind='asset_probe' AND command->>'asset_id'=$2 ORDER BY created_at DESC,id DESC LIMIT 1`, projectID, assetID).Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Status, &value.Command, &value.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return JobRecord{}, ErrNotFound
	}
	return value, err
}

func (s *SQLStore) CreateProjectJob(ctx context.Context, userID, workspaceID, projectID, kind string, command, inputSnapshot json.RawMessage) (JobRecord, error) {
	if kind != "pipeline" && kind != "render" && kind != "asset_probe" && kind != "analysis" && kind != "export" {
		return JobRecord{}, errors.New("invalid job kind")
	}
	if _, err := s.GetProject(ctx, userID, workspaceID, projectID); err != nil {
		return JobRecord{}, err
	}
	if len(command) == 0 {
		command = []byte(`{}`)
	}
	if len(inputSnapshot) == 0 {
		inputSnapshot = command
	}
	var workflowID, pipelineID string
	var pipelineSnapshot json.RawMessage
	err := s.DB.QueryRowContext(ctx, `SELECT w.id::text,p.id::text,jsonb_build_object('pipeline_id',p.id::text,'version',p.version,'schema_version',p.schema_version,'definition',p.definition,'nodes',COALESCE((SELECT jsonb_agg(jsonb_build_object('node_key',n.node_key,'node_type',n.node_type,'execution_class',n.execution_class,'config',n.config) ORDER BY n.node_key) FROM pipeline_nodes n WHERE n.pipeline_id=p.id),'[]'::jsonb)) FROM projects pr JOIN workflows w ON w.id=pr.workflow_id JOIN pipelines p ON p.workflow_id=w.id AND p.status='active' WHERE pr.id=$1 AND pr.workspace_id=$2 ORDER BY p.version DESC LIMIT 1`, projectID, workspaceID).Scan(&workflowID, &pipelineID, &pipelineSnapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return JobRecord{}, ErrNotFound
	}
	if err != nil {
		return JobRecord{}, err
	}
	var value JobRecord
	err = s.DB.QueryRowContext(ctx, `INSERT INTO jobs(project_id,workflow_id,pipeline_id,kind,mode,status,requested_by,command,input_snapshot,pipeline_snapshot) VALUES($1,$2,$3,$4,'automatic','created',$5,$6,$7,$8) RETURNING id::text,project_id::text,kind,status,command,created_at`, projectID, workflowID, pipelineID, kind, userID, command, inputSnapshot, pipelineSnapshot).Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Status, &value.Command, &value.CreatedAt)
	return value, err
}

// CompleteUploadAndCreateProbeJob makes the database side of upload
// completion atomic: the upload is completed, the Asset becomes validating,
// and one durable validation Job is inserted from the immutable pipeline
// snapshot. Storage completion happens before this call; a database failure
// therefore leaves a staged object for reconciliation, never a ready Asset.
func (s *SQLStore) CompleteUploadAndCreateProbeJob(ctx context.Context, userID, workspaceID, projectID, assetID, uploadID string, completedSize int64, command, inputSnapshot json.RawMessage) (UploadRecord, JobRecord, error) {
	if _, err := s.GetAsset(ctx, userID, workspaceID, projectID, assetID); err != nil {
		return UploadRecord{}, JobRecord{}, err
	}
	if len(command) == 0 {
		command = []byte(`{"asset_id":""}`)
	}
	if len(inputSnapshot) == 0 {
		inputSnapshot = command
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return UploadRecord{}, JobRecord{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var upload UploadRecord
	err = tx.QueryRowContext(ctx, `UPDATE asset_uploads u SET status='completed',completed_size=$3,updated_at=now() FROM assets a JOIN projects p ON p.id=a.project_id WHERE u.id=$1 AND u.asset_id=$2 AND a.id=$2 AND p.id=$4 AND p.workspace_id=$5 AND u.status='active' AND u.expires_at > now() RETURNING u.id::text,u.asset_id::text,u.storage_backend,u.staging_object_key,COALESCE(u.provider_upload_id,''),u.status,u.multipart,COALESCE(u.part_size,0),COALESCE(u.expected_size,0),COALESCE(u.completed_size,0),COALESCE(u.expected_sha256,''),u.expires_at,u.created_at,u.updated_at`, uploadID, assetID, completedSize, projectID, workspaceID).Scan(&upload.ID, &upload.AssetID, &upload.Backend, &upload.StagingObjectKey, &upload.ProviderUploadID, &upload.Status, &upload.Multipart, &upload.PartSize, &upload.ExpectedSize, &upload.CompletedSize, &upload.ExpectedSHA256, &upload.ExpiresAt, &upload.CreatedAt, &upload.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return UploadRecord{}, JobRecord{}, ErrVersionConflict
	}
	if err != nil {
		return UploadRecord{}, JobRecord{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE assets SET status='validating',revision=revision+1,updated_by=$3,updated_at=now() WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, assetID, projectID, userID); err != nil {
		return UploadRecord{}, JobRecord{}, err
	}
	var workflowID, pipelineID string
	var pipelineSnapshot json.RawMessage
	err = tx.QueryRowContext(ctx, `SELECT w.id::text,p.id::text,jsonb_build_object('pipeline_id',p.id::text,'version',p.version,'schema_version',p.schema_version,'definition',p.definition,'nodes',COALESCE((SELECT jsonb_agg(jsonb_build_object('node_key',n.node_key,'node_type',n.node_type,'execution_class',n.execution_class,'config',n.config) ORDER BY n.node_key) FROM pipeline_nodes n WHERE n.pipeline_id=p.id),'[]'::jsonb)) FROM projects pr JOIN workflows w ON w.id=pr.workflow_id JOIN pipelines p ON p.workflow_id=w.id AND p.status='active' WHERE pr.id=$1 AND pr.workspace_id=$2 ORDER BY p.version DESC LIMIT 1`, projectID, workspaceID).Scan(&workflowID, &pipelineID, &pipelineSnapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return UploadRecord{}, JobRecord{}, ErrNotFound
	}
	if err != nil {
		return UploadRecord{}, JobRecord{}, err
	}
	var job JobRecord
	err = tx.QueryRowContext(ctx, `INSERT INTO jobs(project_id,workflow_id,pipeline_id,kind,mode,status,requested_by,command,input_snapshot,pipeline_snapshot) VALUES($1,$2,$3,'asset_probe','automatic','created',$4,$5,$6,$7) RETURNING id::text,project_id::text,kind,status,command,created_at`, projectID, workflowID, pipelineID, userID, command, inputSnapshot, pipelineSnapshot).Scan(&job.ID, &job.ProjectID, &job.Kind, &job.Status, &job.Command, &job.CreatedAt)
	if err != nil {
		return UploadRecord{}, JobRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return UploadRecord{}, JobRecord{}, err
	}
	return upload, job, nil
}

func (s *SQLStore) CreateUpload(ctx context.Context, userID, workspaceID, projectID, assetID, backend, stagingKey, providerUploadID string, multipart bool, partSize, expectedSize int64, expectedSHA256 string, expiresAt time.Time) (UploadRecord, error) {
	if _, err := s.GetAsset(ctx, userID, workspaceID, projectID, assetID); err != nil {
		return UploadRecord{}, err
	}
	var value UploadRecord
	err := s.DB.QueryRowContext(ctx, `INSERT INTO asset_uploads(asset_id,storage_backend,staging_object_key,provider_upload_id,status,multipart,part_size,expected_size,expected_sha256,expires_at,created_by) VALUES($1,$2,$3,NULLIF($4,''),'active',$5,$6,$7,NULLIF($8,''),$9,$10) RETURNING id::text,asset_id::text,storage_backend,staging_object_key,COALESCE(provider_upload_id,''),status,multipart,COALESCE(part_size,0),COALESCE(expected_size,0),COALESCE(completed_size,0),COALESCE(expected_sha256,''),expires_at,created_at,updated_at`, assetID, backend, stagingKey, providerUploadID, multipart, partSize, expectedSize, expectedSHA256, expiresAt, userID).Scan(&value.ID, &value.AssetID, &value.Backend, &value.StagingObjectKey, &value.ProviderUploadID, &value.Status, &value.Multipart, &value.PartSize, &value.ExpectedSize, &value.CompletedSize, &value.ExpectedSHA256, &value.ExpiresAt, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func (s *SQLStore) GetUpload(ctx context.Context, userID, workspaceID, projectID, assetID, uploadID string) (UploadRecord, error) {
	if _, err := s.GetAsset(ctx, userID, workspaceID, projectID, assetID); err != nil {
		return UploadRecord{}, err
	}
	var value UploadRecord
	err := s.DB.QueryRowContext(ctx, `SELECT u.id::text,u.asset_id::text,u.storage_backend,u.staging_object_key,COALESCE(u.provider_upload_id,''),u.status,u.multipart,COALESCE(u.part_size,0),COALESCE(u.expected_size,0),COALESCE(u.completed_size,0),COALESCE(u.expected_sha256,''),u.expires_at,u.created_at,u.updated_at FROM asset_uploads u JOIN assets a ON a.id=u.asset_id JOIN projects p ON p.id=a.project_id WHERE u.id=$1 AND u.asset_id=$2 AND p.id=$3 AND p.workspace_id=$4`, uploadID, assetID, projectID, workspaceID).Scan(&value.ID, &value.AssetID, &value.Backend, &value.StagingObjectKey, &value.ProviderUploadID, &value.Status, &value.Multipart, &value.PartSize, &value.ExpectedSize, &value.CompletedSize, &value.ExpectedSHA256, &value.ExpiresAt, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return UploadRecord{}, ErrNotFound
	}
	return value, err
}

func (s *SQLStore) CompleteUpload(ctx context.Context, userID, workspaceID, projectID, assetID, uploadID string, completedSize int64) (UploadRecord, error) {
	if _, err := s.GetAsset(ctx, userID, workspaceID, projectID, assetID); err != nil {
		return UploadRecord{}, err
	}
	var value UploadRecord
	err := s.DB.QueryRowContext(ctx, `UPDATE asset_uploads SET status='completed',completed_size=$3,updated_at=now() WHERE id=$1 AND asset_id=$2 AND status='active' AND expires_at > now() RETURNING id::text,asset_id::text,storage_backend,staging_object_key,COALESCE(provider_upload_id,''),status,multipart,COALESCE(part_size,0),COALESCE(expected_size,0),COALESCE(completed_size,0),COALESCE(expected_sha256,''),expires_at,created_at,updated_at`, uploadID, assetID, completedSize).Scan(&value.ID, &value.AssetID, &value.Backend, &value.StagingObjectKey, &value.ProviderUploadID, &value.Status, &value.Multipart, &value.PartSize, &value.ExpectedSize, &value.CompletedSize, &value.ExpectedSHA256, &value.ExpiresAt, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return UploadRecord{}, ErrVersionConflict
	}
	return value, err
}

func (s *SQLStore) AbortUpload(ctx context.Context, userID, workspaceID, projectID, assetID, uploadID string) error {
	if _, err := s.GetAsset(ctx, userID, workspaceID, projectID, assetID); err != nil {
		return err
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE asset_uploads SET status='aborted',updated_at=now() WHERE id=$1 AND asset_id=$2 AND status='active'`, uploadID, assetID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) ClaimIdempotency(ctx context.Context, workspaceID, key, request string, status int, body json.RawMessage, resourceType, resourceID string) (json.RawMessage, bool, error) {
	if key == "" {
		return nil, false, nil
	}
	digest := sha256.Sum256([]byte(request))
	hash := hex.EncodeToString(digest[:])
	var existingHash string
	var existingBody []byte
	// The insert is the atomic claim. A preceding SELECT would race under
	// concurrent retries and could create duplicate side effects before the
	// unique key is observed.
	err := s.DB.QueryRowContext(ctx, `INSERT INTO idempotency_keys(workspace_id,key,request_hash,response_status,response_body,resource_type,resource_id) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,'')::uuid) ON CONFLICT (workspace_id,key) DO NOTHING RETURNING response_body`, workspaceID, key, hash, status, body, resourceType, resourceID).Scan(&existingBody)
	if err == nil {
		return nil, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT request_hash,response_body FROM idempotency_keys WHERE workspace_id=$1 AND key=$2`, workspaceID, key).Scan(&existingHash, &existingBody); err != nil {
		return nil, false, err
	}
	if existingHash != hash {
		return nil, false, ErrVersionConflict
	}
	return existingBody, true, nil
}

func (s *SQLStore) FinalizeIdempotency(ctx context.Context, workspaceID, key, request string, status int, body json.RawMessage) error {
	if key == "" {
		return nil
	}
	digest := sha256.Sum256([]byte(request))
	hash := hex.EncodeToString(digest[:])
	result, err := s.DB.ExecContext(ctx, `UPDATE idempotency_keys SET response_status=$3,response_body=$4,updated_at=now() WHERE workspace_id=$1 AND key=$2 AND request_hash=$5`, workspaceID, key, status, body, hash)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (s *SQLStore) IdempotencyStatus(ctx context.Context, workspaceID, key string) (int, error) {
	var status int
	err := s.DB.QueryRowContext(ctx, `SELECT response_status FROM idempotency_keys WHERE workspace_id=$1 AND key=$2`, workspaceID, key).Scan(&status)
	return status, err
}

type EventInput struct {
	SchemaVersion, EventType, CorrelationID                                          string
	WorkspaceID, ProjectID, JobID, PipelineRunID, JobStepID, PipelineNodeID, NodeKey string
	Payload                                                                          json.RawMessage
}

func (s *SQLStore) AppendEventAndOutbox(ctx context.Context, input EventInput) (int64, error) {
	if len(input.Payload) == 0 {
		input.Payload = []byte(`{}`)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, input.JobID); err != nil {
		return 0, err
	}
	var seq int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence),0)+1 FROM job_events WHERE job_id=$1`, input.JobID).Scan(&seq); err != nil {
		return 0, err
	}
	var eventID string
	if err = tx.QueryRowContext(ctx, `INSERT INTO job_events(schema_version,workspace_id,project_id,job_id,pipeline_run_id,job_step_id,pipeline_node_id,node_key,sequence,event_type,payload,correlation_id,occurred_at) VALUES($1,NULLIF($2,'')::uuid,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,NULLIF($7,'')::uuid,NULLIF($8,''),$9,$10,$11,$12,now()) RETURNING event_id::text`, input.SchemaVersion, input.WorkspaceID, input.ProjectID, input.JobID, input.PipelineRunID, input.JobStepID, input.PipelineNodeID, input.NodeKey, seq, input.EventType, input.Payload, input.CorrelationID).Scan(&eventID); err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO outbox_events(id,aggregate_type,aggregate_id,event_type,payload,status) VALUES($1,'job',$2,$3,$4,'pending')`, eventID, input.EventType, input.Payload); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return seq, nil
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
func nullableJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func jsonOrEmpty(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	return value
}

func canonicalJSON(value json.RawMessage) (json.RawMessage, error) {
	if len(value) == 0 {
		return json.RawMessage(`{}`), nil
	}
	decoder := json.NewDecoder(strings.NewReader(string(value)))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	return json.Marshal(decoded)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
