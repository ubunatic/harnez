# 760 — Add /harnez-selftest as a bench task

**Status**: Closed — resolved (task added in `7746ae3d`, results recorded in `6888aa1e` and `c4daec2d`)
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: #759 (planned bench run), `internal/bench/tasks.yaml`, `harnez-selftest` skill

---

> `/goal`: Add `/harnez-selftest` (the native agent background task self-test) as a `harnez bench` task, so models can be compared on it, or stop and report when blocked on a user decision or a denied permission.

## 1. Problem & Motivation

The bench today has documentation fact-finding and read tasks, but no task that exercises the agent background-task flow. `/harnez-selftest` checks that flow end to end, and it is the closest existing check to how developer agents actually run in harnez. Adding it as a bench task lets us compare models on it, alongside #759.

## 2. Technical Specification / Findings

- The bench task list lives in `internal/bench/tasks.yaml`. Task specs are validated when loaded, and each task has an `id`, a prompt and mechanical checks.
- Existing tasks: `hello`, eight documentation tasks, and three read tasks (`harnez bench tasks`).
- Open: whether the task runs the skill itself or a copy of its steps, and what its pass checks are. Confirm against the skill's current definition when the ticket is picked up.

## 3. Implementation & Verification Plan

1. Read the current `harnez-selftest` skill and confirm its steps and expected output.
2. Add a task entry to `internal/bench/tasks.yaml` with a prompt, mechanical pass checks and a clear reason column.
3. Run `harnez bench tasks` to confirm the task is listed, and run it once against one model to confirm it passes or fails on the right checks.
4. Run `make install` and the bench tests.

## 4. Results (2026-10-09)

Task run alone (`--task harnez-selftest --repeat 1`), one run per model, `HTO=0`:

| Model | Pass | Turns | Input tokens | Duration |
|---|---|---|---|---|
| claude:haiku-5-5:med | PASS | 2 | 45387 | 39.0s |
| codex:luna:high | PASS | 7 | 156813 | 39.7s |
| codex:sol:med | PASS | 9 | 210229 | 50.4s |

PASS means `harnez agent selftest --step verify` printed `PASS: Background execution self-test passed`. That line is printed only when the saved state records hello, background, confirm and confirm-running in order.

The `harnez-selftest` entry was missing from `internal/bench/tasks.yaml` after another session reverted the file. It was re-added on 2026-10-09 with the same definition used for these runs, and the bench tests pass.
