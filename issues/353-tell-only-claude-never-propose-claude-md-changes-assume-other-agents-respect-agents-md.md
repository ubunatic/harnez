# 353 — Tell only Claude: never propose CLAUDE.md changes, assume other agents respect AGENTS.md

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Docs

---

## Summary

Add a Claude-Code-specific instruction (not a general cross-agent rule — scope this to
Claude only, since other agent harnesses this project supports don't necessarily read
`CLAUDE.md` at all): Claude should never propose edits framed around `CLAUDE.md` as the
managed source file. It should always work in terms of `AGENTS.md` as the single managed
source, and assume that other agents already active on a repo (Codex, Gemini, Prime
Agent, etc.) respect `AGENTS.md`, not a Claude-specific file.

## Background

This project's own tooling (`harnez init`/`apply`) already treats `AGENTS.md` as the
authored source and `CLAUDE.md` as a generated symlink at the project level (see
`docs/CLIDesign.md`'s `init flow`: "create `AGENTS.md` (template)... create `CLAUDE.md`
symlink → `AGENTS.md`"). Despite that, this session repeatedly drafted ticket text
proposing changes "to this project's `CLAUDE.md`" when the actual managed/authored file
is `AGENTS.md` — caught and corrected by the user twice in close succession (issues 351
and 352's drafts). This is a Claude-specific blind spot worth encoding directly rather
than relying on the user to catch it every time.

Scope note: at the *global* `~/.claude` level, `apply`'s own flow writes
`~/.claude/CLAUDE.md` first and symlinks `~/AGENTS.md` to it (the reverse direction from
project-level `init`) — see `docs/CLIDesign.md`'s `apply flow`. This ticket's instruction
should account for that asymmetry rather than overcorrecting into a blanket claim that
`AGENTS.md` is always the literal symlink target at every level; the actionable rule for
Claude is behavioral ("phrase proposals in terms of `AGENTS.md`, don't treat `CLAUDE.md`
as the thing other agents/tools care about"), not a factual claim about which file is a
symlink to which in every context.

## Task

Add an instruction, scoped to Claude Code specifically (not a project-wide `AGENTS.md`
rule that other agents would also load, since it's about Claude's own habits), something
like:

> When proposing documentation or convention changes in this project, always frame them
> in terms of `AGENTS.md`, not `CLAUDE.md`. Assume other agents working on this repo
> (Codex, Gemini, Prime Agent, etc.) read `AGENTS.md`, and that any Claude-specific
> `CLAUDE.md` content is a generated/symlinked artifact of it, not something to edit or
> reference directly in tickets, commit messages, or proposals.

Placement: likely Claude-specific config (e.g. this project's `CLAUDE.md`/`AGENTS.local.md`
local overrides section, or wherever Claude-only behavioral instructions already live —
check how other Claude-only corrections are scoped in this repo before picking a spot, so
this doesn't leak into content other agents also read).

## Definition of done

- Instruction is live somewhere Claude actually loads it, and does not appear in content
  other agent harnesses (Codex/Gemini/Prime Agent) would also read.
- Verify it doesn't contradict the global-level `apply` flow asymmetry noted above (Claude
  should still understand `~/.claude/CLAUDE.md` is a real file it manages globally — the
  rule is about not treating it as the thing to *propose changes to* in project-local
  ticket/doc work, not about pretending the file doesn't exist).

## Related

- Issue 351, 352 — both had ticket drafts this session that incorrectly referenced
  `CLAUDE.md` as the managed file before correction.
- `docs/CLIDesign.md` — `init flow` (project-level: `AGENTS.md` primary, `CLAUDE.md`
  symlink) and `apply flow` (global-level: `CLAUDE.md` primary, `AGENTS.md` symlink) —
  read both before implementing, the direction differs by scope.
