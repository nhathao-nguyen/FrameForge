package product

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

var ErrNotFound = errors.New("resource not found")
var ErrConflict = errors.New("resource conflict")
var ErrIdempotencyConflict = errors.New("idempotency conflict")
var ErrForbidden = errors.New("forbidden")

// Backend is the native Product resource boundary. Store is the deterministic
// in-memory implementation used by unit tests; the API can replace it with a
// durable persistence adapter without changing HTTP contracts.
type Backend interface {
	WorkspaceID() string
	CreateProject(string, string, string, map[string]any) (*Project, error)
	ListProjects(string, int) ([]Project, error)
	GetProject(string, string) (*Project, error)
	UpdateProject(string, string, int64, *string, *string, map[string]any) (*Project, error)
	CreateAsset(string, string, string, string, string, int64, string) (*Asset, error)
	ListAssets(string, string, int) ([]Asset, error)
	GetAsset(string, string, string) (*Asset, error)
	UpdateAsset(string, string, string, int64, map[string]any) (*Asset, error)
	SetAssetStatus(string, string, string, string, string) (*Asset, error)
	CreateUpload(context.Context, string, string, string, int64, string, string, []UploadPart, string) (*UploadSession, error)
	GetUpload(string, string, string, string) (*UploadSession, error)
	PresignUploadParts(context.Context, string, string, string, string, []int) (*UploadSession, error)
	CompleteUpload(context.Context, string, string, string, string, string, []UploadPart) (*UploadSession, *Job, error)
	AbortUpload(context.Context, string, string, string, string) error
	CreateScript(string, string, string, string, map[string]any) (*Script, *ScriptVersion, error)
	GetScript(string, string, string) (*Script, error)
	CreateScriptVersion(string, string, string, string, string, string, map[string]any, int64) (*ScriptVersion, error)
	GetScriptVersion(string, string, string) (*ScriptVersion, error)
	SetScriptVersionStatus(string, string, string, string, string) (*ScriptVersion, error)
	CreateNarration(string, string, string, string, map[string]any, map[string]any) (*Narration, error)
	GetNarration(string, string, string) (*Narration, error)
	CreateTimeline(string, string, string, json.RawMessage) (*Timeline, *TimelineVersion, error)
	GetTimeline(string, string, string) (*Timeline, error)
	GetTimelineVersion(string, string, string) (*TimelineVersion, error)
	AddTimelineVersion(string, string, string, string, string, json.RawMessage, int64) (*TimelineVersion, error)
	SetTimelineVersionStatus(string, string, string, string, string) (*TimelineVersion, error)
	ValidateTimeline(string, string, string) (string, error)
	CreateRenderProfile(string, string, int, map[string]any) (*RenderProfile, error)
	GetRenderProfile(string, string, int) (*RenderProfile, error)
	UpdateRenderProfile(string, string, int, map[string]any) (*RenderProfile, error)
	SetRenderProfileStatus(string, string, int, string) (*RenderProfile, error)
	CreateRender(string, string, string, string, int, map[string]any) (*Render, error)
	GetJob(string, string, string) (*Job, error)
	StartJob(string, string, string) (*Job, error)
	PauseJob(string, string, string) (*Job, error)
	ResumeJob(string, string, string) (*Job, error)
	CancelJob(string, string, string) (*Job, error)
	ListJobEvents(string, string, string, int64, int) ([]JobEvent, error)
	CreateAnalysis(string, string, string, map[string]any, map[string]any) (*Analysis, error)
	GetAnalysis(string, string, string) (*Analysis, error)
	ListAnalyses(string, string) ([]Analysis, error)
	CreateScene(string, string, string, string, float64, float64, string) (*Scene, error)
	ListScenes(string, string) ([]Scene, error)
	GetScene(string, string, string) (*Scene, error)
	CreateCandidateGroup(string, string, []Candidate) (*CandidateGroup, error)
	GetCandidateGroup(string, string, string) (*CandidateGroup, error)
	ListProviders(string) ([]ProviderConfiguration, error)
	CreateProvider(string, string, string, string, map[string]any, map[string]any, string) (*ProviderConfiguration, error)
	GetProvider(string, string) (*ProviderConfiguration, error)
	UpdateProvider(string, string, int64, string, string, string, map[string]any, map[string]any, *string) (*ProviderConfiguration, error)
}

type Store struct {
	mu           sync.RWMutex
	workspaceID  string
	projects     map[string]*Project
	assets       map[string]*Asset
	uploads      map[string]*UploadSession
	scripts      map[string]*Script
	scriptVers   map[string]*ScriptVersion
	narrations   map[string]*Narration
	timelines    map[string]*Timeline
	timelineVers map[string]*TimelineVersion
	analyses     map[string]*Analysis
	scenes       map[string]*Scene
	candidates   map[string]*CandidateGroup
	providers    map[string]*ProviderConfiguration
	profiles     map[string]*RenderProfile
	renders      map[string]*Render
	jobs         map[string]*Job
	jobEvents    map[string][]JobEvent
	runs         map[string]*PipelineRun
	steps        map[string][]JobStep
	reviews      map[string]*Review
	idempotency  map[string]IdempotencyRecord
	storage      storage.StoragePort
	storageMu    sync.Mutex
	storageSlots map[string]storage.UploadSession
	staged       map[string]storage.StagedObject
}

type Project struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	Name        string         `json:"name"`
	WorkflowKey string         `json:"workflow_key"`
	Status      string         `json:"status"`
	Settings    map[string]any `json:"settings"`
	Revision    int64          `json:"revision"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}
type Asset struct {
	ID                 string         `json:"id"`
	ProjectID          string         `json:"project_id"`
	Kind               string         `json:"kind"`
	Status             string         `json:"status"`
	OriginalFilename   string         `json:"original_filename,omitempty"`
	DeclaredMIME       string         `json:"declared_mime_type,omitempty"`
	DetectedMIME       string         `json:"detected_mime_type,omitempty"`
	SizeBytes          int64          `json:"size_bytes,omitempty"`
	SHA256             string         `json:"sha256,omitempty"`
	OriginalArtifactID string         `json:"original_artifact_id,omitempty"`
	Revision           int64          `json:"revision"`
	Metadata           map[string]any `json:"metadata"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}
