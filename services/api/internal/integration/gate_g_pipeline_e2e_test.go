package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/artifact"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/persistence"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/product"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

func TestGateGMovieRecapProductAPIThroughRedisWorkersToRenderArtifact(t *testing.T) {
	f := newGateEDurableFixture(t)
	ffmpeg := strings.TrimSpace(os.Getenv("NH_MEDIA_FFMPEG_PATH"))
	ffprobe := strings.TrimSpace(os.Getenv("NH_MEDIA_FFPROBE_PATH"))
	if ffmpeg == "" || ffprobe == "" {
		t.Fatal("Gate G pipeline E2E requires reviewed NH_MEDIA_FFMPEG_PATH and NH_MEDIA_FFPROBE_PATH")
	}
	repoRoot := repositoryRoot(t)
	token := f.login(t)
	projectStatus, projectBody := f.request(t, http.MethodPost, "/api/v1/projects", token, map[string]any{
		"name": "gate-g-movie-recap", "workflow_key": "movie_recap",
	}, map[string]string{"Idempotency-Key": "gate-g-project-" + gateESuffix()})
	if projectStatus != http.StatusCreated {
		t.Fatalf("create Project status=%d body=%s", projectStatus, projectBody)
	}
	var projectValue product.Project
	if err := json.Unmarshal(projectBody, &projectValue); err != nil || projectValue.ID == "" {
		t.Fatalf("decode Project: value=%+v err=%v", projectValue, err)
	}
	defer f.closeProject(t, projectValue.ID, "")

	fixturePath := filepath.Join(t.TempDir(), "gate-g-source.mp4")
	createGateGMediaFixture(t, ffmpeg, fixturePath)
	source := commitGateGSource(t, f, projectValue.ID, fixturePath)
	assertGateGWorkerResolveBoundary(t, f, projectValue.ID, source)

	stopControllers := startGateGControllers(t, f)
	defer stopControllers()
	stopWorkers, workerLogs := startGateGWorkers(t, f, repoRoot, ffmpeg, ffprobe)
	defer stopWorkers()

	jobStatus, jobBody := f.request(t, http.MethodPost, "/api/v1/projects/"+projectValue.ID+"/jobs", token, map[string]any{
		"kind": "pipeline", "workflow_key": "movie_recap", "mode": "automatic", "auto_start": true,
		"input": map[string]any{"artifacts": []map[string]any{{
			"artifact_id": contractArtifactID(source.ID), "role": "source_original", "sha256": source.SHA256,
		}}},
		"params": map[string]any{"duration_sec": 3, "language": "en"},
	}, map[string]string{"Idempotency-Key": "gate-g-job-" + gateESuffix()})
	if jobStatus != http.StatusAccepted {
		t.Fatalf("create/start movie_recap Job status=%d body=%s", jobStatus, jobBody)
	}
	var job product.Job
	if err := json.Unmarshal(jobBody, &job); err != nil || job.ID == "" {
		t.Fatalf("decode movie_recap Job: value=%+v err=%v", job, err)
	}

	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		status, body := f.request(t, http.MethodGet, "/api/v1/projects/"+projectValue.ID+"/jobs/"+job.ID, token, nil, nil)
		if status != http.StatusOK {
			t.Fatalf("poll movie_recap Job status=%d body=%s", status, body)
		}
		if err := json.Unmarshal(body, &job); err != nil {
			t.Fatal(err)
		}
		if job.Status == domain.JobCompleted {
			break
		}
		if job.Status == domain.JobFailed || job.Status == domain.JobDeadLettered || job.Status == domain.JobCancelled {
			t.Fatalf("movie_recap Job terminated as %s; steps=%s workers=%s", job.Status, gateGStepDiagnostics(t, f, job.ID), workerLogs())
		}
		time.Sleep(250 * time.Millisecond)
	}
	if job.Status != domain.JobCompleted {
		t.Fatalf("movie_recap Job did not complete; status=%s steps=%s workers=%s", job.Status, gateGStepDiagnostics(t, f, job.ID), workerLogs())
	}

	assertGateGStepsCompleted(t, f, job.ID)
	assertGateGArtifacts(t, f, projectValue.ID, job.ID, ffprobe)
}

func assertGateGWorkerResolveBoundary(t *testing.T, f *gateEDurableFixture, projectID string, source artifact.Artifact) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"project_id": contractProjectID(projectID), "artifact_id": contractArtifactID(source.ID), "sha256": source.SHA256,
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, f.server.URL+"/internal/v1/worker/artifacts/resolve", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+f.workerToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := f.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("worker source resolve boundary status=%d body=%s project=%s artifact=%s", response.StatusCode, responseBody, contractProjectID(projectID), contractArtifactID(source.ID))
	}
}

