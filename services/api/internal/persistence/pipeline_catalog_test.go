package persistence

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNativeMovieRecapPersistenceInputKeepsSemanticNodesAndEdges(t *testing.T) {
	input, err := nativeMovieRecapInput("owner_1")
	if err != nil {
		t.Fatal(err)
	}
	nodes := map[string]PipelineNodeInput{}
	for _, node := range input.Nodes {
		nodes[node.NodeKey] = node
	}
	for _, key := range []string{"align_audio", "generate_subtitle", "export_clips"} {
		if len(nodes[key].RequiredArtifactRoles) == 0 && key != "generate_subtitle" {
			t.Fatalf("%s lost required artifact roles in persistence input", key)
		}
		if len(nodes[key].ProducedArtifactRoles) == 0 {
			t.Fatalf("%s lost produced artifact roles in persistence input", key)
		}
	}
	var required []string
	if err := json.Unmarshal(nodes["generate_subtitle"].RequiredArtifactRoles, &required); err != nil {
		t.Fatal(err)
	}
	if len(required) != 1 || required[0] != "timing_alignment" {
		t.Fatalf("generate_subtitle required roles drifted: %v", required)
	}
	var produced []string
	if err := json.Unmarshal(nodes["export_clips"].ProducedArtifactRoles, &produced); err != nil {
		t.Fatal(err)
	}
	if len(produced) != 2 || produced[0] != "clip_exports" || produced[1] != "clip_manifest" {
		t.Fatalf("export_clips produced roles drifted: %v", produced)
	}
	var definition map[string]any
	if err := json.Unmarshal(input.Definition, &definition); err != nil {
		t.Fatal(err)
	}
	if definition["version"] != float64(5) {
		t.Fatalf("native definition version was not persisted: %#v", definition["version"])
	}
	if len(input.Dependencies) == 0 {
		t.Fatal("native graph dependencies were not persisted")
	}
	for _, dependency := range input.Dependencies {
		if dependency.NodeKey == "generate_subtitle" && dependency.DependsOnNodeKey == "translate_subtitles" {
			if dependency.Required {
				t.Fatal("optional subtitle translation was persisted as required")
			}
			return
		}
	}
	t.Fatal("subtitle translation dependency was not persisted")
}

func TestPipelineJobSnapshotExpressionIncludesExecutablePolicy(t *testing.T) {
	for _, field := range []string{"required_artifact_roles", "produced_artifact_roles", "retry_policy", "checkpoint_policy", "idempotency_policy", "dependencies", "required"} {
		if !strings.Contains(pipelineSnapshotExpression, field) {
			t.Fatalf("pipeline snapshot expression lost %q", field)
		}
	}
}
