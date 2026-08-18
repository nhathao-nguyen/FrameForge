package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nhathao-nguyen/NH-Media/packages/shared-contracts/go/security"
)

type Tool string

const (
	ToolFFmpeg  Tool = "ffmpeg"
	ToolFFprobe Tool = "ffprobe"
)

type Policy struct {
	FFmpegPath     string
	FFprobePath    string
	SandboxRoot    string
	MaxOutputBytes int64
	MaxInputBytes  int64
	MaxDuration    time.Duration
	MaxArgs        int
	MaxChildren    int
	MaxDiskBytes   int64
}

type Spec struct {
	Tool        Tool
	Args        []string
	InputPaths  []string
	OutputPaths []string
	Timeout     time.Duration
}

type Result struct {
	Tool       Tool
	ExitCode   int
	Duration   time.Duration
	Stdout     string
	Diagnostic string
	TimedOut   bool
	Cancelled  bool
}

var ErrPolicyViolation = errors.New("media process policy violation")

func (p Policy) Validate() error {
	if strings.TrimSpace(p.FFmpegPath) == "" || strings.TrimSpace(p.FFprobePath) == "" {
		return errors.New("ffmpeg and ffprobe paths are required")
	}
	if strings.TrimSpace(p.SandboxRoot) == "" {
		return errors.New("media sandbox root is required")
	}
	if p.MaxOutputBytes <= 0 || p.MaxDuration <= 0 {
		return errors.New("media process limits must be positive")
	}
	return nil
}

