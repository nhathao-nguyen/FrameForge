package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	sharedschema "github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/schema"
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
	// StructuralOnly is used while applying a command. Durable references are
	// checked for identity and shape here, then resolved against project-owned
	// current state by the persistence boundary before the version is stored.
	StructuralOnly bool
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

// NormalizeTimelineIdentity applies server-owned aggregate identity to a
// document before it is validated or persisted. Client-provided identifiers
// are accepted for create/import input, but a stored TimelineVersion must
// describe the exact Timeline and immutable version row that owns it.
func NormalizeTimelineIdentity(document json.RawMessage, timelineID, versionID string, version int) (json.RawMessage, error) {
	if strings.TrimSpace(timelineID) == "" || strings.TrimSpace(versionID) == "" || version < 1 {
		return nil, errors.New("timeline identity is incomplete")
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("timeline JSON: %w", err)
	}
	if root == nil {
		return nil, errors.New("timeline must be an object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, errors.New("timeline JSON contains trailing values")
		}
		return nil, fmt.Errorf("timeline JSON: %w", err)
	}
	root["timeline_id"] = timelineID
	root["timeline_version_id"] = versionID
	root["version"] = version
	return CanonicalJSON(root)
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
	if err := sharedschema.ValidateTimelineSchema(document); err != nil {
		return "", fmt.Errorf("timeline schema: %w", err)
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
	trackOrders := map[int]struct{}{}
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
		order, orderOK := integerValue(track, "order")
		if !orderOK {
			return "", fmt.Errorf("track %q order is required", trackID)
		}
		if _, exists := trackOrders[order]; exists {
			return "", fmt.Errorf("track order %d is duplicated", order)
		}
		trackOrders[order] = struct{}{}
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
			if err := validateClipSource(clip, projectID, options); err != nil {
				return "", fmt.Errorf("clip %q: %w", clipID, err)
			}
			if err := validateClipTiming(clip, out-in); err != nil {
				return "", fmt.Errorf("clip %q: %w", clipID, err)
			}
			if err := validateClipTransform(clip); err != nil {
				return "", fmt.Errorf("clip %q: %w", clipID, err)
			}
			if kind == "subtitle" && clip["subtitle"] == nil {
				return "", fmt.Errorf("subtitle track clip %q requires subtitle cue", clipID)
			}
			if kind == "music" {
				source := clip["source"].(map[string]any)
				status, _ := source["rights_status"].(string)
				metadata, _ := source["rights_metadata"].(map[string]any)
				if status != "owned" && status != "cleared" {
					return "", fmt.Errorf("music track clip %q is rejected by the BGM rights policy", clipID)
				}
				if len(metadata) == 0 {
					return "", fmt.Errorf("music track clip %q requires rights metadata", clipID)
				}
			}
			if err := validateSubtitleCue(clip, out-in); err != nil {
				return "", fmt.Errorf("clip %q: %w", clipID, err)
			}
			if err := validateClipTransitions(clip, out-in); err != nil {
				return "", fmt.Errorf("clip %q: %w", clipID, err)
			}
			if kind == "narration" {
				if _, ok := clip["narration"].(map[string]any); !ok {
					return "", fmt.Errorf("narration track clip %q requires narration reference", clipID)
				}
				source := clip["source"].(map[string]any)
				if source["type"] != "artifact" {
					return "", fmt.Errorf("narration track clip %q requires artifact source", clipID)
				}
			}
			if narration, ok := clip["narration"].(map[string]any); ok {
				narrationID, _ := narration["narration_id"].(string)
				scriptVersionID, _ := narration["script_version_id"].(string)
				if strings.TrimSpace(narrationID) == "" || strings.TrimSpace(scriptVersionID) == "" {
					return "", errors.New("narration reference identity is required")
				}
				if options.Resolver == nil {
					if options.StructuralOnly {
						continue
					}
					return "", errors.New("narration reference resolver is required")
				}
				ready, err := options.Resolver.ResolveNarration(narrationID, scriptVersionID, projectID)
				if err != nil || !ready {
					return "", errors.New("narration reference is not ready or not owned by the project")
				}
			}
		}
		if err := validateTrackOverlap(kind, clips); err != nil {
			return "", fmt.Errorf("track %q: %w", trackID, err)
		}
	}
	if markers, ok := root["markers"].([]any); ok {
		markerIDs := map[string]struct{}{}
		for _, rawMarker := range markers {
			marker, ok := rawMarker.(map[string]any)
			if !ok {
				return "", errors.New("marker must be an object")
			}
			markerID := stringValue(marker, "id")
			if _, exists := markerIDs[markerID]; exists {
				return "", fmt.Errorf("duplicate marker id %q", markerID)
			}
			markerIDs[markerID] = struct{}{}
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

func validateClipSource(clip map[string]any, projectID string, options TimelineValidationOptions) error {
	source, ok := clip["source"].(map[string]any)
	if !ok {
		return errors.New("source is required")
	}
	typeName, _ := source["type"].(string)
	if typeName == "generated" {
		if stringValue(source, "generator_ref") == "" {
			return errors.New("generated source requires generator_ref")
		}
		if artifactID := stringValue(source, "artifact_id"); artifactID != "" && options.Resolver != nil {
			committed, err := options.Resolver.ResolveArtifact(artifactID, projectID)
			if err != nil || !committed {
				return errors.New("generated artifact is not committed or not owned by the project")
			}
		}
		return nil
	}
	if typeName == "none" {
		if stringValue(source, "inline_id") == "" {
			return errors.New("none source requires inline_id")
		}
		if _, hasIn := numberOptional(clip, "source_in_sec"); hasIn {
			return errors.New("inline source cannot have a media source range")
		}
		if _, hasOut := numberOptional(clip, "source_out_sec"); hasOut {
			return errors.New("inline source cannot have a media source range")
		}
		return nil
	}
	if options.Resolver == nil && !options.StructuralOnly {
		return errors.New("durable source reference resolver is required")
	}
	switch typeName {
	case "asset":
		assetID, artifactID := stringValue(source, "asset_id"), stringValue(source, "artifact_id")
		if assetID == "" || artifactID == "" {
			return errors.New("asset source requires asset_id and artifact_id")
		}
		if options.Resolver == nil {
			return validateSourceRange(clip, 0, 0, 0, 0)
		}
		ready, duration, width, height, err := options.Resolver.ResolveAssetArtifact(assetID, artifactID, projectID)
		if err != nil || !ready {
			return errors.New("asset/artifact is not ready or not owned by the project")
		}
		return validateSourceRange(clip, 0, duration, width, height)
	case "scene":
		sceneID, assetID, artifactID := stringValue(source, "scene_id"), stringValue(source, "asset_id"), stringValue(source, "artifact_id")
		if sceneID == "" || assetID == "" || artifactID == "" {
			return errors.New("scene source requires scene_id, asset_id and artifact_id")
		}
		if options.Resolver == nil {
			return validateSourceRange(clip, 0, 0, 0, 0)
		}
		ready, start, end, err := options.Resolver.ResolveScene(sceneID, assetID, artifactID, projectID)
		if err != nil || !ready {
			return errors.New("scene source is not ready or not owned by the project")
		}
		return validateSourceRange(clip, start, end, 0, 0)
	case "artifact":
		artifactID := stringValue(source, "artifact_id")
		if artifactID == "" {
			return errors.New("artifact source requires artifact_id")
		}
		if options.Resolver == nil {
			return validateSourceRange(clip, 0, 0, 0, 0)
		}
		committed, err := options.Resolver.ResolveArtifact(artifactID, projectID)
		if err != nil || !committed {
			return errors.New("artifact is not committed or not owned by the project")
		}
	default:
		return fmt.Errorf("unsupported source type %q", typeName)
	}
	return nil
}

func validateSourceRange(clip map[string]any, lowerBound, upperBound float64, width, height int) error {
	in, hasIn := numberOptional(clip, "source_in_sec")
	out, hasOut := numberOptional(clip, "source_out_sec")
	if hasIn != hasOut {
		return errors.New("source range must include both source_in_sec and source_out_sec")
	}
	if !hasIn {
		return nil
	}
	if in < lowerBound-0.001 || in < 0 || out <= in || (upperBound > 0 && out > upperBound+0.001) {
		return errors.New("source range is outside exact source duration")
	}
	if crop, ok := clip["transform"].(map[string]any); ok {
		if cropValue, ok := crop["crop"].(map[string]any); ok && cropValue["unit"] == "normalized" {
			x, _ := numberOptional(cropValue, "x")
			y, _ := numberOptional(cropValue, "y")
			w, _ := numberOptional(cropValue, "width")
			h, _ := numberOptional(cropValue, "height")
			if x+w > 1.001 || y+h > 1.001 || x > 1 || y > 1 || w > 1 || h > 1 {
				return errors.New("normalized crop is outside source bounds")
			}
		} else if cropValue, ok := crop["crop"].(map[string]any); ok && cropValue["unit"] == "pixels" && width > 0 && height > 0 {
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

func validateClipTiming(clip map[string]any, timelineDuration float64) error {
	source, _ := clip["source"].(map[string]any)
	if source == nil || source["type"] == "none" {
		return nil
	}
	in, hasIn := numberOptional(clip, "source_in_sec")
	out, hasOut := numberOptional(clip, "source_out_sec")
	if !hasIn || !hasOut || clip["loop"] == true || clip["freeze_frame"] == true {
		return nil
	}
	speed := 1.0
	if value, ok := numberOptional(clip, "speed"); ok {
		speed = value
	}
	if speed <= 0 {
		return errors.New("speed must be positive")
	}
	if math.Abs((out-in)/speed-timelineDuration) > 0.001 {
		return errors.New("source and timeline durations do not match speed")
	}
	return nil
}

func validateClipTransform(clip map[string]any) error {
	transform, _ := clip["transform"].(map[string]any)
	if transform == nil {
		return nil
	}
	crop, _ := transform["crop"].(map[string]any)
	if crop == nil || crop["unit"] != "normalized" {
		return nil
	}
	x, _ := numberOptional(crop, "x")
	y, _ := numberOptional(crop, "y")
	width, _ := numberOptional(crop, "width")
	height, _ := numberOptional(crop, "height")
	if x > 1 || y > 1 || width > 1 || height > 1 || x+width > 1.001 || y+height > 1.001 {
		return errors.New("normalized crop is outside source bounds")
	}
	return nil
}

func validateSubtitleCue(clip map[string]any, clipDuration float64) error {
	cue, ok := clip["subtitle"].(map[string]any)
	if !ok {
		return nil
	}
	words, _ := cue["words"].([]any)
	previousEnd := 0.0
	for index, rawWord := range words {
		word, ok := rawWord.(map[string]any)
		if !ok {
			return fmt.Errorf("subtitle word %d must be an object", index)
		}
		start, startOK := numberOptional(word, "start_offset_sec")
		end, endOK := numberOptional(word, "end_offset_sec")
		if !startOK || !endOK || start < previousEnd || end <= start || end > clipDuration+0.001 {
			return fmt.Errorf("subtitle word %d has invalid timing", index)
		}
		previousEnd = end
	}
	return nil
}

func validateClipTransitions(clip map[string]any, clipDuration float64) error {
	for _, field := range []string{"transition_in", "transition_out"} {
		value, exists := clip[field]
		if !exists {
			continue
		}
		transition, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be an object", field)
		}
		duration, err := number(transition, "duration_sec")
		if err != nil || duration < 0 || duration > clipDuration+0.001 {
			return fmt.Errorf("%s duration exceeds clip duration", field)
		}
	}
	return nil
}

func validateTrackOverlap(kind string, rawClips []any) error {
	type interval struct {
		clip map[string]any
		in   float64
		out  float64
	}
	intervals := make([]interval, 0, len(rawClips))
	for _, rawClip := range rawClips {
		clip := rawClip.(map[string]any)
		in, _ := number(clip, "timeline_in_sec")
		out, _ := number(clip, "timeline_out_sec")
		intervals = append(intervals, interval{clip: clip, in: in, out: out})
	}
	sort.SliceStable(intervals, func(i, j int) bool {
		if intervals[i].in == intervals[j].in {
			return intervals[i].out < intervals[j].out
		}
		return intervals[i].in < intervals[j].in
	})
	for left := 0; left < len(intervals); left++ {
		for right := left + 1; right < len(intervals) && intervals[right].in < intervals[left].out-0.001; right++ {
			switch kind {
			case "music", "sfx":
				continue
			case "narration":
				return fmt.Errorf("clips %q and %q overlap on a narration track", stringValue(intervals[left].clip, "id"), stringValue(intervals[right].clip, "id"))
			default:
				if !explicitClipOverlap(intervals[left].clip, intervals[right].clip) {
					return fmt.Errorf("clips %q and %q overlap without an explicit transition", stringValue(intervals[left].clip, "id"), stringValue(intervals[right].clip, "id"))
				}
			}
		}
	}
	return nil
}

func explicitClipOverlap(left, right map[string]any) bool {
	_, leftTransition := left["transition_out"]
	_, rightTransition := right["transition_in"]
	return leftTransition || rightTransition
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

func integerValue(value map[string]any, key string) (int, bool) {
	raw, exists := value[key]
	if !exists {
		return 0, false
	}
	switch typed := raw.(type) {
	case json.Number:
		parsed, err := strconv.ParseInt(typed.String(), 10, 64)
		if err != nil || int64(int(parsed)) != parsed {
			return 0, false
		}
		return int(parsed), true
	case float64:
		if math.Trunc(typed) != typed {
			return 0, false
		}
		return int(typed), true
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

// FindTimelineClip returns an immutable copy of a clip from a validated
// TimelineVersion document. It is used by guarded rollback commands so the
// HTTP boundary can verify that a client is restoring an actual historical
// clip rather than inventing provenance.
func FindTimelineClip(document json.RawMessage, id string) (map[string]any, error) {
	var root map[string]any
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return nil, err
	}
	tracks, ok := root["tracks"].([]any)
	if !ok || strings.TrimSpace(id) == "" {
		return nil, errors.New("timeline clip lookup requires tracks and clip id")
	}
	for _, rawTrack := range tracks {
		track, _ := rawTrack.(map[string]any)
		clips, _ := track["clips"].([]any)
		for _, rawClip := range clips {
			clip, _ := rawClip.(map[string]any)
			if stringValue(clip, "id") == id {
				encoded, err := CanonicalJSON(clip)
				if err != nil {
					return nil, err
				}
				var copy map[string]any
				decoder := json.NewDecoder(bytes.NewReader(encoded))
				decoder.UseNumber()
				if err := decoder.Decode(&copy); err != nil {
					return nil, err
				}
				return copy, nil
			}
		}
	}
	return nil, fmt.Errorf("clip %q not found", id)
}

func ApplyTimelineCommand(document json.RawMessage, command TimelineCommand) (json.RawMessage, string, error) {
	if command.SchemaVersion == "" {
		command.SchemaVersion = "1.0"
	}
	if command.SchemaVersion != "1.0" || command.Kind == "" || command.ExpectedVersion < 1 {
		return nil, "", errors.New("unsupported timeline command")
	}
	var root map[string]any
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return nil, "", err
	}
	if command.ExpectedVersion > 0 {
		version, ok := integerValue(root, "version")
		if !ok || version != command.ExpectedVersion {
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
		foundTrack := false
		for _, rawTrack := range tracks {
			track, _ := rawTrack.(map[string]any)
			if stringValue(track, "id") == trackID {
				foundTrack = true
				clips, _ := track["clips"].([]any)
				track["clips"] = append(clips, clip)
				break
			}
		}
		if !foundTrack {
			return nil, "", fmt.Errorf("track %q not found", trackID)
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
		if toID == "" {
			return nil, "", errors.New("MoveClip requires track_id")
		}
		clips, _ := from["clips"].([]any)
		id := stringValue(command.Payload, "clip_id")
		filtered := clips[:0]
		for _, rawClip := range clips {
			if stringValue(rawClip.(map[string]any), "id") != id {
				filtered = append(filtered, rawClip)
			}
		}
		from["clips"] = filtered
		foundTarget := false
		for _, rawTrack := range tracks {
			track, _ := rawTrack.(map[string]any)
			if stringValue(track, "id") == toID {
				foundTarget = true
				target, _ := track["clips"].([]any)
				track["clips"] = append(target, clip)
				break
			}
		}
		if !foundTarget {
			return nil, "", fmt.Errorf("track %q not found", toID)
		}
	case "ReplaceClip":
		_, clip, err := findClip(stringValue(command.Payload, "clip_id"))
		if err != nil {
			return nil, "", err
		}
		replacement, ok := command.Payload["clip"].(map[string]any)
		if !ok {
			return nil, "", errors.New("ReplaceClip requires clip")
		}
		if replacementID := stringValue(replacement, "id"); replacementID != "" && replacementID != stringValue(clip, "id") {
			return nil, "", errors.New("ReplaceClip cannot change clip id")
		}
		for key, value := range replacement {
			switch key {
			case "id", "origin", "proposal_refs", "evidence_refs":
				continue
			default:
				clip[key] = value
			}
		}
		clip["origin"] = "user"
	case "RestoreClip":
		if stringValue(command.Payload, "restore_from_version_id") == "" {
			return nil, "", errors.New("RestoreClip requires restore_from_version_id")
		}
		_, clip, err := findClip(stringValue(command.Payload, "clip_id"))
		if err != nil {
			return nil, "", err
		}
		replacement, ok := command.Payload["clip"].(map[string]any)
		if !ok {
			return nil, "", errors.New("RestoreClip requires clip")
		}
		if replacementID := stringValue(replacement, "id"); replacementID != "" && replacementID != stringValue(clip, "id") {
			return nil, "", errors.New("RestoreClip cannot change clip id")
		}
		for key := range clip {
			delete(clip, key)
		}
		for key, value := range replacement {
			clip[key] = value
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
		foundTrack := false
		for _, rawTrack := range tracks {
			track, _ := rawTrack.(map[string]any)
			if stringValue(track, "id") == trackID {
				foundTrack = true
				track["order"] = order
				break
			}
		}
		if !foundTrack {
			return nil, "", fmt.Errorf("track %q not found", trackID)
		}
	case "UpdateScene":
		_, clip, err := findClip(stringValue(command.Payload, "clip_id"))
		if err != nil {
			return nil, "", err
		}
		metadata, ok := command.Payload["metadata"].(map[string]any)
		if !ok {
			return nil, "", errors.New("UpdateScene requires metadata")
		}
		existingMetadata, _ := clip["metadata"].(map[string]any)
		if existingMetadata == nil {
			existingMetadata = map[string]any{}
		}
		for key, value := range metadata {
			existingMetadata[key] = value
		}
		clip["metadata"] = existingMetadata
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
	hash, err := ValidateTimeline(result, TimelineValidationOptions{ReplaceUserOverride: command.ReplaceUserOverride, StructuralOnly: true})
	return result, hash, err
}
