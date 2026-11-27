---
kind: sprint
date: 2026-11-27
tickets: [1308, 1300]
---

# Lean sprint 1308: 5-hour share per session and a per-session factor below 1 weekly pp

## Setup
- Started in decide mode on the owner's "how to file/update issues and run sprints so we can answer #2 and #5 properly".
- Developer: `dev-1308`, claude:haiku:med; final one-shot review agy:neon:low (the owner's picks for 1305, reused).
- Searcher skipped: the host filed 1308 from live report output and code locations (report.go functions by line);
  the developer's first turn is read-only and re-verifies them.
- M0 done by the host: it needs real quota-history, which helper prompts must not read.

## Log
- Filing: 1308 (33eff0ec); 1300 status points to it. Why one ticket: the 5-hour share (Q2) is what makes the
  per-session factor (Q5) finer.
- M0 (344b14b7): one 5-hour pp ≈ 1/12 weekly pp (587 / 47 without the first window; 9.2-16.7 for windows with
  ≥ 4 weekly pp). Verdict: range ratio, windows with ≥ 4 weekly pp.
- Turn 1 (read-only plan): dispatched.
- Turn 1 (1m9s, peak 64.6k): plan accepted. Five premise notes (weekly path hard-coded; quiet check needs
  weekly-equivalent pp; per-window weekly delta undefined; sum acceptance is not an identity; buckets weekly-only)
  and five design questions. Host decided all five in the ticket (contiguous weekly delta per window, capped
  windows count, quiet check via ratio, both factors at ≥ 1 weekly pp, sum as a reported gap).
- Turn 2 (4m32s, peak 89.9k, one `make test-q1`, 83 ok): M1 84a0712c. Host live check: 5-hour shares + not
  explained = 661 pp exactly; ratio 661/55 = 12.0 (M0 12.5); 59 of 123 sessions ≥ 1 5-hour pp (5 have ≥ 1 weekly
  pp). A 1-call $0.0019 session takes a whole 2 pp rise until the M2 quiet check. Per-milestone Claude review
  skipped to spare the weekly quota (92 %); one agy:neon:low review for M1+M2.
- Turn 3 (M2 251237c0, 83 ok; several package runs before `make test-q1`, over budget): quiet fix holds (the
  $0.0019 session is quiet now), 57 of 123 sessions get a 5-hour factor (22 measured). Host plausibility check
  caught a range ratio of 6.88 against M0 12.5. The first window after the weekly reset took the whole pre-sample
  weekly level as its rise. Green tests hid it; the partition-sum test from the M1 pre-work was never written.
  Logged as M2b pre-work. Claude weekly 93 %.
- Turn 4 (M2b 899e358c, 2m47s, one `make test-q1`, 83 ok; budget kept): reset baseline fix plus the missing
  partition-sum test, both failing first. Live: ratio 12.19, Σ weekly-eq 38.97 vs 42 attributed (gap −3.0 pp);
  Haiku 25x, Opus 76x on solo rises. One agy:neon:low review over M1..M2b dispatched.
- Review (agy:neon:low, 2m10s, 222k tokens incl. cached): weekly path unchanged after the refactor. Found: an old
  weekly group's rise is dropped when a 5-hour window spans the reset (real, matters for `--since 7d`); a first
  window without a weekly baseline; threshold, split-basis and capped-ratio tests missing; two stale comments. It
  also flagged range-edge windows: decided as intended (whole windows) and documented. All of it is M3 pre-work.
- Turn 5 (M3 475cbff6 + 47df949a, 3m39s, one `make test-q1`): reset-span and late-sample fixes, the missing tests,
  Telemetry.md. Live acceptance passed: ratio 11.98 (9 windows), gap −2.5 pp, 58/126 sessions with a factor, all
  invariants exact. The hand check found the legend's "API $" is really the cost in attributed 5-hour rises: M3b.
