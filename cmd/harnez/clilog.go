// clilog implements issue 326's write path: one cli_invocations telemetry
// row per harnez CLI invocation, recording which subcommand ran, when, in
// which project, by whom, and whether it succeeded. `harnez log` (issue
// 327) is the read surface over what this file writes.
//
// The single load-bearing design decision here is *where* the write
// happens. The obvious hook points both fail:
//
//   - root.PersistentPreRunE (where sessionTipHook lives) fires before the
//     command body, so it can never observe exit_code or duration_ms.
//   - Cobra's PersistentPostRunE does not run at all when the command's
//     RunE returns an error — exactly the invocation most worth recording.
//
// So the write happens in main() itself, around root.Execute(), via
// executeAndRecord below: start time captured before, the returned error
// (and therefore the exit code) after, and the resolved command path
// recovered with root.Find. See TestExecuteAndRecord_FailingSubcommand.
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/telemetry"
)

// disableCLILogEnv opts out of cli_invocations recording entirely, checked
// before any DB work happens. Named after the existing
// HARNEZ_DISABLE_RATE_FEEDBACK convention (issue 142) rather than inventing
// a second opt-out shape.
const disableCLILogEnv = "HARNEZ_DISABLE_CLI_LOG"

// redactedPlaceholder is what a suppressed flag value or unsafe-looking
// positional is stored as, instead of dropping the token entirely — the
// shape of an invocation ("index -d <redacted>") stays legible in `harnez
// log` without the value itself being retained.
const redactedPlaceholder = "<redacted>"

