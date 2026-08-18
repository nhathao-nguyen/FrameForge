package health

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

type testDependency struct {
	name string
	err  error
}

func (d testDependency) Name() string                { return d.name }
func (d testDependency) Check(context.Context) error { return d.err }

func TestReadinessReportsDependencyOutageAndDrain(t *testing.T) {
	registry := NewRegistry([]Dependency{testDependency{name: "redis", err: errors.New("down")}}, time.Second)
	request := httptest.NewRequest("GET", "/api/v1/ready", nil)
	response := httptest.NewRecorder()
	registry.ReadyHandler().ServeHTTP(response, request)
	if response.Code != 503 || response.Body.String() == "" {
		t.Fatalf("outage was not visible: %d %s", response.Code, response.Body.String())
	}
	registry.SetDraining(true)
	response = httptest.NewRecorder()
	registry.ReadyHandler().ServeHTTP(response, request)
	if response.Code != 503 || response.Body.String() == "" {
		t.Fatalf("drain was not visible: %d %s", response.Code, response.Body.String())
	}
}
