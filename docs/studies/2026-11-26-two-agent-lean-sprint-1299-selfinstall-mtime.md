---
kind: sprint
date: 2026-11-26
ticket: 1299
branch: sprint/1299
models:
  searcher: skipped # host filed the ticket from the failing log; dev plan turn verified it
  developer: claude:haiku # developer lean default
  reviewer: claude # reviewer default
---

# Two-Agent Lean Sprint: #1299 — selfinstall mtime-dependent test

## Context

`TestRunLocalSelfInstallSymlinksGoBinToLocalBin` failed in the 1298 q1 run. The gobin fixture written during the test was newer than the cached test binary, so goself's shadow warning fired.

## Milestones

| Step | Commit | Result |
|---|---|---|
| Plan (read-only) | none | The check lives in `ubunatic.com/goself` v0.5.0 (`shadowWarning`) and the production logic is correct, so the fix is test-only |
| Fix | d1b8820b | fixture pinned to 2000-01-01; new `TestRunLocalSelfWarnsWhenGoBinIsNewer` |
| Review pre-work | 6342a544 | `t.Setenv("GOBIN", goBin)` in both tests |

## Verification

- The developer's quota runs passed twice, each after a source edit.
- The host's uncached run after `make install` passed.
- The host's run with `GOBIN` and `GOPATH` pointed elsewhere passed.

## Observations

- **The reviewer caught a second hidden dependency.** harnez never passes `Options.GoBinDir` to goself, so goself resolves GOBIN from the environment. The mtime pin alone would still fail on any host that exports `GOBIN` or `GOPATH`. The host's diff review had passed it. This is one more case for the one-shot reviewer, even on a 40-line test diff.
- **No separate red run.** A failure-first run would not have failed reliably, because it depended on the cache's age. The 1298 log was the reproduction, and the new test is the counter-check that the warning still fires.

## Follow-ups

- Noted in the ticket, out of scope: the shadow warning checks goself's gobin, not harnez's `Options.GoBinDir`.
