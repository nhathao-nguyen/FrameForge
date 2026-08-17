package httpapi

import (
	"net/http"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/auth"
)

func requirePrincipal(s *Server, w http.ResponseWriter, r *http.Request) (auth.Principal, bool) {
	principal, err := s.Auth.Authenticate(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		s.writeError(w, r, http.StatusUnauthorized, "authentication_required", "Authentication is required.")
		return auth.Principal{}, false
	}
	if principal.WorkspaceID == "" {
		principal.WorkspaceID = s.Product.WorkspaceID()
	}
	return principal, true
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	token, principal, err := s.LocalAuth.Refresh(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		s.writeError(w, r, http.StatusUnauthorized, "authentication_failed", "Authentication failed.")
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"access_token": token, "token_type": "Bearer", "subject": principal.Subject, "workspace_id": principal.WorkspaceID})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if _, ok := requirePrincipal(s, w, r); !ok {
		return
	}
	_ = s.LocalAuth.Logout(r.Context(), r.Header.Get("Authorization"))
	s.writeJSON(w, r, http.StatusNoContent, nil)
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"user": map[string]string{"id": principal.UserID, "subject": principal.Subject, "status": "active"}, "memberships": []map[string]string{{"workspace_id": principal.WorkspaceID, "role": principal.Role}}, "expires_at": nil})
}

func (s *Server) workspaces(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"items": []map[string]string{{"id": principal.WorkspaceID, "slug": "default", "name": "Default Workspace", "status": "active", "role": principal.Role}}, "next_cursor": nil})
}

func (s *Server) workspace(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(s, w, r)
	if !ok {
		return
	}
	if r.PathValue("workspace_id") != principal.WorkspaceID {
		s.writeError(w, r, http.StatusNotFound, "not_found", "Resource not found.")
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]string{"id": principal.WorkspaceID, "slug": "default", "name": "Default Workspace", "status": "active", "role": principal.Role})
}
