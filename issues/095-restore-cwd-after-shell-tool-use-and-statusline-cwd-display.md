# 095 — Advise agentic workers to restore the original working directory; surface `cwd` in the Claude Code status line

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `internal/claude/apply.go`, `docs/Bash.md`

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

### 2. Show `cwd` in the Claude Code status line

Even with the convention above, directory drift can still happen (a tool
crashes mid-`cd`, a convention gets missed, a different agent/tool doesn't
follow it). Make it visible rather than silent: extend `harnez apply` to
install/update a Claude Code `statusLine` command
(`.claude/settings.json` → `"statusLine": {"type": "command", "command":
"..."}`) that renders the current working directory the status line script
receives (Claude Code's status line payload already includes
`workspace.current_dir` in the JSON piped to the command — no new Claude
Code capability needed, this is purely a `harnez`-side config/script
addition).

This turns "the shell silently isn't where you think it is" into
something visible at a glance in the prompt, instead of a confusing
downstream `make`/`git`/`cd` error several commands later.

## Non-goals

- Not a fix for the specific `ubunatic.com` incident (already resolved
  live in that session by `cd`-ing back).
- Not implementing either the convention doc or the status line change
  here — this ticket is the proposal only.
- Not proposing that the agent's shell tool become fully sandboxed/private
  per call — that's a harness-level design decision with much broader
  scope than this ticket; the status line is the mitigation for whatever
  sharing model is actually in effect.
