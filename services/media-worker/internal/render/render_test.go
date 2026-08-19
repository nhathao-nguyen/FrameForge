package render

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/media-worker/internal/process"
)

func fixtureTimeline() []byte {
	return []byte(`{"schema_version":"1.0","timeline_id":"timeline_gate_g","timeline_version_id":"timeline_version_gate_g","project_id":"project_gate_g","version":1,"duration_sec":1,"tracks":[{"id":"video_1","kind":"video","name":"Video","order":1,"clips":[{"id":"clip_1","timeline_in_sec":0,"timeline_out_sec":1,"source":{"type":"artifact","artifact_id":"source_artifact"},"source_in_sec":0,"source_out_sec":1,"origin":"ai","transition_out":{"kind":"fade","duration_sec":0.1}}]},{"id":"subtitle_1","kind":"subtitle","name":"Subtitles","order":2,"clips":[{"id":"subtitle_clip_1","timeline_in_sec":0,"timeline_out_sec":1,"source":{"type":"none","inline_id":"subtitle_1"},"origin":"system","subtitle":{"text":"Gate G","language":"en"}}]}]}`)
}

func fixtureRenderTimeline() []byte {
	return []byte(`{"schema_version":"1.0","timeline_id":"timeline_gate_g","timeline_version_id":"timeline_version_gate_g","project_id":"project_gate_g","version":1,"duration_sec":1,"tracks":[{"id":"video_1","kind":"video","name":"Video","order":1,"clips":[{"id":"clip_1","timeline_in_sec":0,"timeline_out_sec":1,"source":{"type":"artifact","artifact_id":"source_artifact"},"source_in_sec":0,"source_out_sec":1,"origin":"ai","transition_out":{"kind":"fade","duration_sec":0.1}}]}]}`)
}

func fixtureMultiClipTimeline() []byte {
	return []byte(`{"schema_version":"1.0","timeline_id":"timeline_multi","timeline_version_id":"timeline_multi_v1","project_id":"project_gate_g","version":1,"duration_sec":3,"tracks":[{"id":"video_1","kind":"video","name":"Video","order":1,"clips":[{"id":"clip_1","timeline_in_sec":0,"timeline_out_sec":1,"source":{"type":"artifact","artifact_id":"source_1"},"source_in_sec":0,"source_out_sec":1,"origin":"ai","transition_out":{"kind":"fade","duration_sec":0.1}},{"id":"clip_2","timeline_in_sec":1,"timeline_out_sec":2,"source":{"type":"artifact","artifact_id":"source_2"},"source_in_sec":0,"source_out_sec":1,"origin":"user","transition_in":{"kind":"fade","duration_sec":0.1}},{"id":"clip_3","timeline_in_sec":2,"timeline_out_sec":3,"source":{"type":"artifact","artifact_id":"source_3"},"source_in_sec":0,"source_out_sec":1,"origin":"ai"}]},{"id":"narration_1","kind":"narration","name":"Narration","order":2,"mix":{"volume":0.35,"gain_db":-1,"fade_in_sec":0.05,"fade_out_sec":0.1},"clips":[{"id":"narration_clip","timeline_in_sec":0,"timeline_out_sec":3,"source":{"type":"artifact","artifact_id":"source_1"},"origin":"system"}]},{"id":"subtitle_1","kind":"subtitle","name":"Subtitles","order":3,"clips":[{"id":"subtitle_clip_1","timeline_in_sec":0.1,"timeline_out_sec":0.9,"source":{"type":"none","inline_id":"cue_1"},"origin":"system","subtitle":{"text":"Xin chào: 'Gate\\\\G'; \u4e16\u754c","language":"vi"}}]}]}`)
}

