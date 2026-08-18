package inventory

import "testing"

func TestCompareReportsMissingOrphanAndCorruptObjects(t *testing.T) {
	report := Compare([]ExpectedArtifact{
		{Backend: "minio", ObjectKey: "final/a.mp4", SHA256: "aaa", SizeBytes: 10},
		{Backend: "minio", ObjectKey: "resume/a.json", SHA256: "bbb", SizeBytes: 2},
	}, []Object{
		{Backend: "minio", ObjectKey: "final/a.mp4", SHA256: "wrong", SizeBytes: 10},
		{Backend: "minio", ObjectKey: "scratch/x", SHA256: "ccc", SizeBytes: 1},
	})
	if report.Passed || len(report.Findings) != 3 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestRetentionRequiresExplicitAuditedDelete(t *testing.T) {
	if RetentionAllowsDelete("source", false) || RetentionAllowsDelete("final", false) || RetentionAllowsDelete("resume-required", false) {
		t.Fatal("durable Artifact retention was bypassed")
	}
	if !RetentionAllowsDelete("scratch", false) || !RetentionAllowsDelete("final", true) {
		t.Fatal("retention policy is too strict")
	}
}
