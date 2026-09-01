# Study: Claude Code clean-session system-prompt audit (repetition & prunability)

<!-- harnez:topic: Clean-session self-audit method (`claude -p` / `claude -c -p` in an empty scratch dir): Claude Code's own harness-native system prompt measured for repetition and prunability, and why the Tool Feedback Protocol directive gets skipped despite being correctly injected -->

**Date**: 2026-08-31
**Scope**: First completed audit for [128](../../issues/128-per-agent-full-system-prompt-self-audit-for-repetition.md) —
a two-turn `claude -p` / `claude -c -p` conversation, run in an empty scratch directory with no
project `CLAUDE.md`/`AGENTS.md`, asking Claude Code to introspect its own live system prompt for
size, cross-layer repetition, and generic-vs-harness-specific content.
**Related Issues**: [128](../../issues/128-per-agent-full-system-prompt-self-audit-for-repetition.md),
[040](../../issues/040-agent-context-duplication-and-file-read-discipline.md),
[122](../../issues/122-agent-instruction-tool-feedback-protocol.md)
**Status**: One agent (Claude Code), one clean-baseline pass done. Other agent environments and a
with-project-docs pass are still open per 128's scope.

---

## 1. Method

Ran in a throwaway directory (`scratchpad/clean-audit/`, no `CLAUDE.md`, no `AGENTS.md`, no
harnez-managed files) so the only project-layer content that could load was whatever global
config applies regardless of directory:

```
claude -p "<introspection prompt>" --output-format json
claude -c -p "<follow-up prompt>" --output-format json   # same session, resumed via -c
```

Turn 1 asked for a self-report on the session's actual delivered system prompt (not a guess at
what a typical one contains): rough size by section, internal repetition, and a
generic-best-practice vs. harness-load-bearing split. Turn 2 asked two targeted follow-ups: why one
real instruction (the `harnez rate` protocol) went unenforced while a structurally similar one
(the git commit trailer) is reliably followed, and which one of the found repetitions to cut first.

Session id: `e7d68bad-1317-4f29-9145-acafd6e9de1e`.

## 2. What the clean baseline actually contains

Even with zero project docs on disk, the session's system prompt was **not** empty — the global
`~/.claude/CLAUDE.md` loaded anyway, "Tool Feedback Protocol" (the `harnez rate` directive) and
all. This confirms that section is genuinely session-global, not something contributed per-project
by `init`/`apply`'s project-scoped writes — a fact the audit needed to check rather than assume,
since 128's scope explicitly asks which layer each piece of content actually comes from.

Reported composition (self-estimated, not a real tokenizer):

- ~5,000–6,000 tokens of prose instructions.
- ~3,000–4,000 tokens of tool JSON schemas.
- Ten identifiable sections: tool-use preamble, coding-task philosophy, action-risk framework, git
  workflow (commit + PR), tool-usage-parallelism guidance, tone/style, text-output rules,
  harness-specific session guidance (subagents/skills), the auto-memory spec, and the environment
  block.

## 3. Repetition found

| Instance | Locations | Notes |
|---|---|---|
| Emoji restraint | "Doing tasks", "Tone and style", Write tool's own description | 3x, each a static, context-free restatement |
| Destructive-op caution | dedicated Git Safety Protocol, general "Executing actions with care" | 2x, but with *different* worked examples each time — the model judged this repetition as adding value rather than being pure waste |
| Brevity / no-narration | "Tone and style", "Text output" | 2x, functionally overlapping sections |

Turn 2's own prioritization: if only one cut were allowed, drop the Write-tool-description copy of
the emoji rule (cheapest, most context-free redundancy) and keep "Tone and style" as the canonical
location, since that section is the one actually named for output conventions. It explicitly did
**not** recommend collapsing the destructive-op repetition, because the two instances carry
different examples and therefore different marginal instruction value — a useful distinction for
128's follow-up trimming work: repetition of the *same fact* is a pure cut candidate, repetition of
the *same rule with different worked examples* is not automatically waste.

## 4. Why one real instruction gets missed and another doesn't

This was the direct trigger for filing 128 in the first place (see 2026-08-31 conversation: the
`harnez rate` call was skipped on the very first tool call of a session, despite being present in
context from turn one). Asked to diagnose this about itself, the audited session pointed to
structural differences rather than restating "it's low priority":

- The git-trailer rule sits inside a section named for exactly that behavior ("Committing changes
  with git"), is exercised as a literal copy-this-template step every time the commit workflow
  runs, and is rehearsed into habit by repeated use within a session.
- The `harnez rate` line sits under a generic "Tool Feedback Protocol" heading in the *global*
  CLAUDE.md, phrased as standing background policy ("After executing any internal tool...
  immediately record") rather than tied to a specific upcoming action. It is also the only
  instruction in that file demanding a side-effecting shell command with **no example invocation
  shown working, no confirmation it's wired up, and no stated consequence for skipping** — it reads
  as policy-as-prose, not an operational step, so nothing in normal flow ever triggers "now go run
  this."

This generalizes past this one directive: instructions that are procedural, exercised on every use,
and demonstrated with a concrete template get internalized; instructions that are declarative
policy statements with no example, no wiring confirmation, and no stated cost of skipping do not,
regardless of how early in the session they were present.

## 5. Implications for 128's follow-up trims

Concrete, low-risk candidates surfaced by this one pass:

1. Drop the Write-tool description's inline "no emojis" restatement; keep "Tone and style" as
   canonical (model's own recommendation, turn 2).
2. Consider merging "Tone and style" and "Text output" — the audited session read them as
   functionally overlapping rather than covering distinct concerns.
3. For any future *global*, side-effecting agent directive (not just `harnez rate`): pair it with
   a concrete example invocation and, if possible, tie it to a specific triggering action rather
   than stating it as standing background policy — per §4, that is what actually gets a directive
   followed versus silently present-but-inert.

None of these were applied in this pass — 128 scopes implementing trims to follow-up tickets, not
this audit itself.

## 6. Open scope (unfinished per 128)

- This is a single clean-baseline pass for one agent (Claude Code) with **no project docs loaded**.
  128 also calls for a pass *with* this repo's full doc stack (global + project CLAUDE.md +
  `AGENTS.local.md` + bundled `docs/*`) to measure repetition across that larger, real stack, not
  just the harness-native baseline measured here.
- Other agent environments (Prime Agent / `~/.prime/agent`, others listed in `docs/README.md`) have
  not been audited yet.
- No follow-up trim tickets have been filed yet for the three items in §5 — 128's acceptance
  criteria still require these before it can close.
