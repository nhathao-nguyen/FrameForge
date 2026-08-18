package render

// Native TimelineVersion compiler and deliverable QA.  This package accepts
// only typed Timeline/Profile data and executor-scoped Artifact handles.  It
// never calls providers, rematches scenes or accepts raw FFmpeg arguments.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nhathao-nguyen/NH-Media/services/media-worker/internal/process"
)

type RenderProfile struct {
	Key             string
	Version         int
	Width           int
	Height          int
	VideoCodec      string
	AudioCodec      string
	PixelFormat     string
	Bitrate         string
	FPS             int
	AudioTargetLUFS float64
	TruePeakDB      float64
	AutoReframe     bool
}

func DefaultProfile(key string) (RenderProfile, error) {
	profiles := map[string]RenderProfile{
		"youtube_16_9": {Key: "youtube_16_9", Version: 1, Width: 640, Height: 360, VideoCodec: "libx264", AudioCodec: "aac", PixelFormat: "yuv420p", FPS: 30, AudioTargetLUFS: -16, TruePeakDB: -1},
		"shorts_9_16":  {Key: "shorts_9_16", Version: 1, Width: 360, Height: 640, VideoCodec: "libx264", AudioCodec: "aac", PixelFormat: "yuv420p", FPS: 30, AudioTargetLUFS: -16, TruePeakDB: -1},
		"square_1_1":   {Key: "square_1_1", Version: 1, Width: 480, Height: 480, VideoCodec: "libx264", AudioCodec: "aac", PixelFormat: "yuv420p", FPS: 30, AudioTargetLUFS: -16, TruePeakDB: -1},
	}
	profile, ok := profiles[key]
	if !ok {
		return RenderProfile{}, fmt.Errorf("render profile %q is not allowlisted", key)
	}
	return profile, nil
}

type CompileInput struct {
	TimelineDocument []byte
	Profile          RenderProfile
	ArtifactPaths    map[string]string
	OutputPath       string
}

type Plan struct {
	Args                []string
	InputPaths          []string
	OutputPaths         []string
	OutputPath          string
	SourceFingerprint   string
	PlanFingerprint     string
	ProfileSnapshot     RenderProfile
	ProviderCallCount   int
	RematchingPerformed bool
}

type RenderResult struct {
	Plan        Plan
	OutputPath  string
	SHA256      string
	SizeBytes   int64
	DurationSec float64
	Width       int
	Height      int
	HasVideo    bool
	HasAudio    bool
	QAReport    QAReport
}

// ClipExportRequest is the reviewed, typed boundary for a short/clip
// derivative. The caller supplies a selection from the canonical timeline;
// it never supplies arbitrary FFmpeg argv.
type ClipExportRequest struct {
	SelectionID string
	SourcePath  string
	StartSec    float64
	EndSec      float64
	OutputPath  string
	Profile     RenderProfile
}

type ClipExport struct {
	SelectionID       string
	StartSec          float64
	EndSec            float64
	OutputPath        string
	SourceFingerprint string
	SHA256            string
	SizeBytes         int64
	DurationSec       float64
	Width             int
	Height            int
	VideoCodec        string
	AudioCodec        string
	QAReport          QAReport
}

type ClipExportManifest struct {
	SchemaVersion     string
	SourceFingerprint string
	Exports           []ClipExport
}

type ArtifactCandidate struct {
	Kind       string
	Role       string
	Path       string
	SHA256     string
	SizeBytes  int64
	Provenance map[string]any
}

type ArtifactCommitter interface {
	Commit(ctx context.Context, candidate ArtifactCandidate) error
}

type QAReport struct {
	Passed      bool
	Errors      []string
	DurationSec float64
	Width       int
	Height      int
	HasVideo    bool
	HasAudio    bool
	VideoCodec  string
	AudioCodec  string
}

type timelineDocument struct {
	SchemaVersion string          `json:"schema_version"`
	DurationSec   float64         `json:"duration_sec"`
	Tracks        []timelineTrack `json:"tracks"`
}

