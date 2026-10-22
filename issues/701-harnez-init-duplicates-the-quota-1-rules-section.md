# 701 — harnez init duplicates the Quota-1 rules section

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation

Running `harnez init --variant lite --quota-1` in the Tilix checkout generated `.harnez/rules/Quota.md` with two `## Quota-1 Guardrails` sections. The first section contains the single-test boundary, code-modification requirement, clean-tree requirement, enforced test target, and bypass restriction. The second repeats those rules and adds the report-after-test requirement. This duplication makes the generated guidance noisy and spends extra agent context on repeated instructions.

**Goal**: `/goal` Ensure `harnez init --quota-1` generates one coherent Quota-1 rules section without duplicated rules, and verify the generated output with a regression check; if the duplication cannot be reproduced, stop and report the evidence.

## 2. Technical Specification / Findings

The duplicate appeared in one init run on 2026-10-04. Each section begins with the same heading and repeats the same five rules; the second also includes `Report Untested Edits`. Expected output is one Quota-1 heading with each applicable rule stated once.

## 3. Implementation & Verification Plan

Trace the generated rule to its source and remove the duplicate contribution. Add or update an init regression test that checks the generated Quota-1 section, then verify with the focused test and a fresh init output.

## 4. Second reproduction (cati, 2026-10-07)
`harnez init --quota-1 --variant lite` with harnez 0.1.24 in `~/projects/cati` produced
`.harnez/rules/Quota.md` with an untitled-section copy first (`# Quota-1 Guardrails`, six
rules incl. **Media & Demo Verification Gate**) and then `## Quota-1 Guardrails` with five
rules (no media gate, no `Report Untested Edits`). So the two copies differ in a different
way than in the Tilix run: the generator seems to merge a file-level template and an
AGENTS.md-style section rather than duplicating one block.

## Live check (2026-10-22, lean sprint): still current

`migrateManagedBlocks` appends the migrated Quota-1 block to `.harnez/rules/Quota.md`
(`internal/claude/init.go:665`) while `writeRules` keeps the existing file (`init.go:505`); no dedupe.
Way forward: reconcile the migrated block with the generated `Quota.md` content (one managed section,
local text kept). Test in `internal/claude/quota1_init_test.go`: generated Quota.md plus a managed Quota-1
block in AGENTS.md → init → exactly one heading; second init idempotent.
