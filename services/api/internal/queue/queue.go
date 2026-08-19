package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/redis/go-redis/v9"
)

const (
	defaultGroup      = "nh-media-workers"
	maxBatch          = 100
	resultReclaimIdle = 30 * time.Second
)

type Message struct {
	MessageID      string
	Capability     string
	JobID          string
	PipelineRunID  string
	JobStepID      string
	PipelineNodeID string
	NodeKey        string
	Attempt        int
}

func (m Message) validate() error {
	if strings.TrimSpace(m.MessageID) == "" || strings.TrimSpace(m.Capability) == "" || strings.TrimSpace(m.JobID) == "" || strings.TrimSpace(m.JobStepID) == "" {
		return errors.New("queue message requires message, capability, job and step IDs")
	}
	if m.Attempt < 1 {
		return errors.New("queue attempt must be positive")
	}
	for key, value := range map[string]string{"message_id": m.MessageID, "capability": m.Capability, "job_id": m.JobID, "pipeline_run_id": m.PipelineRunID, "job_step_id": m.JobStepID, "pipeline_node_id": m.PipelineNodeID, "node_key": m.NodeKey} {
		if strings.ContainsAny(value, "\r\n") || strings.Contains(value, "\\") {
			return fmt.Errorf("queue field %s contains unsafe characters", key)
		}
	}
	return nil
}

type Delivery struct {
	Stream  string
	EntryID string
	Message Message
}

// ResultDelivery is the worker result envelope. Attempt is deliberately kept
// outside the versioned result contract so the result body remains portable;
// the controller uses it to match the leased attempt before it mutates state.
type ResultDelivery struct {
	Stream  string
	EntryID string
	Attempt int
	JobID   string
	StepID  string
	Result  worker.Result
}

type QueuePort interface {
	Enqueue(context.Context, Message) (string, error)
	Consume(context.Context, string, string, int, time.Duration) ([]Delivery, error)
	Ack(context.Context, string, string) error
	Reclaim(context.Context, string, string, time.Duration, int) ([]Delivery, error)
}

type RedisOptions struct {
	Addr          string
	Password      string
	DB            int
	StreamPrefix  string
	ConsumerGroup string
	MaxLen        int64
}

type RedisQueue struct {
	client        *redis.Client
	prefix        string
	group         string
	maxLen        int64
	groupMu       sync.Mutex
	groupsCreated map[string]bool
}

func NewRedisQueue(options RedisOptions) (*RedisQueue, error) {
	if strings.TrimSpace(options.Addr) == "" {
		return nil, errors.New("Redis queue address is required")
	}
	prefix := strings.Trim(strings.TrimSpace(options.StreamPrefix), ":")
	if prefix == "" {
		prefix = "nh-media"
	}
	group := strings.TrimSpace(options.ConsumerGroup)
	if group == "" {
		group = defaultGroup
	}
	maxLen := options.MaxLen
	if maxLen <= 0 {
		maxLen = 10000
	}
	return &RedisQueue{client: redis.NewClient(&redis.Options{Addr: options.Addr, Password: options.Password, DB: options.DB}), prefix: prefix, group: group, maxLen: maxLen, groupsCreated: make(map[string]bool)}, nil
}

func (q *RedisQueue) Close() error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.Close()
}

func (q *RedisQueue) Ping(ctx context.Context) error {
	if q == nil || q.client == nil {
		return errors.New("Redis queue is not initialized")
	}
	return q.client.Ping(ctx).Err()
}

func (q *RedisQueue) stream(capability string) (string, error) {
	capability = strings.TrimSpace(capability)
	if capability == "" || strings.ContainsAny(capability, ":\r\n\\") {
		return "", errors.New("invalid queue capability")
	}
	return q.prefix + ":" + capability, nil
}

func (q *RedisQueue) ensureGroup(ctx context.Context, stream string) error {
	q.groupMu.Lock()
	if q.groupsCreated[stream] {
		q.groupMu.Unlock()
		return nil
	}
	q.groupMu.Unlock()
	err := q.client.XGroupCreateMkStream(ctx, stream, q.group, "0").Err()
	if err != nil && !strings.Contains(strings.ToUpper(err.Error()), "BUSYGROUP") {
		return err
	}
	q.groupMu.Lock()
	q.groupsCreated[stream] = true
	q.groupMu.Unlock()
	return nil
}

