# 893 — Haiku 5.5 as searcher (advisor role) and simple-task developer in spec and sprint skills

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Docs
**Related**: 755 (add Haiku 5.5), 510 (web-only researcher role), docs/commands/lean-sprint.md, spec/agent.yaml

---

/goal Haiku 5.5 is a documented choice for read-only searchers (code and web search, `--role advisor`) and
for developers on simple tasks, consistently in `spec/agent.yaml` and in the sprint skills and practice docs;
verified by `harnez agent start --role advisor --model claude:haiku:med` working without a warning and by the
rendered skill text; or stop and report when a role-policy decision is needed.

## Problem (owner, 2026-10-24)

The owner uses Haiku 5.5 (much better than earlier Haikus) as searcher in sprints and web searches and as
developer for simple tasks. Another session hesitated: harnez lists Haiku only for
`developer, reviewer, mechanical` (`spec/agent.yaml:108`), not `advisor`. Meanwhile:

- `--role advisor --model claude:haiku:med` already works (sprint 4 searcher, 2026-10-24), so the spec `roles`
  list is advisory, not enforced. It still steers agents away from a valid choice.
- Lean-sprint names searchers `luna:low` / `luna-search` / `argon:low` (`docs/commands/lean-sprint.md:45`) and
  developers `luna:high` / `sol:low` (`:57`); `docs/practices/AgenticLoop.md:292-316` the same. Haiku is missing.

## Wanted

- Add `advisor` to Haiku's roles in `spec/agent.yaml` (check what `roles` drives: help tables, warnings).
- Name Haiku as a searcher (code and web) and as a simple-task developer in lean-sprint, lean-sprint-v1 and the
  AgenticLoop sources (full and lite), then sync root copies.
- State in one line that `roles` in spec is a recommendation, if it is not enforced.
