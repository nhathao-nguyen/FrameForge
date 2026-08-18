package node

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/media-worker/internal/probe"
	"github.com/nhathao-nguyen/NH-Media/services/media-worker/internal/process"
)

type fakeMaterializer struct{ value MaterializedArtifact }

func (f fakeMaterializer) Materialize(context.Context, string, worker.ArtifactRef) (MaterializedArtifact, error) {
	return f.value, nil
}

type fakeStager struct{ outputs []worker.OutputRef }

func (f *fakeStager) Stage(_ context.Context, _ string, kind, role string, r io.Reader, _ string) (worker.OutputRef, error) {
	_, _ = io.ReadAll(r)
	value := worker.OutputRef{ArtifactID: "artifact_probe_001", Kind: kind, Role: role, SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", SizeBytes: 10}
	f.outputs = append(f.outputs, value)
	return value, nil
}
func (f *fakeStager) StageFile(_ context.Context, _ string, kind, role, path, _ string) (worker.OutputRef, error) {
	_, _ = os.ReadFile(path)
	value := worker.OutputRef{ArtifactID: "artifact_thumb_001", Kind: kind, Role: role, SHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", SizeBytes: 10}
	f.outputs = append(f.outputs, value)
	return value, nil
}

type fakeProcess struct{}

func (fakeProcess) Run(_ context.Context, spec process.Spec) (process.Result, error) {
	if len(spec.OutputPaths) > 0 {
		if err := os.WriteFile(spec.OutputPaths[0], []byte("thumbnail"), 0o600); err != nil {
			return process.Result{}, err
		}
	}
	return process.Result{ExitCode: 0}, nil
}

func command(capability string) worker.Command {
	return worker.Command{SchemaVersion: worker.CommandSchemaVersion, MessageID: "msg_probe_001", Capability: capability, WorkspaceID: "workspace_probe_001", ProjectID: "project_probe_001", JobID: "job_probe_001", PipelineRunID: "run_probe_001", JobStepID: "step_probe_001", NodeKey: "asset_probe", Attempt: 1, InputRefs: []worker.ArtifactRef{{ArtifactID: "artifact_source_001", Role: "source", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}, Config: map[string]any{"declared_mime": "video/mp4"}}
}

func TestProbeNodeStagesOnlyValidatedProbeArtifact(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "source.mp4")
	if err := os.WriteFile(path, []byte{0, 0, 0, 0, 'f', 't', 'y', 'p', 'i', 's', 'o'}, 0o600); err != nil {
		t.Fatal(err)
	}
	stager := &fakeStager{}
	n := Node{Materializer: fakeMaterializer{value: MaterializedArtifact{Path: path, ScratchDir: root}}, Stager: stager, Validator: probe.Validator{Probe: func(context.Context, string) ([]byte, error) {
		return []byte(`{"format":{"duration":"1"},"streams":[{"codec_type":"video","width":1,"height":1}]}`), nil
	}}}
	result, err := n.Execute(context.Background(), command("probe"))
	if err != nil || result.Status != "completed" || len(result.OutputRefs) != 1 || len(stager.outputs) != 1 {
		t.Fatalf("probe node failed: %+v err=%v", result, err)
	}
}

func TestProbeNodeQuarantineProducesNoOutput(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "source.mp4")
	if err := os.WriteFile(path, []byte{0, 0, 0, 0, 'f', 't', 'y', 'p', 'i', 's', 'o'}, 0o600); err != nil {
		t.Fatal(err)
	}
	stager := &fakeStager{}
	n := Node{Materializer: fakeMaterializer{value: MaterializedArtifact{Path: path}}, Stager: stager, Validator: probe.Validator{Probe: func(context.Context, string) ([]byte, error) {
		return []byte(`{"format":{"duration":"1"},"streams":[]}`), nil
	}}}
	result, err := n.Execute(context.Background(), command("probe"))
	if err != nil || result.Status != "failed" || len(result.OutputRefs) != 0 || len(stager.outputs) != 0 || result.SafeError == nil {
		t.Fatalf("quarantine was not fail-closed: %+v err=%v", result, err)
	}
}

func TestThumbnailNodeUsesProcessAndFileStager(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "source.mp4")
	if err := os.WriteFile(path, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	stager := &fakeStager{}
	n := Node{Materializer: fakeMaterializer{value: MaterializedArtifact{Path: path, ScratchDir: root}}, Stager: stager, FileStager: stager, Process: fakeProcess{}}
	result, err := n.Execute(context.Background(), command("thumbnail"))
	if err != nil || result.Status != "completed" || len(result.OutputRefs) != 1 || result.OutputRefs[0].Role != "thumbnail" {
		t.Fatalf("thumbnail node failed: %+v err=%v", result, err)
	}
}
