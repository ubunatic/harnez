// exec implements `harnez exec`, a shell/CLI execution interceptor that
// runs an arbitrary command, proxies its stdio unbuffered, preserves its
// exit code, and records a call_type='shell' telemetry row. See
// issues/118-harnez-exec-shell-interceptor.md.
//
// Issue 226: a command prefixed with HARNEZ_EXPECT_FAILURE=1 (see
// expectFailureEnv/detectExpectFailure below) records call_type
// telemetry.ExpectedFailureCallType instead of "shell" — its real exit
// code is still preserved and stored, only the failure-signal
// classification changes.
//
// Two-stage split per docs/HookRewritePattern.md (mirroring
// distill.go's newDistillHookCmd/runDistillHook vs runDistillWrapper):
//
//   - `harnez exec --tool <tool_name> [--ticket <ticket_id>] -- <cmd...>`
//     is the execution stage: it spawns the real subprocess, proxies its
//     stdio, and writes telemetry. This is what a rewritten Bash command
//     (or manual/scripted use) actually runs as.
//   - `harnez exec hook` is the PreToolUse handshake stage: stdin JSON in,
//     rewrite JSON out, no side effects. It never spawns the child or
//     writes telemetry itself — see docs/HookRewritePattern.md for why
//     the two can't be merged.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/distill"
	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/telemetry"
)

// defaultExecInsertTimeout bounds how long the wrapper will wait for the
// best-effort telemetry write before giving up and returning the child's
// exit code anyway. See execOptions.InsertTimeout and runExecWrapper's doc
// comment: the wrapped command's own execution/exit must never be at the
// mercy of a slow or hung DB write.
const defaultExecInsertTimeout = 200 * time.Millisecond

// expectFailureEnv is issue 226's direction-2 convention: an agent that
// runs a shell command it deliberately expects to fail (probing whether a
// server is down, reproducing a bug to observe its exact failure mode)
// prefixes the command with this var so the resulting tool_calls row is
// recorded with call_type=telemetry.ExpectedFailureCallType instead of
// "shell" — excluded from GroupStats.FailureCount/UnratedFailureCount,
// but never with a faked or swallowed exit code; see detectExpectFailure
// and runExecWrapper's use of it below.
//
//	HARNEZ_EXPECT_FAILURE=1 curl -sf http://maybe-down-host/health
const expectFailureEnv = "HARNEZ_EXPECT_FAILURE"

// expectFailureCmdRE matches a leading HARNEZ_EXPECT_FAILURE=1 (or =true,
// any case) shell-style assignment at the start of a command string,
// optionally preceded by other leading VAR=value assignments — the shape
// the PreToolUse hook's rewritten `bash -c '<original command>'` carries
// when an agent types `HARNEZ_EXPECT_FAILURE=1 <command>` directly.
var expectFailureCmdRE = regexp.MustCompile(`^\s*(?:[A-Za-z_][A-Za-z0-9_]*=\S*\s+)*` + expectFailureEnv + `=(1|[Tt][Rr][Uu][Ee])\b`)

// detectExpectFailure reports whether args (the command runExecWrapper is
// about to spawn) should be recorded as an intentionally-expected failure
// per issue 226's direction 2. It checks two independent signals, since
// this command never reaches this process's own environment the same way
// twice depending on how it was invoked:
//
//   - opts.Getenv(expectFailureEnv): covers direct/manual `harnez exec`
//     use, where the caller's own process environment already carries the
//     var (e.g. a scripted `HARNEZ_EXPECT_FAILURE=1 harnez exec --tool
//     ... --` invocation).
//   - expectFailureCmdRE against each arg: covers the normal agent path.
//     The agent never invokes `harnez exec` directly — the PreToolUse hook
//     (runExecHook, below) has already rewritten the Bash tool call into
//     `harnez exec ... -- bash -c '<original command>'` before this
//     process starts, so an env-var assignment the agent typed at the
//     front of their command lives inside args (the wrapped script text),
//     not in this process's own os.Environ().
//
// This never changes the child's actual exit code — see runExecWrapper's
// doc comment: only how the resulting telemetry row is classified.
func detectExpectFailure(opts execOptions, args []string) bool {
	getenv := opts.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	if v := getenv(expectFailureEnv); v == "1" || strings.EqualFold(v, "true") {
		return true
	}
	for _, a := range args {
		if expectFailureCmdRE.MatchString(a) {
			return true
		}
	}
	return false
}

