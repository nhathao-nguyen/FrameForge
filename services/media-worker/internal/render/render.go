package render

// Native TimelineVersion compiler and deliverable QA.  This package accepts
// only typed Timeline/Profile data and executor-scoped Artifact handles.  It
// never calls providers, rematches scenes or accepts raw FFmpeg arguments.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/nhathao-nguyen/NH-Media/services/media-worker/internal/process"
)

type RenderProfile struct {
	Key              string
	Version          int
	Width            int
	Height           int
	VideoCodec       string
	AudioCodec       string
	PixelFormat      string
	Bitrate          string
	FPS              int
	AudioTargetLUFS  float64
	TruePeakDB       float64
	AutoReframe      bool
	SubtitleFontPath string
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
	TimelineDocument      []byte
	Profile               RenderProfile
	ArtifactPaths         map[string]string
	OutputPath            string
	SubjectTracks         []SubjectTrack
	SubjectSourceRevision string
	SubtitleFiles         map[string]string
}

type SubjectTrack struct {
	ID             string
	SourceRevision string
	Points         []SubjectPoint
}

type SubjectPoint struct {
	TimeSec float64
	X       float64
	Y       float64
	Width   float64
	Height  float64
}

type Plan struct {
	Args                    []string
	InputPaths              []string
	OutputPaths             []string
	OutputPath              string
	SourceFingerprint       string
	PlanFingerprint         string
	ProfileSnapshot         RenderProfile
	ExpectedDurationSec     float64
	ProviderCallCount       int
	RematchingPerformed     bool
	RequiresAudio           bool
	SubjectReframeMode      string
	SubjectTrackIDs         []string
	SubjectTrackFingerprint string
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
	Passed             bool               `json:"passed"`
	Errors             []string           `json:"errors"`
	DurationSec        float64            `json:"duration_sec"`
	Width              int                `json:"width"`
	Height             int                `json:"height"`
	HasVideo           bool               `json:"has_video"`
	HasAudio           bool               `json:"has_audio"`
	VideoCodec         string             `json:"video_codec"`
	AudioCodec         string             `json:"audio_codec"`
	BlackDetected      bool               `json:"black_detected"`
	BlackDurationSec   float64            `json:"black_duration_sec"`
	BlackRatio         float64            `json:"black_ratio"`
	BlackSegments      []MediaSegment     `json:"black_segments"`
	BlackPolicy        BlackContentPolicy `json:"black_policy"`
	SilenceDetected    bool               `json:"silence_detected"`
	SilenceDurationSec float64            `json:"silence_duration_sec"`
	SilenceRatio       float64            `json:"silence_ratio"`
	SilenceSegments    []MediaSegment     `json:"silence_segments"`
	SilencePolicy      SilencePolicy      `json:"silence_policy"`
}

type MediaSegment struct {
	StartSec    float64 `json:"start_sec"`
	EndSec      float64 `json:"end_sec"`
	DurationSec float64 `json:"duration_sec"`
}

type BlackContentPolicy struct {
	MinimumSegmentSec    float64 `json:"minimum_segment_sec"`
	PixelThreshold       float64 `json:"pixel_threshold"`
	MaximumRatio         float64 `json:"maximum_ratio"`
	MaximumContinuousSec float64 `json:"maximum_continuous_sec"`
}

type SilencePolicy struct {
	NoiseThresholdDB     float64 `json:"noise_threshold_db"`
	MinimumSegmentSec    float64 `json:"minimum_segment_sec"`
	MaximumRatio         float64 `json:"maximum_ratio"`
	MaximumContinuousSec float64 `json:"maximum_continuous_sec"`
}

var defaultBlackContentPolicy = BlackContentPolicy{
	MinimumSegmentSec:    0.15,
	PixelThreshold:       0.10,
	MaximumRatio:         0.35,
	MaximumContinuousSec: 1.25,
}

var defaultSilencePolicy = SilencePolicy{
	NoiseThresholdDB:     -50,
	MinimumSegmentSec:    0.15,
	MaximumRatio:         0.75,
	MaximumContinuousSec: 0.75,
}

type subjectReframePlan struct {
	CenterX     float64
	CenterY     float64
	TrackIDs    []string
	Fingerprint string
}

type timelineDocument struct {
	SchemaVersion     string          `json:"schema_version"`
	TimelineID        string          `json:"timeline_id"`
	TimelineVersionID string          `json:"timeline_version_id"`
	ProjectID         string          `json:"project_id"`
	Version           int             `json:"version"`
	DurationSec       float64         `json:"duration_sec"`
	FrameRate         map[string]any  `json:"frame_rate,omitempty"`
	Canvas            map[string]any  `json:"canvas,omitempty"`
	Tracks            []timelineTrack `json:"tracks"`
	Markers           []any           `json:"markers,omitempty"`
	Metadata          map[string]any  `json:"metadata,omitempty"`
}

