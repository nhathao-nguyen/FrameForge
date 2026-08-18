package execution

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type memoryOutbox struct {
	records   []OutboxRecord
	published []string
	failed    []string
}

func (m *memoryOutbox) ClaimOutbox(context.Context, int) ([]OutboxRecord, error) {
	return append([]OutboxRecord(nil), m.records...), nil
}
func (m *memoryOutbox) MarkOutboxPublished(_ context.Context, id string) error {
	m.published = append(m.published, id)
	return nil
}
func (m *memoryOutbox) MarkOutboxFailed(_ context.Context, id, _ string, _ time.Time) error {
	m.failed = append(m.failed, id)
	return nil
}

type memorySink struct {
	fail bool
	seen []string
}

func (m *memorySink) Publish(_ context.Context, record OutboxRecord) error {
	if m.fail {
		return errors.New("temporary sink outage")
	}
	m.seen = append(m.seen, record.ID)
	return nil
}

func TestOutboxPublisherPublishesStableIDsAndRetriesFailures(t *testing.T) {
	store := &memoryOutbox{records: []OutboxRecord{{ID: "evt_1", EventType: "job.queued", Payload: json.RawMessage(`{}`)}}}
	sink := &memorySink{}
	publisher := OutboxPublisher{Store: store, Sink: sink, BatchSize: 1, Clock: func() time.Time { return time.Unix(10, 0) }}
	report, err := publisher.PublishOnce(context.Background())
	if err != nil || report != (PublishReport{Claimed: 1, Published: 1}) {
		t.Fatalf("unexpected publish result: %+v, %v", report, err)
	}
	if len(sink.seen) != 1 || sink.seen[0] != "evt_1" || store.published[0] != "evt_1" {
		t.Fatalf("stable ID was not preserved: sink=%v published=%v", sink.seen, store.published)
	}

	store = &memoryOutbox{records: []OutboxRecord{{ID: "evt_2"}}}
	sink = &memorySink{fail: true}
	report, err = (OutboxPublisher{Store: store, Sink: sink, RetryAfter: time.Second, Clock: func() time.Time { return time.Unix(10, 0) }}).PublishOnce(context.Background())
	if err != nil || report.Failed != 1 || len(store.failed) != 1 {
		t.Fatalf("failed publish was not retryable: %+v, %v, failed=%v", report, err, store.failed)
	}
}

func TestOutboxPublisherRequiresBothBoundaries(t *testing.T) {
	_, err := (OutboxPublisher{}).PublishOnce(context.Background())
	if err == nil {
		t.Fatal("expected missing boundary error")
	}
}
