// Package security contains small, dependency-free policy primitives shared by
// the API and disposable workers. Policies fail closed and carry no product
// state, credentials, or local executor paths across a contract boundary.
package security

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var ErrPolicyViolation = errors.New("security policy violation")

// PathPolicy verifies a path stays below Root after parent symlinks/junctions
// are resolved. A missing final leaf is allowed only when AllowMissingLeaf is
// explicitly set; its parent must still be real and contained.
type PathPolicy struct {
	Root             string
	AllowMissingLeaf bool
}

func (p PathPolicy) Validate(path string) error {
	if strings.TrimSpace(p.Root) == "" || strings.TrimSpace(path) == "" {
		return fmt.Errorf("%w: root and path are required", ErrPolicyViolation)
	}
	root, err := filepath.Abs(filepath.Clean(p.Root))
	if err != nil {
		return fmt.Errorf("%w: resolve root: %v", ErrPolicyViolation, err)
	}
	candidate, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("%w: resolve path: %v", ErrPolicyViolation, err)
	}
	if !contained(root, candidate) {
		return fmt.Errorf("%w: path escapes sandbox", ErrPolicyViolation)
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("%w: resolve sandbox: %v", ErrPolicyViolation, err)
	}
	check := candidate
	if _, statErr := os.Lstat(candidate); statErr != nil {
		if !p.AllowMissingLeaf || !os.IsNotExist(statErr) {
			return fmt.Errorf("%w: path is unavailable", ErrPolicyViolation)
		}
		check = filepath.Dir(candidate)
	}
	resolved, err := filepath.EvalSymlinks(check)
	if err != nil {
		return fmt.Errorf("%w: resolve path: %v", ErrPolicyViolation, err)
	}
	if !contained(rootReal, resolved) {
		return fmt.Errorf("%w: symlink or reparse path escapes sandbox", ErrPolicyViolation)
	}
	return nil
}

func contained(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func ValidateObjectKey(key string) error {
	if strings.TrimSpace(key) == "" || strings.ContainsRune(key, 0) || strings.ContainsAny(key, "\\\r\n\t") || strings.HasPrefix(key, "/") || strings.Contains(key, ":") {
		return fmt.Errorf("%w: invalid object key", ErrPolicyViolation)
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || part == ".." || strings.ContainsRune(part, 0) {
			return fmt.Errorf("%w: invalid object key segment", ErrPolicyViolation)
		}
	}
	return nil
}

func ValidateArchiveEntry(name string) error {
	if strings.TrimSpace(name) == "" || strings.ContainsRune(name, 0) || strings.ContainsAny(name, "\r\n") {
		return fmt.Errorf("%w: invalid archive entry", ErrPolicyViolation)
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
		return fmt.Errorf("%w: absolute archive entry", ErrPolicyViolation)
	}
	for _, part := range strings.FieldsFunc(strings.ReplaceAll(name, "\\", "/"), func(r rune) bool { return r == '/' }) {
		if part == ".." || part == "." {
			return fmt.Errorf("%w: archive traversal", ErrPolicyViolation)
		}
	}
	return nil
}

// EndpointRule is an exact, operator-declared egress destination. Wildcards
// are intentionally unsupported. Private addresses are permitted only when
// the rule explicitly opts into the local/LAN dependency.
type EndpointRule struct {
	Scheme       string
	Host         string
	Ports        []int
	AllowPrivate bool
}

type EgressPolicy struct {
	Offline  bool
	Rules    []EndpointRule
	Resolver func(string) ([]net.IP, error)
}

func (p EgressPolicy) Validate(raw string) error {
	if p.Offline {
		return fmt.Errorf("%w: outbound network is disabled", ErrPolicyViolation)
	}
	u, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Hostname() == "" || u.User != nil || u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: URL is not an approved HTTP endpoint", ErrPolicyViolation)
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	port := 0
	if rawPort := u.Port(); rawPort != "" {
		port, err = strconv.Atoi(rawPort)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("%w: invalid endpoint port", ErrPolicyViolation)
		}
	} else if u.Scheme == "https" {
		port = 443
	} else {
		port = 80
	}
	for _, rule := range p.Rules {
		if strings.ToLower(rule.Scheme) != u.Scheme || strings.ToLower(strings.TrimSuffix(rule.Host, ".")) != host || !containsPort(rule.Ports, port) {
			continue
		}
		ips, lookupErr := lookupHost(host, p.Resolver)
		if lookupErr != nil {
			return fmt.Errorf("%w: endpoint DNS lookup failed", ErrPolicyViolation)
		}
		for _, ip := range ips {
			if isPrivateOrSpecial(ip) && !rule.AllowPrivate {
				return fmt.Errorf("%w: private or special endpoint address denied", ErrPolicyViolation)
			}
		}
		return nil
	}
	return fmt.Errorf("%w: endpoint is not allowlisted", ErrPolicyViolation)
}

