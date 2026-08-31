# 130 — Instruction-distribution audit follow-ups

**Status**: Open
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[128-per-agent-full-system-prompt-self-audit-for-repetition]], [[122-agent-instruction-tool-feedback-protocol]], [[124-posttooluse-internal-tool-call-auto-capture]], [[040-agent-context-duplication-and-file-read-discipline]], [[075-concisemode-promote-doc-to-real-skill]], [docs/studies/2026-08-31-instruction-distribution-audit-synthesis.md](../docs/studies/2026-08-31-instruction-distribution-audit-synthesis.md)

## Problem

A Product Discovery + Technical Advisor subagent pair audited harnez's own
instruction-distribution system (`apply`/`init`, `config.yaml`'s
`agents_md.global`/project sections, docs bundling) — as a follow-up to
128's per-agent prompt audits, this time auditing what harnez itself puts
into that context rather than what each harness natively contributes. Full
findings: [docs/studies/2026-08-31-instruction-distribution-audit-synthesis.md](../docs/studies/2026-08-31-instruction-distribution-audit-synthesis.md).
This ticket scopes the concrete fixes; the study itself makes no changes.

## Scope

Eight findings, prioritized (see study §7 for full detail):

1. [x] **P1 — Rescope Tool Feedback Protocol out of `agents_md.global`.**
   `config.yaml:242-249`'s invocation template presumes the target project
   uses harnez's ticket convention (`issues/NNN-*.md`), which is itself only
   project-scoped and opt-in — injecting it unconditionally into every
   global target contradicts the adjacent "Minimal Global Docs" section in
   the same file. Move to `agents_md.local`/opt-in-by-`init`, or add an
   explicit graceful-degradation statement for ticket-tracker-less projects.
2. [x] **P1 — Improve Tool Feedback Protocol's prose.** Add a worked example
   invocation (a real `harnez rate Read 5 "..." harnez/117-...` line) and
   an explicit trigger/consequence, mirroring why the git-commit-trailer
   convention is reliably followed (exercised as a literal template every
   commit) vs. this directive's current unscoped-standing-policy phrasing.
   Confirmed independently that a `PostToolUse` hook (124) **cannot**
   substitute — 124's own scope excludes score, which only the agent can
   judge.
3. [x] **P1 — Fix `docs/README.md`'s false "Copyable doc" claim for
   `DeploymentTransparency.md`.** No `config.yaml` `languages:` entry backs
   it; either add the missing source/target/local entry or drop the
   copyable claim from the index row.
4. [x] **P2 — Collapse the 4-way "Context Discipline" restatement**
   (`config.yaml:232`, `docs/README.md:7`, `AgenticLoop.md:45` Invariant #5,
   this repo's own `AGENTS.md:131-134`) into one canonical statement (likely
   `AgenticLoop.md`'s Invariant #5) with the other three as `@docs/...`
   pointers instead of independently-worded restatements. Resolved: kept
   `docs/practices/AgenticLoop.md` Invariant 6 (Context Discipline &
   Range-Bounded Ingestion) as canonical; `config.yaml`'s Instructions
   Hierarchy bullet, `docs/README.md`'s tip box, and this repo's own
   `AGENTS.md` section now all point to it instead of restating it.
5. [x] **P2 — Replace harnez's own root `CLAUDE.md`'s full P0-P3 schema
   transcription with `@docs/IssueTracking.md`.** The project already has
   the doc installed locally via its own `init --docs issue-tracking`; the
   full hand-copy is duplication with no copy-mechanism excuse and violates
   the project's own documented summary-vs-full-doc discipline. Resolved:
   `AGENTS.md`/`CLAUDE.md` (a symlink to it) now carries a short pointer
   instead of the full schema/metadata-template transcription.
6. [x] **P3 — Merge "Minimal Global Docs" into "Instructions Hierarchy"** as
   one additional bullet rather than a separate marked section (removes one
   marker pair, no content loss). Resolved in `config.yaml`; verified the
   stale `Minimal Global Docs` marker block no longer exists in either real
   target file (`~/.claude/CLAUDE.md`, `~/.prime/agent/AGENTS.md`) after
   apply — `ApplyAll`/`CleanAll` do not auto-remove sections dropped from
   config, so the orphaned block had to be stripped manually once.
7. **P3 — Add a `claude_skills_target` write path** (`apply.go`/
   `config.yaml`) so harnez-authored skills can become genuinely
   auto-triggered for Claude Code (description-matched, loaded on
   relevance) instead of reaching it only as manually-invoked slash
   commands. Prerequisite for #8.
8. **P3, blocked on #7 — Convert `Website.md` (then `ConciseMode.md`, then
   Tool Feedback Protocol itself) from always-summarized doc + manual slash
   command into an auto-triggered Skill.** Removes their footprint from
   every project's default `AGENTS.md` injection for projects that never
   touch that concern.

## Acceptance Criteria

- [x] Items 1-3 (P1) resolved.
- [x] Items 4-5 (P2) resolved.
- [ ] Items 6-8 (P3) scheduled or explicitly deferred with a reason (item 8
      is blocked on item 7 by design). Item 6 done; items 7-8 remain open.
- [ ] `harnez diff`/`harnez status` still report clean after any
      `config.yaml` section changes (idempotency preserved).

## Progress

Items 1-3 (P1) resolved: `config.yaml`'s Tool Feedback Protocol section now carries an explicit
harnez-tracker-only scoping clause plus a worked `harnez rate` example, and a `deployment-transparency`
`languages:` entry was added so `docs/README.md`'s "Copyable doc" claim for
`DeploymentTransparency.md` is now backed by a real install mechanism. `harnez apply`/`harnez diff`
confirmed idempotent after the change.

Items 4-6 resolved: `docs/practices/AgenticLoop.md` Invariant 6 kept as the canonical
Context Discipline statement, with `config.yaml`, `docs/README.md`, and this repo's own
`AGENTS.md` collapsed to pointers at it; this repo's root `CLAUDE.md` (symlinked to
`AGENTS.md`) no longer hand-transcribes the P0-P3 Issue Priority Schema, pointing to
`@docs/IssueTracking.md` instead; and `config.yaml`'s "Minimal Global Docs" section was
folded into "Instructions Hierarchy" as one bullet, with the orphaned marker block
manually stripped from both real target files (`ApplyAll`/`CleanAll` don't auto-remove
sections dropped from config). `harnez apply`/`harnez diff`/`harnez status` confirmed
clean afterward. Items 7-8 remain open (P3, item 8 blocked on item 7); ticket stays Open
overall.

## Notes

**Self-improvement skill idea (not scoped here)**: the user raised
packaging this whole audit method (two clean-session per-agent
introspection passes + a Product-Discovery/Technical-Advisor pair auditing
the distribution system + synthesis) as a repeatable skill, so the
assessment can be re-run later as the project evolves without this much
orchestration overhead. Deliberately not built in this ticket — better
built after the fixes above have landed and the method has been re-run at
least once more, so the skill reflects real re-run friction rather than a
single pass. File a separate ticket for it once that second run happens.
