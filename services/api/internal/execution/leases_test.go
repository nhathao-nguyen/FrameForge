package execution

import (
	"testing"
	"time"
)

func TestNewLeaseOnlyExposesRawTokenToCallerAndStoresHash(t *testing.T) {
	now := time.Unix(100, 0)
	lease, err := NewLease(30*time.Second, 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if lease.Token == "" || lease.TokenHash == lease.Token || lease.TokenHash != HashLeaseToken(lease.Token) || !lease.ExpiresAt.Equal(now.Add(30*time.Second)) {
		t.Fatalf("invalid lease material: %+v", lease)
	}
}