type timelineTrack struct {
	ID    string         `json:"id"`
	Kind  string         `json:"kind"`
	Clips []timelineClip `json:"clips"`
	Mix   *audioMix      `json:"mix,omitempty"`
	Style map[string]any `json:"style,omitempty"`
}

type timelineClip struct {
	ID            string         `json:"id"`
	TimelineIn    float64        `json:"timeline_in_sec"`
	TimelineOut   float64        `json:"timeline_out_sec"`
	SourceIn      *float64       `json:"source_in_sec,omitempty"`
	SourceOut     *float64       `json:"source_out_sec,omitempty"`
	Source        sourceRef      `json:"source"`
	Speed         float64        `json:"speed,omitempty"`
	TransitionIn  *transition    `json:"transition_in,omitempty"`
	TransitionOut *transition    `json:"transition_out,omitempty"`
	Style         map[string]any `json:"style,omitempty"`
	Subtitle      *subtitleCue   `json:"subtitle,omitempty"`
	Origin        string         `json:"origin"`
}

type sourceRef struct {
	Type       string `json:"type"`
	ArtifactID string `json:"artifact_id"`
}

type transition struct {
	Kind     string  `json:"kind"`
	Duration float64 `json:"duration_sec"`
}

type subtitleCue struct {
	Text     string `json:"text"`
	Language string `json:"language"`
}

type audioMix struct {
	Enabled         *bool    `json:"enabled,omitempty"`
	Volume          float64  `json:"volume,omitempty"`
	GainDB          float64  `json:"gain_db,omitempty"`
	FadeInSec       float64  `json:"fade_in_sec,omitempty"`
	FadeOutSec      float64  `json:"fade_out_sec,omitempty"`
	DuckDB          float64  `json:"duck_db,omitempty"`
	DuckUnderTracks []string `json:"duck_under_track_ids,omitempty"`
}

