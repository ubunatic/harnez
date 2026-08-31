// Package telemetry owns the shared tool-call storage layer consumed by
// `harnez rate` (call_type = internal, issue 117), `harnez exec` (call_type
// = shell, issue 118), and `harnez stats` (issue 120). See issue 116.
//
// Engine: modernc.org/sqlite (pure Go, no cgo). Issue 115 canary'd
// DuckDB first and found it NO-GO — a hard single-writer-per-file lock
// that fails a second concurrent writer's Open() outright, plus a cgo
// dependency this repo's Go conventions (docs/lang/Go.md) reject by
// default. Issue 116's own canary (see its "## Canary" section)
// confirmed modernc.org/sqlite clears the same bar DuckDB failed:
// CGO_ENABLED=0 builds, sub-20ms single-row inserts, and — critically —
// two concurrent writers against the same file queue via WAL mode +
// busy_timeout rather than erroring at connect time.
package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DefaultDBPath returns the default tool_calls database location,
// ~/.harnez/tool_catalog.sqlite.
func DefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("telemetry: resolve home dir: %w", err)
	}
	return filepath.Join(home, ".harnez", "tool_catalog.sqlite"), nil
}

// openRetries and openRetryDelay bound the cold-start retry loop in Open.
// See this ticket's "## Canary" finding: two processes racing to create
// the same brand-new db file can hit SQLITE_BUSY on the one-time
// journal_mode->WAL conversion even with busy_timeout set, because that
// conversion's lock class isn't covered by the pragma's own retry window.
// Retrying the whole Open+schema-create step is a much smaller mitigation
// than a hand-rolled flock/lockfile layer.
const (
	openRetries    = 10
	openRetryDelay = 20 * time.Millisecond
)

// DB wraps the underlying *sql.DB with the schema already ensured.
type DB struct {
	sql *sql.DB
}

// Open opens (creating parent directories and the schema if needed) the
// tool_calls SQLite database at path. WAL mode and a busy_timeout PRAGMA
// are set so concurrent writers from separate harnez processes queue
// rather than fail. Schema creation is idempotent (CREATE TABLE IF NOT
// EXISTS + indexes) and safe to run on every Open — no migration
// framework, per this repo's "just change the code" bias.
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("telemetry: create db dir %s: %w", dir, err)
		}
	}

	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"

	var lastErr error
	for attempt := 0; attempt < openRetries; attempt++ {
		sqlDB, err := sql.Open("sqlite", dsn)
		if err != nil {
			lastErr = err
		} else if _, err := sqlDB.Exec(schemaDDL); err != nil {
			sqlDB.Close()
			lastErr = fmt.Errorf("telemetry: create schema: %w", err)
		} else {
			return &DB{sql: sqlDB}, nil
		}
		time.Sleep(openRetryDelay)
	}
	return nil, fmt.Errorf("telemetry: open %s: %w", path, lastErr)
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.sql.Close()
}

// context is used for the query surface's default timeout when callers
// don't supply their own context.Context.
func defaultContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}