type timelineTrack struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	Name     string         `json:"name"`
	Order    int            `json:"order"`
	Clips    []timelineClip `json:"clips"`
	Mix      *audioMix      `json:"mix,omitempty"`
	Style    map[string]any `json:"style,omitempty"`
	Muted    bool           `json:"muted,omitempty"`
	Visible  bool           `json:"visible,omitempty"`
	Locked   bool           `json:"locked,omitempty"`
	Language string         `json:"language,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type timelineClip struct {
	ID            string         `json:"id"`
	TimelineIn    float64        `json:"timeline_in_sec"`
	TimelineOut   float64        `json:"timeline_out_sec"`
	SourceIn      *float64       `json:"source_in_sec,omitempty"`
	SourceOut     *float64       `json:"source_out_sec,omitempty"`
	Source        sourceRef      `json:"source"`
	Speed         float64        `json:"speed,omitempty"`
	Loop          bool           `json:"loop,omitempty"`
	FreezeFrame   bool           `json:"freeze_frame,omitempty"`
	Transform     map[string]any `json:"transform,omitempty"`
	Audio         *audioMix      `json:"audio,omitempty"`
	TransitionIn  *transition    `json:"transition_in,omitempty"`
	TransitionOut *transition    `json:"transition_out,omitempty"`
	Style         map[string]any `json:"style,omitempty"`
	Subtitle      *subtitleCue   `json:"subtitle,omitempty"`
	Text          string         `json:"text,omitempty"`
	Narration     map[string]any `json:"narration,omitempty"`
	Confidence    *float64       `json:"confidence,omitempty"`
	ProposalRef   string         `json:"proposal_ref,omitempty"`
	ProposalRefs  []string       `json:"proposal_refs,omitempty"`
	EvidenceRefs  []string       `json:"evidence_refs,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Origin        string         `json:"origin"`
}

type sourceRef struct {
	Type           string            `json:"type"`
	ArtifactID     string            `json:"artifact_id"`
	AssetID        string            `json:"asset_id,omitempty"`
	SceneID        string            `json:"scene_id,omitempty"`
	InlineID       string            `json:"inline_id,omitempty"`
	Role           string            `json:"role,omitempty"`
	RightsStatus   string            `json:"rights_status,omitempty"`
	RightsMetadata map[string]string `json:"rights_metadata,omitempty"`
}

type transition struct {
	Kind     string         `json:"kind"`
	Duration float64        `json:"duration_sec"`
	Params   map[string]any `json:"params,omitempty"`
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
	Pan             float64  `json:"pan,omitempty"`
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
	decoder := json.NewDecoder(bytes.NewReader(input.TimelineDocument))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return Plan{}, fmt.Errorf("timeline document is invalid: %w", err)
	}
	if document.SchemaVersion != "1.0" || document.TimelineID == "" || document.TimelineVersionID == "" || document.ProjectID == "" || document.Version < 1 || document.DurationSec <= 0 || len(document.Tracks) == 0 {
		return Plan{}, errors.New("timeline document does not satisfy the renderer contract")
	}
	var subjectReframe *subjectReframePlan
	if input.Profile.AutoReframe {
		var err error
		subjectReframe, err = buildSubjectReframePlan(input.SubjectTracks, input.SubjectSourceRevision, document.DurationSec)
		if err != nil {
			return Plan{}, err
		}
	}
	videoClips := make([]timelineClip, 0)
	videoTrackByClip := make(map[string]string)
	inputPaths := make([]string, 0)
	inputIndex := make(map[string]int)
	requiresAudio := false
	for _, track := range document.Tracks {
		if err := validateMix(track.Mix); err != nil {
			return Plan{}, fmt.Errorf("track %q: %w", track.ID, err)
		}
		if track.Kind == "narration" || track.Kind == "music" || track.Kind == "sfx" {
			requiresAudio = true
		}
		if track.Kind == "music" {
			for _, clip := range track.Clips {
				if clip.Source.RightsStatus != "owned" && clip.Source.RightsStatus != "cleared" {
					return Plan{}, fmt.Errorf("music clip %q is rejected by the BGM rights policy", clip.ID)
				}
				if len(clip.Source.RightsMetadata) == 0 {
					return Plan{}, fmt.Errorf("music clip %q is missing required rights metadata", clip.ID)
				}
			}
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
				path := filepath.Clean(input.ArtifactPaths[clip.Source.ArtifactID])
				if _, ok := inputIndex[path]; !ok {
					inputIndex[path] = len(inputPaths)
					inputPaths = append(inputPaths, path)
				}
				inputIndex[clip.Source.ArtifactID] = inputIndex[path]
			}
			if track.Kind == "video" {
				videoClips = append(videoClips, clip)
				videoTrackByClip[clip.ID] = track.ID
			}
			if track.Kind == "subtitle" && clip.Subtitle == nil {
				return Plan{}, fmt.Errorf("subtitle clip %q has no typed cue", clip.ID)
			}
			if track.Kind != "subtitle" && (clip.Text != "" || clip.Subtitle != nil) {
				return Plan{}, fmt.Errorf("clip %q contains text outside the subtitle track", clip.ID)
			}
			if clip.Transform != nil {
				return Plan{}, fmt.Errorf("clip %q uses an unsupported transform; only profile scale/pad is compiled", clip.ID)
			}
		}
	}
	if len(videoClips) == 0 {
		return Plan{}, errors.New("timeline must contain at least one video clip")
	}
	sort.SliceStable(videoClips, func(left, right int) bool { return videoClips[left].TimelineIn < videoClips[right].TimelineIn })
	if math.Abs(videoClips[0].TimelineIn) > 0.001 {
		return Plan{}, errors.New("video timeline must start at zero; gaps require an explicit generated background")
	}
	for index := 1; index < len(videoClips); index++ {
		if math.Abs(videoClips[index].TimelineIn-videoClips[index-1].TimelineOut) > 0.001 {
			return Plan{}, fmt.Errorf("video clips %q and %q are not contiguous; unsupported timeline gap/overlap", videoClips[index-1].ID, videoClips[index].ID)
		}
	}
	expectedDuration := 0.0
	for index, clip := range videoClips {
		expectedDuration += clip.TimelineOut - clip.TimelineIn
		if index > 0 {
			transitionDuration := transitionDurationFor(clip, videoClips[index-1])
			expectedDuration -= transitionDuration
		}
	}
	if expectedDuration <= 0 || math.Abs(expectedDuration-document.DurationSec) > 0.05 {
		return Plan{}, errors.New("video timeline duration does not match canonical TimelineVersion duration")
	}
	if strings.TrimSpace(input.OutputPath) == "" {
		return Plan{}, errors.New("render output must be a distinct executor-scoped path")
	}
	for _, path := range inputPaths {
		if filepath.Clean(input.OutputPath) == path {
			return Plan{}, errors.New("render output must be distinct from every source Artifact")
		}
	}
	for _, path := range input.SubtitleFiles {
		cleanPath := filepath.Clean(path)
		if filepath.Clean(input.OutputPath) == cleanPath {
			return Plan{}, errors.New("subtitle scratch file must be distinct from the render output")
		}
		if strings.ContainsAny(cleanPath, "\x00\r\n&|$") {
			return Plan{}, errors.New("subtitle scratch path is invalid")
		}
	}
	graph, videoLabel, audioLabel, err := compileFilterGraph(document, videoClips, inputIndex, input.Profile, input.SubtitleFiles, subjectReframe)
	if err != nil {
		return Plan{}, err
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}
	for _, path := range inputPaths {
		args = append(args, "-i", path)
	}
	args = append(args, "-filter_complex", graph, "-map", "["+videoLabel+"]")
	if audioLabel != "" {
		args = append(args, "-map", "["+audioLabel+"]")
	}
	args = append(args, "-c:v", input.Profile.VideoCodec, "-pix_fmt", input.Profile.PixelFormat, "-r", strconv.Itoa(input.Profile.FPS))
	if audioLabel != "" {
		args = append(args, "-c:a", input.Profile.AudioCodec)
	}
	args = append(args, "-movflags", "+faststart", input.OutputPath)
	outputPaths := []string{input.OutputPath}
	// This is deliberately profile-independent: a profile-only rerender may
	// create a new plan/output but must reuse every upstream AI/media Artifact.
	sourceFingerprint := sha256JSON(canonicalJSON(input.TimelineDocument))
	planFingerprint := sha256JSON(struct {
		Source             string
		Profile            RenderProfile
		Args               []string
		SubjectFingerprint string
	}{Source: sourceFingerprint, Profile: input.Profile, Args: args, SubjectFingerprint: subjectFingerprint(subjectReframe)})
	plan := Plan{Args: args, InputPaths: inputPaths, OutputPaths: outputPaths, OutputPath: input.OutputPath, SourceFingerprint: sourceFingerprint, PlanFingerprint: planFingerprint, ProfileSnapshot: input.Profile, ExpectedDurationSec: expectedDuration, RequiresAudio: requiresAudio || audioLabel != ""}
	if subjectReframe != nil {
		plan.SubjectReframeMode = "subject_aware"
		plan.SubjectTrackIDs = append([]string(nil), subjectReframe.TrackIDs...)
		plan.SubjectTrackFingerprint = subjectReframe.Fingerprint
	}
	return plan, nil
}

