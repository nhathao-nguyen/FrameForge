package security

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPathPolicyRejectsTraversalAndSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	inside := filepath.Join(root, "inside")
	if err := os.Mkdir(inside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inside, "ok.bin"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	policy := PathPolicy{Root: root, AllowMissingLeaf: true}
	if err := policy.Validate(filepath.Join(inside, "ok.bin")); err != nil {
		t.Fatal(err)
	}
	if err := policy.Validate(filepath.Join(root, "..", filepath.Base(outside), "bad")); !errors.Is(err, ErrPolicyViolation) {
		t.Fatalf("traversal accepted: %v", err)
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err == nil {
		if err := policy.Validate(filepath.Join(link, "bad.bin")); !errors.Is(err, ErrPolicyViolation) {
			t.Fatalf("symlink escape accepted: %v", err)
		}
	}
}

func TestEgressPolicyIsExactAndBlocksSSRF(t *testing.T) {
	resolver := func(host string) ([]net.IP, error) {
		if host == "media.example" {
			return []net.IP{net.ParseIP("203.0.113.10")}, nil
		}
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}
	policy := EgressPolicy{Rules: []EndpointRule{{Scheme: "https", Host: "media.example", Ports: []int{443}}}, Resolver: resolver}
	if err := policy.Validate("https://media.example/api"); err != nil {
		t.Fatal(err)
	}
	if err := policy.Validate("https://media.example.evil/api"); !errors.Is(err, ErrPolicyViolation) {
		t.Fatalf("suffix host accepted: %v", err)
	}
	if err := policy.Validate("https://metadata.invalid/"); !errors.Is(err, ErrPolicyViolation) {
		t.Fatalf("unapproved host accepted: %v", err)
	}
	local := EgressPolicy{Rules: []EndpointRule{{Scheme: "http", Host: "127.0.0.1", Ports: []int{8080}, AllowPrivate: true}}, Resolver: resolver}
	if err := local.Validate("http://127.0.0.1:8080/health"); err != nil {
		t.Fatal(err)
	}
}

func TestSecurityBudgetsScopesPluginsAndRedactionFailClosed(t *testing.T) {
	if err := (ResourceBudget{MaxInputBytes: 1, MaxOutputBytes: 1, MaxDuration: time.Second, MaxArgs: 1, MaxChildren: 1, MaxDiskBytes: 1}).ValidateInput(2); !errors.Is(err, ErrPolicyViolation) {
		t.Fatal(err)
	}
	if err := ValidateSecretScope("render", []string{"llm"}); !errors.Is(err, ErrPolicyViolation) {
		t.Fatalf("render received provider secret: %v", err)
	}
	if err := (PluginPolicy{}).Validate(PluginManifest{Name: "x", Version: "1", SHA256: ""}); !errors.Is(err, ErrPolicyViolation) {
		t.Fatalf("disabled plugin accepted: %v", err)
	}
	if RedactText(`authorization=secret-value C:\private\file.mp4`) != "[REDACTED]" {
		t.Fatal("sensitive diagnostic was not redacted")
	}
}