// safeArgTokenRe matches a positional/flag value considered safe to store
// verbatim: a short, single-token identifier with no whitespace, no path
// separator, and no shell-ish punctuation. Subcommand-shaped words
// ("issues", "new"), tool names ("Read"), numbers, and short filter
// expressions ("status:open") pass; absolute paths, quoted prose
// (descriptions, ticket titles, commit messages), URLs, and anything long
// do not. Deliberately an allow-list: a deny-list of "things that look
// sensitive" fails open on the case nobody anticipated, which is the wrong
// default for a column that records every invocation forever.
var safeArgTokenRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,39}$`)

// redactValueFlagSubstrings is the deny-list of long-flag name fragments
// whose *values* are always redacted, on top of the allow-list above. The
// allow-list already catches most hazards, but these flags take values that
// can be short and token-shaped while still being exactly what must not be
// retained: a hostname (`usage --host workstation`), a filesystem location
// (`init -d proj`), an opaque credential, or a free-text note. Matched as
// substrings so `--host`, `--watch-host`, and `--default-host` are all
// covered by one entry.
var redactValueFlagSubstrings = []string{
	"host",
	"path",
	"dir",
	"file",
	"out",
	"target",
	"config",
	"token",
	"secret",
	"key",
	"password",
	"url",
	"session",
	"ticket",
	"note",
	"message",
}

// redactValueShorthands is the shorthand counterpart to
// redactValueFlagSubstrings. It lists only shorthands this CLI binds to
// *string* flags (-t target, -d docs/dir, -c config, -o out): a boolean
// shorthand must never appear here, because redactArgs would then treat the
// following positional as that flag's value and redact it.
var redactValueShorthands = map[string]bool{
	"-t": true,
	"-d": true,
	"-c": true,
	"-o": true,
}

// redactsValue reports whether flag (a "--name" or "-x" token, already
// stripped of any "=value" suffix) is one whose value must not be stored.
func redactsValue(flag string) bool {
	if redactValueShorthands[flag] {
		return true
	}
	if !strings.HasPrefix(flag, "--") {
		return false
	}
	name := strings.ToLower(strings.TrimPrefix(flag, "--"))
	for _, frag := range redactValueFlagSubstrings {
		if strings.Contains(name, frag) {
			return true
		}
	}
	return false
}

// redactArgs renders argv into the sanitized form stored in
// cli_invocations.args. Flag *names* are always kept (they are the useful
// part: "was --json passed?", "did this run use --all?"); values and
// positionals survive only if redactsValue says otherwise and they match
// safeArgTokenRe.
func redactArgs(argv []string) string {
	out := make([]string, 0, len(argv))
	redactNext := false
	for _, a := range argv {
		switch {
		case strings.HasPrefix(a, "-") && a != "-" && a != "--":
			redactNext = false
			name, value, hasValue := strings.Cut(a, "=")
			switch {
			case !hasValue:
				out = append(out, name)
				redactNext = redactsValue(name)
			case redactsValue(name):
				out = append(out, name+"="+redactedPlaceholder)
			default:
				out = append(out, name+"="+safeToken(value))
			}
		case redactNext:
			redactNext = false
			out = append(out, redactedPlaceholder)
		default:
			out = append(out, safeToken(a))
		}
	}
	return strings.Join(out, " ")
}

// safeToken returns v unchanged if it passes safeArgTokenRe, else the
// placeholder.
func safeToken(v string) string {
	if safeArgTokenRe.MatchString(v) {
		return v
	}
	return redactedPlaceholder
}

// cliLogOptions bundles the seams that let this write path be tested
// against a throwaway DB and a synthetic environment, matching the
// DBPath/Getenv/StateDir test-only-override pattern statsOptions and
// findHistoryOptions already use. Production (main) passes the zero value.
type cliLogOptions struct {
	DBPath   string              // empty means telemetry.DefaultDBPath()
	Getenv   func(string) string // nil means os.Getenv
	StateDir string              // resolve.Session/Ticket state dir override
	IsTTY    func() bool         // nil means stdinIsTTY; issue 328's attribution seam
}

func (o cliLogOptions) getenv(name string) string {
	if o.Getenv == nil {
		return os.Getenv(name)
	}
	return o.Getenv(name)
}

// executeAndRecord runs root against argv and records the invocation into
// cli_invocations afterwards. This is main()'s body, extracted so the
// recording behavior — in particular that a *failing* subcommand still
// produces a row — is testable without a subprocess. The returned error is
// root.Execute's, unchanged: recording never alters the command's outcome.
func executeAndRecord(root *cobra.Command, argv []string, opts cliLogOptions) error {
	rewritten := rewriteArgsForBareLimit(argv)
	root.SetArgs(rewritten)

	start := time.Now()
	runErr := root.Execute()
	elapsed := time.Since(start)

	recordCLIInvocation(root, argv, rewritten, elapsed, runErr, opts)
	return runErr
}

// recordCLIInvocation writes one best-effort cli_invocations row. Every
// failure path here — opt-out set, unresolvable session, missing or
// unwritable DB — returns silently: this is passive telemetry, and per
// issue 326 it must never block, fail, or add output to a real command.
func recordCLIInvocation(root *cobra.Command, argv, rewritten []string, elapsed time.Duration, runErr error, opts cliLogOptions) {
	if opts.getenv(disableCLILogEnv) != "" {
		return
	}

	sessionID, err := resolve.Session(resolve.SessionOptions{
		Getenv:  opts.Getenv,
		LockDir: opts.StateDir,
	})
	if err != nil || sessionID == "" {
		return
	}

	dbPath := opts.DBPath
	if dbPath == "" {
		p, err := telemetry.DefaultDBPath()
		if err != nil {
			return
		}
		dbPath = p
	}
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return
	}
	defer db.Close()

	exitCode := 0
	if runErr != nil {
		exitCode = 1
	}
	wd, err := os.Getwd()
	if err != nil {
		wd = ""
	}
	ticketID, _ := resolve.Ticket(resolve.TicketOptions{SessionID: sessionID, StateDir: opts.StateDir})

	_ = db.InsertCLIInvocation(telemetry.CLIInvocation{
		SessionID:     sessionID,
		AgentID:       classifyInvoker(opts.Getenv, sessionID, opts.IsTTY),
		Command:       resolveCommandPath(root, rewritten),
		Args:          redactArgs(argv),
		ProjectName:   filepath.Base(wd),
		WorkingDir:    wd,
		TicketID:      ticketID,
		ExitCode:      &exitCode,
		DurationMs:    elapsed.Milliseconds(),
		HarnezVersion: harnez.Version,
	})
}

// resolveCommandPath recovers the full cobra path of the command argv
// selected — "issues new", not "new" — without depending on any hook having
// run. root.Find is the same traversal Execute itself uses, and it resolves
// correctly whether the command succeeded, failed, or was never reached.
//
// Fallbacks cover the two cases Find can't answer: a bare `harnez` (no
// subcommand, prints help) records the root's own name, and an unknown
// subcommand records the token the user actually typed, so a typo is
// visible in `harnez log` instead of silently collapsing into "harnez".
func resolveCommandPath(root *cobra.Command, args []string) string {
	if cmd, _, err := root.Find(args); err == nil && cmd != nil {
		if path := strings.TrimSpace(strings.TrimPrefix(cmd.CommandPath(), root.Name())); path != "" {
			return path
		}
	}
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return safeToken(a)
		}
	}
	return root.Name()
}
