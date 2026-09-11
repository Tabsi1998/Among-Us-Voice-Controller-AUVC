package sqlite

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openTemp(t *testing.T) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "amongus.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

// latestMigration is derived from the embedded scripts rather than written
// down, so adding a migration does not mean editing a number in two places
// and the test keeps asserting what it means: every migration is applied.
func latestMigration(t *testing.T) int {
	t.Helper()

	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("no migrations are embedded")
	}
	return migrations[len(migrations)-1].version
}

func TestOpenAppliesMigrations(t *testing.T) {
	db, _ := openTemp(t)

	version, err := db.SchemaVersion()
	if err != nil {
		t.Fatalf("schema version: %v", err)
	}
	if want := latestMigration(t); version != want {
		t.Errorf("schema version = %d, want %d", version, want)
	}
}

func TestOpenCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "data", "amongus.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open nested database: %v", err)
	}
	db.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file was not created: %v", err)
	}
}

func TestOpenRejectsAnEmptyPath(t *testing.T) {
	if _, err := Open("   "); err == nil {
		t.Fatal("expected an empty database path to fail")
	}
}

// Opening an existing database must not try to re-run migrations it already
// applied, or every restart would fail on "table already exists".
func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "amongus.db")

	for i := 0; i < 3; i++ {
		db, err := Open(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		version, err := db.SchemaVersion()
		if err != nil {
			t.Fatalf("schema version %d: %v", i, err)
		}
		if want := latestMigration(t); version != want {
			t.Errorf("open %d: schema version = %d, want %d", i, version, want)
		}
		db.Close()
	}
}

// Running an old binary against a newer database silently is how data gets
// corrupted, so it must be refused outright.
func TestOpenRefusesANewerDatabase(t *testing.T) {
	db, path := openTemp(t)

	if _, err := db.db.Exec(
		"INSERT INTO schema_migrations (version, name, applied_at) VALUES (999, 'from the future', unixepoch())",
	); err != nil {
		t.Fatalf("seed future migration: %v", err)
	}
	db.Close()

	reopened, err := Open(path)
	if err == nil {
		reopened.Close()
		t.Fatal("expected opening a newer database to fail")
	}
	if !strings.Contains(err.Error(), "newer than this build") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMigrationFilesAreWellFormed(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("no migrations were embedded")
	}
	for i, m := range migrations {
		if m.version != i+1 {
			t.Errorf("migration %d has version %d; versions must be gapless and start at 1", i, m.version)
		}
		if m.script == "" {
			t.Errorf("migration %d (%s) is empty", m.version, m.name)
		}
	}
}

func TestErrNotFoundIsMatchable(t *testing.T) {
	db, _ := openTemp(t)

	_, err := db.GuildConfig("absent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
