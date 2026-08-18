package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Scope struct {
	WorkspaceID string
	ProjectID   string
}

type StorageLocator struct {
	Backend       string `json:"backend"`
	ObjectKey     string `json:"object_key"`
	ObjectVersion string `json:"object_version,omitempty"`
}

type ObjectMetadata struct {
	Locator     StorageLocator
	SizeBytes   int64
	ContentType string
	ETag        string
	SHA256      string
	ModifiedAt  time.Time
}

type UploadConstraints struct {
	KeyPrefix    string
	ExpectedSize int64
	ContentType  string
	PartSize     int64
	ExpiresAt    time.Time
	Multipart    bool
}

type UploadSession struct {
	ID         string
	Locator    StorageLocator
	ProviderID string
	Multipart  bool
	PartSize   int64
	ExpiresAt  time.Time
}

type SignedRequest struct {
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers"`
	ExpiresAt  time.Time         `json:"expires_at"`
	PartNumber int               `json:"part_number,omitempty"`
}

type UploadedPart struct {
	PartNumber int
	ETag       string
}

type StagedObject struct {
	Locator     StorageLocator
	SizeBytes   int64
	SHA256      string
	ContentType string
	localPath   string
}

type Preconditions struct {
	IfNoneMatch bool
	IfMatchETag string
}

type LocalHandle struct {
	path string
}

func (h LocalHandle) Path() string { return h.path }

type StoragePort interface {
	InitiateUpload(context.Context, Scope, UploadConstraints) (UploadSession, error)
	PresignPart(context.Context, UploadSession, int) (SignedRequest, error)
	CompleteUpload(context.Context, UploadSession, []UploadedPart) (StagedObject, error)
	AbortUpload(context.Context, UploadSession) error
	Stat(context.Context, StorageLocator) (ObjectMetadata, error)
	OpenRead(context.Context, StorageLocator, *ByteRange) (io.ReadCloser, error)
	PutStaged(context.Context, Scope, io.Reader, string) (StagedObject, error)
	Promote(context.Context, StagedObject, string, Preconditions) (ObjectMetadata, error)
	Materialize(context.Context, StorageLocator, string) (LocalHandle, error)
	PresignDownload(context.Context, StorageLocator, time.Duration, string) (SignedRequest, error)
	Delete(context.Context, StorageLocator, Preconditions) error
	ListStaging(context.Context, string) ([]StorageLocator, error)
}

// WorkerTransferPort grants lease-scoped executors direct object-store
// transfer without serializing StorageLocator or local paths in queue
// commands. Product state still stores only committed Artifact refs.
type WorkerTransferPort interface {
	WorkerBackend() string
	PresignWorkerUpload(context.Context, string, time.Duration, string) (SignedRequest, error)
	VerifyWorkerStage(context.Context, string, string, int64, string) (StagedObject, error)
}

func WorkerStageKey(workspaceRef, projectRef, messageID, artifactID string) (string, error) {
	for _, value := range []string{workspaceRef, projectRef, messageID, artifactID} {
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "/\\:\r\n\t") || value == "." || value == ".." {
			return "", errors.New("worker stage identity is invalid")
		}
	}
	return "workspaces/" + workspaceRef + "/projects/" + projectRef + "/staging/workers/" + messageID + "/" + artifactID, nil
}

type ByteRange struct{ Start, End int64 }

func (r *ByteRange) Validate() error {
	if r == nil {
		return nil
	}
	if r.Start < 0 || r.End < r.Start {
		return errors.New("invalid byte range")
	}
	return nil
}

func validateKey(key string) error {
	if key == "" || strings.ContainsRune(key, 0) || strings.ContainsAny(key, "\\\r\n\t") || strings.HasPrefix(key, "/") || strings.Contains(key, ":") {
		return errors.New("invalid storage key")
	}
	parts := strings.Split(key, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "\x00\r\n") {
			return errors.New("invalid storage key segment")
		}
	}
	return nil
}

func scopedKey(scope Scope, suffix string) (string, error) {
	if strings.TrimSpace(scope.WorkspaceID) == "" || strings.TrimSpace(scope.ProjectID) == "" {
		return "", errors.New("workspace and project scope are required")
	}
	if err := validateKey(scope.WorkspaceID); err != nil {
		return "", err
	}
	if err := validateKey(scope.ProjectID); err != nil {
		return "", err
	}
	if err := validateKey(suffix); err != nil {
		return "", err
	}
	return "workspaces/" + scope.WorkspaceID + "/projects/" + scope.ProjectID + "/" + suffix, nil
}

