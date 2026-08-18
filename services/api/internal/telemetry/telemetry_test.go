package telemetry

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestMetricsBoundCardinalityAndSafeLabels(t *testing.T) {
	metrics := NewMetrics(1)
	metrics.Observe("http requests", "/api/v1/projects/user-secret", "200")
	metrics.Observe("http requests", "/api/v1/jobs/other-secret", "500")
	if len(metrics.Snapshot()) != 1 {
		t.Fatalf("unbounded metric series: %#v", metrics.Snapshot())
	}
	text := metrics.Prometheus()
	if !strings.Contains(text, `route="/api/v1/projects"`) || strings.Contains(text, "user-secret") {
		t.Fatalf("route label was not bounded: %s", text)
	}
}

func TestStructuredEventRedactsSecretsAndPaths(t *testing.T) {
	var output bytes.Buffer
	ctx := context.WithValue(context.WithValue(context.Background(), requestIDKey{}, "req_1"), correlationIDKey{}, "corr_1")
	if err := WriteEvent(&output, ctx, "info", "worker completed", map[string]string{"job_id": "job_1", "authorization": "Bearer secret", "path": `C:\private\file.mp4`}); err != nil {
		t.Fatal(err)
	}
	value := output.String()
	if strings.Contains(value, "secret") || strings.Contains(value, "private") || !strings.Contains(value, "req_1") {
		t.Fatalf("unsafe event: %s", value)
	}
}
