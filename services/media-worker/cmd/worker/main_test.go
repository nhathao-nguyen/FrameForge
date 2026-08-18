package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompatibilityWorkerStdioUsesVersionedArtifactResult(t *testing.T) {
	input := "{\"schema_version\":\"worker-command/v1\",\"message_id\":\"msg_media_001\",\"capability\":\"probe\",\"workspace_id\":\"workspace_media_001\",\"project_id\":\"project_media_001\",\"job_id\":\"job_media_001\",\"pipeline_run_id\":\"run_media_001\",\"job_step_id\":\"step_media_001\",\"node_key\":\"analysis\",\"attempt\":1,\"input_refs\":[],\"config\":{\"mode\":\"contract_probe\"}}\n"
	var output bytes.Buffer
	if err := runStdio(strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "\"schema_version\":\"worker-result/v1\"") || !strings.Contains(output.String(), "\"status\":\"completed\"") {
		t.Fatalf("unexpected worker output: %s", output.String())
	}
}