func compileFilterGraph(document timelineDocument, videoClips []timelineClip, inputIndex map[string]int, profile RenderProfile, subtitleFiles map[string]string, subjectReframe *subjectReframePlan) (string, string, string, error) {
	parts := make([]string, 0, len(videoClips)*2)
	videoLabels := make([]string, 0, len(videoClips))
	audioLabels := make([]string, 0, len(videoClips))
	durations := make([]float64, 0, len(videoClips))
	for index, clip := range videoClips {
		inputNumber := inputIndex[filepath.Clean(clip.Source.ArtifactID)]
		duration := clip.TimelineOut - clip.TimelineIn
		speed := clipSpeed(clip)
		durations = append(durations, duration)
		videoLabel := fmt.Sprintf("vclip%d", index)
		videoLabels = append(videoLabels, videoLabel)
		trimDuration := duration * speed
		if clip.SourceOut != nil && clip.SourceIn != nil {
			trimDuration = *clip.SourceOut - *clip.SourceIn
		}
		trim := fmt.Sprintf("trim=start=%s:duration=%s", formatSeconds(sourceIn(clip)), formatSeconds(trimDuration))
		videoFilter := fmt.Sprintf("[%d:v]%s,setpts=PTS-STARTPTS,setpts=PTS/%s,%s", inputNumber, trim, formatSeconds(speed), scaleFilter(profile, subjectReframe))
		if transition := clip.TransitionIn; transition != nil && transition.Kind == "fade" {
			videoFilter += fmt.Sprintf(",fade=t=in:st=0:d=%s", formatSeconds(transition.Duration))
		}
		if transition := clip.TransitionOut; transition != nil && transition.Kind == "fade" {
			videoFilter += fmt.Sprintf(",fade=t=out:st=%s:d=%s", formatSeconds(maxFloat(0, duration-transition.Duration)), formatSeconds(transition.Duration))
		}
		parts = append(parts, videoFilter+"["+videoLabel+"]")
		if inputNumber >= 0 {
			audioLabel := fmt.Sprintf("aclip%d", index)
			audioLabels = append(audioLabels, audioLabel)
			parts = append(parts, fmt.Sprintf("[%d:a]atrim=start=%s:duration=%s,asetpts=PTS-STARTPTS,asetpts=PTS/%s[%s]", inputNumber, formatSeconds(sourceIn(clip)), formatSeconds(trimDuration), formatSeconds(speed), audioLabel))
		}
	}
	videoOutput := "vcat"
	if hasCrossfade(videoClips) {
		current := videoLabels[0]
		accumulated := durations[0]
		for index := 1; index < len(videoLabels); index++ {
			transitionDuration := transitionDurationFor(videoClips[index], videoClips[index-1])
			if transitionDuration <= 0 {
				transitionDuration = 0.001
			}
			next := fmt.Sprintf("vx%d", index)
			parts = append(parts, fmt.Sprintf("[%s][%s]xfade=transition=fade:duration=%s:offset=%s[%s]", current, videoLabels[index], formatSeconds(transitionDuration), formatSeconds(maxFloat(0, accumulated-transitionDuration)), next))
			current = next
			accumulated += durations[index] - transitionDuration
		}
		videoOutput = current
	} else {
		concatInputs := ""
		for _, label := range videoLabels {
			concatInputs += "[" + label + "]"
		}
		parts = append(parts, fmt.Sprintf("%sconcat=n=%d:v=1:a=0[%s]", concatInputs, len(videoLabels), videoOutput))
	}
	subtitleIndex := 0
	for _, track := range document.Tracks {
		if track.Kind == "subtitle" {
			for _, clip := range track.Clips {
				if clip.Subtitle == nil {
					return "", "", "", fmt.Errorf("subtitle clip %q has no typed cue", clip.ID)
				}
				filter, err := drawTextFilter(*clip.Subtitle, clip.TimelineIn, clip.TimelineOut)
				if textPath := subtitleFiles[clip.ID]; textPath != "" {
					filter, err = drawTextFileFilter(*clip.Subtitle, clip.TimelineIn, clip.TimelineOut, textPath, profile.SubtitleFontPath)
				} else if profile.SubtitleFontPath != "" {
					filter, err = drawTextFilterWithFont(*clip.Subtitle, clip.TimelineIn, clip.TimelineOut, profile.SubtitleFontPath)
				}
				if err != nil {
					return "", "", "", fmt.Errorf("subtitle clip %q: %w", clip.ID, err)
				}
				next := fmt.Sprintf("vsubtitle%d", subtitleIndex)
				subtitleIndex++
				parts = append(parts, fmt.Sprintf("[%s]%s[%s]", videoOutput, filter, next))
				videoOutput = next
			}
		}
	}
	if len(audioLabels) == 0 {
		return strings.Join(parts, ";"), videoOutput, "", nil
	}
	audioOutput := ""
	if hasCrossfade(videoClips) {
		current := audioLabels[0]
		for index := 1; index < len(audioLabels); index++ {
			transitionDuration := transitionDurationFor(videoClips[index], videoClips[index-1])
			if transitionDuration <= 0 {
				transitionDuration = 0.001
			}
			next := fmt.Sprintf("ax%d", index)
			parts = append(parts, fmt.Sprintf("[%s][%s]acrossfade=d=%s:c1=tri:c2=tri[%s]", current, audioLabels[index], formatSeconds(transitionDuration), next))
			current = next
		}
		audioOutput = current
	} else {
		concatAudioInputs := ""
		for _, label := range audioLabels {
			concatAudioInputs += "[" + label + "]"
		}
		parts = append(parts, fmt.Sprintf("%sconcat=n=%d:v=0:a=1[acat]", concatAudioInputs, len(audioLabels)))
		audioOutput = "acat"
	}
	extraIndex := 0
	for trackIndex, track := range document.Tracks {
		if track.Kind != "narration" && track.Kind != "music" && track.Kind != "sfx" {
			continue
		}
		for _, clip := range track.Clips {
			inputNumber, ok := inputIndex[filepath.Clean(clip.Source.ArtifactID)]
			if !ok {
				continue
			}
			label := "extra" + clip.ID
			volume := 1.0
			gainDB := 0.0
			fadeIn := 0.0
			fadeOut := 0.0
			if track.Mix != nil {
				if track.Mix.Enabled != nil && !*track.Mix.Enabled {
					volume = 0
				} else if track.Mix.Volume > 0 {
					volume = track.Mix.Volume
				}
				gainDB = track.Mix.GainDB
				fadeIn = track.Mix.FadeInSec
				fadeOut = track.Mix.FadeOutSec
				if track.Kind == "music" && track.Mix.DuckDB < 0 {
					volume *= math.Pow(10, track.Mix.DuckDB/20)
				}
			}
			filter := fmt.Sprintf("[%d:a]atrim=start=%s:duration=%s,asetpts=PTS-STARTPTS,adelay=%d|%d,volume=%s", inputNumber, formatSeconds(sourceIn(clip)), formatSeconds(clip.TimelineOut-clip.TimelineIn), int(clip.TimelineIn*1000), int(clip.TimelineIn*1000), formatSeconds(volume))
			if gainDB != 0 {
				filter += fmt.Sprintf(",volume=%sdB", formatSeconds(gainDB))
			}
			if fadeIn > 0 {
				filter += fmt.Sprintf(",afade=t=in:st=0:d=%s", formatSeconds(fadeIn))
			}
			if fadeOut > 0 {
				filter += fmt.Sprintf(",afade=t=out:st=%s:d=%s", formatSeconds(maxFloat(0, clip.TimelineOut-clip.TimelineIn-fadeOut)), formatSeconds(fadeOut))
			}
			parts = append(parts, filter+"["+label+"]")
			mixedLabel := fmt.Sprintf("amixout%d", extraIndex+trackIndex)
			extraIndex++
			parts = append(parts, fmt.Sprintf("[%s][%s]amix=inputs=2:duration=longest:dropout_transition=0,loudnorm=I=%s:TP=%s:print_format=summary[%s]", audioOutput, label, formatSeconds(profile.AudioTargetLUFS), formatSeconds(profile.TruePeakDB), mixedLabel))
			audioOutput = mixedLabel
		}
	}
	return strings.Join(parts, ";"), videoOutput, audioOutput, nil
}

