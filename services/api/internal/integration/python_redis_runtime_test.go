package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/queue"
)

type integrationResolver struct{ command worker.Command }

func (r integrationResolver) ResolveCommand(context.Context, queue.Message) (worker.Command, error) {
	return r.command, nil
}

func TestPythonWorkerRedisRuntime(t *testing.T) {
	address := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_REDIS_ADDR"))
	if address == "" {
		t.Skip("set NH_MEDIA_TEST_REDIS_ADDR to run the live Redis-to-Python worker harness")
	}
	root := repositoryRoot(t)
	prefix := "nh-test-" + strings.ReplaceAll(time.Now().UTC().Format("150405.000"), ".", "-")
	transport, err := queue.NewRedisQueue(queue.RedisOptions{Addr: address, Password: os.Getenv("NH_QUEUE_PASSWORD"), StreamPrefix: prefix, ConsumerGroup: "nh-test-workers"})
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := transport.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	workerCommand := worker.Command{SchemaVersion: worker.CommandSchemaVersion, MessageID: "msg_analysis_redis_001", Capability: "analysis", ProjectID: "project_analysis_001", JobID: "job_analysis_001", PipelineRunID: "run_analysis_001", JobStepID: "step_analysis_001", Attempt: 1, InputRefs: []worker.ArtifactRef{}, Config: map[string]any{"mode": "deterministic"}}
	if _, err := transport.Enqueue(ctx, queue.Message{MessageID: workerCommand.MessageID, Capability: "analysis", JobID: workerCommand.JobID, PipelineRunID: workerCommand.PipelineRunID, JobStepID: workerCommand.JobStepID, Attempt: 1}); err != nil {
		t.Fatal(err)
	}

	python := exec.CommandContext(ctx, "uv", "run", "--project", filepath.Join(root, "services", "ml-worker"), "python", "-m", "nh_media.worker")
	python.Dir = root
	python.Stdout = os.Stdout
	python.Stderr = os.Stderr
	endpoint := address
	if !strings.HasPrefix(endpoint, "redis://") && !strings.HasPrefix(endpoint, "rediss://") {
		endpoint = "redis://" + endpoint
	}
	python.Env = append(os.Environ(), "NH_MEDIA_REDIS_WORKER=1", "NH_QUEUE_ENDPOINT="+endpoint, "NH_QUEUE_STREAM_PREFIX="+prefix, "NH_MEDIA_QUEUE_CAPABILITY=analysis", "NH_QUEUE_CONSUMER_GROUP=nh-test-workers", "NH_MEDIA_WORKER_CONSUMER=python-test-1")
	if err := python.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if python.Process != nil {
			stopProcessTree(python)
		}
	}()

	dispatcher := execution.Dispatcher{Queue: transport, Resolver: integrationResolver{command: workerCommand}, Publisher: transport}
	for attempt := 0; attempt < 20; attempt++ {
		if processed, dispatchErr := dispatcher.DispatchOnce(ctx, "analysis", "api-test-1", 1); dispatchErr != nil {
			t.Fatal(dispatchErr)
		} else if processed == 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	for attempt := 0; attempt < 40; attempt++ {
		values, consumeErr := transport.ConsumeWorkerResults(ctx, "analysis", "api-test-result-1", 1, 250*time.Millisecond)
		if consumeErr != nil {
			t.Fatal(consumeErr)
		}
		if len(values) == 1 {
			result := values[0].Result
			if result.Status != "completed" || len(result.OutputRefs) != 1 || result.OutputRefs[0].Role != "analysis_report" {
				t.Fatalf("unexpected worker result: %+v", result)
			}
			if err := transport.AckWorkerResult(ctx, values[0].Stream, values[0].EntryID); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("Python worker did not publish a result")
}

func TestPythonWorkerRedisRestartReclaimsPendingEntry(t *testing.T) {
	address := strings.TrimSpace(os.Getenv("NH_MEDIA_TEST_REDIS_ADDR"))
	if address == "" {
		t.Skip("set NH_MEDIA_TEST_REDIS_ADDR to run the live Redis worker restart harness")
	}
	root := repositoryRoot(t)
	prefix := "nh-test-restart-" + strings.ReplaceAll(time.Now().UTC().Format("150405.000"), ".", "-")
	transport, err := queue.NewRedisQueue(queue.RedisOptions{Addr: address, Password: os.Getenv("NH_QUEUE_PASSWORD"), StreamPrefix: prefix, ConsumerGroup: "nh-test-restart-workers"})
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := transport.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	command := worker.Command{SchemaVersion: worker.CommandSchemaVersion, MessageID: "msg_analysis_restart_001", Capability: "analysis", ProjectID: "project_analysis_001", JobID: "job_analysis_001", PipelineRunID: "run_analysis_001", JobStepID: "step_analysis_001", Attempt: 1, InputRefs: []worker.ArtifactRef{}, Config: map[string]any{"mode": "deterministic", "test_delay_ms": 1500}}
	if _, err := transport.Enqueue(ctx, queue.Message{MessageID: command.MessageID, Capability: command.Capability, JobID: command.JobID, PipelineRunID: command.PipelineRunID, JobStepID: command.JobStepID, Attempt: command.Attempt}); err != nil {
		t.Fatal(err)
	}
	start := func(consumer string) *exec.Cmd {
		endpoint := address
		if !strings.HasPrefix(endpoint, "redis://") && !strings.HasPrefix(endpoint, "rediss://") {
			endpoint = "redis://" + endpoint
		}
		process := exec.CommandContext(ctx, "uv", "run", "--project", filepath.Join(root, "services", "ml-worker"), "python", "-m", "nh_media.worker")
		process.Dir = root
		process.Stdout = os.Stdout
		process.Stderr = os.Stderr
		process.Env = append(os.Environ(), "NH_MEDIA_REDIS_WORKER=1", "NH_QUEUE_ENDPOINT="+endpoint, "NH_QUEUE_PASSWORD="+os.Getenv("NH_QUEUE_PASSWORD"), "NH_QUEUE_STREAM_PREFIX="+prefix, "NH_MEDIA_QUEUE_CAPABILITY=analysis", "NH_QUEUE_CONSUMER_GROUP=nh-test-restart-workers", "NH_MEDIA_WORKER_CONSUMER="+consumer, "NH_MEDIA_RECLAIM_IDLE_MS=200")
		if err := process.Start(); err != nil {
			t.Fatal(err)
		}
		return process
	}
	first := start("python-restart-1")
	if processed, err := (execution.Dispatcher{Queue: transport, Resolver: integrationResolver{command: command}, Publisher: transport}).DispatchOnce(ctx, "analysis", "api-restart-controller", 1); err != nil || processed != 1 {
		t.Fatalf("dispatch processed=%d err=%v", processed, err)
	}
	time.Sleep(500 * time.Millisecond)
	stopProcessTree(first)
	second := start("python-restart-2")
	defer func() { stopProcessTree(second) }()
	for attempt := 0; attempt < 40; attempt++ {
		values, err := transport.ConsumeWorkerResults(ctx, "analysis", "api-restart-result", 1, 250*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		if len(values) == 1 {
			if values[0].Result.Status != "completed" {
				t.Fatalf("unexpected restart result: %+v", values[0].Result)
			}
			if err := transport.AckWorkerResult(ctx, values[0].Stream, values[0].EntryID); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("restarted Python worker did not reclaim the pending command")
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

func stopProcessTree(command *exec.Cmd) {
	if command == nil || command.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(command.Process.Pid), "/T", "/F").Run()
	} else {
		_ = command.Process.Kill()
	}
	_ = command.Wait()
}
