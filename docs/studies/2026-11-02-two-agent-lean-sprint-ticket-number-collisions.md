---
kind: sprint
date: 2026-11-02
tickets: [1099]
---
# Lean sprint 1099: ticket numbers claimed by open PRs

- **Setup:** host Opus 5.5 with developer `dev-1099` on haiku:med, chosen by the owner. No searcher: the plan-only first turn did the discovery.
- **Plan turn:** the developer inspected the live code and probed Codeberg read-only.
  - `ls-remote` lists all 81 `refs/pull/*/head`, merged PRs included, with no state.
  - `/pulls?state=open` gives the open set.
  - The developer offered option A (open-PR scan at filing time), option B (a pre-merge check) or both, and four open decisions.
- **Owner sign-off:** both checks, scan all known forges, offline warns and never blocks. Recorded under `## Decision` (commit `bba698ce`).
- **Delivery:** one resume implemented M1 to M4.
  - `d9e6af47`: forge open-PR list.
  - `20b21774`: `issues new`, `find issues next` and `issues mv` share the claims set.
  - `6b05afcb`: `issues lint --against`, plus the pre-push hook and the `/issue` step.
  - `2c2238e3`: docs.
  - The developer ran `make test-q1` once: exit 0, 68 ok.
- **Host verification:** the installed binary was run live: a two-forge scan in 0.77s, a warning when offline, and a collision in a scratch clone naming both paths.
- **Gaps:**
  - The open-PR scan has not yet been checked with a PR actually open; none was open during the sprint.
  - The pre-push hook checks the working tree, not the pushed commits.
  - `find issues next` now makes network calls.
- **Flow:** two developer turns in total. The plan-first sign-off cost one extra turn and avoided a `refs/pull` fetch design that would have counted merged PRs.
