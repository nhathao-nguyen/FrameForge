package execution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/queue"
)

type fakeLeaseResolver struct {
	command worker.Command
	lease   Lease
}

func (f fakeLeaseResolver) ClaimCommand(context.Context, queue.Message) (worker.Command, Lease, error) {
	return f.command, f.lease, nil
}

type fakeClaimQueue struct {
	deliveries []queue.Delivery
	acked      int
}

func (f *fakeClaimQueue) Enqueue(context.Context, queue.Message) (string, error) { return "", nil }
func (f *fakeClaimQueue) Consume(context.Context, string, string, int, time.Duration) ([]queue.Delivery, error) {
	return f.deliveries, nil
}
func (f *fakeClaimQueue) Ack(context.Context, string, string) error { f.acked++; return nil }
func (f *fakeClaimQueue) Reclaim(context.Context, string, string, time.Duration, int) ([]queue.Delivery, error) {
	return nil, nil
}

type fakeLeasePublisher struct{ count int }

func (f *fakeLeasePublisher) PublishWorkerCommand(context.Context, worker.Command) (string, error) {
	f.count++
	return "1-0", nil
}

func TestClaimingDispatcherRegistersLeaseBeforeFastWorkerResult(t *testing.T) {
	q := &fakeClaimQueue{deliveries: []queue.Delivery{{Stream: "stream", EntryID: "1-0", Message: queue.Message{MessageID: "msg_analysis_001", Capability: "analysis", JobID: "job_analysis_001", JobStepID: "step_analysis_001", Attempt: 1}}}}
	command := worker.Command{SchemaVersion: worker.CommandSchemaVersion, MessageID: "msg_analysis_001", Capability: "analysis", WorkspaceID: "workspace_analysis_001", ProjectID: "project_analysis_001", JobID: "job_analysis_001", PipelineRunID: "run_analysis_001", JobStepID: "step_analysis_001", NodeKey: "analysis", Attempt: 1, InputRefs: []worker.ArtifactRef{}, Config: map[string]any{}}
	leases := NewLeaseRegistry()
	publisher := &fakeLeasePublisher{}
	count, err := (ClaimingDispatcher{Queue: q, Resolver: fakeLeaseResolver{command: command}, Publisher: publisher, Claims: leases}).DispatchOnce(context.Background(), "analysis", "controller", 1)
	if err != nil || count != 1 || q.acked != 1 || publisher.count != 1 {
		t.Fatalf("count=%d err=%v acked=%d published=%d", count, err, q.acked, publisher.count)
	}
	if _, ok := leases.Take(command.MessageID); !ok {
		t.Fatal("claimed lease was not retained")
	}
}

type fakeClaimResultSource struct{ acked int }

func (f *fakeClaimResultSource) ConsumeWorkerResults(context.Context, string, string, int, time.Duration) ([]queue.ResultDelivery, error) {
	return []queue.ResultDelivery{{Stream: "results", EntryID: "1-0", Result: worker.Result{MessageID: "msg_missing_001"}}}, nil
}
func (f *fakeClaimResultSource) AckWorkerResult(context.Context, string, string) error {
	f.acked++
	return nil
}

type fakeClaimResultApplier struct{ err error }

func (f fakeClaimResultApplier) ApplyClaimedResult(context.Context, ClaimedResult) error {
	return f.err
}

func TestLeaseAwareResultDoesNotAckUnknownControllerLease(t *testing.T) {
	source := &fakeClaimResultSource{}
	count, err := (LeaseAwareResultReconciler{Source: source, Claims: NewLeaseRegistry(), Applier: fakeClaimResultApplier{err: errors.New("must not run")}}).ReconcileOnce(context.Background(), "analysis", "controller", 1, time.Second)
	if !errors.Is(err, ErrLeaseConflict) || count != 0 || source.acked != 0 {
		t.Fatalf("count=%d err=%v acked=%d", count, err, source.acked)
	}
}
