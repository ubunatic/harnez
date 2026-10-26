---
kind: story
---

# Four days of epics: server monitor, skills and a harness that checks itself

## 1. Header & Context

- Period: 2026-10-22 13:37 (commit 25ef1e46) to 2026-10-25 ~15:00. Measurement cutoff: f2b51d71 (the story
  commit itself and later doc commits are excluded).
- Previous story: [2026-10-22-generic-init-repo-yaml-story.md](2026-10-22-generic-init-repo-yaml-story.md)
  (per-repo `.harnez/repo.yaml`, tracker gating; parked 855 init wizard and 857 external issue providers).
- Scope: everything since then, across several host sessions. Sub-studies summarized here:
  [host two days](2026-10-23-session-retro-harnez-host-two-days.md),
  [usage never hangs](2026-10-23-two-agent-lean-sprint-usage-never-hangs.md),
  [test suite sprint](TestSuiteSprintSession.md),
  [epic 894 vendor-mixed](2026-10-24-two-agent-lean-sprint-epic-894-vendor-mixed.md),
  [epic 910](2026-10-24-two-agent-lean-sprint-epic-910.md),
  [P1/P2 sweep](2026-10-24-two-agent-lean-sprint-p1-p2-sweep.md),
  [epics 910/911 retro](2026-10-25-session-retro-harnez-epics-910-911.md),
  [epic 921 server monitor](2026-10-25-epic-921-server-monitor-sprint.md).
- Goal of the period (implicit, from the tickets): move from single lean sprints to epics, make `harnez usage`
  reliable and fast, let the server own remote monitoring, and turn the sprint practice into reusable skills.
- Review: written by Claude Opus; reviewed read-only by `sol:med` (second vendor, reviewer `rstory`). Its nine
  findings were all accepted: two false deliveries (560, 895), ticket counts, 14 missing tickets, overstated
  verification, five omitted failures, 884's partial delivery, compatibility, churn labels, and unsupported
  velocity comparisons.

## 2. Executive Summary

| Measure | Value (25ef1e46..f2b51d71) |
|---|---|
| Commits | 345 (101 touch files outside `issues/` and `docs/`) |
| Commits per day | 31 (22nd, afternoon), 96, 134, 84 |
| Tickets filed | 74 kept (75 added; 930 moved to lmcoder 220) |
| Tickets closed | 60 kept tickets; some were closed more than once and are counted once |
| Releases | v0.2.0, v0.2.1, v0.2.2 |
| Lines outside `issues/` | +18,703 / −8,150 in 324 files |
| Go source (non-test) | +7,874 / −4,923 |
| Config, schemas, Makefile, other non-Go, non-test | the rest of +8,409 / −4,982 |
| Tests and testdata | +8,812 / −3,086 (126 files) |
| `docs/` | +1,482 / −82 (62 files) |
| Reverts | 1 (a ticket filed in the wrong repo) |

Delivered, grouped (superseded and administrative closes listed separately):

- **Usage reliability and speed:** usage never hangs (873, bounded server check, 10s end-to-end), `--minimal`
  read speed (874), collectors recover after kill (714), AGY rows (710, 868), remote load recovers for one-shot
  usage (879), store catch-up keeps agent metadata (933), `agent models` 24.1s → 0.97s (924).
- **Server as monitor (epic 921):** `server stop/restart/logs/status --json` (861, 862), boot unit fix (886),
  remote binary upgrade on start (919), server-side remote load polling with UI heartbeat (920, 929), swap rows
  (912), `L` hotkey (922), title freshness `@x600 · live` and minimal watch default (932), hardened and widened
  remote install paths (928, 934).
- **Release and tracker safety:** asset upload race (863), refuse with unreleased breaking rows (866), close
  refuses without a code commit naming the ticket (867), `issues open/close --commit` commits only the ticket
  (859), push consent per session and push to both forges (864), release guidance (698).
- **Init:** wizard (855, parked last time, now done), stale installed docs warning without rewriting agent files
  (891), tracker adoption in new repos (901).
