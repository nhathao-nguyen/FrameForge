package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

var ErrDuplicateCanonicalRole = errors.New("canonical artifact role already exists")
var ErrChecksumMismatch = errors.New("artifact checksum mismatch")

type Artifact struct {
	ID          string
	ProjectID   string
	Kind        string
	Role        string
	Status      string
	Locator     storage.StorageLocator
	SizeBytes   int64
	SHA256      string
	ContentType string
	Metadata    map[string]any
	CommittedAt *time.Time
}

type Repository interface {
	PublishCommitted(context.Context, Artifact) (Artifact, error)
	FindCanonicalRole(context.Context, string, string) (Artifact, bool, error)
	MarkOrphan(context.Context, storage.StorageLocator, string) error
}

type CommitRequest struct {
	ProjectID      string
	Kind           string
	Role           string
	FinalKey       string
	ExpectedSHA256 string
	Staged         CommitStaged
	ContentType    string
	Metadata       map[string]any
	IfNoneMatch    bool
}

// Staged is intentionally a small wrapper. The storage adapter is the only
// place that can manufacture it; callers cannot fabricate a committed locator
// from a local path.
func NewStaged(object storage.StagedObject) CommitStaged { return CommitStaged{object: object} }

type CommitStaged struct{ object storage.StagedObject }

type CommitService struct {
	Storage    storage.StoragePort
	Repository Repository
}

func (s CommitService) Commit(ctx context.Context, request CommitRequest) (Artifact, error) {
	if s.Storage == nil || s.Repository == nil {
		return Artifact{}, errors.New("artifact commit dependencies are required")
	}
	if request.ProjectID == "" || request.Kind == "" || request.Role == "" {
		return Artifact{}, errors.New("artifact identity is required")
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(request.Staged.object.SHA256) {
		return Artifact{}, errors.New("staged checksum is required")
	}
	if request.ExpectedSHA256 != "" && request.ExpectedSHA256 != request.Staged.object.SHA256 {
		return Artifact{}, ErrChecksumMismatch
	}
	if existing, ok, err := s.Repository.FindCanonicalRole(ctx, request.ProjectID, request.Role); err != nil {
		return Artifact{}, err
	} else if ok && request.IfNoneMatch {
		return existing, ErrDuplicateCanonicalRole
	}
	metadata, err := s.Storage.Promote(ctx, request.Staged.object, request.FinalKey, storage.Preconditions{IfNoneMatch: true})
	if err != nil {
		if !errors.Is(err, storage.ErrDestinationExists) {
			return Artifact{}, fmt.Errorf("promote artifact: %w", err)
		}
		// A controller may have promoted the immutable object and then
		// restarted before publishing its database row. Verify the object
		// bytes before treating the replay as an idempotent commit; an
		// unrelated object at the same key must never be adopted.
		metadata, err = verifyExistingObject(ctx, s.Storage, request.FinalKey, request.Staged.object, request.ContentType)
		if err != nil {
			return Artifact{}, fmt.Errorf("verify existing artifact: %w", err)
		}
	}
	value := Artifact{ProjectID: request.ProjectID, Kind: request.Kind, Role: request.Role, Status: "committed", Locator: metadata.Locator, SizeBytes: metadata.SizeBytes, SHA256: request.Staged.object.SHA256, ContentType: request.ContentType, Metadata: request.Metadata}
	now := time.Now().UTC()
	value.CommittedAt = &now
	committed, err := s.Repository.PublishCommitted(ctx, value)
	if err != nil {
		_ = s.Repository.MarkOrphan(ctx, metadata.Locator, "database publish failed")
		return Artifact{}, fmt.Errorf("publish artifact: %w", err)
	}
	return committed, nil
}

func verifyExistingObject(ctx context.Context, store storage.StoragePort, finalKey string, staged storage.StagedObject, contentType string) (storage.ObjectMetadata, error) {
	metadata, err := store.Stat(ctx, storage.StorageLocator{Backend: staged.Locator.Backend, ObjectKey: finalKey})
	if err != nil {
		return storage.ObjectMetadata{}, err
	}
	if metadata.SizeBytes != staged.SizeBytes {
		return storage.ObjectMetadata{}, fmt.Errorf("existing artifact size %d does not match staged size %d", metadata.SizeBytes, staged.SizeBytes)
	}
	reader, err := store.OpenRead(ctx, metadata.Locator, nil)
	if err != nil {
		return storage.ObjectMetadata{}, err
	}
	hash := sha256.New()
	readSize, copyErr := io.Copy(hash, reader)
	closeErr := reader.Close()
	if copyErr != nil {
		return storage.ObjectMetadata{}, copyErr
	}
	if closeErr != nil {
		return storage.ObjectMetadata{}, closeErr
	}
	if readSize != staged.SizeBytes {
		return storage.ObjectMetadata{}, fmt.Errorf("existing artifact read size %d does not match staged size %d", readSize, staged.SizeBytes)
	}
	if actual := hex.EncodeToString(hash.Sum(nil)); actual != staged.SHA256 {
		return storage.ObjectMetadata{}, errors.New("existing artifact checksum does not match staged checksum")
	}
	metadata.SHA256 = staged.SHA256
	if metadata.ContentType == "" {
		metadata.ContentType = contentType
	}
	return metadata, nil
}