type LocalStorage struct {
	root      string
	cacheRoot string
	mu        sync.Mutex
	uploads   map[string]localUpload
}

type localUpload struct {
	session UploadSession
	parts   map[int]string
}

func NewLocalStorage(root, cacheRoot string) (*LocalStorage, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("local storage root is required")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(rootAbs, 0o700); err != nil {
		return nil, err
	}
	if cacheRoot == "" {
		cacheRoot = filepath.Join(rootAbs, ".cache")
	}
	cacheAbs, err := filepath.Abs(cacheRoot)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cacheAbs, 0o700); err != nil {
		return nil, err
	}
	return &LocalStorage{root: rootAbs, cacheRoot: cacheAbs, uploads: map[string]localUpload{}}, nil
}

func (s *LocalStorage) resolve(key string, createParent bool) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	path := filepath.Join(s.root, filepath.FromSlash(key))
	rootReal, err := filepath.EvalSymlinks(s.root)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(path)
	if createParent {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			return "", err
		}
	}
	parentReal, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootReal, parentReal)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("storage path escapes configured root")
	}
	return path, nil
}

func (s *LocalStorage) InitiateUpload(_ context.Context, scope Scope, constraints UploadConstraints) (UploadSession, error) {
	suffix := "staging/uploads/" + newRandomID() + "/object"
	key, err := scopedKey(scope, suffix)
	if err != nil {
		return UploadSession{}, err
	}
	partSize := constraints.PartSize
	if partSize <= 0 {
		partSize = 8 << 20
	}
	expires := constraints.ExpiresAt
	if expires.IsZero() {
		expires = time.Now().UTC().Add(2 * time.Hour)
	}
	id := newRandomID()
	session := UploadSession{ID: id, Locator: StorageLocator{Backend: "local", ObjectKey: key}, Multipart: constraints.Multipart, PartSize: partSize, ExpiresAt: expires}
	s.mu.Lock()
	s.uploads[id] = localUpload{session: session, parts: map[int]string{}}
	s.mu.Unlock()
	return session, nil
}

func (s *LocalStorage) PresignPart(_ context.Context, session UploadSession, part int) (SignedRequest, error) {
	if part < 1 || time.Now().After(session.ExpiresAt) {
		return SignedRequest{}, errors.New("upload session expired or part is invalid")
	}
	if session.Locator.Backend != "local" {
		return SignedRequest{}, errors.New("wrong storage backend")
	}
	s.mu.Lock()
	stored, ok := s.uploads[session.ID]
	s.mu.Unlock()
	if !ok || stored.session.Locator != session.Locator {
		return SignedRequest{}, errors.New("upload session not found")
	}
	return SignedRequest{Method: "PUT", URL: "nh-local://" + url.PathEscape(session.ID) + "/" + fmt.Sprint(part), Headers: map[string]string{"Content-Type": "application/octet-stream"}, ExpiresAt: session.ExpiresAt, PartNumber: part}, nil
}

func (s *LocalStorage) CompleteUpload(_ context.Context, session UploadSession, parts []UploadedPart) (StagedObject, error) {
	s.mu.Lock()
	stored, ok := s.uploads[session.ID]
	if ok && stored.session.Locator != session.Locator {
		s.mu.Unlock()
		return StagedObject{}, errors.New("upload session scope mismatch")
	}
	if ok {
		delete(s.uploads, session.ID)
	}
	s.mu.Unlock()
	if !ok {
		return StagedObject{}, errors.New("upload session not found")
	}
	if len(parts) == 0 {
		return StagedObject{}, errors.New("at least one part is required")
	}
	// Local direct uploads are represented by a single staged object. Callers may
	// use PutStaged for the actual bytes; completion remains metadata-only and
	// never promotes an unvalidated Asset.
	return StagedObject{Locator: session.Locator}, nil
}

func (s *LocalStorage) AbortUpload(_ context.Context, session UploadSession) error {
	s.mu.Lock()
	if stored, ok := s.uploads[session.ID]; ok && stored.session.Locator != session.Locator {
		s.mu.Unlock()
		return errors.New("upload session scope mismatch")
	}
	delete(s.uploads, session.ID)
	s.mu.Unlock()
	return nil
}

