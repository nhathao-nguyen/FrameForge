package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

const DefinitionSchemaVersion = "pipeline-definition/v1"

var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

var registeredNodeTypes = map[string]bool{
	"fake": true, "analysis": true, "asset_probe": true, "resolve_source_asset": true,
	"prepare_media_assets": true, "research_metadata": true, "generate_script": true,
	"review_script": true, "generate_narration": true, "align_audio": true,
	"detect_scenes": true, "extract_transcript": true, "analyze_scenes": true,
	"detect_characters": true, "embed_media": true, "generate_match_candidates": true,
	"evaluate_candidates": true, "select_candidate": true, "coverage_feedback": true,
	"translate_subtitles": true, "generate_subtitle": true, "build_timeline": true,
	"review_timeline": true, "mix_audio": true, "run_qa_gate": true,
	"render_timeline": true, "validate_deliverable": true, "export_clips": true,
}

var dependencyConditions = map[string]bool{
	"success":                  true,
	"soft":                     true,
	"success_or_declared_soft": true,
}

// artifactRoleContract is the reviewed native catalog contract.  The
// scheduler only sees dependencies; it must not infer these meanings from
// node keys or executor classes.  A role is a semantic Artifact boundary, not
// a filesystem name.
type artifactRoleContract struct {
	required []string
	produced []string
}

