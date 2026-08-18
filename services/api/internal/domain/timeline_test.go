package domain

import (
	"encoding/json"
	"testing"
)

func validTimeline() []byte {
	return []byte(`{"schema_version":"1.0","timeline_id":"tl_1","timeline_version_id":"tlv_1","project_id":"proj_1","version":1,"duration_sec":10,"tracks":[{"id":"track_1","kind":"video","name":"Footage","order":0,"clips":[{"id":"clip_1","timeline_in_sec":0,"timeline_out_sec":4,"source":{"type":"none","inline_id":"clip_1"},"origin":"user"}]}]}`)
}

func TestTimelineValidationAndHashAreDeterministic(t *testing.T) {
	one, err := ValidateTimeline(validTimeline(), TimelineValidationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	two, err := ValidateTimeline([]byte(`{"tracks":[{"clips":[{"origin":"user","source":{"inline_id":"clip_1","type":"none"},"timeline_out_sec":4,"timeline_in_sec":0,"id":"clip_1"}],"order":0,"name":"Footage","kind":"video","id":"track_1"}],"duration_sec":10,"version":1,"project_id":"proj_1","timeline_version_id":"tlv_1","timeline_id":"tl_1","schema_version":"1.0"}`), TimelineValidationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if one != two || len(one) != 64 {
		t.Fatalf("hash not deterministic: %s %s", one, two)
	}
}

func TestTimelineRejectsRangeAndSupportsStableIDCommand(t *testing.T) {
	invalid := []byte(`{"schema_version":"1.0","timeline_id":"tl_1","timeline_version_id":"tlv_1","project_id":"proj_1","version":1,"duration_sec":10,"tracks":[{"id":"track_1","kind":"video","name":"Footage","order":0,"clips":[{"id":"clip_1","timeline_in_sec":0,"timeline_out_sec":11,"source":{"type":"none","inline_id":"clip_1"},"origin":"ai"}]}]}`)
	if _, err := ValidateTimeline(invalid, TimelineValidationOptions{}); err == nil {
		t.Fatal("out-of-range clip accepted")
	}
	var command TimelineCommand
	if err := json.Unmarshal([]byte(`{"kind":"AddClip","schema_version":"1.0","based_on_version_id":"tlv_1","expected_version":1,"payload":{"track_id":"track_1","clip":{"id":"clip_2","timeline_in_sec":4,"timeline_out_sec":8,"source":{"type":"none","inline_id":"clip_2"},"origin":"user"}}}`), &command); err != nil {
		t.Fatal(err)
	}
	document, hash, err := ApplyTimelineCommand(validTimeline(), command)
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Fatal("command did not return content hash")
	}
	if _, err := ValidateTimeline(document, TimelineValidationOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestTimelineRejectsExternalPathsAndSensitiveFields(t *testing.T) {
	for _, fragment := range []string{`"url":"https://example.invalid/source.mp4"`, `"local_path":"C:\\media\\source.mp4"`, `"provider_token":"secret"`} {
		document := []byte(`{"schema_version":"1.0","timeline_id":"tl_1","timeline_version_id":"tlv_1","project_id":"proj_1","duration_sec":10,"tracks":[{"id":"track_1","kind":"video","clips":[{"id":"clip_1","timeline_in_sec":0,"timeline_out_sec":4,"source":{"type":"none"},"origin":"user"}]}],` + fragment + `}`)
		if _, err := ValidateTimeline(document, TimelineValidationOptions{}); err == nil {
			t.Fatalf("unsafe timeline fragment accepted: %s", fragment)
		}
	}
}

func TestTimelineCommandSeparatesStructuralAndDurableReferenceValidation(t *testing.T) {
	document := []byte(`{"schema_version":"1.0","timeline_id":"tl_1","timeline_version_id":"tlv_1","project_id":"proj_1","version":1,"duration_sec":10,"tracks":[{"id":"track_1","kind":"video","name":"Footage","order":0,"clips":[{"id":"clip_1","timeline_in_sec":0,"timeline_out_sec":4,"source":{"type":"asset","asset_id":"asset_1","artifact_id":"artifact_1"},"origin":"ai","proposal_refs":["proposal_1"],"evidence_refs":["evidence_1"]}]}]}`)
	command := TimelineCommand{Kind: "ReplaceClip", SchemaVersion: "1.0", ExpectedVersion: 1, Payload: map[string]any{
		"clip_id": "clip_1", "clip": map[string]any{"id": "clip_1", "timeline_in_sec": 1.0, "timeline_out_sec": 5.0, "source": map[string]any{"type": "asset", "asset_id": "asset_1", "artifact_id": "artifact_1"}},
	}}
	updated, _, err := ApplyTimelineCommand(document, command)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateTimeline(updated, TimelineValidationOptions{StructuralOnly: true}); err != nil {
		t.Fatalf("structural command result was rejected: %v", err)
	}
	if _, err := ValidateTimeline(updated, TimelineValidationOptions{}); err == nil {
		t.Fatal("durable reference was accepted without the resolver")
	}
	var root map[string]any
	if err := json.Unmarshal(updated, &root); err != nil {
		t.Fatal(err)
	}
	clip := root["tracks"].([]any)[0].(map[string]any)["clips"].([]any)[0].(map[string]any)
	if clip["origin"] != "user" || len(clip["proposal_refs"].([]any)) != 1 || len(clip["evidence_refs"].([]any)) != 1 {
		t.Fatalf("ReplaceClip lost provenance: %#v", clip)
	}
	command.ExpectedVersion = 1
	if _, _, err := ApplyTimelineCommand(updated, command); err == nil {
		t.Fatal("stale timeline command was accepted")
	}
}
