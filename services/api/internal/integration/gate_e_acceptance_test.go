package integration

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/auth"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/config"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/health"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/httpapi"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/persistence"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/product"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/queue"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

type gateEDurableFixture struct {
	db          *sql.DB
	store       *persistence.SQLStore
	backend     *persistence.DurableBackend
	queue       *queue.RedisQueue
	objectStore storage.StoragePort
	userID      string
	workspaceID string
	server      *httptest.Server
	client      *http.Client
}

func newGateEDurableFixture(t *testing.T) *gateEDurableFixture {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_DATABASE_URL"))
	redisAddress := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_REDIS_ADDR"))
	minioEndpoint := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_MINIO_ENDPOINT"))
	accessKey := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_MINIO_ACCESS_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_MINIO_SECRET_KEY"))
	bucket := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_MINIO_BUCKET"))
	if dsn == "" || redisAddress == "" || minioEndpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		t.Skip("set PostgreSQL, Redis and MinIO test variables for Gate E durable acceptance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := persistence.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	store, err := persistence.NewSQLStore(db)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	suffix := gateESuffix()
	bootstrap, err := store.BootstrapLocalInstallation(ctx, persistence.BootstrapInput{
		Username: "gate-e-admin-" + suffix, DisplayName: "Gate E acceptance", PasswordHash: auth.CredentialHash("gate-e-password"),
		WorkspaceSlug: "gate-e-" + suffix, WorkspaceName: "Gate E acceptance " + suffix,
	})
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := store.EnsureDefaultWorkflow(ctx, bootstrap.UserID); err != nil {
		db.Close()
		t.Fatal(err)
	}
	objectStore, err := storage.NewS3Storage(minioEndpoint, accessKey, secretKey, bucket, false)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	transport, err := queue.NewRedisQueue(queue.RedisOptions{
		Addr: redisAddress, Password: os.Getenv("NH_QUEUE_PASSWORD"), StreamPrefix: "gate-e-" + suffix,
		ConsumerGroup: "gate-e-workers-" + suffix,
	})
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	backend, err := persistence.NewDurableBackend(store, bootstrap.UserID, bootstrap.WorkspaceID, objectStore)
	if err != nil {
		transport.Close()
		db.Close()
		t.Fatal(err)
	}
	backend.SetQueue(transport)
	provider, err := auth.NewLocalAuthProvider("gate-e-http-admin", "gate-e-http-password")
	if err != nil {
		transport.Close()
		db.Close()
		t.Fatal(err)
	}
	if err := provider.ConfigurePrincipal("local-admin", bootstrap.UserID, bootstrap.WorkspaceID, "owner"); err != nil {
		transport.Close()
		db.Close()
		t.Fatal(err)
	}
	provider.SetSessionStore(store)
	provider.SetPrincipalValidator(func(ctx context.Context, principal auth.Principal) (auth.Principal, error) {
		role, err := store.RequireWorkspaceRole(ctx, principal.UserID, principal.WorkspaceID)
		if err != nil {
			return auth.Principal{}, err
		}
		principal.Role = role
		return principal, nil
	})
	value, err := config.LoadAPI(func(name string) string {
		values := map[string]string{
			"NH_MEDIA_PROFILE": "local", "NH_API_ALLOWED_ORIGINS": "http://localhost:3000",
			"NH_API_MASTER_KEY": base64.RawStdEncoding.EncodeToString(make([]byte, 32)),
		}
		return values[name]
	})
	if err != nil {
		transport.Close()
		db.Close()
		t.Fatal(err)
	}
	server, err := httpapi.NewServerWithBackend(value, provider, health.NewRegistry(nil, time.Second), backend, objectStore)
	if err != nil {
		transport.Close()
		db.Close()
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.Handler())
	f := &gateEDurableFixture{db: db, store: store, backend: backend, queue: transport, objectStore: objectStore, userID: bootstrap.UserID, workspaceID: bootstrap.WorkspaceID, server: httpServer, client: httpServer.Client()}
	t.Cleanup(func() {
		httpServer.Close()
		transport.Close()
		db.Close()
	})
	return f
}

func (f *gateEDurableFixture) closeProject(t *testing.T, projectID, jobID string) {
	t.Helper()
	cleanupDurableRuntime(t, f.db, f.objectStore, projectID, jobID)
}

func (f *gateEDurableFixture) request(t *testing.T, method, path, token string, body any, headers map[string]string) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, f.server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	response, err := f.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, data
}

func (f *gateEDurableFixture) login(t *testing.T) string {
	t.Helper()
	status, body := f.request(t, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"username": "gate-e-http-admin", "password": "gate-e-http-password"}, nil)
	if status != http.StatusOK {
		t.Fatalf("login status=%d body=%s", status, body)
	}
	var value struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &value); err != nil || value.AccessToken == "" {
		t.Fatalf("login token missing: %s", body)
	}
	return value.AccessToken
}

