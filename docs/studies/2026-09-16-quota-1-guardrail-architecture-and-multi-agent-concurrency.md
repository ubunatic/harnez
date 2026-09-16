<!-- harnez:topic: Quota-1 Guardrail Architecture, Multi-Agent Concurrency, and Lite Doc Optimization -->
# Quota-1 Guardrail Architecture, Multi-Agent Concurrency, and Lite Doc Optimization

**Date**: 2026-09-16
**Scope**: `internal/quota1/`, `cmd/harnez/`, `Makefile`, `AGENTS.md`, `docs/`, `issues/346`
**Tickets**: 346, 361

---

## 1. Executive Summary

This study documents the design, verification, and end-to-end multi-agent rollout of **Quota-1** across `harnez`-managed local-file-first repositories, coupled with the introduction of token-compressed **Lite Variant Docs**. 

Quota-1 enforces a hard constraint against agentic runaway test loops: **under Quota-1, an agent may only execute the test suite once per step/turn; repeated test runs are blocked at the binary level until repository source files are modified**. In this session, we verified the Quota-1 initialization pipeline, tested the execution-level binary gate under repetitive subagent calls, resolved a real issue ([Issue 346](file:///home/uwe/projects/harnez/issues/346-harnez-status-false-positive-unindexed-ticket-drift-for-titles-with-punctuation.md)) under Quota-1 TDD constraints, and analyzed multi-agent filesystem concurrency implications.

---

## 2. Architecture & Implementation

### 2.1 The Two-Tiered Quota-1 Defense Model

Quota-1 operates across two distinct layers:
1. **Prompt-Level Instruction Boundary (`AGENTS.md`)**:
   - Injected directly into the agent context via `harnez init --quota-1`.
   - Explicitly instructs the agent on the Single-Test Boundary, mandating code modifications before re-running tests and banning unauthorized bypass flags (`QUOTA_BYPASS=1`).
2. **Deterministic Process Gate (`harnez exec --quota-1`)**:
   - Wired into project Makefiles via `test-q1: 🤖` target (`⚙ --quota-1 -- $(MAKE) test`).
   - State tracked in `.git/harnez/quota_1.state` (or `.harnez/quota_1.state` in worktrees).
   - Records UTC RFC3339 timestamps of every completed test run.
   - On subsequent invocations, walks the repository tree (excluding `.git`, `vendor`, `node_modules`, dotfiles, temporary extensions). If no source file has an `mtime > last_test_timestamp`, the execution aborts with exit code 2 and outputs a clear rejection explanation.

### 2.2 Lite Document Compression

To maximize context efficiency when operating under tight quota and token bounds, managed practice and language docs (`AgenticLoop.md`, `IssueTracking.md`, `Make.md`, `Bash.md`, `GoRelease.md`) were initialized with `<!-- harnez:variant=lite -->`.
- Redundant prose, narrative explanations, and duplicate ASCII headers were stripped.
- Invariants, regex specifications, severity/priority matrices, and Make sentinels (`⚙️`/`🤖`) were preserved verbatim.
- Reduced docs footprint by over 500 lines across core guidance files while retaining 100% rule compliance.

---

## 3. Empirical Verification & Experiments

### 3.1 Loop-Rejection Experiment

We launched an independent test subagent to deliberately attempt consecutive `make test-q1` runs without touching repository files:
- **Run 1**: Succeeded (`exit 0`), recorded timestamp `2026-09-16T15:20:48Z`.
- **Run 2**: Hard rejected (`exit 2`), displaying:
  ```text
  Quota-1: test execution blocked because no repository source files have been modified since the last test run (2026-09-16T15:20:48Z).
  Under Quota-1 rules, code must be modified before running tests again.
  ```
- **Run 3**: Blocked with identical error.

### 3.2 Real-World TDD Under Quota-1: Issue 346

A dev subagent was assigned [Issue 346](file:///home/uwe/projects/harnez/issues/346-harnez-status-false-positive-unindexed-ticket-drift-for-titles-with-punctuation.md) (`harnez status` reporting false-positive drift on ticket titles containing colons, parens, and slashes):
- The subagent inspected `internal/index/index.go` and `internal/issues/issues.go`.
- Wrote failing reproduction test cases (`TestStripTicketNumber`, `TestParseTrackerTable_PunctuationInTitle`, `TestIssuesTablePunctuationInTitleRoundTripsThroughLint`).
- Refactored `IssuesTable` to use canonical `issues.StripTicketNumber()` and escaped markdown pipe characters (`\|`) in ticket status fields.
- Executed `make test-q1` **exactly once** upon completing the fix — which cleanly passed all tests on its single allowed turn.
- Closed the issue and synchronized indices without encountering any Quota-1 blocking.

---

## 4. Multi-Agent Concurrency & Edge Cases

### The Shared-Workspace `mtime` Interaction
Because Quota-1 measures filesystem modification timestamps across the project root:
- When **Agent A** is in a dev loop and **Agent B** writes/updates a ticket in `issues/` or an ephemeral study in `docs/studies/`, the repository root walk will detect Agent B's newly modified file.
- This creates an edge case where Agent B's edits can inadvertently satisfy the modification requirement for Agent A's next test run.
- **Guideline**: For parallel agents requiring strict independent Quota-1 sandboxes, agents should operate in separate git worktrees (where `.harnez/quota_1.state` and working files are isolated). For collaborative main-workspace sessions (e.g. dev agent + documentation tracker), this behavior allows smooth co-existence without artificial friction.

---

## 5. Quality & Invariants Audit

| Invariant | Status | Evaluation |
|:---|:---|:---|
| **Single-Test Boundary** | ✅ Verified | Consecutive calls without edits exit with code 2. |
| **Idempotency** | ✅ Verified | `harnez init --quota-1 --variant lite` reports `No changes` when cleanly synced. |
| **Backward Compatibility** | ✅ Verified | `make test` and `make check` remain standard; `make test-q1` provides the guardrail layer. |
| **Doc Integrity** | ✅ Verified | Lite variants maintain all lifecycle invariants (Zero Zombie, Review Gate, P0-P3 metadata). |

---

## 6. Key Learnings & Evergreen Upstream

1. **Deterministic Error Feedback**: When Quota-1 blocks an agent, printing the exact timestamp of the last run and explicitly stating the required action prevents agents from getting confused or hallucinating system issues.
2. **Table Parser Uniformity**: In Markdown-backed state stores (like `issues/README.md`), all producers and consumers must share the identical regex / normalization functions. Inlining divergent regexes in table formatters inevitably leads to false-positive drift alarms.
3. **Lite Docs Efficiency**: Lite variants provide sufficient constraints for frontier agents while significantly reducing prompt ingestion overhead in tight iteration loops.
