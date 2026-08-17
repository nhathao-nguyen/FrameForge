package main

import (
	"testing"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

func TestLoadStorageRejectsAmbiguousDurableConfiguration(t *testing.T) {
	env := func(name string) string {
		if name == "NH_STORAGE_BACKEND" {
			return "s3"
		}
		return ""
	}
	if _, err := loadStorage(env); err == nil {
		t.Fatal("S3 storage without endpoint was accepted")
	}
	env = func(name string) string {
		if name == "NH_STORAGE_BACKEND" {
			return "unknown"
		}
		return ""
	}
	if _, err := loadStorage(env); err == nil {
		t.Fatal("unknown storage backend was accepted")
	}
}

func TestLoadStorageLocalUsesExplicitRoot(t *testing.T) {
	root := t.TempDir()
	backend, err := loadStorage(func(name string) string {
		switch name {
		case "NH_STORAGE_BACKEND":
			return "local"
		case "NH_STORAGE_LOCAL_ROOT":
			return root
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := backend.(*storage.LocalStorage); !ok {
		t.Fatalf("unexpected storage type %T", backend)
	}
}