func (p Policy) Run(ctx context.Context, spec Spec) (Result, error) {
	if err := p.Validate(); err != nil {
		return Result{}, err
	}
	if ctx == nil {
		return Result{}, errors.New("context is required")
	}
	path, err := p.toolPath(spec.Tool)
	if err != nil {
		return Result{}, err
	}
	budget := p.resourceBudget()
	if err := budget.Validate(); err != nil {
		return Result{}, err
	}
	if len(spec.Args) > budget.MaxArgs {
		return Result{}, fmt.Errorf("%w: argv count exceeds limit", ErrPolicyViolation)
	}
	if err := validateArgs(spec.Args, spec.InputPaths, spec.OutputPaths); err != nil {
		return Result{}, err
	}
	for _, value := range append(append([]string{}, spec.InputPaths...), spec.OutputPaths...) {
		if err := (security.PathPolicy{Root: p.SandboxRoot, AllowMissingLeaf: true}).Validate(value); err != nil {
			return Result{}, fmt.Errorf("%w: %v", ErrPolicyViolation, err)
		}
		if info, statErr := os.Stat(value); statErr == nil && contains(spec.InputPaths, value) {
			if err := budget.ValidateInput(info.Size()); err != nil {
				return Result{}, fmt.Errorf("%w: %v", ErrPolicyViolation, err)
			}
		}
	}
	timeout := spec.Timeout
	if timeout <= 0 || timeout > p.MaxDuration {
		timeout = p.MaxDuration
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.Command(path, spec.Args...)
	configureProcessTree(command)
	command.Env = minimalEnvironment()
	command.Dir = p.SandboxRoot
	stdout := &limitedBuffer{limit: p.MaxOutputBytes}
	stderr := &limitedBuffer{limit: p.MaxOutputBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	started := time.Now()
	if err = command.Start(); err == nil {
		done := make(chan error, 1)
		go func() { done <- command.Wait() }()
		select {
		case err = <-done:
		case <-runCtx.Done():
			_ = killProcessTree(command.Process.Pid)
			err = <-done
		}
	}
	duration := time.Since(started)
	result := Result{Tool: spec.Tool, Duration: duration, Stdout: trimDiagnostic(stdout.String()), Diagnostic: trimDiagnostic(stderr.String())}
	if stdout.limited || stderr.limited {
		return result, fmt.Errorf("%w: process output exceeded limit", ErrPolicyViolation)
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
		return result, context.DeadlineExceeded
	}
	if errors.Is(runCtx.Err(), context.Canceled) {
		result.Cancelled = true
		return result, context.Canceled
	}
	if err != nil {
		result.ExitCode = exitCode(err)
		return result, fmt.Errorf("media process failed: exit code %d: %s", result.ExitCode, safeProcessError(err))
	}
	result.ExitCode = 0
	return result, nil
}

func (p Policy) resourceBudget() security.ResourceBudget {
	maxInput := p.MaxInputBytes
	if maxInput <= 0 {
		maxInput = 8 << 30
	}
	maxArgs := p.MaxArgs
	if maxArgs <= 0 {
		maxArgs = 256
	}
	maxChildren := p.MaxChildren
	if maxChildren <= 0 {
		maxChildren = 1
	}
	maxDisk := p.MaxDiskBytes
	if maxDisk <= 0 {
		maxDisk = 16 << 30
	}
	return security.ResourceBudget{MaxInputBytes: maxInput, MaxOutputBytes: p.MaxOutputBytes, MaxDuration: p.MaxDuration, MaxArgs: maxArgs, MaxChildren: maxChildren, MaxDiskBytes: maxDisk}
}

func (p Policy) toolPath(tool Tool) (string, error) {
	switch tool {
	case ToolFFmpeg:
		return filepath.Clean(p.FFmpegPath), nil
	case ToolFFprobe:
		return filepath.Clean(p.FFprobePath), nil
	default:
		return "", fmt.Errorf("%w: executable tool is not allowlisted", ErrPolicyViolation)
	}
}

func validateArgs(args, inputs, outputs []string) error {
	allowedPaths := make(map[string]struct{}, len(inputs)+len(outputs))
	for _, value := range append(append([]string{}, inputs...), outputs...) {
		allowedPaths[filepath.Clean(value)] = struct{}{}
	}
	filterValue := false
	for _, arg := range args {
		if arg == "" || strings.ContainsAny(arg, "\x00\r\n") {
			return fmt.Errorf("%w: empty or control-character argv", ErrPolicyViolation)
		}
		if filterValue {
			if strings.ContainsAny(arg, "\x00\r\n&$") {
				return fmt.Errorf("%w: unsafe generated media filter", ErrPolicyViolation)
			}
			filterValue = false
			continue
		}
		if arg == "-filter_complex" || arg == "-vf" || arg == "-af" {
			filterValue = true
			continue
		}
		if strings.ContainsAny(arg, ";&|$`()") {
			return fmt.Errorf("%w: shell syntax in argv", ErrPolicyViolation)
		}
		lower := strings.ToLower(arg)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "rtmp://") || strings.HasPrefix(lower, "file://") {
			return fmt.Errorf("%w: network or file URL is not allowed", ErrPolicyViolation)
		}
		if lower == "-protocol_whitelist" || lower == "-safe" || lower == "-filter_complex_script" || lower == "-pass" || lower == "-passlogfile" || lower == "-headers" {
			return fmt.Errorf("%w: unsafe ffmpeg option %q", ErrPolicyViolation, arg)
		}
		if filepath.IsAbs(arg) || strings.Contains(arg, "/") || strings.Contains(arg, "\\") {
			if _, ok := allowedPaths[filepath.Clean(arg)]; !ok {
				return fmt.Errorf("%w: path is not an approved process input/output", ErrPolicyViolation)
			}
		}
	}
	return nil
}

func validateSandboxPath(root, value string) error {
	return (security.PathPolicy{Root: root, AllowMissingLeaf: true}).Validate(value)
}

func minimalEnvironment() []string {
	if runtime.GOOS == "windows" {
		return []string{"SystemRoot=" + os.Getenv("SystemRoot"), "PATH=" + os.Getenv("PATH"), "TEMP=" + os.Getenv("TEMP"), "TMP=" + os.Getenv("TMP"), "LC_ALL=C"}
	}
	return []string{"PATH=" + os.Getenv("PATH"), "HOME=", "LC_ALL=C"}
}

type limitedBuffer struct {
	bytes.Buffer
	limit   int64
	limited bool
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	remaining := b.limit - int64(b.Len())
	if remaining <= 0 {
		b.limited = true
		return len(value), nil
	}
	if int64(len(value)) > remaining {
		_, _ = b.Buffer.Write(value[:remaining])
		b.limited = true
		return len(value), nil
	}
	return b.Buffer.Write(value)
}

func trimDiagnostic(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 1024 {
		// Media QA filters emit their typed markers after ffmpeg's bounded input
		// summary. Preserve both ends so blackdetect/silencedetect evidence is
		// not discarded while keeping diagnostics strictly size-bounded.
		value = value[:512] + "\n...[diagnostic truncated]...\n" + value[len(value)-512:]
	}
	return value
}

func exitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func safeProcessError(err error) string {
	if err == nil {
		return "unknown process error"
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 256 {
		message = message[:256]
	}
	return security.RedactText(message)
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if filepath.Clean(value) == filepath.Clean(candidate) {
			return true
		}
	}
	return false
}

var _ io.Writer = (*limitedBuffer)(nil)
