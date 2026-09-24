# 536 — harnez statusline unregistered (exit 1 every render); show cwd and cwd drift

**Status**: Closed — c4b4b0f M1 register + guard test, e04a347 M2 cwd/drift, ee940cf M3 arrow order
**Priority**: P1
**Severity**: Medium
**Category**: Bug / UX
**Related**: [[143-show-git-status-in-all-agent-status-bars]]

---

## Problem

`harnez apply` writes `"command": "harnez statusline"` into
`~/.claude/settings.json`, but the binary does not have that command:

    $ harnez statusline
    Error: unknown command "statusline" for "harnez"

`cmd/harnez/statusline.go` defines `newStatuslineCmd()` (added in 6b6c65d),
but no code calls it, so the command is never added to the root command.
`harnez log` shows `statusline  exit 1` on every render. Claude Code shows no
status line.

## /goal

1. Register the command. Add a test that fails if any command `apply`
   configures does not exist on the root command.
2. The status line shows the current (effective) working directory.
3. When the effective cwd differs from the session start directory
   (`workspace.current_dir` != `workspace.project_dir` in the statusLine JSON),
   show both, e.g. `~/projects/voxi → /tmp`.

## Notes

- Check which fields the statusLine payload has in the current Claude Code version before relying on them.

## Root cause (2026-09-24)

`f282e45` (feat(codex): post-tool telemetry hook adapter, issue 420 M2, 2026-09-18) rewrote the
`root.AddCommand(...)` line and dropped `newStatuslineCmd()`. That is a regression, not a missing wire-up.
The /goal 1 test (every command that `apply` configures must be registered) would have caught it.

## Payload sample (Claude Code 2.1.281, captured 2026-09-24 from lucky-fox)

- Confirmed: `workspace.current_dir`, `workspace.project_dir`, `workspace.added_dirs`,
  `workspace.repo.{host,owner,name}`, plus top-level `cwd`, `session_id`, `session_name`, `transcript_path`.
- In the sample `current_dir == project_dir`. The drift case (different values after a `cd`) is still
  unverified, so the code should compare the two fields rather than assume either one moves.

## Milestones (lean sprint, 2026-09-24)

### M1 — register the command again, plus a guard test
- Add `newStatuslineCmd()` to `root.AddCommand` again.
- Test: every `harnez <cmd>` that `apply` writes into managed settings or hooks (statusLine and hook
  commands) resolves on the root command. It must fail on HEAD before the fix.
- Test: `harnez statusline` with the captured payload shape (see Payload sample) exits 0.

**M1 delivered (register the command again + guard test): c4b4b0f.**

### M2 — show the cwd and cwd drift
- Pre-Work (from the M1 review): with the captured voxi payload on stdin, run from ~/projects/harnez,
  `harnez statusline` prints `~/projects/harnez`, so it renders the process cwd, not the payload's.
  Add a test that fails on this: payload dir != process cwd → the payload dir wins.
- Show the effective cwd (`workspace.current_dir`, falling back to `cwd`). When it differs from
  `workspace.project_dir`, show both (`~/projects/voxi → /tmp`). Shorten `$HOME` to `~`.
- Tests with a fixture payload for the same-dir and drift cases. The rendered width stays sensible
  (rune/display width).

**M2 delivered (cwd + drift): e04a347.**
- Correction by the host: the M2 pre-work observation was wrong. The captured sample had been
  overwritten by the host session's own render, so it was a harnez payload. The command never
  rendered the process cwd.

### M3 — fix the drift arrow direction
- Pre-Work / Required Refinements: M2 renders `current → project`. The spec above is
  `project_dir → current_dir` (start dir → where the session is now, e.g. `~/projects/voxi → /tmp`).
  Flip it and make the test assert the order.

**M3 delivered (arrow order): ee940cf.** Live check: `~/projects/voxi → /tmp`.

Finding: in this Claude Code setup, a Bash `cd` is reset after each call, so `workspace.current_dir`
likely stays equal to `project_dir` in normal work, and the drift display only shows up in edge cases.
