package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/media-worker/internal/executor"
	"github.com/nhathao-nguyen/NH-Media/services/media-worker/internal/runtime"
)

type compatibilityExecutor struct{}

func (compatibilityExecutor) Execute(_ context.Context, command worker.Command) (worker.Result, error) {
	// Stdio remains a no-storage contract probe. Production Redis execution is
	// always wired to the native Gate G executor below.
	emptySHA := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	return worker.Result{
		SchemaVersion: worker.ResultSchemaVersion, MessageID: command.MessageID,
		JobID: command.JobID, JobStepID: command.JobStepID, Status: "completed",
		OutputRefs: []worker.OutputRef{{ArtifactID: "artifact_" + command.MessageID, Kind: "deterministic_media_report", Role: "media_report", SHA256: emptySHA, SizeBytes: 0}},
	}, nil
}

func runStdio(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 1024), 1<<20)
	encoder := json.NewEncoder(output)
	executor := compatibilityExecutor{}
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		command, err := worker.DecodeCommand([]byte(scanner.Text()))
		if err != nil {
			return err
		}
		result, err := executor.Execute(context.Background(), command)
		if err != nil {
			return err
		}
		if err := worker.ValidateResult(result); err != nil {
			return err
		}
		if err := encoder.Encode(result); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func main() {
	if os.Getenv("NH_MEDIA_REDIS_WORKER") == "1" {
		native, err := executor.New(executor.Config{
			APIURL:      firstNonEmpty(os.Getenv("NH_MEDIA_WORKER_API_URL"), "http://127.0.0.1:8080"),
			WorkerToken: os.Getenv("NH_MEDIA_WORKER_TOKEN"),
			LocalRoot:   os.Getenv("NH_STORAGE_LOCAL_ROOT"),
			ScratchRoot: firstNonEmpty(os.Getenv("NH_MEDIA_WORKER_SCRATCH_ROOT"), filepath.Join(os.TempDir(), "nh-media-worker")),
			FFmpegPath:  os.Getenv("NH_MEDIA_FFMPEG_PATH"),
			FFprobePath: os.Getenv("NH_MEDIA_FFPROBE_PATH"),
		})
		if err != nil {
			log.Fatal(err)
		}
		capabilities := splitCapabilities(firstNonEmpty(os.Getenv("NH_MEDIA_QUEUE_CAPABILITIES"), os.Getenv("NH_MEDIA_QUEUE_CAPABILITY"), "probe,thumbnail,media,render"))
		if len(capabilities) == 0 {
			log.Fatal("at least one media worker capability is required")
		}
		errors := make(chan error, len(capabilities))
		for _, capability := range capabilities {
			controller, controllerErr := runtime.NewRedisController(runtime.RedisOptions{
				Addr: firstNonEmpty(os.Getenv("NH_MEDIA_WORKER_QUEUE_ENDPOINT"), os.Getenv("NH_QUEUE_ENDPOINT")), Password: firstNonEmpty(os.Getenv("NH_QUEUE_PASSWORD"), os.Getenv("REDIS_PASSWORD")),
				StreamPrefix:  firstNonEmpty(os.Getenv("NH_QUEUE_STREAM_PREFIX"), "nh-media"),
				ConsumerGroup: firstNonEmpty(os.Getenv("NH_QUEUE_CONSUMER_GROUP"), "nh-media-workers"),
				Consumer:      firstNonEmpty(os.Getenv("NH_MEDIA_WORKER_CONSUMER"), "media-worker-1") + "-" + capability,
				Capability:    capability,
			}, native)
			if controllerErr != nil {
				log.Fatal(controllerErr)
			}
			defer controller.Close()
			go func() { errors <- controller.Run(context.Background()) }()
		}
		if err := <-errors; err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := runStdio(os.Stdin, os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func splitCapabilities(value string) []string {
	seen := map[string]bool{}
	result := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
