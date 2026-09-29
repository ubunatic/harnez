# 636 — Skill to assess and onboard external skills from a URL

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [[631-external-skills-canary-then-user-registry-b-then-vendored-skills-a]], [[632-re-home-mattpocock-derived-skills-as-external-skills-issue-631-registry]]

---

/goal Ship a harnez-owned skill that, given a repo URL, assesses its skills and recommends which
to onboard, then onboards the user's choice via `harnez skill`; or stop and report when blocked
on a user decision or denied permission.

## 1. Problem
When the user hands over a skills repo URL, the agent has no defined workflow to judge it.
Onboarding the Pocock skills (632) was done ad hoc.

## 2. Workflow the skill defines
1. **Assess** (read-only): run `harnez skill explore <url>`; read each SKILL.md and its scripts.
   Judge per skill:
   - scope: what it does, what it needs (tools, keys, network, scripts);
   - quality: clarity, maintenance (recent commits), license, risky scripts;
   - fit for harnez: overlap or clash with own skills (`config.yaml` `skills:`) and bundled ones;
   - fit for the current repo: does its workflow match this repo's rules and tools
     (e.g. tickets go to `issues/`, not an external tracker).
2. **Present**: a short table per skill with a recommendation (onboard / skip / why), and the
   onboarding mode: registry install (this machine) or bundled (ships with harnez; only in the
   harnez repo). Ask the user to choose.
3. **Onboard** the chosen skills: `harnez skill install <url> <skill>` (explicit-only by default,
   `--as` on name clash), or for bundled add to `bundled_skills:` and run `harnez skill vendor`
   plus `harnez apply`. Verify the agents list them.

## 3. Open points
- Skill name (e.g. `skill-scout`), and whether it is explicit-only.
- Whether `explore` needs more output (license, last commit date) to support the assessment.

## 4. Note (2026-09-29): harnez decide
The fit checks (scope, quality, fit for harnez and the repo) could later run as `harnez decide`
questions (issue 633). The first version uses the normal agent and does not wait for 633.
