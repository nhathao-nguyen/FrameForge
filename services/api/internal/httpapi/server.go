package httpapi

import (
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
)

type contextKey string

const requestIDKey contextKey = "request_id"
const correlationIDKey contextKey = "correlation_id"

type Server struct {
	Config     config.APIConfig
	Auth       auth.AuthPort
	LocalAuth  *auth.LocalAuthProvider
	Health     *health.Registry
	HTTPServer *http.Server
}

func NewServer(value config.APIConfig, provider *auth.LocalAuthProvider, registry *health.Registry) (*Server, error) {
	if err := value.Validate(); err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, errors.New("AuthPort is required")
	}
	if registry == nil {
		registry = health.NewRegistry(nil, 2*time.Second)
	}
	server := &Server{Config: value, Auth: provider, LocalAuth: provider, Health: registry}
	server.HTTPServer = &http.Server{Addr: value.Bind, Handler: server.Handler(), ReadHeaderTimeout: value.ReadHeaderTimeout, ReadTimeout: value.ReadTimeout, WriteTimeout: value.WriteTimeout, IdleTimeout: value.IdleTimeout, MaxHeaderBytes: value.MaxHeaderBytes}
	return server, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1", s.version)
	mux.Handle("GET /api/v1/live", s.Health.LiveHandler())
	mux.Handle("GET /api/v1/ready", s.Health.ReadyHandler())
	mux.HandleFunc("POST /api/v1/auth/local/login", s.login)
	mux.HandleFunc("GET /api/v1/diagnostics", s.diagnostics)
	return s.middleware(mux)
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
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
		next.ServeHTTP(w, r)
	})
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
