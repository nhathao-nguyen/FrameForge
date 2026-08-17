package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/domain"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/product"
)

func (s *Server) registerProjectRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/projects", s.createProject)
	mux.HandleFunc("GET /api/v1/projects", s.listProjects)
	mux.HandleFunc("GET /api/v1/projects/{project_id}", s.getProject)
	mux.HandleFunc("PATCH /api/v1/projects/{project_id}", s.patchProject)
	mux.HandleFunc("DELETE /api/v1/projects/{project_id}", s.deleteProject)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/assets/upload-sessions", s.createUpload)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/assets", s.listAssets)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/assets/{asset_id}", s.getAsset)
	mux.HandleFunc("PATCH /api/v1/projects/{project_id}/assets/{asset_id}", s.patchAsset)
	mux.HandleFunc("DELETE /api/v1/projects/{project_id}/assets/{asset_id}", s.deleteAsset)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/assets/{asset_id}/upload-sessions/{upload_id}/parts", s.uploadParts)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/assets/{asset_id}/upload-sessions/{upload_id}/complete", s.completeUpload)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/assets/{asset_id}/upload-sessions/{upload_id}/abort", s.abortUpload)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/scripts", s.createScript)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/scripts/{script_id}", s.getScript)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/scripts/{script_id}/versions", s.createScriptVersion)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/scripts/{script_id}/versions/{version_id}", s.getScriptVersion)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/scripts/{script_id}/versions/{version_id}/approve", s.approveScriptVersion)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/scripts/{script_id}/versions/{version_id}/reject", s.rejectScriptVersion)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/narrations", s.createNarration)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/narrations/{narration_id}", s.getNarration)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/timelines", s.createTimeline)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/timelines/{timeline_id}", s.getTimeline)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/timelines/{timeline_id}/commands", s.timelineCommand)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/timelines/{timeline_id}/versions/{version_id}", s.getTimelineVersion)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/timelines/{timeline_id}/versions/{version_id}/validate", s.validateTimeline)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/timelines/{timeline_id}/versions/{version_id}/approve", s.approveTimeline)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/timelines/{timeline_id}/versions/{version_id}/lock", s.lockTimeline)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/renders", s.createRender)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/analyses", s.createAnalysis)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/analyses", s.listAnalyses)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/analyses/{analysis_id}", s.getAnalysis)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/scenes", s.listScenes)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/scenes", s.createScene)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/scenes/{scene_id}", s.getScene)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/candidate-groups", s.createCandidateGroup)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/candidate-groups/{group_id}", s.getCandidateGroup)
	mux.HandleFunc("GET /api/v1/provider-configurations", s.listProviders)
	mux.HandleFunc("POST /api/v1/provider-configurations", s.createProvider)
	mux.HandleFunc("GET /api/v1/provider-configurations/{provider_id}", s.getProvider)
	mux.HandleFunc("PATCH /api/v1/provider-configurations/{provider_id}", s.patchProvider)
	mux.HandleFunc("POST /api/v1/provider-configurations/{provider_id}/validate", s.validateProvider)
	mux.HandleFunc("POST /api/v1/provider-configurations/{provider_id}/disable", s.disableProvider)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	var input struct {
		Name        string         `json:"name"`
		WorkflowKey string         `json:"workflow_key"`
		Settings    map[string]any `json:"settings"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	value, err := s.Product.CreateProject(principal.WorkspaceID, input.Name, input.WorkflowKey, input.Settings)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeRevisionJSON(w, r, http.StatusCreated, value, value.Revision)
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	limit := queryLimit(r)
	values, err := s.Product.ListProjects(principal.WorkspaceID, limit)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"items": values, "next_cursor": nil})
}
func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.GetProject(principal.WorkspaceID, r.PathValue("project_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeRevisionJSON(w, r, http.StatusOK, value, value.Revision)
}
func (s *Server) patchProject(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	revision, valid := ifMatch(r)
	if !valid {
		s.writeError(w, r, http.StatusPreconditionFailed, "ETAG_REQUIRED", "If-Match is required.")
		return
	}
	var input struct {
		Name     *string        `json:"name"`
		Status   *string        `json:"status"`
		Settings map[string]any `json:"settings"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	value, err := s.Product.UpdateProject(principal.WorkspaceID, r.PathValue("project_id"), revision, input.Name, input.Status, input.Settings)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeRevisionJSON(w, r, http.StatusOK, value, value.Revision)
}
func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	revision, valid := ifMatch(r)
	if !valid {
		revision = 1
	}
	value, err := s.Product.UpdateProject(principal.WorkspaceID, r.PathValue("project_id"), revision, nil, stringPtr("deleted"), nil)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeRevisionJSON(w, r, http.StatusAccepted, value, value.Revision)
}

