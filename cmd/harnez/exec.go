// exec implements `harnez exec`, a shell/CLI execution interceptor that
// runs an arbitrary command, proxies its stdio unbuffered, preserves its
// exit code, and records a call_type='shell' telemetry row. See
// issues/118-harnez-exec-shell-interceptor.md.
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

func newExecCmd() *cobra.Command {
	var toolFlag string
	var ticketFlag string

	cmd := &cobra.Command{
		Use:   "exec --tool <tool_name> [--ticket <ticket_id>] -- <command...>",
		Short: "Run a command, proxy its stdio unbuffered, and record shell-call telemetry",
		Long: `exec wraps an arbitrary command: it spawns <command...> as a subprocess,
proxies its stdin/stdout/stderr to the caller with no added buffering
latency, preserves and re-exits with the child's exact exit code
(including signal-terminated cases), and writes one call_type='shell'
tool_calls row recording duration_ms and raw_bytes (stdout+stderr byte
count).

  harnez exec --tool git -- git status
  harnez exec --tool npm --ticket harnez/118-harnez-exec-shell-interceptor -- npm test

The telemetry write is best-effort and bounded: it never delays the
wrapped command's own execution, and gives up waiting on a slow/hung DB
write after a short bound rather than hanging the caller.

distilled_bytes is currently always left NULL: nothing in
'harnez distill' today exposes a byte-count signal this wrapper could
read back (see issues/118's Notes) — that needs a minimal follow-up in
distill itself, out of this ticket's scope.

See 'harnez exec hook' for the separate PreToolUse rewrite stage that
points an agent's Bash tool calls at this command.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			exitCode, err := runExecWrapper(args, execOptions{Tool: toolFlag, Ticket: ticketFlag},
				cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if exitCode != 0 {
				os.Exit(exitCode)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&toolFlag, "tool", "", "tool identifier the wrapped command belongs to, e.g. git, npm, Bash (required)")
	cmd.Flags().StringVar(&ticketFlag, "ticket", "", "ticket_id override (default: resolve.Ticket())")

	cmd.AddCommand(newExecHookCmd())
	return cmd
}

// execOptions bundles runExecWrapper's inputs. The zero value plus a real
// Tool matches production behavior (real env, real cwd, default DB path,
// default bounded insert); tests override the remaining fields to isolate
// from the caller's real environment/DB and to inject slow/failing
// writers for the non-blocking-telemetry acceptance criterion.
type execOptions struct {
	Tool   string
	Ticket string

	Getenv        func(string) string // nil means os.Getenv
	TicketDir     string              // resolve.TicketOptions.Dir override
	StateDir      string              // resolve.Session/Ticket state/lock dir override
	DBPath        string              // telemetry DB path override; empty means telemetry.DefaultDBPath()
	InsertTimeout time.Duration       // bound on waiting for the telemetry write; <=0 means defaultExecInsertTimeout
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
	if len(args) == 0 {
		return 0, fmt.Errorf("exec: no command given (usage: harnez exec --tool <tool_name> -- <command...>)")
	}
	if opts.Tool == "" {
		return 0, fmt.Errorf("exec: --tool is required")
	}

	counter := &byteCounter{}
	c := exec.Command(args[0], args[1:]...)
	c.Stdin = in
	c.Stdout = io.MultiWriter(out, counter)
	c.Stderr = io.MultiWriter(errOut, counter)

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

	recordExecTelemetry(opts, execCall{
		Tool:       opts.Tool,
		ExitCode:   exitCode,
		DurationMs: duration.Milliseconds(),
		RawBytes:   counter.total(),
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
	Tool       string
	ExitCode   int
	DurationMs int64
	RawBytes   int64
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
			return
		}
		ticketID, err := resolve.Ticket(resolve.TicketOptions{
			Explicit:  opts.Ticket,
			Dir:       opts.TicketDir,
			SessionID: sessionID,
			StateDir:  opts.StateDir,
		})
		if err != nil {
			return
		}

		wd, err := os.Getwd()
		if err != nil {
			wd = ""
		}

		exitCode := call.ExitCode
		tc := telemetry.ToolCall{
			SessionID:   sessionID,
			TicketID:    ticketID,
			ProjectName: filepath.Base(wd),
			WorkingDir:  wd,
			AgentID:     detectAgent("", opts.Getenv),
			ToolName:    call.Tool,
			CallType:    "shell",
			ExitCode:    &exitCode,
			DurationMs:  call.DurationMs,
			RawBytes:    call.RawBytes,
			// DistilledBytes intentionally left nil (SQL NULL): see this
			// file's top-of-file doc comment and issues/118's Notes —
			// nothing in internal/distill exposes a byte-count signal
			// today, and this ticket does not add one.
		}

		dbPath := opts.DBPath
		if dbPath == "" {
			p, err := telemetry.DefaultDBPath()
			if err != nil {
				return
			}
			dbPath = p
		}

		insert := opts.Insert
		if insert == nil {
			insert = defaultInsertExecRow
		}
		_ = insert(dbPath, tc)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
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
	var payload hookInput
	if err := json.NewDecoder(in).Decode(&payload); err != nil {
		return fmt.Errorf("decode hook payload: %w", err)
	}
	if payload.ToolName != "Bash" {
		return nil
	}
	command := payload.ToolInput.Command
	if command == "" || alreadyRoutedThroughExec(command) {
		return nil
	}

	effective := command
	if rewritten, ok := distillAutopipeRewrite(command); ok {
		effective = rewritten
	}

	rewritten := fmt.Sprintf("harnez exec --tool %s -- bash -c %s", payload.ToolName, shellQuote(effective))
	return json.NewEncoder(out).Encode(hookOutput{
		HookSpecificOutput: hookSpecificOutput{
			HookEventName: "PreToolUse",
			UpdatedInput:  map[string]string{"command": rewritten},
		},
	})
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
