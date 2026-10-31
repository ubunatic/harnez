# 1069 — Add trial skill for concise 10-5 capability audits (10 things we can do, 5 things we cannot)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [retro](file:///home/uwe/.gemini/skills/retro/SKILL.md), [study](file:///home/uwe/.gemini/skills/study/SKILL.md), [focus](file:///home/uwe/.gemini/skills/focus/SKILL.md)

---

## 1. Problem & Motivation

During long development sessions, sprints, or feature milestones, users frequently request a high-leverage checkpoint summarizing what was actually unlocked versus remaining boundaries:
- **Ten Things We Can Do Now**: Recent capabilities, verified bug fixes, new CLI commands, and proven workflows.
- **Five Things We Cannot Do Yet**: Remaining gaps, unverified canary assumptions, open tickets, or blocking bugs.

Today, this capability check (the "10-5 trial") is conducted ad-hoc in conversation. Packaging this into an invocable `/trial` (or `/audit-10-5`) skill standardizes how agents inspect live git commits, closed tickets, and verified features to produce grounded, evidence-backed capability summaries without fluff or hallucinated claims.

## 2. Technical Specification

1. **Skill Definition (`skills/trial/SKILL.md`)**:
   - Invocable via `/trial` or `/trial <scope>` (e.g. `/trial today`, `/trial sprint`, `/trial <ticket>`).
   - Grounding rules:
     - Must inspect recent git commit history (`git log --since=...` or `git log -n ...`) and recently closed tickets (`issues/CLOSED.md`).
     - Extracts exactly 10 concrete, delivered capabilities with verifiable evidence (linking commit hashes or tickets).
     - Extracts exactly 5 clear boundaries, open blockers, or pending follow-ups (linking open tickets or unmerged PRs).
   - Enforces Concise Mode: verbatim commands, no opening/closing pleasantries, sharp contrast between what works and what does not.

2. **Integration into Harnez Skills**:
   - Register `/trial` in the skills registry (`internal/skillreg` or installed skill catalog).
   - Document the `/trial` skill in `docs/practices/AgenticLoop.md` alongside `/evergreen`, `/retro`, and `/study`.

## 3. Goal & Acceptance Plan

/goal Author and register the /trial skill to produce grounded 10-5 capability audits from live git history and issue tracker state, verified with automated skill validation tests, or stop and report when blocked on user input or denied permission.

1. **Author `SKILL.md`**: Create the skill instruction file with clear frontmatter, workflow guidelines, and the 10-can / 5-cannot output template.
2. **Registration & Docs**: Add `/trial` to skill definitions and relevant documentation.
3. **Verification**: Run a live trial invocation across recent commits to verify that it outputs 10 accurate capabilities and 5 grounded limitations with exact file/commit links.

---

## 4. Implementation Notes

- Skill source file: `docs/commands/trial.md` (registered in `config.yaml` as `trial`, `user-invocable-only` under the skill presets; covered by `internal/claude/claudeskills_test.go`; documented in `docs/practices/AgenticLoop.md` and its lite copy).

## 5. Live Verification: `/trial sprint` on `sprint/1069-trial-skill`

Evidence: `git log main..HEAD --oneline` returned one commit (`a3028bf0`). Shortfall rule applied; counts are stated, not padded.

### Things We Can Do Now (1 verified)

1. **Invoke a 10-5 audit with scoped evidence rules** — `/trial`, `/trial today`, `/trial sprint`, `/trial <ticket>` skill in `docs/commands/trial.md`, registered by the `trial` entry in `config.yaml` and checked by `internal/claude/claudeskills_test.go` ([a3028bf0](issue 1069)).

### Things We Cannot Do Yet (3 verified)

1. **Import forge issues** — `harnez issues import` does not exist yet; open [1064](1064-harnez-issues-import-command-to-fetch-remote-forge-issues-into-local-tracker.md).
2. **Export local issues to a forge** — `harnez issues export` does not exist yet; open [1067](1067-add-harnez-issues-export-command-to-export-local-issues-to-remote-forge-tracker-via-agit-or-fj.md).
3. **Pin docs, skills and tools per role** — role profiles not built; open [1068](1068-adopt-plain-mode-for-general-tasks-and-create-role-profiles-to-pin-docs-skills-and-tools.md).