func (f *gateEDurableFixture) createSingleNodeWorkflow(t *testing.T, suffix string, maxAttempts int, reviewPolicy string) string {
	t.Helper()
	key := "gate_e_" + suffix
	_, err := f.store.EnsurePipelineDefinition(context.Background(), persistence.PipelineDefinitionInput{
		WorkflowKey: key, WorkflowName: "Gate E " + suffix, Description: "disposable Gate E acceptance graph", SchemaVersion: "1.0",
		Definition: []byte(`{"kind":"gate_e_acceptance","nodes":["analysis"]}`), Version: 1, Status: "active", CreatedBy: f.userID,
		Nodes: []persistence.PipelineNodeInput{{
			NodeKey: "analysis", NodeType: "analysis", DisplayName: "Gate E analysis", ExecutionClass: "probe", FailureMode: "hard", ReviewPolicy: reviewPolicy,
			Config: []byte(`{"mode":"deterministic"}`), TimeoutSec: 60, MaxAttempts: maxAttempts,
			RetryPolicy: []byte(`{"base_delay_ms":1,"max_delay_ms":10}`), CheckpointPolicy: []byte(`{"enabled":true}`),
			ResourceRequirements: []byte(`{"network":"none"}`), IdempotencyPolicy: []byte(`{"key":"message_id"}`), ProgressWeight: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func (f *gateEDurableFixture) createTwoNodeWorkflow(t *testing.T, suffix string) string {
	t.Helper()
	key := "gate_e_partial_" + suffix
	_, err := f.store.EnsurePipelineDefinition(context.Background(), persistence.PipelineDefinitionInput{
		WorkflowKey: key, WorkflowName: "Gate E partial " + suffix, Description: "two-node partial execution graph", SchemaVersion: "1.0",
		Definition: []byte(`{"kind":"gate_e_partial","nodes":["analysis","finalize"]}`), Version: 1, Status: "active", CreatedBy: f.userID,
		Nodes: []persistence.PipelineNodeInput{
			{NodeKey: "analysis", NodeType: "analysis", DisplayName: "Gate E analysis", ExecutionClass: "probe", FailureMode: "hard", ReviewPolicy: "none", Config: []byte(`{"mode":"deterministic"}`), TimeoutSec: 60, MaxAttempts: 2, RetryPolicy: []byte(`{"base_delay_ms":1,"max_delay_ms":10}`), CheckpointPolicy: []byte(`{"enabled":true}`), ResourceRequirements: []byte(`{"network":"none"}`), IdempotencyPolicy: []byte(`{"key":"message_id"}`), ProgressWeight: 1},
			{NodeKey: "finalize", NodeType: "analysis", DisplayName: "Gate E finalize", ExecutionClass: "probe", FailureMode: "hard", ReviewPolicy: "none", Config: []byte(`{"mode":"deterministic"}`), TimeoutSec: 60, MaxAttempts: 2, RetryPolicy: []byte(`{"base_delay_ms":1,"max_delay_ms":10}`), CheckpointPolicy: []byte(`{"enabled":true}`), ResourceRequirements: []byte(`{"network":"none"}`), IdempotencyPolicy: []byte(`{"key":"message_id"}`), ProgressWeight: 1},
		},
		Dependencies: []persistence.PipelineDependencyInput{{NodeKey: "finalize", DependsOnNodeKey: "analysis", Condition: []byte(`{"kind":"success"}`), Required: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func (f *gateEDurableFixture) createProject(t *testing.T, workflowKey, name string) *product.Project {
	t.Helper()
	value, err := f.backend.CreateProject(f.workspaceID, name, workflowKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func gateESuffix() string {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return hex.EncodeToString(raw[:])
	}
	return strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
}

func readSSEFrame(t *testing.T, response *http.Response) string {
	t.Helper()
	reader := bufio.NewReader(response.Body)
	var frame strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		frame.WriteString(line)
		if strings.HasSuffix(frame.String(), "\n\n") {
			return frame.String()
		}
	}
}

func artifactResult(messageID, jobID, stepID string) worker.Result {
	return artifactResultWithID("gate_e_report", messageID, jobID, stepID)
}

func artifactResultWithID(artifactID, messageID, jobID, stepID string) worker.Result {
	return worker.Result{
		SchemaVersion: worker.ResultSchemaVersion, MessageID: messageID, JobID: jobID, JobStepID: stepID, Status: "completed",
		OutputRefs: []worker.OutputRef{{ArtifactID: artifactID, Kind: "deterministic_analysis", Role: "analysis_report", SHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", SizeBytes: 0}},
	}
}

func failedTransientResult(messageID, jobID, stepID string) worker.Result {
	return worker.Result{
		SchemaVersion: worker.ResultSchemaVersion, MessageID: messageID, JobID: jobID, JobStepID: stepID, Status: "failed", OutputRefs: []worker.OutputRef{},
		SafeError: &worker.SafeError{Code: "TEMPORARY_GATE_E_FAILURE", Category: "transient", Retryable: true, SafeMessage: "A disposable transient failure was injected."},
	}
}

func TestProductAPIDurableClientDisconnectReconnect(t *testing.T) {
	f := newGateEDurableFixture(t)
	token := f.login(t)
	projectStatus, projectBody := f.request(t, http.MethodPost, "/api/v1/projects", token, map[string]any{"name": "client-disconnect", "workflow_key": "movie_recap"}, map[string]string{"Idempotency-Key": "gate-e-project-" + gateESuffix()})
	if projectStatus != http.StatusCreated {
		t.Fatalf("project status=%d body=%s", projectStatus, projectBody)
	}
	var project product.Project
	if err := json.Unmarshal(projectBody, &project); err != nil {
		t.Fatal(err)
	}
	if project.ID == "" {
		t.Fatal("Product API did not return a Project ID")
	}
	defer f.closeProject(t, project.ID, "")
	validJobBody := map[string]any{"kind": "asset_probe", "mode": "automatic", "input": map[string]any{"asset_id": "asset_gate_e"}, "start_from": "asset_probe", "enable_dlq": true}
	jobStatus, jobBody := f.request(t, http.MethodPost, "/api/v1/projects/"+project.ID+"/jobs", token, validJobBody, map[string]string{"Idempotency-Key": "gate-e-job-" + gateESuffix()})
	if jobStatus != http.StatusCreated {
		t.Fatalf("job status=%d body=%s", jobStatus, jobBody)
	}
	var job product.Job
	if err := json.Unmarshal(jobBody, &job); err != nil {
		t.Fatal(err)
	}
	invalidStatus, _ := f.request(t, http.MethodPost, "/api/v1/projects/"+project.ID+"/jobs", token, map[string]any{"kind": "asset_probe", "start_from": "missing-node"}, map[string]string{"Idempotency-Key": "gate-e-invalid-" + gateESuffix()})
	if invalidStatus != http.StatusUnprocessableEntity {
		t.Fatalf("invalid start_from status=%d", invalidStatus)
	}
	startStatus, startBody := f.request(t, http.MethodPost, "/api/v1/projects/"+project.ID+"/jobs/"+job.ID+"/start", token, nil, nil)
	if startStatus != http.StatusAccepted {
		t.Fatalf("start status=%d body=%s", startStatus, startBody)
	}

	// The observing client disconnects after receiving the durable snapshot.
	streamContext, disconnect := context.WithCancel(context.Background())
	streamRequest, err := http.NewRequestWithContext(streamContext, http.MethodGet, f.server.URL+"/api/v1/projects/"+project.ID+"/jobs/"+job.ID+"/events/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	streamRequest.Header.Set("Authorization", "Bearer "+token)
	streamResponse, err := f.client.Do(streamRequest)
	if err != nil {
		t.Fatal(err)
	}
	if streamResponse.StatusCode != http.StatusOK {
		t.Fatalf("initial SSE status=%d", streamResponse.StatusCode)
	}
	if frame := readSSEFrame(t, streamResponse); !strings.Contains(frame, "stream.snapshot") || !strings.Contains(frame, "steps") {
		t.Fatalf("initial SSE snapshot did not include canonical state: %s", frame)
	}
	disconnect()
	_ = streamResponse.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	deliveries, err := f.queue.Consume(ctx, "probe", "gate-e-worker-crashed", 1, time.Second)
	if err != nil || len(deliveries) != 1 {
		t.Fatalf("first worker delivery=%d err=%v", len(deliveries), err)
	}
	time.Sleep(1200 * time.Millisecond)
	reclaimed, err := f.queue.Reclaim(ctx, "probe", "gate-e-worker-restarted", time.Second, 1)
	if err != nil || len(reclaimed) != 1 {
		t.Fatalf("reclaimed worker delivery=%d err=%v", len(reclaimed), err)
	}
	delivery := reclaimed[0]
	resolver := persistence.ScopedClaimResolver{SQL: f.store, Workspace: f.workspaceID, WorkerID: "gate-e-controller", Duration: time.Minute}
	command, lease, err := resolver.ClaimCommand(ctx, delivery.Message)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.queue.Ack(ctx, delivery.Stream, delivery.EntryID); err != nil {
		t.Fatal(err)
	}
	artifactRepository, err := persistence.NewSQLArtifactRepository(f.store, f.userID, f.workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	applier := persistence.ScopedResultApplier{SQL: f.store, Workspace: f.workspaceID, WorkerID: "gate-e-controller", Artifacts: persistence.WorkerArtifactCommitter{Storage: f.objectStore, Repository: artifactRepository}}
	claimed := execution.ClaimedResult{Delivery: queue.ResultDelivery{Stream: "gate-e-results", EntryID: "1-0", Attempt: lease.Attempt, Result: artifactResult(command.MessageID, command.JobID, command.JobStepID)}, Claim: execution.ClaimedCommand{Message: delivery.Message, Command: command, Lease: lease}}
	if err := applier.ApplyClaimedResult(ctx, claimed); err != nil {
		t.Fatal(err)
	}

	getStatus, getBody := f.request(t, http.MethodGet, "/api/v1/projects/"+project.ID+"/jobs/"+job.ID, token, nil, nil)
	if getStatus != http.StatusOK {
		t.Fatalf("reconnected REST state status=%d body=%s", getStatus, getBody)
	}
	var final product.Job
	if err := json.Unmarshal(getBody, &final); err != nil || final.Status != "completed" {
		t.Fatalf("canonical REST state=%#v body=%s err=%v", final, getBody, err)
	}
	replayRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, f.server.URL+"/api/v1/projects/"+project.ID+"/jobs/"+job.ID+"/events/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	replayRequest.Header.Set("Authorization", "Bearer "+token)
	replayRequest.Header.Set("Last-Event-ID", "1")
	replayResponse, err := f.client.Do(replayRequest)
	if err != nil {
		t.Fatal(err)
	}
	replayBody, err := io.ReadAll(replayResponse.Body)
	_ = replayResponse.Body.Close()
	if err != nil || replayResponse.StatusCode != http.StatusOK {
		t.Fatalf("SSE reconnect status=%d err=%v body=%s", replayResponse.StatusCode, err, replayBody)
	}
	if !bytes.Contains(replayBody, []byte("job.completed")) || !bytes.Contains(replayBody, []byte("node.completed")) {
		t.Fatalf("SSE reconnect missed terminal replay: %s", replayBody)
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(replayBody), "\n") {
		if strings.HasPrefix(line, "id: ") {
			id := strings.TrimSpace(strings.TrimPrefix(line, "id: "))
			if id != "" && seen[id] {
				t.Fatalf("duplicate SSE event ID %s", id)
			}
			seen[id] = true
		}
	}
	restStatus, restBody := f.request(t, http.MethodGet, "/api/v1/projects/"+project.ID+"/jobs/"+job.ID+"/events?after_sequence=0&limit=101", token, nil, nil)
	if restStatus != http.StatusOK {
		t.Fatalf("REST replay status=%d body=%s", restStatus, restBody)
	}
	var rest struct {
		Items []product.JobEvent `json:"items"`
	}
	if err := json.Unmarshal(restBody, &rest); err != nil || len(rest.Items) == 0 || len(rest.Items) > 100 {
		t.Fatalf("REST replay/bounded limit items=%d err=%v body=%s", len(rest.Items), err, restBody)
	}
	unauthStatus, _ := f.request(t, http.MethodGet, "/api/v1/projects/"+project.ID+"/jobs/"+job.ID+"/events/stream", "", nil, nil)
	if unauthStatus != http.StatusUnauthorized {
		t.Fatalf("unauthenticated SSE status=%d", unauthStatus)
	}
	invalidCursor, _ := f.request(t, http.MethodGet, "/api/v1/projects/"+project.ID+"/jobs/"+job.ID+"/events/stream", token, nil, map[string]string{"Last-Event-ID": "not-a-sequence"})
	if invalidCursor != http.StatusBadRequest {
		t.Fatalf("invalid cursor status=%d", invalidCursor)
	}
	foreign, _ := f.request(t, http.MethodGet, "/api/v1/projects/00000000-0000-0000-0000-000000000000/jobs/"+job.ID+"/events/stream", token, nil, nil)
	if foreign != http.StatusNotFound {
		t.Fatalf("foreign Project SSE status=%d", foreign)
	}
	f.backend.SetQueue(nil)
	if durable, err := f.backend.GetJob(f.workspaceID, project.ID, job.ID); err != nil || durable.Status != "completed" {
		t.Fatalf("PostgreSQL authoritative state after Redis loss: %#v err=%v", durable, err)
	}
}

func TestProductAPIDurableStopAfterResumeAndPausedReconnect(t *testing.T) {
	f := newGateEDurableFixture(t)
	token := f.login(t)
	workflowKey := f.createTwoNodeWorkflow(t, gateESuffix())
	project := f.createProject(t, workflowKey, "partial-resume")
	defer f.closeProject(t, project.ID, "")
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	invalidStatus, _ := f.request(t, http.MethodPost, "/api/v1/projects/"+project.ID+"/jobs", token, map[string]any{"kind": "pipeline", "workflow_key": workflowKey, "stop_after": "missing-node"}, map[string]string{"Idempotency-Key": "gate-e-invalid-stop-" + gateESuffix()})
	if invalidStatus != http.StatusUnprocessableEntity {
		t.Fatalf("invalid stop_after status=%d", invalidStatus)
	}
	jobStatus, jobBody := f.request(t, http.MethodPost, "/api/v1/projects/"+project.ID+"/jobs", token, map[string]any{"kind": "pipeline", "workflow_key": workflowKey, "start_from": "analysis", "stop_after": "analysis"}, map[string]string{"Idempotency-Key": "gate-e-partial-job-" + gateESuffix()})
	if jobStatus != http.StatusCreated {
		t.Fatalf("partial job status=%d body=%s", jobStatus, jobBody)
	}
	var job product.Job
	if err := json.Unmarshal(jobBody, &job); err != nil {
		t.Fatal(err)
	}
	if startStatus, body := f.request(t, http.MethodPost, "/api/v1/projects/"+project.ID+"/jobs/"+job.ID+"/start", token, nil, nil); startStatus != http.StatusAccepted {
		t.Fatalf("partial start status=%d body=%s", startStatus, body)
	}
	deliveries, err := f.queue.Consume(ctx, "probe", "gate-e-partial-worker-1", 1, time.Second)
	if err != nil || len(deliveries) != 1 || deliveries[0].Message.NodeKey != "analysis" {
		t.Fatalf("partial first delivery=%+v err=%v", deliveries, err)
	}
	delivery := deliveries[0]
	command, lease, err := (persistence.ScopedClaimResolver{SQL: f.store, Workspace: f.workspaceID, WorkerID: "gate-e-partial-worker-1", Duration: time.Minute}).ClaimCommand(ctx, delivery.Message)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.queue.Ack(ctx, delivery.Stream, delivery.EntryID); err != nil {
		t.Fatal(err)
	}
	artifactRepository, err := persistence.NewSQLArtifactRepository(f.store, f.userID, f.workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	applier := persistence.ScopedResultApplier{SQL: f.store, Workspace: f.workspaceID, WorkerID: "gate-e-partial-worker-1", Artifacts: persistence.WorkerArtifactCommitter{Storage: f.objectStore, Repository: artifactRepository}}
	claimed := execution.ClaimedResult{Delivery: queue.ResultDelivery{Stream: "gate-e-partial-results", EntryID: "1-0", Attempt: lease.Attempt, Result: artifactResultWithID("gate_e_partial_analysis", command.MessageID, command.JobID, command.JobStepID)}, Claim: execution.ClaimedCommand{Message: delivery.Message, Command: command, Lease: lease}}
	if err := applier.ApplyClaimedResult(ctx, claimed); err != nil {
		t.Fatal(err)
	}
	pausedStatus, pausedBody := f.request(t, http.MethodGet, "/api/v1/projects/"+project.ID+"/jobs/"+job.ID, token, nil, nil)
	if pausedStatus != http.StatusOK {
		t.Fatalf("paused GET status=%d body=%s", pausedStatus, pausedBody)
	}
	var paused product.Job
	if err := json.Unmarshal(pausedBody, &paused); err != nil || paused.Status != "paused" {
		t.Fatalf("stop_after did not pause job=%#v err=%v", paused, err)
	}
	streamRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, f.server.URL+"/api/v1/projects/"+project.ID+"/jobs/"+job.ID+"/events/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	streamRequest.Header.Set("Authorization", "Bearer "+token)
	streamResponse, err := f.client.Do(streamRequest)
	if err != nil {
		t.Fatal(err)
	}
	if frame := readSSEFrame(t, streamResponse); !strings.Contains(frame, "stream.snapshot") || !strings.Contains(frame, "paused") {
		t.Fatalf("paused reconnect snapshot=%s", frame)
	}
	_ = streamResponse.Body.Close()
	if resumeStatus, body := f.request(t, http.MethodPost, "/api/v1/projects/"+project.ID+"/jobs/"+job.ID+"/resume", token, nil, nil); resumeStatus != http.StatusAccepted {
		t.Fatalf("resume status=%d body=%s", resumeStatus, body)
	}
	next, err := f.queue.Consume(ctx, "probe", "gate-e-partial-worker-2", 1, time.Second)
	if err != nil || len(next) != 1 || next[0].Message.NodeKey != "finalize" {
		t.Fatalf("resume did not queue first incomplete node=%+v err=%v", next, err)
	}
	nextDelivery := next[0]
	nextCommand, nextLease, err := (persistence.ScopedClaimResolver{SQL: f.store, Workspace: f.workspaceID, WorkerID: "gate-e-partial-worker-2", Duration: time.Minute}).ClaimCommand(ctx, nextDelivery.Message)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.queue.Ack(ctx, nextDelivery.Stream, nextDelivery.EntryID); err != nil {
		t.Fatal(err)
	}
	nextApplier := persistence.ScopedResultApplier{SQL: f.store, Workspace: f.workspaceID, WorkerID: "gate-e-partial-worker-2", Artifacts: persistence.WorkerArtifactCommitter{Storage: f.objectStore, Repository: artifactRepository}}
	nextClaim := execution.ClaimedResult{Delivery: queue.ResultDelivery{Stream: "gate-e-partial-results", EntryID: "2-0", Attempt: nextLease.Attempt, Result: artifactResultWithID("gate_e_partial_finalize", nextCommand.MessageID, nextCommand.JobID, nextCommand.JobStepID)}, Claim: execution.ClaimedCommand{Message: nextDelivery.Message, Command: nextCommand, Lease: nextLease}}
	if err := nextApplier.ApplyClaimedResult(ctx, nextClaim); err != nil {
		t.Fatal(err)
	}
	finalStatus, finalBody := f.request(t, http.MethodGet, "/api/v1/projects/"+project.ID+"/jobs/"+job.ID, token, nil, nil)
	if finalStatus != http.StatusOK {
		t.Fatalf("partial final GET status=%d body=%s", finalStatus, finalBody)
	}
	var final product.Job
	if err := json.Unmarshal(finalBody, &final); err != nil || final.Status != "completed" {
		t.Fatalf("resume did not complete descendants=%#v err=%v", final, err)
	}
}

func TestDurableCrashResumeBoundariesAndStaleLease(t *testing.T) {
	f := newGateEDurableFixture(t)
	workflowKey := f.createSingleNodeWorkflow(t, "resume-"+gateESuffix(), 2, "none")
	project := f.createProject(t, workflowKey, "crash-resume")
	defer f.closeProject(t, project.ID, "")
	job, err := f.backend.CreateExecutionJob(f.workspaceID, project.ID, product.JobCreateInput{Kind: "analysis", StartFrom: "analysis"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	f.backend.SetQueue(nil)
	if _, err := f.backend.StartJob(f.workspaceID, project.ID, job.ID); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := f.db.QueryRowContext(ctx, `SELECT status FROM job_steps WHERE pipeline_run_id=$1::uuid`, job.PipelineRunID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Fatalf("queued-before-publish step status=%s", status)
	}
	f.backend.SetQueue(f.queue)
	if _, err := f.backend.StartJob(f.workspaceID, project.ID, job.ID); err != nil {
		t.Fatal(err)
	}
	deliveries, err := f.queue.Consume(ctx, "probe", "gate-e-resume-worker-1", 1, time.Second)
	if err != nil || len(deliveries) != 1 {
		t.Fatalf("queued recovery delivery=%d err=%v", len(deliveries), err)
	}
	time.Sleep(1200 * time.Millisecond)
	reclaimed, err := f.queue.Reclaim(ctx, "probe", "gate-e-resume-worker-2", time.Second, 1)
	if err != nil || len(reclaimed) != 1 {
		t.Fatalf("crash reclaim delivery=%d err=%v", len(reclaimed), err)
	}
	delivery := reclaimed[0]
	resolver := persistence.ScopedClaimResolver{SQL: f.store, Workspace: f.workspaceID, WorkerID: "gate-e-resume-worker", Duration: 250 * time.Millisecond}
	command, lease, err := resolver.ClaimCommand(ctx, delivery.Message)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.queue.Ack(ctx, delivery.Stream, delivery.EntryID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(350 * time.Millisecond)
	if err := f.store.ReconcileExpiredStep(ctx, execution.LeaseClaim{WorkspaceID: f.workspaceID, ProjectID: project.ID, JobID: job.ID, PipelineRunID: delivery.Message.PipelineRunID, JobStepID: delivery.Message.JobStepID, PipelineNodeID: delivery.Message.PipelineNodeID, NodeKey: delivery.Message.NodeKey}); err != nil {
		t.Fatal(err)
	}
	stale := worker.Result{SchemaVersion: worker.ResultSchemaVersion, MessageID: command.MessageID, JobID: command.JobID, JobStepID: command.JobStepID, Status: "completed", OutputRefs: []worker.OutputRef{}}
	if err := f.store.ApplyWorkerResult(ctx, persistence.WorkerResultInput{WorkspaceID: f.workspaceID, ProjectID: project.ID, JobID: job.ID, PipelineRunID: delivery.Message.PipelineRunID, JobStepID: delivery.Message.JobStepID, PipelineNodeID: delivery.Message.PipelineNodeID, NodeKey: delivery.Message.NodeKey, WorkerID: "gate-e-resume-worker", LeaseToken: lease.Token, Attempt: lease.Attempt, Result: stale}); !errors.Is(err, execution.ErrLeaseConflict) {
		t.Fatalf("stale worker result was accepted: %v", err)
	}
	time.Sleep(25 * time.Millisecond)
	if queued, err := f.backend.RequeueRetryingSteps(ctx); err != nil || queued != 1 {
		t.Fatalf("retry resume queued=%d err=%v", queued, err)
	}
	retryDeliveries, err := f.queue.Consume(ctx, "probe", "gate-e-resume-worker-3", 1, time.Second)
	if err != nil || len(retryDeliveries) != 1 || retryDeliveries[0].Message.Attempt != 2 {
		t.Fatalf("retry delivery=%d err=%v value=%+v", len(retryDeliveries), err, retryDeliveries)
	}
	retryDelivery := retryDeliveries[0]
	retryCommand, retryLease, err := (persistence.ScopedClaimResolver{SQL: f.store, Workspace: f.workspaceID, WorkerID: "gate-e-resume-worker-3", Duration: time.Minute}).ClaimCommand(ctx, retryDelivery.Message)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.queue.Ack(ctx, retryDelivery.Stream, retryDelivery.EntryID); err != nil {
		t.Fatal(err)
	}
	staged, err := f.objectStore.PutStaged(ctx, storage.Scope{WorkspaceID: f.workspaceID, ProjectID: project.ID}, strings.NewReader("staged-but-unreconciled"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	defer f.objectStore.Delete(ctx, staged.Locator, storage.Preconditions{})
	var stagedArtifactCount int
	if err := f.db.QueryRowContext(ctx, `SELECT count(*) FROM artifacts WHERE project_id=$1::uuid AND object_key=$2`, project.ID, staged.Locator.ObjectKey).Scan(&stagedArtifactCount); err != nil {
		t.Fatal(err)
	}
	if stagedArtifactCount != 0 {
		t.Fatalf("staged object became authoritative Artifact before reconciliation: %d", stagedArtifactCount)
	}
	artifactRepository, err := persistence.NewSQLArtifactRepository(f.store, f.userID, f.workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	applier := persistence.ScopedResultApplier{SQL: f.store, Workspace: f.workspaceID, WorkerID: "gate-e-resume-worker-3", Artifacts: persistence.WorkerArtifactCommitter{Storage: f.objectStore, Repository: artifactRepository}}
	claimed := execution.ClaimedResult{Delivery: queue.ResultDelivery{Stream: "gate-e-resume-results", EntryID: "1-0", Attempt: retryLease.Attempt, Result: artifactResult(retryCommand.MessageID, retryCommand.JobID, retryCommand.JobStepID)}, Claim: execution.ClaimedCommand{Message: retryDelivery.Message, Command: retryCommand, Lease: retryLease}}
	if err := applier.ApplyClaimedResult(ctx, claimed); err != nil {
		t.Fatal(err)
	}
	var eventsBefore int
	if err := f.db.QueryRowContext(ctx, `SELECT count(*) FROM job_events WHERE job_id=$1::uuid`, job.ID).Scan(&eventsBefore); err != nil {
		t.Fatal(err)
	}
	if err := applier.ApplyClaimedResult(ctx, claimed); !errors.Is(err, execution.ErrLeaseConflict) {
		t.Fatalf("duplicate completed result was accepted: %v", err)
	}
	var finalJob, finalRun, finalStep string
	if err := f.db.QueryRowContext(ctx, `SELECT j.status,r.status,s.status FROM jobs j JOIN pipeline_runs r ON r.id=j.current_pipeline_run_id JOIN job_steps s ON s.pipeline_run_id=r.id WHERE j.id=$1::uuid`, job.ID).Scan(&finalJob, &finalRun, &finalStep); err != nil {
		t.Fatal(err)
	}
	if finalJob != "completed" || finalRun != "completed" || finalStep != "completed" {
		t.Fatalf("aggregate after resume job=%s run=%s step=%s", finalJob, finalRun, finalStep)
	}
	var eventsAfter, committedArtifacts int
	if err := f.db.QueryRowContext(ctx, `SELECT count(*) FROM job_events WHERE job_id=$1::uuid`, job.ID).Scan(&eventsAfter); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRowContext(ctx, `SELECT count(*) FROM artifacts WHERE project_id=$1::uuid AND producer_job_id=$2::uuid AND status='committed'`, project.ID, job.ID).Scan(&committedArtifacts); err != nil {
		t.Fatal(err)
	}
	if eventsAfter != eventsBefore || committedArtifacts != 1 {
		t.Fatalf("duplicate terminal output changed durable history events_before=%d events_after=%d artifacts=%d", eventsBefore, eventsAfter, committedArtifacts)
	}
}

func TestDurableCancellationRetryExhaustionDLQAndReplay(t *testing.T) {
	f := newGateEDurableFixture(t)
	workflowKey := f.createSingleNodeWorkflow(t, "dlq-"+gateESuffix(), 2, "none")
	project := f.createProject(t, workflowKey, "cancel-dlq")
	defer f.closeProject(t, project.ID, "")
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	job, err := f.backend.CreateExecutionJob(f.workspaceID, project.ID, product.JobCreateInput{Kind: "analysis", EnableDLQ: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.backend.StartJob(f.workspaceID, project.ID, job.ID); err != nil {
		t.Fatal(err)
	}
	first, err := f.queue.Consume(ctx, "probe", "gate-e-dlq-worker-1", 1, time.Second)
	if err != nil || len(first) != 1 {
		t.Fatalf("DLQ first delivery=%d err=%v", len(first), err)
	}
	firstDelivery := first[0]
	firstCommand, firstLease, err := (persistence.ScopedClaimResolver{SQL: f.store, Workspace: f.workspaceID, WorkerID: "gate-e-dlq-worker-1", Duration: time.Minute}).ClaimCommand(ctx, firstDelivery.Message)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.queue.Ack(ctx, firstDelivery.Stream, firstDelivery.EntryID); err != nil {
		t.Fatal(err)
	}
	firstClaim := execution.ClaimedResult{Delivery: queue.ResultDelivery{Stream: "gate-e-dlq-results", EntryID: "1-0", Attempt: firstLease.Attempt, Result: failedTransientResult(firstCommand.MessageID, firstCommand.JobID, firstCommand.JobStepID)}, Claim: execution.ClaimedCommand{Message: firstDelivery.Message, Command: firstCommand, Lease: firstLease}}
	firstApplier := persistence.ScopedResultApplier{SQL: f.store, Workspace: f.workspaceID, WorkerID: "gate-e-dlq-worker-1"}
	if err := firstApplier.ApplyClaimedResult(ctx, firstClaim); err != nil {
		t.Fatal(err)
	}
	time.Sleep(25 * time.Millisecond)
	if queued, err := f.backend.RequeueRetryingSteps(ctx); err != nil || queued != 1 {
		t.Fatalf("DLQ retry queue=%d err=%v", queued, err)
	}
	second, err := f.queue.Consume(ctx, "probe", "gate-e-dlq-worker-2", 1, time.Second)
	if err != nil || len(second) != 1 {
		t.Fatalf("DLQ second delivery=%d err=%v", len(second), err)
	}
	secondDelivery := second[0]
	secondCommand, secondLease, err := (persistence.ScopedClaimResolver{SQL: f.store, Workspace: f.workspaceID, WorkerID: "gate-e-dlq-worker-2", Duration: time.Minute}).ClaimCommand(ctx, secondDelivery.Message)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.queue.Ack(ctx, secondDelivery.Stream, secondDelivery.EntryID); err != nil {
		t.Fatal(err)
	}
	secondClaim := execution.ClaimedResult{Delivery: queue.ResultDelivery{Stream: "gate-e-dlq-results", EntryID: "2-0", Attempt: secondLease.Attempt, Result: failedTransientResult(secondCommand.MessageID, secondCommand.JobID, secondCommand.JobStepID)}, Claim: execution.ClaimedCommand{Message: secondDelivery.Message, Command: secondCommand, Lease: secondLease}}
	secondApplier := persistence.ScopedResultApplier{SQL: f.store, Workspace: f.workspaceID, WorkerID: "gate-e-dlq-worker-2"}
	if err := secondApplier.ApplyClaimedResult(ctx, secondClaim); err != nil {
		t.Fatal(err)
	}
	final, err := f.backend.GetJob(f.workspaceID, project.ID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != "dead_lettered" {
		t.Fatalf("retry exhaustion status=%s", final.Status)
	}
	var deadLetters int
	var safeError string
	if err := f.db.QueryRowContext(ctx, `SELECT count(*),COALESCE((SELECT safe_error::text FROM dead_letters WHERE job_id=$1::uuid), '') FROM jobs WHERE id=$1::uuid`, job.ID).Scan(&deadLetters, &safeError); err != nil {
		t.Fatal(err)
	}
	if deadLetters != 1 || strings.Contains(strings.ToLower(safeError), "traceback") || strings.Contains(strings.ToLower(safeError), "path") {
		t.Fatalf("DLQ record unsafe or missing count=%d safe_error=%s", deadLetters, safeError)
	}
	var eventCount int
	if err := f.db.QueryRowContext(ctx, `SELECT count(*) FROM job_events WHERE job_id=$1::uuid`, job.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	replayed, err := f.backend.RetryExecutionJob(f.workspaceID, project.ID, job.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ID == job.ID || replayed.Command["supersedes_job_id"] != job.ID {
		t.Fatalf("replay lineage old=%s new=%s command=%#v", job.ID, replayed.ID, replayed.Command)
	}
	if old, err := f.backend.GetJob(f.workspaceID, project.ID, job.ID); err != nil || old.Status != "dead_lettered" {
		t.Fatalf("terminal predecessor mutated: %#v err=%v", old, err)
	}
	var eventCountAfter int
	if err := f.db.QueryRowContext(ctx, `SELECT count(*) FROM job_events WHERE job_id=$1::uuid`, job.ID).Scan(&eventCountAfter); err != nil {
		t.Fatal(err)
	}
	if eventCountAfter != eventCount {
		t.Fatalf("replay mutated predecessor event history before=%d after=%d", eventCount, eventCountAfter)
	}

	queuedCancel, err := f.backend.CreateExecutionJob(f.workspaceID, project.ID, product.JobCreateInput{Kind: "analysis"})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := f.backend.CancelJob(f.workspaceID, project.ID, queuedCancel.ID)
	if err != nil || cancelled.Status != "cancelled" {
		t.Fatalf("queued cancellation status=%v err=%v", cancelled, err)
	}
	var queuedRunStatus, queuedStepStatus string
	if err := f.db.QueryRowContext(ctx, `SELECT r.status,s.status FROM pipeline_runs r JOIN job_steps s ON s.pipeline_run_id=r.id WHERE r.job_id=$1::uuid`, queuedCancel.ID).Scan(&queuedRunStatus, &queuedStepStatus); err != nil {
		t.Fatal(err)
	}
	if queuedRunStatus != "cancelled" || queuedStepStatus != "cancelled" {
		t.Fatalf("queued cancellation graph run=%s step=%s", queuedRunStatus, queuedStepStatus)
	}

	runningCancel, err := f.backend.CreateExecutionJob(f.workspaceID, project.ID, product.JobCreateInput{Kind: "analysis"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.backend.StartJob(f.workspaceID, project.ID, runningCancel.ID); err != nil {
		t.Fatal(err)
	}
	runningDelivery, err := f.queue.Consume(ctx, "probe", "gate-e-cancel-worker", 1, time.Second)
	if err != nil || len(runningDelivery) != 1 {
		t.Fatalf("running cancellation delivery=%d err=%v", len(runningDelivery), err)
	}
	claimedDelivery := runningDelivery[0]
	cancelCommand, cancelLease, err := (persistence.ScopedClaimResolver{SQL: f.store, Workspace: f.workspaceID, WorkerID: "gate-e-cancel-worker", Duration: time.Minute}).ClaimCommand(ctx, claimedDelivery.Message)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.queue.Ack(ctx, claimedDelivery.Stream, claimedDelivery.EntryID); err != nil {
		t.Fatal(err)
	}
	requested, err := f.backend.CancelJob(f.workspaceID, project.ID, runningCancel.ID)
	if err != nil || requested.Status != "cancelling" {
		t.Fatalf("running cancellation request status=%v err=%v", requested, err)
	}
	staleResult := worker.Result{SchemaVersion: worker.ResultSchemaVersion, MessageID: cancelCommand.MessageID, JobID: cancelCommand.JobID, JobStepID: cancelCommand.JobStepID, Status: "completed", OutputRefs: []worker.OutputRef{}}
	if err := f.store.ApplyWorkerResult(ctx, persistence.WorkerResultInput{WorkspaceID: f.workspaceID, ProjectID: project.ID, JobID: runningCancel.ID, PipelineRunID: claimedDelivery.Message.PipelineRunID, JobStepID: claimedDelivery.Message.JobStepID, PipelineNodeID: claimedDelivery.Message.PipelineNodeID, NodeKey: claimedDelivery.Message.NodeKey, WorkerID: "gate-e-cancel-worker", LeaseToken: cancelLease.Token, Attempt: cancelLease.Attempt, Result: staleResult}); !errors.Is(err, execution.ErrLeaseConflict) {
		t.Fatalf("stale worker committed after cancellation: %v", err)
	}
	cancelResult := worker.Result{SchemaVersion: worker.ResultSchemaVersion, MessageID: cancelCommand.MessageID, JobID: cancelCommand.JobID, JobStepID: cancelCommand.JobStepID, Status: "cancelled", OutputRefs: []worker.OutputRef{}}
	if err := f.store.ApplyWorkerResult(ctx, persistence.WorkerResultInput{WorkspaceID: f.workspaceID, ProjectID: project.ID, JobID: runningCancel.ID, PipelineRunID: claimedDelivery.Message.PipelineRunID, JobStepID: claimedDelivery.Message.JobStepID, PipelineNodeID: claimedDelivery.Message.PipelineNodeID, NodeKey: claimedDelivery.Message.NodeKey, WorkerID: "gate-e-cancel-worker", LeaseToken: cancelLease.Token, Attempt: cancelLease.Attempt, Result: cancelResult}); err != nil {
		t.Fatal(err)
	}
	terminal, err := f.backend.CancelJob(f.workspaceID, project.ID, runningCancel.ID)
	if err != nil || terminal.Status != "cancelled" {
		t.Fatalf("terminal cancellation status=%v err=%v", terminal, err)
	}
}

func TestGateESafeRetryTaxonomyMatrix(t *testing.T) {
	cases := []struct {
		name        string
		category    execution.FailureCategory
		timeoutOK   bool
		resourceOK  bool
		shouldRetry bool
	}{
		{name: "transient", category: execution.FailureTransient, shouldRetry: true},
		{name: "provider throttling", category: execution.FailureProviderThrottle, shouldRetry: true},
		{name: "timeout policy", category: execution.FailureTimeout, timeoutOK: true, shouldRetry: true},
		{name: "timeout denied", category: execution.FailureTimeout},
		{name: "resource policy", category: execution.FailureResource, resourceOK: true, shouldRetry: true},
		{name: "invalid input", category: execution.FailureInvalidInput, timeoutOK: true, resourceOK: true},
		{name: "cancellation", category: execution.FailureCancellation, timeoutOK: true, resourceOK: true},
		{name: "permanent", category: execution.FailurePermanent, timeoutOK: true, resourceOK: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := execution.Retryable(testCase.category, testCase.timeoutOK, testCase.resourceOK); got != testCase.shouldRetry {
				t.Fatalf("Retryable(%q)=%v want %v", testCase.category, got, testCase.shouldRetry)
			}
		})
	}
}
