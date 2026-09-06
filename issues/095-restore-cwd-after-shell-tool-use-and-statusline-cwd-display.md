# 095 — Advise agentic workers to restore the original working directory; surface `cwd` in the Claude Code status line

**Status**: Closed — part 1 resolved in 22503de + 0ccea9f (docs/lang/Bash.md §8 + config.yaml WD Hygiene bullet); part 2 (statusline) previously shipped
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

**Sharpened motivation (2026-08-29, same session as the statusLine fix
above):** this isn't only about tidiness. Claude Code asks once, at
session start, whether it's okay to work in a given project directory —
that prompt is the trust boundary for the whole session. But nothing
re-checks or re-confirms once the Bash tool is approved: a `cd` inside any
approved Bash call can take the shell anywhere the OS user can reach, with
no further gate. The one-time directory prompt implies "I'm scoped to this
folder"; the actual capability is "any Bash call can navigate anywhere."
A documented convention (this section) is a **mitigation an agent can
choose to follow**, not an **enforcement mechanism** — it doesn't close
that gap, it just makes a well-behaved agent less likely to fall into it.
Actually closing the gap (re-confirming or hard-scoping directory access
after the initial prompt) is outside what `harnez apply` can deliver — it
would require a Claude Code product-level permission change, not a
project-doc convention. Filed as product feedback separately; this ticket
stays scoped to the advisory-convention mitigation.

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

---

## Implementation Plan

Scoped to **part 1 only** (the documented convention). Part 2 shipped; nothing to plan there.

### Where the rule belongs

Two candidate homes, and the answer is **both, with different weight**:

- **`config.yaml` → `agents_md.global.sections`** (rendered into `~/.claude/CLAUDE.md` by
  `harnez apply`, via `internal/claude/apply.go` → `markdown.Apply`). This is the
  load-bearing surface: it lands in *every* session's system prompt, in every project,
  automatically. The existing `Instructions Hierarchy` section already carries exactly this
  class of cross-cutting tool-use rule (Context Discipline, Editing Discipline, Voice
  Input), so this rule is a sibling bullet, not a new mechanism.
- **`docs/lang/Bash.md`** — the copyable Bash conventions doc. Good for the *long* version
  (rationale, the `ubunatic.com` incident, the trust-boundary caveat), but it is only
  loaded when a project opts in via `init --docs Bash`, so it cannot be the only home.

Constraint from `CLAUDE.md`'s own "Minimal Global Docs" rule: the global file must stay
short. So keep the global text to **2–3 lines max**, and put the reasoning in `Bash.md`.

### Steps

1. **`docs/lang/Bash.md`** — add a new numbered section (after §6 "Commands & Traps", before
   §7 "Functions"; renumber accordingly, or append as a new §8 "Working-Directory Hygiene"
   to avoid renumbering churn) covering:
   - Prefer `make -C <dir>`, `git -C <dir>`, absolute paths, or a subshell `(cd dir && cmd)`
     over a bare `cd dir && cmd` chain.
   - If a bare `cd` is unavoidable, `cd` back before the tool call ends.
   - Why: an agent shell tool's cwd can be shared with the human's interactive shell; the
     failure surfaces several commands later as an unrelated-looking `make`/`git` error.
   - Explicit note that this is an **advisory mitigation, not enforcement** — carry over the
     ticket's "Sharpened motivation" paragraph in condensed form so the doc doesn't overclaim.

2. **`config.yaml`** — add one bullet to the existing `agents_md.global.sections` entry named
   `Instructions Hierarchy` (currently ~lines 304-322), in the same style as the neighbouring
   `Context Discipline:` / `Editing Discipline:` bullets:
   ```yaml
   - Working-Directory Hygiene: prefer `make -C`/`git -C`/absolute paths or a subshell
     over bare `cd`; if you must `cd`, return to the starting directory before the tool
     call ends — the shell may be shared with the user's own terminal.
     See @docs/Bash.md.
   ```
   Do **not** create a new top-level section for this — a new section means a new managed
   `harnez:begin/end` block in every project's CLAUDE.md, and this rule is one bullet.

3. **Verify propagation**: `harnez diff` (dry) should show the `Instructions Hierarchy`
   section changing exactly once; a subsequent `harnez apply` then `harnez diff` must report
   "No changes" (idempotency). `scripts/smoke-test.sh` covers this loop.

4. **Tests**: `internal/claude`'s existing apply/diff tests operate on section content
   generically, so no new Go test is strictly required. If `config.yaml` content is asserted
   anywhere (check `internal/claude/*_test.go` for hardcoded section text), update that
   fixture; otherwise no code change at all — this ticket is config + docs only.

### Design decisions

- **One bullet in an existing section, not a new section.** Adding a section costs a managed
  block in every downstream `CLAUDE.md` forever; the rule doesn't earn that.
- **No enforcement mechanism.** The ticket already establishes that hard-scoping cwd is a
  Claude Code product-level change, outside `harnez apply`. Do not attempt a hook-based
  guard (e.g. a `PostToolUse` hook that `cd`s back) — hooks run in their own process and
  cannot mutate the agent shell's cwd, so it would silently do nothing.
- Keep wording tool-agnostic ("the shell tool") rather than naming Claude Code's `Bash`
  tool, since the same global file is applied to Codex/AGY harnesses too.

### Risks / open questions

- Whether the shell tool's session is actually shared with the user's terminal varies by
  harness and by version; the rule is phrased as a cheap always-on habit precisely so it
  doesn't depend on getting that answer right.
- Minor conflict risk with the `~/projects/CLAUDE.md` "Multi-repo shell commands" section
  (uman-managed, not harnez-managed), which already says something adjacent for a different
  reason. Cross-reference it rather than restating it, so the two don't drift.

### Scope

**Small** — one config bullet, one docs section, one smoke-test verification pass. No Go
code changes expected.