func startGateGControllers(t *testing.T, f *gateEDurableFixture) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	claims := execution.NewLeaseRegistry()
	repository, err := persistence.NewSQLArtifactRepository(f.store, f.userID, f.workspaceID)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	for _, capability := range []string{"probe", "ai", "ml", "media", "render", "system"} {
		workerID := "gate-g-api-" + capability
		dispatcher := execution.ClaimingDispatcher{
			Queue: f.queue, Resolver: persistence.ScopedClaimResolver{SQL: f.store, Workspace: f.workspaceID, WorkerID: workerID, Duration: 2 * time.Minute},
			Publisher: f.queue, Claims: claims,
		}
		reconciler := execution.LeaseAwareResultReconciler{
			Source: f.queue, Claims: claims,
			Applier: persistence.ScopedResultApplier{SQL: f.store, Workspace: f.workspaceID, WorkerID: workerID, Artifacts: persistence.WorkerArtifactCommitter{Storage: f.objectStore, Repository: repository}},
		}
		go runGateGDispatcher(ctx, capability, dispatcher)
		go runGateGReconciler(ctx, capability, reconciler)
	}
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for ctx.Err() == nil {
			_, _ = f.backend.ScheduleReadySteps(ctx)
			_, _ = f.backend.RequeueRetryingSteps(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return cancel
}

func runGateGDispatcher(ctx context.Context, capability string, dispatcher execution.ClaimingDispatcher) {
	for ctx.Err() == nil {
		_ = dispatcher.Run(ctx, capability, "gate-g-dispatch-"+capability, 100*time.Millisecond)
	}
}

func runGateGReconciler(ctx context.Context, capability string, reconciler execution.LeaseAwareResultReconciler) {
	for ctx.Err() == nil {
		_ = reconciler.Run(ctx, capability, "gate-g-result-"+capability, 100*time.Millisecond)
	}
}

func startGateGWorkers(t *testing.T, f *gateEDurableFixture, repoRoot, ffmpeg, ffprobe string) (func(), func() string) {
	t.Helper()
	root := t.TempDir()
	mediaBinary := filepath.Join(root, "nh-media-worker")
	if runtime.GOOS == "windows" {
		mediaBinary += ".exe"
	}
	build := exec.Command("go", "build", "-o", mediaBinary, "./services/media-worker/cmd/worker")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build media worker: %v: %s", err, output)
	}
	python := filepath.Join(repoRoot, "services", "ml-worker", ".venv", "Scripts", "python.exe")
	if runtime.GOOS != "windows" {
		python = filepath.Join(repoRoot, "services", "ml-worker", ".venv", "bin", "python")
	}
	if _, err := os.Stat(python); err != nil {
		uv, lookErr := exec.LookPath("uv")
		if lookErr != nil {
			t.Fatal("uv is required to prepare the Gate G Python worker E2E")
		}
		syncCommand := exec.Command(uv, "sync", "--frozen", "--project", "services/ml-worker")
		syncCommand.Dir = repoRoot
		if output, syncErr := syncCommand.CombinedOutput(); syncErr != nil {
			t.Fatalf("prepare Python worker: %v: %s", syncErr, output)
		}
	}
	redisAddress := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_REDIS_ADDR"))
	common := map[string]string{
		"NH_MEDIA_REDIS_WORKER": "1", "NH_MEDIA_WORKER_API_URL": f.server.URL,
		"NH_MEDIA_WORKER_TOKEN": f.workerToken, "NH_QUEUE_STREAM_PREFIX": f.queuePrefix,
		"NH_QUEUE_PASSWORD": os.Getenv("NH_QUEUE_PASSWORD"), "NH_MEDIA_FFMPEG_PATH": ffmpeg,
		"NH_MEDIA_FFPROBE_PATH": ffprobe,
	}
	mediaEnv := copyEnvironment(common)
	mediaEnv = setEnvironment(mediaEnv, "NH_MEDIA_WORKER_QUEUE_ENDPOINT", redisAddress)
	mediaEnv = setEnvironment(mediaEnv, "NH_MEDIA_QUEUE_CAPABILITIES", "probe,media,render")
	mediaEnv = setEnvironment(mediaEnv, "NH_MEDIA_WORKER_SCRATCH_ROOT", filepath.Join(root, "media-scratch"))
	mlEnv := copyEnvironment(common)
	mlEnv = setEnvironment(mlEnv, "NH_QUEUE_ENDPOINT", "redis://"+redisAddress)
	mlEnv = setEnvironment(mlEnv, "NH_MEDIA_QUEUE_CAPABILITIES", "ai,ml,system")

	mediaLog, err := os.Create(filepath.Join(root, "media-worker.log"))
	if err != nil {
		t.Fatal(err)
	}
	mlLog, err := os.Create(filepath.Join(root, "ml-worker.log"))
	if err != nil {
		_ = mediaLog.Close()
		t.Fatal(err)
	}
	media := exec.Command(mediaBinary)
	media.Dir, media.Env, media.Stdout, media.Stderr = repoRoot, mediaEnv, mediaLog, mediaLog
	ml := exec.Command(python, "-m", "nh_media.worker")
	ml.Dir, ml.Env, ml.Stdout, ml.Stderr = repoRoot, mlEnv, mlLog, mlLog
	if err := media.Start(); err != nil {
		t.Fatal(err)
	}
	if err := ml.Start(); err != nil {
		_ = media.Process.Kill()
		_, _ = media.Process.Wait()
		t.Fatal(err)
	}
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		_ = media.Process.Kill()
		_ = ml.Process.Kill()
		_, _ = media.Process.Wait()
		_, _ = ml.Process.Wait()
		_ = mediaLog.Close()
		_ = mlLog.Close()
	}
	logs := func() string {
		_ = mediaLog.Sync()
		_ = mlLog.Sync()
		mediaBody, _ := os.ReadFile(mediaLog.Name())
		mlBody, _ := os.ReadFile(mlLog.Name())
		return boundedText("media="+string(mediaBody)+" ml="+string(mlBody), 4000)
	}
	t.Cleanup(stop)
	return stop, logs
}

