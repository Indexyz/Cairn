// Package store owns the SQLite schema and write transactions.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DefaultPath is where the database lives when no flag is given.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "cairn.db"
	}
	return filepath.Join(home, ".local", "share", "cairn", "cairn.db")
}

// DB is the schema owner. One connection serializes writers inside the process;
// BEGIN IMMEDIATE keeps concurrent processes from racing.
type DB struct {
	sql *sql.DB
}

var schema = []string{
	`CREATE TABLE IF NOT EXISTS settings (
		intent_timeout INTEGER NOT NULL DEFAULT 15,
		reason_timeout INTEGER NOT NULL DEFAULT 15,
		max_workers INTEGER NOT NULL DEFAULT 0,
		max_running_projects INTEGER NOT NULL DEFAULT 0,
		max_project_workers INTEGER NOT NULL DEFAULT 0
	)`,
	`INSERT OR IGNORE INTO settings (rowid, intent_timeout, reason_timeout) VALUES (1, 15, 15)`,
	`CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		bootstrap_enabled INTEGER NOT NULL DEFAULT 1,
		max_steps INTEGER,
		created_at TEXT NOT NULL,
		reason_worker TEXT,
		reason_trigger TEXT,
		reason_started_at TEXT,
		reason_last_heartbeat_at TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS facts (
		id TEXT NOT NULL,
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		description TEXT NOT NULL,
		PRIMARY KEY (id, project_id)
	)`,
	`CREATE TABLE IF NOT EXISTS intents (
		id TEXT NOT NULL,
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		to_fact_id TEXT,
		description TEXT NOT NULL,
		creator TEXT NOT NULL,
		worker TEXT,
		last_heartbeat_at TEXT,
		created_at TEXT NOT NULL,
		concluded_at TEXT,
		PRIMARY KEY (id, project_id)
	)`,
	`CREATE TABLE IF NOT EXISTS intent_sources (
		intent_id TEXT NOT NULL,
		project_id TEXT NOT NULL,
		fact_id TEXT NOT NULL,
		PRIMARY KEY (intent_id, project_id, fact_id),
		FOREIGN KEY (intent_id, project_id) REFERENCES intents(id, project_id) ON DELETE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS hints (
		id TEXT NOT NULL,
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		content TEXT NOT NULL,
		creator TEXT NOT NULL,
		created_at TEXT NOT NULL,
		PRIMARY KEY (id, project_id)
	)`,
	`CREATE TABLE IF NOT EXISTS models (
		name TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		task_types TEXT NOT NULL,
		max_running INTEGER NOT NULL,
		priority INTEGER NOT NULL,
		env TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS counters (
		name TEXT PRIMARY KEY,
		value INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE TABLE IF NOT EXISTS mcp_servers (
		name TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		command TEXT NOT NULL DEFAULT '',
		args TEXT NOT NULL DEFAULT '[]',
		env TEXT NOT NULL DEFAULT '{}',
		url TEXT NOT NULL DEFAULT '',
		headers TEXT NOT NULL DEFAULT '{}',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`INSERT OR IGNORE INTO counters (name, value) VALUES ('project', 0)`,
	`INSERT OR IGNORE INTO counters (name, value) VALUES ('notification_channel', 0)`,
	`INSERT OR IGNORE INTO counters (name, value) VALUES ('schedule', 0)`,
	`CREATE TABLE IF NOT EXISTS schedules (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		cron TEXT NOT NULL,
		timezone TEXT NOT NULL DEFAULT 'UTC',
		enabled INTEGER NOT NULL DEFAULT 1,
		title TEXT NOT NULL,
		origin TEXT NOT NULL,
		goal TEXT NOT NULL,
		bootstrap_enabled INTEGER NOT NULL DEFAULT 1,
		max_steps INTEGER,
		hints TEXT NOT NULL DEFAULT '[]',
		next_run_at TEXT,
		last_run_at TEXT,
		last_status TEXT,
		last_detail TEXT,
		last_project_id TEXT,
		skip_count INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS notification_channels (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		kind TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		events TEXT NOT NULL,
		config TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS scoped_counters (
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		kind TEXT NOT NULL,
		value INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (project_id, kind)
	)`,
}

// Open creates the database file, applies the schema and legacy migrations.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(15000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	raw.SetMaxOpenConns(1)
	raw.SetMaxIdleConns(1)
	db := &DB{sql: raw}
	if err := db.migrate(context.Background()); err != nil {
		raw.Close()
		return nil, err
	}
	return db, nil
}

// Close releases the database.
func (db *DB) Close() error { return db.sql.Close() }

func (db *DB) migrate(ctx context.Context) error {
	return db.WithTx(ctx, func(conn *sql.Conn) error {
		for _, statement := range schema {
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("schema: %w", err)
			}
		}
		columns := map[string]bool{}
		rows, err := conn.QueryContext(ctx, "PRAGMA table_info(projects)")
		if err != nil {
			return err
		}
		for rows.Next() {
			var cid, notNull, pk int
			var name, columnType string
			var defaultValue sql.NullString
			if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
				rows.Close()
				return err
			}
			columns[name] = true
		}
		rows.Close()
		if !columns["bootstrap_enabled"] {
			if _, err := conn.ExecContext(ctx, "ALTER TABLE projects ADD COLUMN bootstrap_enabled INTEGER NOT NULL DEFAULT 1"); err != nil {
				return err
			}
			if columns["bootstrap_mode"] {
				if _, err := conn.ExecContext(ctx, "UPDATE projects SET bootstrap_enabled = CASE WHEN bootstrap_mode = 'disabled' THEN 0 ELSE 1 END"); err != nil {
					return err
				}
			}
		}
		if !columns["max_steps"] {
			if _, err := conn.ExecContext(ctx, "ALTER TABLE projects ADD COLUMN max_steps INTEGER"); err != nil {
				return err
			}
		}
		if !columns["schedule_id"] {
			if _, err := conn.ExecContext(ctx, "ALTER TABLE projects ADD COLUMN schedule_id TEXT REFERENCES schedules(id) ON DELETE SET NULL"); err != nil {
				return err
			}
		}
		settingsColumns := map[string]bool{}
		settingsRows, err := conn.QueryContext(ctx, "PRAGMA table_info(settings)")
		if err != nil {
			return err
		}
		for settingsRows.Next() {
			var cid, notNull, pk int
			var name, columnType string
			var defaultValue sql.NullString
			if err := settingsRows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
				settingsRows.Close()
				return err
			}
			settingsColumns[name] = true
		}
		settingsRows.Close()
		for _, column := range []string{"max_workers", "max_running_projects", "max_project_workers"} {
			if settingsColumns[column] {
				continue
			}
			if _, err := conn.ExecContext(ctx, "ALTER TABLE settings ADD COLUMN "+column+" INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
		}
		return nil
	})
}

// WithTx runs fn inside a BEGIN IMMEDIATE transaction.
func (db *DB) WithTx(ctx context.Context, fn func(conn *sql.Conn) error) error {
	conn, err := db.sql.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	if err := fn(conn); err != nil {
		_, _ = conn.ExecContext(ctx, "ROLLBACK")
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		_, _ = conn.ExecContext(ctx, "ROLLBACK")
		return err
	}
	return nil
}

// Read runs fn in a transaction too, so reads never observe a half-written graph.
func (db *DB) Read(ctx context.Context, fn func(conn *sql.Conn) error) error {
	return db.WithTx(ctx, fn)
}
