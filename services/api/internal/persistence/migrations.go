package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Migration is an immutable, versioned SQL change. The checksum is deliberately
// calculated from the exact bytes on disk so an applied migration cannot be
// silently edited after it has reached a database.
type Migration struct {
	Version  int64
	Name     string
	SQL      string
	Checksum string
}

type MigrationRunner struct {
	DB             *sql.DB
	Directory      string
	AllowDirtyRead bool
}

func NewMigrationRunner(db *sql.DB, directory string) (*MigrationRunner, error) {
	if db == nil {
		return nil, errors.New("migration database is required")
	}
	if strings.TrimSpace(directory) == "" {
		return nil, errors.New("migration directory is required")
	}
	return &MigrationRunner{DB: db, Directory: directory}, nil
}

func (r *MigrationRunner) Load() ([]Migration, error) {
	entries, err := os.ReadDir(r.Directory)
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	result := make([]Migration, 0, len(entries))
	seen := map[int64]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("migration filename %q must start with numeric version", entry.Name())
		}
		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("migration filename %q has invalid version", entry.Name())
		}
		if _, ok := seen[version]; ok {
			return nil, fmt.Errorf("duplicate migration version %d", version)
		}
		seen[version] = struct{}{}
		path := filepath.Join(r.Directory, entry.Name())
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		digest := sha256.Sum256(contents)
		name := strings.TrimSuffix(parts[1], ".sql")
		result = append(result, Migration{Version: version, Name: name, SQL: string(contents), Checksum: hex.EncodeToString(digest[:])})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Version < result[j].Version })
	return result, nil
}

func (r *MigrationRunner) Apply(ctx context.Context) error {
	if err := r.ensureTable(ctx); err != nil {
		return err
	}
	migrations, err := r.Load()
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		var checksum string
		var dirty bool
		err := r.DB.QueryRowContext(ctx, `SELECT checksum, dirty FROM schema_migrations WHERE version = $1`, migration.Version).Scan(&checksum, &dirty)
		if err == nil {
			if dirty && !r.AllowDirtyRead {
				return fmt.Errorf("migration %d is dirty; repair the failed migration before continuing", migration.Version)
			}
			if checksum != migration.Checksum {
				return fmt.Errorf("migration %d checksum mismatch: database=%s filesystem=%s", migration.Version, checksum, migration.Checksum)
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("inspect migration %d: %w", migration.Version, err)
		}
		if _, err := r.DB.ExecContext(ctx, `INSERT INTO schema_migrations(version, name, checksum, dirty) VALUES ($1, $2, $3, true)`, migration.Version, migration.Name, migration.Checksum); err != nil {
			return fmt.Errorf("mark migration %d dirty: %w", migration.Version, err)
		}
		tx, err := r.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", migration.Version, err)
		}
		if _, err = tx.ExecContext(ctx, migration.SQL); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", migration.Version, err)
		}
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", migration.Version, err)
		}
		if _, err = r.DB.ExecContext(ctx, `UPDATE schema_migrations SET dirty = false, applied_at = now() WHERE version = $1 AND checksum = $2`, migration.Version, migration.Checksum); err != nil {
			return fmt.Errorf("mark migration %d applied: %w", migration.Version, err)
		}
	}
	return nil
}

// GrantAppRole is intentionally separate from Apply: the migration role owns
// schema changes, while the Product API role receives only data privileges.
func (r *MigrationRunner) GrantAppRole(ctx context.Context, role string) error {
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(role) {
		return errors.New("invalid application role")
	}
	quoted := `"` + role + `"`
	for _, statement := range []string{
		`GRANT USAGE ON SCHEMA public TO ` + quoted,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO ` + quoted,
		`GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO ` + quoted,
		`ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO ` + quoted,
		`ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO ` + quoted,
	} {
		if _, err := r.DB.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("grant application role: %w", err)
		}
	}
	return nil
}

func (r *MigrationRunner) ensureTable(ctx context.Context) error {
	_, err := r.DB.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version bigint PRIMARY KEY,
    name text NOT NULL,
    checksum char(64) NOT NULL,
    dirty boolean NOT NULL DEFAULT false,
    applied_at timestamptz
)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return nil
}

func OpenPostgres(ctx context.Context, dsn string) (*sql.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("postgres DSN is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}