func Compile(input CompileInput) (Plan, error) {
	if len(input.TimelineDocument) == 0 {
		return Plan{}, errors.New("timeline document is required")
	}
	if err := validateProfile(input.Profile); err != nil {
		return Plan{}, err
	}
	var document timelineDocument
	if err := json.Unmarshal(input.TimelineDocument, &document); err != nil {
		return Plan{}, fmt.Errorf("timeline document is invalid: %w", err)
	}
	if document.SchemaVersion != "1.0" || document.DurationSec <= 0 || len(document.Tracks) == 0 {
		return Plan{}, errors.New("timeline document does not satisfy the renderer contract")
	}
	videoClips := make([]timelineClip, 0)
	inputPaths := make([]string, 0)
	seenInputs := map[string]bool{}
	for _, track := range document.Tracks {
		if err := validateMix(track.Mix); err != nil {
			return Plan{}, fmt.Errorf("track %q: %w", track.ID, err)
		}
		for _, clip := range track.Clips {
			if err := validateClip(document.DurationSec, clip, input.ArtifactPaths); err != nil {
				return Plan{}, fmt.Errorf("clip %q: %w", clip.ID, err)
			}
			if clip.TransitionIn != nil {
				if err := validateTransition(*clip.TransitionIn, clip.TimelineOut-clip.TimelineIn); err != nil {
					return Plan{}, fmt.Errorf("clip %q transition_in: %w", clip.ID, err)
				}
			}
			if clip.TransitionOut != nil {
				if err := validateTransition(*clip.TransitionOut, clip.TimelineOut-clip.TimelineIn); err != nil {
					return Plan{}, fmt.Errorf("clip %q transition_out: %w", clip.ID, err)
				}
			}
			if clip.Source.Type == "asset" || clip.Source.Type == "scene" || clip.Source.Type == "artifact" {
				if !seenInputs[input.ArtifactPaths[clip.Source.ArtifactID]] {
					inputPaths = append(inputPaths, input.ArtifactPaths[clip.Source.ArtifactID])
					seenInputs[input.ArtifactPaths[clip.Source.ArtifactID]] = true
				}
			}
			if track.Kind == "video" {
				videoClips = append(videoClips, clip)
			}
			if track.Kind == "subtitle" && clip.Subtitle == nil {
				return Plan{}, fmt.Errorf("subtitle clip %q has no typed cue", clip.ID)
			}
		}
	}
	if len(videoClips) == 0 || len(videoClips) > 1 {
		return Plan{}, errors.New("the deterministic baseline requires exactly one video clip; multi-clip concat is a separate allowlisted plan")
	}
	if strings.TrimSpace(input.OutputPath) == "" || input.OutputPath == input.ArtifactPaths[videoClips[0].Source.ArtifactID] {
		return Plan{}, errors.New("render output must be a distinct executor-scoped path")
	}
	clip := videoClips[0]
	if input.Profile.AutoReframe {
		return Plan{}, errors.New("subject-aware reframe must be compiled as a profile-specific intermediate artifact before render")
	}
	filter := scaleFilter(input.Profile)
	if clip.Style != nil {
		effect, _ := clip.Style["effect"].(string)
		if effect != "" && effect != "none" && effect != "fade_in" && effect != "fade_out" {
			return Plan{}, fmt.Errorf("effect %q is not allowlisted", effect)
		}
	}
	for _, track := range document.Tracks {
		if track.Kind == "subtitle" {
			for _, subtitleClip := range track.Clips {
				if subtitleClip.Subtitle != nil {
					textFilter, err := drawTextFilter(*subtitleClip.Subtitle, subtitleClip.TimelineIn, subtitleClip.TimelineOut)
					if err != nil {
						return Plan{}, fmt.Errorf("subtitle clip %q: %w", subtitleClip.ID, err)
					}
					filter += "," + textFilter
				}
			}
		}
	}
	outputPaths := []string{input.OutputPath}
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", input.ArtifactPaths[clip.Source.ArtifactID], "-ss", formatSeconds(sourceIn(clip)), "-t", formatSeconds(clip.TimelineOut - clip.TimelineIn), "-vf", filter, "-map", "0:v:0", "-map", "0:a?", "-c:v", input.Profile.VideoCodec, "-pix_fmt", input.Profile.PixelFormat, "-c:a", input.Profile.AudioCodec, "-movflags", "+faststart", input.OutputPath}
	// This is deliberately profile-independent: a profile-only rerender may
	// create a new plan/output but must reuse every upstream AI/media Artifact.
	sourceFingerprint := sha256JSON(canonicalJSON(input.TimelineDocument))
	planFingerprint := sha256JSON(struct {
		Source  string
		Profile RenderProfile
		Args    []string
	}{Source: sourceFingerprint, Profile: input.Profile, Args: args})
	return Plan{Args: args, InputPaths: inputPaths, OutputPaths: outputPaths, OutputPath: input.OutputPath, SourceFingerprint: sourceFingerprint, PlanFingerprint: planFingerprint, ProfileSnapshot: input.Profile}, nil
}

func Render(ctx context.Context, input CompileInput, policy process.Policy) (RenderResult, error) {
	plan, err := Compile(input)
	if err != nil {
		return RenderResult{}, err
	}
	if err := ensureSandboxPath(policy.SandboxRoot, plan.OutputPath); err != nil {
		return RenderResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(plan.OutputPath), 0o700); err != nil {
		return RenderResult{}, fmt.Errorf("prepare render output: %w", err)
	}
	_, err = policy.Run(ctx, process.Spec{Tool: process.ToolFFmpeg, Args: plan.Args, InputPaths: plan.InputPaths, OutputPaths: plan.OutputPaths, Timeout: policy.MaxDuration})
	if err != nil {
		return RenderResult{}, err
	}
	qa, err := Inspect(ctx, plan.OutputPath, policy)
	if err != nil {
		return RenderResult{}, err
	}
	if !qa.Passed {
		return RenderResult{}, fmt.Errorf("deliverable QA failed: %s", strings.Join(qa.Errors, ", "))
	}
	content, err := os.ReadFile(plan.OutputPath)
	if err != nil {
		return RenderResult{}, fmt.Errorf("read render output: %w", err)
	}
	digest := sha256.Sum256(content)
	return RenderResult{Plan: plan, OutputPath: plan.OutputPath, SHA256: hex.EncodeToString(digest[:]), SizeBytes: int64(len(content)), DurationSec: qa.DurationSec, Width: qa.Width, Height: qa.Height, HasVideo: qa.HasVideo, HasAudio: qa.HasAudio, QAReport: qa}, nil
}

