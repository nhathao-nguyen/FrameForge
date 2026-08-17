package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"
)

type Principal struct {
	Subject string
	Role    string
}

type AuthPort interface {
	Authenticate(ctx context.Context, bearerToken string) (Principal, error)
}

type LocalAuthProvider struct {
	usernameHash [32]byte
	passwordHash [32]byte
	sessions     map[[32]byte]session
	mu           sync.RWMutex
	sessionTTL   time.Duration
}

type session struct {
	principal Principal
	expiresAt time.Time
}

func NewLocalAuthProvider(username, password string) (*LocalAuthProvider, error) {
	if strings.TrimSpace(username) == "" || password == "" {
		return nil, errors.New("local auth bootstrap credentials are required")
	}
	return &LocalAuthProvider{usernameHash: sha256.Sum256([]byte(username)), passwordHash: sha256.Sum256([]byte(password)), sessions: make(map[[32]byte]session), sessionTTL: 8 * time.Hour}, nil
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
	principal := Principal{Subject: "local-admin", Role: "owner"}
	p.mu.Lock()
	p.sessions[tokenHash] = session{principal: principal, expiresAt: time.Now().UTC().Add(p.sessionTTL)}
	p.mu.Unlock()
	return token, principal, nil
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
	value, ok := p.sessions[hash]
	p.mu.RUnlock()
	if !ok || time.Now().UTC().After(value.expiresAt) {
		return Principal{}, errors.New("invalid session")
	}
	return value.principal, nil
}
