package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type OutboxRecord struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       json.RawMessage
	Attempts      int
	AvailableAt   time.Time
}

type EventRecord struct {
	ID             string
	SchemaVersion  string
	EventType      string
	Sequence       int64
	OccurredAt     time.Time
	WorkspaceID    string
	ProjectID      string
	JobID          string
	PipelineRunID  string
	JobStepID      string
	PipelineNodeID string
	NodeKey        string
	CorrelationID  string
	Payload        json.RawMessage
}

type OutboxStore interface {
	ClaimOutbox(context.Context, int) ([]OutboxRecord, error)
	MarkOutboxPublished(context.Context, string) error
	MarkOutboxFailed(context.Context, string, string, time.Time) error
}

type EventSink interface {
	Publish(context.Context, OutboxRecord) error
}

type OutboxPublisher struct {
	Store      OutboxStore
	Sink       EventSink
	BatchSize  int
	RetryAfter time.Duration
	Clock      func() time.Time
}

type PublishReport struct {
	Claimed   int
	Published int
	Failed    int
}

func (p OutboxPublisher) PublishOnce(ctx context.Context) (PublishReport, error) {
	if p.Store == nil || p.Sink == nil {
		return PublishReport{}, errors.New("outbox store and sink are required")
	}
	limit := p.BatchSize
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	retryAfter := p.RetryAfter
	if retryAfter <= 0 {
		retryAfter = 5 * time.Second
	}
	clock := p.Clock
	if clock == nil {
		clock = time.Now
	}
	records, err := p.Store.ClaimOutbox(ctx, limit)
	if err != nil {
		return PublishReport{}, err
	}
	report := PublishReport{Claimed: len(records)}
	for _, record := range records {
		if err := p.Sink.Publish(ctx, record); err != nil {
			report.Failed++
			// The durable row stays retryable. The message ID is stable, so a
			// crash between sink publication and this update is harmless to an
			// idempotent sink.
			if markErr := p.Store.MarkOutboxFailed(ctx, record.ID, safePublishError(err), clock().UTC().Add(retryAfter)); markErr != nil {
				return report, markErr
			}
			continue
		}
		if err := p.Store.MarkOutboxPublished(ctx, record.ID); err != nil {
			return report, err
		}
		report.Published++
	}
	return report, nil
}

func safePublishError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 240 {
		message = message[:240]
	}
	return fmt.Sprintf("publish failed: %s", message)
}
