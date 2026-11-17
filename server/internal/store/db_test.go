package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func legacy(t *testing.T, statements ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open legacy: %v", err)
	}
	defer db.Close()
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
	return path
}

func columnSet(t *testing.T, path string) map[string]bool {
	t.Helper()
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	columns := map[string]bool{}
	if err := db.WithTx(context.Background(), func(conn *sql.Conn) error {
		rows, err := conn.QueryContext(context.Background(), "PRAGMA table_info(projects)")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var cid, notNull, pk int
			var name, columnType string
			var defaultValue sql.NullString
			if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
				return err
			}
			columns[name] = true
		}
		return nil
	}); err != nil {
		t.Fatalf("columns: %v", err)
	}
	return columns
}

func TestOpenAddsBootstrapEnabledToLegacyProjectsTable(t *testing.T) {
	path := legacy(t,
		`CREATE TABLE projects (id TEXT PRIMARY KEY, title TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', created_at TEXT NOT NULL,
			reason_worker TEXT, reason_trigger TEXT, reason_started_at TEXT, reason_last_heartbeat_at TEXT)`,
		`INSERT INTO projects (id, title, created_at) VALUES ('proj_001', 'legacy', '2026-01-01T00:00:00Z')`,
	)

	if !columnSet(t, path)["bootstrap_enabled"] {
		t.Fatal("bootstrap_enabled not added")
	}
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.WithTx(context.Background(), func(conn *sql.Conn) error {
		var bootstrap int
		return conn.QueryRowContext(context.Background(), "SELECT bootstrap_enabled FROM projects WHERE id = 'proj_001'").Scan(&bootstrap)
	}); err != nil {
		t.Fatalf("read: %v", err)
	}
}

func TestOpenMapsDisabledBootstrapModeToFalse(t *testing.T) {
	path := legacy(t,
		`CREATE TABLE projects (id TEXT PRIMARY KEY, title TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active',
			bootstrap_mode TEXT NOT NULL DEFAULT 'auto', created_at TEXT NOT NULL,
			reason_worker TEXT, reason_trigger TEXT, reason_started_at TEXT, reason_last_heartbeat_at TEXT)`,
		`INSERT INTO projects (id, title, bootstrap_mode, created_at) VALUES ('proj_001', 'disabled', 'disabled', '2026-01-01T00:00:00Z')`,
		`INSERT INTO projects (id, title, bootstrap_mode, created_at) VALUES ('proj_002', 'enabled', 'enabled', '2026-01-01T00:00:00Z')`,
	)

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.WithTx(context.Background(), func(conn *sql.Conn) error {
		rows, err := conn.QueryContext(context.Background(), "SELECT id, bootstrap_enabled FROM projects ORDER BY id")
		if err != nil {
			return err
		}
		defer rows.Close()
		got := map[string]int{}
		for rows.Next() {
			var id string
			var enabled int
			if err := rows.Scan(&id, &enabled); err != nil {
				return err
			}
			got[id] = enabled
		}
		if len(got) != 2 || got["proj_001"] != 0 || got["proj_002"] != 1 {
			t.Fatalf("mapping = %v", got)
		}
		return nil
	}); err != nil {
		t.Fatalf("select: %v", err)
	}
}

func TestOpenAddsMaxStepsToLegacyProjectsTable(t *testing.T) {
	path := legacy(t,
		`CREATE TABLE projects (id TEXT PRIMARY KEY, title TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active',
			bootstrap_enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL)`,
		`INSERT INTO projects (id, title, created_at) VALUES ('proj_001', 'legacy', '2026-01-01T00:00:00Z')`,
	)

	if !columnSet(t, path)["max_steps"] {
		t.Fatal("max_steps not added")
	}
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.WithTx(context.Background(), func(conn *sql.Conn) error {
		var maxSteps sql.NullInt64
		return conn.QueryRowContext(context.Background(), "SELECT max_steps FROM projects WHERE id = 'proj_001'").Scan(&maxSteps)
	}); err != nil {
		t.Fatalf("read: %v", err)
	}
}
