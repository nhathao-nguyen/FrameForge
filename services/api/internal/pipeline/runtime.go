package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

type Outcome string

const (
	Completed        Outcome = "completed"
	Skipped          Outcome = "skipped"
	Paused           Outcome = "paused"
	WaitingForReview Outcome = "waiting_for_review"
	Failed           Outcome = "failed"
	Cancelled        Outcome = "cancelled"
)

type NodeError struct {
	Category  string
	Message   string
	Retryable bool
}

type Result struct {
	Outcome    Outcome
	Outputs    map[string]any
	Checkpoint map[string]any
	Error      *NodeError
	Review     map[string]any
}

type Executor interface {
	Execute(context.Context, Node, State) Result
}

type State struct {
	JobID       string
	RunID       string
	Inputs      map[string]any
	NodeOutputs map[string]map[string]any
	Attempts    map[string]int
}

type Request struct {
	JobID     string
	RunID     string
	Inputs    map[string]any
	StartFrom string
	StopAfter string
	Strict    bool
}

type NodeResult struct {
	Key        string         `json:"key"`
	Outcome    Outcome        `json:"outcome"`
	Attempts   int            `json:"attempts"`
	Outputs    map[string]any `json:"outputs,omitempty"`
	Checkpoint map[string]any `json:"checkpoint,omitempty"`
	Error      *NodeError     `json:"error,omitempty"`
	Review     map[string]any `json:"review,omitempty"`
	Reused     bool           `json:"reused,omitempty"`
}

type RunResult struct {
	Outcome     Outcome      `json:"outcome"`
	Nodes       []NodeResult `json:"nodes"`
	Checkpoints []NodeResult `json:"checkpoints,omitempty"`
	Fingerprint string       `json:"fingerprint"`
}

type Runtime struct {
	Definition Definition
	Executors  map[string]Executor
	Committed  map[string]Result
}

func (r *Runtime) Run(ctx context.Context, request Request) RunResult {
	result := RunResult{Outcome: Completed, Nodes: make([]NodeResult, 0, len(r.Definition.Nodes))}
	if ctx == nil {
		ctx = context.Background()
	}
	state := State{JobID: request.JobID, RunID: request.RunID, Inputs: cloneMap(request.Inputs), NodeOutputs: map[string]map[string]any{}, Attempts: map[string]int{}}
	nodes := r.Definition.NodeMap()
	completed := map[string]bool{}
	for _, node := range r.Definition.Nodes {
		if request.StartFrom != "" && node.Key != request.StartFrom && !hasPathTo(node.Key, request.StartFrom, nodes) {
			completed[node.Key] = true
			result.Nodes = append(result.Nodes, NodeResult{Key: node.Key, Outcome: Skipped, Reused: true})
		}
	}
	remaining := len(nodes) - len(completed)
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			result.Outcome = Cancelled
			result.Nodes = append(result.Nodes, NodeResult{Key: "runtime", Outcome: Cancelled, Error: &NodeError{Category: "cancellation", Message: "run cancelled"}})
			break
		}
		ready := readyNodes(r.Definition.Nodes, completed, result.Nodes)
		if len(ready) == 0 {
			result.Outcome = Failed
			result.Nodes = append(result.Nodes, NodeResult{Key: "runtime", Outcome: Failed, Error: &NodeError{Category: "permanent", Message: "DAG cannot make progress"}})
			break
		}
		for _, node := range ready {
			if remaining == 0 {
				break
			}
			if err := ctx.Err(); err != nil {
				result.Outcome = Cancelled
				break
			}
			fingerprint := nodeFingerprint(r.Definition, node, state)
			if node.Idempotency.Reuse {
				if committed, ok := r.Committed[fingerprint]; ok && committed.Outcome == Completed {
					completed[node.Key] = true
					remaining--
					state.NodeOutputs[node.Key] = cloneMap(committed.Outputs)
					result.Nodes = append(result.Nodes, NodeResult{Key: node.Key, Outcome: Completed, Attempts: 0, Outputs: cloneMap(committed.Outputs), Reused: true})
					continue
				}
			}
			executor := r.Executors[node.Key]
			if executor == nil {
				executor = r.Executors[node.Type]
			}
			if executor == nil {
				completed[node.Key] = true
				remaining--
				result.Outcome = Failed
				result.Nodes = append(result.Nodes, NodeResult{Key: node.Key, Outcome: Failed, Error: &NodeError{Category: "permanent", Message: "node executor is unavailable"}})
				break
			}
			maxAttempts := node.Retry.MaxAttempts
			if maxAttempts < 1 {
				maxAttempts = 1
			}
			var nodeResult NodeResult
			for attempt := 1; attempt <= maxAttempts; attempt++ {
				state.Attempts[node.Key] = attempt
				invokeCtx := ctx
				var cancel context.CancelFunc
				if node.Timeout.WallTimeSec > 0 {
					invokeCtx, cancel = context.WithTimeout(ctx, time.Duration(node.Timeout.WallTimeSec)*time.Second)
				}
				value := executor.Execute(invokeCtx, node, state)
				if cancel != nil {
					cancel()
				}
				if invokeCtx.Err() == context.DeadlineExceeded && value.Outcome == "" {
					value = Result{Outcome: Failed, Error: &NodeError{Category: "timeout", Message: "node exceeded wall-time policy", Retryable: true}}
				}
				nodeResult = NodeResult{Key: node.Key, Outcome: value.Outcome, Attempts: attempt, Outputs: cloneMap(value.Outputs), Checkpoint: cloneMap(value.Checkpoint), Error: value.Error, Review: cloneMap(value.Review)}
				if value.Outcome == Completed || value.Outcome == Skipped || value.Outcome == WaitingForReview || value.Outcome == Paused || value.Outcome == Cancelled {
					break
				}
				if value.Error == nil || !value.Error.Retryable || attempt == maxAttempts {
					break
				}
			}
			completed[node.Key] = true
			remaining--
			result.Nodes = append(result.Nodes, nodeResult)
			switch nodeResult.Outcome {
			case Completed:
				state.NodeOutputs[node.Key] = cloneMap(nodeResult.Outputs)
				if nodeResult.Checkpoint != nil {
					result.Checkpoints = append(result.Checkpoints, nodeResult)
				}
				if r.Committed == nil {
					r.Committed = map[string]Result{}
				}
				r.Committed[fingerprint] = Result{Outcome: Completed, Outputs: cloneMap(nodeResult.Outputs), Checkpoint: cloneMap(nodeResult.Checkpoint)}
			case Skipped:
				if request.Strict || node.FailureMode == "hard" {
					result.Outcome = Failed
				}
			case WaitingForReview:
				result.Outcome = WaitingForReview
			case Paused:
				result.Outcome = Paused
			case Cancelled:
				result.Outcome = Cancelled
			case Failed:
				if request.Strict || node.FailureMode == "hard" {
					result.Outcome = Failed
				}
			}
			if result.Outcome != Completed {
				remaining = 0
				break
			}
			if request.StopAfter == node.Key {
				result.Outcome = Paused
				break
			}
		}
		if result.Outcome != Completed {
			break
		}
	}
	result.Fingerprint = runFingerprint(result)
	return result
}

