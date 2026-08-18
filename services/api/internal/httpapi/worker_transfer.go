package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

type WorkerArtifactTransfer interface {
	ResolveWorkerArtifact(context.Context, string, string, string) (storage.SignedRequest, error)
	PrepareWorkerStage(context.Context, string, string, string, string, string, string, string) (storage.SignedRequest, error)
}

func (s *Server) requireWorker(w http.ResponseWriter, r *http.Request) bool {
	provided := strings.TrimPrefix(strings.TrimSpace(r.Header.Get("Authorization")), "Bearer ")
	if s.WorkerArtifacts == nil || len(s.WorkerToken) < 24 || len(provided) != len(s.WorkerToken) || subtle.ConstantTimeCompare([]byte(provided), []byte(s.WorkerToken)) != 1 {
		s.writeError(w, r, http.StatusUnauthorized, "worker_authentication_required", "Worker authentication is required.")
		return false
	}
	return true
}

func (s *Server) resolveWorkerArtifact(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorker(w, r) {
		return
	}
	var input struct {
		ProjectID  string `json:"project_id"`
		ArtifactID string `json:"artifact_id"`
		SHA256     string `json:"sha256"`
	}
	if err := s.decodeJSON(w, r, &input); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "invalid_worker_transfer", "Worker Artifact request is invalid.")
		return
	}
	request, err := s.WorkerArtifacts.ResolveWorkerArtifact(r.Context(), input.ProjectID, input.ArtifactID, input.SHA256)
	if err != nil {
		workerTransferError(s, w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, request)
}

func (s *Server) stageWorkerArtifact(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorker(w, r) {
		return
	}
	var input struct {
		WorkspaceID string `json:"workspace_id"`
		ProjectID   string `json:"project_id"`
		JobID       string `json:"job_id"`
		JobStepID   string `json:"job_step_id"`
		MessageID   string `json:"message_id"`
		ArtifactID  string `json:"artifact_id"`
		ContentType string `json:"content_type"`
	}
	if err := s.decodeJSON(w, r, &input); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "invalid_worker_transfer", "Worker stage request is invalid.")
		return
	}
	request, err := s.WorkerArtifacts.PrepareWorkerStage(r.Context(), input.WorkspaceID, input.ProjectID, input.JobID, input.JobStepID, input.MessageID, input.ArtifactID, input.ContentType)
	if err != nil {
		workerTransferError(s, w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, request)
}

func workerTransferError(s *Server, w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, context.Canceled) {
		s.writeError(w, r, http.StatusRequestTimeout, "worker_transfer_cancelled", "Worker transfer was cancelled.")
		return
	}
	s.writeError(w, r, http.StatusNotFound, "worker_artifact_unavailable", "Worker Artifact is unavailable.")
}
