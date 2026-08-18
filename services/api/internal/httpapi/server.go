package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/auth"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/config"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/health"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/product"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

type contextKey string

const requestIDKey contextKey = "request_id"
const correlationIDKey contextKey = "correlation_id"

type Server struct {
	Config          config.APIConfig
	Auth            auth.AuthPort
	LocalAuth       *auth.LocalAuthProvider
	Health          *health.Registry
	HTTPServer      *http.Server
	Product         product.Backend
	Storage         storage.StoragePort
	WorkerArtifacts WorkerArtifactTransfer
	WorkerToken     string
}

func NewServer(value config.APIConfig, provider *auth.LocalAuthProvider, registry *health.Registry) (*Server, error) {
	return NewServerWithStorage(value, provider, registry, nil)
}

func NewServerWithStorage(value config.APIConfig, provider *auth.LocalAuthProvider, registry *health.Registry, backend storage.StoragePort) (*Server, error) {
	return NewServerWithBackend(value, provider, registry, product.NewStoreWithStorage(providerWorkspace(provider), backend), backend)
}

func NewServerWithBackend(value config.APIConfig, provider *auth.LocalAuthProvider, registry *health.Registry, productBackend product.Backend, backend storage.StoragePort) (*Server, error) {
	if err := value.Validate(); err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, errors.New("AuthPort is required")
	}
	if registry == nil {
		registry = health.NewRegistry(nil, 2*time.Second)
	}
	if productBackend == nil {
		return nil, errors.New("Product backend is required")
	}
	server := &Server{Config: value, Auth: provider, LocalAuth: provider, Health: registry, Product: productBackend, Storage: backend}
	server.HTTPServer = &http.Server{Addr: value.Bind, Handler: server.Handler(), ReadHeaderTimeout: value.ReadHeaderTimeout, ReadTimeout: value.ReadTimeout, WriteTimeout: value.WriteTimeout, IdleTimeout: value.IdleTimeout, MaxHeaderBytes: value.MaxHeaderBytes}
	return server, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1", s.version)
	mux.Handle("GET /api/v1/live", s.Health.LiveHandler())
	mux.Handle("GET /api/v1/ready", s.Health.ReadyHandler())
	mux.HandleFunc("POST /api/v1/auth/local/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", s.refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.HandleFunc("GET /api/v1/auth/session", s.session)
	mux.HandleFunc("GET /api/v1/workspaces", s.workspaces)
	mux.HandleFunc("GET /api/v1/workspaces/{workspace_id}", s.workspace)
	mux.HandleFunc("GET /api/v1/diagnostics", s.diagnostics)
	mux.HandleFunc("POST /internal/v1/worker/artifacts/resolve", s.resolveWorkerArtifact)
	mux.HandleFunc("POST /internal/v1/worker/artifacts/stage", s.stageWorkerArtifact)
	s.registerProjectRoutes(mux)
	return s.middleware(mux)
}

func (s *Server) ConfigureWorkerArtifacts(transfer WorkerArtifactTransfer, token string) {
	s.WorkerArtifacts = transfer
	s.WorkerToken = strings.TrimSpace(token)
}

func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, r, http.StatusOK, map[string]string{"service": "nh-media", "api_version": "v1", "serialization_version": "1"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := s.decodeJSON(w, r, &input); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "invalid_request", "The request could not be understood.")
		return
	}
	token, principal, err := s.LocalAuth.Login(r.Context(), input.Username, input.Password)
	if err != nil {
		s.writeError(w, r, http.StatusUnauthorized, "authentication_failed", "Authentication failed.")
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]string{"access_token": token, "token_type": "Bearer", "subject": principal.Subject})
}

func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request) {
	if _, err := s.Auth.Authenticate(r.Context(), r.Header.Get("Authorization")); err != nil {
		s.writeError(w, r, http.StatusUnauthorized, "authentication_required", "Authentication is required.")
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"status": "ok", "profile": s.Config.Profile, "checks": []string{"configuration", "health"}})
}

