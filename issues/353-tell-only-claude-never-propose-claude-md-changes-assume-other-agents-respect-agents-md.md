# 353 — Tell only Claude: never propose CLAUDE.md changes; check repo docs/AGENTS.md before any instruction-file change

**Status**: Open
**Priority**: P1
**Severity**: Minor
**Category**: Docs

---

## Summary

Two related instructions, both about agent instruction-file habits:

1. A Claude-Code-specific instruction (not a general cross-agent rule — scope this to
   Claude only, since other agent harnesses this project supports don't necessarily read
   `CLAUDE.md` at all): Claude should never propose edits framed around `CLAUDE.md` as the
   managed source file. It should always work in terms of `AGENTS.md` as the single managed
   source, and assume that other agents already active on a repo (Codex, Gemini, Prime
   Agent, etc.) respect `AGENTS.md`, not a Claude-specific file.
2. A broader discipline (see "Extension" section below): before any agent proposes a
   change to its own instruction file at all, it should first check whether the repo's
   own `docs/` and `AGENTS.md` already cover the topic, or should — instruction-file
   edits are for agent-specific habits, not a substitute for shared, versioned project
   documentation.

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

## Extension (2026-09-15): docs-first discipline before proposing instruction-file changes

Broader than the Claude-only rule above — this applies to any agent working in a repo,
regardless of which instruction file it personally reads (`CLAUDE.md` for Claude,
`AGENTS.md` for Codex/others, `instructions.md` for other harnesses).

**Problem**: an agent that jumps straight to "let's add a line to my instructions file"
when it hits friction or learns something skips a cheaper, more durable step: checking
whether the repo's own `docs/` and `AGENTS.md` already cover it, or should. This project's
own `docs/evergreen` skill already encodes the right ordering for *session recaps*
(evergreen docs and issues first, instruction-file changes only proposed to the user, not
applied automatically) — but that discipline should apply any time an agent is about to
suggest an instruction-file edit, not only during an explicit `/evergreen` pass. Symptom
seen this session: multiple tickets (348, 349, 351, 352, 353 itself) proposed instruction-
file additions before checking whether `docs/Spec.md`, `docs/CLIDesign.md`, or
`docs/Permissions.md` already said (or should say) the same thing at the evergreen-doc
layer, where it's shared, versioned, and visible to every agent — not just the one whose
instruction file gets edited.

**Task**: add a rule — for Claude specifically, and worth phrasing so it's easy to port
into `AGENTS.md` for other agents too — along these lines:

> Before proposing a change to your own instruction file (`CLAUDE.md`/`AGENTS.md`/
> `instructions.md`), first check whether the repo's own `docs/` (evergreen docs) and
> `AGENTS.md` already cover the topic, or whether the learning belongs there instead.
> Evergreen docs are shared across every agent working on the repo and are the right home
> for durable decisions, pitfalls, and architecture notes; an instruction-file edit should
> be reserved for agent-specific behavioral habits (like this ticket's own CLAUDE.md-vs-
> AGENTS.md example) that don't belong in shared project documentation. When in doubt,
> propose the `docs/` change first and only add an instruction-file pointer if the repo
> docs alone wouldn't surface it reliably.

**Definition of done (extension)**:
- Rule is live wherever the rest of 353's Claude-specific instruction lands.
- Doesn't contradict `docs/evergreen`'s existing "propose harness improvements, let the
  user decide" step-3 guidance — this extension is about the *order of consideration*
  (repo docs/AGENTS.md first) before that proposal is even made, not about removing the
  user's final say.

## Related

- Issue 351, 352 — both had ticket drafts this session that incorrectly referenced
  `CLAUDE.md` as the managed file before correction.
- `docs/CLIDesign.md` — `init flow` (project-level: `AGENTS.md` primary, `CLAUDE.md`
  symlink) and `apply flow` (global-level: `CLAUDE.md` primary, `AGENTS.md` symlink) —
  read both before implementing, the direction differs by scope.
- `/evergreen` skill — already encodes docs-first discipline for session recaps; this
  ticket's extension generalizes that ordering to any instruction-file proposal, not only
  explicit evergreen passes.

## Update (2026-09-21): unbreakable rule, spelling, hook, placement

**Decisions from the user:**

- "We never talk about CxxxE.md!" is an **unbreakable rule for Claude only**. Other agents work
  fine with `AGENTS.md`, so the rule must not land in content they also read.
- Claude always edits `AGENTS.md` and assumes the CxxxE.md symlink exists (it does at project
  level; `init` creates it). Claude may still know the global `~/.claude` file is real.
- When Claude MUST mention the file, it writes it as **`CxxxE.md`**, because this project works on
  Claude's own logic and the literal name pollutes searches and tickets. Use this spelling in this
  ticket and everywhere from now on.
- **Hook**: a Claude hook counts mentions of the literal filename in Claude's own messages; every
  5 mentions, emit a `harnez tip` warning that re-teaches the rule. Open question for planning:
  which hook event exposes the assistant's message text (Stop or a transcript read) and where the
  counter lives (session-state file, like the session-tip hook).

**Placement (settled in the same session):** the root `AGENTS.md` is not a pure template. It holds
authored rules outside the managed blocks, so *repo-wide* rules go there (now documented in
`AGENTS.md` under "Where Repo Rules Go"). `AGENTS.local.md` is git-excluded (`.git/info/exclude`),
so it is not a place for durable rules. The Claude-only rule cannot go in `AGENTS.md`; use an
agent-scoped instruction profile (see 149) or a Claude-only hook message, and confirm during
planning which Claude-only channel already exists.

**/goal (updated):** Claude sessions in harnez-managed repos never propose or edit the literal
CxxxE.md, always target `AGENTS.md`, spell the file `CxxxE.md` when unavoidable, and get a
`harnez tip` every 5 mentions. Verified by a hook test and a Claude-only instruction that Codex/agy
profiles do not receive.

## Lean sprint (2026-09-24): raised to P1 by the user

Preflight: no Claude-only instruction or hook for this exists yet. The host (a Claude session) used the
literal name several times today without any warning.

### M1 — Claude-only instruction channel + rule text
- Find or create a channel that only Claude sessions load (e.g. an `apply`-managed block in the global
  `~/.claude` instructions, or a Claude-only SessionStart hook message). Codex/agy/Pi must not receive it.
- Rule text: never propose or edit the literal CxxxE.md; always edit `AGENTS.md`; write `CxxxE.md` when
  the name is unavoidable.
- Tests: the Claude channel contains the rule, and the Codex/agy outputs do not.

### M2 — mention counter hook
- A Claude hook counts literal-filename mentions in Claude's own messages and emits a `harnez tip`
  every 5 mentions. Pick the event that exposes assistant text (Stop + transcript tail) and keep the
  counter in the session-state file. Tests for counting and the every-5 cadence.
