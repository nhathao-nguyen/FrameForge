package persistence

import "testing"

func TestArtifactRefsFromSnapshotNormalizesProductIDs(t *testing.T) {
	refs := artifactRefsFromSnapshot([]byte(`{"artifacts":[{"artifact_id":"dbe4fa51-f262-469f-832c-8d80419c99fb","role":"source_original","sha256":"6b9c1dd8c6f6b8cf93d5f1f762078b09d87c80f3d2ea83b3807a76d4356b109e"}]}`))
	if len(refs) != 1 || refs[0].ArtifactID != "artifact_dbe4fa51_f262_469f_832c_8d80419c99fb" {
		t.Fatalf("refs=%+v", refs)
	}
	prefixed := artifactRefsFromSnapshot([]byte(`[{"artifact_id":"artifact_source_001","role":"source","sha256":"6b9c1dd8c6f6b8cf93d5f1f762078b09d87c80f3d2ea83b3807a76d4356b109e"}]`))
	if len(prefixed) != 1 || prefixed[0].ArtifactID != "artifact_source_001" {
		t.Fatalf("prefixed refs=%+v", prefixed)
	}
}

func TestCapabilityForStepHonorsExecutionClass(t *testing.T) {
	if value, err := capabilityForStep("probe", "analysis"); err != nil || value != "probe" {
		t.Fatalf("probe capability=%q err=%v", value, err)
	}
	if value, err := capabilityForStep("ml", "analysis"); err != nil || value != "analysis" {
		t.Fatalf("native analysis capability=%q err=%v", value, err)
	}
	if value, err := capabilityForStep("ml", "other"); err != nil || value != "ml" {
		t.Fatalf("ml capability=%q err=%v", value, err)
	}
}

func TestRetryJobQueueTransitionIsDeduplicatedPerJob(t *testing.T) {
	queuedJobs := map[string]struct{}{}
	if !retryJobNeedsQueueTransition("job-1", queuedJobs) {
		t.Fatal("first retrying step should transition its Job")
	}
	if retryJobNeedsQueueTransition("job-1", queuedJobs) {
		t.Fatal("second retrying step from the same Job must not transition the Job again")
	}
	if !retryJobNeedsQueueTransition("job-2", queuedJobs) {
		t.Fatal("a different Job still needs its own transition")
	}
}
