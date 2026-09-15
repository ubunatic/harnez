# Claude Code Auto-Memory: Content Assessment & Cleanup Options

Research-only pass, no code changes. Scope limited to Claude Code's built-in
"auto-memory" feature (per-project `memory/` dirs under `~/.claude/projects/`).
Codex/other agents' memory mechanisms are out of scope for this report.

## 1. What's actually in there, and is it worth the upfront tokens?

Auto-memory lives at `~/.claude/projects/<project-slug>/memory/`, one dir per
project working directory. `MEMORY.md` in each dir is loaded into every
session's context unconditionally (confirmed by the harness's own system
prompt: "This directory already exists... MEMORY.md is always loaded into
your conversation context — lines after 200 will be truncated"). Individual
memory files (`feedback_*.md`, `user_*.md`, `project_*.md`, `reference_*.md`)
are loaded on demand when the agent judges them relevant, not upfront.

Surveyed all 26 project memory dirs under `~/.claude/projects/`:

| Project | memory/ size | MEMORY.md lines |
|---|---|---|
| harnez | 44K | 12 |
| projects (root) | 24K | 4 |
| lmcoder | 24K | 7 |
| voxi | 16K | 3 |
| psync | 12K | 2 |
| trafficsim | 12K | 2 |
| ubunatic-com | 8K | 1 |
| proctop | 8K | 1 |
| books | 8K | 1 |
| 17 others | 0 (empty dir, never populated) | — |

harnez is the heaviest user by a wide margin — 11 memory files, 2846 words
total, ~44K on disk. The other 8 populated projects are lightly used (1-7
index lines). 17 of 26 projects have an empty `memory/` dir that was created
but never written to.

**Token cost**: only `MEMORY.md` is unconditionally injected, and it's
explicitly capped at 200 lines. harnez's is 12 lines / ~255 words — call it
~350-400 tokens upfront, every session, whether or not the conversation ever
touches the topics indexed. That's cheap in absolute terms. The real cost
is indirect: each `MEMORY.md` line is a *recall hook* — it primes the agent to
`Read` the full memory file "when relevant," and the fork/subagent-spawning
guidance elsewhere in the system prompt means an over-eager relevance match
can pull a 300-word feedback file into context for a task that only
tangentially resembles the memory's trigger. Reviewing harnez's 11 files:

- **Well-targeted, likely load-bearing**: `feedback_no_per_ticket_branches`,
  `feedback_no_worktree_isolation_for_sequential_tickets`,
  `feedback_harnez_rate_after_tool_calls`, `feedback_check_session_trailer` —
  these encode corrections that aren't otherwise written down anywhere in
  repo docs (CLAUDE.md/AGENTS.local.md), so memory is doing real work here.
- **Partially redundant with docs**: `reference_harnez_kernel_standard_metrics_policy`
  points at `docs/studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md`,
  which already exists in-repo and is discoverable via normal doc reading.
  The memory entry adds a pointer but the underlying fact is repo-durable,
  not session-ephemeral.
- **Cross-project pollution risk**: `reference_voxi_tui_prior_art` and
  `feedback_tui_terminal_bugs_need_pty_repro` are filed under the harnez
  project memory but describe voxi-specific/TUI-general lessons — they'll
  surface (or fail to surface) based on which project's memory dir happens
  to be active, not based on actual relevance.
- **Borderline "user preference" vs. project-local fact**:
  `user_harnez_usage_watch_workflow` is genuinely about how *this user* works
  and belongs in memory (can't be derived from repo state).

Net assessment: the content is small, cheap, and mostly non-duplicative of
repo docs, but roughly a third of it (reference-doc pointers, cross-project
TUI lessons) would be equally or better served by a repo doc + an
"@docs/..." reference than by an agent's personal memory store — see §3.

## 2. Cleanup: does Claude Code support it, and how?

No first-party command exists. Checked:
- `claude --help` — no `memory` subcommand, no `clear`/`reset`/`forget`
  verb targeting memory (the only `reset` hit is `auto-mode` classifier
  config, unrelated).
- `~/.claude/settings.json` / `settings.local.json` — no memory-related keys.
- No `/memory` slash command exists in `~/.claude/commands/`.

The only documented lever is `claude --bare`, which *disables* auto-memory
(among hooks, LSP, plugin sync, attribution, background prefetches) for that
invocation — it doesn't clear existing memory, just skips writing/reading it
for the session.

Practical cleanup today is filesystem-level only:
```sh
rm -rf ~/.claude/projects/<project-slug>/memory/*
```
or selectively removing individual `*.md` files and re-running the index
sync by hand (there's no reindex command either — `MEMORY.md` is maintained
by the agent editing it directly per the memory-writing protocol in the
system prompt, not by a tool).

Because 17/26 project dirs have an empty `memory/` with no `MEMORY.md`, an
agent asked to "check memory" in those projects should report "no memory
recorded" rather than silently doing nothing — worth confirming agents
do this (not verified in this pass).

## 3. Should agents be told to prefer repo docs over memory?

Yes, with one clarification: harnez's own memory instructions (baked into
the system prompt, not something this repo controls) already say **not** to
save things "derivable from reading the current project state" or "already
documented in CLAUDE.md files." The gap isn't the rule — it's that nothing
prompts an agent to *check* repo docs first before reaching for memory, and
the harnez-specific case above (`reference_harnez_kernel_standard_metrics_policy`
duplicating a `docs/studies/` file) shows the rule doesn't self-enforce.

A short addition to `AGENTS.local.md` or a harnez-managed doc along the lines
of:

> Before writing a memory entry, check whether the fact is already (or
> should be) in a repo doc (`docs/`, `CLAUDE.md`, `AGENTS.md`). Repo docs are
> shared and versioned; personal memory is private to this agent+project
> pair and invisible to teammates, other tools (Codex), and code review.
> Memory is for things repo docs structurally can't hold: this user's
> working style, corrections given in chat, cross-session task state.

would close that gap cheaply. This is a docs/process change, not a code
change — no CLI work implied.

## 4. Should harnez add a `harnez clear`-style memory command?

Worth scoping narrowly if pursued — recommend as a candidate ticket, not
implementing now:

**In favor**: no first-party way exists today (§2); a project accumulating
stale/wrong memory (e.g. a corrected preference that's since changed) has no
clean path except manual `rm`. A `harnez memory` subcommand
(`list`/`show`/`clear [--project <dir>]`/`prune-empty`) would also let
`harnez find`-style discovery apply here instead of raw `ls`.

**Against / open questions**:
- This is Claude-Code-specific state living outside any repo harnez tracks
  (`~/.claude/projects/...`), not `issues/`-shaped — it's a different kind
  of "managed" than harnez's existing docs/issues sync model.
- Scope question: per-project clear only, or also handle the 8-other-agent
  generalization implied by "all known agents' memory" in the original ask?
  This report only assessed Claude Code; Codex and other tool memory stores
  weren't inventoried, so a cross-agent `clear` command would need that
  survey first.
- Low urgency: current storage is small (largest project 44K) and no
  evidence yet of memory causing incorrect agent behavior in this repo —
  the risk is theoretical (stale entries) not observed.

Recommendation: file this as a P2/P3 issue for future consideration rather
than building now; the immediate low-cost win is the docs-first reminder in
§3.