func (s *LocalStorage) Stat(_ context.Context, locator StorageLocator) (ObjectMetadata, error) {
	if locator.Backend != "local" {
		return ObjectMetadata{}, errors.New("wrong storage backend")
	}
	path, err := s.resolve(locator.ObjectKey, false)
	if err != nil {
		return ObjectMetadata{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return ObjectMetadata{}, err
	}
	return ObjectMetadata{Locator: locator, SizeBytes: info.Size(), ModifiedAt: info.ModTime().UTC()}, nil
}

func (s *LocalStorage) OpenRead(_ context.Context, locator StorageLocator, byteRange *ByteRange) (io.ReadCloser, error) {
	if locator.Backend != "local" {
		return nil, errors.New("wrong storage backend")
	}
	if err := byteRange.Validate(); err != nil {
		return nil, err
	}
	path, err := s.resolve(locator.ObjectKey, false)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if byteRange == nil {
		return file, nil
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if byteRange.Start >= info.Size() || byteRange.End >= info.Size() {
		_ = file.Close()
		return nil, errors.New("byte range outside object")
	}
	return &sectionReadCloser{Reader: io.NewSectionReader(file, byteRange.Start, byteRange.End-byteRange.Start+1), closer: file}, nil
}

func (s *LocalStorage) PutStaged(_ context.Context, scope Scope, reader io.Reader, contentType string) (StagedObject, error) {
	key, err := scopedKey(scope, "staging/objects/"+newRandomID())
	if err != nil {
		return StagedObject{}, err
	}
	path, err := s.resolve(key, true)
	if err != nil {
		return StagedObject{}, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return StagedObject{}, err
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(file, hash), reader)
	closeErr := file.Close()
	if copyErr != nil {
		return StagedObject{}, copyErr
	}
	if closeErr != nil {
		return StagedObject{}, closeErr
	}
	return StagedObject{Locator: StorageLocator{Backend: "local", ObjectKey: key}, SizeBytes: size, SHA256: hex.EncodeToString(hash.Sum(nil)), ContentType: contentType, localPath: path}, nil
}

func (s *LocalStorage) Promote(_ context.Context, staged StagedObject, finalKey string, preconditions Preconditions) (ObjectMetadata, error) {
	if staged.Locator.Backend != "local" {
		return ObjectMetadata{}, errors.New("staged object belongs to another backend")
	}
	source, err := s.resolve(staged.Locator.ObjectKey, false)
	if err != nil {
		return ObjectMetadata{}, err
	}
	destination, err := s.resolve(finalKey, true)
	if err != nil {
		return ObjectMetadata{}, err
	}
	if _, statErr := os.Stat(destination); statErr == nil {
		if preconditions.IfNoneMatch {
			return ObjectMetadata{}, errors.New("destination already exists")
		}
		if preconditions.IfMatchETag != "" {
			return ObjectMetadata{}, errors.New("etag precondition cannot be checked without checksum")
		}
		return ObjectMetadata{}, errors.New("destination already exists")
	} else if !os.IsNotExist(statErr) {
		return ObjectMetadata{}, statErr
	}
	if err := os.Rename(source, destination); err != nil {
		return ObjectMetadata{}, err
	}
	info, err := os.Stat(destination)
	if err != nil {
		return ObjectMetadata{}, err
	}
	return ObjectMetadata{Locator: StorageLocator{Backend: "local", ObjectKey: finalKey}, SizeBytes: info.Size(), SHA256: staged.SHA256, ContentType: staged.ContentType, ModifiedAt: info.ModTime().UTC()}, nil
}

func (s *LocalStorage) Materialize(_ context.Context, locator StorageLocator, cacheScope string) (LocalHandle, error) {
	if locator.Backend != "local" {
		return LocalHandle{}, errors.New("wrong storage backend")
	}
	if err := validateKey(locator.ObjectKey); err != nil {
		return LocalHandle{}, err
	}
	name := filepath.Join(s.cacheRoot, filepath.Base(cacheScope)+"-"+newRandomID())
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return LocalHandle{}, err
	}
	in, err := s.OpenRead(context.Background(), locator, nil)
	if err != nil {
		return LocalHandle{}, err
	}
	defer in.Close()
	out, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return LocalHandle{}, err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return LocalHandle{}, err
	}
	if err := out.Close(); err != nil {
		return LocalHandle{}, err
	}
	return LocalHandle{path: name}, nil
}

