// Package sqlite persists AUVC guild configuration and Discord player links.
//
// It replaces the upstream PostgreSQL and Redis storage that phase 14 removes.
// The driver is modernc.org/sqlite, a pure-Go implementation: the Docker image
// builds with CGO_ENABLED=0, so a cgo-based driver would not link there at all.
package sqlite

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// DB is an open AUVC database with its schema already migrated.
type DB struct {
	db *sql.DB
}

// Open opens the database at path, creating it if necessary, and brings the
// schema up to date. The caller owns the returned DB and must Close it.
//
// Use ":memory:" for tests.
func Open(path string) (*DB, error) {
	// Foreign keys are off by default in SQLite and busy_timeout avoids an
	// immediate SQLITE_BUSY when two goroutines write at once.
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	if path == ":memory:" {
		// WAL is meaningless for an in-memory database and shared cache keeps
		// every connection in the pool looking at the same one.
		dsn = "file::memory:?cache=shared&_pragma=foreign_keys(1)"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("reach sqlite database at %s: %w", path, err)
	}

	wrapped := &DB{db: db}
	if err := wrapped.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return wrapped, nil
}

// Close releases the database handle.
func (d *DB) Close() error {
	return d.db.Close()
}

// SchemaVersion reports the highest migration applied to this database.
func (d *DB) SchemaVersion() (int, error) {
	var version int
	err := d.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

type migration struct {
	version int
	name    string
	script  string
}

// migrate applies every migration this binary carries that the database has not
// seen yet, each in its own transaction, in version order.
//
// A database newer than the binary is refused rather than downgraded: silently
// running an old binary against a newer schema is how data gets corrupted.
func (d *DB) migrate() error {
	if _, err := d.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER NOT NULL PRIMARY KEY,
		name       TEXT    NOT NULL,
		applied_at INTEGER NOT NULL
	) STRICT`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	current, err := d.SchemaVersion()
	if err != nil {
		return err
	}

	available, err := loadMigrations()
	if err != nil {
		return err
	}

	if len(available) > 0 && current > available[len(available)-1].version {
		return fmt.Errorf(
			"database schema version %d is newer than this build knows (%d); "+
				"upgrade AUVC instead of downgrading the database",
			current, available[len(available)-1].version)
	}

	for _, m := range available {
		if m.version <= current {
			continue
		}
		if err := d.apply(m); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) apply(m migration) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", m.version, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(m.script); err != nil {
		return fmt.Errorf("apply migration %d (%s): %w", m.version, m.name, err)
	}
	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, unixepoch())",
		m.version, m.name,
	); err != nil {
		return fmt.Errorf("record migration %d: %w", m.version, err)
	}
	return tx.Commit()
}

// loadMigrations reads the embedded scripts. File names must start with a
// zero-padded version and an underscore, for example 0001_initial_schema.sql.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	migrations := make([]migration, 0, len(entries))
	seen := map[int]string{}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		prefix, rest, found := strings.Cut(entry.Name(), "_")
		if !found {
			return nil, fmt.Errorf("migration %q has no version prefix", entry.Name())
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migration %q has a non-numeric version prefix: %w", entry.Name(), err)
		}
		if version < 1 {
			return nil, fmt.Errorf("migration %q must have a version of at least 1", entry.Name())
		}
		if other, clash := seen[version]; clash {
			return nil, fmt.Errorf("migrations %q and %q share version %d", other, entry.Name(), version)
		}
		seen[version] = entry.Name()

		script, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}

		migrations = append(migrations, migration{
			version: version,
			name:    strings.TrimSuffix(rest, ".sql"),
			script:  string(script),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})
	return migrations, nil
}

// ErrNotFound is returned when a lookup has no row.
var ErrNotFound = errors.New("not found")
