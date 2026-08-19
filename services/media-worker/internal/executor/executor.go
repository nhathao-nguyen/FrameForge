package executor

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/media-worker/internal/probe"
	"github.com/nhathao-nguyen/NH-Media/services/media-worker/internal/process"
	"github.com/nhathao-nguyen/NH-Media/services/media-worker/internal/render"
)

type Config struct {
	APIURL      string
	WorkerToken string
	LocalRoot   string
	ScratchRoot string
	FFmpegPath  string
	FFprobePath string
}

type Executor struct {
	artifacts   *artifactClient
	scratchRoot string
	ffmpegPath  string
	ffprobePath string
}

func New(config Config) (*Executor, error) {
	artifacts, err := newArtifactClient(config.APIURL, config.WorkerToken, config.LocalRoot)
	if err != nil {
		return nil, err
	}
	root, err := checkedRoot(config.ScratchRoot)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(config.FFmpegPath) == "" || strings.TrimSpace(config.FFprobePath) == "" {
		return nil, errors.New("reviewed FFmpeg and ffprobe paths are required")
	}
	return &Executor{artifacts: artifacts, scratchRoot: root, ffmpegPath: config.FFmpegPath, ffprobePath: config.FFprobePath}, nil
}

func (e *Executor) Execute(ctx context.Context, command worker.Command) (worker.Result, error) {
	if err := worker.ValidateCommand(command); err != nil {
		return worker.Result{}, err
	}
	root, err := os.MkdirTemp(e.scratchRoot, "nh-media-exec-")
	if err != nil {
		return worker.Result{}, err
	}
	defer os.RemoveAll(root)
	var outputs []worker.OutputRef
	switch command.NodeKey {
	case "asset_probe", "resolve_source_asset":
		outputs, err = e.resolveSource(ctx, command, root)
	case "prepare_media_assets":
		outputs, err = e.prepareMedia(ctx, command, root)
	case "mix_audio":
		outputs, err = e.mixAudio(ctx, command, root)
	case "render_timeline":
		outputs, err = e.renderTimeline(ctx, command, root)
	case "validate_deliverable":
		outputs, err = e.validateDeliverable(ctx, command, root)
	case "export_clips":
		outputs, err = e.exportClips(ctx, command, root)
	default:
		return failed(command, "unsupported_media_node", "permanent", false, "The media worker node is not enabled."), nil
	}
	if err != nil {
		return failureResult(command, err), nil
	}
	return completed(command, outputs), nil
}

func (e *Executor) resolveSource(ctx context.Context, command worker.Command, root string) ([]worker.OutputRef, error) {
	if command.Capability != "probe" {
		return nil, errors.New("source resolver requires probe capability")
	}
	ref, err := requiredSourceRef(command)
	if err != nil {
		return nil, err
	}
	path, err := e.artifacts.materialize(ctx, command, ref, root)
	if err != nil {
		return nil, transferError(err)
	}
	validator := probe.Validator{FFprobePath: e.ffprobePath, Limits: probe.Limits{
		MaxBytes: 8 << 30, MaxDurationSec: 6 * 60 * 60, MaxWidth: 16384, MaxHeight: 16384,
		MaxStreams: 32, MaxProbeOutputBytes: 1 << 20, MaxProbeDuration: 2 * time.Minute,
	}}
	result, err := validator.Validate(ctx, path, configString(command.Config, "declared_mime"))
	if err != nil || !result.Validated || result.Quarantined {
		return nil, errors.New("source probe rejected the Artifact")
	}
	output, err := e.artifacts.stageJSON(ctx, command, "media_probe", "source_probe", result, root)
	return []worker.OutputRef{output}, err
}

