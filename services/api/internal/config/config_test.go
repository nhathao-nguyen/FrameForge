package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestAPINamespacesAndLocalBounds(t *testing.T) {
	key := base64.RawStdEncoding.EncodeToString(make([]byte, 32))
	env := map[string]string{"NH_MEDIA_PROFILE": "local", "NH_API_MASTER_KEY": key, "NH_API_ALLOWED_ORIGINS": "http://localhost:3000"}
	value, err := LoadAPI(func(name string) string { return env[name] })
	if err != nil {
		t.Fatal(err)
	}
	if value.Profile != "local" || !strings.HasPrefix(value.Bind, "127.0.0.1:") {
		t.Fatalf("unexpected config: %+v", value)
	}
	if _, err := NewSecretStore(value.MasterKey); err != nil {
		t.Fatal(err)
	}
}

func TestSecretStoreDoesNotExposePlaintext(t *testing.T) {
	store, err := NewSecretStore(bytes32(9))
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.Encrypt(SecretReference{Ref: "provider/demo", Version: 1}, []byte("provider-secret-value"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(record.Ciphertext, "provider-secret-value") || record.Nonce == "" {
		t.Fatalf("unsafe secret record: %+v", record)
	}
	value, err := store.Decrypt(record)
	if err != nil || string(value) != "provider-secret-value" {
		t.Fatalf("decrypt: %v", err)
	}
}

func TestRedactionAndProjectExecutableBoundary(t *testing.T) {
	fields := RedactFields(map[string]string{"secret": "abc", "path": "C:\\secret", "request_id": "req_1", "code": "failed"})
	if fields["secret"] == "abc" || fields["path"] == "C:\\secret" || fields["request_id"] != "req_1" {
		t.Fatalf("redaction failed: %#v", fields)
	}
	if got := RedactString("provider secret leaked"); got == "provider secret leaked" {
		t.Fatal("secret text was not redacted")
	}
}

func bytes32(value byte) []byte {
	result := make([]byte, 32)
	for index := range result {
		result[index] = value
	}
	return result
}
