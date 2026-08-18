package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/product"
)

func TestJobSSESnapshotReplayReconnectAndInvalidCursor(t *testing.T) {
	server, _ := testServer(t, nil)
	login := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"correct-password"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(login, loginRequest)
	if login.Code != http.StatusOK {
		t.Fatalf("login: %d %s", login.Code, login.Body.String())
	}
	var loginBody map[string]any
	_ = json.Unmarshal(login.Body.Bytes(), &loginBody)
	token, _ := loginBody["access_token"].(string)
	withAuth := func(request *http.Request) {
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
	}

	projectResponse := httptest.NewRecorder()
	projectRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"name":"sse","workflow_key":"movie_recap"}`))
	withAuth(projectRequest)
	server.Handler().ServeHTTP(projectResponse, projectRequest)
	if projectResponse.Code != http.StatusCreated {
		t.Fatalf("project: %d %s", projectResponse.Code, projectResponse.Body.String())
	}
	var project product.Project
	if err := json.Unmarshal(projectResponse.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}

	jobResponse := httptest.NewRecorder()
	jobRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/jobs", strings.NewReader(`{"kind":"analysis","input":{"nodes":["analysis"]}}`))
	withAuth(jobRequest)
	server.Handler().ServeHTTP(jobResponse, jobRequest)
	if jobResponse.Code != http.StatusCreated {
		t.Fatalf("job: %d %s", jobResponse.Code, jobResponse.Body.String())
	}
	var job product.Job
	if err := json.Unmarshal(jobResponse.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	store, ok := server.Product.(*product.Store)
	if !ok {
		t.Fatal("test server did not use the in-memory product store")
	}
	if _, err := store.StartJob(store.WorkspaceID(), project.ID, job.ID); err != nil {
		t.Fatal(err)
	}

	streamResponse := httptest.NewRecorder()
	streamRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/jobs/"+job.ID+"/events/stream", nil)
	withAuth(streamRequest)
	done := make(chan struct{})
	go func() {
		server.Handler().ServeHTTP(streamResponse, streamRequest)
		close(done)
	}()
	time.Sleep(350 * time.Millisecond)
	if _, err := store.CancelJob(store.WorkspaceID(), project.ID, job.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("SSE stream did not close after terminal event")
	}
	body := streamResponse.Body.String()
	for _, marker := range []string{"stream.snapshot", "job.created", "job.queued", "job.cancelled"} {
		if !strings.Contains(body, marker) {
			t.Fatalf("SSE body missing %q: %s", marker, body)
		}
	}

	reconnect := httptest.NewRecorder()
	reconnectRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/jobs/"+job.ID+"/events/stream", nil)
	withAuth(reconnectRequest)
	reconnectRequest.Header.Set("Last-Event-ID", "1")
	server.Handler().ServeHTTP(reconnect, reconnectRequest)
	if !strings.Contains(reconnect.Body.String(), "job.cancelled") {
		t.Fatalf("reconnect did not replay terminal event: %s", reconnect.Body.String())
	}

	invalid := httptest.NewRecorder()
	invalidRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/jobs/"+job.ID+"/events/stream", nil)
	withAuth(invalidRequest)
	invalidRequest.Header.Set("Last-Event-ID", "not-a-sequence")
	server.Handler().ServeHTTP(invalid, invalidRequest)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid Last-Event-ID status=%d body=%s", invalid.Code, invalid.Body.String())
	}
}
