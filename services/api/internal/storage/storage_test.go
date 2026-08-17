package storage

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestLocalStorageRejectsTraversalAndPreservesRangeSemantics(t *testing.T) {
	store, err := NewLocalStorage(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	scope := Scope{WorkspaceID: "ws_abc", ProjectID: "proj_abc"}
	if _, err := store.PutStaged(context.Background(), scope, strings.NewReader("payload"), "text/plain"); err != nil {
		t.Fatal(err)
	}
	staged, err := store.PutStaged(context.Background(), scope, strings.NewReader("0123456789"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Promote(context.Background(), staged, "workspaces/ws_abc/projects/proj_abc/artifacts/a1", Preconditions{IfNoneMatch: true}); err != nil {
		t.Fatal(err)
	}
	reader, err := store.OpenRead(context.Background(), StorageLocator{Backend: "local", ObjectKey: "workspaces/ws_abc/projects/proj_abc/artifacts/a1"}, &ByteRange{Start: 2, End: 5})
	if err != nil {
		t.Fatal(err)
	}
	value, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "2345" {
		t.Fatalf("range: %q", value)
	}
	for _, key := range []string{"../escape", "/absolute", `C:\\escape`, "workspaces/ws_abc/projects/proj_abc/../escape", "workspaces/ws_abc/projects/proj_abc/a\tb"} {
		if _, err := store.Stat(context.Background(), StorageLocator{Backend: "local", ObjectKey: key}); err == nil {
			t.Fatalf("unsafe key accepted: %q", key)
		}
	}
}

func TestLocalStorageStagingIsScopedAndIdempotentDelete(t *testing.T) {
	store, err := NewLocalStorage(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.InitiateUpload(context.Background(), Scope{WorkspaceID: "ws_abc", ProjectID: "proj_abc"}, UploadConstraints{Multipart: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), StorageLocator{Backend: "local", ObjectKey: "workspaces/ws_abc/projects/proj_abc/staging/missing"}, Preconditions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InitiateUpload(context.Background(), Scope{WorkspaceID: "", ProjectID: "proj_abc"}, UploadConstraints{}); err == nil {
		t.Fatal("unscoped upload accepted")
	}
}
