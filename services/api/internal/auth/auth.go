package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

type Principal struct {
	Subject     string
	UserID      string
	WorkspaceID string
	Role        string
	Scopes      []string
}

type AuthPort interface {
	Authenticate(ctx context.Context, bearerToken string) (Principal, error)
}

var ErrSessionNotFound = errors.New("session not found")

type SessionStore interface {
	CreateSession(context.Context, string, string, string, time.Time) error
	GetSession(context.Context, string) (Principal, time.Time, error)
	RevokeSession(context.Context, string) error
}

type LocalAuthProvider struct {
	usernameHash [32]byte
	passwordHash [32]byte
	principal    Principal
	validate     func(context.Context, Principal) (Principal, error)
	sessionStore SessionStore
	sessions     map[[32]byte]session
	mu           sync.RWMutex
	sessionTTL   time.Duration
}

type session struct {
	principal Principal
	expiresAt time.Time
	revokedAt *time.Time
}

func NewLocalAuthProvider(username, password string) (*LocalAuthProvider, error) {
	if strings.TrimSpace(username) == "" || password == "" {
		return nil, errors.New("local auth bootstrap credentials are required")
	}
	return &LocalAuthProvider{usernameHash: sha256.Sum256([]byte(username)), passwordHash: sha256.Sum256([]byte(password)), principal: Principal{Subject: "local-admin", UserID: "user_local_admin", WorkspaceID: "ws_default", Role: "owner", Scopes: []string{"*"}}, sessions: make(map[[32]byte]session), sessionTTL: 8 * time.Hour}, nil
}

func (p *LocalAuthProvider) Login(ctx context.Context, username, password string) (string, Principal, error) {
	if err := ctx.Err(); err != nil {
		return "", Principal{}, err
	}
	usernameHash := sha256.Sum256([]byte(username))
	passwordHash := sha256.Sum256([]byte(password))
	if subtle.ConstantTimeCompare(usernameHash[:], p.usernameHash[:]) != 1 || subtle.ConstantTimeCompare(passwordHash[:], p.passwordHash[:]) != 1 {
		return "", Principal{}, errors.New("invalid credentials")
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", Principal{}, errors.New("session issuance failed")
	}
	token := "nhs_" + base64.RawURLEncoding.EncodeToString(tokenBytes)
	tokenHash := sha256.Sum256([]byte(token))
	p.mu.RLock()
	principal := p.principal
	validate := p.validate
	sessionStore := p.sessionStore
	p.mu.RUnlock()
	if validate != nil {
		validated, err := validate(ctx, principal)
		if err != nil {
			return "", Principal{}, errors.New("principal is no longer authorized")
		}
		principal = validated
	}
	expiresAt := time.Now().UTC().Add(p.sessionTTL)
	if sessionStore != nil {
		if err := sessionStore.CreateSession(ctx, principal.UserID, hex.EncodeToString(tokenHash[:]), "api", expiresAt); err != nil {
			return "", Principal{}, errors.New("session persistence failed")
		}
	}
	p.mu.Lock()
	p.sessions[tokenHash] = session{principal: principal, expiresAt: expiresAt}
	p.mu.Unlock()
	return token, principal, nil
}

// ConfigurePrincipal binds the LocalAuth shell to the durable bootstrap IDs.
// It is called during API startup after the transactional DB bootstrap and
// never accepts values from an HTTP request.
func (p *LocalAuthProvider) ConfigurePrincipal(subject, userID, workspaceID, role string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(role) == "" {
		return errors.New("durable principal identity is incomplete")
	}
	p.mu.Lock()
	p.principal = Principal{Subject: firstNonEmpty(subject, "local-admin"), UserID: userID, WorkspaceID: workspaceID, Role: role, Scopes: []string{"*"}}
	p.mu.Unlock()
	return nil
}

func (p *LocalAuthProvider) PrincipalDefaults() Principal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := p.principal
	result.Scopes = append([]string(nil), result.Scopes...)
	return result
}