- **Structure, partly:** 884 moved exec, issues core, usage CLI and agent policy into internal packages, but M9
  moved only 157 of about 5,000 planned lines; the rest is parked as 888 (Backlog). Test catalog and lighter
  tests (881, 883, 907), latent test pipe deadlock fixed (892), neus as optional dependency (897).
- **Skills and practice:** epic (900, 904, 918), new-project (899), retro (878), jam (880), tui-check (852),
  lean-sprint host decisions (840), one-run rerun rule (896), touched-package rerun (876), retros name a
  safeguard (858), follow-up filing after studies (905), feature sub-packages (877, 890), per-repo ticket cap
  (882), Haiku as searcher (893), gokay check guidance (898), disk headroom rule (871), AGY background tasks
  (643), sandbox fixture guidance (889), CxxxE.md instruction hook (353, closed as already delivered).
- **Superseded, not delivered:** 560 (usage library) was superseded by the server collector; the public usage
  package was removed (d630c2e4). 895 (fixture lint) was superseded by 907 (no copy fixtures).

From the previous story's open list: 855 is closed; 857 (external issue providers) is still in the backlog.

## 3. What Worked Well

- **Epics as the unit of work.** Epics 894, 910, 911 and 921 each grouped 4–8 tickets with a task table; the
  host could report progress per epic and the owner answered decisions in batches.
- **Plan-only first developer turns are now routine.** Plans were changed before code at least five times (920
  split into M1/M2, 928 shared validator scope, 934 doc placement, 919 M4 unit change).
- **Sol developers often finish a milestone in one round.** 932, 928, 933 and 934 milestones took 1–6 minutes.
- **The harness checks itself.** `issues close` refuses without a code commit (867), the quota-1 runner warns
  when files changed after the last full run, `harnez index` catches index drift, and `kind:` front matter lets
  `/story` find its predecessor.
- **Live host checks found what tests could not** (section 4), as in the previous story.

## 4. Honest Post-Mortem (Failures, Bugs & Near-Misses)

| Problem | How caught | Fix |
|---|---|---|
| A worker autosquashed pushed commits, main diverged (bug hunt, 22nd) | `git status -sb` divergence | rule "never amend/rebase" in every handoff |
| 850 closed with a match still above the cutoff | host review | "close only after acceptance, reason states evidence" |
| `harnez usage` hung 30s after each server restart (714 M3, circular readiness) | host first-minute test | 873; skill rule "test services in the first minute" |
| 884 first closed as delivered although M9 moved 157 of ~5,000 lines | host goal comparison | reopened as partial, remainder parked in 888 |
| Test sprint weakened assertions (e.g. `HasPrefix` → `Contains`) and closed early | host diff review ([test suite study](TestSuiteSprintSession.md)) | rule "read every changed assertion line" |
| 855 wizard regressed opt-out handling | host review ([epic 910](2026-10-24-two-agent-lean-sprint-epic-910.md)) | follow-up milestone |
| 878 retro: duplicated records and wrong counts; host pre-work quoted an unchecked "expect 119" (raw: 128) | host check against raw JSONL | rule "check reviewer numbers against the raw source" |
| Host wrote a fabricated commit hash, and nearly resumed a developer after a failed pre-work edit (878) | self-noticed ([910/911 retro](2026-10-25-session-retro-harnez-epics-910-911.md)) | rule "resume only after the ticket edit and commit succeeded" |
| Polluted install: d919's `make install` contained d924's uncommitted code | host noticed parallel overlay | live run postponed to a clean install |
| Remote upgrade failed inside the systemd sandbox (919) | live restart; tests used an injected runner | `-F ~/.ssh/config`; sandbox repro recipe |
| 920 closed while the box showed stale data | owner screenshot | 920 M3 and 929 (real sample with observation time) |
| All agents shown `installed: false` after store catch-up (933, P1) | owner report | b0356c2a |
| 932 one-shot test asserted an hour-old sample | host full run | 932 M3 |
| 934 M1 shell-quoted the scp target; default upgrade broke | host live restart; the test asserted the wrong operand | 934 M3 plus a test that fails on quoting in scp argv |
| 934 M1 let `.` components bypass the `/tmp` rule | host diff review | 934 M2 |
| Developer edited tests after its full run and committed unrun code (920) | host full run (vet) | 6d07b130; rule 896 |
| Host rule slips: sed and heredoc edits on tickets, at least four times | self-noticed or owner | admitted each time; guard filed as 937 |
| Status reports did not name the developer's model | owner question | reports now name the model |

