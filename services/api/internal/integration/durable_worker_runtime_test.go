package integration

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/auth"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/persistence"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/queue"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

func TestDurableWorkerArtifactRuntime(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_DATABASE_URL"))
	redisAddress := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_REDIS_ADDR"))
	minioEndpoint := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_MINIO_ENDPOINT"))
	accessKey := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_MINIO_ACCESS_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_MINIO_SECRET_KEY"))
	bucket := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_MINIO_BUCKET"))
	if dsn == "" || redisAddress == "" || minioEndpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		t.Skip("set PostgreSQL, Redis and MinIO test variables for the live durable worker harness")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	bootstrap, err := sqlStore.BootstrapLocalInstallation(ctx, persistence.BootstrapInput{Username: "runtime-test-admin", DisplayName: "Runtime Test Admin", PasswordHash: auth.CredentialHash("runtime-test-password"), WorkspaceSlug: "runtime-test", WorkspaceName: "Runtime Test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlStore.EnsureDefaultWorkflow(ctx, bootstrap.UserID); err != nil {
		t.Fatal(err)
	}
	project, err := sqlStore.CreateProject(ctx, bootstrap.UserID, bootstrap.WorkspaceID, "runtime-"+time.Now().UTC().Format("150405.000"), "movie_recap", nil)
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
	transport, err := queue.NewRedisQueue(queue.RedisOptions{Addr: redisAddress, Password: os.Getenv("NH_QUEUE_PASSWORD"), StreamPrefix: "nh-durable-test-" + strings.ReplaceAll(job.ID, "-", "_"), ConsumerGroup: "nh-durable-test-workers"})
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
	deliveries, err := transport.Consume(ctx, "probe", "durable-test-controller", 1, time.Second)
	if err != nil || len(deliveries) != 1 {
		t.Fatalf("queue deliveries=%d err=%v", len(deliveries), err)
	}
	delivery := deliveries[0]
	resolver := persistence.ScopedClaimResolver{SQL: sqlStore, Workspace: bootstrap.WorkspaceID, WorkerID: "durable-test-controller", Duration: time.Minute}
	command, lease, err := resolver.ClaimCommand(ctx, delivery.Message)
	if err != nil {
		t.Fatal(err)
	}
	if err := transport.Ack(ctx, delivery.Stream, delivery.EntryID); err != nil {
		t.Fatal(err)
	}
	artifactRepository, err := persistence.NewSQLArtifactRepository(sqlStore, bootstrap.UserID, bootstrap.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	result := worker.Result{SchemaVersion: worker.ResultSchemaVersion, MessageID: command.MessageID, JobID: command.JobID, JobStepID: command.JobStepID, Status: "completed", OutputRefs: []worker.OutputRef{{ArtifactID: "artifact_runtime_report", Kind: "deterministic_analysis", Role: "analysis_report", SHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", SizeBytes: 0}}}
	applier := persistence.ScopedResultApplier{SQL: sqlStore, Workspace: bootstrap.WorkspaceID, WorkerID: "durable-test-controller", Artifacts: persistence.WorkerArtifactCommitter{Storage: objectStore, Repository: artifactRepository}}
	claimed := execution.ClaimedResult{Delivery: queue.ResultDelivery{Stream: "results", EntryID: "1-0", Attempt: lease.Attempt, Result: result}, Claim: execution.ClaimedCommand{Message: delivery.Message, Command: command, Lease: lease}}
	if err := applier.ApplyClaimedResult(ctx, claimed); err != nil {
		t.Fatal(err)
	}
	var stepStatus string
	if err := db.QueryRowContext(ctx, `SELECT s.status FROM job_steps s WHERE s.id=$1::uuid`, delivery.Message.JobStepID).Scan(&stepStatus); err != nil {
		t.Fatal(err)
	}
	if stepStatus != "completed" {
		t.Fatalf("step status=%s", stepStatus)
	}
	var committedCount, eventCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM artifacts WHERE project_id=$1::uuid AND status='committed' AND role='analysis_report'`, project.ID).Scan(&committedCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM job_events WHERE job_id=$1::uuid AND event_type='node.completed'`, job.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if committedCount != 1 || eventCount != 1 {
		t.Fatalf("committed artifacts=%d completion events=%d", committedCount, eventCount)
	}
}

func cleanupDurableRuntime(t *testing.T, db *sql.DB, objectStore storage.StoragePort, projectID, jobID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, `SELECT storage_backend,object_key,COALESCE(object_version,'') FROM artifacts WHERE project_id=$1::uuid`, projectID)
	if err == nil {
		for rows.Next() {
			var backend, key, version string
			if scanErr := rows.Scan(&backend, &key, &version); scanErr == nil {
				_ = objectStore.Delete(ctx, storage.StorageLocator{Backend: backend, ObjectKey: key, ObjectVersion: version}, storage.Preconditions{})
			}
		}
		_ = rows.Close()
	}
	// Job events are append-only by design, so a live acceptance fixture is
	// retained as audit history while its Project/Artifacts are soft-deleted.
	if _, err := db.ExecContext(ctx, `UPDATE artifacts SET status='deleted',updated_at=now() WHERE project_id=$1::uuid AND status <> 'deleted'`, projectID); err != nil {
		t.Logf("soft-delete test Artifacts failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE projects SET status='deleted',deleted_at=COALESCE(deleted_at,now()),updated_at=now() WHERE id=$1::uuid`, projectID); err != nil {
		t.Logf("soft-delete test Project failed: %v", err)
	}
	_ = jobID
}
