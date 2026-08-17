package persistence

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLoadMigrationsIsOrderedAndChecksummed(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "0002_second.sql"), []byte("select 2;"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "0001_first.sql"), []byte("select 1;"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner, err := NewMigrationRunner(nil, directory)
	if err == nil || runner != nil {
		t.Fatal("nil database must be rejected")
	}
	// Load is intentionally usable in a file-only review tool as well.
	runner = &MigrationRunner{Directory: directory}
	migrations, err := runner.Load()
	if err != nil || len(migrations) != 2 {
		t.Fatalf("load: %v %#v", err, migrations)
	}
	if migrations[0].Version != 1 || migrations[1].Version != 2 || migrations[0].Checksum == "" || migrations[0].Checksum == migrations[1].Checksum {
		t.Fatalf("unexpected migration plan: %#v", migrations)
	}
}

func TestLoadRejectsDuplicateVersions(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"0001_first.sql", "0001_duplicate.sql"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("select 1;"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runner := &MigrationRunner{Directory: directory}
	if _, err := runner.Load(); err == nil {
		t.Fatal("duplicate migration version was accepted")
	}
}

func TestApplyEmptyDirectoryIsSuccessful(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	directory := t.TempDir()
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).WillReturnResult(sqlmock.NewResult(0, 0))
	runner := &MigrationRunner{DB: db, Directory: directory}
	if err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyIsRepeatableAndMarksMigrationClean(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "0001_first.sql"), []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &MigrationRunner{DB: db, Directory: directory}
	checksum := "" // the exact value is asserted after the file plan is loaded.
	plan, err := runner.Load()
	if err != nil {
		t.Fatal(err)
	}
	checksum = plan[0].Checksum
	ensure := regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")
	selectExisting := regexp.QuoteMeta("SELECT checksum, dirty FROM schema_migrations WHERE version = $1")
	insertDirty := regexp.QuoteMeta("INSERT INTO schema_migrations(version, name, checksum, dirty) VALUES ($1, $2, $3, true)")
	updateClean := regexp.QuoteMeta("UPDATE schema_migrations SET dirty = false, applied_at = now() WHERE version = $1 AND checksum = $2")
	for _, existing := range []bool{false, true} {
		mock.ExpectExec(ensure).WillReturnResult(sqlmock.NewResult(0, 0))
		if existing {
			mock.ExpectQuery(selectExisting).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"checksum", "dirty"}).AddRow(checksum, false))
		} else {
			mock.ExpectQuery(selectExisting).WithArgs(int64(1)).WillReturnError(sql.ErrNoRows)
			mock.ExpectExec(insertDirty).WithArgs(int64(1), "first", checksum).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta("SELECT 1;")).WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectCommit()
			mock.ExpectExec(updateClean).WithArgs(int64(1), checksum).WillReturnResult(sqlmock.NewResult(0, 1))
		}
		if err := runner.Apply(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRefusesDirtyMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "0001_first.sql"), []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &MigrationRunner{DB: db, Directory: directory}
	plan, err := runner.Load()
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT checksum, dirty FROM schema_migrations WHERE version = $1")).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"checksum", "dirty"}).AddRow(plan[0].Checksum, true))
	if err := runner.Apply(context.Background()); err == nil {
		t.Fatal("dirty migration was accepted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyLeavesFailedMigrationDirtyForRepair(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "0001_broken.sql"), []byte("SELECT broken;"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &MigrationRunner{DB: db, Directory: directory}
	plan, err := runner.Load()
	if err != nil {
		t.Fatal(err)
	}
	ensure := regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")
	selectExisting := regexp.QuoteMeta("SELECT checksum, dirty FROM schema_migrations WHERE version = $1")
	insertDirty := regexp.QuoteMeta("INSERT INTO schema_migrations(version, name, checksum, dirty) VALUES ($1, $2, $3, true)")
	mock.ExpectExec(ensure).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(selectExisting).WithArgs(int64(1)).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(insertDirty).WithArgs(int64(1), "broken", plan[0].Checksum).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT broken;")).WillReturnError(errors.New("migration syntax failure"))
	mock.ExpectRollback()
	if err := runner.Apply(context.Background()); err == nil {
		t.Fatal("failed migration was accepted")
	}
	mock.ExpectExec(ensure).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(selectExisting).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"checksum", "dirty"}).AddRow(plan[0].Checksum, true))
	if err := runner.Apply(context.Background()); err == nil {
		t.Fatal("dirty migration was not retained")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGrantAppRoleRejectsSQLIdentifierInjection(t *testing.T) {
	runner := &MigrationRunner{DB: &sql.DB{}, Directory: t.TempDir()}
	if err := runner.GrantAppRole(context.Background(), `app_role; DROP TABLE users;`); err == nil {
		t.Fatal("unsafe application role was accepted")
	}
}
