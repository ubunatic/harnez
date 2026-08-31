# 130 — Instruction-distribution audit follow-ups

**Status**: Closed — resolved in 96e7220
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[128-per-agent-full-system-prompt-self-audit-for-repetition]], [[122-agent-instruction-tool-feedback-protocol]], [[124-posttooluse-internal-tool-call-auto-capture]], [[040-agent-context-duplication-and-file-read-discipline]], [[075-concisemode-promote-doc-to-real-skill]], [[134-conversemode-to-skill-conversion]], [docs/studies/2026-08-31-instruction-distribution-audit-synthesis.md](../docs/studies/2026-08-31-instruction-distribution-audit-synthesis.md)

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
7. [x] **P3 — Add a `claude_skills_target` write path** (`apply.go`/
   `config.yaml`) so harnez-authored skills can become genuinely
   auto-triggered for Claude Code (description-matched, loaded on
   relevance) instead of reaching it only as manually-invoked slash
   commands. Prerequisite for #8.
8. [x] **P3, blocked on #7 — Convert `Website.md` (then Tool Feedback Protocol
   itself) from always-summarized doc + manual slash command into an
   auto-triggered Skill.** Removes their footprint from every project's
   default `AGENTS.md` injection for projects that never touch that
   concern. (`ConciseMode.md`'s conversion split out to [[134]] — it's a
   more involved case since `harnez mode` currently persists state via
   `AGENTS.md`-section rewrite, not just always-on prose.)

## Acceptance Criteria

- [x] Items 1-3 (P1) resolved.
- [x] Items 4-5 (P2) resolved.
- [x] Items 6-8 (P3) resolved. Item 7 added a `claude_skills_target` write
      path (`apply.go`/`config.yaml`/`status.go`), verified end-to-end
      against the real `~/.claude/skills/` dir on this machine. Item 8
      converted `Website.md` and the Tool Feedback Protocol to real Claude
      Code Agent Skills via that path.
- [x] `harnez diff`/`harnez status` still report clean after any
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

Items 7-8 resolved. Item 7: added `claude_skills_target` (default `~/.claude/skills`) to
`config.yaml` and `internal/claude/config.go`; `skillTargets()` in `apply.go` now includes it
alongside the Gemini/Codex/Prime roots, so every `skills:` entry gets a real
`~/.claude/skills/<name>/SKILL.md` (frontmatter `name:`/`description:`) in addition to the
existing `~/.claude/commands/<name>.md` slash command — the same `SKILL.md` shape harnez
already writes for Gemini/Codex/Prime, confirmed against Claude Code's own documented personal
Skills mechanism (description-matched, auto-loaded, no code path needed on Claude's side).
`DiffAll`/`CleanAll` previously had no skill-drift/removal logic at all for *any* skill target
(a latent gap, not something introduced here) — added it so `harnez diff` detects skill drift
and `harnez clean` removes skill files/dirs for all four targets, verified live: `harnez apply`
wrote all 9 skills to `~/.claude/skills/`, `harnez diff` reported no drift, `harnez clean`
removed them all (down to an empty `~/.claude/skills/`), and a follow-up `harnez apply`
restored clean state. New test `internal/claude/claudeskills_test.go` covers the full
apply/diff/status/clean round trip in isolation; `internal/claude/integration_test.go` and
`prime_test.go` extended to assert the new target too. Because adding `claude_skills_target`
applies uniformly to the whole `skills:` list, this incidentally makes every existing skill
entry (`evergreen`, `story`, `domain-modeling`, `sprint`, `fresh-sprint`, `mode`, `website`,
`harnez-sync`) reach Claude Code as a real Skill, not just the two item 8 targets — this is the
general infrastructure item 7 asked for, not scope creep into 134 (ConciseMode.md/`harnez mode`
itself was not touched).

Item 8a (`Website.md`): flipped its `languages:` entry `default:` from `true` to `false` in
`config.yaml` — it's now delivered to Claude Code as the real `website` Agent Skill (already
present in the `skills:` list, description already read as a trigger condition) instead of
being baked into every new project's baseline `AGENTS.md` "Language Conventions" section via
`init`. Still copyable on request via `init --docs website`, and still installed as a raw doc
file to `~/.claude/docs/Website.md`/`~/.prime/agent/docs/Website.md` via the repo's own
top-level `docs:` list (a separate, always-on global-doc-copy mechanism, not per-project
injection). The `/website` slash command is left in place — harmless overlap; a user typing
`/website` still works, and the Skill covers the auto-trigger case the command couldn't. Fixed
five now-stale `default:true` expectations in `docs_test.go`'s `TestAutoDetectDocs_PolyglotMatrix`.

Item 8b (Tool Feedback Protocol): added a new `tool-feedback-protocol` entry to `config.yaml`'s
`skills:` list (inline `content:`, not a new `commands:` entry — no slash command added) whose
description reads as an explicit trigger ("use immediately after completing any internal tool
call... in a harnez-tracked project") and whose body reuses the worked `harnez rate` example
plus adds the "why now" framing (score is a judgment call only the agent can make in the
moment; batching from memory at session end is noise) that always-on prose can't carry.
Decision: kept the existing `agents_md.global.sections` "Tool Feedback Protocol" entry as-is —
it's the only delivery mechanism for non-Claude-Code harnesses (Prime Agent gets it via
`~/.prime/agent/AGENTS.md`; Skills are a Claude-Code-only mechanism), so removing it would
silently drop the guidance for those harnesses. Claude Code now gets both: the background
reminder from the global section and the contextually-triggered Skill.

Full re-verification after both items: `go build ./...` and `go test ./...` clean (one
unrelated pre-existing failure in `scripts/canary-remote-load-stream` predates this ticket and
is untouched by it); `make install`; `harnez apply`/`harnez diff`/`harnez status` clean on the
real machine; `harnez clean` removes all `~/.claude/skills/*` (verified), followed by a
restoring `harnez apply`. Ticket 130 is now fully resolved (all 8 items done, with 134 split
out and separately tracked) — moving Status to Closed.

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
