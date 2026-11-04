# 1115 — harnez apply keeps docup skill dirs with extra files in all five agent roots

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Investigation
**Related**: 301 (added /docup), 821 (decommissioned-skill cleanup)

---

/goal Decide whether harnez may take over the installed `docup` skill dirs, make `harnez apply`
update them cleanly (or document why it must not), and verify a second apply prints no `kept`
line for docup; or stop and report when blocked on a user decision about local files.

## 1. Problem & Motivation
`harnez apply` (v0.5.2, 2026-11-03) prints `kept .../skills/docup: directory contains files beyond
SKILL.md` for all five roots (`~/.claude`, `~/.codex`, `~/.gemini`, `~/.pi/agent`, `~/.prime/agent`).
So the bundled docup skill is never updated there, and the user is told to `touch .harnez-managed`
by hand in five places.

## 2. Findings (unverified)
- Each dir has `SKILL.md` plus a `references/` subdir. The Codex copy was edited on 2026-10-26
  (larger SKILL.md, newer references); the Claude copy dates from 2026-09-24.
- Open questions: does the bundled docup skill ship `references/` itself (then apply's
  "extra files" check misjudges its own resource files, cf. 727)? Are the Codex edits local work
  worth keeping or upstreaming?

## 3. Plan
1. Diff each installed dir against the embedded docup source.
2. If the extras are harnez's own resources, fix apply's ownership check; otherwise ask the user
   before marking the dirs managed.
3. Verify: `harnez apply` twice, no docup `kept` line, no user content lost.

## Audit 2026-11-04
Jules audit (session 572906474584759079, against 70da579e, `reports/jules/1115.md`): still valid, 3 of 3
references hold. Answers the first open question: yes, the bundled docup skill ships
`references/DocupTesting.md` and `references/DocupArch.md` itself (`config.yaml:442` `resources:`),
while `skillAdoptionStatus` accepts only a lone `SKILL.md` (`internal/claude/skill_ownership.go:16`).
So step 2's fix applies: let the check accept the skill's own listed resource targets. The Codex
copy's local edits (second question) still need the diff in step 1 before anything is overwritten.
