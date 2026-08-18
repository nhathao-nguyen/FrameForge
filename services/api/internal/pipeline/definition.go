package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
)

const DefinitionSchemaVersion = "pipeline-definition/v1"

var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

type Definition struct {
	SchemaVersion string     `json:"schema_version"`
	WorkflowKey   string     `json:"workflow_key"`
	Version       int        `json:"version"`
	Nodes         []Node     `json:"nodes"`
	Policies      Policies   `json:"policies"`
	Provenance    Provenance `json:"provenance"`
	StartFrom     []string   `json:"start_from,omitempty"`
	StopAfter     []string   `json:"stop_after,omitempty"`
}

type Node struct {
	Key                   string            `json:"key"`
	Type                  string            `json:"type"`
	DisplayName           string            `json:"display_name"`
	DependsOn             []Dependency      `json:"depends_on,omitempty"`
	InputContract         string            `json:"input_contract"`
	OutputContract        string            `json:"output_contract"`
	RequiredArtifactRoles []string          `json:"required_artifact_roles,omitempty"`
	ProducedArtifactRoles []string          `json:"produced_artifact_roles,omitempty"`
	ExecutionClass        string            `json:"execution_class"`
	ResourceRequirements  map[string]any    `json:"resource_requirements"`
	Timeout               TimeoutPolicy     `json:"timeout"`
	Retry                 RetryPolicy       `json:"retry_policy"`
	Progress              ProgressPolicy    `json:"progress_policy"`
	Checkpoint            CheckpointPolicy  `json:"checkpoint_policy"`
	Idempotency           IdempotencyPolicy `json:"idempotency_policy"`
	FailureMode           string            `json:"failure_mode"`
	Soft                  bool              `json:"soft,omitempty"`
	HumanGate             bool              `json:"human_gate,omitempty"`
	ReviewPolicy          string            `json:"review_policy"`
	InputSchema           json.RawMessage   `json:"input_schema"`
	OutputSchema          json.RawMessage   `json:"output_schema"`
}

type Dependency struct {
	NodeKey   string `json:"node_key"`
	Required  bool   `json:"required"`
	Condition string `json:"condition"`
}

type TimeoutPolicy struct {
	WallTimeSec  int `json:"wall_time_sec"`
	IdleTimeSec  int `json:"idle_time_sec,omitempty"`
	KillGraceSec int `json:"kill_grace_sec"`
}

type RetryPolicy struct {
	MaxAttempts         int      `json:"max_attempts"`
	BaseDelayMS         int      `json:"base_delay_ms"`
	MaxDelayMS          int      `json:"max_delay_ms"`
	RetryableCategories []string `json:"retryable_categories"`
}

type ProgressPolicy struct {
	Unit         string  `json:"unit"`
	TotalSource  string  `json:"total_source"`
	EmitInterval float64 `json:"emit_interval_sec"`
}

type CheckpointPolicy struct {
	Mode                string   `json:"mode"`
	ResumeCompatibility string   `json:"resume_compatibility"`
	InvalidationInputs  []string `json:"invalidation_inputs"`
}

type IdempotencyPolicy struct {
	FingerprintFields []string `json:"fingerprint_fields"`
	Reuse             bool     `json:"reuse"`
	SideEffects       string   `json:"side_effects"`
}

type Policies struct {
	Strict            bool               `json:"strict"`
	Checkpoint        bool               `json:"checkpoint"`
	ArtifactRetention string             `json:"artifact_retention"`
	ProgressWeights   map[string]float64 `json:"progress_weights"`
}

type Provenance struct {
	NHMediaRelease       string `json:"nh_media_release"`
	ContractVersion      string `json:"contract_version"`
	ReferenceObservation string `json:"optional_reference_observation,omitempty"`
}

type CapabilitySet map[string]bool

type ValidationError struct {
	Path  string
	Issue string
}

func (e ValidationError) Error() string { return e.Path + ": " + e.Issue }

func (d Definition) NodeMap() map[string]Node {
	result := make(map[string]Node, len(d.Nodes))
	for _, node := range d.Nodes {
		result[node.Key] = node
	}
	return result
}

