package execution

import (
	"context"
	"errors"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/queue"
)

// CommandResolver loads the immutable pipeline/job snapshot from the Product
// database. Queue messages intentionally carry only IDs and an attempt.
type CommandResolver interface {
	ResolveCommand(context.Context, queue.Message) (worker.Command, error)
}

type CommandPublisher interface {
	PublishWorkerCommand(context.Context, worker.Command) (string, error)
}

// Dispatcher performs the trusted first hop: resolve durable state, validate
// the versioned worker command, publish it to a capability-specific worker
// stream, then acknowledge the ID-only execution message. If the process dies
// before the acknowledgement, the original message is safely redelivered.
type Dispatcher struct {
	Queue     queue.QueuePort
	Resolver  CommandResolver
	Publisher CommandPublisher
}

func (d Dispatcher) Run(ctx context.Context, capability, consumer string, interval time.Duration) error {
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := d.DispatchOnce(ctx, capability, consumer, 10); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (d Dispatcher) DispatchOnce(ctx context.Context, capability, consumer string, count int) (int, error) {
	if d.Queue == nil || d.Resolver == nil || d.Publisher == nil {
		return 0, errors.New("execution dispatcher dependencies are required")
	}
	deliveries, err := d.Queue.Consume(ctx, capability, consumer, count, 0)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, delivery := range deliveries {
		command, err := d.Resolver.ResolveCommand(ctx, delivery.Message)
		if err != nil {
			return processed, err
		}
		if _, err := d.Publisher.PublishWorkerCommand(ctx, command); err != nil {
			return processed, err
		}
		if err := d.Queue.Ack(ctx, delivery.Stream, delivery.EntryID); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

type ResultSource interface {
	ConsumeWorkerResults(context.Context, string, string, int, time.Duration) ([]queue.ResultDelivery, error)
	AckWorkerResult(context.Context, string, string) error
}

type ResultSink interface {
	HandleWorkerResult(context.Context, queue.ResultDelivery) error
}

// ResultReconciler applies worker results through a guarded state port before
// acknowledging the result stream. It is deliberately separate from the
// worker process so PostgreSQL remains the source of truth.
type ResultReconciler struct {
	Source ResultSource
	Sink   ResultSink
}

func (r ResultReconciler) ReconcileOnce(ctx context.Context, capability, consumer string, count int, block time.Duration) (int, error) {
	if r.Source == nil || r.Sink == nil {
		return 0, errors.New("result reconciler dependencies are required")
	}
	deliveries, err := r.Source.ConsumeWorkerResults(ctx, capability, consumer, count, block)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, delivery := range deliveries {
		if err := r.Sink.HandleWorkerResult(ctx, delivery); err != nil {
			return processed, err
		}
		if err := r.Source.AckWorkerResult(ctx, delivery.Stream, delivery.EntryID); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}
