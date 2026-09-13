package main

import (
	"path/filepath"
	"testing"

	"ubunatic.com/harnez/internal/sessionstate"
	"ubunatic.com/harnez/internal/telemetry"
)

// envFunc builds a getenv stub returning only the given variables.
func envFunc(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

// TestClassifyInvoker covers issue 328's classification table in full,
// including the case the whole ticket exists for: a ppid-derived session id
// with no TTY is NOT a human (it's a script, CI, or an agent that exports
// no recognised env var), so it must land in "unknown" rather than
// inflating `harnez log --human`.
func TestClassifyInvoker(t *testing.T) {
	tty := func() bool { return true }
	notTTY := func() bool { return false }

	cases := []struct {
		name      string
		env       map[string]string
		sessionID string
		isTTY     func() bool
		want      string
	}{
		{
			name:      "claude env var wins over a TTY",
			env:       map[string]string{"CLAUDE_CODE_SESSION_ID": "abc"},
			sessionID: "abc",
			isTTY:     tty,
			want:      "agent:claude",
		},
		{
			name:      "claude env var wins without a TTY",
			env:       map[string]string{"CLAUDE_CODE_SESSION_ID": "abc"},
			sessionID: "abc",
			isTTY:     notTTY,
			want:      "agent:claude",
		},
		{
			name:      "explicit HARNEZ_AGENT is honoured",
			env:       map[string]string{"HARNEZ_AGENT": "codex"},
			sessionID: "ppid-deadbeef-1",
			isTTY:     tty,
			want:      "agent:codex",
		},
		{
			name:      "ppid session id plus a TTY is a human",
			env:       nil,
			sessionID: "ppid-deadbeef-1757756561",
			isTTY:     tty,
			want:      "human",
		},
		{
			name:      "ppid session id without a TTY is unknown, not human",
			env:       nil,
			sessionID: "ppid-deadbeef-1757756561",
			isTTY:     notTTY,
			want:      "unknown",
		},
		{
			name:      "non-ppid session id without an agent env var is unknown",
			env:       nil,
			sessionID: "some-externally-supplied-id",
			isTTY:     tty,
			want:      "unknown",
		},
		{
			name:      "empty session id is unknown",
			env:       nil,
			sessionID: "",
			isTTY:     tty,
			want:      "unknown",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyInvoker(envFunc(tc.env), tc.sessionID, tc.isTTY)
			if got != tc.want {
				t.Errorf("classifyInvoker(%q, tty=%v) = %q, want %q", tc.sessionID, tc.isTTY(), got, tc.want)
			}
		})
	}
}

// TestExecuteAndRecord_ClassifiesHuman asserts the classifier is actually
// wired into issue 326's write path: the stored agent_id uses issue 328's
// vocabulary, not detectAgent's raw sentinel.
func TestExecuteAndRecord_ClassifiesHuman(t *testing.T) {
	opts := recordOpts(t)
	opts.IsTTY = func() bool { return true }
	if err := executeAndRecord(testRoot(), []string{"status"}, opts); err != nil {
		t.Fatalf("executeAndRecord: %v", err)
	}

	rows := readRows(t, opts)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].AgentID != InvokerHuman {
		t.Errorf("AgentID = %q, want %q (session %q)", rows[0].AgentID, InvokerHuman, rows[0].SessionID)
	}
}

// TestExecuteAndRecord_ClassifiesAgent is the same wiring check for the
// agent branch.
func TestExecuteAndRecord_ClassifiesAgent(t *testing.T) {
	opts := recordOpts(t)
	opts.IsTTY = func() bool { return false }
	opts.Getenv = envFunc(map[string]string{"CLAUDE_CODE_SESSION_ID": "sess-claude"})

	if err := executeAndRecord(testRoot(), []string{"status"}, opts); err != nil {
		t.Fatalf("executeAndRecord: %v", err)
	}

	rows := readRows(t, opts)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].AgentID != InvokerAgentPrefix+"claude" {
		t.Errorf("AgentID = %q, want %q", rows[0].AgentID, InvokerAgentPrefix+"claude")
	}
	if rows[0].SessionID != "sess-claude" {
		t.Errorf("SessionID = %q, want the agent-supplied id", rows[0].SessionID)
	}
}