func newExecCmd() *cobra.Command {
	var toolFlag string
	var ticketFlag string
	var distillFlag string
	var expectFailureFlag bool

	cmd := &cobra.Command{
		Use:     "exec [--tool <tool_name>] [--ticket <ticket_id>] [--distill[=<mode>]] [--expect-failure] -- <command...>",
		Aliases: []string{"⚙", "⚙️"},
		Short:   "Run a command, proxy its stdio unbuffered, and record shell-call telemetry",
		Long: `exec wraps an arbitrary command: it spawns <command...> as a subprocess,
proxies its stdin/stdout/stderr to the caller with no added buffering
latency, preserves and re-exits with the child's exact exit code
(including signal-terminated cases), and writes one call_type='shell'
tool_calls row recording duration_ms, raw_bytes, distilled_bytes (when
distillation is active), and synthetic quality score (1-5).

  harnez exec --tool git -- git status
  harnez exec --tool npm --ticket harnez/118-harnez-exec-shell-interceptor -- npm test
  harnez exec --tool Bash --distill -- go test ./...
  ⚙ echo "hello from gear"
  ⚙ --tool test -- true

The telemetry write is best-effort and bounded: it never delays the
wrapped command's own execution, and gives up waiting on a slow/hung DB
write after a short bound rather than hanging the caller.

See 'harnez exec hook' for the separate PreToolUse rewrite stage that
points an agent's Bash tool calls at this command.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			tool := toolFlag
			if tool == "" {
				tool = "Bash"
			}
			exitCode, err := runExecWrapper(args, execOptions{
				Tool:          tool,
				Ticket:        ticketFlag,
				Distill:       distillFlag,
				ExpectFailure: expectFailureFlag,
			}, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if exitCode != 0 {
				os.Exit(exitCode)
			}
			return nil
		},
	}
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().StringVar(&toolFlag, "tool", "", "tool identifier the wrapped command belongs to, e.g. git, npm, Bash (default: Bash)")
	cmd.Flags().StringVar(&ticketFlag, "ticket", "", "ticket_id override (default: resolve.Ticket())")
	cmd.Flags().StringVar(&distillFlag, "distill", "", "distillation filter mode (auto, gotest, git, raw)")
	cmd.Flags().BoolVar(&expectFailureFlag, "expect-failure", false, "record telemetry row with call_type=shell-expected")
	cmd.Flags().Lookup("distill").NoOptDefVal = "auto"

	cmd.AddCommand(newExecHookCmd())
	return cmd
}

// execOptions bundles runExecWrapper's inputs. The zero value plus a real
// Tool matches production behavior (real env, real cwd, default DB path,
// default bounded insert); tests override the remaining fields to isolate
// from the caller's real environment/DB and to inject slow/failing
// writers for the non-blocking-telemetry acceptance criterion.
type execOptions struct {
	Tool          string
	Ticket        string
	Distill       string
	ExpectFailure bool

	Getenv        func(string) string                                // nil means os.Getenv
	StateDir      string                                             // resolve.Session/Ticket state/lock dir override
	DBPath        string                                             // telemetry DB path override; empty means telemetry.DefaultDBPath()
	InsertTimeout time.Duration                                      // bound on waiting for the telemetry write; <=0 means defaultExecInsertTimeout
	Insert        func(dbPath string, call telemetry.ToolCall) error // nil means defaultInsertExecRow
}

// byteCounter is an io.Writer that only counts bytes written to it,
// safe for concurrent use since exec.Cmd copies a non-*os.File Stdout and
// Stderr on separate goroutines.
type byteCounter struct{ n int64 }

func (c *byteCounter) Write(p []byte) (int, error) {
	atomic.AddInt64(&c.n, int64(len(p)))
	return len(p), nil
}

func (c *byteCounter) total() int64 {
	return atomic.LoadInt64(&c.n)
}

// runExecWrapper spawns args as a subprocess, proxying stdin/stdout/stderr
// to in/out/errOut with no added buffering latency (canary-verified: see
// issues/118's Canary section — Stdin passed through as the underlying
// *os.File directly, Stdout/Stderr tee'd through io.MultiWriter to a byte
// counter without introducing timer-based delay). It returns the child's
// exit code (including the 128+signal convention for signal-terminated
// children) and preserves it exactly; a non-nil error here means the
// command itself could not be spawned at all (e.g. not found), not that
// the child exited non-zero.
//
// After the child exits, it makes one best-effort, bounded attempt to
// write a call_type='shell' telemetry row: the write runs on its own
// goroutine and the wrapper waits at most opts.InsertTimeout (default
// defaultExecInsertTimeout) for it before giving up and returning anyway
// — a slow or hung DB write can delay the wrapper's own return by at most
// that bound, but never by the writer's actual (possibly unbounded) delay.
func runExecWrapper(args []string, opts execOptions, in io.Reader, out, errOut io.Writer) (int, error) {
	debugLog("exec wrapper: invoked, tool=%q ticket=%q distill=%q args=%v", opts.Tool, opts.Ticket, opts.Distill, args)
	if len(args) == 0 {
		return 0, fmt.Errorf("exec: no command given (usage: harnez exec --tool <tool_name> -- <command...>)")
	}
	if opts.Tool == "" {
		return 0, fmt.Errorf("exec: --tool is required")
	}

	counter := &byteCounter{}
	var capturedOutput bytes.Buffer

	c := exec.Command(args[0], args[1:]...)
	c.Stdin = in

	var distOpts distill.Options
	distillActive := opts.Distill != ""
	if distillActive {
		mode := distill.Mode(opts.Distill)
		if mode == "true" || mode == "1" || mode == distill.ModeAuto {
			mode = distill.DetectModeFromArgs(args)
		}
		distOpts = distill.Options{Mode: mode, MaxLines: 300}
		c.Stdout = io.MultiWriter(&capturedOutput, counter)
		c.Stderr = io.MultiWriter(&capturedOutput, counter)
	} else {
		c.Stdout = io.MultiWriter(out, counter, &capturedOutput)
		c.Stderr = io.MultiWriter(errOut, counter, &capturedOutput)
	}

	start := time.Now()
	runErr := c.Run()
	duration := time.Since(start)

	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) {
		// The command could not even be spawned (not found, permission
		// denied, ...) — this is a harnez-level error, not a child exit
		// code to preserve, so no telemetry row is written for it.
		return 1, fmt.Errorf("exec: run %v: %w", args, runErr)
	}
	exitCode := exitCodeFromError(runErr)

	rawBytes := counter.total()
	var distilledBytesPtr *int64

	if distillActive {
		rawStr := capturedOutput.String()
		distilled, rBytes, dBytes := distill.DistillWithMetrics(rawStr, distOpts)
		fmt.Fprintln(out, distilled)
		rawBytes = rBytes
		if distOpts.Mode != distill.ModeRaw {
			distilledBytesPtr = &dBytes
		}
	}

	score, note := telemetry.ScoreShell(capturedOutput.String(), exitCode)

	recordExecTelemetry(opts, execCall{
		Tool:           opts.Tool,
		ExitCode:       exitCode,
		DurationMs:     duration.Milliseconds(),
		RawBytes:       rawBytes,
		DistilledBytes: distilledBytesPtr,
		Score:          &score,
		Note:           note,
		ExpectFailure:  opts.ExpectFailure || detectExpectFailure(opts, args),
	})

	return exitCode, nil
}

// exitCodeFromError extracts a shell-convention exit code from the result
// of exec.Cmd.Run(): nil is 0, a signal-terminated child reports
// 128+signal (the same convention bash uses), a normal non-zero exit
// reports its own code, and -1 signals a wait/status error distinct from
// either.
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		return exitErr.ExitCode()
	}
	return -1
}

// execCall carries the post-run facts recordExecTelemetry needs to build a
// telemetry.ToolCall; kept separate from telemetry.ToolCall itself so this
// file doesn't need to know about columns (session/ticket/agent/project)
// it resolves itself.
type execCall struct {
	Tool           string
	ExitCode       int
	DurationMs     int64
	RawBytes       int64
	DistilledBytes *int64
	Score          *int
	Note           string
	// ExpectFailure marks this row as an intentionally-expected failure
	// (issue 226's HARNEZ_EXPECT_FAILURE convention, detected by
	// detectExpectFailure) — recordExecTelemetry writes call_type
	// telemetry.ExpectedFailureCallType instead of "shell" when true. The
	// row's ExitCode is never altered by this flag; only the call_type
	// classification changes.
	ExpectFailure bool
}

// recordExecTelemetry makes the single best-effort, bounded attempt at
// writing call's telemetry row described in runExecWrapper's doc comment.
// It never returns an error: on any failure (resolve, DB open, insert, or
// simply running out of time) the row is silently dropped, per this
// ticket's "telemetry is best-effort, never on the command's critical
// path" requirement.
func recordExecTelemetry(opts execOptions, call execCall) {
	timeout := opts.InsertTimeout
	if timeout <= 0 {
		timeout = defaultExecInsertTimeout
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		sessionID, err := resolve.Session(resolve.SessionOptions{
			Getenv:  opts.Getenv,
			LockDir: opts.StateDir,
		})
		if err != nil {
			debugLog("exec wrapper: resolve.Session failed, dropping telemetry row: %v", err)
			return
		}
		// An unresolved ticket_id is expected, not fatal (see resolve.Ticket's
		// doc comment): this repo's own usage never branches per ticket, and
		// a session may start outside any git repo entirely. A resolve.Ticket
		// error here is now only the rare writeLastTicket I/O failure case,
		// and even that must not cost the row — ticket_id degrades to "".
		ticketID, err := resolve.Ticket(resolve.TicketOptions{
			Explicit:  opts.Ticket,
			SessionID: sessionID,
			StateDir:  opts.StateDir,
		})
		if err != nil {
			debugLog("exec wrapper: resolve.Ticket failed, using empty ticket_id: %v", err)
			ticketID = ""
		}

		wd, err := os.Getwd()
		if err != nil {
			wd = ""
		}

		exitCode := call.ExitCode
		callType := "shell"
		if call.ExpectFailure {
			callType = telemetry.ExpectedFailureCallType
		}
		tc := telemetry.ToolCall{
			SessionID:      sessionID,
			TicketID:       ticketID,
			ProjectName:    filepath.Base(wd),
			WorkingDir:     wd,
			AgentID:        detectAgent("", opts.Getenv),
			ToolName:       call.Tool,
			CallType:       callType,
			Score:          call.Score,
			Note:           call.Note,
			ExitCode:       &exitCode,
			DurationMs:     call.DurationMs,
			RawBytes:       call.RawBytes,
			DistilledBytes: call.DistilledBytes,
		}

		dbPath := opts.DBPath
		if dbPath == "" {
			p, err := telemetry.DefaultDBPath()
			if err != nil {
				debugLog("exec wrapper: DefaultDBPath failed, dropping telemetry row: %v", err)
				return
			}
			dbPath = p
		}

		insert := opts.Insert
		if insert == nil {
			insert = defaultInsertExecRow
		}
		if err := insert(dbPath, tc); err != nil {
			debugLog("exec wrapper: insert failed at %s: %v", dbPath, err)
		} else {
			debugLog("exec wrapper: inserted row tool=%q session=%q ticket=%q", tc.ToolName, tc.SessionID, tc.TicketID)
		}
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		debugLog("exec wrapper: telemetry write exceeded %s timeout, returning without waiting", timeout)
	}
}

// defaultInsertExecRow opens the telemetry DB at dbPath and writes one
// row. Split out from recordExecTelemetry so tests can inject a
// slow/failing writer via execOptions.Insert without touching the user's
// real DB.
func defaultInsertExecRow(dbPath string, call telemetry.ToolCall) error {
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return fmt.Errorf("exec: open telemetry db: %w", err)
	}
	defer db.Close()
	return db.Insert(call)
}

// newExecHookCmd implements `harnez exec hook`, the PreToolUse handshake
// stage described in docs/HookRewritePattern.md. It reuses distill.go's
// hookInput/hookOutput/hookSpecificOutput types (same package, same
// Claude Code hook payload shape) rather than redefining them.
func newExecHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hook",
		Short: "PreToolUse hook: rewrite Bash commands to route through 'harnez exec'",
		Long: `hook implements a Claude Code PreToolUse hook for the Bash matcher.

It reads the tool-call JSON payload from stdin and, for Bash commands not
already routed through 'harnez exec', emits a rewrite envelope pointing
the command at:

  harnez exec --tool <tool_name> -- <original command>

This is the endpoint issues/119's apply-managed hook entries invoke; the
hook itself never spawns the command or writes telemetry — see
docs/HookRewritePattern.md for why the rewrite and execution stages are
kept as separate commands. Installing this hook into a user's Claude Code
settings is issues/119's job, out of this command's scope.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExecHook(os.Stdin, os.Stdout)
		},
	}
}

