# 1100 — Split the advisor role into scout and consult

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 692, 893; codeberg.org/harnez/horg#7, horg#8

---

/goal The `advisor` role is split into two named roles: `scout` (cheap read-only search) and `consult`
(judgment, with a `cross` trait for a one-shot second opinion from another vendor). Done when spec, flags,
model picks, help, docs and bundled skills use the new names, `advisor` still works as a deprecated alias,
and `make test-q1` is green. Stop and report when blocked on a user decision or denied permission.
Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## 1. Problem & Motivation

`advisor` covers two different jobs: cheap searchers (`--model cheap`) and judgment calls or second opinions
(`--model smart`, `cross`). Skills and briefs must add the trait to say which one is meant, and the name says
neither. The owner proposed the split in horg#7 (2026-11-01). `horg pick` already accepts `scout` and `consult`
as its own aliases (horg `82786a5`), so a harnez rename lets horg#8 hand roles straight to `harnez agent start`.

## 2. Technical Specification / Findings

- `spec/agent.yaml:350` defines `advisor` with aliases `[search, arch, explorer, scout, auditor, planner]` and
  models `cheap` and `smart`. `scout` is an alias today, so it maps to the same role and defaults to `cheap`.
- Owner picks (`~/.config/harnez/models.yaml`) use `roles.advisor.models.{cheap,smart,cross}`; user files
  must keep working.
- Open: whether `consult` is a new role with its own rules and default trait (`smart`) or an alias that only
  changes the default trait. Decide before coding.

## 3. Implementation & Verification Plan

Re-check live code first. Rename in spec, role resolution, model-picks schema (accept old keys), help goldens,
`docs/` and bundled skills (`/decide`, `/epic`, lean-sprint). Verify: `harnez agent start --role scout` and
`--role consult` resolve to the expected chains, `--role advisor` still works with a deprecation note, q1 green.