func (q *RedisQueue) Enqueue(ctx context.Context, message Message) (string, error) {
	if q == nil || q.client == nil {
		return "", errors.New("Redis queue is not initialized")
	}
	if err := message.validate(); err != nil {
		return "", err
	}
	stream, err := q.stream(message.Capability)
	if err != nil {
		return "", err
	}
	if err := q.ensureGroup(ctx, stream); err != nil {
		return "", err
	}
	return q.client.XAdd(ctx, &redis.XAddArgs{Stream: stream, MaxLen: q.maxLen, Approx: true, Values: map[string]string{
		"message_id": message.MessageID, "capability": message.Capability, "job_id": message.JobID, "pipeline_run_id": message.PipelineRunID,
		"job_step_id": message.JobStepID, "pipeline_node_id": message.PipelineNodeID, "node_key": message.NodeKey, "attempt": strconv.Itoa(message.Attempt),
	}}).Result()
}

// WorkerStreamName is the second-hop stream used after the controller has
// resolved the durable ID-only message into a versioned worker command. The
// public execution queue never contains a command or mutable input snapshot.
func (q *RedisQueue) WorkerStreamName(capability string) (string, error) {
	stream, err := q.stream(capability)
	if err != nil {
		return "", err
	}
	return stream + ":worker", nil
}

// ResultStreamName is the bounded worker-to-controller result stream.
func (q *RedisQueue) ResultStreamName(capability string) (string, error) {
	stream, err := q.stream(capability)
	if err != nil {
		return "", err
	}
	return stream + ":results", nil
}

// PublishWorkerCommand publishes a validated command to the worker-specific
// second hop. The command is safe contract data; it never contains local paths,
// credentials, bearer tokens or expiring URLs.
func (q *RedisQueue) PublishWorkerCommand(ctx context.Context, command worker.Command) (string, error) {
	if q == nil || q.client == nil {
		return "", errors.New("Redis queue is not initialized")
	}
	if err := worker.ValidateCommand(command); err != nil {
		return "", err
	}
	stream, err := q.WorkerStreamName(command.Capability)
	if err != nil {
		return "", err
	}
	if err := q.ensureGroup(ctx, stream); err != nil {
		return "", err
	}
	body, err := json.Marshal(command)
	if err != nil {
		return "", err
	}
	return q.client.XAdd(ctx, &redis.XAddArgs{Stream: stream, MaxLen: q.maxLen, Approx: true, Values: map[string]string{
		"message_id": command.MessageID,
		"attempt":    strconv.Itoa(command.Attempt),
		"command":    string(body),
	}}).Result()
}

// ConsumeWorkerResults reads only bounded, validated result envelopes. A
// controller acknowledges the entry after the guarded database result write.
func (q *RedisQueue) ConsumeWorkerResults(ctx context.Context, capability, consumer string, count int, block time.Duration) ([]ResultDelivery, error) {
	if q == nil || q.client == nil {
		return nil, errors.New("Redis queue is not initialized")
	}
	if strings.TrimSpace(consumer) == "" {
		return nil, errors.New("Redis result consumer is required")
	}
	stream, err := q.ResultStreamName(capability)
	if err != nil {
		return nil, err
	}
	if err := q.ensureGroup(ctx, stream); err != nil {
		return nil, err
	}
	count = boundedCount(count)
	block = normalizedBlock(block)
	if block > 30*time.Second {
		block = 30 * time.Second
	}
	// Results are acknowledged only after the guarded database apply. Reclaim
	// abandoned pending entries so a controller restart can finish a result
	// that arrived just before the old reconciler died. The idle threshold is
	// long enough to avoid stealing a result during normal bounded apply work.
	claimed, _, err := q.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: stream, Group: q.group, Consumer: consumer, MinIdle: resultReclaimIdle, Start: "0-0", Count: int64(count)}).Result()
	if err != nil {
		return nil, err
	}
	messages := append([]redis.XMessage(nil), claimed...)
	if len(messages) < count {
		entries, readErr := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: q.group, Consumer: consumer, Streams: []string{stream, ">"}, Count: int64(count - len(messages)), Block: block}).Result()
		if readErr != nil && !errors.Is(readErr, redis.Nil) {
			return nil, readErr
		}
		for _, group := range entries {
			messages = append(messages, group.Messages...)
		}
	}
	result := make([]ResultDelivery, 0, len(messages))
	for _, entry := range messages {
		read := func(key string) string {
			value, _ := entry.Values[key]
			switch typed := value.(type) {
			case string:
				return typed
			case []byte:
				return string(typed)
			default:
				return fmt.Sprint(typed)
			}
		}
		attempt, err := strconv.Atoi(read("attempt"))
		if err != nil || attempt < 1 {
			return nil, errors.New("worker result attempt is invalid")
		}
		decoded, err := worker.DecodeResult([]byte(read("result")))
		if err != nil {
			return nil, fmt.Errorf("decode worker result: %w", err)
		}
		result = append(result, ResultDelivery{Stream: stream, EntryID: entry.ID, Attempt: attempt, JobID: read("job_id"), StepID: read("job_step_id"), Result: decoded})
	}
	return result, nil
}