// execInvocationRE-equivalent guard: avoid re-wrapping a command that
// already routes through harnez exec.
func alreadyRoutedThroughExec(command string) bool {
	return strings.Contains(command, "harnez exec")
}

// runExecHook decodes a PreToolUse hook payload from in and, if it's a
// non-empty Bash command not already wrapped, writes the rewrite envelope
// to out. It is side-effect-free: no subprocess is spawned and no
// telemetry row is written here (see docs/HookRewritePattern.md).
//
// The rewritten command routes through 'bash -c <quoted original>' rather
// than splicing the original command's tokens directly after "--": Claude
// Code re-executes the rewritten string via its own outer 'bash -c', so
// any shell metacharacters in the original command (pipes, &&, ;, quoting)
// would otherwise be re-interpreted by that outer shell instead of reaching
// harnez exec as a single argument — silently breaking telemetry capture
// and, for '&&'/';', silently running part of the command outside harnez
// exec's wrapping entirely. Wrapping in a quoted 'bash -c' argument keeps
// the original command intact as one shell string, exactly as distill's
// own hook rewrite already does (see internal/distill/hook.go).
//
// This hook also applies distill's PreToolUse rewrite (HARNEZ_DISTILL_AUTOPIPE)
// itself, composing it into the one rewrite this hook emits, rather than
// relying on Claude Code to run two separate PreToolUse hooks on the same
// Bash matcher: per Claude Code's hooks-guide ("Limitations" — when
// multiple PreToolUse hooks return updatedInput for the same tool, hooks
// run in parallel and the last one to finish wins, non-deterministically),
// two independently-rewriting hooks on the same matcher is a real bug, not
// a hypothetical — see docs/HookRewritePattern.md. apply only installs
// this one PreToolUse/Bash hook; distill's own hook command still exists
// and works standalone, it's just not separately wired into apply's
// managed hooks anymore (see config.yaml).
func runExecHook(in io.Reader, out io.Writer) error {
	raw, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("decode hook payload: %w", err)
	}
	debugLog("exec hook: invoked, payload=%s", string(raw))

	var payload hookInput
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode hook payload: %w", err)
	}
	if payload.ToolName != "Bash" {
		debugLog("exec hook: skip, tool_name=%q != Bash", payload.ToolName)
		return nil
	}
	command := payload.ToolInput.Command
	if command == "" || alreadyRoutedThroughExec(command) {
		debugLog("exec hook: skip, command empty or already routed: %q", command)
		return nil
	}

	effective := command
	distillFlag := ""
	if isDistillAutopipeEnabled() && distill.MatchesNoisy(command) {
		distillFlag = " --distill"
	}

	rewritten := fmt.Sprintf("harnez exec --tool %s%s -- bash -c %s", payload.ToolName, distillFlag, shellQuote(effective))
	debugLog("exec hook: rewriting %q -> %q", command, rewritten)
	return json.NewEncoder(out).Encode(hookOutput{
		HookSpecificOutput: hookSpecificOutput{
			HookEventName: "PreToolUse",
			UpdatedInput:  map[string]string{"command": rewritten},
		},
	})
}

