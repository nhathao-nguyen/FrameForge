package pipeline

import (
	"encoding/json"
	"testing"
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
}

func TestBuildTimelineAcceptsReferencesStructurallyAndRecordsDegradedInputs(t *testing.T) {
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
	degraded, ok := document["degraded"].([]any)
	if !ok || len(degraded) != 1 || degraded[0].(map[string]any)["soft"] != true {
		t.Fatalf("degraded input was not preserved: %#v", document["degraded"])
	}
}
