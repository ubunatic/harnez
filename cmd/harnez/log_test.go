package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
)

// TestLogCmd_HasNoSubcommands is a structural assertion, not a
// completeness check: issue 327's mitigation against `harnez log` growing
// `docs`/`issues`/`stats` subcommands that duplicate dochistory,
// `find issues history`, and stats is that it takes no subcommands at all.
func TestLogCmd_HasNoSubcommands(t *testing.T) {
	if subs := newLogCmd().Commands(); len(subs) != 0 {
		names := make([]string, 0, len(subs))
		for _, c := range subs {
			names = append(names, c.Name())
		}
		t.Fatalf("harnez log must have no subcommands, found: %v", names)
	}
}

// TestLogCmd_HelpCrossReferences asserts issue 327's discoverability
// requirement: since `log` deliberately has no subcommands, its help is the
// only place the neighbouring history commands get named.
func TestLogCmd_HelpCrossReferences(t *testing.T) {
	long := newLogCmd().Long
	for _, want := range []string{
		"git log",
		"harnez stats",
		"harnez find issues history",
		"harnez dochistory",
		"git log -- docs/ issues/",
	} {
		if !strings.Contains(long, want) {
			t.Errorf("log --help does not mention %q", want)
		}
	}
}

// seedLogDB writes a fixed set of invocations and returns the DB path.
func seedLogDB(t *testing.T, rows []telemetry.CLIInvocation) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	for i, r := range rows {
		if err := db.InsertCLIInvocation(r); err != nil {
			t.Fatalf("seed row %d: %v", i, err)
		}
	}
	return dbPath
}

func exitPtr(v int) *int { return &v }

// logFixture is the shared corpus for the filter tests: two projects, two
// attributions, one failure, spread across a known timeline.
func logFixture(t *testing.T) string {
	t.Helper()
	base := time.Date(2026, 9, 13, 9, 0, 0, 0, time.UTC)
	return seedLogDB(t, []telemetry.CLIInvocation{
		{CreatedAt: base, SessionID: "s1", AgentID: "human", Command: "apply", ProjectName: "smarthome", ExitCode: exitPtr(1), DurationMs: 210},
		{CreatedAt: base.Add(time.Minute), SessionID: "s2", AgentID: "agent:claude", Command: "index", ProjectName: "harnez", ExitCode: exitPtr(0), DurationMs: 88},
		{CreatedAt: base.Add(2 * time.Minute), SessionID: "s2", AgentID: "agent:claude", Command: "issues new", ProjectName: "harnez", ExitCode: exitPtr(0), DurationMs: 41},
		{CreatedAt: base.Add(3 * time.Minute), SessionID: "s2", AgentID: "agent:claude", Command: "index", ProjectName: "harnez", ExitCode: exitPtr(2), DurationMs: 12},
	})
}

func runLogString(t *testing.T, opts logOptions) string {
	t.Helper()
	var buf strings.Builder
	if err := runLog(&buf, opts); err != nil {
		t.Fatalf("runLog: %v", err)
	}
	return buf.String()
}

// TestLog_DefaultIsBoundedAndNewestFirst asserts the zero-arg contract:
// at most defaultLogLimit rows, newest first.
func TestLog_DefaultIsBoundedAndNewestFirst(t *testing.T) {
	base := time.Date(2026, 9, 13, 9, 0, 0, 0, time.UTC)
	var seed []telemetry.CLIInvocation
	for i := 0; i < defaultLogLimit+5; i++ {
		seed = append(seed, telemetry.CLIInvocation{
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
			SessionID: "s", AgentID: "human", Command: "status",
			ProjectName: "proj", ExitCode: exitPtr(0), DurationMs: int64(i),
		})
	}
	dbPath := seedLogDB(t, seed)

	// Dir points at a directory whose base name is not a recorded project,
	// exercising the documented fall-back to the all-projects view.
	out := runLogString(t, logOptions{DBPath: dbPath, Limit: defaultLogLimit, Dir: t.TempDir()})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != defaultLogLimit+1 { // + header
		t.Fatalf("expected %d rows plus a header, got %d lines", defaultLogLimit, len(lines))
	}
	if !strings.HasPrefix(lines[0], "TIME") {
		t.Errorf("missing header row, got %q", lines[0])
	}
	first := lines[1]
	last := lines[len(lines)-1]
	if !strings.Contains(first, base.Add(time.Duration(defaultLogLimit+4)*time.Minute).Format("15:04:05")) {
		t.Errorf("expected newest row first, got %q", first)
	}
	if !strings.Contains(last, base.Add(5*time.Minute).Format("15:04:05")) {
		t.Errorf("expected the oldest surviving row last, got %q", last)
	}
}

