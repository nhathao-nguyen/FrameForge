// Package inventory compares durable Artifact metadata with object-store
// observations. It never deletes objects; deletion remains an explicit,
// audited operation after a clean report.
package inventory

import "sort"

type ExpectedArtifact struct {
	Backend           string
	ObjectKey         string
	SHA256            string
	SizeBytes         int64
	Role              string
	RetainUntilDelete bool
}

type Object struct {
	Backend   string
	ObjectKey string
	SHA256    string
	SizeBytes int64
}

type Finding struct {
	Kind      string `json:"kind"`
	Backend   string `json:"backend"`
	ObjectKey string `json:"object_key"`
	Reason    string `json:"reason"`
}

type Report struct {
	Expected int       `json:"expected"`
	Observed int       `json:"observed"`
	Findings []Finding `json:"findings"`
	Passed   bool      `json:"passed"`
}

func Compare(expected []ExpectedArtifact, observed []Object) Report {
	byKey := make(map[string]Object, len(observed))
	for _, value := range observed {
		byKey[key(value.Backend, value.ObjectKey)] = value
	}
	seen := make(map[string]bool, len(expected))
	findings := make([]Finding, 0)
	for _, value := range expected {
		id := key(value.Backend, value.ObjectKey)
		seen[id] = true
		actual, ok := byKey[id]
		if !ok {
			findings = append(findings, Finding{Kind: "missing", Backend: value.Backend, ObjectKey: value.ObjectKey, Reason: "durable Artifact metadata has no matching object"})
			continue
		}
		if value.SizeBytes != actual.SizeBytes || value.SHA256 != "" && actual.SHA256 != "" && value.SHA256 != actual.SHA256 {
			findings = append(findings, Finding{Kind: "corrupt", Backend: value.Backend, ObjectKey: value.ObjectKey, Reason: "size or checksum differs from durable Artifact metadata"})
		}
	}
	for _, value := range observed {
		if !seen[key(value.Backend, value.ObjectKey)] {
			findings = append(findings, Finding{Kind: "orphan", Backend: value.Backend, ObjectKey: value.ObjectKey, Reason: "object has no committed Artifact metadata"})
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		return findings[i].ObjectKey < findings[j].ObjectKey
	})
	return Report{Expected: len(expected), Observed: len(observed), Findings: findings, Passed: len(findings) == 0}
}

func key(backend, objectKey string) string { return backend + "\x00" + objectKey }

func RetentionAllowsDelete(role string, explicitAuditedDelete bool) bool {
	if explicitAuditedDelete {
		return true
	}
	return role == "scratch"
}