func fixtureProfile(t *testing.T, key string) RenderProfile {
	t.Helper()
	profile, err := DefaultProfile(key)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func TestCompileIsDeterministicAndProviderFree(t *testing.T) {
	profile := fixtureProfile(t, "youtube_16_9")
	input := CompileInput{TimelineDocument: fixtureTimeline(), Profile: profile, ArtifactPaths: map[string]string{"source_artifact": "source.mp4"}, OutputPath: "output.mp4"}
	first, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	if first.PlanFingerprint != second.PlanFingerprint || first.SourceFingerprint != second.SourceFingerprint || first.ProviderCallCount != 0 || first.RematchingPerformed {
		t.Fatalf("compiler was not deterministic/provider-free: %+v %+v", first, second)
	}
	if strings.Contains(strings.Join(first.Args, "\x00"), "matches.json") || strings.Contains(strings.Join(first.Args, "\x00"), "http") {
		t.Fatal("compiled plan contains an external decision dependency")
	}
}

func TestCompileRejectsInjectionAndMissingSubjectAwareReframe(t *testing.T) {
	profile := fixtureProfile(t, "youtube_16_9")
	document := strings.Replace(string(fixtureTimeline()), "Gate G", "Gate G$(whoami)", 1)
	_, err := Compile(CompileInput{TimelineDocument: []byte(document), Profile: profile, ArtifactPaths: map[string]string{"source_artifact": "source.mp4"}, OutputPath: "output.mp4"})
	if err == nil {
		t.Fatal("unsafe subtitle text was accepted")
	}
	profile.AutoReframe = true
	_, err = Compile(CompileInput{TimelineDocument: fixtureTimeline(), Profile: profile, ArtifactPaths: map[string]string{"source_artifact": "source.mp4"}, OutputPath: "output.mp4"})
	if err == nil || !strings.Contains(err.Error(), "subject-aware") {
		t.Fatalf("missing subject-aware reframe was not rejected: %v", err)
	}
}

func TestCompileSubjectAwareReframeUsesTrackCoordinates(t *testing.T) {
	profile := fixtureProfile(t, "shorts_9_16")
	profile.AutoReframe = true
	left := []SubjectTrack{{ID: "subject-left", SourceRevision: "source-v1", Points: []SubjectPoint{{TimeSec: 0, X: 0.05, Y: 0.2, Width: 0.1, Height: 0.3}}}}
	right := []SubjectTrack{{ID: "subject-right", SourceRevision: "source-v1", Points: []SubjectPoint{{TimeSec: 0, X: 0.85, Y: 0.2, Width: 0.1, Height: 0.3}}}}
	leftPlan, err := Compile(CompileInput{TimelineDocument: fixtureRenderTimeline(), Profile: profile, ArtifactPaths: map[string]string{"source_artifact": "source.mp4"}, OutputPath: "left.mp4", SubjectTracks: left})
	if err != nil {
		t.Fatal(err)
	}
	rightPlan, err := Compile(CompileInput{TimelineDocument: fixtureRenderTimeline(), Profile: profile, ArtifactPaths: map[string]string{"source_artifact": "source.mp4"}, OutputPath: "right.mp4", SubjectTracks: right})
	if err != nil {
		t.Fatal(err)
	}
	if leftPlan.PlanFingerprint == rightPlan.PlanFingerprint || leftPlan.SubjectTrackFingerprint == rightPlan.SubjectTrackFingerprint || leftPlan.SourceFingerprint != rightPlan.SourceFingerprint {
		t.Fatalf("subject track coordinates did not change only the profile render plan: left=%+v right=%+v", leftPlan, rightPlan)
	}
	joinedLeft := strings.Join(leftPlan.Args, "\x00")
	joinedRight := strings.Join(rightPlan.Args, "\x00")
	if !strings.Contains(joinedLeft, "crop=") || joinedLeft == joinedRight || leftPlan.SubjectReframeMode != "subject_aware" {
		t.Fatalf("subject-aware crop was not compiled from track coordinates: left=%s right=%s", joinedLeft, joinedRight)
	}
	invalid := left
	invalid[0].Points = []SubjectPoint{{TimeSec: 0, X: 0.9, Y: 0.9, Width: 0.2, Height: 0.1}}
	if _, err := Compile(CompileInput{TimelineDocument: fixtureRenderTimeline(), Profile: profile, ArtifactPaths: map[string]string{"source_artifact": "source.mp4"}, OutputPath: "invalid.mp4", SubjectTracks: invalid}); err == nil {
		t.Fatal("invalid normalized subject coordinates were accepted")
	}
	if _, err := Compile(CompileInput{TimelineDocument: fixtureRenderTimeline(), Profile: profile, ArtifactPaths: map[string]string{"source_artifact": "source.mp4"}, OutputPath: "mismatch.mp4", SubjectTracks: []SubjectTrack{{ID: "one", SourceRevision: "source-v1", Points: []SubjectPoint{{TimeSec: 1, X: 0.1, Y: 0.1, Width: 0.1, Height: 0.1}}}, {ID: "two", SourceRevision: "source-v2", Points: []SubjectPoint{{TimeSec: 1, X: 0.2, Y: 0.1, Width: 0.1, Height: 0.1}}}}}); err == nil {
		t.Fatal("mismatched subject source revisions were accepted")
	}
}

func TestCompileSupportsMultiClipTransitionsAudioAndSafeSubtitles(t *testing.T) {
	profile := fixtureProfile(t, "youtube_16_9")
	plan, err := Compile(CompileInput{TimelineDocument: fixtureMultiClipTimeline(), Profile: profile, ArtifactPaths: map[string]string{"source_1": "source_1.mp4", "source_2": "source_2.mp4", "source_3": "source_3.mp4"}, OutputPath: "output.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Args, "\x00")
	for _, required := range []string{"concat=n=3", "fade=t=out", "fade=t=in", "amix=inputs=2", "-shortest", "drawtext=text='Xin chào\\:", "\\; 世界"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("compiled plan omitted %q: %s", required, joined)
		}
	}
	if plan.ProviderCallCount != 0 || plan.RematchingPerformed || !plan.RequiresAudio || len(plan.InputPaths) != 3 {
		t.Fatalf("multi-clip plan crossed a provider boundary or lost audio inputs: %+v", plan)
	}
}

func TestRenderRealMediaAndReuseThreeProfiles(t *testing.T) {
	ffmpeg := os.Getenv("NH_MEDIA_FFMPEG_PATH")
	ffprobe := os.Getenv("NH_MEDIA_FFPROBE_PATH")
	if ffmpeg == "" || ffprobe == "" {
		t.Fatal("Gate G real-media proof requires reviewed NH_MEDIA_FFMPEG_PATH and NH_MEDIA_FFPROBE_PATH; unexpected skip")
	}
	if _, err := os.Stat(ffmpeg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ffprobe); err != nil {
		t.Fatal(err)
	}
	sandbox := t.TempDir()
	policy := process.Policy{FFmpegPath: ffmpeg, FFprobePath: ffprobe, SandboxRoot: sandbox, MaxOutputBytes: 2 << 20, MaxDuration: 2 * time.Minute}
	source := filepath.Join(sandbox, "source.mp4")
	_, err := policy.Run(context.Background(), process.Spec{Tool: process.ToolFFmpeg, Args: []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "color=c=blue:s=320x180:d=1", "-f", "lavfi", "-i", "sine=frequency=800:duration=1", "-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", source}, OutputPaths: []string{source}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	var previousSource string
	for _, key := range []string{"youtube_16_9", "shorts_9_16", "square_1_1"} {
		profile := fixtureProfile(t, key)
		output := filepath.Join(sandbox, key+".mp4")
		result, err := Render(context.Background(), CompileInput{TimelineDocument: fixtureRenderTimeline(), Profile: profile, ArtifactPaths: map[string]string{"source_artifact": source}, OutputPath: output}, policy)
		if err != nil {
			t.Fatal(err)
		}
		if result.Width != profile.Width || result.Height != profile.Height || !result.HasVideo || !result.HasAudio || !result.QAReport.Passed || result.SizeBytes <= 0 {
			t.Fatalf("profile render failed QA: %+v", result)
		}
		if previousSource != "" && previousSource != result.Plan.SourceFingerprint {
			t.Fatal("profile-only rerender changed upstream fingerprint")
		}
		previousSource = result.Plan.SourceFingerprint
	}
	shortProfile := fixtureProfile(t, "shorts_9_16")
	clipOutput := filepath.Join(sandbox, "clip-short.mp4")
	manifest, err := ExportClips(context.Background(), []ClipExportRequest{{
		SelectionID: "selection-short-01",
		SourcePath:  source,
		StartSec:    0.2,
		EndSec:      0.8,
		OutputPath:  clipOutput,
		Profile:     shortProfile,
	}}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != "1.0" || len(manifest.Exports) != 1 || manifest.SourceFingerprint == "" {
		t.Fatalf("clip manifest is incomplete: %+v", manifest)
	}
	clip := manifest.Exports[0]
	if clip.SelectionID != "selection-short-01" || clip.StartSec != 0.2 || clip.EndSec != 0.8 || clip.Width != 360 || clip.Height != 640 || clip.VideoCodec != "h264" || clip.AudioCodec != "aac" || clip.SizeBytes <= 0 || !clip.QAReport.Passed {
		t.Fatalf("clip export did not satisfy selection/profile/codec QA: %+v", clip)
	}
	content, err := os.ReadFile(clipOutput)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	if clip.SHA256 != hex.EncodeToString(digest[:]) || clip.SourceFingerprint != manifest.SourceFingerprint {
		t.Fatalf("clip checksum manifest mismatch: %+v", clip)
	}
}

func TestRenderRealMultiClipMovieWithAudioAndSubtitles(t *testing.T) {
	ffmpeg := os.Getenv("NH_MEDIA_FFMPEG_PATH")
	ffprobe := os.Getenv("NH_MEDIA_FFPROBE_PATH")
	if ffmpeg == "" || ffprobe == "" {
		t.Fatal("Gate G real-media proof requires reviewed NH_MEDIA_FFMPEG_PATH and NH_MEDIA_FFPROBE_PATH; unexpected skip")
	}
	sandbox := t.TempDir()
	policy := process.Policy{FFmpegPath: ffmpeg, FFprobePath: ffprobe, SandboxRoot: sandbox, MaxOutputBytes: 4 << 20, MaxDuration: 2 * time.Minute}
	paths := []string{filepath.Join(sandbox, "source_1.mp4"), filepath.Join(sandbox, "source_2.mp4"), filepath.Join(sandbox, "source_3.mp4")}
	colors := []string{"blue", "red", "green"}
	for index, source := range paths {
		_, err := policy.Run(context.Background(), process.Spec{Tool: process.ToolFFmpeg, Args: []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "color=c=" + colors[index] + ":s=320x180:d=1", "-f", "lavfi", "-i", "sine=frequency=" + strconv.Itoa(700+index*100) + ":duration=1", "-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", source}, OutputPaths: []string{source}, Timeout: time.Minute})
		if err != nil {
			t.Fatal(err)
		}
	}
	artifactPaths := map[string]string{"source_1": paths[0], "source_2": paths[1], "source_3": paths[2]}
	output := filepath.Join(sandbox, "multi.mp4")
	fontPath := filepath.Join(sandbox, "arial.ttf")
	fontBytes, err := os.ReadFile(`C:\Windows\Fonts\arial.ttf`)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fontPath, fontBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	profile := fixtureProfile(t, "youtube_16_9")
	profile.SubtitleFontPath = fontPath
	compileInput := CompileInput{TimelineDocument: fixtureMultiClipTimeline(), Profile: profile, ArtifactPaths: artifactPaths, OutputPath: output}
	plan, err := Compile(compileInput)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ExpectedDurationSec != 3 || !plan.RequiresAudio {
		t.Fatalf("compiled plan has incorrect duration/audio contract: %+v", plan)
	}
	result, err := Render(context.Background(), compileInput, policy)
	if err != nil {
		t.Fatal(err)
	}
	if !result.QAReport.Passed || !result.HasVideo || !result.HasAudio || result.Width != 640 || result.Height != 360 || result.DurationSec < 2.8 || result.DurationSec > 3.2 || result.SizeBytes <= 0 || result.SHA256 == "" {
		t.Fatalf("multi-clip deliverable QA failed: %+v", result)
	}
}

func TestInspectRejectsMissingVideo(t *testing.T) {
	if os.Getenv("NH_MEDIA_FFMPEG_PATH") == "" || os.Getenv("NH_MEDIA_FFPROBE_PATH") == "" {
		t.Fatal("Gate G real-media proof requires reviewed NH_MEDIA_FFMPEG_PATH and NH_MEDIA_FFPROBE_PATH; unexpected skip")
	}
	// A nonexistent input must fail before any canonical Artifact claim.
	policy := process.Policy{FFmpegPath: os.Getenv("NH_MEDIA_FFMPEG_PATH"), FFprobePath: os.Getenv("NH_MEDIA_FFPROBE_PATH"), SandboxRoot: t.TempDir(), MaxOutputBytes: 1 << 20, MaxDuration: time.Second}
	_, err := Inspect(context.Background(), filepath.Join(policy.SandboxRoot, "missing.mp4"), policy)
	if err == nil {
		t.Fatal("missing deliverable unexpectedly passed inspection")
	}
}

func TestDeliverableQARealBlackAndSilencePolicies(t *testing.T) {
	ffmpeg := os.Getenv("NH_MEDIA_FFMPEG_PATH")
	ffprobe := os.Getenv("NH_MEDIA_FFPROBE_PATH")
	if ffmpeg == "" || ffprobe == "" {
		t.Fatal("Gate G real-media proof requires reviewed NH_MEDIA_FFMPEG_PATH and NH_MEDIA_FFPROBE_PATH; unexpected skip")
	}
	sandbox := t.TempDir()
	policy := process.Policy{FFmpegPath: ffmpeg, FFprobePath: ffprobe, SandboxRoot: sandbox, MaxOutputBytes: 4 << 20, MaxDuration: time.Minute}
	normal := makeQAFixture(t, policy, "normal.mp4", "color=c=blue:s=320x180:d=1", "sine=frequency=880:duration=1", "", "")
	qa, err := InspectWithRequirements(context.Background(), normal, policy, true)
	if err != nil || !qa.Passed || qa.BlackDetected || qa.SilenceDetected || qa.BlackPolicy.MaximumRatio != 0.35 || qa.SilencePolicy.NoiseThresholdDB != -50 {
		t.Fatalf("normal audible deliverable did not pass typed QA: qa=%+v err=%v", qa, err)
	}
	black := makeQAFixture(t, policy, "black.mp4", "color=c=black:s=320x180:d=1", "sine=frequency=880:duration=1", "", "")
	qa, err = InspectWithRequirements(context.Background(), black, policy, true)
	if err != nil || qa.Passed || !qa.BlackDetected || qa.BlackRatio < 0.9 || !containsError(qa.Errors, "excessive_black_content") {
		t.Fatalf("fully black video was not rejected by real blackdetect: qa=%+v err=%v", qa, err)
	}
	longBlack := makeQAFixtureDuration(t, policy, "long-black.mp4", "color=c=black:s=320x180:d=2", "sine=frequency=880:duration=2", "", "", "2")
	qa, err = InspectWithRequirements(context.Background(), longBlack, policy, true)
	if err != nil || qa.Passed || !containsError(qa.Errors, "excessive_black_duration") {
		t.Fatalf("excessive continuous black duration was not rejected: qa=%+v err=%v", qa, err)
	}
	fade := makeQAFixture(t, policy, "fade.mp4", "color=c=blue:s=320x180:d=1", "sine=frequency=880:duration=1", "fade=t=out:st=0.8:d=0.2", "")
	qa, err = InspectWithRequirements(context.Background(), fade, policy, true)
	if err != nil || !qa.Passed || qa.BlackDurationSec > 0.5 {
		t.Fatalf("short legitimate fade-to-black was rejected: qa=%+v err=%v", qa, err)
	}
	silent := makeQAFixture(t, policy, "silent.mp4", "color=c=blue:s=320x180:d=1", "anullsrc=channel_layout=mono:sample_rate=16000", "", "")
	qa, err = InspectWithRequirements(context.Background(), silent, policy, true)
	if err != nil || qa.Passed || !qa.SilenceDetected || qa.SilenceRatio < 0.9 || !containsError(qa.Errors, "excessive_silence_content") {
		t.Fatalf("fully silent required audio was not rejected: qa=%+v err=%v", qa, err)
	}
	shortSilence := makeQAFixture(t, policy, "short-silence.mp4", "color=c=blue:s=320x180:d=1", "sine=frequency=880:duration=1", "", "volume=enable=between(t\\,0.4\\,0.6):volume=0")
	qa, err = InspectWithRequirements(context.Background(), shortSilence, policy, true)
	if err != nil || !qa.Passed || !qa.SilenceDetected || qa.SilenceDurationSec > 0.5 {
		t.Fatalf("brief intentional silence was rejected: qa=%+v err=%v", qa, err)
	}
	audioOnly := filepath.Join(sandbox, "audio-only.m4a")
	if _, err := policy.Run(context.Background(), process.Spec{Tool: process.ToolFFmpeg, Args: []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "sine=frequency=880:duration=1", "-c:a", "aac", audioOnly}, OutputPaths: []string{audioOnly}, Timeout: time.Minute}); err != nil {
		t.Fatal(err)
	}
	qa, err = InspectWithRequirements(context.Background(), audioOnly, policy, true)
	if err != nil || qa.Passed || !containsError(qa.Errors, "video_stream_missing") {
		t.Fatalf("missing video did not fail QA: qa=%+v err=%v", qa, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := InspectWithRequirements(cancelled, normal, policy, true); err == nil {
		t.Fatal("cancelled QA unexpectedly completed")
	}
}

func TestSubjectAwareReframeChangesRealPixels(t *testing.T) {
	ffmpeg := os.Getenv("NH_MEDIA_FFMPEG_PATH")
	ffprobe := os.Getenv("NH_MEDIA_FFPROBE_PATH")
	if ffmpeg == "" || ffprobe == "" {
		t.Fatal("Gate G real-media proof requires reviewed NH_MEDIA_FFMPEG_PATH and NH_MEDIA_FFPROBE_PATH; unexpected skip")
	}
	sandbox := t.TempDir()
	policy := process.Policy{FFmpegPath: ffmpeg, FFprobePath: ffprobe, SandboxRoot: sandbox, MaxOutputBytes: 4 << 20, MaxDuration: time.Minute}
	source := filepath.Join(sandbox, "split.mp4")
	if _, err := policy.Run(context.Background(), process.Spec{Tool: process.ToolFFmpeg, Args: []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "color=c=blue:s=160x90:d=1", "-f", "lavfi", "-i", "color=c=red:s=160x90:d=1", "-f", "lavfi", "-i", "sine=frequency=880:duration=1", "-filter_complex", "[0:v][1:v]hstack=inputs=2[v]", "-map", "[v]", "-map", "2:a:0", "-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", source}, InputPaths: []string{}, OutputPaths: []string{source}, Timeout: time.Minute}); err != nil {
		t.Fatal(err)
	}
	profile := fixtureProfile(t, "shorts_9_16")
	profile.AutoReframe = true
	leftOutput := filepath.Join(sandbox, "left.mp4")
	rightOutput := filepath.Join(sandbox, "right.mp4")
	left, err := Render(context.Background(), CompileInput{TimelineDocument: fixtureRenderTimeline(), Profile: profile, ArtifactPaths: map[string]string{"source_artifact": source}, OutputPath: leftOutput, SubjectTracks: []SubjectTrack{{ID: "subject", SourceRevision: "split-v1", Points: []SubjectPoint{{TimeSec: 0, X: 0.02, Y: 0.1, Width: 0.1, Height: 0.5}}}}}, policy)
	if err != nil {
		t.Fatal(err)
	}
	right, err := Render(context.Background(), CompileInput{TimelineDocument: fixtureRenderTimeline(), Profile: profile, ArtifactPaths: map[string]string{"source_artifact": source}, OutputPath: rightOutput, SubjectTracks: []SubjectTrack{{ID: "subject", SourceRevision: "split-v1", Points: []SubjectPoint{{TimeSec: 0, X: 0.88, Y: 0.1, Width: 0.1, Height: 0.5}}}}}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if left.SHA256 == right.SHA256 || left.Width != 360 || left.Height != 640 || !left.QAReport.Passed || !right.QAReport.Passed {
		t.Fatalf("subject coordinates did not affect real rendered pixels: left=%+v right=%+v", left, right)
	}
}

func TestDurationAndQAFailuresPreventArtifactCommit(t *testing.T) {
	ffmpeg := os.Getenv("NH_MEDIA_FFMPEG_PATH")
	ffprobe := os.Getenv("NH_MEDIA_FFPROBE_PATH")
	if ffmpeg == "" || ffprobe == "" {
		t.Fatal("Gate G real-media proof requires reviewed NH_MEDIA_FFMPEG_PATH and NH_MEDIA_FFPROBE_PATH; unexpected skip")
	}
	sandbox := t.TempDir()
	policy := process.Policy{FFmpegPath: ffmpeg, FFprobePath: ffprobe, SandboxRoot: sandbox, MaxOutputBytes: 4 << 20, MaxDuration: time.Minute}
	source := makeQAFixture(t, policy, "duration-source.mp4", "color=c=blue:s=320x180:d=1", "sine=frequency=880:duration=1", "", "")
	mismatchTimeline := strings.Replace(string(fixtureRenderTimeline()), `"duration_sec":1,"tracks"`, `"duration_sec":0.5,"tracks"`, 1)
	mismatchTimeline = strings.Replace(mismatchTimeline, `"timeline_out_sec":1,"source"`, `"timeline_out_sec":0.5,"loop":true,"source"`, 1)
	committer := &memoryCommitter{}
	profile := fixtureProfile(t, "youtube_16_9")
	if _, _, err := RenderAndCommit(context.Background(), CompileInput{TimelineDocument: []byte(mismatchTimeline), Profile: profile, ArtifactPaths: map[string]string{"source_artifact": source}, OutputPath: filepath.Join(sandbox, "mismatch.mp4")}, policy, committer); err == nil || committer.committed != nil {
		t.Fatalf("duration-mismatched render was committed: err=%v candidate=%+v", err, committer.committed)
	}
	black := makeQAFixture(t, policy, "black-commit.mp4", "color=c=black:s=320x180:d=1", "sine=frequency=880:duration=1", "", "")
	committer = &memoryCommitter{}
	if _, _, err := RenderAndCommit(context.Background(), CompileInput{TimelineDocument: fixtureRenderTimeline(), Profile: profile, ArtifactPaths: map[string]string{"source_artifact": black}, OutputPath: filepath.Join(sandbox, "black-render.mp4")}, policy, committer); err == nil || committer.committed != nil {
		t.Fatalf("black deliverable was offered for Artifact commit: err=%v candidate=%+v", err, committer.committed)
	}
}

func makeQAFixture(t *testing.T, policy process.Policy, name, videoInput, audioInput, videoFilter, audioFilter string) string {
	return makeQAFixtureDuration(t, policy, name, videoInput, audioInput, videoFilter, audioFilter, "1")
}

func makeQAFixtureDuration(t *testing.T, policy process.Policy, name, videoInput, audioInput, videoFilter, audioFilter, duration string) string {
	t.Helper()
	path := filepath.Join(policy.SandboxRoot, name)
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", videoInput, "-f", "lavfi", "-i", audioInput}
	if videoFilter != "" {
		args = append(args, "-vf", videoFilter)
	}
	if audioFilter != "" {
		args = append(args, "-af", audioFilter)
	}
	args = append(args, "-t", duration, "-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", path)
	if _, err := policy.Run(context.Background(), process.Spec{Tool: process.ToolFFmpeg, Args: args, OutputPaths: []string{path}, Timeout: time.Minute}); err != nil {
		t.Fatal(err)
	}
	return path
}

func containsError(errors []string, expected string) bool {
	for _, value := range errors {
		if value == expected {
			return true
		}
	}
	return false
}

type memoryCommitter struct {
	committed *ArtifactCandidate
}

func (m *memoryCommitter) Commit(_ context.Context, candidate ArtifactCandidate) error {
	m.committed = &candidate
	return nil
}

func TestArtifactCommitIsAfterDeliverableQA(t *testing.T) {
	committer := &memoryCommitter{}
	profile := fixtureProfile(t, "youtube_16_9")
	_, _, err := RenderAndCommit(context.Background(), CompileInput{TimelineDocument: fixtureTimeline(), Profile: profile, ArtifactPaths: map[string]string{"source_artifact": "source.mp4"}, OutputPath: "output.mp4"}, process.Policy{}, committer)
	if err == nil || committer.committed != nil {
		t.Fatal("render Artifact was offered to storage without executable media/QA evidence")
	}
}
