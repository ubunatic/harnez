# 808 — Managed docs/practices/Testing.md: short/pkg/full/container/CI test tiers

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Documentation
**Related**: 276, 057

---

## 1. Problem & Motivation
Repos lack a shared vocabulary for test tiers; agents run the full suite when a package test would do, and container/CI parity is undocumented.

## 2. Technical Specification / Findings
Add `docs/practices/Testing.md` (copyable, distributed by `harnez init`): tiers `make test-short`, `make test-pkg PKG=`, `make test`, `make container-test`, CI; `make lint`; no wall-clock sleeps; raceFactor budgets; TTY/PTY tests in the full tier.

## 3. Implementation & Verification Plan
Register in config.yaml docs, sync root copy, `TestRootDocCopiesMatchSources` green; add matching Makefile targets in harnez.
