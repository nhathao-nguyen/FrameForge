package product

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
)

func timelineDocumentFor(projectID string, source map[string]any, narration map[string]any) json.RawMessage {
	clip := map[string]any{
		"id":               "clip_1",
		"timeline_in_sec":  0,
		"timeline_out_sec": 4,
		"source":           source,
		"origin":           "user",
	}
	if narration != nil {
		clip["narration"] = narration
	}
	document := map[string]any{
		"schema_version":      "1.0",
		"timeline_id":         "tl_client",
		"timeline_version_id": "tlv_client",
		"project_id":          projectID,
		"version":             1,
		"duration_sec":        10,
		"tracks": []any{map[string]any{
			"id": "track_1", "kind": "video", "name": "Footage", "order": 0,
			"clips": []any{clip},
		}},
	}
	value, _ := json.Marshal(document)
	return value
}

func mustProject(t *testing.T, store *Store, name string) *Project {
	t.Helper()
	value, err := store.CreateProject(store.WorkspaceID(), name, "movie_recap", nil)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustReadyAsset(t *testing.T, store *Store, projectID, artifactID string) *Asset {
	t.Helper()
	asset, err := store.CreateAsset(store.WorkspaceID(), projectID, "video", "source.mp4", "video/mp4", 12, "")
	if err != nil {
		t.Fatal(err)
	}
	asset, err = store.SetAssetStatus(store.WorkspaceID(), projectID, asset.ID, "ready", artifactID)
	if err != nil {
		t.Fatal(err)
	}
	return asset
}

func TestStoreTimelineReferencesFailClosedAcrossOwnershipAndState(t *testing.T) {
	store := NewStore("ws_test")
	project := mustProject(t, store, "primary")
	foreignProject := mustProject(t, store, "foreign")
	asset := mustReadyAsset(t, store, project.ID, "artifact_good")
	foreignAsset := mustReadyAsset(t, store, foreignProject.ID, "artifact_foreign")

	if _, _, err := store.CreateTimeline(store.WorkspaceID(), project.ID, "user", timelineDocumentFor(project.ID, map[string]any{"type": "asset", "asset_id": asset.ID, "artifact_id": "artifact_good"}, nil)); err != nil {
		t.Fatalf("valid committed asset reference rejected: %v", err)
	}
	for name, source := range map[string]map[string]any{
		"missing artifact":  {"type": "asset", "asset_id": asset.ID, "artifact_id": "artifact_missing"},
		"foreign asset":     {"type": "asset", "asset_id": foreignAsset.ID, "artifact_id": "artifact_foreign"},
		"foreign artifact":  {"type": "asset", "asset_id": asset.ID, "artifact_id": "artifact_foreign"},
		"unusable artifact": {"type": "artifact", "artifact_id": "artifact_missing"},
	} {
		if _, _, err := store.CreateTimeline(store.WorkspaceID(), project.ID, "user", timelineDocumentFor(project.ID, source, nil)); err == nil {
			t.Fatalf("%s reference was accepted", name)
		}
	}

	staging, err := store.CreateAsset(store.WorkspaceID(), project.ID, "video", "staging.mp4", "video/mp4", 12, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.CreateTimeline(store.WorkspaceID(), project.ID, "user", timelineDocumentFor(project.ID, map[string]any{"type": "asset", "asset_id": staging.ID, "artifact_id": "artifact_staging"}, nil)); err == nil {
		t.Fatal("non-ready asset reference was accepted")
	}
}

func TestStoreTimelineSceneNarrationAndCommandPathsResolveDurableState(t *testing.T) {
	store := NewStore("ws_test")
	project := mustProject(t, store, "primary")
	asset := mustReadyAsset(t, store, project.ID, "artifact_good")
	scene, err := store.CreateScene(store.WorkspaceID(), project.ID, asset.ID, "artifact_good", 1, 5, "scene")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.CreateTimeline(store.WorkspaceID(), project.ID, "user", timelineDocumentFor(project.ID, map[string]any{"type": "scene", "scene_id": scene.ID, "asset_id": asset.ID, "artifact_id": "artifact_good"}, nil)); err != nil {
		t.Fatalf("valid scene reference rejected: %v", err)
	}
	if _, _, err := store.CreateTimeline(store.WorkspaceID(), project.ID, "user", timelineDocumentFor(project.ID, map[string]any{"type": "scene", "scene_id": scene.ID, "asset_id": asset.ID, "artifact_id": "artifact_wrong"}, nil)); err == nil {
		t.Fatal("scene with mismatched source artifact was accepted")
	}

	_, scriptVersion, err := store.CreateScript(store.WorkspaceID(), project.ID, "en", "user", map[string]any{"text": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetScriptVersionStatus(store.WorkspaceID(), project.ID, "missing-script", scriptVersion.ID, "approved"); err == nil {
		t.Fatal("foreign script aggregate was accepted")
	}
	script, err := store.GetScript(store.WorkspaceID(), project.ID, scriptVersion.ScriptID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetScriptVersionStatus(store.WorkspaceID(), project.ID, script.ID, scriptVersion.ID, "approved"); err != nil {
		t.Fatal(err)
	}
	narration, err := store.CreateNarration(store.WorkspaceID(), project.ID, scriptVersion.ID, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.narrations[narration.ID].Status = "ready"
	store.narrations[narration.ID].AudioArtifactID = "artifact_good"
	store.mu.Unlock()
	narrationRef := map[string]any{"narration_id": narration.ID, "script_version_id": scriptVersion.ID}
	if _, _, err := store.CreateTimeline(store.WorkspaceID(), project.ID, "user", timelineDocumentFor(project.ID, map[string]any{"type": "none", "inline_id": "silence"}, narrationRef)); err != nil {
		t.Fatalf("valid narration reference rejected: %v", err)
	}
	if _, _, err := store.CreateTimeline(store.WorkspaceID(), project.ID, "user", timelineDocumentFor(project.ID, map[string]any{"type": "none"}, map[string]any{"narration_id": narration.ID, "script_version_id": "missing"})); err == nil {
		t.Fatal("missing narration version was accepted")
	}

	base, version, err := store.CreateTimeline(store.WorkspaceID(), project.ID, "user", timelineDocumentFor(project.ID, map[string]any{"type": "asset", "asset_id": asset.ID, "artifact_id": "artifact_good"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	badDocument := timelineDocumentFor(project.ID, map[string]any{"type": "asset", "asset_id": asset.ID, "artifact_id": "artifact_missing"}, nil)
	if _, err := store.AddTimelineVersion(store.WorkspaceID(), project.ID, base.ID, version.ID, "user", badDocument, base.Revision); err == nil {
		t.Fatal("domain/persistence version path bypassed cross-reference validation")
	}
	if _, err := store.ValidateTimeline(store.WorkspaceID(), project.ID, version.ID); err != nil {
		t.Fatalf("valid stored timeline failed full validation: %v", err)
	}
}

func TestStoreTimelineEditReloadUndoCreatesImmutableVersionChain(t *testing.T) {
	store := NewStore("ws_test")
	project := mustProject(t, store, "undo")
	document := timelineDocumentFor(project.ID, map[string]any{"type": "none", "inline_id": "clip_1"}, nil)
	var root map[string]any
	if err := json.Unmarshal(document, &root); err != nil {
		t.Fatal(err)
	}
	baseClip := root["tracks"].([]any)[0].(map[string]any)["clips"].([]any)[0].(map[string]any)
	baseClip["origin"] = "ai"
	baseClip["proposal_refs"] = []any{"proposal_1"}
	baseClip["evidence_refs"] = []any{"evidence_1"}
	document, _ = json.Marshal(root)
	timeline, v1, err := store.CreateTimeline(store.WorkspaceID(), project.ID, "system", document)
	if err != nil {
		t.Fatal(err)
	}
	assertStoredTimelineIdentity(t, timeline, v1)

	editCommand := domain.TimelineCommand{Kind: "UpdateScene", SchemaVersion: "1.0", ExpectedVersion: 1, Payload: map[string]any{
		"clip_id": "clip_1", "metadata": map[string]any{"edited_from": "web"},
	}}
	v2Document, _, err := domain.ApplyTimelineCommand(v1.Document, editCommand)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := store.AddTimelineVersion(store.WorkspaceID(), project.ID, timeline.ID, v1.ID, "user", v2Document, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertStoredTimelineIdentity(t, timeline, v2)

	previousClip, err := domain.FindTimelineClip(v1.Document, "clip_1")
	if err != nil {
		t.Fatal(err)
	}
	undoCommand := domain.TimelineCommand{Kind: "RestoreClip", SchemaVersion: "1.0", ExpectedVersion: 2, Payload: map[string]any{
		"clip_id": "clip_1", "restore_from_version_id": v1.ID, "clip": previousClip,
	}}
	v3Document, _, err := domain.ApplyTimelineCommand(v2.Document, undoCommand)
	if err != nil {
		t.Fatal(err)
	}
	v3, err := store.AddTimelineVersion(store.WorkspaceID(), project.ID, timeline.ID, v2.ID, "user", v3Document, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertStoredTimelineIdentity(t, timeline, v3)

	if v1.Document == nil || v2.Document == nil || v3.Document == nil {
		t.Fatal("timeline version documents must remain durable")
	}
	v1Clip, _ := domain.FindTimelineClip(v1.Document, "clip_1")
	v2Clip, _ := domain.FindTimelineClip(v2.Document, "clip_1")
	v3Clip, _ := domain.FindTimelineClip(v3.Document, "clip_1")
	if !reflect.DeepEqual(v3Clip, v1Clip) {
		t.Fatalf("undo did not restore V1 semantic clip: v1=%#v v3=%#v", v1Clip, v3Clip)
	}
	if reflect.DeepEqual(v2Clip, v1Clip) || v2Clip["origin"] != "user" || v2Clip["proposal_refs"].([]any)[0] != "proposal_1" || v2Clip["evidence_refs"].([]any)[0] != "evidence_1" {
		t.Fatalf("V2 was mutated or lost provenance: %#v", v2Clip)
	}
	if _, err := store.AddTimelineVersion(store.WorkspaceID(), project.ID, timeline.ID, v1.ID, "user", v2Document, 3); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale historical base was accepted after V3: %v", err)
	}
	if _, err := store.AddTimelineVersion(store.WorkspaceID(), project.ID, timeline.ID, "", "user", v2Document, 2); !errors.Is(err, ErrConflict) {
		t.Fatalf("timeline edit without an exact base version was accepted: %v", err)
	}
}

func assertStoredTimelineIdentity(t *testing.T, timeline *Timeline, version *TimelineVersion) {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal(version.Document, &root); err != nil {
		t.Fatal(err)
	}
	if root["timeline_id"] != timeline.ID || root["timeline_version_id"] != version.ID || root["version"] != float64(version.Version) {
		t.Fatalf("stored document identity does not match metadata: timeline=%+v version=%+v document=%s", timeline, version, version.Document)
	}
	hash, err := domain.ValidateTimeline(version.Document, domain.TimelineValidationOptions{StructuralOnly: true})
	if err != nil || hash != version.ContentHash {
		t.Fatalf("stored content hash does not cover canonical document: hash=%q stored=%q err=%v", hash, version.ContentHash, err)
	}
}

func TestStoreRenderProfileLifecycleAndExactRenderSnapshot(t *testing.T) {
	store := NewStore("ws_test")
	project := mustProject(t, store, "render")
	profile, err := store.CreateRenderProfile(store.WorkspaceID(), "youtube_16_9", 1, map[string]any{"width": 1920, "height": 1080, "video_codec": "h264"})
	if err != nil {
		t.Fatal(err)
	}
	profile, err = store.UpdateRenderProfile(store.WorkspaceID(), profile.ProfileKey, profile.Version, map[string]any{"width": 1280, "height": 720, "video_codec": "h264"})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Status != "draft" {
		t.Fatalf("draft edit changed lifecycle: %#v", profile)
	}
	if _, err := store.SetRenderProfileStatus(store.WorkspaceID(), profile.ProfileKey, profile.Version, "active"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateRenderProfile(store.WorkspaceID(), profile.ProfileKey, profile.Version, map[string]any{"width": 640}); !errors.Is(err, ErrConflict) {
		t.Fatalf("active profile was mutable: %v", err)
	}

	document := timelineDocumentFor(project.ID, map[string]any{"type": "none", "inline_id": "silence"}, nil)
	timeline, version, err := store.CreateTimeline(store.WorkspaceID(), project.ID, "user", document)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetTimelineVersionStatus(store.WorkspaceID(), project.ID, timeline.ID, version.ID, "approved"); err != nil {
		t.Fatal(err)
	}
	render, err := store.CreateRender(store.WorkspaceID(), project.ID, version.ID, profile.ProfileKey, profile.Version, map[string]any{"overrides": map[string]any{"preview": false}})
	if err != nil {
		t.Fatal(err)
	}
	if render.ProfileID == "" || render.ProfileSnapshot["width"] != float64(1280) {
		t.Fatalf("render did not retain exact active profile snapshot: %#v", render)
	}
	if _, err := store.CreateRender(store.WorkspaceID(), project.ID, version.ID, "missing", 1, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing profile did not fail explicitly: %v", err)
	}
	draft, err := store.CreateRenderProfile(store.WorkspaceID(), "draft_profile", 1, map[string]any{"width": 640})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateRender(store.WorkspaceID(), project.ID, version.ID, draft.ProfileKey, draft.Version, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("draft profile was eligible for render: %v", err)
	}
	if _, err := store.SetRenderProfileStatus(store.WorkspaceID(), profile.ProfileKey, profile.Version, "deprecated"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateRender(store.WorkspaceID(), project.ID, version.ID, profile.ProfileKey, profile.Version, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("deprecated profile was eligible for new render: %v", err)
	}
	profileV2, err := store.CreateRenderProfile(store.WorkspaceID(), profile.ProfileKey, 2, map[string]any{"width": 1080, "height": 1920})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetRenderProfileStatus(store.WorkspaceID(), profileV2.ProfileKey, profileV2.Version, "active"); err != nil {
		t.Fatal(err)
	}
	if render.ProfileVersion != 1 || render.ProfileSnapshot["width"] != float64(1280) {
		t.Fatal("later profile version mutated existing render semantics")
	}
}