func clipSpeed(clip timelineClip) float64 {
	if clip.Speed <= 0 {
		return 1
	}
	return clip.Speed
}
func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
func hasCrossfade(clips []timelineClip) bool {
	for _, clip := range clips {
		if clip.TransitionIn != nil && clip.TransitionIn.Kind == "crossfade" || clip.TransitionOut != nil && clip.TransitionOut.Kind == "crossfade" {
			return true
		}
	}
	return false
}
func transitionDurationFor(current, previous timelineClip) float64 {
	if current.TransitionIn != nil && current.TransitionIn.Kind == "crossfade" {
		return current.TransitionIn.Duration
	}
	if previous.TransitionOut != nil && previous.TransitionOut.Kind == "crossfade" {
		return previous.TransitionOut.Duration
	}
	return 0
}
func Render(ctx context.Context, input CompileInput, policy process.Policy) (RenderResult, error) {
	if input.Profile.SubtitleFontPath != "" {
		if err := ensureSandboxPath(policy.SandboxRoot, input.Profile.SubtitleFontPath); err != nil {
			return RenderResult{}, fmt.Errorf("subtitle font: %w", err)
		}
	}
	subtitleFiles, err := prepareSubtitleFiles(input.TimelineDocument, policy.SandboxRoot)
	if err != nil {
		return RenderResult{}, err
	}
	for _, path := range subtitleFiles {
		defer os.Remove(path)
	}
	input.SubtitleFiles = subtitleFiles
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
	processResult, err := policy.Run(ctx, process.Spec{Tool: process.ToolFFmpeg, Args: plan.Args, InputPaths: plan.InputPaths, OutputPaths: plan.OutputPaths, Timeout: policy.MaxDuration})
	if err != nil {
		if processResult.Diagnostic != "" {
			return RenderResult{}, fmt.Errorf("render media process: %w: %s", err, processResult.Diagnostic)
		}
		return RenderResult{}, err
	}
	qa, err := InspectWithRequirements(ctx, plan.OutputPath, policy, plan.RequiresAudio)
	if err != nil {
		return RenderResult{}, err
	}
	if qa.Width != plan.ProfileSnapshot.Width || qa.Height != plan.ProfileSnapshot.Height {
		qa.Errors = append(qa.Errors, "profile_dimensions_mismatch")
	}
	if plan.RequiresAudio && !qa.HasAudio {
		qa.Errors = append(qa.Errors, "required_audio_stream_missing")
	}
	if qa.VideoCodec != expectedVideoCodec(plan.ProfileSnapshot.VideoCodec) {
		qa.Errors = append(qa.Errors, "profile_video_codec_mismatch")
	}
	if plan.RequiresAudio && qa.AudioCodec != plan.ProfileSnapshot.AudioCodec {
		qa.Errors = append(qa.Errors, "profile_audio_codec_mismatch")
	}
	if plan.ExpectedDurationSec > 0 && math.Abs(qa.DurationSec-plan.ExpectedDurationSec) > 0.15 {
		qa.Errors = append(qa.Errors, "timeline_duration_mismatch")
	}
	qa.Passed = len(qa.Errors) == 0
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
			"-vf", scaleFilter(request.Profile, nil), "-map", "0:v:0", "-map", "0:a?",
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
	return InspectWithRequirements(ctx, path, policy, false)
}

