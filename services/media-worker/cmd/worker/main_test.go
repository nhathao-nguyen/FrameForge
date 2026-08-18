package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDeterministicWorkerStdioUsesVersionedArtifactResult(t *testing.T) {
	input := "{\"schema_version\":\"worker-command/v1\",\"message_id\":\"msg_media_001\",\"capability\":\"probe\",\"project_id\":\"project_media_001\",\"job_id\":\"job_media_001\",\"pipeline_run_id\":\"run_media_001\",\"job_step_id\":\"step_media_001\",\"attempt\":1,\"input_refs\":[],\"config\":{\"mode\":\"deterministic\"}}\n"
	var output bytes.Buffer
	if err := runStdio(strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "\"schema_version\":\"worker-result/v1\"") || !strings.Contains(output.String(), "\"status\":\"completed\"") {
		t.Fatalf("unexpected worker output: %s", output.String())
	}
}
