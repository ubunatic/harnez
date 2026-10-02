# 678 — Make usage view modes spec-driven and independent of --watch

**Status**: Closed — resolved
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[676-add-harnez-usage-minimal-mode]], [[160-watch-viewer-server-split-feasibility]]

---

## 1. Problem & Motivation
In a PTY, `harnez usage` changes its level of detail when `--watch` is added; for example, `--minimal --watch` can show the larger view. Users need view choice to remain stable while watch only controls refresh and interactivity.

## 2. Goal & Acceptance Criteria
- Define `normal`, `compact`, and `minimal` detail modes in `spec/usage.yaml` and its JSON Schema, including an explicit `normal` default when no mode flag is given.
- Add mode selectors `--normal`, `--compact`, and `--minimal`; the selected mode renders the same layout/detail in a PTY with or without `--watch`.
- `--watch` independently enables the render loop and interaction without changing the selected mode. Cover all modes and the no-flag default with parity tests.

## 3. Implementation & Verification Plan
Load the mode definitions from the validated usage spec and verify mode selection and one-shot/watch parity in tests.

## 4. Outcome
- Added spec and schema definitions for all three view modes, including the explicit `normal` default.
- Added mutually exclusive `--normal`, `--compact`, and `--minimal` selectors; `--watch` now only enables the render loop and interaction.
- Shared mode rendering across one-shot and watch, including footer row budgeting for short terminals. Recorded the reusable presentation contract in [[UsageCollection]].
- Added CLI, spec-loader, and parity coverage for the default and all modes, including constrained terminal heights.
- Verified with `HTO=0 make test-q1`, `make install`, and a `codex:gpt-6.1-sol` (`sol:med`) review with no actionable findings.
- Commits: `a09b39fe` implementation; `5b97f7e9` ticket closure.

**Goal**: Implement and verify spec-driven usage modes with watch independent of presentation, or stop and report if blocked on a user decision or denied permission.
