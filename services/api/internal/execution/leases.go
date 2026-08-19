package execution

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

var ErrLeaseConflict = errors.New("execution lease conflict")

type LeaseClaim struct {
	WorkspaceID      string
	ProjectID        string
	JobID            string
	PipelineRunID    string
	JobStepID        string
	PipelineNodeID   string
	NodeKey          string
	WorkerID         string
	ExpectedAttempt  int
	InputFingerprint string
	Duration         time.Duration
}

type Lease struct {
	Token     string
	TokenHash string
	AttemptID string
	Attempt   int
	ExpiresAt time.Time
}

func NewLease(duration time.Duration, attempt int, now time.Time) (Lease, error) {
	if attempt < 1 || duration <= 0 || duration > time.Hour {
		return Lease{}, errors.New("lease parameters are invalid")
	}
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return Lease{}, err
	}
	token := hex.EncodeToString(value)
	return Lease{Token: token, TokenHash: HashLeaseToken(token), Attempt: attempt, ExpiresAt: now.UTC().Add(duration)}, nil
}

func HashLeaseToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