// SetPrincipalValidator lets the durable installation re-check user and
// Workspace status on every request. The callback is server-owned; no request
// payload can replace the authenticated identity or scope.
func (p *LocalAuthProvider) SetPrincipalValidator(validate func(context.Context, Principal) (Principal, error)) {
	p.mu.Lock()
	p.validate = validate
	p.mu.Unlock()
}

func (p *LocalAuthProvider) SetSessionStore(store SessionStore) {
	p.mu.Lock()
	p.sessionStore = store
	p.mu.Unlock()
}

func CredentialHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (p *LocalAuthProvider) Authenticate(ctx context.Context, bearerToken string) (Principal, error) {
	if err := ctx.Err(); err != nil {
		return Principal{}, err
	}
	token := strings.TrimSpace(strings.TrimPrefix(bearerToken, "Bearer "))
	if token == "" {
		return Principal{}, errors.New("missing bearer token")
	}
	hash := sha256.Sum256([]byte(token))
	p.mu.RLock()
	sessionStore := p.sessionStore
	p.mu.RUnlock()
	if sessionStore != nil {
		principal, expiresAt, err := sessionStore.GetSession(ctx, hex.EncodeToString(hash[:]))
		if err != nil || time.Now().UTC().After(expiresAt) {
			return Principal{}, ErrSessionNotFound
		}
		p.mu.RLock()
		validate := p.validate
		p.mu.RUnlock()
		if validate != nil {
			principal, err = validate(ctx, principal)
			if err != nil {
				return Principal{}, errors.New("principal is no longer authorized")
			}
		}
		return principal, nil
	}
	p.mu.RLock()
	value, ok := p.sessions[hash]
	p.mu.RUnlock()
	if !ok || value.revokedAt != nil || time.Now().UTC().After(value.expiresAt) {
		return Principal{}, errors.New("invalid session")
	}
	value.principal.Scopes = append([]string(nil), value.principal.Scopes...)
	p.mu.RLock()
	validate := p.validate
	p.mu.RUnlock()
	if validate != nil {
		validated, err := validate(ctx, value.principal)
		if err != nil {
			return Principal{}, errors.New("principal is no longer authorized")
		}
		value.principal = validated
	}
	return value.principal, nil
}

func (p *LocalAuthProvider) Logout(ctx context.Context, bearerToken string) error {
	token := strings.TrimSpace(strings.TrimPrefix(bearerToken, "Bearer "))
	if token == "" {
		return nil
	}
	hash := sha256.Sum256([]byte(token))
	p.mu.RLock()
	sessionStore := p.sessionStore
	p.mu.RUnlock()
	if sessionStore != nil {
		_ = sessionStore.RevokeSession(ctx, hex.EncodeToString(hash[:]))
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if value, ok := p.sessions[hash]; ok {
		now := time.Now().UTC()
		value.revokedAt = &now
		p.sessions[hash] = value
	}
	return nil
}

func (p *LocalAuthProvider) Refresh(ctx context.Context, bearerToken string) (string, Principal, error) {
	principal, err := p.Authenticate(ctx, bearerToken)
	if err != nil {
		return "", Principal{}, err
	}
	if err := p.Logout(ctx, bearerToken); err != nil {
		return "", Principal{}, err
	}
	return p.issue(ctx, principal)
}

func (p *LocalAuthProvider) issue(ctx context.Context, principal Principal) (string, Principal, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", Principal{}, errors.New("session issuance failed")
	}
	token := "nhs_" + base64.RawURLEncoding.EncodeToString(tokenBytes)
	tokenHash := sha256.Sum256([]byte(token))
	p.mu.RLock()
	sessionStore := p.sessionStore
	p.mu.RUnlock()
	expiresAt := time.Now().UTC().Add(p.sessionTTL)
	if sessionStore != nil {
		if err := sessionStore.CreateSession(ctx, principal.UserID, hex.EncodeToString(tokenHash[:]), "api", expiresAt); err != nil {
			return "", Principal{}, errors.New("session persistence failed")
		}
	}
	p.mu.Lock()
	p.sessions[tokenHash] = session{principal: principal, expiresAt: expiresAt}
	p.mu.Unlock()
	return token, principal, nil
}
