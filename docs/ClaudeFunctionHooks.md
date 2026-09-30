# Claude Code Function Hooks: Runtime Pitfalls

Read before writing or probing any Claude Code function-hook plugin (TS `hooks` module,
`on('<noun>.<event>', ...)`). Learned in issue 646 (Claude Code 2.1.285, early access).
First user: the Jev compaction plugin, see [JevCompaction.md](JevCompaction.md).

## Enabling

- Needs `CLAUDE_CODE_ENABLE_FUNCTION_HOOKS=1` (env or `~/.claude/settings.json` `env`).
- For probes, load the plugin per process: `claude --plugin-dir <dir>`. Do not edit
  `~/.claude/settings.json`, and never use a fresh `CLAUDE_CONFIG_DIR` (no login; credentials
  must not be copied).

## The main failure: zero hooks, silently

The loader statically checks the module. Any rejected construct makes it register **no hooks
at all**; the session runs normally and nothing says why. The reason appears only in
`claude --debug` output. After each change, confirm in the debug log that the hook
`settled` before trusting any result.

Rejected constructs:

- Imports other than the module's own relative files and `"claude-code"`. No `node:fs`,
  `node:child_process` or packages.
- Using `$` as a value. It must be spelled `$.noun.event(...)` at each call site: no
  `const ui = $.ui`, no passing `$` or `$.ui` to a function, no `Object.keys($)` or `$ in x`.

## What the runtime has

- Not available: `fetch`, `process`, `require`, `Bun`, `Deno`, `process.getBuiltinModule`.
- The only way out: `$.process.run([argv...], {stdin})`, which returns exit code, stdout and an
  `isStdoutTruncated` flag. 1 MiB round-trips were verified; the limit is unknown, so check the
  flag.
- Logging: `$.ui.log(...)`.
- Events give only what their payload has. `session.compact` has `trigger` and `messages`
  (`role`, `text`, `toolUses`, `handle`), but no session id or transcript path.

## Probing practice

- One unknown per plugin run. A single rejected construct hides every other result.
- Return a unique marker string from the hook and grep the saved session
  (`~/.claude/projects/<cwd with / and . as ->/*.jsonl`) for it: this proves the returned value
  took effect, not only that the hook fired.
- Examples: `canary/646-session-compact/`, `scripts/canary-646-claude-compact.sh`.
