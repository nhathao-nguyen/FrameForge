package artifact

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

type memoryRepository struct {
	existing   *Artifact
	publishErr error
	published  []Artifact
	orphaned   []storage.StorageLocator
}

func (r *memoryRepository) FindCanonicalRole(_ context.Context, _, _ string) (Artifact, bool, error) {
	if r.existing == nil {
		return Artifact{}, false, nil
	}
	return *r.existing, true, nil
}
func (r *memoryRepository) PublishCommitted(_ context.Context, value Artifact) (Artifact, error) {
	if r.publishErr != nil {
		return Artifact{}, r.publishErr
	}
	r.published = append(r.published, value)
	return value, nil
}
func (r *memoryRepository) MarkOrphan(_ context.Context, locator storage.StorageLocator, _ string) error {
	r.orphaned = append(r.orphaned, locator)
	return nil
}

func TestCommitPromotesAndPublishesCanonicalArtifact(t *testing.T) {
	store, err := storage.NewLocalStorage(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	staged, err := store.PutStaged(context.Background(), storage.Scope{WorkspaceID: "ws_1", ProjectID: "proj_1"}, strings.NewReader("media"), "video/mp4")
	if err != nil {
		t.Fatal(err)
	}
	checksum := staged.SHA256
	repo := &memoryRepository{}
	value, err := (CommitService{Storage: store, Repository: repo}).Commit(context.Background(), CommitRequest{
		ProjectID: "proj_1", Kind: "video", Role: "source", FinalKey: "workspaces/ws_1/projects/proj_1/artifacts/source",
		ExpectedSHA256: checksum, Staged: NewStaged(staged), ContentType: "video/mp4", IfNoneMatch: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if value.Status != "committed" || len(repo.published) != 1 || len(repo.orphaned) != 0 {
		t.Fatalf("unexpected commit result: %#v %#v", value, repo)
	}
	if _, err := store.Stat(context.Background(), value.Locator); err != nil {
		t.Fatalf("promoted artifact is not readable: %v", err)
	}
}

func TestCommitRejectsDuplicateAndMarksOrphanOnPublishFailure(t *testing.T) {
	store, err := storage.NewLocalStorage(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	staged, err := store.PutStaged(context.Background(), storage.Scope{WorkspaceID: "ws_1", ProjectID: "proj_1"}, strings.NewReader("media"), "video/mp4")
	if err != nil {
		t.Fatal(err)
	}
	existing := Artifact{ID: "a_existing", ProjectID: "proj_1", Role: "source", Status: "committed"}
	_, err = (CommitService{Storage: store, Repository: &memoryRepository{existing: &existing}}).Commit(context.Background(), CommitRequest{
		ProjectID: "proj_1", Kind: "video", Role: "source", FinalKey: "workspaces/ws_1/projects/proj_1/artifacts/source",
		ExpectedSHA256: staged.SHA256, Staged: NewStaged(staged), IfNoneMatch: true,
	})
	if !errors.Is(err, ErrDuplicateCanonicalRole) {
		t.Fatalf("duplicate canonical role was accepted: %v", err)
	}
	repo := &memoryRepository{publishErr: errors.New("database unavailable")}
	_, err = (CommitService{Storage: store, Repository: repo}).Commit(context.Background(), CommitRequest{
		ProjectID: "proj_1", Kind: "video", Role: "derived", FinalKey: "workspaces/ws_1/projects/proj_1/artifacts/derived",
		ExpectedSHA256: staged.SHA256, Staged: NewStaged(staged), IfNoneMatch: true,
	})
	if err == nil || len(repo.orphaned) != 1 {
		t.Fatalf("publish failure was not compensated: err=%v orphaned=%d", err, len(repo.orphaned))
	}
}

func TestCommitReusesMatchingObjectAfterPromotionReplay(t *testing.T) {
	store, err := storage.NewLocalStorage(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.PutStaged(context.Background(), storage.Scope{WorkspaceID: "ws_1", ProjectID: "proj_1"}, strings.NewReader("media"), "video/mp4")
	if err != nil {
		t.Fatal(err)
	}
	finalKey := "workspaces/ws_1/projects/proj_1/jobs/job_1/steps/step_1/outputs/output_1"
	if _, err := store.Promote(context.Background(), first, finalKey, storage.Preconditions{IfNoneMatch: true}); err != nil {
		t.Fatal(err)
	}
	// The second delivery has a distinct staging object but the same
	// deterministic output. It must publish the already-promoted bytes instead
	// of failing on the immutable final-key precondition.
	second, err := store.PutStaged(context.Background(), storage.Scope{WorkspaceID: "ws_1", ProjectID: "proj_1"}, strings.NewReader("media"), "video/mp4")
	if err != nil {
		t.Fatal(err)
	}
	repo := &memoryRepository{}
	value, err := (CommitService{Storage: store, Repository: repo}).Commit(context.Background(), CommitRequest{
		ProjectID: "proj_1", Kind: "video", Role: "render_video", FinalKey: finalKey,
		ExpectedSHA256: second.SHA256, Staged: NewStaged(second), ContentType: "video/mp4",
	})
	if err != nil {
		t.Fatal(err)
	}
	if value.Status != "committed" || value.SHA256 != second.SHA256 || len(repo.published) != 1 {
		t.Fatalf("unexpected replay commit: %#v %#v", value, repo)
	}
}
