# 878 — retro skill: session report from compaction summaries and key user messages

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[241-session-tree-tool-call-efficiency-audit-command-and-agent-skill]],
[[635-harnez-decide-fast-verbatim-transcript-compaction-explore-picker-skills]], skills `story`
and `evergreen` (`~/.claude/skills/`), ubunatic.com `docs/studies/2026-10-23-session-retro-staging-jam-sitegen-headbox-viewer.md` (the manual run this ticket repeats)

---

## 1. Goal

/goal A skill (working name `retro`) turns one long agent session into a candid study in
`docs/studies/` from a cheap extract (compaction summaries plus the owner's own messages), without
reading the raw transcript, and hands durable rules to `evergreen`; or stop and report when a
provider's transcript format offers no compaction summaries to extract.

## 2. Problem & Motivation (owner, 2026-10-23)

A three-day Claude Code session in ubunatic.com (17 compactions) produced most of what we know
about running a website through agents, but its lessons lived only in the compacted transcript.
The raw transcript was 94 MB (tool output, screenshots, agent logs), far too expensive to read.

The manual run that worked: `jq` extracted the 17 compaction summaries (178 KB) and the owner's
102 messages (16 KB), about 50k tokens; the host added `git log` counts per repo and the ticket
closes, then wrote a story-shaped study and moved two durable rules into an evergreen doc. Cost
was well under 1% of the transcript.

`story` today gathers ground truth from git only (status, log, diff) and loses everything a
compaction dropped; `evergreen` "recaps the current session" from live context, which after many
compactions is just the last summary. Neither knows the transcript on disk.

## 3. Proposal

1. **Extract (no model tokens).** A harnez command or script, e.g.
   `harnez session extract [--session <id>] [--out <dir>]`, writes two files:
   - every compaction summary with its timestamp;
   - the owner's own messages, with injected text stripped: system reminders, task
     notifications, skill bodies, command wrappers, tool results, the "continued from a
     previous conversation" preamble.
   Keep slash-command arguments (the manual run lost `/lean-sprint 097`-style messages) and mark
   pasted images as `[Image #n]` placeholders. Print sizes and an estimated token count.
2. **Ground truth.** Commits per repo since session start, tickets filed and closed, studies
   written, agents started (`harnez agent list` session footer).
3. **Write.** The `story` section structure (context, summary, what worked, post-mortem, quality
   table, efficiency, learnings, files), plus a "method and gaps" note naming what the extract
   cannot see. Go back to the raw transcript only with targeted `grep` for a specific fact.
4. **Hand off.** Learnings that are durable go to `evergreen`'s targets (repo docs, AGENTS.md);
   harnez-managed docs and skills get proposals, not edits. Proposed harness changes are listed
   for the owner to decide.

Alignment to settle in the design: either `retro` is a mode of `story` ("story from transcript")
that `evergreen` and `wrap` can call, or a separate skill that both reference. Avoid a third copy
of the story template.

## 4. Findings from the manual run (for the design)

- Compaction summaries repeat earlier content; the "Errors and fixes" and "All user messages"
  sections carry most of the signal, so section-wise extraction can shrink input further.
- Owner messages with a leading `<system-reminder>` block must be stripped, not dropped, or real
  messages vanish.
- Claude Code stores transcripts as JSONL under `~/.claude/projects/<dir>/<session>.jsonl`;
  summaries carry `isCompactSummary: true`. Codex and agy formats need their own probe
  (canary first).
- Pasted images are not in the transcript text; a retro cannot judge visual feedback it never saw.

## 5. Acceptance

- The extract of the ubunatic.com session `13d4828e-…` reproduces the manual numbers (17
  summaries, about 100 owner messages) and keeps slash-command arguments.
- One retro written with the skill on another long session, reviewed by the owner.
