# 351 — Add CLAUDE.md pointer: check config.yaml/Spec.md before hardcoding named business-value lists in Go

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Docs

---

## Summary

Add a short line to this project's `CLAUDE.md` (or `AGENTS.local.md`):

> Before adding any Go slice/map of named business values (tool lists, presets,
> categories), check `config.yaml`/`docs/Spec.md` first.

## Background

Issue 316's debloat feature initially hardcoded its preset deny-lists
(`minimal`/`aggressive` tool-name membership) as Go package vars in
`internal/claude/debloat.go` — a direct violation of `docs/Spec.md`'s core rule
("config.yaml/spec files are the single source of truth; application code must not
duplicate or shadow spec values"). The violation compiled cleanly and passed every
test; it was only caught because the user reviewed the diff and asked directly. Fixed
same-session by moving the lists into a new `debloat:` section in `config.yaml`.

`docs/Spec.md` already documents the rule and now has a dated case-in-point for this
exact incident (see its "Anti-Patterns to Avoid" section, 2026-09-15 addition). But
`docs/Spec.md` isn't part of every session's automatically-loaded context the way
`CLAUDE.md` is — an agent has to already know to consult it. A one-line pointer in
`CLAUDE.md` would surface the check *before* the violation happens, not just document
it afterward for the next reader who happens to open `Spec.md`.

## Task

Add the note above to this project's `CLAUDE.md` (harnez-managed conventions block, or
wherever similar cross-references to `docs/Spec.md` already live — check how other
`@docs/` references are wired into the managed CLAUDE.md sections in `config.yaml`'s
`agents_md` config, since this project generates its own CLAUDE.md from that config
rather than hand-editing it directly).

## Definition of done

- The pointer line is live in the actual installed `~/.claude/CLAUDE.md` /
  `~/.prime/agent/AGENTS.md` (via `harnez apply`), not just in this repo's source
  template — confirm with `harnez status` or a direct read after applying.
- Doesn't duplicate `docs/Spec.md`'s full content — just points to it, matching how
  other `@docs/` references in `CLAUDE.md` behave.

## Related

- Issue 316 — `harnez apply --debloat` (source of the incident this pointer prevents).
- `docs/Spec.md` — the rule itself and its 2026-09-15 case-in-point.
