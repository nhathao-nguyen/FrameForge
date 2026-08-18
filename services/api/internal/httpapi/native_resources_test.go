package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

func TestNativeProjectUploadTimelineAndProviderBoundaries(t *testing.T) {
	server, _ := testServer(t, nil)
	login := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"correct-password"}`))
	request.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(login, request)
	if login.Code != http.StatusOK {
		t.Fatalf("login: %d %s", login.Code, login.Body.String())
	}
	var authResponse map[string]any
	_ = json.Unmarshal(login.Body.Bytes(), &authResponse)
	token, _ := authResponse["access_token"].(string)
	header := func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
	}
	projectResponse := httptest.NewRecorder()
	projectRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"name":"native","workflow_key":"movie_recap","settings":{}}`))
	header(projectRequest)
	server.Handler().ServeHTTP(projectResponse, projectRequest)
	if projectResponse.Code != http.StatusCreated {
		t.Fatalf("project: %d %s", projectResponse.Code, projectResponse.Body.String())
	}
	var project map[string]any
	_ = json.Unmarshal(projectResponse.Body.Bytes(), &project)
	projectID, _ := project["id"].(string)
	uploadResponse := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/assets/upload-sessions", strings.NewReader(`{"kind":"video","filename":"source.mp4","content_type":"video/mp4","size_bytes":12,"multipart":true}`))
	header(uploadRequest)
	server.Handler().ServeHTTP(uploadResponse, uploadRequest)
	if uploadResponse.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", uploadResponse.Code, uploadResponse.Body.String())
	}
	if strings.Contains(uploadResponse.Body.String(), "not-a-media") {
		t.Fatal("upload response must not contain media bytes")
	}
	var uploadEnvelope map[string]any
	_ = json.Unmarshal(uploadResponse.Body.Bytes(), &uploadEnvelope)
	assetID, _ := uploadEnvelope["asset"].(map[string]any)["id"].(string)
	uploadID, _ := uploadEnvelope["upload"].(map[string]any)["id"].(string)
	completeResponse := httptest.NewRecorder()
	completeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/assets/"+assetID+"/upload-sessions/"+uploadID+"/complete", strings.NewReader(`{"parts":[{"part_number":1,"etag":"etag-1"}]}`))
	header(completeRequest)
	server.Handler().ServeHTTP(completeResponse, completeRequest)
	if completeResponse.Code != http.StatusAccepted || !strings.Contains(completeResponse.Body.String(), "validation_job_id") {
		t.Fatalf("upload completion: %d %s", completeResponse.Code, completeResponse.Body.String())
	}
	timeline := `{"origin":"user","document":{"schema_version":"1.0","timeline_id":"tl_1","timeline_version_id":"tlv_1","project_id":"` + projectID + `","version":1,"duration_sec":10,"tracks":[{"id":"track_1","kind":"video","name":"Footage","order":0,"clips":[{"id":"clip_1","timeline_in_sec":0,"timeline_out_sec":4,"source":{"type":"none","inline_id":"clip_1"},"origin":"user"}]}]}}`
	timelineResponse := httptest.NewRecorder()
	timelineRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/timelines", strings.NewReader(timeline))
	header(timelineRequest)
	server.Handler().ServeHTTP(timelineResponse, timelineRequest)
	if timelineResponse.Code != http.StatusCreated {
		t.Fatalf("timeline: %d %s", timelineResponse.Code, timelineResponse.Body.String())
	}
	var timelineEnvelope map[string]any
	_ = json.Unmarshal(timelineResponse.Body.Bytes(), &timelineEnvelope)
	version, _ := timelineEnvelope["version"].(map[string]any)
	timelineID, _ := timelineEnvelope["timeline"].(map[string]any)["id"].(string)
	versionID, _ := version["id"].(string)
	document, _ := version["document"].(map[string]any)
	if document["timeline_id"] != timelineID || document["timeline_version_id"] != versionID || document["version"] != float64(1) {
		t.Fatalf("stored TimelineVersion identity drifted: timeline=%q version=%q document=%#v", timelineID, versionID, document)
	}
	listResponse := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/timelines/"+timelineID+"/versions", nil)
	header(listRequest)
	server.Handler().ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), versionID) {
		t.Fatalf("timeline version history route failed: %d %s", listResponse.Code, listResponse.Body.String())
	}
	wrongTimelineResponse := httptest.NewRecorder()
	wrongTimelineRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/timelines/not-the-owner/versions/"+versionID, nil)
	header(wrongTimelineRequest)
	server.Handler().ServeHTTP(wrongTimelineResponse, wrongTimelineRequest)
	if wrongTimelineResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-timeline version path was not scoped: %d %s", wrongTimelineResponse.Code, wrongTimelineResponse.Body.String())
	}
	approveResponse := httptest.NewRecorder()
	approveRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/timelines/"+timelineID+"/versions/"+versionID+"/approve", strings.NewReader(`{}`))
	header(approveRequest)
	server.Handler().ServeHTTP(approveResponse, approveRequest)
	if approveResponse.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", approveResponse.Code, approveResponse.Body.String())
	}
	providerResponse := httptest.NewRecorder()
	providerRequest := httptest.NewRequest(http.MethodPost, "/api/v1/provider-configurations", strings.NewReader(`{"kind":"tts","adapter_key":"fake","display_name":"local","secret":"must-not-be-accepted"}`))
	header(providerRequest)
	server.Handler().ServeHTTP(providerResponse, providerRequest)
	if providerResponse.Code != http.StatusUnprocessableEntity || !strings.Contains(providerResponse.Body.String(), "PLAINTEXT_SECRET") {
		t.Fatalf("secret boundary: %d %s", providerResponse.Code, providerResponse.Body.String())
	}
}