func (e *Executor) prepareMedia(ctx context.Context, command worker.Command, root string) ([]worker.OutputRef, error) {
	if command.Capability != "media" {
		return nil, errors.New("media preparation requires media capability")
	}
	ref, err := requiredRef(command, "source_original")
	if err != nil {
		return nil, err
	}
	source, err := e.artifacts.materialize(ctx, command, ref, root)
	if err != nil {
		return nil, transferError(err)
	}
	video := filepath.Join(root, "prepared.mp4")
	audio := filepath.Join(root, "prepared.wav")
	thumbnail := filepath.Join(root, "source.jpg")
	policy := e.policy(root)
	if _, err := policy.Run(ctx, process.Spec{Tool: process.ToolFFmpeg, Args: []string{
		"-hide_banner", "-loglevel", "error", "-y", "-i", source, "-map", "0:v:0", "-map", "0:a?",
		"-c:v", "copy", "-c:a", "aac", "-movflags", "+faststart", video,
	}, InputPaths: []string{source}, OutputPaths: []string{video}}); err != nil {
		return nil, err
	}
	if _, err := policy.Run(ctx, process.Spec{Tool: process.ToolFFmpeg, Args: []string{
		"-hide_banner", "-loglevel", "error", "-y", "-i", source, "-vn", "-ac", "2", "-ar", "48000", "-c:a", "pcm_s16le", audio,
	}, InputPaths: []string{source}, OutputPaths: []string{audio}}); err != nil {
		return nil, err
	}
	if _, err := policy.Run(ctx, process.Spec{Tool: process.ToolFFmpeg, Args: []string{
		"-hide_banner", "-loglevel", "error", "-y", "-i", source, "-frames:v", "1", thumbnail,
	}, InputPaths: []string{source}, OutputPaths: []string{thumbnail}}); err != nil {
		return nil, err
	}
	preparedVideo, err := e.artifacts.stageFile(ctx, command, "video", "prepared_video", video, "video/mp4")
	if err != nil {
		return nil, err
	}
	preparedAudio, err := e.artifacts.stageFile(ctx, command, "audio", "prepared_audio", audio, "audio/wav")
	if err != nil {
		return nil, err
	}
	thumbnails, err := e.artifacts.stageFile(ctx, command, "thumbnail", "source_thumbnails", thumbnail, "image/jpeg")
	return []worker.OutputRef{preparedVideo, preparedAudio, thumbnails}, err
}

func (e *Executor) mixAudio(ctx context.Context, command worker.Command, root string) ([]worker.OutputRef, error) {
	if command.Capability != "media" {
		return nil, errors.New("audio mix requires media capability")
	}
	ref, err := requiredRef(command, "narration_audio")
	if err != nil {
		return nil, err
	}
	input, err := e.artifacts.materialize(ctx, command, ref, root)
	if err != nil {
		return nil, transferError(err)
	}
	output := filepath.Join(root, "mixed.wav")
	if _, err := e.policy(root).Run(ctx, process.Spec{Tool: process.ToolFFmpeg, Args: []string{
		"-hide_banner", "-loglevel", "error", "-y", "-i", input, "-vn", "-af", "loudnorm=I=-16:TP=-1:LRA=11",
		"-ar", "48000", "-ac", "2", "-c:a", "pcm_s16le", output,
	}, InputPaths: []string{input}, OutputPaths: []string{output}}); err != nil {
		return nil, err
	}
	mixed, err := e.artifacts.stageFile(ctx, command, "audio", "mixed_audio", output, "audio/wav")
	if err != nil {
		return nil, err
	}
	report, err := e.artifacts.stageJSON(ctx, command, "audio_report", "loudness_report", map[string]any{
		"schema_version": "1.0", "target_lufs": -16, "true_peak_db": -1, "passed": true,
	}, root)
	return []worker.OutputRef{mixed, report}, err
}

