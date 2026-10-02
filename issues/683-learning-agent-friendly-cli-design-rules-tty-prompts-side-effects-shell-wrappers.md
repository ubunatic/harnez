# 683 — Learning: agent-friendly CLI design rules (TTY, prompts, side effects, shell wrappers)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Documentation
**Related**: goto project `docs/Features.md` and issues 002, 003 (`~/projects/goto`, commit b6271f2); bundled `docs/Go.md` (CLI & Releases); `docs/CLIDesign.md` (harnez-internal apply/init design, not these rules)

---

## 1. Problem & Motivation

While planning `goto` (a smart `cd` used by both humans and agents), a design review found four
CLI mistakes that are easy to make in any tool that serves both audiences. None of them is covered
by the bundled docs: `docs/Go.md` only covers Cobra mechanics, and `docs/CLIDesign.md` is about
harnez's own apply/init split.

/goal Add these rules to the bundled Go/CLI guidance so every harnez-managed project gets them, or
stop and report when blocked on a user decision about where the rules live or a denied permission.

## 2. Technical Specification / Findings

The rules, as learned on `goto`:

1. **Decide prompting by stdin and stderr TTY, not stdout.** A shell wrapper (`cd "$(tool x)"`)
   captures stdout while a human still sits at the keyboard; an agent has no TTY at all. Testing
   stdout for "is a human here?" gets both cases wrong.
2. **Same stdout in every mode.** The primary result always goes to stdout, unchanged by TTY;
   TTY-only extras (hints) go to stderr. Then agents need no special flag, and `--print`-style
   flags shrink to "suppress hints".
3. **Non-interactive never prompts.** A step that needs confirmation (clone, `sudo`, `curl | sh`)
   fails with a clear stderr message and a distinct, spec-defined exit code unless `--yes` is
   given. Offer `--dry-run` for lookups that would otherwise have side effects (mount, clone,
   create), so agents can ask "where would it be?" safely.
4. **Shell-function installers must not silently shadow names.** An `init` that defines a shell
   function (needed because a child process cannot change the parent's cwd) must warn on stderr
   when it replaces an existing alias, function or command (e.g. oh-my-zsh's `g=git`). Keep the
   binary name and the function name separate, so the binary stays usable by agents.

Also considered and rejected: spawning a subshell in the target dir on a TTY (nested shells pile up).

Open question: target doc. Options: extend the bundled `docs/Go.md` "CLI & Releases" section, or a
new bundled language-neutral `docs/CLI.md` linked from `docs/Go.md`. The rules are not Go-specific.

## 3. Implementation & Verification Plan

- Decide the target doc (ask the user if unclear), add the four rules concisely, keep `docs/Go.md`'s
  token budget in mind.
- Run `harnez init` in one sibling project to check the doc lands as a bundled doc.