func (s *LocalStorage) PresignDownload(_ context.Context, locator StorageLocator, ttl time.Duration, disposition string) (SignedRequest, error) {
	if locator.Backend != "local" {
		return SignedRequest{}, errors.New("wrong storage backend")
	}
	if err := validateKey(locator.ObjectKey); err != nil {
		return SignedRequest{}, err
	}
	if ttl <= 0 || ttl > 15*time.Minute {
		return SignedRequest{}, errors.New("download TTL must be 1..900 seconds")
	}
	return SignedRequest{Method: "GET", URL: "nh-local://" + url.PathEscape(locator.ObjectKey) + "?ttl=" + fmt.Sprint(int(ttl.Seconds())) + "&disposition=" + url.QueryEscape(disposition), ExpiresAt: time.Now().UTC().Add(ttl)}, nil
}

func (s *LocalStorage) Delete(_ context.Context, locator StorageLocator, _ Preconditions) error {
	if locator.Backend != "local" {
		return errors.New("wrong storage backend")
	}
	if err := validateKey(locator.ObjectKey); err != nil {
		return err
	}
	candidate := filepath.Join(s.root, filepath.FromSlash(locator.ObjectKey))
	if _, statErr := os.Lstat(candidate); os.IsNotExist(statErr) {
		return nil
	}
	path, err := s.resolve(locator.ObjectKey, false)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *LocalStorage) ListStaging(_ context.Context, prefix string) ([]StorageLocator, error) {
	if err := validateKey(prefix); err != nil {
		return nil, err
	}
	root := filepath.Join(s.root, filepath.FromSlash(prefix))
	var result []StorageLocator
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		result = append(result, StorageLocator{Backend: "local", ObjectKey: filepath.ToSlash(rel)})
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ObjectKey < result[j].ObjectKey })
	return result, err
}

func (s *LocalStorage) WorkerBackend() string { return "local" }

func (s *LocalStorage) PresignWorkerUpload(_ context.Context, key string, ttl time.Duration, contentType string) (SignedRequest, error) {
	if err := validateKey(key); err != nil || ttl <= 0 || ttl > 15*time.Minute || strings.TrimSpace(contentType) == "" {
		return SignedRequest{}, errors.New("worker upload request is invalid")
	}
	return SignedRequest{Method: "PUT", URL: "nh-local://" + url.PathEscape(key), Headers: map[string]string{"Content-Type": contentType}, ExpiresAt: time.Now().UTC().Add(ttl)}, nil
}

func (s *LocalStorage) VerifyWorkerStage(ctx context.Context, key, expectedSHA string, expectedSize int64, contentType string) (StagedObject, error) {
	return verifyWorkerStage(ctx, s, StorageLocator{Backend: "local", ObjectKey: key}, expectedSHA, expectedSize, contentType)
}

type sectionReadCloser struct {
	io.Reader
	closer io.Closer
}

func (s *sectionReadCloser) Close() error { return s.closer.Close() }

func newRandomID() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

// S3Storage is the MinIO/S3-compatible adapter. It shares the same key and
// reference rules as LocalStorage; credentials remain inside this adapter.
type S3Storage struct {
	client    *minio.Core
	apiClient *minio.Client
	bucket    string
	endpoint  string
	uploads   sync.Map
	uploadMu  sync.Mutex
}
type s3Upload struct {
	session UploadSession
	key     string
}

func NewS3Storage(endpoint, accessKey, secretKey, bucket string, secure bool) (*S3Storage, error) {
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		return nil, errors.New("S3 endpoint, credentials and bucket are required")
	}
	client, err := minio.NewCore(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: secure})
	if err != nil {
		return nil, err
	}
	apiClient, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: secure})
	if err != nil {
		return nil, err
	}
	return &S3Storage{client: client, apiClient: apiClient, bucket: bucket, endpoint: endpoint}, nil
}