func TestNativeWorkspaceGuessFailsClosed(t *testing.T) {
	server, _ := testServer(t, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws_other", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected unauthenticated response: %d", response.Code)
	}
}

func TestNativeUploadUsesInjectedStoragePortAndQueuesValidation(t *testing.T) {
	backend, err := storage.NewLocalStorage(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	server, _ := testServerWithStorage(t, nil, backend)
	login := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"correct-password"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(login, loginRequest)
	var loginBody map[string]any
	_ = json.Unmarshal(login.Body.Bytes(), &loginBody)
	token, _ := loginBody["access_token"].(string)
	if token == "" {
		t.Fatalf("login: %d %s", login.Code, login.Body.String())
	}
	header := func(request *http.Request) {
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
	}
	projectResponse := httptest.NewRecorder()
	projectRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"name":"storage-backed","workflow_key":"movie_recap"}`))
	header(projectRequest)
	server.Handler().ServeHTTP(projectResponse, projectRequest)
	var project map[string]any
	_ = json.Unmarshal(projectResponse.Body.Bytes(), &project)
	projectID, _ := project["id"].(string)
	uploadResponse := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/assets/upload-sessions", strings.NewReader(`{"kind":"video","filename":"source.mp4","content_type":"video/mp4","size_bytes":10,"multipart":true}`))
	header(uploadRequest)
	server.Handler().ServeHTTP(uploadResponse, uploadRequest)
	if uploadResponse.Code != http.StatusCreated || strings.Contains(uploadResponse.Body.String(), "nh-media-direct://") {
		t.Fatalf("storage-backed upload was not presigned through StoragePort: %d %s", uploadResponse.Code, uploadResponse.Body.String())
	}
	var envelope map[string]any
	_ = json.Unmarshal(uploadResponse.Body.Bytes(), &envelope)
	assetID, _ := envelope["asset"].(map[string]any)["id"].(string)
	uploadID, _ := envelope["upload"].(map[string]any)["id"].(string)
	completeResponse := httptest.NewRecorder()
	completeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/assets/"+assetID+"/upload-sessions/"+uploadID+"/complete", strings.NewReader(`{"parts":[{"part_number":1,"etag":"local-etag"}]}`))
	header(completeRequest)
	server.Handler().ServeHTTP(completeResponse, completeRequest)
	if completeResponse.Code != http.StatusAccepted {
		t.Fatalf("storage-backed completion: %d %s", completeResponse.Code, completeResponse.Body.String())
	}
	assetResponse := httptest.NewRecorder()
	assetRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/assets/"+assetID, nil)
	header(assetRequest)
	server.Handler().ServeHTTP(assetResponse, assetRequest)
	var asset map[string]any
	_ = json.Unmarshal(assetResponse.Body.Bytes(), &asset)
	if asset["status"] != "validating" {
		t.Fatalf("completed upload became ready without validation: %#v", asset)
	}
}
