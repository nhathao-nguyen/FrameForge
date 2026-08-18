package execution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/queue"
)

type fakeExecutionQueue struct {
	deliveries []queue.Delivery
	acked      int
}

func (f *fakeExecutionQueue) Enqueue(context.Context, queue.Message) (string, error) { return "", nil }
func (f *fakeExecutionQueue) Consume(context.Context, string, string, int, time.Duration) ([]queue.Delivery, error) {
	return f.deliveries, nil
}
func (f *fakeExecutionQueue) Ack(context.Context, string, string) error { f.acked++; return nil }
func (f *fakeExecutionQueue) Reclaim(context.Context, string, string, time.Duration, int) ([]queue.Delivery, error) {
	return nil, nil
}

type fakeCommandResolver struct{ command worker.Command }

func (f fakeCommandResolver) ResolveCommand(context.Context, queue.Message) (worker.Command, error) {
	return f.command, nil
}

type fakeCommandPublisher struct{ published int }

func (f *fakeCommandPublisher) PublishWorkerCommand(context.Context, worker.Command) (string, error) {
	f.published++
	return "1-0", nil
}

func TestDispatcherAcknowledgesOnlyAfterWorkerPublish(t *testing.T) {
	q := &fakeExecutionQueue{deliveries: []queue.Delivery{{Stream: "nh-media:analysis", EntryID: "1-0", Message: queue.Message{MessageID: "msg_analysis_001", Capability: "analysis", JobID: "job_analysis_001", JobStepID: "step_analysis_001", Attempt: 1}}}}
	publisher := &fakeCommandPublisher{}
	command := worker.Command{SchemaVersion: worker.CommandSchemaVersion, MessageID: "msg_analysis_001", Capability: "analysis", ProjectID: "project_analysis_001", JobID: "job_analysis_001", PipelineRunID: "run_analysis_001", JobStepID: "step_analysis_001", Attempt: 1, InputRefs: []worker.ArtifactRef{}, Config: map[string]any{}}
	count, err := (Dispatcher{Queue: q, Resolver: fakeCommandResolver{command: command}, Publisher: publisher}).DispatchOnce(context.Background(), "analysis", "controller_001", 10)
	if err != nil || count != 1 || q.acked != 1 || publisher.published != 1 {
		t.Fatalf("dispatch count=%d err=%v acked=%d published=%d", count, err, q.acked, publisher.published)
	}
}

type fakeResultSource struct {
	delivery queue.ResultDelivery
	acked    int
}

func (f *fakeResultSource) ConsumeWorkerResults(context.Context, string, string, int, time.Duration) ([]queue.ResultDelivery, error) {
	return []queue.ResultDelivery{f.delivery}, nil
}
func (f *fakeResultSource) AckWorkerResult(context.Context, string, string) error {
	f.acked++
	return nil
}

type fakeResultSink struct {
	seen int
	err  error
}

func (f *fakeResultSink) HandleWorkerResult(context.Context, queue.ResultDelivery) error {
	f.seen++
	return f.err
}

func TestResultReconcilerDoesNotAckFailedStateWrite(t *testing.T) {
	source := &fakeResultSource{delivery: queue.ResultDelivery{Stream: "results", EntryID: "1-0", Attempt: 1, Result: worker.Result{SchemaVersion: worker.ResultSchemaVersion, MessageID: "msg_analysis_001", JobID: "job_analysis_001", JobStepID: "step_analysis_001", Status: "completed", OutputRefs: []worker.OutputRef{}}}}
	sink := &fakeResultSink{err: errors.New("state unavailable")}
	count, err := (ResultReconciler{Source: source, Sink: sink}).ReconcileOnce(context.Background(), "analysis", "controller_001", 1, time.Second)
	if err == nil || count != 0 || source.acked != 0 || sink.seen != 1 {
		t.Fatalf("count=%d err=%v acked=%d seen=%d", count, err, source.acked, sink.seen)
	}
}
