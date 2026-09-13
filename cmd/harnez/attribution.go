// attribution implements issue 328's invoker classification: deciding
// whether a harnez invocation came from an agent, from a human at a
// terminal, or from something that can't be told apart (a script, CI, or an
// agent that exports no session/agent env var).
//
// Why this isn't just detectAgent: detectAgent returns the literal string
// "unknown" for "no agent env var found", which conflates a human at a
// terminal with an *unrecognised agent*. That conflation is not
// hypothetical — resolve.SessionEnvVars itself documents CODEX_SESSION_ID
// and ANTIGRAVITY_SESSION_ID as unconfirmed guesses, so a real Codex run
// today can land in the same bucket a human does. Building `harnez log
// --human` on that sentinel would report agent traffic as user traffic,
// which is exactly the number this feature exists to make trustworthy.
//
// So classification requires a *second, independent* signal to promote
// "no agent env var" to "human": the ppid- session-id prefix (present only
// when no agent env var supplied a session id — see resolve.Session's
// lock-file fallback) plus stdin being a TTY. Both must agree. Where they
// don't, the answer is the third bucket, "unknown" — deliberately not
// forced into a binary.
package main

import (
	"os"
	"strings"

	"ubunatic.com/harnez/internal/telemetry"
)

// The cli_invocations.agent_id vocabulary (issue 328). These three values
// are a closed set: every row written by this CLI carries one of
// InvokerHuman, InvokerUnknown, or InvokerAgentPrefix + <agent id>.
const (
	// InvokerHuman means: no agent env var, a ppid-derived session id, and
	// an interactive stdin. A person typed this.
	InvokerHuman = "human"
	// InvokerUnknown means the signals disagreed or were absent — a
	// non-interactive script, a CI job, or an agent harness that exports no
	// recognised session/agent env var. Explicitly *not* a synonym for
	// "human": see this file's package comment.
	InvokerUnknown = "unknown"
	// InvokerAgentPrefix namespaces a detected agent id ("agent:claude"),
	// so `harnez log --agent claude` filters on a documented value rather
	// than on a bare sentinel that could also mean "nobody knows".
	InvokerAgentPrefix = "agent:"
)

// ppidSessionPrefix is the prefix resolve.Session's lock-file fallback
// mints its ids with. Its presence is the load-bearing signal that *no*
// agent env var supplied a session id for this invocation.
const ppidSessionPrefix = "ppid-"

// stdinIsTTY reports whether stdin is an interactive terminal. Injected
// through classifyInvoker's isTTY parameter so the classifier is unit
// testable in environments (CI, sandboxed agent runs) that never have one.
func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// classifyInvoker returns the cli_invocations.agent_id value for this
// invocation, per issue 328's table:
//
//	agent env var present | any session id | any TTY -> "agent:<id>"
//	no agent env var      | ppid- id       | TTY     -> "human"
//	no agent env var      | ppid- id       | no TTY  -> "unknown"
//	no agent env var      | other id       | any TTY -> "unknown"
//
// getenv may be nil (means os.Getenv); isTTY may be nil (means stdinIsTTY).
func classifyInvoker(getenv func(string) string, sessionID string, isTTY func() bool) string {
	if agent := detectAgent("", getenv); agent != InvokerUnknown {
		return InvokerAgentPrefix + agent
	}
	if !strings.HasPrefix(sessionID, ppidSessionPrefix) {
		// An id came from somewhere other than the ppid fallback while no
		// agent env var was set — e.g. an explicit --session override, or a
		// harness exporting an id harnez doesn't recognise. Not attributable.
		return InvokerUnknown
	}
	if isTTY == nil {
		isTTY = stdinIsTTY
	}
	if !isTTY() {
		return InvokerUnknown
	}
	return InvokerHuman
}

// sessionCallCounts reads this session's per-subcommand invocation counts
// out of cli_invocations (issue 328), keyed the way internal/sessionstate
// keys them so the two compose: sessionstate.Record uses cobra's
// cmd.Name() — the *leaf* name — whereas cli_invocations.command stores the
// full path ("issues list"), so the path is reduced to its last segment
// here. Returns nil when the DB can't be read; callers fall back to the
// JSON state file's own counts.
func sessionCallCounts(dbPath, sessionID string) map[string]int {
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return nil
	}
	defer db.Close()

	raw, err := db.CLIInvocationCounts(telemetry.Filter{SessionID: sessionID})
	if err != nil {
		return nil
	}
	out := make(map[string]int, len(raw))
	for path, n := range raw {
		out[leafCommandName(path)] += n
	}
	return out
}

// leafCommandName reduces a cli_invocations command path to the leaf name
// sessionstate.Record records ("issues list" -> "list").
func leafCommandName(path string) string {
	fields := strings.Fields(path)
	if len(fields) == 0 {
		return path
	}
	return fields[len(fields)-1]
}
