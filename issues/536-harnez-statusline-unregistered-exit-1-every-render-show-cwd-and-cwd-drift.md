# 536 — harnez statusline unregistered (exit 1 every render); show cwd and cwd drift

**Status**: Open
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

### M2 — show the cwd and cwd drift
- Show the effective cwd (`workspace.current_dir`, falling back to `cwd`). When it differs from
  `workspace.project_dir`, show both (`~/projects/voxi → /tmp`). Shorten `$HOME` to `~`.
- Tests with a fixture payload for the same-dir and drift cases. The rendered width stays sensible
  (rune/display width).
