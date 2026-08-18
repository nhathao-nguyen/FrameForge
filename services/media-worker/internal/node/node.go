package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/media-worker/internal/probe"
	"github.com/nhathao-nguyen/NH-Media/services/media-worker/internal/process"
)

type MaterializedArtifact struct {
	Path       string
	ScratchDir string
	Cleanup    func()
}

type ArtifactMaterializer interface {
	Materialize(context.Context, string, worker.ArtifactRef) (MaterializedArtifact, error)
}

type ArtifactStager interface {
	Stage(context.Context, string, string, string, io.Reader, string) (worker.OutputRef, error)
}

type FileArtifactStager interface {
	StageFile(context.Context, string, string, string, string, string) (worker.OutputRef, error)
}

type MediaProcess interface {
	Run(context.Context, process.Spec) (process.Result, error)
}

type Node struct {
	Materializer ArtifactMaterializer
	Stager       ArtifactStager
	FileStager   FileArtifactStager
	Validator    probe.Validator
	Process      MediaProcess
}

func (n Node) Execute(ctx context.Context, command worker.Command) (worker.Result, error) {
	if err := worker.ValidateCommand(command); err != nil {
		return worker.Result{}, err
	}
	if n.Materializer == nil || n.Stager == nil {
		return worker.Result{}, errors.New("node materializer and Artifact stager are required")
	}
	if len(command.InputRefs) != 1 {
		return failed(command, "invalid_input_refs", "exactly one source Artifact is required"), nil
	}
	materialized, err := n.Materializer.Materialize(ctx, command.ProjectID, command.InputRefs[0])
	if err != nil {
		return failed(command, "artifact_unavailable", "the source Artifact could not be materialized"), nil
	}
	if materialized.Cleanup != nil {
		defer materialized.Cleanup()
	}
	if strings.TrimSpace(materialized.Path) == "" {
		return failed(command, "artifact_unavailable", "the source Artifact has no materialized handle"), nil
	}
	if command.Capability == "probe" {
		return n.executeProbe(ctx, command, materialized)
	}
	if command.Capability == "thumbnail" {
		return n.executeThumbnail(ctx, command, materialized)
	}
	return failed(command, "unsupported_capability", "the worker capability is not enabled"), nil
}

func (n Node) executeProbe(ctx context.Context, command worker.Command, materialized MaterializedArtifact) (worker.Result, error) {
	declaredMIME := configString(command.Config, "declared_mime")
	value, err := n.Validator.Validate(ctx, materialized.Path, declaredMIME)
	if err != nil {
		return failed(command, "probe_failed", "media validation could not be completed"), nil
	}
	if value.Quarantined || !value.Validated {
		return failed(command, "media_quarantined", safeReason(value.SafeReason)), nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return worker.Result{}, err
	}
	output, err := n.Stager.Stage(ctx, command.ProjectID, "media_probe", "probe_report", strings.NewReader(string(payload)), "application/json")
	if err != nil {
		return failed(command, "artifact_stage_failed", "the probe Artifact could not be staged"), nil
	}
	return worker.Result{SchemaVersion: worker.ResultSchemaVersion, MessageID: command.MessageID, JobID: command.JobID, JobStepID: command.JobStepID, Status: "completed", OutputRefs: []worker.OutputRef{output}}, nil
}

func (n Node) executeThumbnail(ctx context.Context, command worker.Command, materialized MaterializedArtifact) (worker.Result, error) {
	if n.Process == nil || n.FileStager == nil {
		return failed(command, "thumbnail_unavailable", "thumbnail execution is not configured"), nil
	}
	scratch := materialized.ScratchDir
	if scratch == "" {
		scratch = filepath.Dir(materialized.Path)
	}
	output, err := os.CreateTemp(scratch, "nh-media-thumbnail-*.jpg")
	if err != nil {
		return failed(command, "thumbnail_stage_failed", "thumbnail scratch storage is unavailable"), nil
	}
	outputPath := output.Name()
	_ = output.Close()
	defer os.Remove(outputPath)
	_, err = n.Process.Run(ctx, process.Spec{Tool: process.ToolFFmpeg, Args: []string{"-v", "error", "-i", materialized.Path, "-frames:v", "1", "-f", "image2", outputPath}, InputPaths: []string{materialized.Path}, OutputPaths: []string{outputPath}})
	if err != nil {
		return failed(command, "thumbnail_failed", "thumbnail generation failed"), nil
	}
	value, err := n.FileStager.StageFile(ctx, command.ProjectID, "thumbnail", "thumbnail", outputPath, "image/jpeg")
	if err != nil {
		return failed(command, "artifact_stage_failed", "the thumbnail Artifact could not be staged"), nil
	}
	return worker.Result{SchemaVersion: worker.ResultSchemaVersion, MessageID: command.MessageID, JobID: command.JobID, JobStepID: command.JobStepID, Status: "completed", OutputRefs: []worker.OutputRef{value}}, nil
}

func failed(command worker.Command, code, message string) worker.Result {
	return worker.Result{SchemaVersion: worker.ResultSchemaVersion, MessageID: command.MessageID, JobID: command.JobID, JobStepID: command.JobStepID, Status: "failed", SafeError: &worker.SafeError{Code: code, Category: "permanent", Retryable: false, SafeMessage: message}}
}

func configString(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return value
}

func safeReason(reason string) string {
	if reason == "" {
		return "media validation failed"
	}
	if len(reason) > 128 {
		return reason[:128]
	}
	return fmt.Sprintf("media validation failed: %s", reason)
}
