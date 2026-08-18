package scheduler

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type Node struct {
	Key          string
	Dependencies []string
	Optional     bool
}

type Step struct {
	Key    string
	Status string
}

type Frontier struct {
	Ready   []string
	Blocked []string
}

var terminalSuccess = map[string]bool{"completed": true, "skipped": true}
var terminalFailure = map[string]bool{"failed": true, "cancelled": true, "blocked": true}

// ResolveFrontier computes a deterministic graph frontier without knowing the
// workflow's business meaning. It only understands node dependencies and the
// canonical JobStep state vocabulary.
func ResolveFrontier(nodes []Node, steps []Step, max int) (Frontier, error) {
	graph, err := validateGraph(nodes)
	if err != nil {
		return Frontier{}, err
	}
	if max <= 0 {
		max = 100
	}
	if max > 500 {
		max = 500
	}
	states := make(map[string]string, len(steps))
	for _, step := range steps {
		if _, ok := graph[step.Key]; !ok {
			return Frontier{}, fmt.Errorf("step %q is not in the pipeline graph", step.Key)
		}
		if _, exists := states[step.Key]; exists {
			return Frontier{}, fmt.Errorf("duplicate step %q", step.Key)
		}
		states[step.Key] = step.Status
	}
	result := Frontier{}
	for _, node := range nodes {
		if states[node.Key] != "pending" {
			continue
		}
		blocked := false
		ready := true
		for _, dependency := range node.Dependencies {
			status, exists := states[dependency]
			if !exists {
				return Frontier{}, fmt.Errorf("dependency step %q is missing", dependency)
			}
			if terminalFailure[status] {
				blocked = true
				break
			}
			if !terminalSuccess[status] {
				ready = false
			}
		}
		if blocked {
			result.Blocked = append(result.Blocked, node.Key)
		} else if ready {
			result.Ready = append(result.Ready, node.Key)
		}
	}
	sort.Strings(result.Ready)
	sort.Strings(result.Blocked)
	if len(result.Ready) > max {
		result.Ready = result.Ready[:max]
	}
	if len(result.Blocked) > max {
		result.Blocked = result.Blocked[:max]
	}
	return result, nil
}

func validateGraph(nodes []Node) (map[string]Node, error) {
	if len(nodes) == 0 {
		return nil, errors.New("pipeline graph cannot be empty")
	}
	graph := make(map[string]Node, len(nodes))
	for _, node := range nodes {
		if strings.TrimSpace(node.Key) == "" {
			return nil, errors.New("pipeline node key is required")
		}
		if _, exists := graph[node.Key]; exists {
			return nil, fmt.Errorf("duplicate pipeline node %q", node.Key)
		}
		graph[node.Key] = node
	}
	for _, node := range nodes {
		seen := make(map[string]struct{}, len(node.Dependencies))
		for _, dependency := range node.Dependencies {
			if dependency == node.Key {
				return nil, fmt.Errorf("node %q depends on itself", node.Key)
			}
			if _, ok := graph[dependency]; !ok {
				return nil, fmt.Errorf("node %q depends on missing node %q", node.Key, dependency)
			}
			if _, ok := seen[dependency]; ok {
				return nil, fmt.Errorf("node %q repeats dependency %q", node.Key, dependency)
			}
			seen[dependency] = struct{}{}
		}
	}
	if hasCycle(graph) {
		return nil, errors.New("pipeline graph contains a cycle")
	}
	return graph, nil
}

func hasCycle(graph map[string]Node) bool {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) bool
	visit = func(key string) bool {
		if visiting[key] {
			return true
		}
		if visited[key] {
			return false
		}
		visiting[key] = true
		for _, dependency := range graph[key].Dependencies {
			if visit(dependency) {
				return true
			}
		}
		delete(visiting, key)
		visited[key] = true
		return false
	}
	for key := range graph {
		if visit(key) {
			return true
		}
	}
	return false
}
