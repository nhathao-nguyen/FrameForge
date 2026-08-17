package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

const TimelineSchemaVersion = "1.0"

type TimelineReferenceResolver interface {
	ResolveAssetArtifact(assetID, artifactID, projectID string) (ready bool, durationSec float64, width, height int, err error)
	ResolveScene(sceneID, assetID, artifactID, projectID string) (ready bool, startSec, endSec float64, err error)
	ResolveArtifact(artifactID, projectID string) (committed bool, err error)
	ResolveNarration(narrationID, scriptVersionID, projectID string) (ready bool, err error)
}

type TimelineValidationOptions struct {
	Resolver            TimelineReferenceResolver
	ReplaceUserOverride bool
}

func CanonicalJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var normalized any
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&normalized); err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

func TimelineContentHash(document any) (string, error) {
	canonical, err := CanonicalJSON(document)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func ValidateTimeline(document json.RawMessage, options TimelineValidationOptions) (string, error) {
	var root map[string]any
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return "", fmt.Errorf("timeline JSON: %w", err)
	}
	if root == nil {
		return "", errors.New("timeline must be an object")
	}
	if err := validateTimelineSafety(root, "timeline"); err != nil {
		return "", err
	}
	if value, ok := root["schema_version"].(string); !ok || value != TimelineSchemaVersion {
		return "", fmt.Errorf("timeline schema_version must be %s", TimelineSchemaVersion)
	}
	for _, field := range []string{"timeline_id", "timeline_version_id", "project_id"} {
		value, ok := root[field].(string)
		if !ok || strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("timeline %s is required", field)
		}
	}
	duration, err := number(root, "duration_sec")
	if err != nil || duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return "", errors.New("timeline duration_sec must be a positive finite number")
	}
	tracks, ok := root["tracks"].([]any)
	if !ok || len(tracks) == 0 || len(tracks) > 256 {
		return "", errors.New("timeline tracks must contain 1..256 tracks")
	}
	trackIDs := map[string]struct{}{}
	clipIDs := map[string]struct{}{}
	projectID := root["project_id"].(string)
	for trackIndex, rawTrack := range tracks {
		track, ok := rawTrack.(map[string]any)
		if !ok {
			return "", fmt.Errorf("track %d must be an object", trackIndex)
		}
		trackID, ok := track["id"].(string)
		if !ok || trackID == "" {
			return "", fmt.Errorf("track %d id is required", trackIndex)
		}
		if _, exists := trackIDs[trackID]; exists {
			return "", fmt.Errorf("duplicate track id %q", trackID)
		}
		trackIDs[trackID] = struct{}{}
		kind, _ := track["kind"].(string)
		if !validTrackKind(kind) {
			return "", fmt.Errorf("track %q has invalid kind", trackID)
		}
		clips, ok := track["clips"].([]any)
		if !ok || len(clips) > 100000 {
			return "", fmt.Errorf("track %q clips must be an array", trackID)
		}
		for clipIndex, rawClip := range clips {
			clip, ok := rawClip.(map[string]any)
			if !ok {
				return "", fmt.Errorf("clip %d in %q must be an object", clipIndex, trackID)
			}
			clipID, ok := clip["id"].(string)
			if !ok || clipID == "" {
				return "", fmt.Errorf("clip %d in %q id is required", clipIndex, trackID)
			}
			if _, exists := clipIDs[clipID]; exists {
				return "", fmt.Errorf("duplicate clip id %q", clipID)
			}
			clipIDs[clipID] = struct{}{}
			in, inErr := number(clip, "timeline_in_sec")
			out, outErr := number(clip, "timeline_out_sec")
			if inErr != nil || outErr != nil || in < 0 || out <= in || out > duration+0.001 {
				return "", fmt.Errorf("clip %q has invalid timeline range", clipID)
			}
			origin, _ := clip["origin"].(string)
			if !validOrigin(origin) {
				return "", fmt.Errorf("clip %q has invalid origin", clipID)
			}
			if origin == "user" && options.ReplaceUserOverride {
				// The command layer must authorize this flag. The validator accepts it
				// only as an explicit option, never as an implicit proposal merge.
			}
			if err := validateClipSource(clip, projectID, options.Resolver); err != nil {
				return "", fmt.Errorf("clip %q: %w", clipID, err)
			}
			if kind == "subtitle" && clip["subtitle"] == nil {
				return "", fmt.Errorf("subtitle track clip %q requires subtitle cue", clipID)
			}
			if narration, ok := clip["narration"].(map[string]any); ok && options.Resolver != nil {
				narrationID, _ := narration["narration_id"].(string)
				scriptVersionID, _ := narration["script_version_id"].(string)
				ready, err := options.Resolver.ResolveNarration(narrationID, scriptVersionID, projectID)
				if err != nil || !ready {
					return "", errors.New("narration reference is not ready or not owned by the project")
				}
			}
		}
	}
	if markers, ok := root["markers"].([]any); ok {
		for _, rawMarker := range markers {
			marker, ok := rawMarker.(map[string]any)
			if !ok {
				return "", errors.New("marker must be an object")
			}
			at, err := number(marker, "time_sec")
			if err != nil || at < 0 || at > duration+0.001 {
				return "", errors.New("marker time is outside timeline duration")
			}
		}
	}
	return TimelineContentHash(root)
}

