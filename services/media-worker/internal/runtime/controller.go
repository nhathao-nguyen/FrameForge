package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/security"
	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/worker"
	"github.com/redis/go-redis/v9"
)

type Executor interface {
	Execute(context.Context, worker.Command) (worker.Result, error)
}

type RedisOptions struct {
	Addr          string
	Password      string
	DB            int
	StreamPrefix  string
	ConsumerGroup string
	Consumer      string
	Capability    string
	MaxLen        int64
}

type RedisController struct {
	client       *redis.Client
	prefix       string
	group        string
	consumer     string
	capability   string
	maxLen       int64
	groupCreated bool
	executor     Executor
	draining     atomic.Bool
}

func NewRedisController(options RedisOptions, executor Executor) (*RedisController, error) {
	if strings.TrimSpace(options.Addr) == "" || executor == nil {
		return nil, errors.New("worker Redis address and executor are required")
	}
	capability := strings.TrimSpace(options.Capability)
	if capability != "probe" && capability != "thumbnail" && capability != "analysis" && capability != "media" && capability != "render" {
		return nil, errors.New("worker capability is not enabled")
	}
	prefix := strings.Trim(strings.TrimSpace(options.StreamPrefix), ":")
	if prefix == "" {
		prefix = "nh-media"
	}
	group := strings.TrimSpace(options.ConsumerGroup)
	if group == "" {
		group = "nh-media-workers"
	}
	consumer := strings.TrimSpace(options.Consumer)
	if consumer == "" {
		consumer = "media-worker-1"
	}
	maxLen := options.MaxLen
	if maxLen <= 0 {
		maxLen = 10000
	}
	return &RedisController{client: redis.NewClient(&redis.Options{Addr: options.Addr, Password: options.Password, DB: options.DB}), prefix: prefix, group: group, consumer: consumer, capability: capability, maxLen: maxLen, executor: executor}, nil
}

func (c *RedisController) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

func (c *RedisController) Ping(ctx context.Context) error {
	if c == nil || c.client == nil {
		return errors.New("worker Redis controller is not initialized")
	}
	return c.client.Ping(ctx).Err()
}

// BeginDrain stops new claims while allowing the current RunOnce invocation to
// finish. PostgreSQL remains authoritative; a later controller can reclaim any
// lease that expires without fabricating database state.
func (c *RedisController) BeginDrain() {
	if c != nil {
		c.draining.Store(true)
	}
}
func (c *RedisController) IsDraining() bool { return c != nil && c.draining.Load() }

func (c *RedisController) workerStream() string { return c.prefix + ":" + c.capability + ":worker" }
func (c *RedisController) resultStream() string { return c.prefix + ":" + c.capability + ":results" }

func (c *RedisController) ensureGroup(ctx context.Context) error {
	if c.groupCreated {
		return nil
	}
	err := c.client.XGroupCreateMkStream(ctx, c.workerStream(), c.group, "0").Err()
	if err != nil && !strings.Contains(strings.ToUpper(err.Error()), "BUSYGROUP") {
		return err
	}
	c.groupCreated = true
	return nil
}

func (c *RedisController) RunOnce(ctx context.Context, block time.Duration) (int, error) {
	if c == nil || c.client == nil {
		return 0, errors.New("worker Redis controller is not initialized")
	}
	if c.draining.Load() {
		return 0, nil
	}
	if err := c.ensureGroup(ctx); err != nil {
		return 0, err
	}
	if block < 0 {
		block = 0
	}
	if block > 30*time.Second {
		block = 30 * time.Second
	}
	groups, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: c.group, Consumer: c.consumer, Streams: []string{c.workerStream(), ">"}, Count: 1, Block: block}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}
	processed := 0
	for _, group := range groups {
		for _, entry := range group.Messages {
			command, err := decodeCommand(entry.Values)
			if err != nil {
				return processed, err
			}
			result, executeErr := c.executor.Execute(ctx, command)
			if executeErr != nil {
				result = failedResult(command, executeErr)
			}
			if err := worker.ValidateResult(result); err != nil {
				return processed, fmt.Errorf("worker executor returned invalid result: %w", err)
			}
			body, err := json.Marshal(result)
			if err != nil {
				return processed, err
			}
			if _, err := c.client.XAdd(ctx, &redis.XAddArgs{Stream: c.resultStream(), MaxLen: c.maxLen, Approx: true, Values: map[string]string{"message_id": result.MessageID, "attempt": strconv.Itoa(command.Attempt), "result": string(body)}}).Result(); err != nil {
				return processed, err
			}
			if err := c.client.XAck(ctx, c.workerStream(), c.group, entry.ID).Err(); err != nil {
				return processed, err
			}
			processed++
		}
	}
	return processed, nil
}

func (c *RedisController) Run(ctx context.Context) error {
	for {
		if _, err := c.RunOnce(ctx, time.Second); err != nil {
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

func decodeCommand(values map[string]any) (worker.Command, error) {
	raw, ok := values["command"]
	if !ok {
		return worker.Command{}, errors.New("worker stream entry has no command")
	}
	var body string
	switch typed := raw.(type) {
	case string:
		body = typed
	case []byte:
		body = string(typed)
	default:
		body = fmt.Sprint(typed)
	}
	return worker.DecodeCommand([]byte(body))
}

func failedResult(command worker.Command, cause error) worker.Result {
	message := security.SafeError(cause)
	return worker.Result{SchemaVersion: worker.ResultSchemaVersion, MessageID: command.MessageID, JobID: command.JobID, JobStepID: command.JobStepID, Status: "failed", OutputRefs: []worker.OutputRef{}, SafeError: &worker.SafeError{Code: "WORKER_EXECUTION_FAILED", Category: "internal", Retryable: true, SafeMessage: message}}
}
