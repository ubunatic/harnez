# 128 — Per-agent full system-prompt self-audit for repetition and conciseness

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[040-agent-context-duplication-and-file-read-discipline]], [[122-agent-instruction-tool-feedback-protocol]], [[075-concisemode-promote-doc-to-real-skill]], [docs/studies/2026-08-31-claude-code-clean-session-system-prompt-audit.md](../docs/studies/2026-08-31-claude-code-clean-session-system-prompt-audit.md), [docs/studies/2026-08-31-agy-clean-session-system-prompt-audit.md](../docs/studies/2026-08-31-agy-clean-session-system-prompt-audit.md)

## Problem

We keep adding instruction content (global `CLAUDE.md`, project `CLAUDE.md`,
`AGENTS.local.md`, bundled `docs/lang|practices|other/*.md`, memory index)
without ever measuring the compounding cost from the agent's own point of
view: how much of what lands in its live system prompt is model-native
boilerplate it already knows, how much is duplicated across layers, and how
much is genuinely load-bearing harness/project-specific instruction. The
`harnez rate` tool-feedback-protocol line (see conversation of 2026-08-31)
is a concrete instance: a real, already-configured directive that got missed
because it was buried, unemphasized, in a large stacked instruction block —
exactly the failure mode 040 flagged for file-read duplication, but here
applied to the instruction stack itself rather than to doc re-reads.

We have no per-agent visibility into:
- What the assembled system prompt actually contains end-to-end (harness
  scaffolding + injected docs + project files), since most of it is not
  directly inspectable by us — only by the agent that receives it.
- The repetition rate across layers (e.g. does `docs/Bash.md`'s summary in
  `AGENTS.md` restate content already in the bundled doc; does global and
  project `CLAUDE.md` both explain the same convention).
- Which parts are harness/project-specific (must stay) vs. generic
  model/domain knowledge any current-generation model already has (candidate
  for removal) vs. content we could shrink but can't remove (built-in
  harness/CLI system-prompt scaffolding for cloud agents — not ours to edit,
  only to work around or override).

## Scope

- Each target agent, in a **clean project** and a **clean session** (no
  prior conversation, no warmed context), is asked to introspect and report
  on its own complete effective system prompt: harness-native scaffolding +
  every `CLAUDE.md`/`AGENTS.md`/bundled-doc/memory layer actually loaded for
  that project.
- Start with Claude Code (this harness); expand to other supported agent
  environments (e.g. Prime Agent / `~/.prime/agent`, others in
  `docs/README.md`'s agent list) only after the Claude Code pass validates
  the method. **Done for agy (Antigravity)** — see the agy study linked
  above; `harnez rate` enforcement is explicitly out of scope for agy since
  it isn't wired up there yet (122 only covers Claude Code), so that pass
  only checked repetition/prunability, not directive-following.
- Ask the agent to report:
  1. Approximate token/line count per layer (harness-native, global
     harnez-managed, project-managed, per-doc).
  2. Rate of repetition — same fact/convention stated more than once across
     layers (e.g. "conventional commits" appearing in both a global doc and
     a project doc).
  3. Content it would already know/apply without being told (generic
     software-engineering or model-native behavior) vs. content that is
     genuinely harness- or project-specific and must stay.
  4. For harness-native/cloud-agent scaffolding that cannot be edited
     (Claude Code's own system prompt, cloud-agent boilerplate): whether it
     can be *overridden* (a later, more specific instruction supersedes it)
     even though it can't be removed, and where such overrides currently
     happen vs. where they're missing.
- Distinguish this from 040: 040 is about agents *re-reading* files already
  in context; this ticket is about auditing what harnez itself *puts* into
  that context in the first place, and trimming at the source
  (`config.yaml`'s `agents_md.*.sections`, `docs/*`, doc-bundling in
  `apply`/`init`) rather than telling agents to read less.
- Out of scope for this ticket: implementing the trims. This ticket only
  covers running the audit and producing findings; follow-up tickets should
  file the specific cuts/consolidations found.

## Acceptance Criteria

- [x] A repeatable audit procedure exists (a prompt/script or a documented
      manual steps list) for asking an agent to self-report on its own
      system prompt in a clean session, without polluting a real working
      session's context. Done via a two-turn `claude -p` / `claude -c -p`
      pair run from an empty scratch directory — documented in the study's
      §1 Method.
- [ ] At least one completed audit report for Claude Code, covering: total
      size by layer, concrete repetition examples (quoted), a list of
      generic/model-native content flagged as removable, and a list of
      harness-native content flagged as non-removable-but-overridable.
      **Partial**: the clean-baseline pass (harness-native layer only, no
      project docs loaded) is done for both Claude Code and agy — see the
      two linked studies. The with-project-docs pass (this repo's full doc
      stack, as each harness would actually load it) is still open for
      both agents.
- [x] Findings are written up in `docs/studies/` (per `docs/Markdown.md`'s
      evergreen/ephemeral split) rather than left only in chat, since this
      is background/reference material for later trimming work. See
      [docs/studies/2026-08-31-claude-code-clean-session-system-prompt-audit.md](../docs/studies/2026-08-31-claude-code-clean-session-system-prompt-audit.md).
- [ ] Follow-up tickets are filed for each concrete trim/consolidation
      opportunity found (do not attempt the trims inside this ticket).
      Candidates so far: Claude Code study §5 (drop Write-tool's inline
      emoji restatement; consider merging "Tone and style" and "Text
      output"; require example+trigger for future global side-effecting
      directives); agy study §4-5 (collapse the anti-polling / "stop
      calling tools" pair into `<messaging>`, pull the copy embedded in the
      `schedule` tool's JSON schema out into that same prose section) —
      none yet filed as separate tickets.

## Notes

Keep the audit itself concise per project convention — the goal is a small
number of high-confidence findings (specific duplicated lines, specific
prunable sections), not an exhaustive line-by-line transcript of the system
prompt pasted into a doc.