func InspectWithRequirements(ctx context.Context, path string, policy process.Policy, requireAudio bool) (QAReport, error) {
	baseQA := QAReport{BlackPolicy: defaultBlackContentPolicy, SilencePolicy: defaultSilencePolicy}
	if strings.TrimSpace(path) == "" {
		return QAReport{}, errors.New("deliverable path is required")
	}
	stat, err := os.Stat(path)
	if err != nil {
		return QAReport{}, fmt.Errorf("deliverable file is not readable: %w", err)
	}
	if stat.Size() <= 0 {
		baseQA.Errors = []string{"deliverable_empty"}
		return baseQA, nil
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
		baseQA.Errors = []string{"duration_missing_or_invalid"}
		return baseQA, nil
	}
	qa := QAReport{DurationSec: duration, BlackPolicy: defaultBlackContentPolicy, SilencePolicy: defaultSilencePolicy}
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
	if requireAudio && !qa.HasAudio {
		qa.Errors = append(qa.Errors, "required_audio_stream_missing")
	}
	if qa.Width <= 0 || qa.Height <= 0 {
		qa.Errors = append(qa.Errors, "video_dimensions_missing")
	}
	if qa.HasVideo {
		segments, err := detectBlackSegments(ctx, path, policy, duration, qa.BlackPolicy)
		if err != nil {
			return QAReport{}, fmt.Errorf("black-content QA: %w", err)
		}
		qa.BlackSegments = segments
		qa.BlackDetected = len(segments) > 0
		for _, segment := range segments {
			qa.BlackDurationSec += segment.DurationSec
			if segment.DurationSec > qa.BlackPolicy.MaximumContinuousSec+0.001 {
				qa.Errors = append(qa.Errors, "excessive_black_duration")
			}
		}
		qa.BlackRatio = boundedRatio(qa.BlackDurationSec, duration)
		if qa.BlackRatio > qa.BlackPolicy.MaximumRatio+0.001 {
			qa.Errors = append(qa.Errors, "excessive_black_content")
		}
	}
	if qa.HasAudio {
		segments, err := detectSilenceSegments(ctx, path, policy, duration, qa.SilencePolicy)
		if err != nil {
			return QAReport{}, fmt.Errorf("silence QA: %w", err)
		}
		qa.SilenceSegments = segments
		qa.SilenceDetected = len(segments) > 0
		for _, segment := range segments {
			qa.SilenceDurationSec += segment.DurationSec
			if requireAudio && segment.DurationSec > qa.SilencePolicy.MaximumContinuousSec+0.001 {
				qa.Errors = append(qa.Errors, "excessive_silence_duration")
			}
		}
		qa.SilenceRatio = boundedRatio(qa.SilenceDurationSec, duration)
		if requireAudio && qa.SilenceRatio > qa.SilencePolicy.MaximumRatio+0.001 {
			qa.Errors = append(qa.Errors, "excessive_silence_content")
		}
	}
	qa.Passed = len(qa.Errors) == 0
	return qa, nil
}

