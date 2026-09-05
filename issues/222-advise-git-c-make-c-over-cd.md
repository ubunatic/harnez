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

---

## Implementation Plan

### Approach

Pure guidance change, no Go code. The rule has two audiences with different
reach requirements, so it lands in two sizes:

- **Full section** in `docs/lang/Bash.md` (copyable, `init --docs bash`,
  auto-detected on `*.sh`) — the flag table and the fallback rule.
- **One line** in `docs/practices/AgenticLoop.md`'s anti-pattern list, which is
  `default: true` and therefore summarized into every managed project's
  `AGENTS.md`. This matters because agents run shell commands in *every* repo,
  including ones with no `*.sh` file that would never auto-install Bash.md.

### Steps

1. **`docs/lang/Bash.md`** — add `## 8. Directory Scoping — Prefer `-C` Over `cd`,`
   after `## 7. Functions` (before the Awk appendix). Contents:
   - Rule: if the command has a directory flag, use it; never `cd` for scoping.
   - Table of the common ones: `git -C <dir>`, `make -C <dir>`,
     `go -C <dir>` (Go 1.20+), `npm --prefix <dir>`, `cargo --manifest-path`,
     `docker build <ctx>`, `tar -C <dir>`, `rsync` trailing-slash semantics.
     Only list flags actually verified — check `git help -C`/`make --help`
     rather than trusting recall for the less common ones.
   - Fallback rule: when no flag exists, keep the `cd` and the command in **one**
     tool call (`cd <dir> && <cmd>`), never as a bare `cd` whose effect is meant
     to persist to a later call. Prefer a subshell — `(cd <dir> && <cmd>)` — so
     cwd is restored even within the call.
   - Why: the shell tool's cwd persists across calls, so a stray `cd` silently
     redirects an unrelated later command with no error (the wrong-repo
     `git status`/`git push` failure).
2. **`config.yaml`** — extend the `bash` doc entry's `hint:` (line ~373) with a
   one-liner: `Use git -C/make -C, not cd`. The hint is what lands in AGENTS.md;
   without this the section exists but isn't surfaced.
3. **`docs/practices/AgenticLoop.md`** — add to `### Anti-Patterns to Avoid`:
   *"❌ `cd`-scoped commands: `cd <dir> && git status` when `git -C <dir> status`
   exists — the shell tool's cwd persists into later, unrelated calls and
   silently targets the wrong repo."*
4. **`docs/README.md`** — update the `lang/Bash.md` row's summary to mention
   directory-flag scoping.
5. **Cross-reference issue 095** — add a line to this ticket's Related section in
   095 pointing back here if not already reciprocal, and note in the Bash.md
   section that even a correctly scoped `cd` can leak into the human's
   interactive shell when the session isn't sandboxed (095 part 1, still open).
6. Verify: `go test ./...` (docs-embedded specs are asserted by some tests),
   `make check`, `make install`, then regenerate an AGENTS.md into a scratch dir
   and confirm the new bash hint line renders.

### Design decisions / tradeoffs

- **Doc + summary hint, not a hook.** A lint hook that rejects `cd ... &&` in
  Bash tool calls would be enforceable rather than advisory, but it would produce
  false positives constantly (legitimate `cd` for tools with no flag, heredocs,
  subshells) and is well out of proportion to a P3. Advisory only.
- **Subshell form as the documented fallback.** `(cd <dir> && cmd)` is strictly
  safer than `cd <dir> && cmd` and costs two characters; documenting the safer
  form as the default fallback prevents the leak even when no `-C` exists.
- **AgenticLoop line is deliberately one line.** It is fleet-wide context; the
  detail belongs in Bash.md.

### Risks / open questions

- **Merge conflict with issue 221**, which also appends to AgenticLoop.md's
  anti-pattern list and edits `docs/README.md`. Sequence them; do not implement
  both in parallel agents.
- The `-C`-style flag is not universal and its spelling differs per tool
  (`--prefix`, `--manifest-path`, `--project-directory`). Verify each entry in
  the table against the tool's actual help output before publishing — a wrong
  flag in a guidance doc is worse than no guidance.
- Advisory guidance may simply not be followed; if the wrong-repo failure recurs
  after this lands, escalate to the hook idea in a new ticket rather than
  expanding this one.

### Scope

**Small** — three doc edits plus one `config.yaml` hint line.
