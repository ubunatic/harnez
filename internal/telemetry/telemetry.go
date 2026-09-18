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
	sql        *sql.DB
	migrations []string
}

// Open opens (creating parent directories and the schema if needed) the
// tool_calls SQLite database at path. WAL mode and a busy_timeout PRAGMA
// are set so concurrent writers from separate harnez processes queue
// rather than fail. Schema creation is idempotent (CREATE TABLE IF NOT
// EXISTS + indexes) and safe to run on every Open — no migration
// framework, per this repo's "just change the code" bias. If the file
// already exists with an older schemaVersion (CREATE ... IF NOT EXISTS
// can't apply a shape change to it), Open fails with a clear message
// rather than an inserting caller hitting a confusing constraint error
// later — see schema.go's schemaVersion doc comment.
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
		} else if tableExisted, err := tableExists(sqlDB, "tool_calls"); err != nil {
			sqlDB.Close()
			lastErr = fmt.Errorf("telemetry: check existing schema: %w", err)
		} else if _, err := sqlDB.Exec(schemaDDL); err != nil {
			sqlDB.Close()
			lastErr = fmt.Errorf("telemetry: create schema: %w", err)
		} else if migrations, err := checkAndMigrateSchema(sqlDB, path, tableExisted); err != nil {
			sqlDB.Close()
			return nil, err
		} else {
			return &DB{sql: sqlDB, migrations: migrations}, nil
		}
		time.Sleep(openRetryDelay)
	}
	return nil, fmt.Errorf("telemetry: open %s: %w", path, lastErr)
}

func migrateV2ToV3(sqlDB *sql.DB) error {
	rows, err := sqlDB.Query("PRAGMA table_info(tool_calls)")
	if err != nil {
		return err
	}
	defer rows.Close()
	columns := make(map[string]bool)
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, column := range []string{"output_bytes", "actual_tokens", "input_tokens", "cached_input_tokens", "output_tokens", "reasoning_tokens", "total_tokens", "potential_savings_tokens", "potential_savings_bytes"} {
		if !columns[column] {
			if _, err := sqlDB.Exec("ALTER TABLE tool_calls ADD COLUMN " + column + " INTEGER"); err != nil {
				return err
			}
		}
	}
	return nil
}

// tableExists reports whether name already exists in the database, checked
// BEFORE running schemaDDL's CREATE TABLE IF NOT EXISTS — this is what lets
// checkAndMigrateSchema tell "genuinely brand-new file, this Open call
// just created the table with the current shape" apart from "a table that
// already existed, for any reason, before this call."
func tableExists(sqlDB *sql.DB, name string) (bool, error) {
	var n int
	err := sqlDB.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// checkAndMigrateSchema reads SQLite's built-in PRAGMA user_version,
// applies version-guarded discrete migrations when current < schemaVersion,
// and stamps user_version to current.
func checkAndMigrateSchema(sqlDB *sql.DB, path string, preexisting bool) ([]string, error) {
	var migrations []string
	var current int
	if err := sqlDB.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return nil, fmt.Errorf("telemetry: read schema version: %w", err)
	}
	if !preexisting {
		// This Open call itself just created the table via schemaDDL, so
		// it's unconditionally current-shape — stamp regardless of
		// whatever user_version happened to read (normally 0).
		if _, err := sqlDB.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
			return nil, fmt.Errorf("telemetry: stamp schema version: %w", err)
		}
		return migrations, nil
	}
	if current < schemaVersion {
		if current < 4 {
			if err := migrateV2ToV3(sqlDB); err != nil {
				return nil, fmt.Errorf("telemetry: migrate schema to v3: %w", err)
			}
			migrations = append(migrations, "tool_calls token columns")
		}
		compactionEventsMigrated, err := migrateCompactionEvents(sqlDB)
		if err != nil {
			return nil, fmt.Errorf("telemetry: migrate compaction events: %w", err)
		}
		if compactionEventsMigrated {
			migrations = append(migrations, "compaction_events.model")
		}
		// Version 5 is additive: schemaDDL creates compaction_events and
		// session_boundaries for existing databases before this check.
		if _, err := sqlDB.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
			return nil, fmt.Errorf("telemetry: stamp migrated schema version: %w", err)
		}
	}
	return migrations, nil
}

func migrateCompactionEvents(sqlDB *sql.DB) (bool, error) {
	rows, err := sqlDB.Query("PRAGMA table_info(compaction_events)")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	modelPresent := false
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == "model" {
			modelPresent = true
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if !modelPresent {
		if _, err = sqlDB.Exec("ALTER TABLE compaction_events ADD COLUMN model TEXT NOT NULL DEFAULT ''"); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.sql.Close()
}

// ValidateSchema confirms that the database has the current version and the
// tables and columns required by telemetry writers.
func (d *DB) ValidateSchema() error {
	var version int
	if err := d.sql.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("telemetry: read schema version: %w", err)
	}
	if version != schemaVersion {
		return fmt.Errorf("telemetry: schema version = %d, want %d", version, schemaVersion)
	}

	required := map[string][]string{
		"compaction_events":    {"model"},
		"session_boundaries":   nil,
		"token_snapshots":      nil,
		"compaction_economics": {"model", "pricing_revision", "status"},
	}
	for table, columns := range required {
		var present int
		if err := d.sql.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&present); err != nil {
			return fmt.Errorf("telemetry: check table %s: %w", table, err)
		}
		if present != 1 {
			return fmt.Errorf("telemetry: schema missing table %s", table)
		}
		for _, column := range columns {
			if err := d.sql.QueryRow("SELECT count(*) FROM pragma_table_info(?) WHERE name = ?", table, column).Scan(&present); err != nil {
				return fmt.Errorf("telemetry: check %s.%s: %w", table, column, err)
			}
			if present != 1 {
				return fmt.Errorf("telemetry: schema missing column %s.%s", table, column)
			}
		}
	}
	return nil
}

// SchemaVersion returns the current telemetry schema version.
func (d *DB) SchemaVersion() (int, error) {
	var version int
	if err := d.sql.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("telemetry: read schema version: %w", err)
	}
	return version, nil
}

// Migrations returns the migrations applied while opening the database.
func (d *DB) Migrations() []string { return append([]string(nil), d.migrations...) }

// context is used for the query surface's default timeout when callers
// don't supply their own context.Context.
func defaultContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}
