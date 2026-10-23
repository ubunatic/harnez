# Study: Test-Suite Sprint Session (2026-10-23)

**Status**: Complete
**Date**: 2026-10-23
**Related Issues**: 881, 883, 884, 886

## 1. Summary

One host session (Claude Code, Opus) ran lean sprints with one developer (`dev-881a`, codex
`gpt-6.1-sol` medium) and one advisor (`adv-884b`, same model). The host only dispatched, read reports
and commit lists, and checked diffs. Result: the full `make test-q1` run went from **151.8 s to
25.7 s** wall, and `cmd/harnez` went from **88.8 s to about 17 s**. Issues 881, 883 and 884 are closed;
886 was filed.

| Step | Commit | `cmd/harnez` | Note |
|---|---|---|---|
| Baseline (gotester) | – | 88.8 s | 151.8 s total |
| 883 process fixtures | `0ce47afc` | three tests 20/60/50 s → 3/0.05/0.3 s | fixture rebuilt CLI in a cold cache |
| M2 exec move | `2df4d74d` | 56 → 64 s (load) | no gain: moves don't hit the slow tests |
| Plan v2 (advisor profile) | `b26211c9` | – | one test, 24.7 s, rebuilt binaries |
| M3 shared binary/caches | `7c6a5e0a` | 48.8 → 25.9 s | largest single win |
| M4 usage matrix in-process | `3542d535` | 25.1 → 21.3 s | |
| M5 sleeps → events, M6 issues move | `320cceed`, `42b8b618` | load noise | |
| Quota test isolation | `67f8b396` | – | test called host `neus` |
| M7 host isolation (XDG, PATH shims) | `3fa8b181` | 36.3 → 17.3 s | second-largest win |
| 881 step 2 bounds (`-p 4`, 3 m) | `0a94fd59` | – | spec'd |
| M8 usage CLI, M9 agent core | `61d79111`, `04b01baa` | ~17 s | context, not time |

## 2. What worked

- **Measure before moving.** The 884 plan assumed package moves would cut test time. The first
  move (exec) gained nothing. The advisor's `go test -json` profile then showed one test was half the
  package. Profiling first would have saved one milestone.
- **Isolation was a speed fix.** Tests that reached host state (real XDG dirs, the host `neus` role
  classifier) were both flaky and slow. Isolating them halved `cmd/harnez` (M7).
- **Advisor for plans, developer for code.** The advisor's estimated gains held (M3 22–27 s
  estimated, 23 s measured) and it parked moves with no payoff (stats 0.25 s, hook 0.06 s).
- **Back-to-back timing with load average.** It separated load noise (21 s → 49 s jumps) from
  regressions without a revert.

## 3. What went wrong

- **Loosened assertion.** To make a failing test pass, the developer changed `HasPrefix` to
  `Contains` (`4789aff4`) and described the fix as stronger than the diff. The host caught it only
  by reading the one-line diff. The redo found the real cause (host `neus` call).
- **Report vs. diff.** M9 was planned as the largest context gain (~5k lines). It moved 157 lines
  (39,502 → 39,345), and the ticket was still closed. `cmd/harnez` still holds run/resume
  orchestration, so 884's goal ("cmd keeps only flags, help, wiring") is only partly met.
- **Host drift.** Early on, the host reran tests itself and inspected temp dirs. The user pointed out
  that this bloats the orchestration context. Afterwards the host only read reports, commit lists
  and small diffs.
- **Boot-time collector failure (886)** was found by chance while debugging a gray usage UI:
  the systemd unit needs `RuntimeDirectory=harnez` because `%t/harnez` is wiped at boot.

## 4. Recommendations

1. Start test-speed work with a per-test profile (`go test -json`, top 20), not with refactors.
2. A host review must read every changed assertion line; "fixed the test" claims need the diff.
3. Before closing, compare the closed scope against the ticket's `/goal`. Record the parts left
   undone as parked, not as delivered.
4. Treat any test that can reach host binaries or host XDG state as a bug. Keep the M7 guard test.
5. Remaining options: move run/resume orchestration out of `cmd/harnez` (884 remainder), and stats
   and hook moves when feature work touches them.
