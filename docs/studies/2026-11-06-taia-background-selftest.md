---
kind: canary
---

# Taia background self-test in Harnez

## Context

On 2026-11-06, the user invoked `/harnez-selftest` in the Harnez repository after personally running `git pull` (already up to date). The host was the new Taia coding agent, using its background-task plugin. The goal was to exercise the Harnez self-test's CLI-guided progression with the host's actual background runner, not to test all plugin features. This account is based on the visible session transcript, not independent telemetry.

## Outcome

The self-test passed. The observed sequence was:

1. The assistant summarized the task, activated `harnez-selftest`, and read the six session rules in one `harnez read` call.
2. `harnez agent selftest --step hello` initialized session `default` and instructed it to start a background job, then confirm it.
3. The assistant called the plugin's `background_task` with `HTO=0 harnez agent selftest --session default --step background`, label `selftest`, and output capture enabled. It returned task #1.
4. `harnez agent selftest --session default --step confirm` reported the task active. `background_task_list` showed #1 running at 4 seconds; `harnez agent selftest --session default --step confirm-running 4s` recorded that observation.
5. The assistant announced it would await notification, but prematurely called `background_task_result` at 9 seconds. It showed the job still running and no output. The assistant ended its turn reporting verification pending.
6. Taia delivered a completion notification: task #1 completed with exit 0. The assistant then read its captured output with `background_task_result`; it reported completion after 30 seconds and requested verification.
7. `harnez agent selftest --session default --step verify` returned: `PASS: Background execution self-test passed successfully. All steps executed in the correct sequence.`

The explicit session argument preserved the `default` identifier across separate tool calls. This run observed job launch, listing, captured output, notification and successful exit; it did not exercise cancellation or establish reliability across repeated runs.

## What Went Wrong

The assistant checked the result once before notification, contrary to the reactive-completion rule in `.harnez/rules/Tools.md` and its own stated intention. The result itself exposed the premature check: the job was still running. No plugin failure was observed. The assistant stopped checking, returned control, and continued only after the actual completion notification. Harnez's PASS validates its recorded CLI sequence, not compliance with the no-polling rule.

## Learnings

- Follow the CLI's requested immediate confirmation and job-list inspection, then stop calling tools until the host delivers completion; retrieve output and verify afterward.
- Preserve the session ID on every subsequent CLI step, and set `HTO=0` on the background command.
- Treat a self-test PASS as evidence for the tested sequence, not for cancellation, repeated reliability, or perfect agent process discipline.

## Follow-Up Tickets

No tickets allocated and no workers delegated. The premature check was an assistant execution mistake already covered by `.harnez/rules/Tools.md` and `docs/AgenticLoop.md`; the run provides no evidence of a product defect requiring a new ticket. Cancellation testing and repeated reliability measurements are deliberate non-goals for this single requested self-test.
