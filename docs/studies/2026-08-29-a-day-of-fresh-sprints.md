---
title: A Day of Fresh Sprints
weight: 90
---

# A Day of Fresh Sprints

*2026-08-29 — a retrospective on one long `/fresh-sprint` session*

## The shape of the day

It started with a single ticket — issue 101, "why does `harnez usage` stop
showing AGY after a few hours?" — and turned into a dozen. Not because scope
crept, but because the lean fresh-handoff loop this repo has been building
toward for weeks (`docs/practices/AgenticLoop.md`, `/fresh-sprint`) finally
had enough surface area to run on its own terms: spawn a subagent with a
tight brief, let it self-verify against real tests and real commands, review
the diff inline, commit, move to the next ticket. Repeat.

By the end of the day, thirteen issues had moved — five filed fresh from
live conversation (097–101), eight resolved or explicitly closed with a
documented reason not to pursue (055, 058, 061, 068, 086, 097, 098, 100),
one deliberately declined (099, AppImage — evaluated and rejected, which is
its own kind of progress), and one real production bug found and fixed that
was never on the ticket list until a user said, twice, "I still can't see
it."

## What actually shipped

The headline items:

- **A real install canary.** `scripts/install-canary.sh` doesn't mock
  anything — it hits the live Codeberg API, pulls the actual `v0.1.5`
  release into a throwaway Debian container with no Go toolchain, and
  asserts `--version` matches. First run, first try, passed against
  production. This is what `docs/other/Canary.md` has been arguing for
  since it was written — probe the real mechanism before trusting it.
- **DEB/RPM packaging**, verified the same way: not "the YAML looks right,"
  but `dpkg -c` and `rpm -qlp` run against locally-built snapshot artifacts,
  inspected by hand.
- **A `/harnez-sync` command** and a new `harnez init --all` batch verb —
  the workspace's answer to "twenty-plus sibling repos, one set of managed
  docs, how do they stay in sync without a human doing it by hand."
- **The AGY quota bug**, which is the most interesting story of the day
  precisely because the first fix wasn't the whole fix.

## The AGY bug, in three passes

This is worth dwelling on, because it's a small case study in why "tests
pass" and "the user can see it work" are different bars.

**Pass one** (issue 101): stop treating "haven't refreshed in 30 minutes"
as "hide the agent." Split the live-refetch threshold from a real
auto-hide threshold, add a "last updated" timestamp. Shipped, tested,
marked Resolved.

**The user checked. It still didn't work.**

**Pass two**: traced it further and found the *write* side had a bug the
read-side fix couldn't see — the collector was overwriting a good cached
snapshot with an emptier live one every time AGY wasn't actively running,
which is most of the time. Added a guard so a write can't destroy signal
the cache already had. Shipped, tested.

**The user checked again. Still nothing.**

**Pass three**: the on-disk snapshot had already been empty for days
before either fix landed — there was nothing left for either guard to
protect. But a separate history log (`usage-history/*.jsonl`) still had
real quota data from six days back. Nobody had wired a fallback to read
it. That fallback is what finally put quota bars back on the screen, timestamped
honestly as "6d4h ago."

Three passes, three distinct root causes, in the same small package, on the
same feature. None of the earlier passes were wrong — they were each a real
fix for a real problem the tests correctly proved. What they weren't was
*sufficient*, and the only way to find that out was to keep running the
actual command and looking at actual output instead of trusting a green
test suite to mean "done." The fix, in the end, was to insist on that live
check every time, not just on the first pass.

## What this says about the harness itself

A few things got reinforced today, worth writing down before they fade:

- **A subagent's scope boundary is a real boundary, not a suggestion.**
  Mid-task, a message arrived asking one docs-only agent to also patch
  `internal/usage`. It refused — correctly — citing its own dispatch
  instructions and this repo's "parallel read, sequential write" rule. That
  refusal was the system working, not a failure to route around.
- **Parallel dispatch is a tool, not a default.** Early in the day, four
  agents ran at once across disjoint files. Later, once several tickets
  converged on the same package (`internal/usage`), sequential execution
  became the only safe mode — confirmed out loud mid-session ("parallel is
  too risky anyway"). Recognizing which regime you're in matters more than
  picking one style and sticking to it.
- **"Resolved" needs a verification story attached to the commit, not just
  a green build.** Every ticket in the table above that touched running
  code got a real invocation, not just `go test ./...` — a live container
  pull, a live `harnez usage` render, a real temp-git-repo diff. The AGY
  saga is the argument for why that discipline exists.
- **Filing a ticket from a live conversation is cheap and worth doing
  immediately.** Issues 097–101 all came from things noticed in passing
  during unrelated work, written down before the context evaporated. Two
  of them (100, 101) turned into same-day implementations. Backlog
  hygiene compounds.

## What's still open

`internal/usage` has more queued — 051 (multi-host dashboard navigation),
087 (generalizing the flock+freshness live-fetch gate from Claude to Codex
and AGY, in progress as of this writing), 090, 093, 094. The older,
pre-priority-field backlog (005–046) is untouched. None of that is a
problem to solve tonight — it's the next day's fresh sprints.