func (s *S3Storage) InitiateUpload(ctx context.Context, scope Scope, constraints UploadConstraints) (UploadSession, error) {
	key, err := scopedKey(scope, "staging/uploads/"+newRandomID()+"/object")
	if err != nil {
		return UploadSession{}, err
	}
	partSize := constraints.PartSize
	if partSize <= 0 {
		partSize = 8 << 20
	}
	expires := constraints.ExpiresAt
	if expires.IsZero() {
		expires = time.Now().UTC().Add(2 * time.Hour)
	}
	providerID, err := s.client.NewMultipartUpload(ctx, s.bucket, key, minio.PutObjectOptions{ContentType: constraints.ContentType})
	if err != nil {
		return UploadSession{}, err
	}
	session := UploadSession{ID: newRandomID(), Locator: StorageLocator{Backend: "minio", ObjectKey: key}, ProviderID: providerID, Multipart: true, PartSize: partSize, ExpiresAt: expires}
	s.uploads.Store(session.ID, s3Upload{session: session, key: key})
	return session, nil
}

func (s *S3Storage) PresignPart(ctx context.Context, session UploadSession, part int) (SignedRequest, error) {
	if session.Locator.Backend != "minio" || session.ProviderID == "" || validateKey(session.Locator.ObjectKey) != nil || part < 1 || time.Now().After(session.ExpiresAt) {
		return SignedRequest{}, errors.New("upload session expired or part invalid")
	}
	params := url.Values{"uploadId": []string{session.ProviderID}, "partNumber": []string{strconv.Itoa(part)}}
	objectURL, err := s.client.Presign(ctx, "PUT", s.bucket, session.Locator.ObjectKey, time.Until(session.ExpiresAt), params)
	if err != nil {
		return SignedRequest{}, err
	}
	return SignedRequest{Method: "PUT", URL: objectURL.String(), Headers: map[string]string{"Content-Type": "application/octet-stream"}, ExpiresAt: session.ExpiresAt, PartNumber: part}, nil
}

