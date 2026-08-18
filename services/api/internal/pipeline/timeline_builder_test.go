package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
)

func TestBuildTimelinePreservesEvidenceAndUserOverride(t *testing.T) {
	value, err := BuildTimeline(BuildTimelineInput{
		TimelineID: "timeline_001", TimelineVersionID: "timeline_version_002", ProjectID: "project_001", Version: 2, DurationSec: 8,
		Tracks: []TimelineTrack{{ID: "video_1", Kind: "video", Name: "Video", Order: 1, Clips: []ClipProposal{{
			ID: "clip_1", TimelineIn: 0, TimelineOut: 4, Origin: "ai", Source: map[string]any{"type": "generated", "generator_ref": "generator_1"},
			ProposalRefs: []string{"proposal_1"}, EvidenceRefs: []string{"evidence_1"}, Metadata: map[string]any{"scene_id": "scene_1"},
		}}}},
		Overrides: []ClipOverride{{ClipID: "clip_1", Fields: map[string]any{"timeline_out_sec": 3.5}}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(value.Document, &document); err != nil {
		t.Fatal(err)
	}
	clip := document["tracks"].([]any)[0].(map[string]any)["clips"].([]any)[0].(map[string]any)
	if clip["origin"] != "user" || clip["proposal_refs"].([]any)[0] != "proposal_1" || clip["evidence_refs"].([]any)[0] != "evidence_1" {
		t.Fatalf("override lost provenance: %#v", clip)
	}
	if clip["timeline_out_sec"] != 3.5 || value.ContentHash == "" || value.InputFingerprint == "" {
		t.Fatalf("invalid built timeline: %+v", value)
	}
}

func TestBuildTimelineRejectsUnknownOverrideAndProducesStableHash(t *testing.T) {
	input := BuildTimelineInput{TimelineID: "timeline_001", TimelineVersionID: "timeline_version_001", ProjectID: "project_001", Version: 1, DurationSec: 2, Tracks: []TimelineTrack{{ID: "video_1", Kind: "video", Name: "Video", Order: 1, Clips: []ClipProposal{{ID: "clip_1", TimelineIn: 0, TimelineOut: 1, Origin: "ai", Source: map[string]any{"type": "none", "inline_id": "clip_1"}}}}}}
	first, err := BuildTimeline(input, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildTimeline(input, nil)
	if err != nil || first.ContentHash != second.ContentHash || first.InputFingerprint != second.InputFingerprint {
		t.Fatalf("timeline output is not deterministic: %v %v", first, second)
	}
	input.Overrides = []ClipOverride{{ClipID: "clip_1", Fields: map[string]any{"arbitrary": "no"}}}
	if _, err := BuildTimeline(input, nil); err == nil {
		t.Fatal("arbitrary override was accepted")
	}
	input.Overrides = nil
	input.Tracks[0].Clips[0].Origin = "provider"
	if _, err := BuildTimeline(input, nil); err == nil {
		t.Fatal("invalid proposal origin was silently normalized")
	}
}

func TestBuildTimelineAcceptsReferencesStructurallyAndKeepsDegradedInputsOutsideDocument(t *testing.T) {
	value, err := BuildTimeline(BuildTimelineInput{
		TimelineID: "timeline_002", TimelineVersionID: "timeline_version_002", ProjectID: "project_002", Version: 1, DurationSec: 5,
		Tracks:   []TimelineTrack{{ID: "video_1", Kind: "video", Name: "Video", Clips: []ClipProposal{{ID: "clip_1", TimelineIn: 0, TimelineOut: 2, Origin: "ai", Source: map[string]any{"type": "asset", "asset_id": "asset_1", "artifact_id": "artifact_1"}}}}},
		Degraded: []DegradedInput{{NodeKey: "extract_transcript", Reason: "worker_unavailable", Consequence: "dialogue matching is disabled", Soft: true, EvidenceRefs: []string{"event_1"}}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(value.Document, &document); err != nil {
		t.Fatal(err)
	}
	if _, exists := document["degraded"]; exists {
		t.Fatal("execution degraded metadata leaked into canonical TimelineVersion")
	}
	if len(value.Degraded) != 1 || !value.Degraded[0].Soft || value.Degraded[0].NodeKey != "extract_transcript" {
		t.Fatalf("degraded execution metadata was not preserved: %#v", value.Degraded)
	}
	if _, err := domain.ValidateTimeline(value.Document, domain.TimelineValidationOptions{StructuralOnly: true}); err != nil {
		t.Fatalf("builder output did not pass production validator: %v", err)
	}
}