// ExportClips creates short/clip derivatives only after validating the typed
// selection, running the allowlisted encoder, and passing ffprobe QA. The
// returned manifest is the only handoff to an Artifact boundary.
func ExportClips(ctx context.Context, requests []ClipExportRequest, policy process.Policy) (ClipExportManifest, error) {
	if len(requests) == 0 {
		return ClipExportManifest{}, errors.New("at least one clip selection is required")
	}
	manifest := ClipExportManifest{SchemaVersion: "1.0"}
	seenSelections := map[string]bool{}
	seenOutputs := map[string]bool{}
	for _, request := range requests {
		if strings.TrimSpace(request.SelectionID) == "" || seenSelections[request.SelectionID] {
			return ClipExportManifest{}, errors.New("clip selection identity must be unique")
		}
		seenSelections[request.SelectionID] = true
		if request.StartSec < 0 || request.EndSec <= request.StartSec {
			return ClipExportManifest{}, fmt.Errorf("clip %q selection range is invalid", request.SelectionID)
		}
		if strings.TrimSpace(request.SourcePath) == "" || strings.TrimSpace(request.OutputPath) == "" {
			return ClipExportManifest{}, fmt.Errorf("clip %q source/output path is required", request.SelectionID)
		}
		if err := validateProfile(request.Profile); err != nil {
			return ClipExportManifest{}, fmt.Errorf("clip %q: %w", request.SelectionID, err)
		}
		if err := ensureSandboxPath(policy.SandboxRoot, request.SourcePath); err != nil {
			return ClipExportManifest{}, fmt.Errorf("clip %q source: %w", request.SelectionID, err)
		}
		if err := ensureSandboxPath(policy.SandboxRoot, request.OutputPath); err != nil {
			return ClipExportManifest{}, fmt.Errorf("clip %q output: %w", request.SelectionID, err)
		}
		cleanOutput := filepath.Clean(request.OutputPath)
		if filepath.Clean(request.SourcePath) == cleanOutput || seenOutputs[cleanOutput] {
			return ClipExportManifest{}, fmt.Errorf("clip %q output must be distinct", request.SelectionID)
		}
		seenOutputs[cleanOutput] = true
		if err := os.MkdirAll(filepath.Dir(request.OutputPath), 0o700); err != nil {
			return ClipExportManifest{}, fmt.Errorf("prepare clip output: %w", err)
		}
		sourceFingerprint, err := sha256File(request.SourcePath)
		if err != nil {
			return ClipExportManifest{}, fmt.Errorf("clip %q source fingerprint: %w", request.SelectionID, err)
		}
		args := []string{
			"-hide_banner", "-loglevel", "error", "-y",
			"-ss", formatSeconds(request.StartSec), "-i", request.SourcePath,
			"-t", formatSeconds(request.EndSec - request.StartSec),
			"-vf", scaleFilter(request.Profile), "-map", "0:v:0", "-map", "0:a?",
			"-c:v", request.Profile.VideoCodec, "-pix_fmt", request.Profile.PixelFormat,
			"-c:a", request.Profile.AudioCodec, "-movflags", "+faststart", request.OutputPath,
		}
		if _, err := policy.Run(ctx, process.Spec{Tool: process.ToolFFmpeg, Args: args, InputPaths: []string{request.SourcePath}, OutputPaths: []string{request.OutputPath}, Timeout: policy.MaxDuration}); err != nil {
			return ClipExportManifest{}, fmt.Errorf("clip %q export: %w", request.SelectionID, err)
		}
		qa, err := Inspect(ctx, request.OutputPath, policy)
		if err != nil {
			return ClipExportManifest{}, fmt.Errorf("clip %q QA: %w", request.SelectionID, err)
		}
		if !qa.Passed || qa.DurationSec > request.EndSec-request.StartSec+0.25 {
			return ClipExportManifest{}, fmt.Errorf("clip %q QA failed: %s", request.SelectionID, strings.Join(qa.Errors, ", "))
		}
		content, err := os.ReadFile(request.OutputPath)
		if err != nil {
			return ClipExportManifest{}, fmt.Errorf("clip %q output: %w", request.SelectionID, err)
		}
		digest := sha256.Sum256(content)
		manifest.SourceFingerprint = sourceFingerprint
		manifest.Exports = append(manifest.Exports, ClipExport{
			SelectionID: request.SelectionID, StartSec: request.StartSec, EndSec: request.EndSec,
			OutputPath: request.OutputPath, SourceFingerprint: sourceFingerprint,
			SHA256: hex.EncodeToString(digest[:]), SizeBytes: int64(len(content)),
			DurationSec: qa.DurationSec, Width: qa.Width, Height: qa.Height,
			VideoCodec: qa.VideoCodec, AudioCodec: qa.AudioCodec, QAReport: qa,
		})
	}
	return manifest, nil
}

