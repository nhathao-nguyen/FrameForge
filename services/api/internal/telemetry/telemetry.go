// Package telemetry provides bounded-cardinality request metrics and redacted
// structured events for the Local/LAN operations boundary.
package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/security"
)

type metricKey struct{ Name, Route, Status string }

type Metrics struct {
	mu        sync.Mutex
	maxSeries int
	counters  map[metricKey]uint64
	dropped   uint64
	startedAt time.Time
}

func NewMetrics(maxSeries int) *Metrics {
	if maxSeries <= 0 {
		maxSeries = 256
	}
	return &Metrics{maxSeries: maxSeries, counters: make(map[metricKey]uint64), startedAt: time.Now().UTC()}
}

func (m *Metrics) Observe(name, route, status string) {
	if m == nil {
		return
	}
	key := metricKey{Name: safeMetricName(name), Route: RouteLabel(route), Status: safeStatus(status)}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.counters[key]; !ok && len(m.counters) >= m.maxSeries {
		m.dropped++
		return
	}
	m.counters[key]++
}

func (m *Metrics) Snapshot() map[string]uint64 {
	result := make(map[string]uint64)
	if m == nil {
		return result
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, value := range m.counters {
		result[key.Name+"|route="+key.Route+"|status="+key.Status] = value
	}
	return result
}

func (m *Metrics) Prometheus() string {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	values := make([]string, 0, len(m.counters)+2)
	values = append(values, "# TYPE nh_media_telemetry_series_dropped_total counter", fmt.Sprintf("nh_media_telemetry_series_dropped_total %d", m.dropped))
	for key, value := range m.counters {
		values = append(values, fmt.Sprintf("nh_media_%s{route=\"%s\",status=\"%s\"} %d", key.Name, escapeLabel(key.Route), escapeLabel(key.Status), value))
	}
	m.mu.Unlock()
	sort.Strings(values)
	return strings.Join(values, "\n") + "\n"
}

func (m *Metrics) Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = io.WriteString(w, m.Prometheus())
}

type Event struct {
	Time          string            `json:"time"`
	Level         string            `json:"level"`
	Message       string            `json:"message"`
	RequestID     string            `json:"request_id,omitempty"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	Fields        map[string]string `json:"fields,omitempty"`
}

func WriteEvent(w io.Writer, ctx context.Context, level, message string, fields map[string]string) error {
	if w == nil {
		return fmt.Errorf("telemetry writer is required")
	}
	redacted := security.RedactFields(fields)
	requestID, correlationID := IDs(ctx)
	event := Event{Time: time.Now().UTC().Format(time.RFC3339Nano), Level: safeMetricName(level), Message: security.RedactText(message), RequestID: requestID, CorrelationID: correlationID, Fields: redacted}
	return json.NewEncoder(w).Encode(event)
}

func IDs(ctx context.Context) (string, string) {
	if ctx == nil {
		return "", ""
	}
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	correlationID, _ := ctx.Value(correlationIDKey{}).(string)
	return requestID, correlationID
}

// These private context keys avoid depending on the HTTP package's context
// implementation while allowing tests and future workers to emit the same
// correlation fields.
type requestIDKey struct{}
type correlationIDKey struct{}

func safeMetricName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func safeStatus(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	if len(value) > 3 {
		value = value[:3]
	}
	return value
}

func escapeLabel(value string) string {
	return strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n").Replace(value)
}

func RouteLabel(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "api" && parts[1] == "v1" {
		if len(parts) >= 3 {
			return "/api/v1/" + parts[2]
		}
		return "/api/v1"
	}
	return "/other"
}