func (s *Server) middleware(next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(s.Config.AllowedOrigins))
	for _, origin := range s.Config.AllowedOrigins {
		allowed[origin] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := safeID(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = newID("req")
		}
		correlationID := safeID(r.Header.Get("X-Correlation-ID"))
		if correlationID == "" {
			correlationID = requestID
		}
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Correlation-ID", correlationID)
		origin := r.Header.Get("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; !ok {
				s.writeErrorWithIDs(w, requestID, correlationID, http.StatusForbidden, "cors_origin_denied", "Origin is not allowed.")
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, If-Match, Idempotency-Key, X-Request-ID, X-Correlation-ID")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.ContentLength > s.Config.MaxBodyBytes {
			s.writeErrorWithIDs(w, requestID, correlationID, http.StatusRequestEntityTooLarge, "request_too_large", "Request body exceeds the configured limit.")
			return
		}
		ctx := context.WithValue(context.WithValue(r.Context(), requestIDKey, requestID), correlationIDKey, correlationID)
		r = r.WithContext(ctx)
		r.Body = http.MaxBytesReader(w, r.Body, s.Config.MaxBodyBytes)
		idempotency, hasIdempotency := s.Product.(httpIdempotencyBackend)
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		shouldIdempotency := hasIdempotency && key != "" && r.Method != http.MethodGet && r.Method != http.MethodOptions && !strings.HasPrefix(r.URL.Path, "/api/v1/auth/")
		if !shouldIdempotency {
			next.ServeHTTP(w, r)
			return
		}
		if len(key) < 8 || len(key) > 200 || safeID(key) == "" {
			s.writeErrorWithIDs(w, requestID, correlationID, http.StatusBadRequest, "IDEMPOTENCY_KEY_INVALID", "Idempotency-Key is invalid.")
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || int64(len(body)) > s.Config.MaxBodyBytes {
			s.writeErrorWithIDs(w, requestID, correlationID, http.StatusRequestEntityTooLarge, "request_too_large", "Request body exceeds the configured limit.")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		principal, authErr := s.Auth.Authenticate(r.Context(), r.Header.Get("Authorization"))
		if authErr != nil || principal.WorkspaceID == "" {
			// Let the normal route produce the canonical 401 response. No
			// idempotency record is claimed for an unauthenticated request.
			next.ServeHTTP(w, r)
			return
		}
		requestFingerprint := string(body)
		replay, replayStatus, exists, claimErr := idempotency.BeginHTTPIdempotency(r.Context(), principal.WorkspaceID, key, requestFingerprint)
		if claimErr != nil {
			s.storeError(w, r, claimErr)
			return
		}
		if exists {
			if bytes.Equal(bytes.TrimSpace(replay), []byte(`{"_pending":true}`)) {
				s.writeError(w, r, http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS", "The idempotent request is already in progress.")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if replayStatus <= 0 {
				replayStatus = http.StatusOK
			}
			w.WriteHeader(replayStatus)
			_, _ = w.Write(replay)
			return
		}
		buffer := newBufferedResponseWriter()
		next.ServeHTTP(buffer, r)
		for header, values := range buffer.header {
			for _, value := range values {
				w.Header().Add(header, value)
			}
		}
		w.WriteHeader(buffer.status)
		_, _ = w.Write(buffer.body.Bytes())
		if finalizeErr := idempotency.FinalizeHTTPIdempotency(r.Context(), principal.WorkspaceID, key, requestFingerprint, buffer.status, buffer.body.Bytes()); finalizeErr != nil {
			// The response has already been delivered. Log-free failure is
			// intentional: the next retry will fail closed rather than mutate
			// a second time, and reconciliation can inspect the key.
			return
		}
	})
}

type httpIdempotencyBackend interface {
	BeginHTTPIdempotency(context.Context, string, string, string) (json.RawMessage, int, bool, error)
	FinalizeHTTPIdempotency(context.Context, string, string, string, int, json.RawMessage) error
}

type bufferedResponseWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newBufferedResponseWriter() *bufferedResponseWriter {
	return &bufferedResponseWriter{header: make(http.Header), status: http.StatusOK}
}

func (w *bufferedResponseWriter) Header() http.Header { return w.header }
func (w *bufferedResponseWriter) WriteHeader(status int) {
	if w.status != http.StatusOK {
		return
	}
	w.status = status
}
func (w *bufferedResponseWriter) Write(value []byte) (int, error) {
	if w.status == http.StatusOK {
		w.status = http.StatusOK
	}
	return w.body.Write(value)
}

func providerWorkspace(provider *auth.LocalAuthProvider) string {
	if provider == nil {
		return "ws_default"
	}
	principal := provider.PrincipalDefaults()
	if principal.WorkspaceID == "" {
		return "ws_default"
	}
	return principal.WorkspaceID
}

func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, s.Config.MaxBodyBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}

func (s *Server) writeJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	requestID, correlationID := IDs(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
	_ = requestID
	_ = correlationID
}
func (s *Server) writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	requestID, correlationID := IDs(r.Context())
	s.writeErrorWithIDs(w, requestID, correlationID, status, code, message)
}
func (s *Server) writeErrorWithIDs(w http.ResponseWriter, requestID, correlationID string, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"serialization_version": "1", "error": map[string]string{"code": code, "message": message, "request_id": requestID, "correlation_id": correlationID}})
}
func IDs(ctx context.Context) (string, string) {
	requestID, _ := ctx.Value(requestIDKey).(string)
	correlationID, _ := ctx.Value(correlationIDKey).(string)
	return requestID, correlationID
}
func safeID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 3 || len(value) > 128 || strings.ContainsAny(value, "\r\n\\/") {
		return ""
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' || character == '-' || character == '.') {
			return ""
		}
	}
	return value
}
func newID(prefix string) string {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%s_fallback", prefix)
	}
	return prefix + "_" + hex.EncodeToString(raw)
}