func readyNodes(def []Node, completed map[string]bool, results []NodeResult) []Node {
	known := map[string]NodeResult{}
	for _, value := range results {
		known[value.Key] = value
	}
	ready := make([]Node, 0)
	for _, node := range def {
		if completed[node.Key] {
			continue
		}
		ok := true
		for _, dependency := range node.DependsOn {
			if !completed[dependency.NodeKey] {
				ok = false
				break
			}
			if dependency.Required {
				value := known[dependency.NodeKey]
				if value.Outcome != Completed && value.Outcome != Skipped {
					ok = false
					break
				}
			}
		}
		if ok {
			ready = append(ready, node)
		}
	}
	sort.Slice(ready, func(i, j int) bool { return ready[i].Key < ready[j].Key })
	return ready
}

func hasPathTo(key, target string, nodes map[string]Node) bool {
	if key == target {
		return true
	}
	for _, dependency := range nodes[key].DependsOn {
		if hasPathTo(dependency.NodeKey, target, nodes) {
			return true
		}
	}
	return false
}

func nodeFingerprint(def Definition, node Node, state State) string {
	payload := map[string]any{"workflow": def.WorkflowKey, "version": def.Version, "node": node.Key, "inputs": state.Inputs, "outputs": state.NodeOutputs, "policy": node.Idempotency.FingerprintFields}
	encoded, _ := json.Marshal(payload)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func runFingerprint(result RunResult) string {
	encoded, _ := json.Marshal(result.Nodes)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	encoded, _ := json.Marshal(value)
	var result map[string]any
	_ = json.Unmarshal(encoded, &result)
	return result
}

type DeterministicExecutor struct {
	Results map[string][]Result
}

func (e *DeterministicExecutor) Execute(_ context.Context, node Node, _ State) Result {
	if e != nil && len(e.Results[node.Key]) > 0 {
		value := e.Results[node.Key][0]
		e.Results[node.Key] = e.Results[node.Key][1:]
		return value
	}
	return Result{Outcome: Completed, Outputs: map[string]any{"node_key": node.Key}}
}

func ValidateResult(value Result) error {
	switch value.Outcome {
	case Completed, Skipped, Paused, WaitingForReview, Failed, Cancelled:
		return nil
	default:
		return fmt.Errorf("invalid node outcome %q", value.Outcome)
	}
}

var _ = errors.New
