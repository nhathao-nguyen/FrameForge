package integration

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/auth"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/persistence"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/product"
)

func TestDurableExecutionCommandSurface(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("set NH_MEDIA_TEST_DATABASE_URL for the durable execution command surface harness")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := persistence.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sqlStore, err := persistence.NewSQLStore(db)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap, err := sqlStore.BootstrapLocalInstallation(ctx, persistence.BootstrapInput{Username: "runtime-command-admin", DisplayName: "Runtime Command Admin", PasswordHash: auth.CredentialHash("runtime-command-password"), WorkspaceSlug: "runtime-command", WorkspaceName: "Runtime Command"})
	if err != nil {
		t.Fatal(err)
	}
	workflowKey := "runtime_command_" + time.Now().UTC().Format("150405.000")
	if _, err := sqlStore.EnsurePipelineDefinition(ctx, persistence.PipelineDefinitionInput{
		WorkflowKey: workflowKey, WorkflowName: "Runtime command", Description: "durable command surface", SchemaVersion: "1.0", Definition: []byte(`{"kind":"runtime_command","nodes":["analysis"]}`), Version: 1, Status: "active", CreatedBy: bootstrap.UserID,
		Nodes: []persistence.PipelineNodeInput{{NodeKey: "analysis", NodeType: "analysis", DisplayName: "Reviewable analysis", ExecutionClass: "ml", FailureMode: "hard", ReviewPolicy: "approval_completes_node", Config: []byte(`{"mode":"deterministic"}`), TimeoutSec: 60, MaxAttempts: 2, RetryPolicy: []byte(`{"base_delay_ms":50,"max_delay_ms":500}`), CheckpointPolicy: []byte(`{"enabled":true}`), ResourceRequirements: []byte(`{"network":"none"}`), IdempotencyPolicy: []byte(`{"key":"message_id"}`), ProgressWeight: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	project, err := sqlStore.CreateProject(ctx, bootstrap.UserID, bootstrap.WorkspaceID, "command-"+time.Now().UTC().Format("150405.000"), workflowKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `UPDATE projects SET status='deleted',updated_at=now() WHERE id=$1::uuid`, project.ID)
	})
	backend, err := persistence.NewDurableBackend(sqlStore, bootstrap.UserID, bootstrap.WorkspaceID, nil)
	if err != nil {
		t.Fatal(err)
	}
	setRunning := func(jobID, runID, stepID string) error {
		if _, err := db.ExecContext(ctx, `UPDATE jobs SET status='running' WHERE id=$1::uuid`, jobID); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `UPDATE pipeline_runs SET status='running' WHERE id=$1::uuid`, runID); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx, `UPDATE job_steps SET status='running' WHERE id=$1::uuid`, stepID)
		return err
	}
	job, err := backend.CreateExecutionJob(bootstrap.WorkspaceID, project.ID, product.JobCreateInput{Kind: "analysis", Mode: "studio"})
	if err != nil {
		t.Fatal(err)
	}
	document := json.RawMessage(`{"schema_version":"1.0","timeline_id":"placeholder","timeline_version_id":"placeholder","project_id":"` + project.ID + `","version":1,"duration_sec":1,"tracks":[{"id":"track","kind":"video","name":"Footage","order":0,"clips":[{"id":"clip","timeline_in_sec":0,"timeline_out_sec":1,"source":{"type":"none","inline_id":"silence"},"origin":"user"}]}]}`)
	timeline, versionOne, err := backend.CreateTimeline(bootstrap.WorkspaceID, project.ID, "user", document)
	if err != nil {
		t.Fatal(err)
	}
	versionTwo, err := backend.AddTimelineVersion(bootstrap.WorkspaceID, project.ID, timeline.ID, versionOne.ID, "user", versionOne.Document, 1)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := backend.ListPipelineRuns(bootstrap.WorkspaceID, project.ID, job.ID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs=%d err=%v", len(runs), err)
	}
	steps, err := backend.ListJobSteps(bootstrap.WorkspaceID, project.ID, job.ID, runs[0].ID)
	if err != nil || len(steps) != 1 {
		t.Fatalf("steps=%d err=%v", len(steps), err)
	}
	if err := setRunning(job.ID, runs[0].ID, steps[0].ID); err != nil {
		t.Fatal(err)
	}
	opened, err := backend.OpenReview(bootstrap.WorkspaceID, project.ID, job.ID, steps[0].ID, "timeline", "timeline_version", versionTwo.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Status != "open" {
		t.Fatalf("review status=%s", opened.Status)
	}
	if _, err := backend.ApproveReview(bootstrap.WorkspaceID, project.ID, job.ID, steps[0].ID, versionOne.ID, 1); err == nil {
		t.Fatal("stale review resource revision was accepted")
	}
	approved, err := backend.ApproveReview(bootstrap.WorkspaceID, project.ID, job.ID, steps[0].ID, versionTwo.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != "queued" {
		t.Fatalf("approved job status=%s", approved.Status)
	}
	var runStatus, stepStatus string
	if err := db.QueryRowContext(ctx, `SELECT r.status,s.status FROM pipeline_runs r JOIN job_steps s ON s.pipeline_run_id=r.id WHERE r.id=$1::uuid AND s.id=$2::uuid`, runs[0].ID, steps[0].ID).Scan(&runStatus, &stepStatus); err != nil {
		t.Fatal(err)
	}
	if runStatus != "queued" || stepStatus != "completed" {
		t.Fatalf("approved graph run=%s step=%s", runStatus, stepStatus)
	}

	job2, err := backend.CreateExecutionJob(bootstrap.WorkspaceID, project.ID, product.JobCreateInput{Kind: "analysis", Mode: "studio"})
	if err != nil {
		t.Fatal(err)
	}
	runs2, _ := backend.ListPipelineRuns(bootstrap.WorkspaceID, project.ID, job2.ID)
	steps2, _ := backend.ListJobSteps(bootstrap.WorkspaceID, project.ID, job2.ID, runs2[0].ID)
	if err := setRunning(job2.ID, runs2[0].ID, steps2[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.OpenReview(bootstrap.WorkspaceID, project.ID, job2.ID, steps2[0].ID, "timeline", "timeline_version", versionOne.ID, 1); err != nil {
		t.Fatal(err)
	}
	paused, err := backend.RejectReview(bootstrap.WorkspaceID, project.ID, job2.ID, steps2[0].ID, "edit_then_resume")
	if err != nil || paused.Status != "paused" {
		t.Fatalf("rejected review job=%v status=%v", err, paused)
	}
	resumed, err := backend.ResumeJob(bootstrap.WorkspaceID, project.ID, job2.ID)
	if err != nil || resumed.Status != "queued" {
		t.Fatalf("resumed job=%v status=%v", err, resumed)
	}
	cancelled, err := backend.CancelJob(bootstrap.WorkspaceID, project.ID, job2.ID)
	if err != nil || cancelled.Status != "cancelled" {
		t.Fatalf("cancelled job=%v status=%v", err, cancelled)
	}
	if err := db.QueryRowContext(ctx, `SELECT r.status,s.status FROM pipeline_runs r JOIN job_steps s ON s.pipeline_run_id=r.id WHERE r.id=$1::uuid AND s.id=$2::uuid`, runs2[0].ID, steps2[0].ID).Scan(&runStatus, &stepStatus); err != nil {
		t.Fatal(err)
	}
	if runStatus != "cancelled" || stepStatus != "cancelled" {
		t.Fatalf("cancelled graph run=%s step=%s", runStatus, stepStatus)
	}

	job3, err := backend.CreateExecutionJob(bootstrap.WorkspaceID, project.ID, product.JobCreateInput{Kind: "analysis"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE jobs SET status='failed' WHERE id=$1::uuid`, job3.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE pipeline_runs SET status='failed' WHERE job_id=$1::uuid`, job3.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE job_steps SET status='failed' WHERE pipeline_run_id=(SELECT current_pipeline_run_id FROM jobs WHERE id=$1::uuid)`, job3.ID); err != nil {
		t.Fatal(err)
	}
	replayed, err := backend.RetryExecutionJob(bootstrap.WorkspaceID, project.ID, job3.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ID == job3.ID || replayed.Command["supersedes_job_id"] != job3.ID {
		t.Fatalf("replay did not create immutable successor: old=%s new=%s command=%#v", job3.ID, replayed.ID, replayed.Command)
	}
	var eventCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM job_events WHERE job_id=$1::uuid AND event_type IN ('review.required','review.approved','review.rejected')`, job.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount < 2 {
		t.Fatalf("review events=%d", eventCount)
	}
}
