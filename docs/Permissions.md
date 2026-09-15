# Claude Code Permission Model

## Two separate permission layers

Claude Code has two independent permission gates:

- **`Bash(...)`** — controls which shell commands Claude can run
- **`Read(...)`** — controls which paths Claude's built-in file-reading tool can access

These are evaluated independently. A `Read(//etc/**)` rule does not affect `Bash`, and a `Bash(grep *)` rule does not affect `Read`.

## Shell commands bypass Read path restrictions

Any shell command that reads files (`grep`, `cat`, `find`, `awk`, `sed`, etc.) operates at the OS level and is not subject to `Read` path rules. For example:

- `Bash(grep *)` permits `grep '' /etc/shadow` regardless of `Read` rules
- `Bash(cat *)` can read any file the shell user can access

This means: once you allow a general shell read command, your `Read` path allowlist becomes cosmetic from a security standpoint. On a personal machine this is usually fine — the main value of `Read` restrictions is limiting accidental broad reads by Claude's file tool, not enforcing hard security boundaries.

## Practical guidance

- Use `Read` path rules to scope Claude's file tool to relevant directories
- Use `Bash` rules to allow specific diagnostic/build commands
- Broad `Bash` globs (`grep *`, `cat *`, `find *`) implicitly grant read access to everything the shell user can see
- Narrow `Bash` rules (e.g. `Bash(sudo smartctl *)`) are meaningful because they limit a specific elevated-privilege command

## Resolved: earlier allow-list assessment (superseded)

The file paths and commands once tracked here as open assessment items
(`~/.claude`, `.bashrc`/`.userrc`/`.zshrc`, `git/*`, `projects/*`,
`~/Pictures/Screenshots`, `.local`, `tree`, `pgrep`) are all already present in
`config.yaml`'s `permissions.allow` list and applied to every managed
`settings.json` — see that file rather than this now-stale TODO. The `sed`/`grep`/`rg`
question was answered implicitly: `Bash(grep *)` and friends are supported the same way
as any other `Bash(...)` allow rule, with the caveat above (shell reads bypass `Read`
scoping) fully in effect for them.

## `permissions.deny` — denying built-in tools (not just Bash/Read scoping)

The two sections above cover `Bash`/`Read` **allow** rules, which scope what Claude's
own file/shell tools can touch. `permissions.deny` is a separate, simpler mechanism:
listing an exact built-in tool name (e.g. `AskUserQuestion`, `SendMessage`,
`DesignSync`) makes that tool unavailable entirely — not scoped, just gone, both from
`ToolSearch` resolution and (for already-loaded tools) from the running session.

Key behaviors confirmed live (issue 316, `harnez apply --debloat` /
`docs/studies/2026-09-15-debloat-context-usage-measurement-and-cli-flag-comparison.md`):

- **Takes effect immediately, mid-session** — not just on next Claude Code startup.
  Editing the real `~/.claude/settings.json` deny list removed tool availability from an
  already-running session within the same turn, and restored it just as immediately on
  revert. Don't assume a deny-list change needs a restart to matter, for better or worse.
- **Token savings are concentrated in interaction/coordination tools, not
  integration-only ones.** Denying verified-unused integration tools (`DesignSync`,
  `PushNotification`, `RemoteTrigger`) saved close to nothing measurably. The real
  savings (~7% of baseline context in one measurement) came from denying
  `AskUserQuestion`/`ScheduleWakeup`/`ReportFindings`/`EnterPlanMode`/`ExitPlanMode` —
  which is a genuine behavioral tradeoff (no more clarifying questions, no plan-mode
  gate), not free cleanup. Don't assume any given deny entry is "free" without measuring.
- **Test additions with `claude --settings '{"permissions":{"deny":[...]}}' -p
  "/context"`** rather than editing the real settings.json first — this merges the deny
  list for one invocation only and is the fastest way to measure a candidate deny
  entry's real token effect before committing to it.
- **`--bare` and `--safe-mode` are not substitutes for `permissions.deny`** — they
  disable entire harness-feature or user-customization axes (CLAUDE.md, skills, hooks),
  not specific built-in tools; `--safe-mode`'s deferred-tool-schema line is measurably
  unchanged from baseline, confirming it doesn't touch this mechanism at all. See the
  study doc above for the full three-axis breakdown (tool denial / harness features /
  user customization) before proposing a new "reduce context" lever.
