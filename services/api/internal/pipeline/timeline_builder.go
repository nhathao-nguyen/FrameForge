package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
)

type ClipProposal struct {
	ID           string
	TimelineIn   float64
	TimelineOut  float64
	Source       map[string]any
	Origin       string
	ProposalRefs []string
	EvidenceRefs []string
	Metadata     map[string]any
	Subtitle     map[string]any
	Narration    map[string]any
}

type TimelineTrack struct {
	ID    string
	Kind  string
	Name  string
	Order int
	Clips []ClipProposal
}

type ClipOverride struct {
	ClipID string
	Fields map[string]any
}

type DegradedInput struct {
	NodeKey      string
	Reason       string
	Consequence  string
	Soft         bool
	EvidenceRefs []string
}

type BuildTimelineInput struct {
	TimelineID        string
	TimelineVersionID string
	ProjectID         string
	Version           int
	DurationSec       float64
	Tracks            []TimelineTrack
	Overrides         []ClipOverride
	Degraded          []DegradedInput
	Metadata          map[string]any
}

type BuiltTimeline struct {
	Document         json.RawMessage
	ContentHash      string
	InputFingerprint string
	Origin           string
	// Degraded is execution metadata.  It is deliberately not embedded in the
	// canonical TimelineVersion document, whose schema is renderer-authoritative.
	Degraded []DegradedInput
}

// BuildTimeline compiles selected proposals into the canonical renderer input.
// It intentionally accepts only references and typed values; it never resolves
// a scene match or executes a media operation.
func BuildTimeline(input BuildTimelineInput, resolver domain.TimelineReferenceResolver) (BuiltTimeline, error) {
	if input.TimelineID == "" || input.TimelineVersionID == "" || input.ProjectID == "" || input.Version < 1 || input.DurationSec <= 0 || len(input.Tracks) == 0 {
		return BuiltTimeline{}, errors.New("timeline identity, positive version/duration and tracks are required")
	}
	overrides := make(map[string]map[string]any, len(input.Overrides))
	for _, override := range input.Overrides {
		if override.ClipID == "" || len(override.Fields) == 0 || overrides[override.ClipID] != nil {
			return BuiltTimeline{}, errors.New("timeline overrides must have unique clip IDs and fields")
		}
		overrides[override.ClipID] = cloneMap(override.Fields)
	}
	degraded := make([]DegradedInput, 0, len(input.Degraded))
	for _, item := range input.Degraded {
		if item.NodeKey == "" || item.Reason == "" || item.Consequence == "" {
			return BuiltTimeline{}, errors.New("degraded input requires node, reason and consequence")
		}
		degraded = append(degraded, DegradedInput{
			NodeKey: item.NodeKey, Reason: item.Reason, Consequence: item.Consequence,
			Soft: item.Soft, EvidenceRefs: append([]string(nil), item.EvidenceRefs...),
		})
	}
	trackIDs := map[string]bool{}
	clipIDs := map[string]bool{}
	tracks := make([]any, 0, len(input.Tracks))
	for _, track := range input.Tracks {
		if track.ID == "" || track.Name == "" || trackIDs[track.ID] {
			return BuiltTimeline{}, fmt.Errorf("track %q is missing or duplicated", track.ID)
		}
		trackIDs[track.ID] = true
		clips := make([]any, 0, len(track.Clips))
		for _, proposal := range track.Clips {
			if proposal.ID == "" || clipIDs[proposal.ID] || proposal.TimelineOut <= proposal.TimelineIn {
				return BuiltTimeline{}, fmt.Errorf("clip %q is missing, duplicated or has invalid range", proposal.ID)
			}
			origin, err := checkedOrigin(proposal.Origin)
			if err != nil {
				return BuiltTimeline{}, fmt.Errorf("clip %q: %w", proposal.ID, err)
			}
			clipIDs[proposal.ID] = true
			clip := map[string]any{
				"id": proposal.ID, "timeline_in_sec": proposal.TimelineIn, "timeline_out_sec": proposal.TimelineOut,
				"source": cloneMap(proposal.Source), "origin": origin,
			}
			if len(proposal.ProposalRefs) > 0 {
				clip["proposal_refs"] = append([]string(nil), proposal.ProposalRefs...)
			}
			if len(proposal.EvidenceRefs) > 0 {
				clip["evidence_refs"] = append([]string(nil), proposal.EvidenceRefs...)
			}
			if len(proposal.Metadata) > 0 {
				clip["metadata"] = cloneMap(proposal.Metadata)
			}
			if len(proposal.Subtitle) > 0 {
				clip["subtitle"] = cloneMap(proposal.Subtitle)
			}
			if len(proposal.Narration) > 0 {
				clip["narration"] = cloneMap(proposal.Narration)
			}
			if fields := overrides[proposal.ID]; fields != nil {
				if err := applyOverride(clip, fields); err != nil {
					return BuiltTimeline{}, fmt.Errorf("clip %q override: %w", proposal.ID, err)
				}
				clip["origin"] = "user"
			}
			clips = append(clips, clip)
		}
		tracks = append(tracks, map[string]any{"id": track.ID, "kind": track.Kind, "name": track.Name, "order": track.Order, "clips": clips})
	}
	metadata := cloneMap(input.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	document := map[string]any{
		"schema_version": "1.0", "timeline_id": input.TimelineID, "timeline_version_id": input.TimelineVersionID,
		"project_id": input.ProjectID, "version": input.Version, "duration_sec": input.DurationSec,
		"tracks": tracks, "metadata": metadata,
	}
	canonical, err := domain.CanonicalJSON(document)
	if err != nil {
		return BuiltTimeline{}, err
	}
	validation := domain.TimelineValidationOptions{Resolver: resolver}
	if resolver == nil {
		// Proposal compilation is a structural step. The persistence/API
		// boundary performs the project-owned durable-reference resolution
		// before accepting the immutable TimelineVersion.
		validation.StructuralOnly = true
	}
	hash, err := domain.ValidateTimeline(canonical, validation)
	if err != nil {
		return BuiltTimeline{}, err
	}
	fingerprintPayload := map[string]any{"document": json.RawMessage(canonical), "degraded": degraded, "proposal_count": len(clipIDs), "override_count": len(overrides)}
	fingerprintBytes, _ := json.Marshal(fingerprintPayload)
	digest := sha256.Sum256(fingerprintBytes)
	return BuiltTimeline{Document: canonical, ContentHash: hash, InputFingerprint: hex.EncodeToString(digest[:]), Origin: "system", Degraded: degraded}, nil
}

func applyOverride(clip map[string]any, fields map[string]any) error {
	allowed := map[string]bool{"timeline_in_sec": true, "timeline_out_sec": true, "source_in_sec": true, "source_out_sec": true, "transform": true, "subtitle": true, "narration": true, "metadata": true}
	for key, value := range fields {
		if !allowed[key] {
			return fmt.Errorf("field %q is not editable", key)
		}
		clip[key] = value
	}
	return nil
}

func checkedOrigin(origin string) (string, error) {
	switch origin {
	case "ai", "user", "imported", "system":
		return origin, nil
	default:
		return "", fmt.Errorf("unsupported origin %q", origin)
	}
}

func SortTracks(tracks []TimelineTrack) {
	sort.SliceStable(tracks, func(i, j int) bool {
		if tracks[i].Order == tracks[j].Order {
			return tracks[i].ID < tracks[j].ID
		}
		return tracks[i].Order < tracks[j].Order
	})
}
