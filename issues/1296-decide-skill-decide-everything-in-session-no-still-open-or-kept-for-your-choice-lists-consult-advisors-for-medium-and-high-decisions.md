# 1296 — decide skill: decide everything in-session, no Still Open or Kept-for-your-choice lists, consult advisors for medium and high decisions

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1085 (decide skill added), 1100 (advisor roles), docs/commands/decide.md

---

## 0. Goal
`/goal` Update the `/decide` skill (`docs/commands/decide.md`, installed to `~/.claude/skills/decide`) so the orchestrating agent decides as much as it can in-session: no "Still Open" or "Kept for your choice" items, a consulted advisor for medium and high decisions, and only very critical choices left to the user. Done when the source doc and its installed copy match, the skill's tests pass, and `make test-q1` is green. Or stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
The owner's current `/decide` output leaves items for the user: "Still Open" and "Kept for your choice" lists. The owner wants those gone. The owner reserves the user's decision for very critical choices only.

Current `docs/commands/decide.md` already says: simple questions answered by the agent, a cheap advisor for a second opinion, a smart advisor for major decisions, and a stop only for five critical categories. It does not say that open items must not be parked, and it does not define medium or high decisions.

## 2. Technical Specification / Findings
- Source: `docs/commands/decide.md`, registered in `config.yaml` under skills (`name: decide`, `file: docs/commands/decide.md`). Installed copy: `~/.claude/skills/decide/SKILL.md`, refreshed by `harnez apply`.
- Advisor call in the current rules: `HTO=0 harnez agent start --role advisor --name <name> --model cheap|smart -p "<question>"`; model aliases live in `spec/agent.yaml`.
- Session End section lists decisions and stops helpers. It has no rule against deferred items.

## 3. Implementation & Verification Plan
Requested changes (owner's words, condensed):
1. Decide as much as possible. Every question that arises gets a decision in this session, with its reason in one line.
2. No "Still Open" and no "Kept for your choice" lists. Carry a decision forward or record it in the ticket, not in a list for the user.
3. Only very critical choices wait for the user. Keep the five existing categories (destructive or irreversible data or history action, secret or security exposure, spending money, publishing outside the normal remotes, action in another repository).
4. Medium and high decisions consult an advisor first. Use a fresh one-shot advisor (`harnez agent start --role advisor --model cheap` for medium, `--model smart` for high) or another live agent session if one is available (a peer from `mcp__taia__ListAgents`, or a `harnez` / `claude` / `agy` harness agent).
5. Keep the existing push, circuit-breaker and session-end rules.

Verification:
- Update `docs/commands/decide.md` and `config.yaml` if the description changes; run `make install`, `harnez init -d .` (or `harnez apply`) so the installed copy matches, and commit both together.
- Add or update the skill round-trip test (the test that asserts the configured description and installed workflow, per 880) so the new rules are checked.
- `HTO=0 make test-q1` green.

## Open choices (decided here)
- "Medium" and "high" follow the existing model aliases: medium uses a cheap advisor, high uses a smart advisor. A peer session is an extra opinion, not a replacement.
