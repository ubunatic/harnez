# 222 — Advise agents to use `git -C`/`make -C` etc. instead of `cd`-ing into dirs

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[095-restore-cwd-after-shell-tool-use-and-statusline-cwd-display]], `docs/Bash.md`

## Problem

Agents commonly write `cd <dir> && <cmd>` (or chain several `cd`s across a
shell call) to run a repo-scoped command. This has repeated failure modes:

- The shell tool's cwd persists across tool calls within one session; a
  `cd` in one call silently changes where the *next* unrelated call runs,
  producing wrong-repo results without erroring (this bit multi-repo
  workspace commands directly — `git status`/`git push` reporting on the
  wrong repo after a stray `cd`).
- It's one more thing to get wrong in a chained `&&` command, and it adds
  noise to the command for no benefit when the tool already supports a
  directory flag.
- It compounds with issue 095: even a *correctly scoped* `cd` still risks
  leaking into the human's interactive shell if the tool session isn't
  sandboxed.

Most CLIs agents reach for already support scoping a command to a
directory without changing cwd: `git -C <dir> ...`, `make -C <dir> ...`,
and equivalents in other tools (`npm --prefix`, etc.).

## Proposal

Add guidance (e.g. in `docs/Bash.md` or wherever multi-repo/shell command
conventions live) advising agents to prefer a directory flag over `cd`
whenever the underlying command supports one:

- `git -C <dir> status` instead of `cd <dir> && git status`
- `make -C <dir> <target>` instead of `cd <dir> && make <target>`
- Only fall back to `cd` when the tool has no such flag, and in that case
  keep the `cd` and the command in the same call so cwd doesn't leak into
  unrelated later commands.

## Related

- [[095-restore-cwd-after-shell-tool-use-and-statusline-cwd-display]] — cwd
  leaking into the human's interactive shell; this ticket is the
  complementary "avoid `cd` in the first place where possible" advice.