func detectBlackSegments(ctx context.Context, path string, policy process.Policy, duration float64, blackPolicy BlackContentPolicy) ([]MediaSegment, error) {
	result, err := policy.Run(ctx, process.Spec{
		Tool:       process.ToolFFmpeg,
		Args:       []string{"-hide_banner", "-loglevel", "info", "-i", path, "-vf", fmt.Sprintf("blackdetect=d=%s:pix_th=%s", formatSeconds(blackPolicy.MinimumSegmentSec), formatSeconds(blackPolicy.PixelThreshold)), "-an", "-f", "null", "-"},
		InputPaths: []string{path},
		Timeout:    policy.MaxDuration,
	})
	if err != nil {
		return nil, err
	}
	segments := make([]MediaSegment, 0)
	for _, line := range strings.Split(result.Diagnostic, "\n") {
		start, end, ok := parseBlackSegment(line)
		if !ok {
			continue
		}
		segment, ok := normalizedSegment(start, end, duration, blackPolicy.MinimumSegmentSec)
		if ok {
			segments = append(segments, segment)
		}
	}
	return mergeSegments(segments), nil
}

func detectSilenceSegments(ctx context.Context, path string, policy process.Policy, duration float64, silencePolicy SilencePolicy) ([]MediaSegment, error) {
	result, err := policy.Run(ctx, process.Spec{
		Tool:       process.ToolFFmpeg,
		Args:       []string{"-hide_banner", "-loglevel", "info", "-i", path, "-af", fmt.Sprintf("silencedetect=noise=%sdB:d=%s", formatSeconds(silencePolicy.NoiseThresholdDB), formatSeconds(silencePolicy.MinimumSegmentSec)), "-vn", "-f", "null", "-"},
		InputPaths: []string{path},
		Timeout:    policy.MaxDuration,
	})
	if err != nil {
		return nil, err
	}
	startPattern := regexp.MustCompile(`silence_start:\s*([0-9]+(?:\.[0-9]+)?)`)
	endPattern := regexp.MustCompile(`silence_end:\s*([0-9]+(?:\.[0-9]+)?)`)
	segments := make([]MediaSegment, 0)
	var start *float64
	for _, line := range strings.Split(result.Diagnostic, "\n") {
		if match := startPattern.FindStringSubmatch(line); len(match) == 2 {
			value, parseErr := strconv.ParseFloat(match[1], 64)
			if parseErr == nil {
				start = &value
			}
		}
		if match := endPattern.FindStringSubmatch(line); len(match) == 2 && start != nil {
			end, parseErr := strconv.ParseFloat(match[1], 64)
			if parseErr == nil {
				if segment, ok := normalizedSegment(*start, end, duration, silencePolicy.MinimumSegmentSec); ok {
					segments = append(segments, segment)
				}
			}
			start = nil
		}
	}
	if start != nil {
		if segment, ok := normalizedSegment(*start, duration, duration, silencePolicy.MinimumSegmentSec); ok {
			segments = append(segments, segment)
		}
	}
	return mergeSegments(segments), nil
}

func parseBlackSegment(line string) (float64, float64, bool) {
	startPattern := regexp.MustCompile(`black_start:\s*([0-9]+(?:\.[0-9]+)?)`)
	endPattern := regexp.MustCompile(`black_end:\s*([0-9]+(?:\.[0-9]+)?)`)
	startMatch := startPattern.FindStringSubmatch(line)
	endMatch := endPattern.FindStringSubmatch(line)
	if len(startMatch) != 2 || len(endMatch) != 2 {
		return 0, 0, false
	}
	start, startErr := strconv.ParseFloat(startMatch[1], 64)
	end, endErr := strconv.ParseFloat(endMatch[1], 64)
	return start, end, startErr == nil && endErr == nil
}