func isDistillAutopipeEnabled() bool {
	enabled := os.Getenv(distillAutopipeEnv)
	return enabled == "1" || strings.EqualFold(enabled, "true")
}

// debugLog appends a timestamped line to ~/.harnez/debug.log when the
// DEBUG env var apply already installs (config.yaml, currently otherwise
// unused by harnez's own code) is "1" or "true". Best-effort: never
// returns an error, never blocks/breaks the caller if home dir or file
// open fails. Added while diagnosing why the PreToolUse hook appeared
// registered (per `/hooks`) but wasn't producing tool_calls rows —
// harnez had no log files at all until this, a real gap for a tool
// meant to observe tool calls.
func debugLog(format string, args ...any) {
	enabled := os.Getenv("DEBUG")
	if enabled != "1" && !strings.EqualFold(enabled, "true") {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(home, ".harnez", "debug.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] "+format+"\n", append([]any{time.Now().UTC().Format(time.RFC3339Nano)}, args...)...)
}

// distillAutopipeRewrite applies distill's own noisy-command rewrite
// (internal/distill.RewriteBashCommand) when HARNEZ_DISTILL_AUTOPIPE is
// enabled, mirroring runDistillHook's (cmd/harnez/distill.go) opt-in gate
// exactly so behavior is unchanged for users who already set that env var.
func distillAutopipeRewrite(command string) (string, bool) {
	enabled := os.Getenv(distillAutopipeEnv)
	if enabled != "1" && !strings.EqualFold(enabled, "true") {
		return command, false
	}
	return distill.RewriteBashCommand(command)
}

// shellQuote wraps s in single quotes for safe embedding as one argument
// in a shell command line, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