type UploadSession struct {
	ID             string       `json:"id"`
	AssetID        string       `json:"asset_id"`
	Status         string       `json:"status"`
	Method         string       `json:"method"`
	PartSize       int64        `json:"part_size"`
	ExpiresAt      time.Time    `json:"expires_at"`
	Parts          []UploadPart `json:"parts,omitempty"`
	ExpectedSize   int64        `json:"expected_size"`
	ExpectedSHA256 string       `json:"expected_sha256,omitempty"`
	RequestHash    string       `json:"-"`
}
type UploadPart struct {
	PartNumber int               `json:"part_number"`
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers"`
	ETag       string            `json:"etag,omitempty"`
}
type Script struct {
	ID               string    `json:"id"`
	ProjectID        string    `json:"project_id"`
	Status           string    `json:"status"`
	CurrentVersionID string    `json:"current_version_id,omitempty"`
	Revision         int64     `json:"revision"`
	Versions         []string  `json:"versions"`
	CreatedAt        time.Time `json:"created_at"`
}
type ScriptVersion struct {
	ID               string         `json:"id"`
	ScriptID         string         `json:"script_id"`
	ProjectID        string         `json:"project_id"`
	Version          int            `json:"version"`
	Status           string         `json:"status"`
	Language         string         `json:"language"`
	Origin           string         `json:"origin"`
	Content          map[string]any `json:"content"`
	ContentHash      string         `json:"content_hash"`
	BasedOnVersionID string         `json:"based_on_version_id,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}
type Narration struct {
	ID               string         `json:"id"`
	ProjectID        string         `json:"project_id"`
	ScriptVersionID  string         `json:"script_version_id"`
	Status           string         `json:"status"`
	ProviderConfigID string         `json:"provider_configuration_id,omitempty"`
	VoiceSnapshot    map[string]any `json:"voice_snapshot"`
	RequestSnapshot  map[string]any `json:"request_snapshot"`
	AudioArtifactID  string         `json:"audio_artifact_id,omitempty"`
	TimingArtifactID string         `json:"timing_artifact_id,omitempty"`
	DurationSec      float64        `json:"duration_sec,omitempty"`
	JobID            string         `json:"job_id"`
	CreatedAt        time.Time      `json:"created_at"`
}
type Timeline struct {
	ID               string    `json:"id"`
	ProjectID        string    `json:"project_id"`
	Status           string    `json:"status"`
	CurrentVersionID string    `json:"current_version_id,omitempty"`
	Revision         int64     `json:"revision"`
	Versions         []string  `json:"versions"`
	CreatedAt        time.Time `json:"created_at"`
}
type TimelineVersion struct {
	ID               string          `json:"id"`
	TimelineID       string          `json:"timeline_id"`
	ProjectID        string          `json:"project_id"`
	Version          int             `json:"version"`
	Status           string          `json:"status"`
	SchemaVersion    string          `json:"schema_version"`
	Document         json.RawMessage `json:"document"`
	ContentHash      string          `json:"content_hash"`
	Origin           string          `json:"origin"`
	BasedOnVersionID string          `json:"based_on_version_id,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}
type Analysis struct {
	ID         string         `json:"id"`
	ProjectID  string         `json:"project_id"`
	Kind       string         `json:"kind"`
	Status     string         `json:"status"`
	InputRefs  map[string]any `json:"input_refs"`
	Result     map[string]any `json:"result,omitempty"`
	Provenance map[string]any `json:"provenance"`
	CreatedAt  time.Time      `json:"created_at"`
}
type Scene struct {
	ID          string  `json:"id"`
	ProjectID   string  `json:"project_id"`
	AssetID     string  `json:"asset_id"`
	ArtifactID  string  `json:"artifact_id"`
	Status      string  `json:"status"`
	StartSec    float64 `json:"source_start_sec"`
	EndSec      float64 `json:"source_end_sec"`
	Description string  `json:"description,omitempty"`
	Revision    int64   `json:"revision"`
}
type Candidate struct {
	ID           string         `json:"id"`
	ProjectID    string         `json:"project_id"`
	GroupID      string         `json:"group_id"`
	JobStepID    string         `json:"job_step_id,omitempty"`
	Status       string         `json:"status"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	Payload      map[string]any `json:"payload"`
	Provenance   map[string]any `json:"provenance"`
	Selected     bool           `json:"selected"`
}
type CandidateGroup struct {
	ID         string      `json:"id"`
	ProjectID  string      `json:"project_id"`
	Candidates []Candidate `json:"candidates"`
	SelectedID string      `json:"selected_id,omitempty"`
}
type ProviderConfiguration struct {
	ID              string         `json:"id"`
	WorkspaceID     string         `json:"workspace_id"`
	Kind            string         `json:"kind"`
	AdapterKey      string         `json:"adapter_key"`
	DisplayName     string         `json:"display_name"`
	Status          string         `json:"status"`
	CredentialRef   string         `json:"credential_ref,omitempty"`
	ModelDefaults   map[string]any `json:"model_defaults"`
	Config          map[string]any `json:"config"`
	Revision        int64          `json:"revision"`
	LastValidatedAt *time.Time     `json:"last_validated_at,omitempty"`
}
type Render struct {
	ID                 string         `json:"id"`
	ProjectID          string         `json:"project_id"`
	TimelineVersionID  string         `json:"timeline_version_id"`
	ProfileKey         string         `json:"profile_key"`
	ProfileVersion     int            `json:"profile_version"`
	Status             string         `json:"status"`
	JobID              string         `json:"job_id"`
	SupersedesRenderID string         `json:"supersedes_render_id,omitempty"`
	RequestHash        string         `json:"request_hash"`
	ProfileID          string         `json:"profile_id,omitempty"`
	ProfileSnapshot    map[string]any `json:"profile_snapshot,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
}
type RenderProfile struct {
	ID            string         `json:"id"`
	WorkspaceID   string         `json:"workspace_id"`
	ProfileKey    string         `json:"profile_key"`
	Version       int            `json:"version"`
	Status        string         `json:"status"`
	SchemaVersion string         `json:"schema_version"`
	Document      map[string]any `json:"document"`
	ContentHash   string         `json:"content_hash"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}
type Job struct {
	ID            string           `json:"id"`
	ProjectID     string           `json:"project_id"`
	Kind          string           `json:"kind"`
	Status        domain.JobStatus `json:"status"`
	PipelineRunID string           `json:"pipeline_run_id,omitempty"`
	Command       map[string]any   `json:"command"`
	CreatedAt     time.Time        `json:"created_at"`
}

type PipelineRun struct {
	ID           string    `json:"id"`
	JobID        string    `json:"job_id"`
	RunNumber    int       `json:"run_number"`
	Status       string    `json:"status"`
	Contract     string    `json:"contract_version"`
	CheckpointID string    `json:"checkpoint_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type JobStep struct {
	ID            string           `json:"id"`
	PipelineRunID string           `json:"pipeline_run_id"`
	NodeKey       string           `json:"node_key"`
	Status        string           `json:"status"`
	Attempt       int              `json:"current_attempt"`
	OutputRefs    []map[string]any `json:"output_refs,omitempty"`
	SkipReason    string           `json:"skip_reason,omitempty"`
	Revision      int64            `json:"revision"`
}

type Review struct {
	ID                       string `json:"id"`
	JobID                    string `json:"job_id"`
	JobStepID                string `json:"job_step_id"`
	PipelineRunID            string `json:"pipeline_run_id"`
	Status                   string `json:"status"`
	ReviewType               string `json:"review_type"`
	ProposedResourceType     string `json:"proposed_resource_type"`
	ProposedResourceID       string `json:"proposed_resource_id"`
	ProposedResourceRevision int64  `json:"proposed_resource_revision,omitempty"`
	SelectedResourceType     string `json:"selected_resource_type,omitempty"`
	SelectedResourceID       string `json:"selected_resource_id,omitempty"`
	SelectedResourceRevision int64  `json:"selected_resource_revision,omitempty"`
	Decision                 string `json:"decision,omitempty"`
	DecisionComment          string `json:"decision_comment,omitempty"`
}

type JobCreateInput struct {
	Kind        string         `json:"kind"`
	WorkflowKey string         `json:"workflow_key,omitempty"`
	Mode        string         `json:"mode,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
	StartFrom   string         `json:"start_from,omitempty"`
	StopAfter   string         `json:"stop_after,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
	Priority    int            `json:"priority,omitempty"`
	MaxRuns     int            `json:"max_runs,omitempty"`
	EnableDLQ   bool           `json:"enable_dlq,omitempty"`
	AutoStart   bool           `json:"auto_start,omitempty"`
}

type ExecutionSurface interface {
	CreateExecutionJob(string, string, JobCreateInput) (*Job, error)
	ListJobs(string, string, string, int) ([]Job, error)
	RetryExecutionJob(string, string, string, bool) (*Job, error)
	ListPipelineRuns(string, string, string) ([]PipelineRun, error)
	GetPipelineRun(string, string, string, string) (*PipelineRun, error)
	ListJobSteps(string, string, string, string) ([]JobStep, error)
	ApproveReview(string, string, string, string, string, int64) (*Job, error)
	RejectReview(string, string, string, string, string) (*Job, error)
}

type RenderSurface interface {
	GetRender(string, string, string) (*Render, error)
	ListRenders(string, string, string, string, int) ([]Render, error)
	CancelRender(string, string, string) (*Render, error)
	RetryRender(string, string, string, bool) (*Render, error)
}
type JobEvent struct {
	ID         string         `json:"event_id"`
	EventType  string         `json:"event_type"`
	Sequence   int64          `json:"sequence"`
	Payload    map[string]any `json:"payload"`
	OccurredAt time.Time      `json:"occurred_at"`
}
type IdempotencyRecord struct {
	RequestHash string
	Status      int
	Body        []byte
}

func NewStore(workspaceID string) *Store {
	return newStore(workspaceID, nil)
}

func NewStoreWithStorage(workspaceID string, backend storage.StoragePort) *Store {
	return newStore(workspaceID, backend)
}

func newStore(workspaceID string, backend storage.StoragePort) *Store {
	if workspaceID == "" {
		workspaceID = "ws_default"
	}
	return &Store{workspaceID: workspaceID, projects: map[string]*Project{}, assets: map[string]*Asset{}, uploads: map[string]*UploadSession{}, scripts: map[string]*Script{}, scriptVers: map[string]*ScriptVersion{}, narrations: map[string]*Narration{}, timelines: map[string]*Timeline{}, timelineVers: map[string]*TimelineVersion{}, analyses: map[string]*Analysis{}, scenes: map[string]*Scene{}, candidates: map[string]*CandidateGroup{}, providers: map[string]*ProviderConfiguration{}, profiles: map[string]*RenderProfile{}, renders: map[string]*Render{}, jobs: map[string]*Job{}, jobEvents: map[string][]JobEvent{}, runs: map[string]*PipelineRun{}, steps: map[string][]JobStep{}, reviews: map[string]*Review{}, idempotency: map[string]IdempotencyRecord{}, storage: backend, storageSlots: map[string]storage.UploadSession{}, staged: map[string]storage.StagedObject{}}
}

func (s *Store) WorkspaceID() string { return s.workspaceID }
func newID(prefix string) string {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return prefix + "_fallback"
	}
	return prefix + "_" + hex.EncodeToString(raw)
}
func now() time.Time { return time.Now().UTC() }
func copyMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	encoded, _ := json.Marshal(value)
	var result map[string]any
	_ = json.Unmarshal(encoded, &result)
	return result
}
func requestHash(value any) string {
	encoded, _ := json.Marshal(value)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func (s *Store) CheckWorkspace(workspaceID string) error {
	if workspaceID == "" || workspaceID != s.workspaceID {
		return ErrNotFound
	}
	return nil
}
func (s *Store) ClaimIdempotency(workspaceID, key string, request any, status int, body []byte) ([]byte, bool, error) {
	if key == "" {
		return nil, false, nil
	}
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, false, err
	}
	hash := requestHash(request)
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.idempotency[key]; ok {
		if existing.RequestHash != hash {
			return nil, false, ErrConflict
		}
		return append([]byte(nil), existing.Body...), true, nil
	}
	s.idempotency[key] = IdempotencyRecord{RequestHash: hash, Status: status, Body: append([]byte(nil), body...)}
	return nil, false, nil
}

