---
kind: sprint
---
# Lean sprint 1207: harnez issues comment via fj (2026-11-10)

## Agents

- Searcher `s1207`, codex:luna:low: forgetracker resolves host/slug from origin (SHA256-safe); the forge API
  already reports PR and state; fj takes `--body-file`.
- Developer `d1207`, codex:terra:low: plan turn, M1 4m12s, M2 4m10s.
- Reviewer `r1207`, codex:terra:low: P2 local-tracker repos refused entirely, P3 weak dry-run/no-fj assertions.

## Milestones

- M1 (API preflight: PR always refused, closed needs `--closed`; fj comment; login error mapping), `c22f0118`.
- M2 (`--forge` for repos with a local `issues/` dir, exact argv and no-fj-run assertions), `33cb9b92`.

## Findings

- The session started from a manual fj test: fj alone fails in SHA256 repos ("no git repo found") and
  gives a raw 401 without login; harnez's own origin parsing avoids the first, error mapping the second.
- M1's only test failure was a stale error wording; the host ran the owed full run (206 ok).
- Live dry runs: taia #253 and harnez #150 (both closed) refused without `--closed`; harnez without
  `--forge` refused with the flag named. No real comment was posted by the new command.
