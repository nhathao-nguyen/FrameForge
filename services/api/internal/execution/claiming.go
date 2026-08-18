package execution

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/queue"
)

type ClaimedCommand struct {
	Message queue.Message
	Command worker.Command
	Lease   Lease
}

type LeaseResolver interface {
	ClaimCommand(context.Context, queue.Message) (worker.Command, Lease, error)
}

// LeaseRegistry keeps only opaque attempt handles in controller memory. The
// database remains canonical; a controller restart intentionally loses this
// map and lets lease reconciliation classify the abandoned attempt.
type LeaseRegistry struct {
	mu    sync.Mutex
	items map[string]ClaimedCommand
}

func NewLeaseRegistry() *LeaseRegistry { return &LeaseRegistry{items: make(map[string]ClaimedCommand)} }

func (r *LeaseRegistry) Put(value ClaimedCommand) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[value.Command.MessageID] = value
}

func (r *LeaseRegistry) Take(messageID string) (ClaimedCommand, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.items[messageID]
	if ok {
		delete(r.items, messageID)
	}
	return value, ok
}

type ClaimingDispatcher struct {
	Queue     queue.QueuePort
	Resolver  LeaseResolver
	Publisher CommandPublisher
	Claims    *LeaseRegistry
}

func (d ClaimingDispatcher) DispatchOnce(ctx context.Context, capability, consumer string, count int) (int, error) {
	if d.Queue == nil || d.Resolver == nil || d.Publisher == nil || d.Claims == nil {
		return 0, errors.New("claiming dispatcher dependencies are required")
	}
	deliveries, err := d.Queue.Consume(ctx, capability, consumer, count, 0)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, delivery := range deliveries {
		command, lease, err := d.Resolver.ClaimCommand(ctx, delivery.Message)
		if err != nil {
			return processed, err
		}
		// Register the lease before publishing. A fast worker can finish the
		// command before PublishWorkerCommand returns, so registering after the
		// publish creates a result-reconciliation race.
		d.Claims.Put(ClaimedCommand{Message: delivery.Message, Command: command, Lease: lease})
		if _, err := d.Publisher.PublishWorkerCommand(ctx, command); err != nil {
			d.Claims.Take(command.MessageID)
			return processed, err
		}
		if err := d.Queue.Ack(ctx, delivery.Stream, delivery.EntryID); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (d ClaimingDispatcher) Run(ctx context.Context, capability, consumer string, interval time.Duration) error {
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

type ClaimedResult struct {
	Delivery queue.ResultDelivery
	Claim    ClaimedCommand
}

type ClaimedResultApplier interface {
	ApplyClaimedResult(context.Context, ClaimedResult) error
}

type LeaseAwareResultReconciler struct {
	Source  queueResultSource
	Claims  *LeaseRegistry
	Applier ClaimedResultApplier
}

type queueResultSource interface {
	ConsumeWorkerResults(context.Context, string, string, int, time.Duration) ([]queue.ResultDelivery, error)
	AckWorkerResult(context.Context, string, string) error
}

func (r LeaseAwareResultReconciler) ReconcileOnce(ctx context.Context, capability, consumer string, count int, block time.Duration) (int, error) {
	if r.Source == nil || r.Claims == nil || r.Applier == nil {
		return 0, errors.New("lease-aware result reconciler dependencies are required")
	}
	deliveries, err := r.Source.ConsumeWorkerResults(ctx, capability, consumer, count, block)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, delivery := range deliveries {
		claim, ok := r.Claims.Take(delivery.Result.MessageID)
		if !ok {
			// Do not ACK a result whose controller lease is gone. A later
			// reconciliation pass can inspect the durable attempt/expiry.
			return processed, ErrLeaseConflict
		}
		if err := r.Applier.ApplyClaimedResult(ctx, ClaimedResult{Delivery: delivery, Claim: claim}); err != nil {
			r.Claims.Put(claim)
			return processed, err
		}
		if err := r.Source.AckWorkerResult(ctx, delivery.Stream, delivery.EntryID); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (r LeaseAwareResultReconciler) Run(ctx context.Context, capability, consumer string, interval time.Duration) error {
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	for {
		if _, err := r.ReconcileOnce(ctx, capability, consumer, 10, interval); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}
