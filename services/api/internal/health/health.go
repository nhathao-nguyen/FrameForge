package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

type Dependency interface {
	Name() string
	Check(ctx context.Context) error
}

type Registry struct {
	dependencies []Dependency
	timeout      time.Duration
	draining     atomic.Bool
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
		for _, dependency := range r.dependencies {
			if err := dependency.Check(ctx); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
