package render

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
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
	document := strings.Replace(string(fixtureTimeline()), "Gate G", "Gate G;whoami", 1)
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

func TestRenderRealMediaAndReuseThreeProfiles(t *testing.T) {
	ffmpeg := os.Getenv("NH_MEDIA_FFMPEG_PATH")
	ffprobe := os.Getenv("NH_MEDIA_FFPROBE_PATH")
	if ffmpeg == "" || ffprobe == "" {
		t.Skip("Gate G real-media proof requires reviewed NH_MEDIA_FFMPEG_PATH and NH_MEDIA_FFPROBE_PATH")
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

func TestInspectRejectsMissingVideo(t *testing.T) {
	if os.Getenv("NH_MEDIA_FFMPEG_PATH") == "" || os.Getenv("NH_MEDIA_FFPROBE_PATH") == "" {
		t.Skip("Gate G real-media proof requires reviewed FFmpeg paths")
	}
	// A nonexistent input must fail before any canonical Artifact claim.
	policy := process.Policy{FFmpegPath: os.Getenv("NH_MEDIA_FFMPEG_PATH"), FFprobePath: os.Getenv("NH_MEDIA_FFPROBE_PATH"), SandboxRoot: t.TempDir(), MaxOutputBytes: 1 << 20, MaxDuration: time.Second}
	_, err := Inspect(context.Background(), filepath.Join(policy.SandboxRoot, "missing.mp4"), policy)
	if err == nil {
		t.Fatal("missing deliverable unexpectedly passed inspection")
	}
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
