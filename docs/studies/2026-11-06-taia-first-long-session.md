---
kind: retro
---

# First longer Taia session in Harnez

## Context

On 2026-11-06, the owner and Terra used the young Taia host for a longer Harnez session. This study combines the [background self-test account](2026-11-06-taia-background-selftest.md), [issue 1012](../../issues/1012-agent-selftest-hello-and-later-steps-omit-session-so-separate-tool-shells-fail.md), [issue 1181](../../issues/1181-isolate-harnez-session-state-and-constrain-installation-destinations.md), git history, and the owner's session account. The self-test account describes visible transcript evidence, not independent telemetry; the owner's account supplies the operational observations below.

## Outcome

The background self-test passed with one explicit session ID carried through its steps. Issue 1012 exposed why the printed commands initially failed across separate tool shells: session resolution could change between calls. Commit `f0bd6090` made the commands carry the resolved session and added cross-shell tests; the owner ran `make test` outside Taia, and the issue was closed.

The owner also made permissions and installation fixes during the session. Issue 1181 remains open: its characterization and host-neutral design work address session isolation, installation destinations and process safety. Owner review selected a limited fail-closed mitigation for agent-stop targets with unknown start times; that stop work is ongoing, not a completed isolation solution.

## What Went Wrong

The self-test's original guidance did not preserve continuity across shells. In the later successful run, the assistant checked a background result once before the completion notification; the recorded PASS did not establish compliance with the no-polling rule.

Taia's PTY and cross-device restrictions prevented the owner from relying on the in-host test run, so the owner ran `make test` outside it. Bubblewrap also reported `setting up uid map: Permission denied`; issue 1181 treats namespace policy separately from filesystem permissions. Reviews caught test and process safety issues. In particular, a passing fake-signal characterization in commit `c88e8a7f` shows that a forged process record can cause a fake SIGTERM; it does not prove record authentication or signal safety.

## Learnings

- Copyable cross-shell commands must carry their state identity explicitly, and tests should exercise separate invocations rather than assume a shared shell.
- A passing self-test proves its recorded sequence, not cancellation, repeated reliability, or correct waiting discipline.
- Separate observed sandbox restrictions from their causes: the reported Bubblewrap error is not resolved merely by granting filesystem access.
- The evidence suggests host-neutral contracts and owner review are useful before changing shared process state or installation paths. Same-user modes, start-time checks and fake signals alone do not establish isolation.

## Follow-Up Tickets

- [Issue 1012](../../issues/1012-agent-selftest-hello-and-later-steps-omit-session-so-separate-tool-shells-fail.md) is closed; its regression coverage and externally run suite address the observed continuity failure.
- [Issue 1181](../../issues/1181-isolate-harnez-session-state-and-constrain-installation-destinations.md) is actionable and open: finish and review the stop mitigation, then address authenticated session boundaries, constrained installation and namespace-policy investigation under its acceptance criteria.
- Taia issues 140 and 142 were reported; those are Taia issue numbers, not Harnez tracker numbers (Harnez's issues 140 and 142 concern unrelated usage-watch and rate-feedback work). This study does not assert what those Taia issues cover.
- The PTY and cross-device restriction was reported, but read-only Harnez tracker searches for `taia PTY`, `PTY Taia`, and `Taia` did not identify a matching Harnez ticket. Its ticket ID remains unknown; do not infer one. No actionable follow-up beyond issue 1181's scoped isolation/install and namespace investigation is evidenced here, so no additional ticket is needed from this study.

No new issue is warranted by the premature result check alone: existing rules already cover that execution mistake.