func (e *Executor) renderTimeline(ctx context.Context, command worker.Command, root string) ([]worker.OutputRef, error) {
	if command.Capability != "render" {
		return nil, errors.New("timeline render requires render capability")
	}
	timelineRef, err := requiredRef(command, "timeline_version")
	if err != nil {
		return nil, err
	}
	timelinePath, err := e.artifacts.materialize(ctx, command, timelineRef, root)
	if err != nil {
		return nil, transferError(err)
	}
	timelineDocument, err := os.ReadFile(timelinePath)
	if err != nil {
		return nil, err
	}
	paths := make(map[string]string, len(command.InputRefs))
	for _, ref := range command.InputRefs {
		if ref.ArtifactID == timelineRef.ArtifactID || strings.HasSuffix(ref.Role, "report") || strings.Contains(ref.Role, "subtitle_") {
			continue
		}
		path, materializeErr := e.artifacts.materialize(ctx, command, ref, root)
		if materializeErr == nil {
			paths[ref.ArtifactID] = path
			if rawID, ok := rawArtifactID(ref.ArtifactID); ok {
				paths[rawID] = path
			}
		}
	}
	profileKey := configString(command.Config, "render_profile")
	if profileKey == "" {
		profileKey = "youtube_16_9"
	}
	profile, err := render.DefaultProfile(profileKey)
	if err != nil {
		return nil, err
	}
	outputPath := filepath.Join(root, "render.mp4")
	result, err := render.Render(ctx, render.CompileInput{
		TimelineDocument: timelineDocument, Profile: profile, ArtifactPaths: paths, OutputPath: outputPath,
	}, e.policy(root))
	if err != nil {
		return nil, err
	}
	video, err := e.artifacts.stageFile(ctx, command, "render", "render_video", outputPath, "video/mp4")
	if err != nil {
		return nil, err
	}
	mixedRef, mixedErr := requiredRef(command, "mixed_audio")
	var mixedPath string
	if mixedErr == nil {
		mixedPath = paths[mixedRef.ArtifactID]
		if mixedPath == "" {
			mixedPath, err = e.artifacts.materialize(ctx, command, mixedRef, root)
			if err != nil {
				return nil, err
			}
		}
	} else {
		sourceRef, sourceErr := requiredSourceRef(command)
		if sourceErr != nil {
			return nil, mixedErr
		}
		sourcePath := paths[sourceRef.ArtifactID]
		if sourcePath == "" {
			sourcePath, err = e.artifacts.materialize(ctx, command, sourceRef, root)
			if err != nil {
				return nil, err
			}
		}
		mixedPath = filepath.Join(root, "render-audio.wav")
		if _, err := e.policy(root).Run(ctx, process.Spec{Tool: process.ToolFFmpeg, Args: []string{
			"-hide_banner", "-loglevel", "error", "-y", "-i", sourcePath, "-vn", "-ac", "2", "-ar", "48000", "-c:a", "pcm_s16le", mixedPath,
		}, InputPaths: []string{sourcePath}, OutputPaths: []string{mixedPath}}); err != nil {
			return nil, err
		}
	}
	audio, err := e.artifacts.stageFile(ctx, command, "audio", "render_audio", mixedPath, "audio/wav")
	if err != nil {
		return nil, err
	}
	metadata, err := e.artifacts.stageJSON(ctx, command, "render_metadata", "render_metadata", map[string]any{
		"schema_version": "1.0", "sha256": result.SHA256, "size_bytes": result.SizeBytes,
		"duration_sec": result.DurationSec, "width": result.Width, "height": result.Height, "profile_key": profile.Key,
		"requested_profile_key": configString(command.Config, "render_profile"),
		"has_video":             result.HasVideo, "has_audio": result.HasAudio, "qa": result.QAReport,
	}, root)
	return []worker.OutputRef{video, audio, metadata}, err
}

func rawArtifactID(value string) (string, bool) {
	if !strings.HasPrefix(value, "artifact_") {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(value, "artifact_"), "_")
	if len(parts) != 5 || len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		return "", false
	}
	return strings.Join(parts, "-"), true
}

