package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
)

// maxLegacyAttach keeps one merge below SQLite's default limit of 10
// attached databases.
const maxLegacyAttach = 9

// LegacyMergeResult reports the rows one legacy file added.
type LegacyMergeResult struct {
	Source string
	Added  int64
}

// LegacyToolCallsMissing counts the rows in the SQLite file at legacyPath
// that this database does not have yet (issue 735). A file without a
// tool_calls table has none.
func (d *DB) LegacyToolCallsMissing(ctx context.Context, legacyPath string) (int64, error) {
	var missing int64
	err := d.withLegacy(ctx, []string{legacyPath}, func(conn *sql.Conn, attached []attachedLegacy) error {
		for _, a := range attached {
			if len(a.columns) == 0 {
				continue
			}
			if err := conn.QueryRowContext(ctx, legacySQL("legacy_tool_calls_missing", a.alias)).Scan(&missing); err != nil {
				return err
			}
		}
		return nil
	})
	return missing, err
}

// MergeLegacyToolCalls copies the tool_calls rows of the SQLite files at
// legacyPaths that this database does not have yet, all in one transaction
// (issue 735: the XDG migration renamed a legacy database to a backup
// without copying its rows when the new database already existed). Only
// columns both tables share are copied; the rest take their defaults.
func (d *DB) MergeLegacyToolCalls(ctx context.Context, legacyPaths []string) ([]LegacyMergeResult, error) {
	results := make([]LegacyMergeResult, len(legacyPaths))
	err := d.withLegacy(ctx, legacyPaths, func(conn *sql.Conn, attached []attachedLegacy) error {
		if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
			return err
		}
		// Roll back with a fresh context so a cancelled ctx or failed COMMIT
		// never returns the pinned connection with the transaction open.
		committed := false
		defer func() {
			if !committed {
				_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
			}
		}()
		for i, a := range attached {
			results[i].Source = legacyPaths[i]
			if len(a.columns) == 0 {
				continue
			}
			query := strings.ReplaceAll(legacySQL("legacy_tool_calls_merge", a.alias), "{{columns}}", strings.Join(a.columns, ", "))
			res, err := conn.ExecContext(ctx, query)
			if err == nil {
				results[i].Added, err = res.RowsAffected()
			}
			if err != nil {
				return fmt.Errorf("merge %s: %w", legacyPaths[i], err)
			}
		}
		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
			return err
		}
		committed = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

type attachedLegacy struct {
	alias   string
	columns []string
}

func legacySQL(name, alias string) string {
	return strings.ReplaceAll(mustTelemetrySQL().Statements[name], "{{legacy}}", alias)
}

// withLegacy attaches each path as schema legacy_<i> on a pinned connection
// and passes the tool_calls columns each shares with the live table to fn
// (none when the file has no tool_calls table).
func (d *DB) withLegacy(ctx context.Context, paths []string, fn func(*sql.Conn, []attachedLegacy) error) error {
	if len(paths) > maxLegacyAttach {
		return fmt.Errorf("telemetry: merge at most %d legacy files at once, got %d", maxLegacyAttach, len(paths))
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("telemetry: legacy database: %w", err)
		}
	}
	conn, err := d.sql.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	attached := make([]attachedLegacy, 0, len(paths))
	defer func() {
		for _, a := range attached {
			_, _ = conn.ExecContext(context.Background(), legacySQL("legacy_detach", a.alias))
		}
	}()
	for i, path := range paths {
		alias := fmt.Sprintf("legacy_%d", i)
		if _, err := conn.ExecContext(ctx, legacySQL("legacy_attach", alias), path); err != nil {
			return fmt.Errorf("telemetry: attach %s: %w", path, err)
		}
		a := attachedLegacy{alias: alias}
		attached = append(attached, a)
		rows, err := conn.QueryContext(ctx, legacySQL("legacy_tool_call_columns", alias))
		if err != nil {
			return err
		}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return err
			}
			a.columns = append(a.columns, name)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		attached[i] = a
	}
	return fn(conn, attached)
}

// ToolCallCount returns the number of tool_calls rows.
func (d *DB) ToolCallCount(ctx context.Context) (int64, error) {
	var n int64
	err := d.sql.QueryRowContext(ctx, mustTelemetrySQL().Statements["tool_call_count"]).Scan(&n)
	return n, err
}

// BackupInto writes a consistent copy of the live database to path with
// VACUUM INTO, which reads inside one transaction and so is safe while other
// processes write (unlike copying the file).
func (d *DB) BackupInto(ctx context.Context, path string) error {
	_, err := d.sql.ExecContext(ctx, mustTelemetrySQL().Statements["backup_into"], path)
	return err
}