func validateTimelineSafety(value any, path string) error {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			lowerKey := strings.ToLower(key)
			if strings.Contains(lowerKey, "secret") || strings.Contains(lowerKey, "password") || strings.Contains(lowerKey, "token") || strings.Contains(lowerKey, "credential") || strings.Contains(lowerKey, "presigned") {
				return fmt.Errorf("timeline contains forbidden sensitive field %q", path+"."+key)
			}
			if strings.Contains(lowerKey, "url") || strings.Contains(lowerKey, "path") {
				return fmt.Errorf("timeline contains forbidden external path field %q", path+"."+key)
			}
			if err := validateTimelineSafety(child, path+"."+key); err != nil {
				return err
			}
		}
	case []any:
		for index, child := range current {
			if err := validateTimelineSafety(child, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	case string:
		lower := strings.ToLower(strings.TrimSpace(current))
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "file://") || strings.HasPrefix(lower, "//") || strings.HasPrefix(lower, `\\`) || (len(lower) >= 3 && lower[1] == ':' && (lower[2] == '/' || lower[2] == '\\')) {
			return fmt.Errorf("timeline contains forbidden external path or URL at %s", path)
		}
	}
	return nil
}

func validateClipSource(clip map[string]any, projectID string, resolver TimelineReferenceResolver) error {
	source, ok := clip["source"].(map[string]any)
	if !ok {
		return errors.New("source is required")
	}
	typeName, _ := source["type"].(string)
	if resolver == nil {
		return nil
	}
	switch typeName {
	case "asset":
		ready, duration, width, height, err := resolver.ResolveAssetArtifact(stringValue(source, "asset_id"), stringValue(source, "artifact_id"), projectID)
		if err != nil || !ready {
			return errors.New("asset/artifact is not ready or not owned by the project")
		}
		return validateSourceRange(clip, duration, width, height)
	case "scene":
		ready, start, end, err := resolver.ResolveScene(stringValue(source, "scene_id"), stringValue(source, "asset_id"), stringValue(source, "artifact_id"), projectID)
		if err != nil || !ready {
			return errors.New("scene source is not ready or not owned by the project")
		}
		return validateSourceRange(clip, end-start, 0, 0)
	case "artifact":
		committed, err := resolver.ResolveArtifact(stringValue(source, "artifact_id"), projectID)
		if err != nil || !committed {
			return errors.New("artifact is not committed or not owned by the project")
		}
	case "generated", "none":
		return nil
	default:
		return fmt.Errorf("unsupported source type %q", typeName)
	}
	return nil
}

func validateSourceRange(clip map[string]any, duration float64, width, height int) error {
	in, hasIn := numberOptional(clip, "source_in_sec")
	out, hasOut := numberOptional(clip, "source_out_sec")
	if hasIn != hasOut {
		return errors.New("source range must include both source_in_sec and source_out_sec")
	}
	if !hasIn {
		return nil
	}
	if in < 0 || out <= in || (duration > 0 && out > duration+0.001) {
		return errors.New("source range is outside exact source duration")
	}
	if crop, ok := clip["transform"].(map[string]any); ok {
		if cropValue, ok := crop["crop"].(map[string]any); ok && cropValue["unit"] == "pixels" && width > 0 && height > 0 {
			x, _ := numberOptional(cropValue, "x")
			y, _ := numberOptional(cropValue, "y")
			w, _ := numberOptional(cropValue, "width")
			h, _ := numberOptional(cropValue, "height")
			if x+w > float64(width)+0.001 || y+h > float64(height)+0.001 {
				return errors.New("pixel crop is outside source dimensions")
			}
		}
	}
	return nil
}

func number(value map[string]any, key string) (float64, error) {
	result, ok := numberOptional(value, key)
	if !ok {
		return 0, errors.New("number is required")
	}
	return result, nil
}

func numberOptional(value map[string]any, key string) (float64, bool) {
	raw, ok := value[key]
	if !ok {
		return 0, false
	}
	switch number := raw.(type) {
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil
	case float64:
		return number, true
	default:
		return 0, false
	}
}

func stringValue(value map[string]any, key string) string {
	result, _ := value[key].(string)
	return result
}
func validOrigin(value string) bool {
	return value == "ai" || value == "user" || value == "imported" || value == "system"
}
func validTrackKind(value string) bool {
	return value == "video" || value == "narration" || value == "music" || value == "sfx" || value == "subtitle" || value == "overlay"
}

type TimelineCommand struct {
	Kind                string         `json:"kind"`
	SchemaVersion       string         `json:"schema_version"`
	BasedOnVersionID    string         `json:"based_on_version_id"`
	ExpectedVersion     int            `json:"expected_version"`
	ReplaceUserOverride bool           `json:"replace_user_override,omitempty"`
	Payload             map[string]any `json:"payload"`
}

