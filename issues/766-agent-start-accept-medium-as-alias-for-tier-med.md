# 766 — agent start: accept 'medium' as alias for tier 'med'

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 497 (agent tier/alias mapping), internal/subagent/driver.go, internal/subagent/{claude,codex,agy}.go

---

## Problem & Motivation

`harnez agent start --model claude:haiku:medium` fails with "unknown model tier ... known specs:
... claude:haiku:med ...". "medium" is the natural word (Claude Code and Codex use it for
effort/thinking), so agents and users type it and lose a start (seen 2026-10-10, sprint for 765).

## Goal

/goal `medium` is accepted wherever a tier/effort is parsed (`--model provider:model:medium`,
`model:medium`, and any effort/thinking flag) and behaves exactly like `med`; `med` stays the
canonical form in output and spec; unit tests cover `medium`, `med`, and a near-miss (`mid`,
`medum`) that is still rejected; or stop and report when blocked on a user decision (e.g. whether
`lo`/`hi` aliases are wanted too) or a denied permission.

## Notes

- Check live code first. Tier strings are compared in `driver.go:181-187` and per provider
  (`claude.go:69`, `codex.go:112`, `agy.go:98`); normalize once at parse time, not per provider.
- Keep the error message listing canonical specs.
