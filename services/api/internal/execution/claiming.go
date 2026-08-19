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
	reclaimed, err := d.Queue.Reclaim(ctx, capability, consumer, 2*time.Minute, count)
	if err != nil {
		return 0, err
	}
	deliveries := reclaimed
	if len(deliveries) < count {
		fresh, err := d.Queue.Consume(ctx, capability, consumer, count-len(deliveries), 0)
		if err != nil {
			return 0, err
		}
		deliveries = append(deliveries, fresh...)
	}
	processed := 0
	for _, delivery := range deliveries {
		command, lease, err := d.Resolver.ClaimCommand(ctx, delivery.Message)
		if err != nil {
			if errors.Is(err, ErrLeaseConflict) {
				// A duplicate or stale ID-only delivery is no longer authorized
				// by PostgreSQL. Acknowledge only this queue entry and continue;
				// one stale entry must not stop the capability dispatcher.
				if ackErr := d.Queue.Ack(ctx, delivery.Stream, delivery.EntryID); ackErr != nil {
					return processed, ackErr
				}
				continue
			}
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

// RecoveredResultApplier is the durable restart path. The raw lease token is
// intentionally memory-only, so a restarted controller uses the non-secret
// attempt fencing ID carried by a newer worker result instead.
type RecoveredResultApplier interface {
	ApplyRecoveredResult(context.Context, queue.ResultDelivery) error
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
			if recovered, recoverable := r.Applier.(RecoveredResultApplier); recoverable {
				if err := recovered.ApplyRecoveredResult(ctx, delivery); err == nil {
					if err := r.Source.AckWorkerResult(ctx, delivery.Stream, delivery.EntryID); err != nil {
						return processed, err
					}
					processed++
					continue
				} else if !errors.Is(err, ErrLeaseConflict) {
					return processed, err
				}
			}
			// Do not ACK a result whose controller lease is gone. A later
			// reconciliation pass can inspect the durable attempt/expiry. Newer
			// results with an AttemptID may have already been applied above.
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
			if errors.Is(err, ErrLeaseConflict) {
				// A controller restart intentionally loses its in-memory lease
				// registry. Do not terminate the result loop when a late result
				// from that controller is encountered; its durable attempt will
				// be reconciled by the lease sweeper and the stream remains live
				// for newer claims.
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(interval):
				}
				continue
			}
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}