func (s *S3Storage) CompleteUpload(ctx context.Context, session UploadSession, parts []UploadedPart) (StagedObject, error) {
	if session.Locator.Backend != "minio" || session.ProviderID == "" || validateKey(session.Locator.ObjectKey) != nil {
		return StagedObject{}, errors.New("invalid S3 upload session")
	}
	if len(parts) == 0 {
		return StagedObject{}, errors.New("at least one part is required")
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })
	completeParts := make([]minio.CompletePart, 0, len(parts))
	for index, part := range parts {
		if part.PartNumber < 1 || part.ETag == "" || (index > 0 && parts[index-1].PartNumber == part.PartNumber) {
			return StagedObject{}, errors.New("multipart parts must have unique positive numbers and etags")
		}
		completeParts = append(completeParts, minio.CompletePart{PartNumber: part.PartNumber, ETag: part.ETag})
	}
	// The provider upload ID and object key are persisted by the Product API.
	// The in-memory map is only a fast scope check for sessions created by this
	// process; a restart must still be able to complete a durable upload.
	objectKey := session.Locator.ObjectKey
	s.uploadMu.Lock()
	if value, ok := s.uploads.Load(session.ID); ok {
		upload := value.(s3Upload)
		if upload.session.ProviderID != session.ProviderID || upload.key != session.Locator.ObjectKey {
			s.uploadMu.Unlock()
			return StagedObject{}, errors.New("upload session scope mismatch")
		}
		objectKey = upload.key
		s.uploads.Delete(session.ID)
	}
	s.uploadMu.Unlock()
	if _, err := s.client.CompleteMultipartUpload(ctx, s.bucket, objectKey, session.ProviderID, completeParts, minio.PutObjectOptions{}); err != nil {
		return StagedObject{}, err
	}
	meta, err := s.Stat(ctx, session.Locator)
	if err != nil {
		return StagedObject{}, err
	}
	return StagedObject{Locator: session.Locator, SizeBytes: meta.SizeBytes, ContentType: meta.ContentType}, nil
}
func (s *S3Storage) AbortUpload(ctx context.Context, session UploadSession) error {
	s.uploadMu.Lock()
	value, ok := s.uploads.Load(session.ID)
	if !ok {
		s.uploadMu.Unlock()
		if session.Locator.Backend != "minio" || session.ProviderID == "" || validateKey(session.Locator.ObjectKey) != nil {
			return errors.New("invalid S3 upload session")
		}
		return s.client.AbortMultipartUpload(ctx, s.bucket, session.Locator.ObjectKey, session.ProviderID)
	}
	upload := value.(s3Upload)
	if upload.session.ProviderID != session.ProviderID || upload.key != session.Locator.ObjectKey {
		s.uploadMu.Unlock()
		return errors.New("upload session scope mismatch")
	}
	s.uploads.Delete(session.ID)
	s.uploadMu.Unlock()
	return s.client.AbortMultipartUpload(ctx, s.bucket, session.Locator.ObjectKey, session.ProviderID)
}
func (s *S3Storage) Stat(ctx context.Context, locator StorageLocator) (ObjectMetadata, error) {
	if locator.Backend != "minio" {
		return ObjectMetadata{}, errors.New("wrong storage backend")
	}
	if err := validateKey(locator.ObjectKey); err != nil {
		return ObjectMetadata{}, err
	}
	info, err := s.client.StatObject(ctx, s.bucket, locator.ObjectKey, minio.StatObjectOptions{})
	if err != nil {
		return ObjectMetadata{}, err
	}
	return ObjectMetadata{Locator: locator, SizeBytes: info.Size, ContentType: info.ContentType, ETag: info.ETag, ModifiedAt: info.LastModified.UTC()}, nil
}
func (s *S3Storage) OpenRead(ctx context.Context, locator StorageLocator, byteRange *ByteRange) (io.ReadCloser, error) {
	if locator.Backend != "minio" {
		return nil, errors.New("wrong storage backend")
	}
	if err := validateKey(locator.ObjectKey); err != nil {
		return nil, err
	}
	if err := byteRange.Validate(); err != nil {
		return nil, err
	}
	options := minio.GetObjectOptions{}
	if byteRange != nil {
		if err := options.SetRange(byteRange.Start, byteRange.End); err != nil {
			return nil, err
		}
	}
	reader, err := s.client.Client.GetObject(ctx, s.bucket, locator.ObjectKey, options)
	return reader, err
}
func (s *S3Storage) PutStaged(ctx context.Context, scope Scope, reader io.Reader, contentType string) (StagedObject, error) {
	key, err := scopedKey(scope, "staging/objects/"+newRandomID())
	if err != nil {
		return StagedObject{}, err
	}
	hash := sha256.New()
	info, err := s.client.Client.PutObject(ctx, s.bucket, key, io.TeeReader(reader, hash), -1, minio.PutObjectOptions{ContentType: contentType, PartSize: 8 << 20})
	if err != nil {
		return StagedObject{}, err
	}
	return StagedObject{Locator: StorageLocator{Backend: "minio", ObjectKey: key}, SizeBytes: info.Size, SHA256: hex.EncodeToString(hash.Sum(nil)), ContentType: contentType}, nil
}
func (s *S3Storage) Promote(ctx context.Context, staged StagedObject, finalKey string, preconditions Preconditions) (ObjectMetadata, error) {
	if staged.Locator.Backend != "minio" || validateKey(staged.Locator.ObjectKey) != nil {
		return ObjectMetadata{}, errors.New("invalid staged S3 object")
	}
	if err := validateKey(finalKey); err != nil {
		return ObjectMetadata{}, err
	}
	if preconditions.IfNoneMatch {
		if _, err := s.client.StatObject(ctx, s.bucket, finalKey, minio.StatObjectOptions{}); err == nil {
			return ObjectMetadata{}, errors.New("destination already exists")
		}
	}
	_, err := s.client.CopyObject(ctx, s.bucket, staged.Locator.ObjectKey, s.bucket, finalKey, nil, minio.CopySrcOptions{}, minio.PutObjectOptions{})
	if err != nil {
		return ObjectMetadata{}, err
	}
	return s.Stat(ctx, StorageLocator{Backend: "minio", ObjectKey: finalKey})
}
func (s *S3Storage) Materialize(ctx context.Context, locator StorageLocator, cacheScope string) (LocalHandle, error) {
	if locator.Backend != "minio" {
		return LocalHandle{}, errors.New("wrong storage backend")
	}
	if err := validateKey(locator.ObjectKey); err != nil {
		return LocalHandle{}, err
	}
	file, err := os.CreateTemp("", "nh-media-"+filepath.Base(cacheScope)+"-*")
	if err != nil {
		return LocalHandle{}, err
	}
	reader, err := s.client.Client.GetObject(ctx, s.bucket, locator.ObjectKey, minio.GetObjectOptions{})
	if err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return LocalHandle{}, err
	}
	if _, err := io.Copy(file, reader); err != nil {
		_ = reader.Close()
		_ = file.Close()
		_ = os.Remove(file.Name())
		return LocalHandle{}, err
	}
	_ = reader.Close()
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return LocalHandle{}, err
	}
	return LocalHandle{path: file.Name()}, nil
}
func (s *S3Storage) PresignDownload(ctx context.Context, locator StorageLocator, ttl time.Duration, disposition string) (SignedRequest, error) {
	if locator.Backend != "minio" {
		return SignedRequest{}, errors.New("wrong storage backend")
	}
	if err := validateKey(locator.ObjectKey); err != nil {
		return SignedRequest{}, err
	}
	if ttl <= 0 || ttl > 15*time.Minute {
		return SignedRequest{}, errors.New("download TTL must be 1..900 seconds")
	}
	signed, err := s.client.PresignedGetObject(ctx, s.bucket, locator.ObjectKey, ttl, url.Values{"response-content-disposition": []string{disposition}})
	if err != nil {
		return SignedRequest{}, err
	}
	return SignedRequest{Method: "GET", URL: signed.String(), ExpiresAt: time.Now().UTC().Add(ttl)}, nil
}
func (s *S3Storage) Delete(ctx context.Context, locator StorageLocator, _ Preconditions) error {
	if locator.Backend != "minio" {
		return errors.New("wrong storage backend")
	}
	if err := validateKey(locator.ObjectKey); err != nil {
		return err
	}
	return s.client.RemoveObject(ctx, s.bucket, locator.ObjectKey, minio.RemoveObjectOptions{VersionID: locator.ObjectVersion})
}
func (s *S3Storage) ListStaging(ctx context.Context, prefix string) ([]StorageLocator, error) {
	if err := validateKey(prefix); err != nil {
		return nil, err
	}
	var result []StorageLocator
	for object := range s.client.Client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return nil, object.Err
		}
		result = append(result, StorageLocator{Backend: "minio", ObjectKey: object.Key, ObjectVersion: object.VersionID})
	}
	return result, nil
}

