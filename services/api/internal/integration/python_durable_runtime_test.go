package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/auth"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/persistence"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/queue"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

func TestPythonWorkerDurableArtifactRuntime(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_DATABASE_URL"))
	redisAddress := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_REDIS_ADDR"))
	minioEndpoint := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_MINIO_ENDPOINT"))
	accessKey := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_MINIO_ACCESS_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_MINIO_SECRET_KEY"))
	bucket := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_MINIO_BUCKET"))
	if dsn == "" || redisAddress == "" || minioEndpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		t.Skip("set PostgreSQL, Redis and MinIO test variables for the combined Python durable harness")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	db, err := persistence.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	sqlStore, err := persistence.NewSQLStore(db)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap, err := sqlStore.BootstrapLocalInstallation(ctx, persistence.BootstrapInput{Username: "runtime-python-admin", DisplayName: "Runtime Python Admin", PasswordHash: auth.CredentialHash("runtime-python-password"), WorkspaceSlug: "runtime-python", WorkspaceName: "Runtime Python"})
	if err != nil {
		t.Fatal(err)
	}
	workflowKey := "runtime_python_" + time.Now().UTC().Format("150405.000")
	if _, err := sqlStore.EnsurePipelineDefinition(ctx, persistence.PipelineDefinitionInput{
		WorkflowKey: workflowKey, WorkflowName: "Runtime Python", Description: "deterministic integration workflow", SchemaVersion: "1.0", Definition: []byte(`{"kind":"runtime_python","nodes":["analysis"]}`), Version: 1, Status: "active", CreatedBy: bootstrap.UserID,
		Nodes: []persistence.PipelineNodeInput{{NodeKey: "analysis", NodeType: "analysis", DisplayName: "Deterministic analysis", ExecutionClass: "ml", FailureMode: "hard", ReviewPolicy: "none", Config: []byte(`{"mode":"deterministic","test_fail_attempt":1}`), TimeoutSec: 60, MaxAttempts: 2, RetryPolicy: []byte(`{"base_delay_ms":50,"max_delay_ms":500}`), CheckpointPolicy: []byte(`{"enabled":false}`), ResourceRequirements: []byte(`{"network":"none"}`), IdempotencyPolicy: []byte(`{"key":"message_id"}`), ProgressWeight: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	project, err := sqlStore.CreateProject(ctx, bootstrap.UserID, bootstrap.WorkspaceID, "python-"+time.Now().UTC().Format("150405.000"), workflowKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	job, err := sqlStore.CreateProjectJob(ctx, bootstrap.UserID, bootstrap.WorkspaceID, project.ID, "analysis", []byte(`{"mode":"deterministic"}`), []byte(`{"mode":"deterministic"}`))
	if err != nil {
		t.Fatal(err)
	}
	objectStore, err := storage.NewS3Storage(minioEndpoint, accessKey, secretKey, bucket, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupDurableRuntime(t, db, objectStore, project.ID, job.ID) })
	prefix := "nh-python-durable-" + strings.ReplaceAll(job.ID, "-", "_")
	transport, err := queue.NewRedisQueue(queue.RedisOptions{Addr: redisAddress, Password: os.Getenv("NH_QUEUE_PASSWORD"), StreamPrefix: prefix, ConsumerGroup: "nh-python-durable-workers"})
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	backend, err := persistence.NewDurableBackend(sqlStore, bootstrap.UserID, bootstrap.WorkspaceID, objectStore)
	if err != nil {
		t.Fatal(err)
	}
	backend.SetQueue(transport)
	if _, err := backend.StartJob(bootstrap.WorkspaceID, project.ID, job.ID); err != nil {
		t.Fatal(err)
	}

	endpoint := redisAddress
	if !strings.HasPrefix(endpoint, "redis://") && !strings.HasPrefix(endpoint, "rediss://") {
		endpoint = "redis://" + endpoint
	}
	root := repositoryRoot(t)
	python := exec.CommandContext(ctx, "uv", "run", "--project", filepath.Join(root, "services", "ml-worker"), "python", "-m", "nh_media.worker")
	python.Dir = root
	python.Stdout = os.Stdout
	python.Stderr = os.Stderr
	python.Env = append(os.Environ(), "NH_MEDIA_REDIS_WORKER=1", "NH_QUEUE_ENDPOINT="+endpoint, "NH_QUEUE_PASSWORD="+os.Getenv("NH_QUEUE_PASSWORD"), "NH_QUEUE_STREAM_PREFIX="+prefix, "NH_MEDIA_QUEUE_CAPABILITY=analysis", "NH_QUEUE_CONSUMER_GROUP=nh-python-durable-workers", "NH_MEDIA_WORKER_CONSUMER=python-durable-1")
	if err := python.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if python.Process != nil {
			stopProcessTree(python)
		}
	}()

	claims := execution.NewLeaseRegistry()
	workerID := "python-durable-controller"
	dispatcher := execution.ClaimingDispatcher{Queue: transport, Resolver: persistence.ScopedClaimResolver{SQL: sqlStore, Workspace: bootstrap.WorkspaceID, WorkerID: workerID, Duration: 2 * time.Minute}, Publisher: transport, Claims: claims}
	for attempt := 0; attempt < 80; attempt++ {
		processed, dispatchErr := dispatcher.DispatchOnce(ctx, "analysis", "api-python-durable", 1)
		if dispatchErr != nil {
			t.Fatal(dispatchErr)
		}
		if processed == 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	artifactRepository, err := persistence.NewSQLArtifactRepository(sqlStore, bootstrap.UserID, bootstrap.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	reconciler := execution.LeaseAwareResultReconciler{Source: transport, Claims: claims, Applier: persistence.ScopedResultApplier{SQL: sqlStore, Workspace: bootstrap.WorkspaceID, WorkerID: workerID, Artifacts: persistence.WorkerArtifactCommitter{Storage: objectStore, Repository: artifactRepository}}}
	requeued := false
	for attempt := 0; attempt < 160; attempt++ {
		if dispatched, dispatchErr := dispatcher.DispatchOnce(ctx, "analysis", "api-python-durable", 1); dispatchErr != nil {
			t.Fatal(dispatchErr)
		} else if dispatched > 0 {
			t.Logf("dispatched retry attempt on poll %d", attempt)
		}
		processed, reconcileErr := reconciler.ReconcileOnce(ctx, "analysis", "api-python-result", 1, 250*time.Millisecond)
		if reconcileErr != nil {
			t.Fatal(reconcileErr)
		}
		if processed == 1 || !requeued {
			var stepStatus, jobStatus string
			if err := db.QueryRowContext(ctx, `SELECT s.status,j.status FROM job_steps s JOIN pipeline_runs r ON r.id=s.pipeline_run_id JOIN jobs j ON j.id=r.job_id WHERE s.pipeline_run_id=$1::uuid`, job.PipelineRunID).Scan(&stepStatus, &jobStatus); err != nil {
				t.Fatal(err)
			}
			t.Logf("poll=%d processed=%d step=%s job=%s requeued=%v", attempt, processed, stepStatus, jobStatus, requeued)
			if stepStatus == "retrying" && !requeued {
				queued, err := backend.RequeueRetryingSteps(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if queued > 0 {
					requeued = true
				}
			}
			if stepStatus == "completed" && jobStatus == "completed" {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("Python worker result was not reconciled")
}
