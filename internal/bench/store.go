package bench

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DefaultDir is ~/.harnez/bench, holding bench.sqlite and the setup marker.
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("bench: resolve home dir: %w", err)
	}
	return filepath.Join(home, ".harnez", "bench"), nil
}

const schema = `
CREATE TABLE IF NOT EXISTS runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ts TEXT NOT NULL,
	task TEXT NOT NULL,
	agent TEXT NOT NULL,
	model TEXT NOT NULL,
	docs TEXT NOT NULL,
	cards INTEGER NOT NULL,
	pass INTEGER NOT NULL,
	detail TEXT NOT NULL DEFAULT '',
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	cost_usd REAL NOT NULL DEFAULT 0,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	response TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	read_mode TEXT NOT NULL DEFAULT '',
	turns INTEGER NOT NULL DEFAULT 0,
	total_tokens INTEGER NOT NULL DEFAULT 0,
	session_id TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS runs_cond ON runs(agent, model, docs, cards);
`

// Store is the bench database, separate from the telemetry store.
type Store struct{ db *sql.DB }

// OpenStore opens (creating if needed) the bench database at path.
func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("bench: create db dir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("bench: open %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("bench: create schema: %w", err)
	}
	// Databases created before read mode existed lack these columns.
	for _, col := range []string{"read_mode TEXT NOT NULL DEFAULT ''", "turns INTEGER NOT NULL DEFAULT 0", "total_tokens INTEGER NOT NULL DEFAULT 0", "session_id TEXT NOT NULL DEFAULT ''"} {
		if _, err := db.Exec("ALTER TABLE runs ADD COLUMN " + col); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			db.Close()
			return nil, fmt.Errorf("bench: migrate schema: %w", err)
		}
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Run is one recorded task execution. Error marks an invocation failure
// (not a scoring failure) so infra flakes stay out of pass rates.
type Run struct {
	ID           int64
	TS           time.Time
	Task         string
	Agent        string
	Model        string
	Docs         string
	Cards        bool
	Pass         bool
	Detail       string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	DurationMS   int64
	Response     string
	Error        string
	ReadMode     string
	Turns        int
	TotalTokens  int
	SessionID    string
}

// Insert records a run.
func (s *Store) Insert(r Run) error {
	if r.TS.IsZero() {
		r.TS = time.Now().UTC()
	}
	_, err := s.db.Exec(`INSERT INTO runs (ts, task, agent, model, docs, cards, pass, detail, input_tokens, output_tokens, cost_usd, duration_ms, response, error, read_mode, turns, total_tokens, session_id)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.TS.Format(time.RFC3339), r.Task, r.Agent, r.Model, r.Docs, b2i(r.Cards), b2i(r.Pass), r.Detail,
		r.InputTokens, r.OutputTokens, r.CostUSD, r.DurationMS, r.Response, r.Error, r.ReadMode, r.Turns, r.TotalTokens, r.SessionID)
	return err
}

// Summary aggregates scored runs (Error == ”) per condition.
type Summary struct {
	Agent, Model, Docs string
	Read               string
	Cards              bool
	Runs, Passes       int
	AvgInput, AvgOut   float64
	AvgTotal           float64
	AvgTurns           float64
	AvgCostUSD         float64
	Errors             int
}

// Summaries groups runs by agent, model, docs mode and cards.
func (s *Store) Summaries() ([]Summary, error) {
	rows, err := s.db.Query(`SELECT agent, model, docs, read_mode, cards,
		SUM(error = ''), SUM(error = '' AND pass = 1),
		COALESCE(AVG(CASE WHEN error = '' THEN input_tokens END), 0),
		COALESCE(AVG(CASE WHEN error = '' THEN output_tokens END), 0),
		COALESCE(AVG(CASE WHEN error = '' THEN total_tokens END), 0),
		COALESCE(AVG(CASE WHEN error = '' THEN cost_usd END), 0),
		SUM(error <> ''),
		COALESCE(AVG(CASE WHEN error = '' THEN turns END), 0)
		FROM runs GROUP BY agent, model, docs, read_mode, cards ORDER BY agent, model, docs, read_mode, cards`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Summary
	for rows.Next() {
		var s Summary
		var cards int
		if err := rows.Scan(&s.Agent, &s.Model, &s.Docs, &s.Read, &cards, &s.Runs, &s.Passes, &s.AvgInput, &s.AvgOut, &s.AvgTotal, &s.AvgCostUSD, &s.Errors, &s.AvgTurns); err != nil {
			return nil, err
		}
		s.Cards = cards == 1
		out = append(out, s)
	}
	return out, rows.Err()
}

// Recent returns the newest limit runs, newest first.
func (s *Store) Recent(limit int) ([]Run, error) {
	rows, err := s.db.Query(`SELECT id, ts, task, agent, model, docs, cards, pass, detail, input_tokens, output_tokens, cost_usd, duration_ms, response, error, read_mode, turns, total_tokens, session_id
		FROM runs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		var r Run
		var ts string
		var cards, pass int
		if err := rows.Scan(&r.ID, &ts, &r.Task, &r.Agent, &r.Model, &r.Docs, &cards, &pass, &r.Detail, &r.InputTokens, &r.OutputTokens, &r.CostUSD, &r.DurationMS, &r.Response, &r.Error, &r.ReadMode, &r.Turns, &r.TotalTokens, &r.SessionID); err != nil {
			return nil, err
		}
		r.TS, _ = time.Parse(time.RFC3339, ts)
		r.Cards, r.Pass = cards == 1, pass == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
