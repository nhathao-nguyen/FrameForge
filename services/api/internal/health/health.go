package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"sync/atomic"
	"time"
)

type Dependency interface {
	Name() string
	Check(ctx context.Context) error
}

// FuncDependency adapts a bounded runtime probe to the public health
// registry without exposing provider-specific error details in HTTP output.
type FuncDependency struct {
	DependencyName string
	CheckFunc      func(context.Context) error
}

func (d FuncDependency) Name() string { return d.DependencyName }

func (d FuncDependency) Check(ctx context.Context) error {
	if d.CheckFunc == nil {
		return context.Canceled
	}
	return d.CheckFunc(ctx)
}

type Registry struct {
	dependencies []Dependency
	timeout      time.Duration
	draining     atomic.Bool
}

type DependencyStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func NewRegistry(dependencies []Dependency, timeout time.Duration) *Registry {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &Registry{dependencies: append([]Dependency(nil), dependencies...), timeout: timeout}
}

func (r *Registry) SetDraining(value bool) { r.draining.Store(value) }
func (r *Registry) IsDraining() bool       { return r.draining.Load() }

func (r *Registry) LiveHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "live"})
	})
}

func (r *Registry) ReadyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if r.draining.Load() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "draining"})
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), r.timeout)
		defer cancel()
		ready, statuses := r.Check(ctx)
		status := "ready"
		code := http.StatusOK
		if !ready {
			status = "not_ready"
			code = http.StatusServiceUnavailable
		}
		// Dependency names and failure details stay on the internal Check/metrics
		// boundary; the public readiness contract remains deliberately minimal.
		_ = statuses
		writeJSON(w, code, map[string]string{"status": status})
	})
}

func (r *Registry) Check(ctx context.Context) (bool, []DependencyStatus) {
	if r == nil {
		return false, []DependencyStatus{{Name: "health_registry", Status: "missing"}}
	}
	statuses := make([]DependencyStatus, 0, len(r.dependencies))
	ready := true
	for _, dependency := range r.dependencies {
		status := "ready"
		if dependency == nil || dependency.Check(ctx) != nil {
			status = "unavailable"
			ready = false
		}
		name := "unknown"
		if dependency != nil && dependency.Name() != "" {
			name = dependency.Name()
		}
		statuses = append(statuses, DependencyStatus{Name: name, Status: status})
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Name < statuses[j].Name })
	return ready, statuses
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
