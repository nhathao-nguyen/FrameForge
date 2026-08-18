package execution

import (
	"errors"
	"fmt"
)

// RecoveryStep is the state needed by the controller to make a crash-resume
// decision. It intentionally contains only durable refs and hashes; an
// executor-local path is never part of a resume plan.
type RecoveryStep struct {
	NodeKey          string
	Status           string
	SchemaVersion    string
	InputFingerprint string
	Checkpoint       *Checkpoint
}

type ResumePlan struct {
	ReuseNodeKeys  []string
	QueueNodeKeys  []string
	PauseAfterNode string
}

// BuildResumePlan deterministically converts a persisted execution graph into
// the next safe frontier after an API/worker crash. Active leases must first be
// reconciled by the lease sweeper; a review gate is never silently bypassed.
func BuildResumePlan(steps []RecoveryStep, schemaVersion, startFrom, stopAfter string) (ResumePlan, error) {
	if len(steps) == 0 {
		return ResumePlan{}, errors.New("resume graph is empty")
	}
	plan := ResumePlan{ReuseNodeKeys: []string{}, QueueNodeKeys: []string{}}
	startIndex, stopIndex := -1, len(steps)-1
	for index, step := range steps {
		if step.NodeKey == "" {
			return ResumePlan{}, errors.New("resume node key is required")
		}
		if startFrom != "" && step.NodeKey == startFrom {
			startIndex = index
		}
		if stopAfter != "" && step.NodeKey == stopAfter {
			stopIndex = index
		}
	}
	if startFrom != "" && startIndex < 0 {
		return ResumePlan{}, fmt.Errorf("start_from node %q is not in the persisted graph", startFrom)
	}
	if stopAfter != "" && stopIndex == len(steps)-1 && steps[stopIndex].NodeKey != stopAfter {
		return ResumePlan{}, fmt.Errorf("stop_after node %q is not in the persisted graph", stopAfter)
	}
	if startIndex >= 0 && stopAfter != "" && startIndex > stopIndex {
		return ResumePlan{}, errors.New("resume boundaries are reversed")
	}

	for index, step := range steps {
		if step.Status == "waiting_for_review" {
			return ResumePlan{}, fmt.Errorf("resume is blocked by review at node %q", step.NodeKey)
		}
		if step.Status == "failed" || step.Status == "cancelled" {
			return ResumePlan{}, fmt.Errorf("resume cannot bypass terminal failure at node %q", step.NodeKey)
		}
		if step.Status == "running" {
			return ResumePlan{}, fmt.Errorf("active lease at node %q must be reconciled before resume", step.NodeKey)
		}
		if step.Status == "completed" || step.Status == "skipped" {
			if step.Checkpoint == nil || !CanReuseCheckpoint(*step.Checkpoint, schemaVersion, step.InputFingerprint) {
				return ResumePlan{}, fmt.Errorf("node %q has no compatible checkpoint", step.NodeKey)
			}
			plan.ReuseNodeKeys = append(plan.ReuseNodeKeys, step.NodeKey)
			continue
		}
		if step.Status != "pending" && step.Status != "ready" && step.Status != "queued" && step.Status != "paused" && step.Status != "retrying" {
			return ResumePlan{}, fmt.Errorf("node %q has unsupported recovery state %q", step.NodeKey, step.Status)
		}
		if startIndex >= 0 && index < startIndex {
			if step.Checkpoint == nil || !CanReuseCheckpoint(*step.Checkpoint, schemaVersion, step.InputFingerprint) {
				return ResumePlan{}, fmt.Errorf("start_from requires compatible checkpoint for node %q", step.NodeKey)
			}
			plan.ReuseNodeKeys = append(plan.ReuseNodeKeys, step.NodeKey)
			continue
		}
		if index > stopIndex {
			continue
		}
		plan.QueueNodeKeys = append(plan.QueueNodeKeys, step.NodeKey)
		if stopAfter != "" && index == stopIndex {
			plan.PauseAfterNode = step.NodeKey
		}
	}
	return plan, nil
}
