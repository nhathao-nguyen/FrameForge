package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/auth"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/config"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/health"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

type dependency struct {
	name  string
	err   error
	delay time.Duration
}

func (d dependency) Name() string { return d.name }
func (d dependency) Check(ctx context.Context) error {
	select {
	case <-time.After(d.delay):
		return d.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func testServer(t *testing.T, dependencies []health.Dependency) (*Server, *auth.LocalAuthProvider) {
	return testServerWithStorage(t, dependencies, nil)
}

func testServerWithStorage(t *testing.T, dependencies []health.Dependency, backend storage.StoragePort) (*Server, *auth.LocalAuthProvider) {
	t.Helper()
	provider, err := auth.NewLocalAuthProvider("admin", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	value, err := config.LoadAPI(func(name string) string {
		values := map[string]string{"NH_MEDIA_PROFILE": "local", "NH_API_ALLOWED_ORIGINS": "http://localhost:3000", "NH_API_MASTER_KEY": base64.RawStdEncoding.EncodeToString(make([]byte, 32))}
		return values[name]
	})
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServerWithStorage(value, provider, health.NewRegistry(dependencies, 20*time.Millisecond), backend)
	if err != nil {
		t.Fatal(err)
	}
	return server, provider
}

func TestIDsErrorsCORSAndBoundedBody(t *testing.T) {
	server, _ := testServer(t, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("X-Request-ID", "req_client_1")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("X-Request-ID") != "req_client_1" || response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("unexpected response: %d %#v", response.Code, response.Header())
	}
	bad := httptest.NewRequest(http.MethodGet, "/api/v1", nil)
	bad.Header.Set("Origin", "http://evil.example")
	denied := httptest.NewRecorder()
	server.Handler().ServeHTTP(denied, bad)
	if denied.Code != http.StatusForbidden || strings.Contains(denied.Body.String(), "secret") {
		t.Fatalf("unsafe CORS response: %s", denied.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(denied.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["serialization_version"] != "1" {
		t.Fatal("missing serialization version")
	}
}

func TestLoginAndProtectedDiagnostics(t *testing.T) {
	server, provider := testServer(t, nil)
	login := httptest.NewRequest(http.MethodPost, "/api/v1/auth/local/login", strings.NewReader(`{"username":"admin","password":"correct-password"}`))
	login.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login: %d %s", loginResponse.Code, loginResponse.Body.String())
	}
	var value map[string]string
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if value["access_token"] == "" {
		t.Fatal("missing session token")
	}
	diagnostics := httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics", nil)
	diagnostics.Header.Set("Authorization", "Bearer "+value["access_token"])
	diagnosticsResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(diagnosticsResponse, diagnostics)
	if diagnosticsResponse.Code != http.StatusOK {
		t.Fatalf("diagnostics: %d", diagnosticsResponse.Code)
	}
	if _, err := provider.Authenticate(context.Background(), value["access_token"]); err != nil {
		t.Fatal(err)
	}
}

func TestReadinessDependencyDownTimeoutAndDrain(t *testing.T) {
	server, _ := testServer(t, []health.Dependency{dependency{name: "postgres", err: errors.New("down")}})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || response.Body.String() != "{\"status\":\"not_ready\"}\n" {
		t.Fatalf("down dependency disclosed/handled incorrectly: %d %s", response.Code, response.Body.String())
	}
	server, _ = testServer(t, []health.Dependency{dependency{name: "redis", delay: 100 * time.Millisecond}})
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("timeout readiness: %d", response.Code)
	}
	server.Health.SetDraining(true)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "draining") {
		t.Fatalf("drain readiness: %d %s", response.Code, response.Body.String())
	}
	live := httptest.NewRecorder()
	server.Handler().ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/api/v1/live", nil))
	if live.Code != http.StatusOK {
		t.Fatalf("liveness during drain: %d", live.Code)
	}
}
