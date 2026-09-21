# 476 — Improve agent start/resume UX with explicit sync, async, workdir, and planning controls

**Status**: Open

**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics

---

## 1. Problem & Motivation

The agent handoff UX is easy to misunderstand even when the CLI is behaving
correctly. In a recent host workflow, the operator misread an empty initial
`harnez agent start` response as a failed launch and became confused about
argument ordering. The CLI already accepts the demonstrated argument ordering
robustly; the problem was the lack of immediate, self-explanatory feedback.

The same workflow also needs an explicit choice between waiting for a worker
and returning control immediately, plus a discoverable way to request planning
before implementation.

## 2. Proposed Improvements

- Make the initial `agent start`/`agent resume` response immediately identify
  the worker name or ID and summarize the task, even when the worker continues
  asynchronously.
- Add explicit `--sync` and `--async` modes, with synchronous behavior as the
  default where practical to reduce zombie-worker risk. On a host session's
  first `agent start` or `agent resume`, print a concise tip explaining the
  modes so the host can choose deliberately.
- Allow `-d <dir>` globally on the `agent` command wherever meaningful: as the
  worker working directory for start/resume and as an appropriate filter or
  scope for list/status-style operations.
- Add a planning control such as `--plan=yes|no|inline`:
  - `yes`: perform the planning preparation and exit without implementation;
  - `no`: do not add an automatic planning phase unless the prompt requests it;
  - `inline`: print the plan, tell the host that work will begin after a short
    wait (for example 30 seconds), and explain how to interrupt synchronous
    work or stop asynchronous work.

The existing flexible argument parsing must remain compatible.

## 3. Verification

- Add CLI tests for flexible argument ordering, immediate launch feedback,
  sync/async selection, first-use guidance, workdir handling, and all planning
  modes.
- Verify that interruption and explicit stop paths do not leave orphaned
  workers.
- Run the repository test suite and relevant CLI smoke tests.

/goal

Make agent start/resume behavior self-describing and predictable: hosts see
immediate launch identity, can choose sync or async execution, can scope the
working directory consistently, and can select an explicit planning mode
without losing existing argument-order compatibility or creating zombie
workers.
