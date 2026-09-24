# 546 — harnez init: toggle the harnez read -I recommendation per project

**Status**: Open
**Priority**: P2
**Severity**: Medium
**Category**: CLI / Managed Conventions
**Related**: [[543-harnez-process-grew-to-15-16-gb-rss-and-was-oom-killed-twice-agent-resume-under-harnez-exec]], [[544-handoff-2026-09-24-open-threads-from-the-harnez-bf-session-clean-agy-routing-exec-cpu-read-i-oom-cxxxe-rule]]

---

## Background

The `harnez read -I` recommendation was paused by editing fixed text (managed AGENTS.md block,
AgenticLoop, lean-sprint/roadmap commands, hook deny message, session tip) after the 543 OOM.
Lifting or reapplying the pause means editing all of these again. The user wants it as a feature
that `harnez init` toggles instead.

## /goal

`harnez init` has an on/off setting for the `read -I` recommendation (flag and/or project config, off
by default until 543 is fixed). The managed block, doc copies, hook deny message and session tip all
render from that one setting; no text says "paused" by hand. Tests cover both states.

## Notes

- Code touchpoints seen: `cmd/harnez/hook.go`, `cmd/harnez/read.go`, `internal/sessionstate`, templates in
  `docs/templates/AGENTS.md` and `config.yaml`. Global copies (`apply`) need a matching default.
- Hold the multi-repo `harnez init` rollout (544 thread 3) until this lands, so repos get one rewrite.

## Decision (2026-09-24): stays off until data shows it works

543 is fixed (bounded exec capture, bounded `read` range), but `read -I` stays **off** by default everywhere until
data shows it works: memory stays bounded in real agent use and agents actually benefit from the cards (usage and
feedback in `harnez stats` / `harnez rate`). The toggle still gets built; switching the default on is a separate
decision that needs that data.
