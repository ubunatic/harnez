# 370 — Offer useful Go agent capabilities through harnez init

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `docs/studies/2026-09-16-ramp-levels-and-go-repository-init.md`, `docs/studies/Quota1Approach.md`, [[368-report-ramp-evidence-and-projected-changes-from-harnez-init]], [[369-detect-go-library-cli-and-tui-shape-for-init-guidance]]

---

## 1. Problem & Motivation

L2 context tells an agent about a Go project, but reusable task procedures can make common changes and reviews more reliable. A generic agent file or decorative workflow would inflate a RAMP label without helping real work.

## 2. Scope & Design

- Offer opt-in, project-local reusable Go procedures via `init`, based on the inventory from 369: change-and-verify and independent review are the initial tasks. State trigger, inputs, steps, output, and verification. Install only when the chosen harness can consume the artifact, and classify it as projected L3 until committed.
- Library procedures should check public API compatibility, package examples and error behavior. CLI procedures should check flags, stdout/stderr, exit codes, configuration and cancellation. TUI additions should check terminal restoration, resize, Unicode display width and deterministic model/state tests.
- Detect Quota-1 rules, wrapper usage and `test-q1` target separately. Use the project’s actual test entry point; never claim Quota-1 counts turns or is enforced on direct test commands. Do not automatically enable it merely for a higher RAMP score.
- Preserve customized skills/commands, support preview and clean re-init, and avoid generated session logs or fake L4 flow files. Leave cross-harness global installation to `apply`.

## 3. Exit Criteria

- [ ] Opt-in is explicit, works for at least one supported harness, and writes only the project repo.
- [ ] A real Go library and CLI/TUI pilot can execute or follow the generated procedure without invented paths or commands.
- [ ] Detection reports L3 only for a real, usable capability artifact; generated working-tree content remains projected.
- [ ] Re-init preserves custom text and makes no change when already synchronized.

## 4. Verification

Run fixture checks and one end-to-end task/review canary for each pilot shape. Use the Quota-1 test target once per source-edit step and record any manual override separately. Review the generated text for unsupported claims.