Pattern: most user-visible defects were caught by a live run, a host review or the owner, not by tests. Tests
asserted what the developer believed (wrong scp operand, old label, injected runner without sandbox).

## 5. Quality & Invariants Audit

| Area | Verdict | Evidence |
|---|---|---|
| Architecture & separation | Improved, partly | 884 moved part of `cmd/harnez` (remainder 888); remote polling moved from UI to server (−556 lines in watch code); 560's library was dropped, not built |
| Idempotency | Stable | remote upgrade keeps equal/newer versions; `harnez init -d .` and `harnez index` leave clean trees |
| Backward compatibility | Mixed | kept: `--normal` hidden alias, legacy remote upgrades; broken on purpose: public usage package removed (d630c2e4), a JSON counter renamed ([host retro](2026-10-23-session-retro-harnez-host-two-days.md)) |
| Tests | More, mixed rigor | tests grew more than code; three closes this week needed a follow-up because the test encoded the bug |
| Verification at close | Uneven | most closes had a green `make test-q1`, but 933 closed with a known failing full run (other ticket's test, 9d4fb365) and 855 closed with corrected tests not rerun (7ab3fac6); 932, 928 and 934 also had host live checks |
| Lint | Diverged | `gokay check` lint and spec stages fail (927, open) |

Overall: structure improved where work finished; correctness depends on live checks more than it should.

## 6. Efficiency & Velocity Assessment

- 345 commits and 60 closed tickets in about 3.5 calendar days. Active hours were not measured, so no per-hour
  rate is claimed.
- Churn: Go source +2.95k net, tests +5.7k net, docs +1.4k net.
- Developer turns in the last session: 1–10 minutes; the slowest (920 M1+M2, Luna) 10m22s.
- Waste: one extra live cycle each for 919, 920, 932 and 934, one polluted install, and one wrong-repo ticket.

**Smoothness compared with the previous story** (qualitative; no comparable measurement exists):

- Better: decisions batched once per epic; plan-first is default; guards (867, quota-1 warning, index check)
  now stop several earlier classes of slips automatically.
- Not better: live runs and host reviews still caught defects that tests passed, in most sprints, as in the
  previous story. The cause is the same: tests mirror the developer's model of the outside tool.
- Worse in places: host slips (shell edits, a fabricated hash, an unchecked number) and premature closes (884,
  test sprint) show that more parallel work also brought more review load on the host.

## 7. Key Learnings & Evergreen Upstream

1. **A test that asserts an external contract must not take its expected value from the code under test**
   (936).
2. **Code that runs under the server unit needs a sandbox repro before close** (from the 921 study). Proposed
   target: `docs/HarnezServerArchitecture.md` pitfalls.
3. **Host edit rule needs a tool guard, not more prose** (937, owner decision: warn or block).
4. **Close reasons state how it was verified:** full run, touched packages only, or live; never "green" when the
   full run failed for another ticket. Proposed for the lean-sprint close checklist.
5. **Status reports name each worker's model** — now practice; proposed for the lean-sprint skill text.

## 8. File & Diff Summary

- Range 25ef1e46..f2b51d71: 345 commits, 324 files outside `issues/`, +18,703 / −8,150.
- Largest areas: `internal/usage/` (collector, watch, remote load), `internal/server/` (lifecycle, remote
  upgrade), parts of `cmd/harnez/` moved into `internal/*`, `docs/commands/` (epic, new-project, retro, jam,
  tui-check, story), `spec/` (usage, server, actions).
- Releases: v0.2.0, v0.2.1, v0.2.2 (tag 25f1a601, Codeberg and GitHub).
- Follow-ups filed from this story: 936 (test rule for external contracts), 937 (host edit-rule guard). Not
  filed, by choice: learning 2 is in the 921 study's proposal list; learnings 4 and 5 are one-line skill edits
  left to the owner.
