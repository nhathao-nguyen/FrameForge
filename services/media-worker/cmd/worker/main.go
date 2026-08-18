package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/media-worker/internal/runtime"
)

type deterministicExecutor struct{}

func (deterministicExecutor) Execute(_ context.Context, command worker.Command) (worker.Result, error) {
	// Gate F keeps real providers out of conformance. This bounded worker hop
	// proves a Go-owned, versioned result path without putting bytes on Redis.
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
	executor := deterministicExecutor{}
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
		controller, err := runtime.NewRedisController(runtime.RedisOptions{
			Addr: firstNonEmpty(os.Getenv("NH_MEDIA_WORKER_QUEUE_ENDPOINT"), os.Getenv("NH_QUEUE_ENDPOINT")), Password: firstNonEmpty(os.Getenv("NH_QUEUE_PASSWORD"), os.Getenv("REDIS_PASSWORD")),
			StreamPrefix:  firstNonEmpty(os.Getenv("NH_QUEUE_STREAM_PREFIX"), "nh-media"),
			ConsumerGroup: firstNonEmpty(os.Getenv("NH_QUEUE_CONSUMER_GROUP"), "nh-media-workers"),
			Consumer:      firstNonEmpty(os.Getenv("NH_MEDIA_WORKER_CONSUMER"), "media-worker-1"),
			Capability:    firstNonEmpty(os.Getenv("NH_MEDIA_QUEUE_CAPABILITY"), "probe"),
		}, deterministicExecutor{})
		if err != nil {
			log.Fatal(err)
		}
		defer controller.Close()
		if err := controller.Run(context.Background()); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := runStdio(os.Stdin, os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