func containsPort(ports []int, port int) bool {
	for _, candidate := range ports {
		if candidate == port {
			return true
		}
	}
	return false
}

func lookupHost(host string, resolver func(string) ([]net.IP, error)) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	if resolver != nil {
		return resolver(host)
	}
	return net.LookupIP(host)
}

func isPrivateOrSpecial(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() || ip.Equal(net.ParseIP("169.254.169.254"))
}

type ResourceBudget struct {
	MaxInputBytes  int64
	MaxOutputBytes int64
	MaxDuration    time.Duration
	MaxArgs        int
	MaxChildren    int
	MaxDiskBytes   int64
}

func (b ResourceBudget) Validate() error {
	if b.MaxInputBytes <= 0 || b.MaxOutputBytes <= 0 || b.MaxDuration <= 0 || b.MaxArgs <= 0 || b.MaxChildren <= 0 || b.MaxDiskBytes <= 0 {
		return fmt.Errorf("%w: resource budget must be positive", ErrPolicyViolation)
	}
	return nil
}

func (b ResourceBudget) ValidateInput(size int64) error {
	if err := b.Validate(); err != nil {
		return err
	}
	if size < 0 || size > b.MaxInputBytes {
		return fmt.Errorf("%w: input resource limit exceeded", ErrPolicyViolation)
	}
	return nil
}

// ValidateSecretScope ensures a worker receives only the narrow secret class
// declared for its capability. Render/probe/media never receive provider
// secrets; provider calls use the AI/ML capability-specific scope.
func ValidateSecretScope(capability string, secretKinds []string) error {
	allowed := map[string]map[string]bool{
		"probe": {}, "thumbnail": {}, "media": {}, "render": {},
		"ai":       {"llm": true, "vlm": true, "tts": true, "asr": true, "embedding": true},
		"ml":       {"llm": true, "vlm": true, "tts": true, "asr": true, "embedding": true},
		"analysis": {"llm": true, "vlm": true, "tts": true, "asr": true, "embedding": true},
	}
	if _, ok := allowed[capability]; !ok {
		return fmt.Errorf("%w: capability is not allowlisted", ErrPolicyViolation)
	}
	for _, kind := range secretKinds {
		if !allowed[capability][strings.ToLower(strings.TrimSpace(kind))] {
			return fmt.Errorf("%w: secret scope is not valid for capability", ErrPolicyViolation)
		}
	}
	return nil
}

type PluginManifest struct {
	Name    string
	Version string
	SHA256  string
}

type PluginPolicy struct {
	Enabled bool
	Allowed []PluginManifest
}

func (p PluginPolicy) Validate(manifest PluginManifest) error {
	if !p.Enabled {
		return fmt.Errorf("%w: third-party extensions are disabled", ErrPolicyViolation)
	}
	for _, allowed := range p.Allowed {
		if allowed == manifest && manifest.Name != "" && manifest.Version != "" && len(manifest.SHA256) == 64 {
			return nil
		}
	}
	return fmt.Errorf("%w: extension manifest is not reviewed", ErrPolicyViolation)
}