func (s *S3Storage) WorkerBackend() string { return "minio" }

func (s *S3Storage) PresignWorkerUpload(ctx context.Context, key string, ttl time.Duration, contentType string) (SignedRequest, error) {
	if err := validateKey(key); err != nil || ttl <= 0 || ttl > 15*time.Minute || strings.TrimSpace(contentType) == "" {
		return SignedRequest{}, errors.New("worker upload request is invalid")
	}
	signed, err := s.client.Client.PresignedPutObject(ctx, s.bucket, key, ttl)
	if err != nil {
		return SignedRequest{}, err
	}
	return SignedRequest{Method: "PUT", URL: signed.String(), Headers: map[string]string{"Content-Type": contentType}, ExpiresAt: time.Now().UTC().Add(ttl)}, nil
}

func (s *S3Storage) VerifyWorkerStage(ctx context.Context, key, expectedSHA string, expectedSize int64, contentType string) (StagedObject, error) {
	return verifyWorkerStage(ctx, s, StorageLocator{Backend: "minio", ObjectKey: key}, expectedSHA, expectedSize, contentType)
}

func verifyWorkerStage(ctx context.Context, backend StoragePort, locator StorageLocator, expectedSHA string, expectedSize int64, contentType string) (StagedObject, error) {
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(expectedSHA) || expectedSize < 0 || strings.TrimSpace(contentType) == "" {
		return StagedObject{}, errors.New("worker stage metadata is invalid")
	}
	reader, err := backend.OpenRead(ctx, locator, nil)
	if err != nil {
		return StagedObject{}, err
	}
	hash := sha256.New()
	size, copyErr := io.Copy(hash, reader)
	closeErr := reader.Close()
	if copyErr != nil {
		return StagedObject{}, copyErr
	}
	if closeErr != nil {
		return StagedObject{}, closeErr
	}
	actualSHA := hex.EncodeToString(hash.Sum(nil))
	if size != expectedSize || actualSHA != expectedSHA {
		return StagedObject{}, errors.New("worker stage checksum or size mismatch")
	}
	return StagedObject{Locator: locator, SizeBytes: size, SHA256: actualSHA, ContentType: contentType}, nil
}
