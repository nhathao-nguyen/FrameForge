package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/minio/minio-go/v7"
)

type InventoryEntry struct {
	Locator    StorageLocator `json:"locator"`
	SizeBytes  int64          `json:"size_bytes"`
	SHA256     string         `json:"sha256"`
	ModifiedAt string         `json:"modified_at"`
}

type InventoryPort interface {
	Inventory(context.Context, string) ([]InventoryEntry, error)
}

func (s *LocalStorage) Inventory(_ context.Context, prefix string) ([]InventoryEntry, error) {
	if err := validateKey(prefix); err != nil {
		return nil, err
	}
	root := filepath.Join(s.root, filepath.FromSlash(prefix))
	entries := make([]InventoryEntry, 0)
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
		locator := StorageLocator{Backend: "local", ObjectKey: filepath.ToSlash(rel)}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		digest, hashErr := digestReader(file)
		closeErr := file.Close()
		if hashErr != nil {
			return hashErr
		}
		if closeErr != nil {
			return closeErr
		}
		entries = append(entries, InventoryEntry{Locator: locator, SizeBytes: info.Size(), SHA256: digest, ModifiedAt: info.ModTime().UTC().Format("2006-01-02T15:04:05.999999999Z07:00")})
		return nil
	})
	if os.IsNotExist(err) {
		return []InventoryEntry{}, nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Locator.ObjectKey < entries[j].Locator.ObjectKey })
	return entries, err
}

func (s *S3Storage) Inventory(ctx context.Context, prefix string) ([]InventoryEntry, error) {
	if err := validateKey(prefix); err != nil {
		return nil, err
	}
	if s == nil || s.apiClient == nil {
		return nil, os.ErrInvalid
	}
	entries := make([]InventoryEntry, 0)
	for object := range s.apiClient.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return nil, object.Err
		}
		reader, err := s.OpenRead(ctx, StorageLocator{Backend: "minio", ObjectKey: object.Key}, nil)
		if err != nil {
			return nil, err
		}
		digest, hashErr := digestReader(reader)
		closeErr := reader.Close()
		if hashErr != nil {
			return nil, hashErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		entries = append(entries, InventoryEntry{Locator: StorageLocator{Backend: "minio", ObjectKey: object.Key, ObjectVersion: object.VersionID}, SizeBytes: object.Size, SHA256: digest, ModifiedAt: object.LastModified.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Locator.ObjectKey < entries[j].Locator.ObjectKey })
	return entries, nil
}

func digestReader(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *S3Storage) inventoryPrefix(prefix string) string { return strings.Trim(prefix, "/") }