func normalizedSegment(start, end, duration, minimum float64) (MediaSegment, bool) {
	if !finite(start) || !finite(end) || !finite(duration) || duration <= 0 {
		return MediaSegment{}, false
	}
	start = math.Max(0, math.Min(start, duration))
	end = math.Max(0, math.Min(end, duration))
	if end <= start || end-start+0.001 < minimum {
		return MediaSegment{}, false
	}
	return MediaSegment{StartSec: start, EndSec: end, DurationSec: end - start}, true
}

func mergeSegments(segments []MediaSegment) []MediaSegment {
	if len(segments) < 2 {
		return segments
	}
	sort.Slice(segments, func(left, right int) bool { return segments[left].StartSec < segments[right].StartSec })
	merged := make([]MediaSegment, 0, len(segments))
	for _, segment := range segments {
		if len(merged) == 0 || segment.StartSec > merged[len(merged)-1].EndSec+0.001 {
			merged = append(merged, segment)
			continue
		}
		last := &merged[len(merged)-1]
		last.EndSec = math.Max(last.EndSec, segment.EndSec)
		last.DurationSec = last.EndSec - last.StartSec
	}
	return merged
}

func boundedRatio(value, duration float64) float64 {
	if duration <= 0 {
		return 0
	}
	return math.Max(0, math.Min(1, value/duration))
}

func buildSubjectReframePlan(tracks []SubjectTrack, expectedSourceRevision string, timelineDuration float64) (*subjectReframePlan, error) {
	if len(tracks) == 0 {
		return nil, errors.New("subject-aware reframe requires typed subject tracks")
	}
	seen := make(map[string]struct{}, len(tracks))
	sourceRevision := ""
	centerX, centerY := 0.0, 0.0
	pointCount := 0
	trackIDs := make([]string, 0, len(tracks))
	for _, track := range tracks {
		if track.ID == "" || track.SourceRevision == "" || len(track.Points) == 0 {
			return nil, errors.New("subject-aware reframe subject track is incomplete")
		}
		if _, exists := seen[track.ID]; exists {
			return nil, errors.New("subject-aware reframe subject track IDs must be unique")
		}
		seen[track.ID] = struct{}{}
		if sourceRevision == "" {
			sourceRevision = track.SourceRevision
		}
		if track.SourceRevision != sourceRevision || (expectedSourceRevision != "" && track.SourceRevision != expectedSourceRevision) {
			return nil, errors.New("subject-aware reframe source revision mismatch")
		}
		previousTime := -1.0
		for _, point := range track.Points {
			values := []float64{point.TimeSec, point.X, point.Y, point.Width, point.Height}
			for _, value := range values {
				if !finite(value) {
					return nil, errors.New("subject-aware reframe point contains a non-finite value")
				}
			}
			if point.TimeSec < 0 || point.TimeSec > timelineDuration+0.001 || point.TimeSec < previousTime-0.001 || point.Width <= 0 || point.Height <= 0 || point.X < 0 || point.Y < 0 || point.X+point.Width > 1.000001 || point.Y+point.Height > 1.000001 {
				return nil, errors.New("subject-aware reframe point coordinates or time ordering are invalid")
			}
			previousTime = point.TimeSec
			centerX += point.X + point.Width/2
			centerY += point.Y + point.Height/2
			pointCount++
		}
		trackIDs = append(trackIDs, track.ID)
	}
	sort.Strings(trackIDs)
	centerX /= float64(pointCount)
	centerY /= float64(pointCount)
	fingerprint := sha256JSON(struct {
		SourceRevision string
		Tracks         []SubjectTrack
		CenterX        float64
		CenterY        float64
	}{sourceRevision, tracks, centerX, centerY})
	return &subjectReframePlan{CenterX: centerX, CenterY: centerY, TrackIDs: trackIDs, Fingerprint: fingerprint}, nil
}