// TestHumanAndAgentPartitionRows asserts issue 328's last verification
// item at the data layer `harnez log --human` / `--agent` sit on: the two
// filters partition the unfiltered set with no overlap and no lost rows
// beyond the deliberate "unknown" bucket.
func TestHumanAndAgentPartitionRows(t *testing.T) {
	dir := t.TempDir()
	db, err := telemetry.Open(filepath.Join(dir, "tool_catalog.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	exit := 0
	for _, agent := range []string{InvokerHuman, InvokerAgentPrefix + "claude", InvokerHuman, InvokerUnknown} {
		if err := db.InsertCLIInvocation(telemetry.CLIInvocation{
			SessionID: "s", AgentID: agent, Command: "status", ExitCode: &exit,
		}); err != nil {
			t.Fatalf("InsertCLIInvocation: %v", err)
		}
	}

	all, err := db.QueryCLIInvocations(telemetry.Filter{}, 0)
	if err != nil {
		t.Fatalf("QueryCLIInvocations: %v", err)
	}
	humans, err := db.QueryCLIInvocations(telemetry.Filter{AgentID: InvokerHuman}, 0)
	if err != nil {
		t.Fatalf("QueryCLIInvocations (human): %v", err)
	}
	agents, err := db.QueryCLIInvocations(telemetry.Filter{AgentID: InvokerAgentPrefix + "claude"}, 0)
	if err != nil {
		t.Fatalf("QueryCLIInvocations (agent): %v", err)
	}
	unknowns, err := db.QueryCLIInvocations(telemetry.Filter{AgentID: InvokerUnknown}, 0)
	if err != nil {
		t.Fatalf("QueryCLIInvocations (unknown): %v", err)
	}

	if len(humans)+len(agents)+len(unknowns) != len(all) {
		t.Fatalf("partition lost rows: %d human + %d agent + %d unknown != %d total",
			len(humans), len(agents), len(unknowns), len(all))
	}
	seen := map[int64]string{}
	for _, set := range [][]telemetry.CLIInvocation{humans, agents, unknowns} {
		for _, r := range set {
			if prev, dup := seen[r.ID]; dup {
				t.Fatalf("row %d appeared in two buckets (%s and %s)", r.ID, prev, r.AgentID)
			}
			seen[r.ID] = r.AgentID
		}
	}
}

// TestApplyCounts_SourcesCountsWithoutTouchingTipBookkeeping asserts issue
// 328's split: the durable table owns counting, sessionstate keeps owning
// the tip/rate bookkeeping fields that exist nowhere else.
func TestApplyCounts_SourcesCountsWithoutTouchingTipBookkeeping(t *testing.T) {
	s := sessionstate.State{
		SessionID:       "s",
		Total:           2,
		Calls:           map[string]sessionstate.Invocation{"status": {Count: 2}},
		TotalAtLastTip:  1,
		TotalAtLastRate: 1,
	}
	sessionstate.ApplyCounts(&s, map[string]int{"status": 7, "index": 3})

	if s.Total != 10 {
		t.Errorf("Total = %d, want 10 (sourced from the counts map)", s.Total)
	}
	if s.Calls["status"].Count != 7 || s.Calls["index"].Count != 3 {
		t.Errorf("per-subcommand counts not sourced: %+v", s.Calls)
	}
	if s.TotalAtLastTip != 1 || s.TotalAtLastRate != 1 {
		t.Errorf("ApplyCounts must not touch tip bookkeeping, got %+v", s)
	}
}

// TestApplyCounts_NilMapIsGracefulDegradation asserts the no-DB path: with
// no counts available the JSON file's own numbers stand unchanged, so gap
// tips keep working off the JSON file alone.
func TestApplyCounts_NilMapIsGracefulDegradation(t *testing.T) {
	s := sessionstate.State{
		SessionID: "s",
		Total:     5,
		Calls:     map[string]sessionstate.Invocation{"status": {Count: 5}},
	}
	sessionstate.ApplyCounts(&s, nil)
	if s.Total != 5 || s.Calls["status"].Count != 5 {
		t.Fatalf("nil counts must leave state untouched, got %+v", s)
	}
}

// TestSessionCallCounts_ReducesPathsToLeafNames asserts the key shape
// handed to sessionstate matches what sessionstate.Record writes
// (cobra's cmd.Name(), the leaf), not cli_invocations' full command path.
func TestSessionCallCounts_ReducesPathsToLeafNames(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	exit := 0
	for _, command := range []string{"issues list", "issues list", "find", "status"} {
		if err := db.InsertCLIInvocation(telemetry.CLIInvocation{
			SessionID: "s", AgentID: InvokerHuman, Command: command, ExitCode: &exit,
		}); err != nil {
			t.Fatalf("InsertCLIInvocation: %v", err)
		}
	}
	db.Close()

	got := sessionCallCounts(dbPath, "s")
	if got["list"] != 2 || got["find"] != 1 || got["status"] != 1 {
		t.Fatalf("unexpected counts: %v", got)
	}
	if _, ok := got["issues list"]; ok {
		t.Error("counts must be keyed by leaf name, not full command path")
	}
}

// TestSessionCallCounts_MissingDBIsNil asserts the graceful-degradation
// signal callers rely on.
func TestSessionCallCounts_MissingDBIsNil(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := writeBlocker(blocker); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	if got := sessionCallCounts(filepath.Join(blocker, "sub", "db.sqlite"), "s"); got != nil {
		t.Fatalf("expected nil counts for an unopenable db, got %v", got)
	}
}