// RenderAndCommit keeps the storage/database split explicit: only a complete
// output with a passing QA report is offered to the Artifact commit boundary.
func RenderAndCommit(ctx context.Context, input CompileInput, policy process.Policy, committer ArtifactCommitter) (RenderResult, ArtifactCandidate, error) {
	if committer == nil {
		return RenderResult{}, ArtifactCandidate{}, errors.New("Artifact committer is required")
	}
	result, err := Render(ctx, input, policy)
	if err != nil {
		return RenderResult{}, ArtifactCandidate{}, err
	}
	candidate := ArtifactCandidate{
		Kind:      "render",
		Role:      "deliverable",
		Path:      result.OutputPath,
		SHA256:    result.SHA256,
		SizeBytes: result.SizeBytes,
		Provenance: map[string]any{
			"timeline_source_fingerprint": result.Plan.SourceFingerprint,
			"render_plan_fingerprint":     result.Plan.PlanFingerprint,
			"render_profile":              result.Plan.ProfileSnapshot,
			"qa_passed":                   result.QAReport.Passed,
		},
	}
	if err := committer.Commit(ctx, candidate); err != nil {
		return RenderResult{}, ArtifactCandidate{}, fmt.Errorf("commit render Artifact: %w", err)
	}
	return result, candidate, nil
}

func Inspect(ctx context.Context, path string, policy process.Policy) (QAReport, error) {
	if strings.TrimSpace(path) == "" {
		return QAReport{}, errors.New("deliverable path is required")
	}
	result, err := policy.Run(ctx, process.Spec{Tool: process.ToolFFprobe, Args: []string{"-v", "error", "-show_entries", "format=duration:stream=codec_type,codec_name,width,height", "-of", "json", path}, InputPaths: []string{path}, Timeout: policy.MaxDuration})
	if err != nil {
		return QAReport{}, err
	}
	var probe struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &probe); err != nil {
		return QAReport{}, fmt.Errorf("ffprobe report is invalid")
	}
	duration, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil || duration <= 0 {
		return QAReport{Errors: []string{"duration_missing_or_invalid"}}, nil
	}
	qa := QAReport{Passed: true, DurationSec: duration}
	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			qa.HasVideo = true
			qa.VideoCodec = stream.CodecName
			if qa.Width == 0 {
				qa.Width, qa.Height = stream.Width, stream.Height
			}
		case "audio":
			qa.HasAudio = true
			qa.AudioCodec = stream.CodecName
		}
	}
	if !qa.HasVideo {
		qa.Errors = append(qa.Errors, "video_stream_missing")
	}
	if qa.Width <= 0 || qa.Height <= 0 {
		qa.Errors = append(qa.Errors, "video_dimensions_missing")
	}
	qa.Passed = len(qa.Errors) == 0
	return qa, nil
}