func (q *RedisQueue) AckWorkerResult(ctx context.Context, stream, entryID string) error {
	if q == nil || q.client == nil {
		return errors.New("Redis queue is not initialized")
	}
	if stream == "" || entryID == "" {
		return errors.New("worker result acknowledgement requires stream and entry ID")
	}
	return q.client.XAck(ctx, stream, q.group, entryID).Err()
}

func (q *RedisQueue) Consume(ctx context.Context, capability, consumer string, count int, block time.Duration) ([]Delivery, error) {
	if q == nil || q.client == nil {
		return nil, errors.New("Redis queue is not initialized")
	}
	if strings.TrimSpace(consumer) == "" {
		return nil, errors.New("Redis consumer is required")
	}
	stream, err := q.stream(capability)
	if err != nil {
		return nil, err
	}
	if err := q.ensureGroup(ctx, stream); err != nil {
		return nil, err
	}
	count = boundedCount(count)
	block = normalizedBlock(block)
	if block > 30*time.Second {
		block = 30 * time.Second
	}
	entries, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: q.group, Consumer: consumer, Streams: []string{stream, ">"}, Count: int64(count), Block: block}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return []Delivery{}, nil
		}
		return nil, err
	}
	return decodeDeliveries(stream, entries)
}

func (q *RedisQueue) Ack(ctx context.Context, stream, entryID string) error {
	if q == nil || q.client == nil {
		return errors.New("Redis queue is not initialized")
	}
	if stream == "" || entryID == "" {
		return errors.New("queue acknowledgement requires stream and entry ID")
	}
	return q.client.XAck(ctx, stream, q.group, entryID).Err()
}

func (q *RedisQueue) Reclaim(ctx context.Context, capability, consumer string, minIdle time.Duration, count int) ([]Delivery, error) {
	if q == nil || q.client == nil {
		return nil, errors.New("Redis queue is not initialized")
	}
	stream, err := q.stream(capability)
	if err != nil {
		return nil, err
	}
	if err := q.ensureGroup(ctx, stream); err != nil {
		return nil, err
	}
	if minIdle < time.Second {
		minIdle = time.Second
	}
	result, _, err := q.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: stream, Group: q.group, Consumer: consumer, MinIdle: minIdle, Start: "0-0", Count: int64(boundedCount(count))}).Result()
	if err != nil {
		return nil, err
	}
	return decodeMessages(stream, result)
}

func boundedCount(count int) int {
	if count <= 0 {
		return 1
	}
	if count > maxBatch {
		return maxBatch
	}
	return count
}

func normalizedBlock(block time.Duration) time.Duration {
	if block < 0 {
		return 0
	}
	if block == 0 {
		// Redis interprets BLOCK 0 as wait forever. The QueuePort contract
		// uses zero for a bounded poll, which dispatchers need for progress
		// loops and shutdown checks.
		return time.Millisecond
	}
	if block > 30*time.Second {
		return 30 * time.Second
	}
	return block
}

func decodeDeliveries(stream string, groups []redis.XStream) ([]Delivery, error) {
	result := make([]Delivery, 0)
	for _, group := range groups {
		values, err := decodeMessages(stream, group.Messages)
		if err != nil {
			return nil, err
		}
		result = append(result, values...)
	}
	return result, nil
}

func decodeMessages(stream string, messages []redis.XMessage) ([]Delivery, error) {
	result := make([]Delivery, 0, len(messages))
	for _, message := range messages {
		value, err := decodeMessage(message.Values)
		if err != nil {
			return nil, err
		}
		result = append(result, Delivery{Stream: stream, EntryID: message.ID, Message: value})
	}
	return result, nil
}

func decodeMessage(fields map[string]any) (Message, error) {
	read := func(key string) string {
		value, _ := fields[key]
		switch typed := value.(type) {
		case string:
			return typed
		case []byte:
			return string(typed)
		default:
			return fmt.Sprint(typed)
		}
	}
	attempt, err := strconv.Atoi(read("attempt"))
	if err != nil {
		return Message{}, errors.New("queue message attempt is invalid")
	}
	message := Message{MessageID: read("message_id"), Capability: read("capability"), JobID: read("job_id"), PipelineRunID: read("pipeline_run_id"), JobStepID: read("job_step_id"), PipelineNodeID: read("pipeline_node_id"), NodeKey: read("node_key"), Attempt: attempt}
	if err := message.validate(); err != nil {
		return Message{}, err
	}
	return message, nil
}
