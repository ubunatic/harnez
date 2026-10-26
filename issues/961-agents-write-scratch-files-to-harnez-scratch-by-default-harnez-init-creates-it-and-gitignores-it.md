# 961 — Agents write scratch files to ./.harnez/scratch by default; harnez init creates it and gitignores it

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 613 (init scaffolds .gitignore)

---

## 1. Problem & Motivation

Agents put intermediate files (review lists, spot-check tables, logs) in a per-session host
scratchpad under `/tmp/...`. The owner cannot find these files easily, and they vanish with the
session. Owner request (tokburn, 2026-10-26): use a repo-local `./.harnez/scratch/`, ignored by git.
tokburn set this up by hand for its T1 spot-check list.

/goal Agents in harnez-managed repos write scratch files to `./.harnez/scratch/` by default, and
`harnez init` creates that dir and adds a `/.harnez/scratch/` rule to `.gitignore` (idempotent);
or stop and report when blocked on an owner decision or denied permission. Circuit breaker: on a
loop or no progress, clear the goal and write a failure report here.

## 2. Technical Specification / Findings

- `harnez init` (and re-runs on existing repos) ensures the dir and the ignore rule, without
  duplicating the rule. See 613 for the existing gitignore scaffolding work.
- Agent rules (`.harnez/rules/`) tell agents to prefer `./.harnez/scratch/` over host temp dirs
  for files the owner may want to open; name the full path when pointing the owner at a file.
- Open: whether host-provided scratchpads (e.g. Claude Code's) should still be used for
  purely internal temp files.

## 3. Implementation & Verification Plan

Re-check live code first. Test: init on a fresh and an existing repo creates the dir, adds the
rule once, and `git status` stays clean after writing a scratch file.
