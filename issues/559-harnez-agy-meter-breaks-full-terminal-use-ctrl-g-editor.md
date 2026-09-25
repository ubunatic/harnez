# harnez-agy --meter breaks full terminal use (Ctrl+G editor)

**Status**: Open
**Priority**: P1
**Severity**: Major
**Category**: Bug

## Problem

In `harnez-agy --meter`, pressing Ctrl+G (open $EDITOR, e.g. nvim, on the prompt) fails with
"cli error / program was killed / program was interrupted". Plain `agy` works.

Host preflight: `internal/agymeter/meter.go` `runChild` (~line 104) sets Stdout/Stderr but never
`Stdin`, so the child gets /dev/null as stdin. Also, `agy-meter-run` does not handle terminal
signals: Ctrl+C / Ctrl+\ / Ctrl+Z go to the whole foreground process group, so the harnez parent
(which hosts the proxy) can die or stop while agy lives on.

## Goal

The metered agy has exactly the terminal experience of plain agy: TUI, $EDITOR via Ctrl+G,
Ctrl+C, Ctrl+Z/fg, resize.

## M1 — full terminal passthrough

- Interactive path (`agy-meter-run`, `Run`): pass `os.Stdin` to the child. Keep the subagent
  path (`harnez agent`, `RunWithEnvDir` with buffers) non-interactive; make stdin an explicit
  parameter or option rather than a hidden global.
- While the child runs, the harnez parent must survive SIGINT/SIGQUIT (child handles them;
  parent keeps the proxy up and exits with the child's status). Do not put the child in its own
  process group; it must stay in the terminal's foreground group. SIGTSTP: parent and child stop
  and resume together (default behaviour is fine if verified). Forward SIGTERM/SIGHUP to the child.
- Do not use exec.CommandContext's kill-on-cancel for signals that the child should handle itself.
- Exit code: propagate the child's exit status.
- Tests: child receives the parent's stdin (fake agy reads a line from stdin); parent ignores
  SIGINT while the child runs and returns the child's exit code. Canary by hand if feasible
  (script(1) or a pty): a fake agy that runs `$EDITOR` gets a tty on stdin.
- Verify: `make test-q1` once, then commit `fix(agymeter): ... (issue 559 M1)`.

## M1 delivered (9eba2cc)

stdin passed to interactive child; parent survives SIGINT/SIGQUIT, forwards TERM/HUP, propagates exit code. Host review: diff OK, make test-q1 green (host run), installed. Awaiting user Ctrl+G confirmation.
