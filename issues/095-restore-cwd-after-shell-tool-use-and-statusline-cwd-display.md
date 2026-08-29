# 095 — Advise agentic workers to restore the original working directory; surface `cwd` in the Claude Code status line

**Status**: Open — partially resolved (part 2 shipped, part 1 still open)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `internal/claude/apply.go`, `internal/statusline/statusline.go`, `cmd/harnez/statusline.go`, `docs/Bash.md`

## Problem

An agentic worker's shell tool (e.g. Claude Code's `Bash` tool) keeps its
working directory persistent across tool calls within a session. When that
shell is the same one the human is typing into interactively — not a
sandboxed subprocess — a `cd` the agent issues outlives the tool call and
silently changes the *human's* prompt too.

Concrete incident (2026-08-29, `ubunatic.com` session): the agent ran
`cd ~/projects/ubunatic.com && make -n ...` to inspect a Makefile. The `cd`
succeeded and the inspection worked, but it left the shared shell sitting
inside `ubunatic.com`. The user's very next command,
`make -C ubunatic.com` (typed expecting to still be in `~/projects`),
then failed with:

```
make: Entering directory '/home/uwe/projects/ubunatic.com'
make: *** ubunatic.com: No such file or directory.  Stop.
make: Leaving directory '/home/uwe/projects/ubunatic.com'
```

— because `-C ubunatic.com` was now resolving to
`ubunatic.com/ubunatic.com`, which doesn't exist. The error message gives
no hint that the root cause is "your shell isn't where you think it is";
the user had to ask "why do you always change my working directory?"
before the actual cause surfaced.

This project's own tooling already carries a related, narrower warning —
`CLAUDE.md`'s multi-repo guidance ("Give every repo-scoped command its own
explicit `cd <repo> && ...`... Don't rely on a prior command's `cd`
carrying over") — but that's about not leaking `cd` *between the agent's
own commands*, not about leaving the *human's* shell in a different place
than it started. Nothing currently tells an agentic worker to leave the
shell exactly where it found it once a turn/tool call ends.

## Proposed Fix

### 1. Documented convention: restore `cwd` before yielding control

Add a rule — likely alongside `docs/Bash.md`'s existing shell-hygiene
rules, or wherever cross-project agent tool-use conventions are bundled —
along these lines:

- Prefer `-C <dir>`, subshells (`(cd dir && cmd)`), or absolute paths over
  bare `cd && cmd` chains, exactly as `CLAUDE.md`'s multi-repo section
  already recommends for a different reason.
- If a bare `cd` is unavoidable (e.g. a tool that only supports relative
  paths), the agent must `cd` back to the directory it started the turn in
  before finishing that tool call or handing control back — do not leave
  a shared, persistent shell in a different directory than where the human
  (or the next tool call) expects it.
- This applies specifically to environments where the shell tool's
  session is known or suspected to be shared with the user's own
  interactive terminal (not an isolated subprocess per call) — the
  failure mode here is invisible when the shell is private to the agent,
  and only bites when a human is also typing into the same session.

Bundle this via the existing `harnez apply` mechanism
(`internal/claude/apply.go`) so it propagates to every managed project's
`AGENTS.md`/`CLAUDE.md`, the same way other bundled sections (Bash
conventions, Git conventions, etc.) already do.

### 2. Show `cwd` in the Claude Code status line — RESOLVED

Implemented: `internal/statusline/statusline.go` reads the Claude Code
statusLine JSON payload from stdin (`cwd` / `workspace.current_dir`) and
prints it, tilde-collapsed relative to `$HOME`. Wired up as `harnez
statusline` (`cmd/harnez/statusline.go`) and installed into the **global**
`~/.claude/settings.json` by `harnez apply` (`internal/claude/apply.go`,
gated by a new `status_line: true` `config.yaml` key, added to
`managedSettingsKeys` so `diff`/`clean` pick it up like every other managed
setting). MVP scope, as specified: cwd only, no git branch, model, cost,
or session info.

**Researched constraint, not a design choice**: per the current official
docs (code.claude.com/docs/en/statusline, checked 2026-08-29), "the status
line renders in its own row above the built-in footer badges and does not
replace them" — a custom `statusLine` **cannot** share a line with Claude
Code's built-in hint footer (`esc to interrupt`, `? for shortcuts`, `hold
space to speak`). Configuring a `statusLine` does suppress most of those
footer hints, but the two remain architecturally separate rows in the
current tool. A second line is therefore unavoidable today — this is not
something `harnez` can design around, only something to note in case a
future Claude Code release changes the contract (re-check the docs above
before revisiting). `cwd`/`workspace.current_dir` are both present verbatim
in the stdin JSON, so no shell-out to `pwd` was needed.

Idempotency verified live against the real `~/.claude/settings.json`:
`harnez apply` → `harnez diff` shows the new key once, a second `apply`
run reports "No changes."

This turns "the shell silently isn't where you think it is" into
something visible at a glance in the prompt, instead of a confusing
downstream `make`/`git`/`cd` error several commands later — though it does
not by itself fix part 1 below; a human still has to notice the line
changed.

## Non-goals

- Not a fix for the specific `ubunatic.com` incident (already resolved
  live in that session by `cd`-ing back).
- Not implementing part 1 (the documented convention) here — still open.
- Not proposing that the agent's shell tool become fully sandboxed/private
  per call — that's a harness-level design decision with much broader
  scope than this ticket; the status line is the mitigation for whatever
  sharing model is actually in effect.
- Not adding git branch, model name, cost, or any field beyond `cwd` to
  the status line — explicitly out of scope for this MVP; a richer status
  line is a separate future ticket if wanted.