func (s *Server) createUpload(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	var input struct {
		Kind        string `json:"kind"`
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		SizeBytes   int64  `json:"size_bytes"`
		SHA256      string `json:"sha256"`
		Multipart   bool   `json:"multipart"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	if input.SizeBytes < 0 {
		s.writeError(w, r, http.StatusRequestEntityTooLarge, "UPLOAD_SIZE_INVALID", "Declared upload size is invalid.")
		return
	}
	asset, err := s.Product.CreateAsset(principal.WorkspaceID, r.PathValue("project_id"), input.Kind, input.Filename, input.ContentType, input.SizeBytes, input.SHA256)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	parts := []product.UploadPart{{PartNumber: 1, Method: "PUT", URL: "nh-media-direct://" + asset.ID + "/1", Headers: map[string]string{"Content-Type": input.ContentType}}}
	upload, err := s.Product.CreateUpload(r.Context(), principal.WorkspaceID, r.PathValue("project_id"), asset.ID, input.SizeBytes, input.SHA256, input.ContentType, parts, "")
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, map[string]any{"asset": asset, "upload": upload})
}
func (s *Server) listAssets(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	values, err := s.Product.ListAssets(principal.WorkspaceID, r.PathValue("project_id"), queryLimit(r))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"items": values, "next_cursor": nil})
}
func (s *Server) getAsset(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.GetAsset(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("asset_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeRevisionJSON(w, r, http.StatusOK, value, value.Revision)
}
func (s *Server) patchAsset(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	revision, valid := ifMatch(r)
	if !valid {
		s.writeError(w, r, http.StatusPreconditionFailed, "ETAG_REQUIRED", "If-Match is required.")
		return
	}
	var input struct {
		Metadata map[string]any `json:"metadata"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	value, err := s.Product.UpdateAsset(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("asset_id"), revision, input.Metadata)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeRevisionJSON(w, r, http.StatusOK, value, value.Revision)
}
func (s *Server) deleteAsset(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.SetAssetStatus(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("asset_id"), "deleted", "")
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeRevisionJSON(w, r, http.StatusAccepted, value, value.Revision)
}

func (s *Server) uploadParts(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	var input struct {
		PartNumbers []int `json:"part_numbers"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	upload, err := s.Product.PresignUploadParts(r.Context(), principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("asset_id"), r.PathValue("upload_id"), input.PartNumbers)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, upload)
}
func (s *Server) completeUpload(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	var input struct {
		Parts []struct {
			PartNumber int    `json:"part_number"`
			ETag       string `json:"etag"`
		} `json:"parts"`
		SHA256 string `json:"sha256"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	if len(input.Parts) == 0 {
		s.writeError(w, r, http.StatusUnprocessableEntity, "UPLOAD_PARTS_REQUIRED", "At least one uploaded part is required.")
		return
	}
	parts := make([]product.UploadPart, 0, len(input.Parts))
	for _, part := range input.Parts {
		parts = append(parts, product.UploadPart{PartNumber: part.PartNumber, ETag: part.ETag})
	}
	upload, job, err := s.Product.CompleteUpload(r.Context(), principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("asset_id"), r.PathValue("upload_id"), input.SHA256, parts)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	asset, err := s.Product.GetAsset(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("asset_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	response := map[string]any{"asset": asset, "upload": upload, "validation_job_id": nil}
	if job != nil {
		response["validation_job_id"] = job.ID
	}
	s.writeJSON(w, r, http.StatusAccepted, response)
}
func (s *Server) abortUpload(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	if err := s.Product.AbortUpload(r.Context(), principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("asset_id"), r.PathValue("upload_id")); err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusAccepted, map[string]string{"status": "aborted"})
}

func (s *Server) createScript(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	var input struct {
		Language string         `json:"language"`
		Origin   string         `json:"origin"`
		Content  map[string]any `json:"content"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	if input.Language == "" {
		input.Language = "vi"
	}
	if input.Origin == "" {
		input.Origin = "user"
	}
	script, version, err := s.Product.CreateScript(principal.WorkspaceID, r.PathValue("project_id"), input.Language, input.Origin, input.Content)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, map[string]any{"script": script, "version": version})
}
func (s *Server) getScript(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.GetScript(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("script_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeRevisionJSON(w, r, http.StatusOK, value, value.Revision)
}
func (s *Server) createScriptVersion(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	revision, valid := ifMatch(r)
	if !valid {
		s.writeError(w, r, http.StatusPreconditionFailed, "ETAG_REQUIRED", "If-Match is required.")
		return
	}
	var input struct {
		BasedOnVersionID string         `json:"based_on_version_id"`
		Language         string         `json:"language"`
		Origin           string         `json:"origin"`
		Content          map[string]any `json:"content"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	value, err := s.Product.CreateScriptVersion(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("script_id"), input.BasedOnVersionID, input.Language, input.Origin, input.Content, revision)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, value)
}
func (s *Server) getScriptVersion(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.GetScriptVersion(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("version_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, value)
}
func (s *Server) approveScriptVersion(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok || !domain.CanEdit(domain.Role(principal.Role)) {
		if ok {
			s.writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Editor permission is required.")
		}
		return
	}
	value, err := s.Product.SetScriptVersionStatus(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("script_id"), r.PathValue("version_id"), "approved")
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, value)
}
func (s *Server) rejectScriptVersion(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.SetScriptVersionStatus(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("script_id"), r.PathValue("version_id"), "superseded")
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, value)
}

func (s *Server) createNarration(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	var input struct {
		ScriptVersionID  string         `json:"script_version_id"`
		ProviderConfigID string         `json:"provider_configuration_id"`
		VoiceSnapshot    map[string]any `json:"voice_snapshot"`
		RequestSnapshot  map[string]any `json:"request_snapshot"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	value, err := s.Product.CreateNarration(principal.WorkspaceID, r.PathValue("project_id"), input.ScriptVersionID, input.ProviderConfigID, input.VoiceSnapshot, input.RequestSnapshot)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusAccepted, value)
}

func (s *Server) getNarration(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.GetNarration(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("narration_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, value)
}

func (s *Server) createTimeline(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	var input struct {
		Origin   string          `json:"origin"`
		Document json.RawMessage `json:"document"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	if len(input.Document) == 0 {
		s.writeError(w, r, http.StatusUnprocessableEntity, "TIMELINE_INVALID", "Timeline document is required.")
		return
	}
	timeline, version, err := s.Product.CreateTimeline(principal.WorkspaceID, r.PathValue("project_id"), input.Origin, input.Document)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, map[string]any{"timeline": timeline, "version": version})
}
func (s *Server) getTimeline(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.GetTimeline(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("timeline_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeRevisionJSON(w, r, http.StatusOK, value, value.Revision)
}
func (s *Server) getTimelineVersion(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.GetTimelineVersion(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("version_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, value)
}
func (s *Server) validateTimeline(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.GetTimelineVersion(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("version_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	hash, validationErr := s.Product.ValidateTimeline(principal.WorkspaceID, r.PathValue("project_id"), value.ID)
	report := map[string]any{"valid": validationErr == nil, "content_hash": hash}
	if validationErr != nil {
		report["error"] = "timeline validation failed"
	}
	s.writeJSON(w, r, http.StatusOK, report)
}
func (s *Server) timelineCommand(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	var command domain.TimelineCommand
	if !s.decodeResource(w, r, &command) {
		return
	}
	timeline, err := s.Product.GetTimeline(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("timeline_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	base, err := s.Product.GetTimelineVersion(principal.WorkspaceID, r.PathValue("project_id"), command.BasedOnVersionID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	document, hash, err := domain.ApplyTimelineCommand(base.Document, command)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = hash
	value, err := s.Product.AddTimelineVersion(principal.WorkspaceID, r.PathValue("project_id"), timeline.ID, base.ID, "user", document, timeline.Revision)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, value)
}
func (s *Server) approveTimeline(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.SetTimelineVersionStatus(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("timeline_id"), r.PathValue("version_id"), "approved")
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, value)
}
func (s *Server) lockTimeline(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	if !domain.CanAdmin(domain.Role(principal.Role)) {
		s.writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Admin permission is required.")
		return
	}
	value, err := s.Product.SetTimelineVersionStatus(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("timeline_id"), r.PathValue("version_id"), "locked")
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, value)
}
func (s *Server) createRender(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	var input struct {
		TimelineVersionID string `json:"timeline_version_id"`
		RenderProfile     struct {
			ProfileKey string `json:"profile_key"`
			Version    int    `json:"version"`
		} `json:"render_profile"`
		Preview   bool           `json:"preview"`
		Overrides map[string]any `json:"overrides"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	if input.Preview { /* preview remains explicit but the initial in-memory path still uses the same reference */
	}
	value, err := s.Product.CreateRender(principal.WorkspaceID, r.PathValue("project_id"), input.TimelineVersionID, input.RenderProfile.ProfileKey, input.RenderProfile.Version, map[string]any{"timeline_version_id": input.TimelineVersionID, "render_profile": input.RenderProfile, "overrides": input.Overrides})
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusAccepted, value)
}

func (s *Server) createAnalysis(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	var input struct {
		Kind       string         `json:"kind"`
		InputRefs  map[string]any `json:"input_refs"`
		Provenance map[string]any `json:"provenance"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	value, err := s.Product.CreateAnalysis(principal.WorkspaceID, r.PathValue("project_id"), input.Kind, input.InputRefs, input.Provenance)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusAccepted, value)
}
func (s *Server) getAnalysis(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.GetAnalysis(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("analysis_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, value)
}
func (s *Server) listAnalyses(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	values, err := s.Product.ListAnalyses(principal.WorkspaceID, r.PathValue("project_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"items": values, "next_cursor": nil})
}
func (s *Server) createScene(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	var input struct {
		AssetID     string  `json:"asset_id"`
		ArtifactID  string  `json:"artifact_id"`
		StartSec    float64 `json:"source_start_sec"`
		EndSec      float64 `json:"source_end_sec"`
		Description string  `json:"description"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	value, err := s.Product.CreateScene(principal.WorkspaceID, r.PathValue("project_id"), input.AssetID, input.ArtifactID, input.StartSec, input.EndSec, input.Description)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, value)
}
func (s *Server) listScenes(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	values, err := s.Product.ListScenes(principal.WorkspaceID, r.PathValue("project_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"items": values, "next_cursor": nil})
}
func (s *Server) getScene(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.GetScene(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("scene_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeRevisionJSON(w, r, http.StatusOK, value, value.Revision)
}
func (s *Server) createCandidateGroup(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	var input struct {
		Candidates []product.Candidate `json:"candidates"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	value, err := s.Product.CreateCandidateGroup(principal.WorkspaceID, r.PathValue("project_id"), input.Candidates)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusAccepted, value)
}
func (s *Server) getCandidateGroup(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.GetCandidateGroup(principal.WorkspaceID, r.PathValue("project_id"), r.PathValue("group_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, value)
}

func (s *Server) listProviders(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	values, err := s.Product.ListProviders(principal.WorkspaceID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"items": values, "next_cursor": nil})
}
func (s *Server) createProvider(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok || !domain.CanAdmin(domain.Role(principal.Role)) {
		if ok {
			s.writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Admin permission is required.")
		}
		return
	}
	var input struct {
		Kind          string          `json:"kind"`
		AdapterKey    string          `json:"adapter_key"`
		DisplayName   string          `json:"display_name"`
		CredentialRef string          `json:"credential_ref"`
		Secret        json.RawMessage `json:"secret"`
		Config        map[string]any  `json:"config"`
		ModelDefaults map[string]any  `json:"model_defaults"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	if len(input.Secret) > 0 && string(input.Secret) != "null" {
		s.writeError(w, r, http.StatusUnprocessableEntity, "PLAINTEXT_SECRET_NOT_ALLOWED", "Use the server-side SecretStore operation; plaintext secrets are not accepted.")
		return
	}
	value, err := s.Product.CreateProvider(principal.WorkspaceID, input.Kind, input.AdapterKey, input.DisplayName, input.Config, input.ModelDefaults, input.CredentialRef)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeRevisionJSON(w, r, http.StatusCreated, value, value.Revision)
}
func (s *Server) getProvider(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	value, err := s.Product.GetProvider(principal.WorkspaceID, r.PathValue("provider_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeRevisionJSON(w, r, http.StatusOK, value, value.Revision)
}
func (s *Server) patchProvider(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok || !domain.CanAdmin(domain.Role(principal.Role)) {
		if ok {
			s.writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Admin permission is required.")
		}
		return
	}
	revision, valid := ifMatch(r)
	if !valid {
		s.writeError(w, r, http.StatusPreconditionFailed, "ETAG_REQUIRED", "If-Match is required.")
		return
	}
	var input struct {
		Status        string          `json:"status"`
		AdapterKey    string          `json:"adapter_key"`
		DisplayName   string          `json:"display_name"`
		CredentialRef *string         `json:"credential_ref"`
		Secret        json.RawMessage `json:"secret"`
		Config        map[string]any  `json:"config"`
		ModelDefaults map[string]any  `json:"model_defaults"`
	}
	if !s.decodeResource(w, r, &input) {
		return
	}
	if len(input.Secret) > 0 && string(input.Secret) != "null" {
		s.writeError(w, r, http.StatusUnprocessableEntity, "PLAINTEXT_SECRET_NOT_ALLOWED", "Plaintext secrets are not accepted.")
		return
	}
	value, err := s.Product.UpdateProvider(principal.WorkspaceID, r.PathValue("provider_id"), revision, input.Status, input.AdapterKey, input.DisplayName, input.Config, input.ModelDefaults, input.CredentialRef)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeRevisionJSON(w, r, http.StatusOK, value, value.Revision)
}
func (s *Server) validateProvider(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok || !domain.CanAdmin(domain.Role(principal.Role)) {
		if ok {
			s.writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Admin permission is required.")
		}
		return
	}
	value, err := s.Product.GetProvider(principal.WorkspaceID, r.PathValue("provider_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusAccepted, map[string]any{"provider_configuration_id": value.ID, "status": "validation_queued"})
}
func (s *Server) disableProvider(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok || !domain.CanAdmin(domain.Role(principal.Role)) {
		if ok {
			s.writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Admin permission is required.")
		}
		return
	}
	current, err := s.Product.GetProvider(principal.WorkspaceID, r.PathValue("provider_id"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	value, err := s.Product.UpdateProvider(principal.WorkspaceID, r.PathValue("provider_id"), current.Revision, "disabled", "", "", nil, nil, nil)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeRevisionJSON(w, r, http.StatusOK, value, value.Revision)
}

func (s *Server) decodeResource(w http.ResponseWriter, r *http.Request, destination any) bool {
	if err := s.decodeJSON(w, r, destination); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "The request could not be understood.")
		return false
	}
	return true
}
func (s *Server) storeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, product.ErrIdempotencyConflict):
		s.writeError(w, r, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "The Idempotency-Key was already used with a different request.")
	case errors.Is(err, product.ErrNotFound):
		s.writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Resource not found.")
	case errors.Is(err, product.ErrConflict):
		s.writeError(w, r, http.StatusPreconditionFailed, "VERSION_CONFLICT", "The resource changed; refresh and retry.")
	default:
		s.writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "The request violates a product invariant.")
	}
}
func (s *Server) writeRevisionJSON(w http.ResponseWriter, r *http.Request, status int, value any, revision int64) {
	w.Header().Set("ETag", fmt.Sprintf("\"%d\"", revision))
	s.writeJSON(w, r, status, value)
}
func ifMatch(r *http.Request) (int64, bool) {
	value := strings.Trim(strings.TrimSpace(r.Header.Get("If-Match")), "\"")
	if value == "" {
		return 0, false
	}
	revision, err := strconv.ParseInt(value, 10, 64)
	return revision, err == nil && revision > 0
}
func queryLimit(r *http.Request) int {
	value, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if value <= 0 {
		value = 50
	}
	if value > 100 {
		value = 100
	}
	return value
}
func stringPtr(value string) *string { return &value }
