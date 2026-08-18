package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/inventory"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/persistence"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

func main() {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv("NH_MEDIA_DATABASE_URL"))
	if dsn == "" {
		fail("NH_MEDIA_DATABASE_URL is required")
	}
	db, err := persistence.OpenPostgres(ctx, dsn)
	if err != nil {
		fail(err.Error())
	}
	defer db.Close()
	store, err := loadStorage()
	if err != nil {
		fail(err.Error())
	}
	port, ok := store.(storage.InventoryPort)
	if !ok {
		fail("configured storage does not support inventory")
	}
	expected, err := loadExpected(ctx, db)
	if err != nil {
		fail(err.Error())
	}
	observed, err := port.Inventory(ctx, "workspaces")
	if err != nil {
		fail(err.Error())
	}
	objects := make([]inventory.Object, 0, len(observed))
	for _, value := range observed {
		objects = append(objects, inventory.Object{Backend: value.Locator.Backend, ObjectKey: value.Locator.ObjectKey, SHA256: value.SHA256, SizeBytes: value.SizeBytes})
	}
	report := inventory.Compare(expected, objects)
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fail(err.Error())
	}
	if !report.Passed && strings.EqualFold(os.Getenv("NH_MEDIA_INVENTORY_STRICT"), "1") {
		os.Exit(2)
	}
}

func loadExpected(ctx context.Context, db *sql.DB) ([]inventory.ExpectedArtifact, error) {
	rows, err := db.QueryContext(ctx, `SELECT storage_backend,object_key,sha256,size_bytes,role FROM artifacts WHERE status='committed' ORDER BY object_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]inventory.ExpectedArtifact, 0)
	for rows.Next() {
		var value inventory.ExpectedArtifact
		if err := rows.Scan(&value.Backend, &value.ObjectKey, &value.SHA256, &value.SizeBytes, &value.Role); err != nil {
			return nil, err
		}
		value.RetainUntilDelete = value.Role != "scratch"
		values = append(values, value)
	}
	return values, rows.Err()
}

func loadStorage() (storage.StoragePort, error) {
	backend := strings.ToLower(strings.TrimSpace(os.Getenv("NH_STORAGE_BACKEND")))
	if backend == "local" {
		return storage.NewLocalStorage(os.Getenv("NH_STORAGE_LOCAL_ROOT"), os.Getenv("NH_STORAGE_CACHE_ROOT"))
	}
	endpoint := strings.TrimSpace(os.Getenv("NH_STORAGE_ENDPOINT"))
	if endpoint == "" {
		return nil, fmt.Errorf("NH_STORAGE_ENDPOINT is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid NH_STORAGE_ENDPOINT")
	}
	secure := parsed.Scheme == "https"
	if parsed.Scheme != "http" && !secure {
		return nil, fmt.Errorf("storage endpoint must use http or https")
	}
	return storage.NewS3Storage(parsed.Host, os.Getenv("NH_STORAGE_ACCESS_KEY"), os.Getenv("NH_STORAGE_SECRET_KEY"), os.Getenv("NH_STORAGE_BUCKET"), secure)
}

func fail(message string) { fmt.Fprintln(os.Stderr, "artifact inventory: "+message); os.Exit(1) }