func subjectFingerprint(plan *subjectReframePlan) string {
	if plan == nil {
		return ""
	}
	return plan.Fingerprint
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
	if clip.ID == "" || clip.Origin == "" || !finite(clip.TimelineIn) || !finite(clip.TimelineOut) || clip.TimelineIn < 0 || clip.TimelineOut <= clip.TimelineIn || clip.TimelineOut > timelineDuration+0.001 {
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
	if !finite(clip.Speed) || clip.Speed < 0 || clip.Speed > 16 {
		return errors.New("clip speed is invalid")
	}
	if clip.SourceIn != nil && (!finite(*clip.SourceIn) || *clip.SourceIn < 0) {
		return errors.New("source range is invalid")
	}
	if clip.SourceOut != nil && (!finite(*clip.SourceOut) || *clip.SourceOut <= 0 || clip.SourceIn == nil || *clip.SourceOut <= *clip.SourceIn) {
		return errors.New("source range is invalid")
	}
	if clip.SourceIn != nil && clip.SourceOut != nil && !clip.Loop && !clip.FreezeFrame {
		effectiveSpeed := clip.Speed
		if effectiveSpeed <= 0 {
			effectiveSpeed = 1
		}
		expectedSourceDuration := (clip.TimelineOut - clip.TimelineIn) * effectiveSpeed
		if math.Abs((*clip.SourceOut-*clip.SourceIn)-expectedSourceDuration) > 0.05 {
			return errors.New("source range duration must match timeline duration unless loop or freeze_frame is explicit")
		}
	}
	return nil
}

func validateTransition(value transition, duration float64) error {
	switch value.Kind {
	case "cut", "fade", "crossfade":
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

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func expectedVideoCodec(codec string) string {
	if codec == "libx264" {
		return "h264"
	}
	return codec
}

func scaleFilter(profile RenderProfile, subjectReframe *subjectReframePlan) string {
	if subjectReframe != nil {
		return subjectCropFilter(profile, subjectReframe.CenterX, subjectReframe.CenterY)
	}
	return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:0:0:color=black", profile.Width, profile.Height, profile.Width, profile.Height)
}

func subjectCropFilter(profile RenderProfile, centerX, centerY float64) string {
	ratio := formatSeconds(float64(profile.Width) / float64(profile.Height))
	width := fmt.Sprintf("if(gt(iw/ih\\,%s)\\,ih*%s\\,iw)", ratio, ratio)
	height := fmt.Sprintf("if(gt(iw/ih\\,%s)\\,ih\\,iw/%s)", ratio, ratio)
	x := fmt.Sprintf("clip(%s*iw-ow/2\\,0\\,iw-ow)", formatSeconds(centerX))
	y := fmt.Sprintf("clip(%s*ih-oh/2\\,0\\,ih-oh)", formatSeconds(centerY))
	return fmt.Sprintf("crop=w=%s:h=%s:x=%s:y=%s,scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:0:0:color=black", width, height, x, y, profile.Width, profile.Height, profile.Width, profile.Height)
}

func drawTextFilterLegacy(cue subtitleCue, in, out float64) (string, error) {
	if cue.Language == "" || cue.Text == "" || strings.ContainsAny(cue.Text, "'\\\n\r;|$`()") {
		return "", errors.New("subtitle text contains unsafe characters")
	}
	escaped := strings.NewReplacer("\\", "\\\\", "'", "\\'", ":", "\\:", ";", "\\;", "[", "\\[", "]", "\\]").Replace(cue.Text)
	return fmt.Sprintf("drawtext=text='%s':enable=between(t\\,%s\\,%s)", escaped, formatSeconds(in), formatSeconds(out)), nil
}

func drawTextFilter(cue subtitleCue, in, out float64) (string, error) {
	if cue.Language == "" || cue.Text == "" || strings.ContainsAny(cue.Text, "\x00\n\r$()&|") {
		return "", errors.New("subtitle text contains unsafe characters")
	}
	escaped := strings.NewReplacer("\\", "\\\\", "'", "\\'", ":", "\\:", ";", "\\;", "[", "\\[", "]", "\\]").Replace(cue.Text)
	return fmt.Sprintf("drawtext=text='%s':enable=between(t\\,%s\\,%s)", escaped, formatSeconds(in), formatSeconds(out)), nil
}

func drawTextFilterWithFont(cue subtitleCue, in, out float64, fontPath string) (string, error) {
	filter, err := drawTextFilter(cue, in, out)
	if err != nil {
		return "", err
	}
	font := strings.NewReplacer("\\", "/", ":", "\\:", "'", "\\'").Replace(fontPath)
	return strings.Replace(filter, "drawtext=", "drawtext=fontfile='"+font+"':", 1), nil
}

func drawTextFileFilter(cue subtitleCue, in, out float64, textPath, fontPath string) (string, error) {
	if cue.Language == "" || cue.Text == "" || strings.ContainsAny(textPath, "\x00\r\n&|$") {
		return "", errors.New("subtitle text file boundary is invalid")
	}
	textFile := strings.NewReplacer("\\", "/", ":", "\\:", "'", "\\'").Replace(textPath)
	fontPrefix := ""
	if fontPath != "" {
		font := strings.NewReplacer("\\", "/", ":", "\\:", "'", "\\'").Replace(fontPath)
		fontPrefix = "fontfile='" + font + "':"
	}
	return fmt.Sprintf("drawtext=%stextfile='%s':reload=0:enable=between(t\\,%s\\,%s)", fontPrefix, textFile, formatSeconds(in), formatSeconds(out)), nil
}

func prepareSubtitleFiles(documentBytes []byte, sandboxRoot string) (map[string]string, error) {
	files := make(map[string]string)
	if len(documentBytes) == 0 || strings.TrimSpace(sandboxRoot) == "" {
		return files, nil
	}
	var document timelineDocument
	decoder := json.NewDecoder(bytes.NewReader(documentBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("timeline document is invalid: %w", err)
	}
	index := 0
	for _, track := range document.Tracks {
		if track.Kind != "subtitle" {
			continue
		}
		for _, clip := range track.Clips {
			if clip.Subtitle == nil {
				continue
			}
			path := filepath.Join(sandboxRoot, fmt.Sprintf("subtitle-%03d.txt", index))
			if err := ensureSandboxPath(sandboxRoot, path); err != nil {
				return nil, err
			}
			if err := os.WriteFile(path, []byte(clip.Subtitle.Text), 0o600); err != nil {
				return nil, fmt.Errorf("prepare subtitle scratch: %w", err)
			}
			files[clip.ID] = path
			index++
		}
	}
	return files, nil
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