// TestLog_ScopesToCurrentProject asserts the cwd-inferred default scoping,
// and that an unrecognised directory falls back to all projects.
func TestLog_ScopesToCurrentProject(t *testing.T) {
	dbPath := logFixture(t)

	// A directory named after a recorded project scopes to it.
	projDir := filepath.Join(t.TempDir(), "harnez")
	if err := mkdir(projDir); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	scoped := runLogString(t, logOptions{DBPath: dbPath, Limit: defaultLogLimit, Dir: projDir})
	if strings.Contains(scoped, "smarthome") {
		t.Errorf("expected output scoped to harnez, got:\n%s", scoped)
	}
	if !strings.Contains(scoped, "issues new") {
		t.Errorf("expected harnez rows in scoped output, got:\n%s", scoped)
	}

	// An unrecorded directory falls back to the cross-project view.
	fallback := runLogString(t, logOptions{DBPath: dbPath, Limit: defaultLogLimit, Dir: t.TempDir()})
	if !strings.Contains(fallback, "smarthome") || !strings.Contains(fallback, "harnez") {
		t.Errorf("expected an all-projects fallback view, got:\n%s", fallback)
	}
}

// TestLog_BareLimitMatchesExplicitLimit asserts `harnez log -5` and
// `harnez log -n 5` are the same invocation, via the shared issue-318
// pre-parser this command reuses.
func TestLog_BareLimitMatchesExplicitLimit(t *testing.T) {
	// The pre-parser turns the bare form into the explicit one before Cobra
	// ever sees it, and leaves an already-explicit -n alone — so the two
	// invocations reach newLogCmd's flag set as literally the same request.
	bareArgs := rewriteArgsForBareLimit([]string{"log", "-5"})
	if strings.Join(bareArgs, " ") != "log --limit 5" {
		t.Fatalf("rewriteArgsForBareLimit(log -5) = %v, want [log --limit 5]", bareArgs)
	}
	explicitArgs := rewriteArgsForBareLimit([]string{"log", "-n", "5"})
	if strings.Join(explicitArgs, " ") != "log -n 5" {
		t.Fatalf("an explicit -n must be left untouched, got %v", explicitArgs)
	}

	// Both parse to the same Limit, so both render the same rows.
	parse := func(argv []string) int {
		cmd := newLogCmd()
		if err := cmd.ParseFlags(argv[1:]); err != nil {
			t.Fatalf("ParseFlags(%v): %v", argv, err)
		}
		n, err := cmd.Flags().GetInt("limit")
		if err != nil {
			t.Fatalf("GetInt(limit): %v", err)
		}
		return n
	}
	if got, want := parse(bareArgs), parse(explicitArgs); got != want || got != 5 {
		t.Fatalf("bare -5 parsed to %d, explicit -n 5 parsed to %d, want 5 for both", got, want)
	}

	dbPath := logFixture(t)
	bare := runLogString(t, logOptions{DBPath: dbPath, Limit: parse(bareArgs), Dir: t.TempDir()})
	explicit := runLogString(t, logOptions{DBPath: dbPath, Limit: parse(explicitArgs), Dir: t.TempDir()})
	if bare != explicit {
		t.Fatalf("bare -N and -n N output differ:\n%s\n---\n%s", bare, explicit)
	}
}

// TestLog_Filters covers --failed, --project, --command, --agent/--human,
// and that project+command combine.
func TestLog_Filters(t *testing.T) {
	dbPath := logFixture(t)
	unscoped := t.TempDir()

	t.Run("failed", func(t *testing.T) {
		out := runLogString(t, logOptions{DBPath: dbPath, Limit: 10, Dir: unscoped, Failed: true})
		if strings.Contains(out, "issues new") {
			t.Errorf("--failed returned a successful row:\n%s", out)
		}
		if !strings.Contains(out, "smarthome") || !strings.Contains(out, "harnez") {
			t.Errorf("--failed lost one of the two failing rows:\n%s", out)
		}
	})

	t.Run("project", func(t *testing.T) {
		out := runLogString(t, logOptions{DBPath: dbPath, Limit: 10, Dir: unscoped, Project: "smarthome"})
		if strings.Contains(out, "harnez") {
			t.Errorf("--project smarthome leaked harnez rows:\n%s", out)
		}
	})

	t.Run("command", func(t *testing.T) {
		out := runLogString(t, logOptions{DBPath: dbPath, Limit: 10, Dir: unscoped, Command: "index"})
		if strings.Contains(out, "issues new") || strings.Contains(out, "apply") {
			t.Errorf("--command index leaked other commands:\n%s", out)
		}
		if strings.Count(out, "index") != 2 {
			t.Errorf("expected 2 index rows, got:\n%s", out)
		}
	})

	t.Run("project and command combine", func(t *testing.T) {
		out := runLogString(t, logOptions{DBPath: dbPath, Limit: 10, Dir: unscoped, Project: "harnez", Command: "index"})
		rows := strings.Split(strings.TrimSpace(out), "\n")
		if len(rows) != 3 { // header + 2 harnez index rows
			t.Errorf("expected 2 combined-filter rows, got:\n%s", out)
		}
	})

	t.Run("human and agent partition", func(t *testing.T) {
		humans := runLogString(t, logOptions{DBPath: dbPath, Limit: 10, Dir: unscoped, Human: true})
		agents := runLogString(t, logOptions{DBPath: dbPath, Limit: 10, Dir: unscoped, Agent: "claude"})
		if !strings.Contains(humans, "smarthome") || strings.Contains(humans, "harnez") {
			t.Errorf("--human returned the wrong rows:\n%s", humans)
		}
		if strings.Contains(agents, "smarthome") {
			t.Errorf("--agent claude leaked the human row:\n%s", agents)
		}
		// The stored form must work identically to the bare id.
		prefixed := runLogString(t, logOptions{DBPath: dbPath, Limit: 10, Dir: unscoped, Agent: "agent:claude"})
		if prefixed != agents {
			t.Errorf("--agent claude and --agent agent:claude must agree:\n%s\n---\n%s", agents, prefixed)
		}
	})
}

