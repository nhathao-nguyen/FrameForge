package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	CommandSchemaVersion  = "worker-command/v1"
	ResultSchemaVersion   = "worker-result/v1"
	ProgressSchemaVersion = "worker-progress/v1"
)

var opaqueID = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,127}$`)
var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

type ArtifactRef struct {
	ArtifactID string `json:"artifact_id"`
	Role       string `json:"role"`
	SHA256     string `json:"sha256"`
}

type Command struct {
	SchemaVersion  string `json:"schema_version"`
	MessageID      string `json:"message_id"`
	Capability     string `json:"capability"`
	WorkspaceID    string `json:"workspace_id"`
	ProjectID      string `json:"project_id"`
	JobID          string `json:"job_id"`
	PipelineRunID  string `json:"pipeline_run_id"`
	JobStepID      string `json:"job_step_id"`
	PipelineNodeID string `json:"pipeline_node_id,omitempty"`
	NodeKey        string `json:"node_key"`
	// AttemptID is a non-secret durable fencing handle. It lets a restarted
	// controller reconcile a result without recovering the old raw lease token.
	AttemptID string         `json:"attempt_id,omitempty"`
	Attempt   int            `json:"attempt"`
	InputRefs []ArtifactRef  `json:"input_refs"`
	Config    map[string]any `json:"config"`
}

type OutputRef struct {
	ArtifactID  string `json:"artifact_id"`
	Kind        string `json:"kind"`
	Role        string `json:"role"`
	SHA256      string `json:"sha256"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type,omitempty"`
}

type SafeError struct {
	Code        string `json:"code"`
	Category    string `json:"category"`
	Retryable   bool   `json:"retryable"`
	SafeMessage string `json:"safe_message"`
}

type Result struct {
	SchemaVersion string      `json:"schema_version"`
	MessageID     string      `json:"message_id"`
	JobID         string      `json:"job_id"`
	JobStepID     string      `json:"job_step_id"`
	AttemptID     string      `json:"attempt_id,omitempty"`
	Status        string      `json:"status"`
	OutputRefs    []OutputRef `json:"output_refs"`
	CheckpointRef string      `json:"checkpoint_ref,omitempty"`
	SafeError     *SafeError  `json:"safe_error,omitempty"`
}

type Progress struct {
	SchemaVersion  string  `json:"schema_version"`
	MessageID      string  `json:"message_id"`
	JobID          string  `json:"job_id"`
	JobStepID      string  `json:"job_step_id"`
	Percent        float64 `json:"percent"`
	UnitsCompleted int64   `json:"units_completed,omitempty"`
	UnitsTotal     int64   `json:"units_total,omitempty"`
	UnitName       string  `json:"unit_name,omitempty"`
	SafeMessage    string  `json:"safe_message,omitempty"`
}

func DecodeCommand(value []byte) (Command, error) {
	var result Command
	if err := json.Unmarshal(value, &result); err != nil {
		return Command{}, err
	}
	if err := ValidateCommand(result); err != nil {
		return Command{}, err
	}
	return result, nil
}

func ValidateCommand(value Command) error {
	if value.SchemaVersion != CommandSchemaVersion || value.Attempt < 1 || value.Attempt > 1000 || !validID(value.MessageID) || !validID(value.WorkspaceID) || !validID(value.ProjectID) || !validID(value.JobID) || !validID(value.PipelineRunID) || !validID(value.JobStepID) || !validField(value.NodeKey) {
		return errors.New("invalid worker command identity/version/attempt")
	}
	if value.PipelineNodeID != "" && !validID(value.PipelineNodeID) {
		return errors.New("invalid worker pipeline node ID")
	}
	if value.AttemptID != "" && !validID(value.AttemptID) {
		return errors.New("invalid worker attempt ID")
	}
	if value.Capability != "probe" && value.Capability != "thumbnail" && value.Capability != "analysis" && value.Capability != "ai" && value.Capability != "ml" && value.Capability != "media" && value.Capability != "render" && value.Capability != "system" {
		return errors.New("unsupported worker capability")
	}
	if value.InputRefs == nil || len(value.InputRefs) > 100 {
		return errors.New("worker input refs must be an array")
	}
	for _, ref := range value.InputRefs {
		if !validID(ref.ArtifactID) || !validField(ref.Role) || !sha256Hex.MatchString(ref.SHA256) {
			return errors.New("invalid worker input Artifact ref")
		}
	}
	return validateSafeValue(value.Config)
}

func DecodeResult(value []byte) (Result, error) {
	var result Result
	if err := json.Unmarshal(value, &result); err != nil {
		return Result{}, err
	}
	if err := ValidateResult(result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func ValidateResult(value Result) error {
	if value.SchemaVersion != ResultSchemaVersion || !validID(value.MessageID) || !validID(value.JobID) || !validID(value.JobStepID) {
		return errors.New("invalid worker result identity/version")
	}
	if value.AttemptID != "" && !validID(value.AttemptID) {
		return errors.New("invalid worker result attempt ID")
	}
	switch value.Status {
	case "completed", "skipped", "failed", "cancelled":
	default:
		return errors.New("invalid worker result status")
	}
	if value.OutputRefs == nil || len(value.OutputRefs) > 100 {
		return errors.New("worker output refs must be an array")
	}
	for _, ref := range value.OutputRefs {
		if !validID(ref.ArtifactID) || !validField(ref.Kind) || !validField(ref.Role) || !sha256Hex.MatchString(ref.SHA256) || ref.SizeBytes < 0 || len(ref.ContentType) > 128 || strings.ContainsAny(ref.ContentType, "\r\n") {
			return errors.New("invalid worker output Artifact ref")
		}
	}
	if value.CheckpointRef != "" && !validID(value.CheckpointRef) {
		return errors.New("invalid worker checkpoint ref")
	}
	if value.SafeError != nil {
		if value.SafeError.Code == "" || value.SafeError.SafeMessage == "" || len(value.SafeError.SafeMessage) > 512 {
			return errors.New("invalid worker safe error")
		}
		switch value.SafeError.Category {
		case "transient", "permanent", "policy", "cancelled", "internal":
		default:
			return errors.New("invalid worker safe error category")
		}
	}
	return nil
}

func ValidateProgress(value Progress) error {
	if value.SchemaVersion != ProgressSchemaVersion || !validID(value.MessageID) || !validID(value.JobID) || !validID(value.JobStepID) || value.Percent < 0 || value.Percent > 100 || value.UnitsCompleted < 0 || value.UnitsTotal < 0 || len(value.SafeMessage) > 256 {
		return errors.New("invalid worker progress")
	}
	return nil
}

func validID(value string) bool { return opaqueID.MatchString(value) }
func validField(value string) bool {
	return strings.TrimSpace(value) != "" && len(value) <= 64 && regexp.MustCompile(`^[a-z][a-z0-9_-]*$`).MatchString(value)
}

func validateSafeValue(value map[string]any) error {
	for key, child := range value {
		lower := strings.ToLower(key)
		if lower != "max_output_tokens" {
			for _, forbidden := range []string{"path", "secret", "token", "password", "traceback", "presigned", "authorization"} {
				if strings.Contains(lower, forbidden) {
					return fmt.Errorf("worker contract contains forbidden field %q", key)
				}
			}
		}
		switch typed := child.(type) {
		case map[string]any:
			if err := validateSafeValue(typed); err != nil {
				return err
			}
		case []any:
			for _, item := range typed {
				if nested, ok := item.(map[string]any); ok {
					if err := validateSafeValue(nested); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
