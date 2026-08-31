# Study: agy (Antigravity) clean-session system-prompt audit (repetition & prunability)

**Date**: 2026-08-31
**Scope**: Second completed audit for [128](../../issues/128-per-agent-full-system-prompt-self-audit-for-repetition.md) —
a two-turn `agy -p` / `agy -c -p` conversation, run in an empty scratch directory with no
project docs, asking agy (Antigravity) to introspect its own live system prompt for size,
cross-layer repetition, and generic-vs-harness-specific content. Repeats the method from
[the Claude Code pass](2026-08-31-claude-code-clean-session-system-prompt-audit.md), scoped down
to pure repetition analysis at the user's request — `harnez rate` isn't wired up for agy yet
(only Claude Code, per [122](../../issues/122-agent-instruction-tool-feedback-protocol.md)), so
this pass doesn't ask about instruction-following-vs-enforcement, only about the prompt content
itself.
**Related Issues**: [128](../../issues/128-per-agent-full-system-prompt-self-audit-for-repetition.md),
[040](../../issues/040-agent-context-duplication-and-file-read-discipline.md)
**Status**: Second agent (agy), one clean-baseline pass done. With-project-docs pass and other
agent environments still open per 128's scope.

---

## 1. Method

Same shape as the Claude Code pass: a throwaway directory with no project config files, two
`agy` invocations against the same conversation.

```
agy -p "<introspection prompt>" --output-format json
agy -c -p "<follow-up prompt>" --output-format json   # same session, resumed via -c
```

Turn 1 asked for a self-report on the session's actual delivered system prompt: size by section,
internal repetition (quoted), generic-vs-harness-specific split. Turn 2 asked which of the found
repetitions carries real risk if forgotten vs. which is cosmetic, and which pair to collapse into
one canonical location — explicitly scoped away from any tool-feedback/rating question, since
`harnez rate` has no agy-side wiring to evaluate yet.

## 2. What the clean baseline actually contains

Unlike the Claude Code pass, no external project-wide config (no `~/.claude/CLAUDE.md`-equivalent)
leaked into this empty directory — agy's reported system prompt was pure harness scaffolding, tag
by tag: `<identity>`, `<user_information>`, `<skills>`, `<subagents>`, `<messaging>`,
`<conversation_transcript>`, `<artifacts>`, `<slash_commands>`, `<guidelines>`,
`<communication_style>`, plus the tool JSON schema block.

Reported composition (self-estimated, not a real tokenizer):

- ~3,000–4,500 tokens total, smaller than Claude Code's ~8,000–10,000 combined estimate.
- ~1,000–1,500 tokens of that is tool JSON schemas — roughly a third of the whole prompt.
- Self-assessed 60–70% of the total is harness scaffolding (tool schemas + workflow plumbing);
  the pure style/behavior layer (`<guidelines>` + `<communication_style>`) is under 100 tokens.

## 3. Repetition found

| Instance | Locations | Notes |
|---|---|---|
| Anti-polling directive | `<subagents>` ("you do NOT need to poll or check your inbox in a loop"), `<messaging>` ("you do **NOT** need to poll in a loop") | Near-verbatim restatement |
| "Stop calling tools to end your turn" | `schedule` tool's own description, `<messaging>` ("simply stop by calling no more tools") | Same directive as anti-polling in different framing — the audited session judged these three instances as really one instruction stated three ways |
| `file://` links for code symbols | `<artifacts>` (embed/line-range links), `<communication_style>` (inline prose links) | Judged as *not* pure waste — different contexts (embed path vs. inline prose), see §4 |

## 4. Priority judgment: which repetition is load-bearing vs. cosmetic

Turn 2's own risk ranking:

- **Anti-polling is high-stakes.** A model that drifts back to instinct and polls in a loop burns
  tool calls, consumes context, and can stall a session indefinitely — a concrete, costly failure
  mode, not a cosmetic one.
- **"Stop calling tools" is redundant with anti-polling**, not a separate risk — dropping one copy
  is low-risk because the other already covers the same behavior.
- **`file://` link duplication is cosmetic.** A forgotten link degrades UX only; nothing breaks.
  The session explicitly recommended *keeping* this one as-is, since `<artifacts>` and
  `<communication_style>` cover genuinely different contexts (embed paths vs. inline prose links)
  — the same "different worked examples deserve their own copy" pattern found in the Claude Code
  pass's destructive-op-caution repetition.

Its own collapse recommendation: merge the anti-polling / "stop calling tools" pair into
`<messaging>` as the single canonical home (the section already dedicated to async communication
behavior), and pull the `schedule` tool's embedded copy out — a directive buried inside a JSON tool
schema is, in the session's own words, "easy to miss" and harder to update than one stated in a
dedicated behavioral section.

## 5. Comparing the two agents' patterns

- Both agents independently converged on the same shape of judgment: repetition of the *same
  fact/directive in the same context* is a genuine cut candidate; repetition that carries
  *different worked examples or different contexts* (Claude Code's destructive-op caution; agy's
  `file://` links) is warranted specialization, not waste. This suggests the "is it actually the
  same repetition" question is a more useful lens for future audits than raw repetition count.
- Both agents flagged directives buried inside tool-schema JSON as structurally worse than the
  same directive stated in a dedicated prose section — echoing the Claude Code pass's finding that
  the `harnez rate` line's problem was partly about being "policy-as-prose" with no clear trigger;
  here it's the inverse failure (buried in schema, not in prose) with the same root cause: the
  directive isn't where an agent's normal flow would naturally look for it.
- agy's prompt is smaller overall (~3-4.5k vs. Claude Code's ~8-10k tokens) and, in this empty-dir
  baseline, carried no external project-config leakage — a direct contrast worth carrying into the
  with-project-docs pass, since Claude Code's global `CLAUDE.md` loading unconditionally (even with
  zero project files present) may not have an agy-side equivalent.

## 6. Open scope (unfinished per 128)

- This is a single clean-baseline pass for agy with no project docs loaded. The with-project-docs
  pass (this repo's `AGENTS.md`/bundled-docs stack as agy would actually see it) is still open.
- No follow-up trim tickets have been filed for either agent's findings yet.
- `harnez rate` wiring for agy is out of scope for this study entirely — noted, not evaluated, per
  the user's explicit instruction that it isn't implemented yet.