func ApplyTimelineCommand(document json.RawMessage, command TimelineCommand) (json.RawMessage, string, error) {
	if command.SchemaVersion == "" {
		command.SchemaVersion = "1.0"
	}
	if command.SchemaVersion != "1.0" || command.Kind == "" {
		return nil, "", errors.New("unsupported timeline command")
	}
	var root map[string]any
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return nil, "", err
	}
	if command.ExpectedVersion > 0 {
		version, _ := root["version"].(json.Number)
		if version.String() != fmt.Sprint(command.ExpectedVersion) {
			return nil, "", fmt.Errorf("timeline version conflict: expected %d", command.ExpectedVersion)
		}
	}
	tracks, ok := root["tracks"].([]any)
	if !ok {
		return nil, "", errors.New("timeline tracks are required")
	}
	findClip := func(id string) (map[string]any, map[string]any, error) {
		for _, rawTrack := range tracks {
			track, _ := rawTrack.(map[string]any)
			clips, _ := track["clips"].([]any)
			for _, rawClip := range clips {
				clip, _ := rawClip.(map[string]any)
				if stringValue(clip, "id") == id {
					return track, clip, nil
				}
			}
		}
		return nil, nil, fmt.Errorf("clip %q not found", id)
	}
	switch command.Kind {
	case "AddClip":
		trackID := stringValue(command.Payload, "track_id")
		clip, ok := command.Payload["clip"].(map[string]any)
		if !ok || trackID == "" {
			return nil, "", errors.New("AddClip requires track_id and clip")
		}
		for _, rawTrack := range tracks {
			track, _ := rawTrack.(map[string]any)
			if stringValue(track, "id") == trackID {
				clips, _ := track["clips"].([]any)
				track["clips"] = append(clips, clip)
				break
			}
		}
	case "RemoveClip":
		track, _, err := findClip(stringValue(command.Payload, "clip_id"))
		if err != nil {
			return nil, "", err
		}
		clips, _ := track["clips"].([]any)
		id := stringValue(command.Payload, "clip_id")
		filtered := clips[:0]
		for _, rawClip := range clips {
			if stringValue(rawClip.(map[string]any), "id") != id {
				filtered = append(filtered, rawClip)
			}
		}
		track["clips"] = filtered
	case "MoveClip":
		from, clip, err := findClip(stringValue(command.Payload, "clip_id"))
		if err != nil {
			return nil, "", err
		}
		toID := stringValue(command.Payload, "track_id")
		clips, _ := from["clips"].([]any)
		id := stringValue(command.Payload, "clip_id")
		filtered := clips[:0]
		for _, rawClip := range clips {
			if stringValue(rawClip.(map[string]any), "id") != id {
				filtered = append(filtered, rawClip)
			}
		}
		from["clips"] = filtered
		for _, rawTrack := range tracks {
			track, _ := rawTrack.(map[string]any)
			if stringValue(track, "id") == toID {
				target, _ := track["clips"].([]any)
				track["clips"] = append(target, clip)
				break
			}
		}
	case "TrimClip":
		_, clip, err := findClip(stringValue(command.Payload, "clip_id"))
		if err != nil {
			return nil, "", err
		}
		for _, key := range []string{"timeline_in_sec", "timeline_out_sec", "source_in_sec", "source_out_sec"} {
			if value, exists := command.Payload[key]; exists {
				clip[key] = value
			}
		}
	case "UpdateSubtitle":
		_, clip, err := findClip(stringValue(command.Payload, "clip_id"))
		if err != nil {
			return nil, "", err
		}
		clip["subtitle"] = command.Payload["subtitle"]
	case "ReplaceNarration":
		_, clip, err := findClip(stringValue(command.Payload, "clip_id"))
		if err != nil {
			return nil, "", err
		}
		clip["narration"] = command.Payload["narration"]
		clip["origin"] = "user"
	case "ChangeTrackOrder":
		trackID := stringValue(command.Payload, "track_id")
		order := command.Payload["order"]
		for _, rawTrack := range tracks {
			track, _ := rawTrack.(map[string]any)
			if stringValue(track, "id") == trackID {
				track["order"] = order
				break
			}
		}
	case "UpdateScene":
		_, clip, err := findClip(stringValue(command.Payload, "clip_id"))
		if err != nil {
			return nil, "", err
		}
		clip["metadata"] = command.Payload["metadata"]
		clip["origin"] = "user"
	default:
		return nil, "", fmt.Errorf("unsupported timeline command %q", command.Kind)
	}
	if command.ExpectedVersion > 0 {
		root["version"] = command.ExpectedVersion + 1
	}
	result, err := CanonicalJSON(root)
	if err != nil {
		return nil, "", err
	}
	hash, err := ValidateTimeline(result, TimelineValidationOptions{ReplaceUserOverride: command.ReplaceUserOverride})
	return result, hash, err
}
