# 727 — Skills on agents without a native skill copy (Prime, Gemini, agy?) cannot load resource files

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `/docup` (ships `references/DocupTesting.md`, `references/DocupArch.md` as resources), issues/726 (found there), `docs/ExternalSkills.md`, `config.yaml` `skills:` `resources:`

---

## 1. Problem & Motivation

Own skills can ship resource files next to `SKILL.md` (`config.yaml` `skills:` entries with
`resources:`; `/docup` does this today). Agents with a native skill copy (Claude,
Codex) read the resource from the installed skill dir. Agents without one load skills through
`harnez skill show <name>`, which prints only `SKILL.md`, so the resource is unreachable.

Found by the issue 726 developer during planning. 726 no longer depends on it: per the user,
`/harnez-handoff` renders its spec into SKILL.md instead of shipping it as a resource.

## 2. Technical Specification / Findings

Uncertainties to verify first (recorded, not confirmed):

- Which agents are affected. `docs/ExternalSkills.md` lists Gemini and Prime as having no native
  copy; the 726 developer named Prime and agy.
- Whether `harnez skill show` serves own skills (`skills:` from `docs/commands/`) at all, or only
  external and bundled skills; its help says "an installed external skill's SKILL.md".

## 3. Implementation & Verification Plan

/goal Agents without a native skill copy can load an own skill and its resource files, verified
with `/docup` on each affected agent. Stop and report when blocked on a user decision or denied
permission.

Before starting, check the live code and recent commits; 726 may have changed the skill and
resource layout.