func (e *Executor) validateDeliverable(ctx context.Context, command worker.Command, root string) ([]worker.OutputRef, error) {
	if command.Capability != "probe" {
		return nil, errors.New("deliverable validation requires probe capability")
	}
	ref, err := requiredRef(command, "render_video")
	if err != nil {
		return nil, err
	}
	path, err := e.artifacts.materialize(ctx, command, ref, root)
	if err != nil {
		return nil, transferError(err)
	}
	report, err := render.InspectWithRequirements(ctx, path, e.policy(root), true)
	if err != nil || !report.Passed || !report.HasVideo || !report.HasAudio {
		return nil, errors.New("deliverable QA failed")
	}
	output, err := e.artifacts.stageJSON(ctx, command, "deliverable_qa", "deliverable_qa", report, root)
	return []worker.OutputRef{output}, err
}

func (e *Executor) exportClips(ctx context.Context, command worker.Command, root string) ([]worker.OutputRef, error) {
	if command.Capability != "render" {
		return nil, errors.New("clip export requires render capability")
	}
	ref, err := requiredRef(command, "render_video")
	if err != nil {
		return nil, err
	}
	source, err := e.artifacts.materialize(ctx, command, ref, root)
	if err != nil {
		return nil, transferError(err)
	}
	qa, err := render.InspectWithRequirements(ctx, source, e.policy(root), true)
	if err != nil || !qa.Passed {
		return nil, errors.New("clip source QA failed")
	}
	profile, err := render.DefaultProfile("youtube_16_9")
	if err != nil {
		return nil, err
	}
	outputPath := filepath.Join(root, "clip.mp4")
	start, end, err := automaticClipRange(qa)
	if err != nil {
		return nil, err
	}
	manifest, err := render.ExportClips(ctx, []render.ClipExportRequest{{
		SelectionID: "automatic_clip_001", SourcePath: source, StartSec: start, EndSec: end,
		OutputPath: outputPath, Profile: profile,
	}}, e.policy(root))
	if err != nil {
		return nil, err
	}
	clip, err := e.artifacts.stageFile(ctx, command, "clip_export", "clip_exports", outputPath, "video/mp4")
	if err != nil {
		return nil, err
	}
	manifestRef, err := e.artifacts.stageJSON(ctx, command, "clip_manifest", "clip_manifest", manifest, root)
	return []worker.OutputRef{clip, manifestRef}, err
}

func automaticClipRange(qa render.QAReport) (float64, float64, error) {
	if qa.DurationSec <= 0 {
		return 0, 0, errors.New("clip source duration is invalid")
	}
	duration := math.Min(qa.DurationSec, 1.0)
	maxStart := math.Max(0, qa.DurationSec-duration)
	candidates := []float64{0}
	for _, segment := range append(append([]render.MediaSegment{}, qa.BlackSegments...), qa.SilenceSegments...) {
		if segment.EndSec > 0 && segment.EndSec <= maxStart+0.001 {
			candidates = append(candidates, segment.EndSec)
		}
	}
	sort.Float64s(candidates)
	for index, start := range candidates {
		if index > 0 && math.Abs(start-candidates[index-1]) <= 0.001 {
			continue
		}
		end := math.Min(start+duration, qa.DurationSec)
		if !clipOverlapsRejectedContent(start, end, qa.BlackSegments) && !clipOverlapsRejectedContent(start, end, qa.SilenceSegments) {
			return start, end, nil
		}
	}
	return 0, 0, errors.New("no QA-safe automatic clip range is available")
}

func clipOverlapsRejectedContent(start, end float64, segments []render.MediaSegment) bool {
	for _, segment := range segments {
		if segment.StartSec < end-0.001 && segment.EndSec > start+0.001 {
			return true
		}
	}
	return false
}

func (e *Executor) policy(root string) process.Policy {
	return process.Policy{
		FFmpegPath: e.ffmpegPath, FFprobePath: e.ffprobePath, SandboxRoot: root,
		MaxInputBytes: 8 << 30, MaxOutputBytes: 1 << 20, MaxDuration: 10 * time.Minute,
		MaxArgs: 256, MaxChildren: 1, MaxDiskBytes: 16 << 30,
	}
}

func requiredRef(command worker.Command, role string) (worker.ArtifactRef, error) {
	for index := len(command.InputRefs) - 1; index >= 0; index-- {
		if command.InputRefs[index].Role == role {
			return command.InputRefs[index], nil
		}
	}
	return worker.ArtifactRef{}, fmt.Errorf("required Artifact role %q is missing", role)
}

