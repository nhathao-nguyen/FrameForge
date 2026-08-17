package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type APIConfig struct {
	Profile           string
	Bind              string
	AllowedOrigins    []string
	AdminUsername     string
	AdminPassword     string
	SessionSecret     string
	MasterKey         []byte
	MaxBodyBytes      int64
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}

type WorkerConfig struct {
	Namespace      string
	MaxConcurrency int
	FFmpegPath     string
	FFprobePath    string
}

type ProviderConfig struct{ DefaultTimeout time.Duration }
type StorageConfig struct{ Endpoint, AccessKeyRef, SecretKeyRef string }
type QueueConfig struct{ Endpoint, PasswordRef string }

func LoadAPI(env func(string) string) (APIConfig, error) {
	profile := first(env("NH_API_PROFILE"), env("NH_MEDIA_PROFILE"), "local")
	bind := first(env("NH_API_BIND"), "127.0.0.1:8080")
	origins := splitCSV(env("NH_API_ALLOWED_ORIGINS"))
	if len(origins) == 0 {
		origins = []string{"http://127.0.0.1:3000", "http://localhost:3000"}
	}
	maxBody, err := positiveInt64(first(env("NH_API_MAX_BODY_BYTES"), "1048576"))
	if err != nil {
		return APIConfig{}, fmt.Errorf("NH_API_MAX_BODY_BYTES: %w", err)
	}
	master, err := decodeMasterKey(env("NH_API_MASTER_KEY"))
	if err != nil && env("NH_API_MASTER_KEY") != "" {
		return APIConfig{}, fmt.Errorf("NH_API_MASTER_KEY: %w", err)
	}
	value := APIConfig{
		Profile: profile, Bind: bind, AllowedOrigins: origins,
		AdminUsername: env("NH_API_ADMIN_USERNAME"), AdminPassword: env("NH_API_ADMIN_PASSWORD"),
		SessionSecret: env("NH_API_SESSION_SECRET"), MasterKey: master, MaxBodyBytes: maxBody,
		ReadHeaderTimeout: duration(env("NH_API_READ_HEADER_TIMEOUT"), 5*time.Second),
		ReadTimeout:       duration(env("NH_API_READ_TIMEOUT"), 15*time.Second),
		WriteTimeout:      duration(env("NH_API_WRITE_TIMEOUT"), 20*time.Second),
		IdleTimeout:       duration(env("NH_API_IDLE_TIMEOUT"), 60*time.Second), MaxHeaderBytes: 32 * 1024,
	}
	if err := value.Validate(); err != nil {
		return APIConfig{}, err
	}
	return value, nil
}

func (c APIConfig) Validate() error {
	if c.Profile != "local" && c.Profile != "lan" {
		return errors.New("profile must be local or lan")
	}
	if c.Bind == "" || len(c.AllowedOrigins) == 0 {
		return errors.New("explicit bind and allowed origins are required")
	}
	for _, origin := range c.AllowedOrigins {
		u, err := url.Parse(origin)
		if err != nil || u.Scheme == "" || u.Host == "" || u.Host == "*" || strings.Contains(origin, "*") {
			return errors.New("allowed origins must be exact origins")
		}
	}
	if c.Profile == "local" && !strings.HasPrefix(c.Bind, "127.0.0.1:") && !strings.HasPrefix(c.Bind, "localhost:") {
		return errors.New("local profile must bind loopback")
	}
	if c.MaxBodyBytes <= 0 || c.MaxHeaderBytes <= 0 {
		return errors.New("HTTP bounds must be positive")
	}
	return nil
}

func LoadWorker(env func(string) string, namespace string) (WorkerConfig, error) {
	value := WorkerConfig{Namespace: namespace, FFmpegPath: env("NH_MEDIA_FFMPEG_PATH"), FFprobePath: env("NH_MEDIA_FFPROBE_PATH")}
	if namespace == "nh_media" {
		value.MaxConcurrency = 1
		if raw := env("NH_MEDIA_MAX_CONCURRENCY"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				return WorkerConfig{}, err
			}
			value.MaxConcurrency = parsed
		}
	} else {
		value.MaxConcurrency = 1
	}
	if value.MaxConcurrency < 1 {
		return WorkerConfig{}, errors.New("worker concurrency must be positive")
	}
	return value, nil
}

type SecretReference struct {
	Ref     string `json:"secret_ref"`
	Version int    `json:"version"`
}
type EncryptedSecretRecord struct {
	Ref        string `json:"secret_ref"`
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type SecretStore struct{ key []byte }

func NewSecretStore(masterKey []byte) (*SecretStore, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("server master key must be 32 bytes")
	}
	copyKey := append([]byte(nil), masterKey...)
	return &SecretStore{key: copyKey}, nil
}

func (s *SecretStore) Encrypt(ref SecretReference, plaintext []byte) (EncryptedSecretRecord, error) {
	if s == nil || ref.Ref == "" || ref.Version < 1 {
		return EncryptedSecretRecord{}, errors.New("invalid secret reference")
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return EncryptedSecretRecord{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return EncryptedSecretRecord{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return EncryptedSecretRecord{}, err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, []byte(ref.Ref))
	return EncryptedSecretRecord{Ref: ref.Ref, Version: ref.Version, Nonce: base64.RawStdEncoding.EncodeToString(nonce), Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext)}, nil
}

func (s *SecretStore) Decrypt(record EncryptedSecretRecord) ([]byte, error) {
	if s == nil || record.Ref == "" || record.Version < 1 {
		return nil, errors.New("invalid encrypted secret record")
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, err := base64.RawStdEncoding.DecodeString(record.Nonce)
	if err != nil {
		return nil, err
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(record.Ciphertext)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, []byte(record.Ref))
}

func RedactString(value string) string {
	for _, key := range []string{"authorization", "password", "secret", "token", "presigned", "traceback"} {
		lower := strings.ToLower(value)
		if strings.Contains(lower, key) {
			return "[REDACTED]"
		}
	}
	return value
}

func RedactFields(fields map[string]string) map[string]string {
	result := make(map[string]string, len(fields))
	for key, value := range fields {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "token") || strings.Contains(lower, "authorization") || strings.Contains(lower, "path") || strings.Contains(lower, "url") || strings.Contains(lower, "traceback") {
			result[key] = "[REDACTED]"
		} else {
			result[key] = value
		}
	}
	return result
}

func decodeMasterKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, nil
	}
	decoded, err := base64.RawStdEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	if len(decoded) != 32 {
		return nil, errors.New("master key must decode to 32 bytes")
	}
	return decoded, nil
}
func positiveInt64(raw string) (int64, error) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, errors.New("must be a positive integer")
	}
	return value, nil
}
func duration(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
func splitCSV(raw string) []string {
	var values []string
	for _, value := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
func MasterKeyFingerprint(key []byte) string {
	if len(key) == 0 {
		return ""
	}
	digest := sha256.Sum256(key)
	return base64.RawURLEncoding.EncodeToString(digest[:6])
}