var artifactRoleContracts = map[string]artifactRoleContract{
	"resolve_source_asset":      {required: []string{"source_original"}, produced: []string{"source_probe"}},
	"prepare_media_assets":      {required: []string{"source_original"}, produced: []string{"prepared_video", "prepared_audio", "source_thumbnails"}},
	"research_metadata":         {produced: []string{"research_metadata"}},
	"generate_script":           {produced: []string{"script_version"}},
	"review_script":             {},
	"generate_narration":        {produced: []string{"narration_audio", "narration_manifest"}},
	"align_audio":               {required: []string{"narration_audio"}, produced: []string{"timing_alignment"}},
	"detect_scenes":             {required: []string{"prepared_video"}, produced: []string{"scene_index", "scene_thumbnails"}},
	"extract_transcript":        {required: []string{"prepared_audio"}, produced: []string{"transcript"}},
	"analyze_scenes":            {required: []string{"scene_index", "scene_thumbnails"}, produced: []string{"scene_analysis"}},
	"detect_characters":         {required: []string{"scene_thumbnails"}, produced: []string{"character_analysis"}},
	"embed_media":               {required: []string{"scene_analysis"}, produced: []string{"media_embeddings"}},
	"generate_match_candidates": {required: []string{"timing_alignment", "scene_index"}, produced: []string{"match_candidates"}},
	"evaluate_candidates":       {required: []string{"match_candidates"}, produced: []string{"candidate_evaluations"}},
	"coverage_feedback":         {produced: []string{"coverage_report"}},
	"translate_subtitles":       {required: []string{"timing_alignment"}, produced: []string{"translated_subtitles"}},
	"generate_subtitle":         {required: []string{"timing_alignment"}, produced: []string{"subtitle_cues", "subtitle_srt", "subtitle_vtt", "subtitle_ass"}},
	"review_timeline":           {},
	"select_candidate":          {required: []string{"candidate_evaluations"}, produced: []string{"selected_match_proposal"}},
	"build_timeline":            {required: []string{"selected_match_proposal", "narration_audio", "subtitle_cues"}, produced: []string{"timeline_version"}},
	"mix_audio":                 {required: []string{"narration_audio"}, produced: []string{"mixed_audio", "loudness_report"}},
	"run_qa_gate":               {required: []string{"mixed_audio"}, produced: []string{"timeline_qa_report"}},
	"render_timeline":           {required: []string{"mixed_audio"}, produced: []string{"render_video", "render_audio", "render_metadata"}},
	"validate_deliverable":      {required: []string{"render_video"}, produced: []string{"deliverable_qa"}},
	"export_clips":              {required: []string{"render_video"}, produced: []string{"clip_exports", "clip_manifest"}},
}

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
		if !keyPattern.MatchString(node.Key) || !registeredNodeTypes[node.Type] || node.DisplayName == "" {
			return ValidationError{"nodes." + node.Key, "key, type and display_name are required"}
		}
		if node.InputContract == "" || node.OutputContract == "" || len(node.InputSchema) == 0 || len(node.OutputSchema) == 0 {
			return ValidationError{"nodes." + node.Key, "input/output contracts and schemas are required"}
		}
		if err := validateArtifactRoles(node); err != nil {
			return ValidationError{"nodes." + node.Key + ".artifact_roles", err.Error()}
		}
		if err := validateJSONSchema(node.InputSchema, "nodes."+node.Key+".input_schema"); err != nil {
			return ValidationError{"nodes." + node.Key + ".input_schema", err.Error()}
		}
		if err := validateJSONSchema(node.OutputSchema, "nodes."+node.Key+".output_schema"); err != nil {
			return ValidationError{"nodes." + node.Key + ".output_schema", err.Error()}
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
		if node.Timeout.WallTimeSec < 1 || node.Timeout.IdleTimeSec < 0 || node.Timeout.KillGraceSec < 0 || node.Retry.MaxAttempts < 1 || node.Retry.BaseDelayMS < 0 || node.Retry.MaxDelayMS < node.Retry.BaseDelayMS || (node.Retry.MaxAttempts > 1 && len(node.Retry.RetryableCategories) == 0) || node.Progress.Unit == "" || node.Progress.TotalSource == "" || node.Progress.EmitInterval <= 0 || node.Checkpoint.Mode == "" || node.Idempotency.SideEffects == "" || len(node.Idempotency.FingerprintFields) == 0 || len(node.ResourceRequirements) == 0 {
			return ValidationError{"nodes." + node.Key + ".policy", "timeout/retry/progress/checkpoint/idempotency policy is incomplete"}
		}
		seenRetryCategories := map[string]bool{}
		for _, category := range node.Retry.RetryableCategories {
			if strings.TrimSpace(category) == "" || seenRetryCategories[category] {
				return ValidationError{"nodes." + node.Key + ".retry_policy", "retryable categories must be non-empty and unique"}
			}
			seenRetryCategories[category] = true
		}
		if weight, ok := d.Policies.ProgressWeights[node.Key]; !ok || weight <= 0 {
			return ValidationError{"policies.progress_weights." + node.Key, "weight must be positive"}
		}
		seenDependencies := map[string]bool{}
		for _, dependency := range node.DependsOn {
			if dependency.NodeKey == node.Key || nodes[dependency.NodeKey].Key == "" || !dependencyConditions[dependency.Condition] || seenDependencies[dependency.NodeKey] {
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

func validateArtifactRoles(node Node) error {
	validate := func(name string, roles []string) error {
		seen := map[string]bool{}
		for _, role := range roles {
			if strings.TrimSpace(role) == "" || seen[role] {
				return fmt.Errorf("%s must contain unique non-empty roles", name)
			}
			seen[role] = true
		}
		return nil
	}
	if err := validate("required_artifact_roles", node.RequiredArtifactRoles); err != nil {
		return err
	}
	if err := validate("produced_artifact_roles", node.ProducedArtifactRoles); err != nil {
		return err
	}
	contract, ok := artifactRoleContracts[node.Type]
	if !ok {
		return nil
	}
	contains := func(have []string, want string) bool {
		for _, role := range have {
			if role == want {
				return true
			}
		}
		return false
	}
	for _, role := range contract.required {
		if !contains(node.RequiredArtifactRoles, role) {
			return fmt.Errorf("required role %q is missing", role)
		}
	}
	for _, role := range contract.produced {
		if !contains(node.ProducedArtifactRoles, role) {
			return fmt.Errorf("produced role %q is missing", role)
		}
	}
	if len(node.RequiredArtifactRoles) != len(contract.required) || len(node.ProducedArtifactRoles) != len(contract.produced) {
		return fmt.Errorf("artifact roles contain an undeclared native role")
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

// validateJSONSchema validates the declared subset of JSON Schema used by
// PipelineNode contracts. It intentionally stays dependency-free, but rejects
// malformed and structurally impossible schemas before a definition can be
// activated. Unknown extension keywords remain allowed for forward
// compatibility; known keywords are type-checked recursively.
func validateJSONSchema(raw json.RawMessage, path string) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("%s contains trailing JSON", path)
	}
	if _, ok := value.(map[string]any); !ok {
		return fmt.Errorf("%s must be a JSON object", path)
	}
	root := value.(map[string]any)
	if _, hasType := root["type"]; !hasType {
		if _, hasRef := root["$ref"]; !hasRef && root["oneOf"] == nil && root["anyOf"] == nil && root["allOf"] == nil {
			return fmt.Errorf("%s must declare type, $ref or a schema composition", path)
		}
	}
	return validateSchemaValue(value, path, 0)
}

func validateSchemaValue(value any, path string, depth int) error {
	if depth > 32 {
		return fmt.Errorf("%s exceeds schema nesting limit", path)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("%s must be a schema object", path)
	}
	if ref, exists := object["$ref"]; exists {
		value, ok := ref.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s.$ref must be a non-empty string", path)
		}
	}
	if rawType, exists := object["type"]; exists {
		switch typed := rawType.(type) {
		case string:
			if !validSchemaType(typed) {
				return fmt.Errorf("%s.type is unsupported", path)
			}
		case []any:
			if len(typed) == 0 {
				return fmt.Errorf("%s.type must not be empty", path)
			}
			seen := map[string]bool{}
			for index, entry := range typed {
				name, ok := entry.(string)
				if !ok || !validSchemaType(name) || seen[name] {
					return fmt.Errorf("%s.type[%d] is invalid or duplicated", path, index)
				}
				seen[name] = true
			}
		default:
			return fmt.Errorf("%s.type must be a string or string array", path)
		}
	}
	if properties, exists := object["properties"]; exists {
		propertyMap, ok := properties.(map[string]any)
		if !ok {
			return fmt.Errorf("%s.properties must be an object", path)
		}
		for name, schema := range propertyMap {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("%s.properties contains an empty key", path)
			}
			if err := validateSchemaValue(schema, path+".properties."+name, depth+1); err != nil {
				return err
			}
		}
	}
	if required, exists := object["required"]; exists {
		values, ok := required.([]any)
		if !ok {
			return fmt.Errorf("%s.required must be a string array", path)
		}
		seen := map[string]bool{}
		for index, value := range values {
			name, ok := value.(string)
			if !ok || strings.TrimSpace(name) == "" || seen[name] {
				return fmt.Errorf("%s.required[%d] is invalid or duplicated", path, index)
			}
			seen[name] = true
		}
	}
	if additional, exists := object["additionalProperties"]; exists {
		switch typed := additional.(type) {
		case bool:
		case map[string]any:
			if err := validateSchemaValue(typed, path+".additionalProperties", depth+1); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s.additionalProperties must be boolean or schema", path)
		}
	}
	if items, exists := object["items"]; exists {
		if err := validateSchemaValue(items, path+".items", depth+1); err != nil {
			return err
		}
	}
	for _, keyword := range []string{"oneOf", "anyOf", "allOf"} {
		if raw, exists := object[keyword]; exists {
			values, ok := raw.([]any)
			if !ok || len(values) == 0 {
				return fmt.Errorf("%s.%s must be a non-empty schema array", path, keyword)
			}
			for index, schema := range values {
				if err := validateSchemaValue(schema, fmt.Sprintf("%s.%s[%d]", path, keyword, index), depth+1); err != nil {
					return err
				}
			}
		}
	}
	if enum, exists := object["enum"]; exists {
		if values, ok := enum.([]any); !ok || len(values) == 0 {
			return fmt.Errorf("%s.enum must be a non-empty array", path)
		}
	}
	if pattern, exists := object["pattern"]; exists {
		value, ok := pattern.(string)
		if !ok {
			return fmt.Errorf("%s.pattern must be a string", path)
		}
		if _, err := regexp.Compile(value); err != nil {
			return fmt.Errorf("%s.pattern is invalid: %w", path, err)
		}
	}
	for _, keyword := range []string{"minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf"} {
		if raw, exists := object[keyword]; exists {
			if number, ok := raw.(json.Number); !ok {
				return fmt.Errorf("%s.%s must be a number", path, keyword)
			} else if _, err := number.Float64(); err != nil {
				return fmt.Errorf("%s.%s is not finite", path, keyword)
			}
		}
	}
	return nil
}

func validSchemaType(value string) bool {
	switch value {
	case "null", "boolean", "object", "array", "number", "integer", "string":
		return true
	default:
		return false
	}
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