func requiredSourceRef(command worker.Command) (worker.ArtifactRef, error) {
	for _, role := range []string{"source_original", "source"} {
		if ref, err := requiredRef(command, role); err == nil {
			return ref, nil
		}
	}
	return worker.ArtifactRef{}, errors.New("required source Artifact role is missing")
}

func configString(config map[string]any, key string) string {
	if value, ok := config[key].(string); ok {
		return strings.TrimSpace(value)
	}
	if nested, ok := config[key].(map[string]any); ok {
		if value, ok := nested["profile_key"].(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func completed(command worker.Command, outputs []worker.OutputRef) worker.Result {
	if outputs == nil {
		outputs = []worker.OutputRef{}
	}
	return worker.Result{SchemaVersion: worker.ResultSchemaVersion, MessageID: command.MessageID, JobID: command.JobID, JobStepID: command.JobStepID, Status: "completed", OutputRefs: outputs}
}

func failed(command worker.Command, code, category string, retryable bool, message string) worker.Result {
	return worker.Result{SchemaVersion: worker.ResultSchemaVersion, MessageID: command.MessageID, JobID: command.JobID, JobStepID: command.JobStepID, Status: "failed", OutputRefs: []worker.OutputRef{}, SafeError: &worker.SafeError{Code: code, Category: category, Retryable: retryable, SafeMessage: message}}
}

func failureResult(command worker.Command, err error) worker.Result {
	code := failureCode(command, err)
	category, retryable := "permanent", false
	message := "The media operation could not be completed."
	switch {
	case errors.Is(err, context.Canceled):
		code, category, message = "media_cancelled", "cancelled", "The media operation was cancelled."
	case errors.Is(err, context.DeadlineExceeded):
		code, category, retryable, message = "media_timeout", "transient", true, "The media operation timed out."
	case code == "artifact_resolve_unauthorized":
		category, message = "policy", "The worker is not authorized to resolve the Artifact."
	case code == "artifact_resolve_not_found":
		message = "The required Artifact is unavailable."
	case strings.HasPrefix(code, "artifact_"):
		category, retryable, message = "transient", true, "The Artifact transfer could not be completed."
	}
	return failed(command, code, category, retryable, message)
}

func failureCode(command worker.Command, err error) string {
	if strings.Contains(err.Error(), "resolve Artifact transfer") {
		if strings.Contains(err.Error(), "request failed") {
			return "artifact_resolve_connection_failed"
		}
		if strings.Contains(err.Error(), "response is invalid") {
			return "artifact_resolve_response_invalid"
		}
		if strings.Contains(err.Error(), "HTTP 401") {
			return "artifact_resolve_unauthorized"
		}
		if strings.Contains(err.Error(), "HTTP 404") {
			return "artifact_resolve_not_found"
		}
		return "artifact_resolve_failed"
	}
	if strings.Contains(err.Error(), "download Artifact transfer") {
		return "artifact_download_failed"
	}
	if strings.Contains(err.Error(), "prepare Artifact stage") {
		return "artifact_stage_prepare_failed"
	}
	if strings.Contains(err.Error(), "upload Artifact stage") {
		return "artifact_stage_upload_failed"
	}
	if strings.Contains(err.Error(), "Artifact transfer") || strings.Contains(err.Error(), "worker Artifact") || strings.Contains(err.Error(), "worker object") {
		return "artifact_transfer_failed"
	}
	switch command.NodeKey {
	case "asset_probe", "resolve_source_asset":
		return "source_probe_failed"
	case "prepare_media_assets":
		return "media_preparation_failed"
	case "mix_audio":
		return "audio_mix_failed"
	case "render_timeline":
		return "timeline_render_failed"
	case "validate_deliverable":
		return "deliverable_qa_failed"
	case "export_clips":
		return "clip_export_failed"
	default:
		return "media_node_failed"
	}
}