func createGateGMediaFixture(t *testing.T, ffmpeg, output string) {
	t.Helper()
	command := exec.Command(ffmpeg,
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=c=black:s=320x180:r=30:d=1",
		"-f", "lavfi", "-i", "color=c=white:s=320x180:r=30:d=1",
		"-f", "lavfi", "-i", "color=c=red:s=320x180:r=30:d=1",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3",
		"-filter_complex", "[0:v][1:v][2:v]concat=n=3:v=1:a=0[v]", "-map", "[v]", "-map", "3:a",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", output,
	)
	if body, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create real Gate G source fixture: %v: %s", err, body)
	}
}

func commitGateGSource(t *testing.T, f *gateEDurableFixture, projectID, path string) artifact.Artifact {
	t.Helper()
	input, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	staged, err := f.objectStore.PutStaged(ctx, storage.Scope{WorkspaceID: f.workspaceID, ProjectID: projectID}, input, "video/mp4")
	if err != nil {
		t.Fatal(err)
	}
	repository, err := persistence.NewSQLArtifactRepository(f.store, f.userID, f.workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	value, err := (artifact.CommitService{Storage: f.objectStore, Repository: repository}).Commit(ctx, artifact.CommitRequest{
		ProjectID: projectID, Kind: "video", Role: "source_original",
		FinalKey:       "workspaces/" + f.workspaceID + "/projects/" + projectID + "/artifacts/gate-g-source.mp4",
		ExpectedSHA256: staged.SHA256, Staged: artifact.NewStaged(staged), ContentType: "video/mp4", IfNoneMatch: true,
		Metadata: map[string]any{"fixture": "gate_g_e2e", "independent": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func assertGateGStepsCompleted(t *testing.T, f *gateEDurableFixture, jobID string) {
	t.Helper()
	rows, err := f.db.Query(`SELECT s.node_key,s.status FROM job_steps s JOIN pipeline_runs r ON r.id=s.pipeline_run_id WHERE r.job_id=$1::uuid ORDER BY s.node_key`, jobID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var node, status string
		if err := rows.Scan(&node, &status); err != nil {
			t.Fatal(err)
		}
		count++
		if status != "completed" {
			t.Errorf("Gate G node %s status=%s", node, status)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 25 {
		t.Fatalf("movie_recap executed %d JobSteps, want 25", count)
	}
}

func assertGateGArtifacts(t *testing.T, f *gateEDurableFixture, projectID, jobID, ffprobe string) {
	t.Helper()
	timeline := readGateGArtifact(t, f, projectID, jobID, "timeline_version")
	if _, err := domain.ValidateTimeline(timeline, domain.TimelineValidationOptions{StructuralOnly: true}); err != nil {
		t.Fatalf("worker-produced canonical Timeline is invalid: %v", err)
	}
	var document struct {
		ProjectID string `json:"project_id"`
		Tracks    []struct {
			Kind  string `json:"kind"`
			Clips []any  `json:"clips"`
		} `json:"tracks"`
	}
	if err := json.Unmarshal(timeline, &document); err != nil {
		t.Fatal(err)
	}
	if document.ProjectID != projectID {
		t.Fatalf("worker-produced Timeline project_id=%s want %s", document.ProjectID, projectID)
	}
	videoClips := 0
	for _, track := range document.Tracks {
		if track.Kind == "video" {
			videoClips += len(track.Clips)
		}
	}
	if videoClips < 3 {
		t.Fatalf("worker-produced Timeline has %d video clips, want at least 3", videoClips)
	}
	renderPath := materializeGateGArtifact(t, f, projectID, jobID, "render_video")
	probe := exec.Command(ffprobe, "-v", "error", "-show_entries", "format=duration:stream=codec_type", "-of", "json", renderPath)
	probeBody, err := probe.CombinedOutput()
	var probeReport struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
		} `json:"streams"`
	}
	if decodeErr := json.Unmarshal(probeBody, &probeReport); decodeErr != nil {
		t.Fatalf("render Artifact returned invalid ffprobe JSON: %v report=%s", decodeErr, probeBody)
	}
	hasVideo, hasAudio := false, false
	for _, stream := range probeReport.Streams {
		hasVideo = hasVideo || stream.CodecType == "video"
		hasAudio = hasAudio || stream.CodecType == "audio"
	}
	if err != nil || !hasVideo || !hasAudio {
		t.Fatalf("render Artifact failed ffprobe: err=%v report=%s", err, probeBody)
	}
	for _, role := range []string{"mixed_audio", "timeline_qa_report", "render_metadata", "deliverable_qa", "clip_exports", "clip_manifest"} {
		body := readGateGArtifact(t, f, projectID, jobID, role)
		if len(body) == 0 {
			t.Fatalf("Gate G Artifact role %s is empty", role)
		}
	}
}

func readGateGArtifact(t *testing.T, f *gateEDurableFixture, projectID, jobID, role string) []byte {
	t.Helper()
	path := materializeGateGArtifact(t, f, projectID, jobID, role)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func materializeGateGArtifact(t *testing.T, f *gateEDurableFixture, projectID, jobID, role string) string {
	t.Helper()
	var locator storage.StorageLocator
	var size int64
	err := f.db.QueryRow(`SELECT storage_backend,object_key,COALESCE(object_version,''),size_bytes FROM artifacts WHERE project_id=$1::uuid AND producer_job_id=$2::uuid AND role=$3 AND status='committed'`, projectID, jobID, role).Scan(&locator.Backend, &locator.ObjectKey, &locator.ObjectVersion, &size)
	if err != nil {
		t.Fatalf("find committed Gate G Artifact role %s: %v", role, err)
	}
	if size <= 0 {
		t.Fatalf("committed Gate G Artifact role %s has invalid size %d", role, size)
	}
	handle, err := f.objectStore.Materialize(context.Background(), locator, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return handle.Path()
}

func gateGStepDiagnostics(t *testing.T, f *gateEDurableFixture, jobID string) string {
	t.Helper()
	rows, err := f.db.Query(`SELECT s.node_key,s.status,COALESCE(a.error_code,''),COALESCE(a.error_message,'') FROM job_steps s JOIN pipeline_runs r ON r.id=s.pipeline_run_id LEFT JOIN job_step_attempts a ON a.job_step_id=s.id AND a.attempt=s.current_attempt WHERE r.job_id=$1::uuid ORDER BY s.node_key`, jobID)
	if err != nil {
		return err.Error()
	}
	defer rows.Close()
	values := make([]string, 0)
	for rows.Next() {
		var node, status, code, message string
		if rows.Scan(&node, &status, &code, &message) == nil {
			values = append(values, fmt.Sprintf("%s=%s/%s/%s", node, status, code, message))
		}
	}
	return strings.Join(values, ",")
}

func contractArtifactID(value string) string {
	return "artifact_" + strings.ReplaceAll(value, "-", "_")
}

func contractProjectID(value string) string {
	return "project_" + strings.ReplaceAll(value, "-", "_")
}

func copyEnvironment(values map[string]string) []string {
	result := append([]string{}, os.Environ()...)
	for key, value := range values {
		result = setEnvironment(result, key, value)
	}
	return result
}

func setEnvironment(values []string, key, value string) []string {
	prefix := strings.ToUpper(key) + "="
	result := make([]string, 0, len(values)+1)
	for _, item := range values {
		if !strings.HasPrefix(strings.ToUpper(item), prefix) {
			result = append(result, item)
		}
	}
	return append(result, key+"="+value)
}

func boundedText(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[len(value)-limit:]
}
