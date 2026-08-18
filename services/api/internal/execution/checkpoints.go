package execution

import (
	"errors"
	"regexp"
)

var fingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Checkpoint struct {
	ID                string
	PipelineRunID     string
	JobStepID         string
	CompletedNodeKey  string
	ContextArtifactID string
	StateHash         string
	SchemaVersion     string
	InputFingerprint  string
	Valid             bool
}

func ValidateCheckpoint(value Checkpoint) error {
	if value.ID == "" || value.PipelineRunID == "" || value.JobStepID == "" || value.CompletedNodeKey == "" || value.SchemaVersion == "" || !fingerprintPattern.MatchString(value.StateHash) || !fingerprintPattern.MatchString(value.InputFingerprint) {
		return errors.New("checkpoint identity and hashes are required")
	}
	if value.ContextArtifactID == "" {
		return errors.New("checkpoint must reference an Artifact context")
	}
	return nil
}

func CanReuseCheckpoint(value Checkpoint, schemaVersion, inputFingerprint string) bool {
	return value.Valid && value.SchemaVersion == schemaVersion && value.InputFingerprint == inputFingerprint && fingerprintPattern.MatchString(inputFingerprint)
}