func (d Definition) Validate(capabilities CapabilitySet) error {
	if d.SchemaVersion != DefinitionSchemaVersion {
		return ValidationError{"schema_version", "unsupported definition schema"}
	}
	if !keyPattern.MatchString(d.WorkflowKey) {
		return ValidationError{"workflow_key", "must be a bounded lowercase key"}
	}
	if d.Version < 1 || len(d.Nodes) == 0 {
		return ValidationError{"nodes", "version and at least one node are required"}
	}
	if d.Provenance.NHMediaRelease == "" || d.Provenance.ContractVersion == "" {
		return ValidationError{"provenance", "release and contract version are required"}
	}
	if d.Policies.ArtifactRetention == "" || len(d.Policies.ProgressWeights) != len(d.Nodes) {
		return ValidationError{"policies", "artifact retention and one progress weight per node are required"}
	}
	nodes := d.NodeMap()
	if len(nodes) != len(d.Nodes) {
		return ValidationError{"nodes", "node keys must be unique"}
	}
	indegree := make(map[string]int, len(nodes))
	dependants := make(map[string][]string, len(nodes))
	for _, node := range d.Nodes {
		indegree[node.Key] = 0
		if !keyPattern.MatchString(node.Key) || node.Type == "" || node.DisplayName == "" {
			return ValidationError{"nodes." + node.Key, "key, type and display_name are required"}
		}
		if node.InputContract == "" || node.OutputContract == "" || len(node.InputSchema) == 0 || len(node.OutputSchema) == 0 {
			return ValidationError{"nodes." + node.Key, "input/output contracts and schemas are required"}
		}
		if node.ExecutionClass == "" || !capabilities[node.ExecutionClass] {
			return ValidationError{"nodes." + node.Key, "execution capability is unavailable"}
		}
		if node.FailureMode != "hard" && node.FailureMode != "soft" {
			return ValidationError{"nodes." + node.Key, "failure_mode must be hard or soft"}
		}
		if node.Soft != (node.FailureMode == "soft") {
			return ValidationError{"nodes." + node.Key, "soft flag must match failure_mode"}
		}
		if node.ReviewPolicy != "none" && node.ReviewPolicy != "approval_completes_node" && node.ReviewPolicy != "approval_resumes_node" {
			return ValidationError{"nodes." + node.Key, "review policy is invalid"}
		}
		if node.HumanGate != (node.ReviewPolicy != "none") {
			return ValidationError{"nodes." + node.Key, "human_gate must match review_policy"}
		}
		if node.Timeout.WallTimeSec < 1 || node.Timeout.KillGraceSec < 0 || node.Retry.MaxAttempts < 1 || node.Retry.MaxDelayMS < node.Retry.BaseDelayMS || node.Progress.Unit == "" || node.Progress.TotalSource == "" || node.Progress.EmitInterval <= 0 || node.Checkpoint.Mode == "" || node.Idempotency.SideEffects == "" || len(node.Idempotency.FingerprintFields) == 0 {
			return ValidationError{"nodes." + node.Key + ".policy", "timeout/retry/progress/checkpoint/idempotency policy is incomplete"}
		}
		if weight, ok := d.Policies.ProgressWeights[node.Key]; !ok || weight <= 0 {
			return ValidationError{"policies.progress_weights." + node.Key, "weight must be positive"}
		}
		seenDependencies := map[string]bool{}
		for _, dependency := range node.DependsOn {
			if dependency.NodeKey == node.Key || nodes[dependency.NodeKey].Key == "" || dependency.Condition == "" || seenDependencies[dependency.NodeKey] {
				return ValidationError{"nodes." + node.Key + ".depends_on", "dependency is missing, duplicated or self-referential"}
			}
			seenDependencies[dependency.NodeKey] = true
			indegree[node.Key]++
			dependants[dependency.NodeKey] = append(dependants[dependency.NodeKey], node.Key)
			if !contractsCompatible(nodes[dependency.NodeKey].OutputContract, node.InputContract) {
				return ValidationError{"nodes." + node.Key + ".depends_on." + dependency.NodeKey, "input/output contracts are incompatible"}
			}
		}
	}
	if err := validateBoundaries(d.StartFrom, nodes, "start_from"); err != nil {
		return err
	}
	if err := validateBoundaries(d.StopAfter, nodes, "stop_after"); err != nil {
		return err
	}
	queue := make([]string, 0)
	for key, degree := range indegree {
		if degree == 0 {
			queue = append(queue, key)
		}
	}
	visited := 0
	for len(queue) > 0 {
		sort.Strings(queue)
		key := queue[0]
		queue = queue[1:]
		visited++
		for _, child := range dependants[key] {
			indegree[child]--
			if indegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}
	if visited != len(nodes) {
		return ValidationError{"nodes", "graph contains a cycle"}
	}
	return nil
}

func validateBoundaries(values []string, nodes map[string]Node, name string) error {
	seen := map[string]bool{}
	for _, value := range values {
		if !keyPattern.MatchString(value) || nodes[value].Key == "" || seen[value] {
			return ValidationError{name, "boundary must reference a unique node key"}
		}
		seen[value] = true
	}
	return nil
}

func contractsCompatible(output, input string) bool {
	return output == input || input == "context/v1" || output == "context/v1"
}

func (d Definition) Hash() (string, error) {
	if err := d.Validate(allCapabilities()); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	var normalized any
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&normalized); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func ActiveDefinition(d Definition, capabilities CapabilitySet) (Definition, string, error) {
	if err := d.Validate(capabilities); err != nil {
		return Definition{}, "", err
	}
	hash, err := d.Hash()
	if err != nil {
		return Definition{}, "", err
	}
	return d, hash, nil
}

func VerifyImmutableActive(previousHash string, next Definition, capabilities CapabilitySet) (string, error) {
	_, hash, err := ActiveDefinition(next, capabilities)
	if err != nil {
		return "", err
	}
	if previousHash != "" && previousHash != hash {
		return "", errors.New("active pipeline definition is immutable")
	}
	return hash, nil
}

func allCapabilities() CapabilitySet {
	return CapabilitySet{"system": true, "probe": true, "ai": true, "ml": true, "media": true, "render": true}
}

func AllCapabilities() CapabilitySet { return allCapabilities() }

func (d Definition) MarshalJSON() ([]byte, error) {
	type alias Definition
	return json.Marshal(alias(d))
}

func (d Definition) String() string {
	value, _ := json.Marshal(d)
	return string(value)
}

func (d Definition) ValidateAll() error { return d.Validate(allCapabilities()) }

var _ = fmt.Sprintf
