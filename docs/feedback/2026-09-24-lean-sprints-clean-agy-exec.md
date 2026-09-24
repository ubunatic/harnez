# 2026-09-24 — Lean sprints 533, 536, 537, 541, 353: what worked, what slipped

Host: Claude session `harnez-bf`. Developers: `codex:terra:med` / `terra:low` via `harnez agent`.
Tickets: 532, 533, 536, 537, 541 closed; 353 in progress; 538–544 filed. Handoff: 544.

## What worked

- **Host diff review caught real defects every sprint.** 533 M2: an unconditional retry reopened the 532 hole.
  536 M2: reversed drift arrow. 537 M5: the coverage view falsely reported shim commands as unrouted. 541 M1: grandchild stops no longer detected.
  None of these failed a test. They were found by running the real command (the lean-sprint "plausibility
  check" is the most valuable step).
- **Pre-work in the ticket** kept every correction in one place, and developers picked it up reliably.
- **Cross-session messages** (loom-05, lucky-fox) delivered first-hand evidence (532, CPU, OOM) that the host
  could verify directly (`journalctl`, cgroup scopes, `/proc`).

## What slipped

- **A performance regression shipped with green tests.** 533 M4 added a 25ms full-/proc poll. Nobody measured
  CPU until a peer saw ~3 cores in `top` (541). Rule: new polling or wait code gets a CPU measurement in review.
- **Missing history check before redesigning.** 537 M2 rebuilt the agy hook rewrite that 271 had retired
  on purpose (UI leak). `harnez find issues "agy shim"` would have shown it in one call. The redesign (hook
  fallback + shim) came only after the user asked "what is the status quo?".
- **`harnez agent stop` came too late.** The developer's turn finished and committed M2 anyway. Stopping is
  not a reliable brake mid-turn; decide before dispatch.
- **Host false alarm.** The 536 M1 "wrong directory" finding came from a statusLine capture that the host session's
  own render had overwritten. Capture into a file per session, or check `session_name` before concluding.
- **Developers closed tickets themselves** (541) and made edits after their single test run (533 M3, 537 M7).
  The host reran `make test-q1` each time. Keep saying "don't close the ticket" in the prompt.
- **Probe side effects.** Checking alias resolution with `harnez agent start --model X -- x` started real
  sessions. Probe with a dry-run or `harnez agent models`, never a real launch.
- **Instruction gaps found by the user, not the harness:** the CxxxE.md naming rule (353 still open),
  milestone labels in status updates, "always `make install`" (now a managed rule).

## Efficiency

- Codex 5h window: ~60% used over the afternoon across this session's and peer sprints. No zombie processes;
  the drain was normal developer turns (the largest single session was ~11%). The `terra:low` and `terra:med`
  developers did comparable work at similar cost here.
- The one-writer rule mattered: host doc edits waited for `cpu541` to commit before `make install`, so a
  half-edited `harnez exec` never got installed.
