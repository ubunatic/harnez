# 319 — Document Linux `timeout` command convention: prefix long-running shell commands in Bash.md

**Status**: Closed — implemented 2026-09-11
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Documentation

---

## Summary

`docs/lang/Bash.md` (the copyable Bash/Shell conventions doc, installed to
`~/.claude/docs/Bash.md` and referenced from every project's AGENTS.md) does
not mention the coreutils `timeout` command anywhere. Confirmed via
`grep -rn "\btimeout\b" docs/Bash.md docs/lang/Bash.md` — no hits.

Agents (and the user) currently have no documented convention telling them to
wrap potentially long-running or hanging shell invocations with
`timeout <secs> <cmd>` before running them directly. This matters
specifically for commands run without the harness's own background-task
mechanism, where a hang blocks the whole turn with no way to recover short of
the user manually killing the shell.

Related but distinct: issue 268 (`harnez exec: add default 60s timeout ...`)
is about `harnez`'s own internal Bash-wrapping hook adding a timeout
automatically. This ticket is about the *documented convention* for agents to
self-apply `timeout` when constructing a command themselves — e.g. for tools,
harnesses, or execution paths that don't go through `harnez exec` at all.

## Proposed change

- Add a short section/bullet to `docs/lang/Bash.md` (and by extension
  `/home/uwe/.claude/docs/Bash.md` once re-applied) recommending:
  - Prefix commands with uncertain or potentially unbounded runtime with
    `timeout <N> <cmd>` (e.g. probing an external service, waiting on a lock,
    a network call without its own client-side timeout flag).
  - Pick `<N>` relative to the command's expected duration, not a single
    fixed global default.
  - Note the exit-code convention: `timeout` returns 124 when it kills the
    command, distinct from the wrapped command's own failure exit codes —
    relevant if downstream logic branches on `$?`.
  - Cross-reference `docs/practices/AgenticLoop.md`'s existing "Blocking
    sleep Waits" and "Buffered Long-Running Output" anti-patterns, since this
    is the same family of guidance (don't let a shell command silently hang
    or block the turn) but for command *execution* rather than *waiting/
    polling*.
- Confirm whether this belongs in `docs/lang/Bash.md` itself or as a new
  bullet in `docs/practices/AgenticLoop.md`'s anti-patterns list — the latter
  already owns "Blocking sleep Waits"/"Buffered Long-Running Output," so a
  short entry there plus a one-line pointer from Bash.md may fit the existing
  docs layout convention (`docs/practices/` = workflow, `docs/lang/` =
  language mechanics) better than duplicating guidance in both.

## Verification

- [x] `docs/lang/Bash.md` (§7 Commands & Traps) documents the `timeout`
      convention with a concrete example, exit-code note, and a
      cross-reference to `docs/practices/AgenticLoop.md`'s existing
      "Blocking sleep Waits"/"Buffered Long-Running Output" anti-patterns.
- [x] `harnez init` propagates the updated doc into this project's local
      `docs/Bash.md` copy, confirmed via `git diff`.
- [x] No code changes required — documentation only.

## Implementation note

Placed in `docs/lang/Bash.md` (language mechanics) rather than
`docs/practices/AgenticLoop.md` (workflow), since `timeout` is a Bash-level
command-invocation mechanic, not an orchestration-level anti-pattern — with a
cross-reference added in both directions is unnecessary; a pointer from
Bash.md to AgenticLoop.md's related anti-patterns is sufficient.

Re-running `harnez init` to verify propagation reproduced issue 315 again
(dropped `prototyping-features` from AGENTS.md) — worked around with
`harnez init --docs prototyping-features` as before; root cause remains open
in issue 315.
