package domain

import (
	"encoding/json"
	"testing"
)

type timelineTestResolver struct{}

func (timelineTestResolver) ResolveAssetArtifact(string, string, string) (bool, float64, int, int, error) {
	return false, 0, 0, 0, nil
}
func (timelineTestResolver) ResolveScene(string, string, string, string) (bool, float64, float64, error) {
	return true, 248.1, 256.3, nil
}
func (timelineTestResolver) ResolveArtifact(string, string) (bool, error) { return true, nil }
func (timelineTestResolver) ResolveNarration(string, string, string) (bool, error) {
	return true, nil
}

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

func TestNormalizeTimelineIdentityRewritesServerOwnedFields(t *testing.T) {
	normalized, err := NormalizeTimelineIdentity(validTimeline(), "timeline_server", "version_server", 7)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(normalized, &root); err != nil {
		t.Fatal(err)
	}
	if root["timeline_id"] != "timeline_server" || root["timeline_version_id"] != "version_server" || root["version"] != float64(7) {
		t.Fatalf("server identity was not applied: %#v", root)
	}
	if _, err := ValidateTimeline(normalized, TimelineValidationOptions{}); err != nil {
		t.Fatalf("normalized document is not canonical: %v", err)
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

func TestTimelineValidatorRejectsCanonicalSchemaDriftAndUnsafeDocuments(t *testing.T) {
	tests := []struct {
		name string
		edit func(map[string]any)
	}{
		{"unknown root field", func(root map[string]any) { root["unknown"] = true }},
		{"missing version", func(root map[string]any) { delete(root, "version") }},
		{"wrong version type", func(root map[string]any) { root["version"] = "1" }},
		{"missing track name", func(root map[string]any) { delete(root["tracks"].([]any)[0].(map[string]any), "name") }},
		{"missing track order", func(root map[string]any) { delete(root["tracks"].([]any)[0].(map[string]any), "order") }},
		{"unknown track property", func(root map[string]any) { root["tracks"].([]any)[0].(map[string]any)["future"] = true }},
		{"unknown clip property", func(root map[string]any) { clip(root)["future"] = true }},
		{"invalid track kind", func(root map[string]any) { root["tracks"].([]any)[0].(map[string]any)["kind"] = "captions" }},
		{"invalid origin", func(root map[string]any) { clip(root)["origin"] = "provider" }},
		{"timeline out exceeds duration", func(root map[string]any) { clip(root)["timeline_out_sec"] = 11 }},
		{"invalid source refs", func(root map[string]any) {
			clip(root)["source"] = map[string]any{"type": "asset", "asset_id": "asset_1", "artifact_id": "staged_artifact"}
		}},
		{"inline source range", func(root map[string]any) { clip(root)["source_out_sec"] = 1 }},
		{"invalid narration refs", func(root map[string]any) { clip(root)["narration"] = map[string]any{"narration_id": "nar_1"} }},
		{"external URL", func(root map[string]any) {
			root["metadata"] = map[string]any{"source": "https://example.invalid/video.mp4"}
		}},
		{"external filesystem path", func(root map[string]any) { root["metadata"] = map[string]any{"source": `C:\\media\\video.mp4`} }},
		{"sensitive field", func(root map[string]any) { root["metadata"] = map[string]any{"provider_token": "must-not-be-stored"} }},
		{"normalized crop outside source", func(root map[string]any) {
			clip(root)["transform"] = map[string]any{"crop": map[string]any{"x": 0.8, "y": 0, "width": 0.5, "height": 1, "unit": "normalized"}}
		}},
		{"subtitle word outside clip", func(root map[string]any) {
			root["tracks"].([]any)[0].(map[string]any)["kind"] = "subtitle"
			clip(root)["subtitle"] = map[string]any{"text": "hello", "language": "en", "words": []any{map[string]any{"text": "hello", "start_offset_sec": 0, "end_offset_sec": 5}}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var root map[string]any
			if err := json.Unmarshal(validTimeline(), &root); err != nil {
				t.Fatal(err)
			}
			test.edit(root)
			document, err := json.Marshal(root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ValidateTimeline(document, TimelineValidationOptions{}); err == nil {
				t.Fatalf("invalid document was accepted: %s", document)
			}
		})
	}
}

func TestTimelineValidatorRejectsUnsafeOverlapAndOversizedTransition(t *testing.T) {
	var root map[string]any
	if err := json.Unmarshal(validTimeline(), &root); err != nil {
		t.Fatal(err)
	}
	track := root["tracks"].([]any)[0].(map[string]any)
	track["clips"] = []any{
		clip(root),
		map[string]any{"id": "clip_2", "timeline_in_sec": 3, "timeline_out_sec": 6, "source": map[string]any{"type": "none", "inline_id": "clip_2"}, "origin": "user"},
	}
	encoded, _ := json.Marshal(root)
	if _, err := ValidateTimeline(encoded, TimelineValidationOptions{}); err == nil {
		t.Fatal("same-track video overlap without a transition was accepted")
	}
	track["clips"].([]any)[0].(map[string]any)["transition_out"] = map[string]any{"kind": "crossfade", "duration_sec": 1}
	if _, err := ValidateTimeline(encoded, TimelineValidationOptions{}); err == nil {
		t.Fatal("stale encoded overlap unexpectedly passed")
	}
	encoded, _ = json.Marshal(root)
	if _, err := ValidateTimeline(encoded, TimelineValidationOptions{}); err != nil {
		t.Fatalf("explicit transition overlap should pass: %v", err)
	}
	track["clips"].([]any)[0].(map[string]any)["transition_out"] = map[string]any{"kind": "crossfade", "duration_sec": 5}
	encoded, _ = json.Marshal(root)
	if _, err := ValidateTimeline(encoded, TimelineValidationOptions{}); err == nil {
		t.Fatal("transition longer than clip was accepted")
	}
}

func clip(root map[string]any) map[string]any {
	return root["tracks"].([]any)[0].(map[string]any)["clips"].([]any)[0].(map[string]any)
}

func TestTimelineValidatorRejectsSourceTimingMismatchAndAcceptsExactSpeed(t *testing.T) {
	var root map[string]any
	if err := json.Unmarshal(validTimeline(), &root); err != nil {
		t.Fatal(err)
	}
	clip(root)["source"] = map[string]any{"type": "generated", "generator_ref": "generated_1"}
	clip(root)["source_in_sec"] = 0.0
	clip(root)["source_out_sec"] = 8.0
	clip(root)["speed"] = 2.0
	encoded, _ := json.Marshal(root)
	if _, err := ValidateTimeline(encoded, TimelineValidationOptions{}); err != nil {
		t.Fatalf("exact source/timeline speed should pass: %v", err)
	}
	clip(root)["speed"] = 1.0
	encoded, _ = json.Marshal(root)
	if _, err := ValidateTimeline(encoded, TimelineValidationOptions{}); err == nil {
		t.Fatal("source/timeline speed mismatch was accepted")
	}
}

func TestTimelineValidatorUsesAbsoluteSceneSourceRange(t *testing.T) {
	var root map[string]any
	if err := json.Unmarshal(validTimeline(), &root); err != nil {
		t.Fatal(err)
	}
	clip(root)["source"] = map[string]any{"type": "scene", "scene_id": "scene_1", "asset_id": "asset_1", "artifact_id": "artifact_1"}
	clip(root)["timeline_out_sec"] = 8.2
	clip(root)["source_in_sec"] = 248.1
	clip(root)["source_out_sec"] = 256.3
	encoded, _ := json.Marshal(root)
	if _, err := ValidateTimeline(encoded, TimelineValidationOptions{Resolver: timelineTestResolver{}}); err != nil {
		t.Fatalf("scene source coordinates from the original asset were rejected: %v", err)
	}
	clip(root)["source_in_sec"] = 0.0
	clip(root)["source_out_sec"] = 8.2
	encoded, _ = json.Marshal(root)
	if _, err := ValidateTimeline(encoded, TimelineValidationOptions{Resolver: timelineTestResolver{}}); err == nil {
		t.Fatal("scene source range outside the resolved scene was accepted")
	}
}

func TestCanonicalTimelineCarriesAndEnforcesBGMSourceRights(t *testing.T) {
	document := []byte(`{"schema_version":"1.0","timeline_id":"tl_music","timeline_version_id":"tlv_music","project_id":"proj_1","version":1,"duration_sec":2,"tracks":[{"id":"music_1","kind":"music","name":"BGM","order":0,"clips":[{"id":"music_clip_1","timeline_in_sec":0,"timeline_out_sec":2,"source":{"type":"artifact","artifact_id":"artifact_music_1","rights_status":"cleared","rights_metadata":{"license":"workspace-cleared"}},"origin":"user"}]}]}`)
	if _, err := ValidateTimeline(document, TimelineValidationOptions{Resolver: timelineTestResolver{}}); err != nil {
		t.Fatalf("canonical BGM rights fields were rejected: %v", err)
	}
	for _, invalid := range [][]byte{
		[]byte(`{"schema_version":"1.0","timeline_id":"tl_music","timeline_version_id":"tlv_music","project_id":"proj_1","version":1,"duration_sec":2,"tracks":[{"id":"music_1","kind":"music","name":"BGM","order":0,"clips":[{"id":"music_clip_1","timeline_in_sec":0,"timeline_out_sec":2,"source":{"type":"artifact","artifact_id":"artifact_music_1","rights_status":"cleared"},"origin":"user"}]}]}`),
		[]byte(`{"schema_version":"1.0","timeline_id":"tl_music","timeline_version_id":"tlv_music","project_id":"proj_1","version":1,"duration_sec":2,"tracks":[{"id":"music_1","kind":"music","name":"BGM","order":0,"clips":[{"id":"music_clip_1","timeline_in_sec":0,"timeline_out_sec":2,"source":{"type":"artifact","artifact_id":"artifact_music_1","rights_status":"rejected","rights_metadata":{"reason":"unknown"}},"origin":"user"}]}]}`),
	} {
		if _, err := ValidateTimeline(invalid, TimelineValidationOptions{Resolver: timelineTestResolver{}}); err == nil {
			t.Fatal("non-renderable BGM rights state was accepted")
		}
	}
}