// TestLog_HumanAndAgentAreMutuallyExclusive guards the flag combination
// that would otherwise silently resolve to one of the two.
func TestLog_HumanAndAgentAreMutuallyExclusive(t *testing.T) {
	var buf strings.Builder
	err := runLog(&buf, logOptions{DBPath: logFixture(t), Limit: 10, Human: true, Agent: "claude"})
	if err == nil {
		t.Fatal("expected --human with --agent to be rejected")
	}
}

// TestLog_Since filters on the created_at window.
func TestLog_Since(t *testing.T) {
	dbPath := logFixture(t)
	now := time.Date(2026, 9, 13, 9, 3, 30, 0, time.UTC)
	out := runLogString(t, logOptions{
		DBPath: dbPath, Limit: 10, Dir: t.TempDir(),
		Since: 2 * time.Minute, Now: func() time.Time { return now },
	})
	rows := strings.Split(strings.TrimSpace(out), "\n")
	if len(rows) != 3 { // header + the two rows inside the 2-minute window
		t.Fatalf("expected 2 rows within --since 2m, got:\n%s", out)
	}
}

// TestLog_AllUncaps asserts the escape hatch returns everything.
func TestLog_AllUncaps(t *testing.T) {
	dbPath := logFixture(t)
	out := runLogString(t, logOptions{DBPath: dbPath, Limit: 1, All: true, Dir: t.TempDir()})
	rows := strings.Split(strings.TrimSpace(out), "\n")
	if len(rows) != 5 { // header + 4 seeded rows
		t.Fatalf("--all should return every row, got:\n%s", out)
	}
}

// TestLog_JSONRoundTripsSameRows asserts --json and the table render the
// same result set.
func TestLog_JSONRoundTripsSameRows(t *testing.T) {
	dbPath := logFixture(t)
	opts := logOptions{DBPath: dbPath, Limit: 10, Dir: t.TempDir()}

	table := runLogString(t, opts)
	opts.JSON = true
	raw := runLogString(t, opts)

	var rows []telemetry.CLIInvocation
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\n%s", err, raw)
	}
	tableRows := strings.Split(strings.TrimSpace(table), "\n")[1:]
	if len(rows) != len(tableRows) {
		t.Fatalf("--json returned %d rows, table returned %d", len(rows), len(tableRows))
	}
	for i, r := range rows {
		if !strings.Contains(tableRows[i], r.Command) {
			t.Errorf("row %d: json command %q not found in table line %q", i, r.Command, tableRows[i])
		}
	}
}

// TestLog_EmptyTablePrintsNoDataAndExitsZero asserts the required
// no-data behavior: a message, not an error.
func TestLog_EmptyTablePrintsNoDataAndExitsZero(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.sqlite")

	var buf strings.Builder
	if err := runLog(&buf, logOptions{DBPath: dbPath, Limit: 10, Dir: t.TempDir()}); err != nil {
		t.Fatalf("an empty table must not error: %v", err)
	}
	if !strings.HasPrefix(buf.String(), "no data:") {
		t.Fatalf("expected a 'no data:' line, got %q", buf.String())
	}
}

// TestLog_EmptyJSONIsAnEmptyArray keeps --json parseable when nothing
// matches, rather than emitting "null".
func TestLog_EmptyJSONIsAnEmptyArray(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.sqlite")
	out := runLogString(t, logOptions{DBPath: dbPath, Limit: 10, JSON: true, Dir: t.TempDir()})
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("expected [], got %q", out)
	}
}

// mkdir is a small helper so the scoping test can create a directory whose
// base name matches a recorded project_name.
func mkdir(path string) error { return os.MkdirAll(path, 0o755) }