func validateProfile(profile RenderProfile) error {
	if profile.Key == "" || profile.Version < 1 || profile.Width < 1 || profile.Height < 1 || profile.Width > 16384 || profile.Height > 16384 {
		return errors.New("render profile dimensions/version are invalid")
	}
	if profile.VideoCodec != "libx264" || profile.AudioCodec != "aac" || profile.PixelFormat != "yuv420p" {
		return errors.New("render profile codec is not allowlisted")
	}
	if profile.FPS < 1 || profile.FPS > 120 || profile.AudioTargetLUFS > 0 || profile.TruePeakDB > 0 {
		return errors.New("render profile quality policy is invalid")
	}
	return nil
}

func validateClip(timelineDuration float64, clip timelineClip, artifactPaths map[string]string) error {
	if clip.ID == "" || clip.Origin == "" || clip.TimelineIn < 0 || clip.TimelineOut <= clip.TimelineIn || clip.TimelineOut > timelineDuration+0.001 {
		return errors.New("clip identity or timeline range is invalid")
	}
	if clip.Source.Type != "asset" && clip.Source.Type != "scene" && clip.Source.Type != "artifact" {
		if clip.Source.Type == "none" && clip.Source.ArtifactID == "" && clip.SourceIn == nil && clip.SourceOut == nil {
			return nil
		}
		return errors.New("render source type is not allowlisted")
	}
	if clip.Source.ArtifactID == "" || strings.TrimSpace(artifactPaths[clip.Source.ArtifactID]) == "" {
		return errors.New("render source Artifact handle is missing")
	}
	if clip.Speed < 0 || clip.Speed > 16 {
		return errors.New("clip speed is invalid")
	}
	if clip.SourceIn != nil && *clip.SourceIn < 0 {
		return errors.New("source range is invalid")
	}
	if clip.SourceOut != nil && (*clip.SourceOut <= 0 || clip.SourceIn == nil || *clip.SourceOut <= *clip.SourceIn) {
		return errors.New("source range is invalid")
	}
	return nil
}

func validateTransition(value transition, duration float64) error {
	switch value.Kind {
	case "cut", "fade", "crossfade", "dip_to_black", "wipe":
	default:
		return errors.New("transition kind is not allowlisted")
	}
	if value.Duration < 0 || value.Duration > duration {
		return errors.New("transition duration is invalid")
	}
	return nil
}

func validateMix(mix *audioMix) error {
	if mix == nil {
		return nil
	}
	if mix.Volume < 0 || mix.Volume > 4 || mix.GainDB < -96 || mix.GainDB > 24 || mix.FadeInSec < 0 || mix.FadeOutSec < 0 || mix.DuckDB < -48 || mix.DuckDB > 0 {
		return errors.New("audio mix policy is invalid")
	}
	return nil
}

func scaleFilter(profile RenderProfile) string {
	// The reviewed process policy rejects shell metacharacters.  The baseline
	// uses deterministic top-left padding; subject-aware profile reframe is a
	// separate typed intermediate and is never silently center-cropped here.
	return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:0:0:color=black", profile.Width, profile.Height, profile.Width, profile.Height)
}

func drawTextFilter(cue subtitleCue, in, out float64) (string, error) {
	if cue.Language == "" || cue.Text == "" || strings.ContainsAny(cue.Text, "'\\\n\r;|$`()") {
		return "", errors.New("subtitle text contains unsafe characters")
	}
	return fmt.Sprintf("drawtext=text='%s':enable='between(t,%s,%s)'", cue.Text, formatSeconds(in), formatSeconds(out)), nil
}

func sourceIn(clip timelineClip) float64 {
	if clip.SourceIn != nil {
		return *clip.SourceIn
	}
	return 0
}

func formatSeconds(value float64) string {
	return strconv.FormatFloat(value, 'f', 3, 64)
}

func canonicalJSON(value []byte) []byte {
	var decoded any
	if json.Unmarshal(value, &decoded) != nil {
		return value
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return value
	}
	return encoded
}

func sha256JSON(value any) string {
	encoded, _ := json.Marshal(value)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func sha256File(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}

func ensureSandboxPath(root, value string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	valueAbs, err := filepath.Abs(value)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(rootAbs, valueAbs)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("render output escapes executor sandbox")
	}
	return nil
}