func (s *Store) CreateProject(workspaceID, name, workflowKey string, settings map[string]any) (*Project, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" || strings.TrimSpace(workflowKey) == "" {
		return nil, errors.New("name and workflow_key are required")
	}
	timestamp := now()
	value := &Project{ID: newID("proj"), WorkspaceID: workspaceID, Name: name, WorkflowKey: workflowKey, Status: "active", Settings: copyMap(settings), Revision: 1, CreatedAt: timestamp, UpdatedAt: timestamp}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projects[value.ID] = value
	return cloneProject(value), nil
}
func (s *Store) ListProjects(workspaceID string, limit int) ([]Project, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Project, 0)
	for _, value := range s.projects {
		if value.WorkspaceID == workspaceID && value.Status != "deleted" {
			result = append(result, *cloneProject(value))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	if limit > 100 {
		limit = 100
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}
func (s *Store) GetProject(workspaceID, id string) (*Project, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.projects[id]
	if !ok || value.WorkspaceID != workspaceID || value.Status == "deleted" {
		return nil, ErrNotFound
	}
	return cloneProject(value), nil
}
func (s *Store) UpdateProject(workspaceID, id string, revision int64, name, status *string, settings map[string]any) (*Project, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.projects[id]
	if !ok || value.WorkspaceID != workspaceID || value.Status == "deleted" {
		return nil, ErrNotFound
	}
	if revision != value.Revision {
		return nil, ErrConflict
	}
	if name != nil && strings.TrimSpace(*name) != "" {
		value.Name = *name
	}
	if status != nil {
		if *status != "active" && *status != "archived" && *status != "deleted" {
			return nil, errors.New("invalid project status")
		}
		value.Status = *status
	}
	if settings != nil {
		value.Settings = copyMap(settings)
	}
	value.Revision++
	value.UpdatedAt = now()
	return cloneProject(value), nil
}

func (s *Store) CreateAsset(workspaceID, projectID, kind, filename, contentType string, size int64, checksum string) (*Asset, error) {
	project, err := s.GetProject(workspaceID, projectID)
	if err != nil {
		return nil, err
	}
	if kind == "" {
		return nil, errors.New("asset kind is required")
	}
	timestamp := now()
	value := &Asset{ID: newID("asset"), ProjectID: project.ID, Kind: kind, Status: "uploading", OriginalFilename: filename, DeclaredMIME: contentType, SizeBytes: size, SHA256: checksum, Revision: 1, Metadata: map[string]any{}, CreatedAt: timestamp, UpdatedAt: timestamp}
	s.mu.Lock()
	s.assets[value.ID] = value
	s.mu.Unlock()
	return cloneAsset(value), nil
}
func (s *Store) GetAsset(workspaceID, projectID, id string) (*Asset, error) {
	project, err := s.GetProject(workspaceID, projectID)
	if err != nil {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.assets[id]
	if !ok || value.ProjectID != project.ID || value.Status == "deleted" {
		return nil, ErrNotFound
	}
	return cloneAsset(value), nil
}
func (s *Store) ListAssets(workspaceID, projectID string, limit int) ([]Asset, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Asset, 0)
	for _, value := range s.assets {
		if value.ProjectID == projectID && value.Status != "deleted" {
			result = append(result, *cloneAsset(value))
		}
	}
	if limit > 100 {
		limit = 100
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}
func (s *Store) UpdateAsset(workspaceID, projectID, id string, revision int64, metadata map[string]any) (*Asset, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.assets[id]
	if !ok || value.ProjectID != projectID || value.Status == "deleted" {
		return nil, ErrNotFound
	}
	if value.Revision != revision {
		return nil, ErrConflict
	}
	if metadata != nil {
		value.Metadata = copyMap(metadata)
	}
	value.Revision++
	value.UpdatedAt = now()
	return cloneAsset(value), nil
}
func (s *Store) SetAssetStatus(workspaceID, projectID, id, status, artifactID string) (*Asset, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.assets[id]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	if status == "ready" && strings.TrimSpace(artifactID) == "" {
		return nil, ErrConflict
	}
	value.Status = status
	value.OriginalArtifactID = artifactID
	value.Revision++
	value.UpdatedAt = now()
	return cloneAsset(value), nil
}
func (s *Store) CreateUpload(ctx context.Context, workspaceID, projectID, assetID string, expectedSize int64, checksum, contentType string, parts []UploadPart, hash string) (*UploadSession, error) {
	if _, err := s.GetAsset(workspaceID, projectID, assetID); err != nil {
		return nil, err
	}
	session := &UploadSession{ID: newID("upl"), AssetID: assetID, Status: "active", Method: "multipart", PartSize: 8 << 20, ExpiresAt: now().Add(2 * time.Hour), Parts: parts, ExpectedSize: expectedSize, ExpectedSHA256: checksum, RequestHash: hash}
	if s.storage != nil {
		providerSession, err := s.storage.InitiateUpload(ctx, storage.Scope{WorkspaceID: workspaceID, ProjectID: projectID}, storage.UploadConstraints{ExpectedSize: expectedSize, ContentType: contentType, PartSize: session.PartSize, Multipart: true, ExpiresAt: session.ExpiresAt})
		if err != nil {
			return nil, fmt.Errorf("initiate storage upload: %w", err)
		}
		session.PartSize = providerSession.PartSize
		session.ExpiresAt = providerSession.ExpiresAt
		requested := parts
		if len(requested) == 0 {
			requested = []UploadPart{{PartNumber: 1}}
		}
		generated := make([]UploadPart, 0, len(requested))
		for _, part := range requested {
			signed, signErr := s.storage.PresignPart(ctx, providerSession, part.PartNumber)
			if signErr != nil {
				_ = s.storage.AbortUpload(ctx, providerSession)
				return nil, fmt.Errorf("presign storage upload: %w", signErr)
			}
			generated = append(generated, UploadPart{PartNumber: part.PartNumber, Method: signed.Method, URL: signed.URL, Headers: signed.Headers})
		}
		session.Parts = generated
		s.storageMu.Lock()
		s.storageSlots[session.ID] = providerSession
		s.storageMu.Unlock()
	}
	s.mu.Lock()
	s.uploads[session.ID] = session
	s.mu.Unlock()
	return cloneUpload(session), nil
}
func (s *Store) GetUpload(workspaceID, projectID, assetID, uploadID string) (*UploadSession, error) {
	if _, err := s.GetAsset(workspaceID, projectID, assetID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.uploads[uploadID]
	if !ok || value.AssetID != assetID {
		return nil, ErrNotFound
	}
	return cloneUpload(value), nil
}
func (s *Store) PresignUploadParts(ctx context.Context, workspaceID, projectID, assetID, uploadID string, partNumbers []int) (*UploadSession, error) {
	if _, err := s.GetAsset(workspaceID, projectID, assetID); err != nil {
		return nil, err
	}
	if len(partNumbers) == 0 {
		return nil, errors.New("at least one part number is required")
	}
	upload, err := s.GetUpload(workspaceID, projectID, assetID, uploadID)
	if err != nil {
		return nil, err
	}
	if s.storage == nil {
		for _, partNumber := range partNumbers {
			if partNumber < 1 {
				return nil, errors.New("part number must be positive")
			}
			upload.Parts = append(upload.Parts, UploadPart{PartNumber: partNumber, Method: "PUT", URL: "nh-media-direct://" + assetID + "/" + strconv.Itoa(partNumber), Headers: map[string]string{}})
		}
		return upload, nil
	}
	s.storageMu.Lock()
	providerSession, ok := s.storageSlots[uploadID]
	s.storageMu.Unlock()
	if !ok {
		return nil, ErrNotFound
	}
	upload.Parts = nil
	for _, partNumber := range partNumbers {
		if partNumber < 1 {
			return nil, errors.New("part number must be positive")
		}
		signed, signErr := s.storage.PresignPart(ctx, providerSession, partNumber)
		if signErr != nil {
			return nil, fmt.Errorf("presign storage upload: %w", signErr)
		}
		upload.Parts = append(upload.Parts, UploadPart{PartNumber: partNumber, Method: signed.Method, URL: signed.URL, Headers: signed.Headers})
	}
	return upload, nil
}

func (s *Store) CompleteUpload(ctx context.Context, workspaceID, projectID, assetID, uploadID, checksum string, parts []UploadPart) (*UploadSession, *Job, error) {
	if _, err := s.GetAsset(workspaceID, projectID, assetID); err != nil {
		return nil, nil, err
	}
	s.mu.Lock()
	upload, ok := s.uploads[uploadID]
	if !ok || upload.AssetID != assetID {
		s.mu.Unlock()
		return nil, nil, ErrNotFound
	}
	if upload.Status == "completed" {
		s.mu.Unlock()
		return cloneUpload(upload), nil, nil
	}
	if now().After(upload.ExpiresAt) {
		upload.Status = "expired"
		s.mu.Unlock()
		return nil, nil, ErrConflict
	}
	if upload.ExpectedSHA256 != "" && checksum != upload.ExpectedSHA256 {
		s.mu.Unlock()
		return nil, nil, errors.New("completed upload checksum does not match the declared checksum")
	}
	seenParts := map[int]struct{}{}
	for _, part := range parts {
		if part.PartNumber < 1 || part.ETag == "" {
			s.mu.Unlock()
			return nil, nil, errors.New("completed upload parts require positive numbers and etags")
		}
		if _, exists := seenParts[part.PartNumber]; exists {
			s.mu.Unlock()
			return nil, nil, errors.New("completed upload contains duplicate parts")
		}
		seenParts[part.PartNumber] = struct{}{}
	}
	s.mu.Unlock()
	var stagedObject storage.StagedObject
	if s.storage != nil {
		s.storageMu.Lock()
		providerSession, providerOK := s.storageSlots[uploadID]
		s.storageMu.Unlock()
		if !providerOK {
			return nil, nil, ErrNotFound
		}
		providerParts := make([]storage.UploadedPart, 0, len(parts))
		for _, part := range parts {
			providerParts = append(providerParts, storage.UploadedPart{PartNumber: part.PartNumber, ETag: part.ETag})
		}
		var storageErr error
		stagedObject, storageErr = s.storage.CompleteUpload(ctx, providerSession, providerParts)
		if storageErr != nil {
			return nil, nil, fmt.Errorf("complete storage upload: %w", storageErr)
		}
	}
	s.mu.Lock()
	upload, ok = s.uploads[uploadID]
	if !ok || upload.AssetID != assetID || upload.Status != "active" {
		s.mu.Unlock()
		return nil, nil, ErrConflict
	}
	upload.Parts = append([]UploadPart(nil), parts...)
	upload.Status = "completed"
	job := &Job{ID: newID("job"), ProjectID: projectID, Kind: "asset_probe", Status: domain.JobCreated, Command: map[string]any{"asset_id": assetID, "purpose": "validation"}, CreatedAt: now()}
	s.jobs[job.ID] = job
	s.mu.Unlock()
	if s.storage != nil {
		s.storageMu.Lock()
		delete(s.storageSlots, uploadID)
		s.staged[uploadID] = stagedObject
		s.storageMu.Unlock()
	}
	_, _ = s.SetAssetStatus(workspaceID, projectID, assetID, "validating", "")
	return cloneUpload(upload), cloneJob(job), nil
}
func (s *Store) AbortUpload(ctx context.Context, workspaceID, projectID, assetID, uploadID string) error {
	if _, err := s.GetAsset(workspaceID, projectID, assetID); err != nil {
		return err
	}
	s.mu.Lock()
	value, ok := s.uploads[uploadID]
	if !ok || value.AssetID != assetID {
		s.mu.Unlock()
		return ErrNotFound
	}
	s.mu.Unlock()
	if s.storage != nil {
		s.storageMu.Lock()
		providerSession, providerOK := s.storageSlots[uploadID]
		s.storageMu.Unlock()
		if providerOK {
			if err := s.storage.AbortUpload(ctx, providerSession); err != nil {
				return fmt.Errorf("abort storage upload: %w", err)
			}
			s.storageMu.Lock()
			delete(s.storageSlots, uploadID)
			s.storageMu.Unlock()
		}
	}
	s.mu.Lock()
	value, ok = s.uploads[uploadID]
	if !ok || value.AssetID != assetID {
		s.mu.Unlock()
		return ErrNotFound
	}
	value.Status = "aborted"
	s.mu.Unlock()
	return nil
}

func (s *Store) GetJob(workspaceID, projectID, jobID string) (*Job, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.jobs[jobID]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	return cloneJob(value), nil
}

func (s *Store) StartJob(workspaceID, projectID, jobID string) (*Job, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.jobs[jobID]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	if value.Status == domain.JobQueued {
		return cloneJob(value), nil
	}
	if err := execution.ValidateTransition(execution.EntityJob, string(value.Status), string(domain.JobQueued)); err != nil {
		return nil, ErrConflict
	}
	from := value.Status
	value.Status = domain.JobQueued
	s.jobEvents[jobID] = append(s.jobEvents[jobID], JobEvent{ID: newID("evt"), EventType: execution.EventType(execution.EntityJob, string(from), string(value.Status)), Sequence: int64(len(s.jobEvents[jobID]) + 1), Payload: map[string]any{"from_status": from, "to_status": value.Status}, OccurredAt: now()})
	if run := s.runs[value.PipelineRunID]; run != nil {
		if run.Status == "created" {
			run.Status = "queued"
			s.appendJobEventLocked(jobID, "pipeline_run.queued", map[string]any{"from_status": "created", "to_status": "queued"})
		}
		for index := range s.steps[run.ID] {
			if s.steps[run.ID][index].Status == "pending" || s.steps[run.ID][index].Status == "ready" {
				s.steps[run.ID][index].Status = "queued"
				s.steps[run.ID][index].Revision++
				s.appendJobEventLocked(jobID, "node.queued", map[string]any{"node_key": s.steps[run.ID][index].NodeKey})
			}
		}
	}
	return cloneJob(value), nil
}

func (s *Store) ListJobEvents(workspaceID, projectID, jobID string, after int64, limit int) ([]JobEvent, error) {
	if _, err := s.GetJob(workspaceID, projectID, jobID); err != nil {
		return nil, err
	}
	if after < 0 {
		after = 0
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]JobEvent, 0, limit)
	for _, value := range s.jobEvents[jobID] {
		if value.Sequence <= after {
			continue
		}
		copyValue := value
		copyValue.Payload = copyMap(value.Payload)
		result = append(result, copyValue)
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

func (s *Store) CreateScript(workspaceID, projectID, language, origin string, content map[string]any) (*Script, *ScriptVersion, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, nil, err
	}
	timestamp := now()
	script := &Script{ID: newID("script"), ProjectID: projectID, Status: "active", Revision: 1, Versions: []string{}, CreatedAt: timestamp}
	version := &ScriptVersion{ID: newID("scriptv"), ScriptID: script.ID, ProjectID: projectID, Version: 1, Status: "draft", Language: language, Origin: origin, Content: copyMap(content), CreatedAt: timestamp}
	version.ContentHash = requestHash(version.Content)
	script.CurrentVersionID = version.ID
	script.Versions = append(script.Versions, version.ID)
	s.mu.Lock()
	s.scripts[script.ID] = script
	s.scriptVers[version.ID] = version
	s.mu.Unlock()
	return cloneScript(script), cloneScriptVersion(version), nil
}
func (s *Store) GetScript(workspaceID, projectID, id string) (*Script, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.scripts[id]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	return cloneScript(value), nil
}
func (s *Store) GetScriptVersion(workspaceID, projectID, id string) (*ScriptVersion, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.scriptVers[id]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	return cloneScriptVersion(value), nil
}
func (s *Store) CreateScriptVersion(workspaceID, projectID, scriptID, basedOn, language, origin string, content map[string]any, expectedRevision int64) (*ScriptVersion, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	script, ok := s.scripts[scriptID]
	if !ok || script.ProjectID != projectID {
		return nil, ErrNotFound
	}
	if expectedRevision != script.Revision {
		return nil, ErrConflict
	}
	version := &ScriptVersion{ID: newID("scriptv"), ScriptID: scriptID, ProjectID: projectID, Version: len(script.Versions) + 1, Status: "draft", Language: language, Origin: origin, Content: copyMap(content), ContentHash: requestHash(content), BasedOnVersionID: basedOn, CreatedAt: now()}
	script.Versions = append(script.Versions, version.ID)
	script.Revision++
	s.scriptVers[version.ID] = version
	return cloneScriptVersion(version), nil
}

func (s *Store) SetScriptVersionStatus(workspaceID, projectID, scriptID, versionID, status string) (*ScriptVersion, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	script, ok := s.scripts[scriptID]
	if !ok || script.ProjectID != projectID {
		return nil, ErrNotFound
	}
	version, ok := s.scriptVers[versionID]
	if !ok || version.ScriptID != scriptID {
		return nil, ErrNotFound
	}
	if status != "approved" && status != "superseded" && status != "draft" {
		return nil, errors.New("invalid script version status")
	}
	version.Status = status
	if status == "approved" {
		script.CurrentVersionID = version.ID
		script.Revision++
	}
	return cloneScriptVersion(version), nil
}

func (s *Store) CreateNarration(workspaceID, projectID, scriptVersionID, providerConfigID string, voiceSnapshot, requestSnapshot map[string]any) (*Narration, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	s.mu.Lock()
	version, ok := s.scriptVers[scriptVersionID]
	if !ok || version.ProjectID != projectID || version.Status != "approved" {
		s.mu.Unlock()
		return nil, ErrConflict
	}
	value := &Narration{ID: newID("narration"), ProjectID: projectID, ScriptVersionID: scriptVersionID, Status: "created", ProviderConfigID: providerConfigID, VoiceSnapshot: copyMap(voiceSnapshot), RequestSnapshot: copyMap(requestSnapshot), JobID: newID("job"), CreatedAt: now()}
	s.narrations[value.ID] = value
	s.jobs[value.JobID] = &Job{ID: value.JobID, ProjectID: projectID, Kind: "pipeline", Status: domain.JobCreated, Command: map[string]any{"narration_id": value.ID, "script_version_id": scriptVersionID}, CreatedAt: value.CreatedAt}
	s.mu.Unlock()
	return cloneNarration(value), nil
}

func (s *Store) GetNarration(workspaceID, projectID, id string) (*Narration, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.narrations[id]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	return cloneNarration(value), nil
}

func ValidateRenderProfileTransition(current, next string) error {
	allowed := map[string]string{"draft": "active", "active": "deprecated", "deprecated": "disabled"}
	if allowed[current] != next {
		return fmt.Errorf("invalid render profile lifecycle transition %s -> %s", current, next)
	}
	return nil
}

func renderProfileMapKey(profileKey string, version int) string {
	return profileKey + "\x00" + strconv.Itoa(version)
}

func (s *Store) CreateRenderProfile(workspaceID, profileKey string, version int, document map[string]any) (*RenderProfile, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(profileKey) == "" || version < 1 || len(document) == 0 {
		return nil, errors.New("render profile key, positive version and document are required")
	}
	key := renderProfileMapKey(profileKey, version)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.profiles[key]; exists {
		return nil, ErrConflict
	}
	content := copyMap(document)
	hash := requestHash(content)
	for _, existing := range s.profiles {
		if existing.ProfileKey == profileKey && existing.ContentHash == hash {
			return nil, ErrConflict
		}
	}
	timestamp := now()
	value := &RenderProfile{ID: newID("render_profile"), WorkspaceID: workspaceID, ProfileKey: profileKey, Version: version, Status: "draft", SchemaVersion: "1.0", Document: content, ContentHash: hash, CreatedAt: timestamp, UpdatedAt: timestamp}
	s.profiles[key] = value
	return cloneRenderProfile(value), nil
}

func (s *Store) GetRenderProfile(workspaceID, profileKey string, version int) (*RenderProfile, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.profiles[renderProfileMapKey(profileKey, version)]
	if !ok || value.WorkspaceID != workspaceID {
		return nil, ErrNotFound
	}
	return cloneRenderProfile(value), nil
}

func (s *Store) UpdateRenderProfile(workspaceID, profileKey string, version int, document map[string]any) (*RenderProfile, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, ErrNotFound
	}
	if len(document) == 0 {
		return nil, errors.New("render profile document is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.profiles[renderProfileMapKey(profileKey, version)]
	if !ok || value.WorkspaceID != workspaceID {
		return nil, ErrNotFound
	}
	if value.Status != "draft" {
		return nil, ErrConflict
	}
	updated := copyMap(document)
	hash := requestHash(updated)
	for key, existing := range s.profiles {
		if key != renderProfileMapKey(profileKey, version) && existing.ProfileKey == profileKey && existing.ContentHash == hash {
			return nil, ErrConflict
		}
	}
	value.Document = updated
	value.ContentHash = hash
	value.UpdatedAt = now()
	return cloneRenderProfile(value), nil
}

func (s *Store) SetRenderProfileStatus(workspaceID, profileKey string, version int, status string) (*RenderProfile, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.profiles[renderProfileMapKey(profileKey, version)]
	if !ok || value.WorkspaceID != workspaceID {
		return nil, ErrNotFound
	}
	if err := ValidateRenderProfileTransition(value.Status, status); err != nil {
		return nil, ErrConflict
	}
	value.Status = status
	value.UpdatedAt = now()
	return cloneRenderProfile(value), nil
}

func (s *Store) CreateTimeline(workspaceID, projectID, origin string, document json.RawMessage) (*Timeline, *TimelineVersion, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, nil, err
	}
	hash, err := domain.ValidateTimeline(document, domain.TimelineValidationOptions{Resolver: s})
	if err != nil {
		return nil, nil, err
	}
	if documentProject, err := timelineDocumentProject(document); err != nil || documentProject != projectID {
		return nil, nil, errors.New("timeline document belongs to another project")
	}
	timestamp := now()
	timeline := &Timeline{ID: newID("timeline"), ProjectID: projectID, Status: "active", Revision: 1, CreatedAt: timestamp, Versions: []string{}}
	version := &TimelineVersion{ID: newID("tlv"), TimelineID: timeline.ID, ProjectID: projectID, Version: 1, Status: "draft", SchemaVersion: domain.TimelineSchemaVersion, Document: append(json.RawMessage(nil), document...), ContentHash: hash, Origin: origin, CreatedAt: timestamp}
	timeline.CurrentVersionID = version.ID
	timeline.Versions = append(timeline.Versions, version.ID)
	s.mu.Lock()
	s.timelines[timeline.ID] = timeline
	s.timelineVers[version.ID] = version
	s.mu.Unlock()
	return cloneTimeline(timeline), cloneTimelineVersion(version), nil
}
func (s *Store) GetTimeline(workspaceID, projectID, id string) (*Timeline, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.timelines[id]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	return cloneTimeline(value), nil
}
func (s *Store) GetTimelineVersion(workspaceID, projectID, id string) (*TimelineVersion, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.timelineVers[id]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	return cloneTimelineVersion(value), nil
}
func (s *Store) AddTimelineVersion(workspaceID, projectID, timelineID, basedOn, origin string, document json.RawMessage, expectedRevision int64) (*TimelineVersion, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	hash, err := domain.ValidateTimeline(document, domain.TimelineValidationOptions{Resolver: s})
	if err != nil {
		return nil, err
	}
	if documentProject, err := timelineDocumentProject(document); err != nil || documentProject != projectID {
		return nil, errors.New("timeline document belongs to another project")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	timeline, ok := s.timelines[timelineID]
	if !ok || timeline.ProjectID != projectID {
		return nil, ErrNotFound
	}
	if timeline.Revision != expectedRevision {
		return nil, ErrConflict
	}
	if basedOn != "" {
		base, ok := s.timelineVers[basedOn]
		if !ok || base.TimelineID != timelineID {
			return nil, ErrConflict
		}
	}
	version := &TimelineVersion{ID: newID("tlv"), TimelineID: timelineID, ProjectID: projectID, Version: len(timeline.Versions) + 1, Status: "draft", SchemaVersion: domain.TimelineSchemaVersion, Document: append(json.RawMessage(nil), document...), ContentHash: hash, Origin: origin, BasedOnVersionID: basedOn, CreatedAt: now()}
	timeline.Versions = append(timeline.Versions, version.ID)
	timeline.CurrentVersionID = version.ID
	timeline.Revision++
	s.timelineVers[version.ID] = version
	return cloneTimelineVersion(version), nil
}

func (s *Store) SetTimelineVersionStatus(workspaceID, projectID, timelineID, versionID, status string) (*TimelineVersion, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	if status == "approved" || status == "locked" {
		if _, err := s.ValidateTimeline(workspaceID, projectID, versionID); err != nil {
			return nil, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	timeline, ok := s.timelines[timelineID]
	if !ok || timeline.ProjectID != projectID {
		return nil, ErrNotFound
	}
	version, ok := s.timelineVers[versionID]
	if !ok || version.TimelineID != timelineID {
		return nil, ErrNotFound
	}
	if status != "approved" && status != "locked" && status != "superseded" && status != "draft" {
		return nil, errors.New("invalid timeline version status")
	}
	version.Status = status
	if status == "approved" || status == "locked" {
		timeline.CurrentVersionID = version.ID
		timeline.Revision++
	}
	return cloneTimelineVersion(version), nil
}

func (s *Store) ValidateTimeline(workspaceID, projectID, versionID string) (string, error) {
	value, err := s.GetTimelineVersion(workspaceID, projectID, versionID)
	if err != nil {
		return "", err
	}
	return domain.ValidateTimeline(value.Document, domain.TimelineValidationOptions{Resolver: s})
}

func (s *Store) ResolveAssetArtifact(assetID, artifactID, projectID string) (bool, float64, int, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.assets[assetID]
	if !ok || value.ProjectID != projectID || value.Status != "ready" || value.OriginalArtifactID == "" || value.OriginalArtifactID != artifactID {
		return false, 0, 0, 0, nil
	}
	return true, metadataFloat(value.Metadata, "duration_sec"), metadataInt(value.Metadata, "width"), metadataInt(value.Metadata, "height"), nil
}

func (s *Store) ResolveScene(sceneID, assetID, artifactID, projectID string) (bool, float64, float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	scene, ok := s.scenes[sceneID]
	asset, assetOK := s.assets[assetID]
	if !ok || !assetOK || scene.ProjectID != projectID || asset.ProjectID != projectID || scene.AssetID != assetID || scene.ArtifactID != artifactID || asset.Status != "ready" || asset.OriginalArtifactID != artifactID || (scene.Status != "detected" && scene.Status != "analyzed" && scene.Status != "confirmed") {
		return false, 0, 0, nil
	}
	return true, scene.StartSec, scene.EndSec, nil
}

func (s *Store) ResolveArtifact(artifactID, projectID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, asset := range s.assets {
		if asset.ProjectID == projectID && asset.Status == "ready" && asset.OriginalArtifactID == artifactID {
			return true, nil
		}
	}
	for _, scene := range s.scenes {
		asset, assetOK := s.assets[scene.AssetID]
		if scene.ProjectID == projectID && assetOK && asset.Status == "ready" && asset.OriginalArtifactID == artifactID && scene.ArtifactID == artifactID && (scene.Status == "detected" || scene.Status == "analyzed" || scene.Status == "confirmed") {
			return true, nil
		}
	}
	for _, narration := range s.narrations {
		if narration.ProjectID == projectID && narration.Status == "ready" && (narration.AudioArtifactID == artifactID || narration.TimingArtifactID == artifactID) {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) ResolveNarration(narrationID, scriptVersionID, projectID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.narrations[narrationID]
	if !ok || value.ProjectID != projectID || value.ScriptVersionID != scriptVersionID || value.Status != "ready" || value.AudioArtifactID == "" {
		return false, nil
	}
	version, ok := s.scriptVers[scriptVersionID]
	if !ok || version.ProjectID != projectID || version.Status != "approved" {
		return false, nil
	}
	for _, asset := range s.assets {
		if asset.ProjectID == projectID && asset.Status == "ready" && asset.OriginalArtifactID == value.AudioArtifactID {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) CreateRender(workspaceID, projectID string, timelineVersionID, profileKey string, profileVersion int, request map[string]any) (*Render, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	if _, err := s.ValidateTimeline(workspaceID, projectID, timelineVersionID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	version, ok := s.timelineVers[timelineVersionID]
	if !ok || version.ProjectID != projectID || (version.Status != "approved" && version.Status != "locked") {
		s.mu.RUnlock()
		return nil, ErrConflict
	}
	profile, profileOK := s.profiles[renderProfileMapKey(profileKey, profileVersion)]
	if !profileOK {
		s.mu.RUnlock()
		return nil, ErrNotFound
	}
	if profile.Status != "active" {
		s.mu.RUnlock()
		return nil, ErrConflict
	}
	profileID := profile.ID
	profileSnapshot := copyMap(profile.Document)
	s.mu.RUnlock()
	value := &Render{ID: newID("render"), ProjectID: projectID, TimelineVersionID: timelineVersionID, ProfileKey: profileKey, ProfileVersion: profileVersion, ProfileID: profileID, ProfileSnapshot: profileSnapshot, Status: "created", RequestHash: requestHash(request), CreatedAt: now()}
	job, err := s.CreateExecutionJob(workspaceID, projectID, JobCreateInput{Kind: "render", Mode: "automatic", Input: copyMap(request), AutoStart: false})
	if err != nil {
		return nil, err
	}
	value.JobID = job.ID
	s.mu.Lock()
	s.renders[value.ID] = value
	if stored := s.jobs[value.JobID]; stored != nil {
		stored.Command["render_id"] = value.ID
	}
	s.mu.Unlock()
	return cloneRender(value), nil
}

func timelineDocumentProject(document json.RawMessage) (string, error) {
	var value struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(document, &value); err != nil {
		return "", err
	}
	return value.ProjectID, nil
}

func (s *Store) CreateAnalysis(workspaceID, projectID, kind string, inputRefs, provenance map[string]any) (*Analysis, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	value := &Analysis{ID: newID("analysis"), ProjectID: projectID, Kind: kind, Status: "created", InputRefs: copyMap(inputRefs), Provenance: copyMap(provenance), CreatedAt: now()}
	if kind == "reference_style" {
		value.Provenance["output_policy"] = "abstract_metrics_only"
	}
	s.mu.Lock()
	s.analyses[value.ID] = value
	s.mu.Unlock()
	return cloneAnalysis(value), nil
}
func (s *Store) GetAnalysis(workspaceID, projectID, id string) (*Analysis, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.analyses[id]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	return cloneAnalysis(value), nil
}
func (s *Store) ListAnalyses(workspaceID, projectID string) ([]Analysis, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []Analysis{}
	for _, value := range s.analyses {
		if value.ProjectID == projectID {
			result = append(result, *cloneAnalysis(value))
		}
	}
	return result, nil
}
func (s *Store) CreateScene(workspaceID, projectID, assetID, artifactID string, start, end float64, description string) (*Scene, error) {
	if _, err := s.GetAsset(workspaceID, projectID, assetID); err != nil {
		return nil, err
	}
	if start < 0 || end <= start {
		return nil, errors.New("scene range is invalid")
	}
	value := &Scene{ID: newID("scene"), ProjectID: projectID, AssetID: assetID, ArtifactID: artifactID, Status: "detected", StartSec: start, EndSec: end, Description: description, Revision: 1}
	s.mu.Lock()
	s.scenes[value.ID] = value
	s.mu.Unlock()
	return cloneScene(value), nil
}
func (s *Store) ListScenes(workspaceID, projectID string) ([]Scene, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []Scene{}
	for _, value := range s.scenes {
		if value.ProjectID == projectID {
			result = append(result, *cloneScene(value))
		}
	}
	return result, nil
}
func (s *Store) GetScene(workspaceID, projectID, id string) (*Scene, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.scenes[id]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	return cloneScene(value), nil
}
func (s *Store) CreateCandidateGroup(workspaceID, projectID string, candidates []Candidate) (*CandidateGroup, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, err
	}
	group := &CandidateGroup{ID: newID("candgroup"), ProjectID: projectID, Candidates: append([]Candidate(nil), candidates...)}
	for index := range group.Candidates {
		if group.Candidates[index].ID == "" {
			group.Candidates[index].ID = newID("candidate")
		}
		group.Candidates[index].ProjectID = projectID
		group.Candidates[index].GroupID = group.ID
	}
	s.mu.Lock()
	s.candidates[group.ID] = group
	s.mu.Unlock()
	return cloneCandidateGroup(group), nil
}
func (s *Store) GetCandidateGroup(workspaceID, projectID, id string) (*CandidateGroup, error) {
	if _, err := s.GetProject(workspaceID, projectID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.candidates[id]
	if !ok || value.ProjectID != projectID {
		return nil, ErrNotFound
	}
	return cloneCandidateGroup(value), nil
}
func (s *Store) CreateProvider(workspaceID, kind, adapter, display string, config, models map[string]any, credentialRef string) (*ProviderConfiguration, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, err
	}
	if kind == "" || adapter == "" || display == "" {
		return nil, errors.New("provider kind, adapter_key and display_name are required")
	}
	value := &ProviderConfiguration{ID: newID("pc"), WorkspaceID: workspaceID, Kind: kind, AdapterKey: adapter, DisplayName: display, Status: "draft", CredentialRef: credentialRef, Config: copyMap(config), ModelDefaults: copyMap(models), Revision: 1}
	s.mu.Lock()
	s.providers[value.ID] = value
	s.mu.Unlock()
	return cloneProvider(value), nil
}
func (s *Store) ListProviders(workspaceID string) ([]ProviderConfiguration, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []ProviderConfiguration{}
	for _, value := range s.providers {
		result = append(result, *cloneProvider(value))
	}
	return result, nil
}
func (s *Store) GetProvider(workspaceID, id string) (*ProviderConfiguration, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.providers[id]
	if !ok || value.WorkspaceID != workspaceID {
		return nil, ErrNotFound
	}
	return cloneProvider(value), nil
}
func (s *Store) UpdateProvider(workspaceID, id string, revision int64, status, adapter, display string, config, models map[string]any, credentialRef *string) (*ProviderConfiguration, error) {
	if err := s.CheckWorkspace(workspaceID); err != nil {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.providers[id]
	if !ok || value.WorkspaceID != workspaceID {
		return nil, ErrNotFound
	}
	if value.Revision != revision {
		return nil, ErrConflict
	}
	if status != "" {
		value.Status = status
	}
	if adapter != "" {
		value.AdapterKey = adapter
	}
	if display != "" {
		value.DisplayName = display
	}
	if config != nil {
		value.Config = copyMap(config)
	}
	if models != nil {
		value.ModelDefaults = copyMap(models)
	}
	if credentialRef != nil {
		value.CredentialRef = *credentialRef
	}
	value.Revision++
	return cloneProvider(value), nil
}

func cloneProject(value *Project) *Project {
	result := *value
	result.Settings = copyMap(value.Settings)
	return &result
}
func cloneAsset(value *Asset) *Asset {
	result := *value
	result.Metadata = copyMap(value.Metadata)
	return &result
}
func cloneUpload(value *UploadSession) *UploadSession {
	result := *value
	result.Parts = append([]UploadPart(nil), value.Parts...)
	return &result
}
func cloneScript(value *Script) *Script {
	result := *value
	result.Versions = append([]string(nil), value.Versions...)
	return &result
}
func cloneScriptVersion(value *ScriptVersion) *ScriptVersion {
	result := *value
	result.Content = copyMap(value.Content)
	return &result
}
func cloneNarration(value *Narration) *Narration {
	result := *value
	result.VoiceSnapshot = copyMap(value.VoiceSnapshot)
	result.RequestSnapshot = copyMap(value.RequestSnapshot)
	return &result
}
func cloneTimeline(value *Timeline) *Timeline {
	result := *value
	result.Versions = append([]string(nil), value.Versions...)
	return &result
}
func cloneTimelineVersion(value *TimelineVersion) *TimelineVersion {
	result := *value
	result.Document = append(json.RawMessage(nil), value.Document...)
	return &result
}
func cloneAnalysis(value *Analysis) *Analysis {
	result := *value
	result.InputRefs = copyMap(value.InputRefs)
	result.Provenance = copyMap(value.Provenance)
	result.Result = copyMap(value.Result)
	return &result
}
func cloneScene(value *Scene) *Scene {
	result := *value
	return &result
}
func cloneCandidateGroup(value *CandidateGroup) *CandidateGroup {
	result := *value
	result.Candidates = make([]Candidate, len(value.Candidates))
	for index, candidate := range value.Candidates {
		result.Candidates[index] = candidate
		result.Candidates[index].Payload = copyMap(candidate.Payload)
		result.Candidates[index].Provenance = copyMap(candidate.Provenance)
	}
	return &result
}
func cloneProvider(value *ProviderConfiguration) *ProviderConfiguration {
	result := *value
	result.Config = copyMap(value.Config)
	result.ModelDefaults = copyMap(value.ModelDefaults)
	return &result
}
func cloneJob(value *Job) *Job {
	result := *value
	result.Command = copyMap(value.Command)
	return &result
}
func cloneRender(value *Render) *Render {
	result := *value
	result.ProfileSnapshot = copyMap(value.ProfileSnapshot)
	return &result
}
func cloneRenderProfile(value *RenderProfile) *RenderProfile {
	result := *value
	result.Document = copyMap(value.Document)
	return &result
}

func metadataFloat(value map[string]any, key string) float64 {
	switch current := value[key].(type) {
	case float64:
		return current
	case json.Number:
		parsed, _ := current.Float64()
		return parsed
	case int:
		return float64(current)
	case int64:
		return float64(current)
	default:
		return 0
	}
}

func metadataInt(value map[string]any, key string) int {
	return int(metadataFloat(value, key))
}
