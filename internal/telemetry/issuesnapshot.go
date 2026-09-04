package telemetry

import (
	"database/sql"
	"fmt"
	"time"
)

// IssueStatusSnapshot is one row of the issue_status_snapshots table (issue
// 228): a dated open/closed/draft/unknown ticket-count rollup for one
// project, written by `harnez index` and read back by `harnez find issues
// history`. Field set mirrors schema.go's issue_status_snapshots DDL.
type IssueStatusSnapshot struct {
	ID           int64     `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	ProjectName  string    `json:"project_name"`
	OpenCount    int       `json:"open_count"`
	ClosedCount  int       `json:"closed_count"`
	DraftCount   int       `json:"draft_count"`
	UnknownCount int       `json:"unknown_count"`
}

// InsertIssueSnapshot writes one IssueStatusSnapshot row, but only if its
// counts differ from ProjectName's most recently recorded snapshot (or no
// snapshot exists yet for that project) -- the dedupe that keeps repeated
// `harnez index` runs against an unchanged issues/ directory from growing
// this history unboundedly (issue 228's acceptance criterion). Returns
// whether a row was actually inserted. s.CreatedAt defaults to
// time.Now().UTC() if zero.
func (d *DB) InsertIssueSnapshot(s IssueStatusSnapshot) (bool, error) {
	last, ok, err := d.latestIssueSnapshot(s.ProjectName)
	if err != nil {
		return false, err
	}
	if ok && last.OpenCount == s.OpenCount && last.ClosedCount == s.ClosedCount &&
		last.DraftCount == s.DraftCount && last.UnknownCount == s.UnknownCount {
		return false, nil
	}

	createdAt := s.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	ctx, cancel := defaultContext()
	defer cancel()

	_, err = d.sql.ExecContext(ctx, `
		INSERT INTO issue_status_snapshots (
			created_at, project_name, open_count, closed_count, draft_count, unknown_count
		) VALUES (?, ?, ?, ?, ?, ?)`,
		createdAt.Format(time.RFC3339Nano),
		s.ProjectName, s.OpenCount, s.ClosedCount, s.DraftCount, s.UnknownCount,
	)
	if err != nil {
		return false, fmt.Errorf("telemetry: insert issue snapshot: %w", err)
	}
	return true, nil
}

// latestIssueSnapshot returns the most recently recorded snapshot for
// project, if any -- the comparison basis InsertIssueSnapshot's dedupe
// check uses.
func (d *DB) latestIssueSnapshot(project string) (IssueStatusSnapshot, bool, error) {
	ctx, cancel := defaultContext()
	defer cancel()

	row := d.sql.QueryRowContext(ctx, `
		SELECT id, created_at, project_name, open_count, closed_count, draft_count, unknown_count
		FROM issue_status_snapshots
		WHERE project_name = ?
		ORDER BY created_at DESC, id DESC
		LIMIT 1`, project)

	var s IssueStatusSnapshot
	var createdAt string
	err := row.Scan(&s.ID, &createdAt, &s.ProjectName, &s.OpenCount, &s.ClosedCount, &s.DraftCount, &s.UnknownCount)
	if err == sql.ErrNoRows {
		return IssueStatusSnapshot{}, false, nil
	}
	if err != nil {
		return IssueStatusSnapshot{}, false, fmt.Errorf("telemetry: query latest issue snapshot: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return IssueStatusSnapshot{}, false, fmt.Errorf("telemetry: parse created_at %q: %w", createdAt, err)
	}
	s.CreatedAt = parsed
	return s, true, nil
}

// QueryIssueSnapshots returns every recorded issue_status_snapshots row,
// oldest first, optionally narrowed to one project (empty project means
// unfiltered, all projects) -- the read path `harnez find issues history`
// (issue 228) renders.
func (d *DB) QueryIssueSnapshots(project string) ([]IssueStatusSnapshot, error) {
	ctx, cancel := defaultContext()
	defer cancel()

	query := `
		SELECT id, created_at, project_name, open_count, closed_count, draft_count, unknown_count
		FROM issue_status_snapshots`
	var args []any
	if project != "" {
		query += ` WHERE project_name = ?`
		args = append(args, project)
	}
	query += ` ORDER BY created_at ASC, id ASC`

	rows, err := d.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query issue snapshots: %w", err)
	}
	defer rows.Close()

	var out []IssueStatusSnapshot
	for rows.Next() {
		var s IssueStatusSnapshot
		var createdAt string
		if err := rows.Scan(&s.ID, &createdAt, &s.ProjectName, &s.OpenCount, &s.ClosedCount, &s.DraftCount, &s.UnknownCount); err != nil {
			return nil, fmt.Errorf("telemetry: scan issue snapshot row: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("telemetry: parse created_at %q: %w", createdAt, err)
		}
		s.CreatedAt = parsed
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("telemetry: query issue snapshot rows: %w", err)
	}
	return out, nil
}
