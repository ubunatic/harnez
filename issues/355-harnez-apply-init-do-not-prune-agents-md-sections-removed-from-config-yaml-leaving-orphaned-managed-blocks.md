# 355 — harnez apply/init do not prune agents_md sections removed from config.yaml, leaving orphaned managed blocks

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: CLI / Templates
**Related**: [Issue 354](354-collapse-duplicated-instruction-blocks-across-claude-md-agents-md-templates-into-single-source-pointer-pattern.md) (found this while implementing)

---

## 1. Problem & Motivation

While implementing issue 354, I removed the `Issue Tracker Discovery` entry from
`config.yaml`'s `agents_md.global.sections` and re-ran `harnez apply` on an existing
install. The generated `~/.claude/CLAUDE.md` still contained the full
`<!-- harnez:begin Issue Tracker Discovery -->...<!-- harnez:end Issue Tracker Discovery -->`
block from the previous apply. `apply` only adds and updates markers it still finds in
the current config; it never removes a marker block whose name is no longer present in
`config.yaml`. The same applies to `agents_md.local.sections` consumed by `init`.

This means any config-driven section rename or removal leaves a permanently orphaned,
stale block in every previously-applied/initialized install unless someone notices and
hand-deletes it. For issue 354 this had to be caught by grep and fixed by hand.

## 2. Proposed Fix

`ApplyAll`/`init`'s section-reconciliation pass should track the set of managed marker
names it wrote last time (or scan the file for all `harnez:begin <name>` markers) and
remove any marker block whose name is not in the current config's section list, in
addition to the existing add/update behavior.

## 3. Acceptance Criteria

- [ ] A section removed from `config.yaml` is deleted (not left stale) from a previously
      applied `~/.claude/CLAUDE.md` on the next `harnez apply`.
- [ ] Same behavior verified for a previously init'd project's `AGENTS.md` local managed
      block on the next `harnez init`.
- [ ] Regression test: apply with section A+B, remove B from config, re-apply, assert B's
      marker block is gone and A's is untouched.
- [ ] `scripts/smoke-test.sh` still passes.
