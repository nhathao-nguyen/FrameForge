package process

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testPolicy(t *testing.T) Policy {
	t.Helper()
	root := t.TempDir()
	tool, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Policy{FFmpegPath: tool, FFprobePath: tool, SandboxRoot: root, MaxOutputBytes: 1024, MaxDuration: time.Second}
}

func TestProcessRejectsUnreviewedToolAndUnsafeArguments(t *testing.T) {
	policy := testPolicy(t)
	for _, spec := range []Spec{
		{Tool: Tool("python"), Args: []string{}},
		{Tool: ToolFFmpeg, Args: []string{"-i", "https://example.invalid/a.mp4"}},
		{Tool: ToolFFmpeg, Args: []string{"-i", "input.mp4; whoami"}},
		{Tool: ToolFFmpeg, Args: []string{"-protocol_whitelist"}},
	} {
		_, err := policy.Run(context.Background(), spec)
		if !errors.Is(err, ErrPolicyViolation) {
			t.Errorf("spec %+v returned %v", spec, err)
		}
	}
}

func TestProcessRejectsPathsOutsideSandboxAndSymlinkEscape(t *testing.T) {
	policy := testPolicy(t)
	outside := filepath.Join(t.TempDir(), "outside.mp4")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := policy.Run(context.Background(), Spec{Tool: ToolFFmpeg, Args: []string{"-i", outside}, InputPaths: []string{outside}})
	if !errors.Is(err, ErrPolicyViolation) {
		t.Fatalf("outside path was accepted: %v", err)
	}
	inside := filepath.Join(policy.SandboxRoot, "inside.mp4")
	if err := os.WriteFile(inside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateSandboxPath(policy.SandboxRoot, inside); err != nil {
		t.Fatal(err)
	}
}

func TestProcessUsesDirectBoundedExecution(t *testing.T) {
	policy := testPolicy(t)
	// The test binary is used only as a deterministic direct executable; skip
	// its test suite so the child does not recursively launch this test.
	result, err := policy.Run(context.Background(), Spec{Tool: ToolFFmpeg, Args: []string{"-test.run=NoSuchTest"}})
	if err != nil || result.ExitCode != 0 || result.TimedOut || strings.Contains(result.Diagnostic, "traceback") {
		t.Fatalf("direct process execution failed: %+v err=%v", result, err)
	}
}
