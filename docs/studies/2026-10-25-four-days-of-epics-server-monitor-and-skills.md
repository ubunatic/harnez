---
kind: story
---

# Four days of epics: server monitor, skills and a harness that checks itself

## 1. Header & Context

- Period: 2026-10-22 13:37 (commit 25ef1e46) to 2026-10-25 ~15:00 (HEAD f2b51d71).
- Previous story: [2026-10-22-generic-init-repo-yaml-story.md](2026-10-22-generic-init-repo-yaml-story.md)
  (per-repo `.harnez/repo.yaml`, tracker gating; parked 855 init wizard and 857 external issue providers).
- Scope: everything since then, across several host sessions. Sub-studies summarized here:
  [host two days](2026-10-23-session-retro-harnez-host-two-days.md),
  [usage never hangs](2026-10-23-two-agent-lean-sprint-usage-never-hangs.md),
  [epic 894 vendor-mixed](2026-10-24-two-agent-lean-sprint-epic-894-vendor-mixed.md),
  [epic 910](2026-10-24-two-agent-lean-sprint-epic-910.md),
  [P1/P2 sweep](2026-10-24-two-agent-lean-sprint-p1-p2-sweep.md),
  [epics 910/911 retro](2026-10-25-session-retro-harnez-epics-910-911.md),
  [epic 921 server monitor](2026-10-25-epic-921-server-monitor-sprint.md).
- Goal of the period (implicit, from the tickets): move from single lean sprints to epics, make `harnez usage`
  reliable and fast, let the server own remote monitoring, and turn the sprint practice into reusable skills.

## 2. Executive Summary

| Measure | Value |
|---|---|
| Commits | 345 (101 touch code outside `issues/` and `docs/`) |
| Commits per day | 31 (22nd, afternoon), 96, 134, 84 |
| Tickets closed | 61 (58 via close commits, plus 932, 933, 934 today) |
| Tickets filed | 57+ (several more today: 925, 927, 928, 929, 931–934) |
| Releases | v0.2.0, v0.2.1, v0.2.2 |
| Lines outside `issues/` | +18,703 / −8,150 in 324 files |
| of which production code | +8,409 / −4,982 (136 files) |
| of which tests and testdata | +8,812 / −3,086 (126 files) |
| of which docs | +1,482 / −82 (62 files) |
| Reverts | 1 (a ticket filed in the wrong repo, moved to lmcoder) |

Delivered, grouped:

- **Usage reliability and speed:** usage never hangs (873, bounded server check, 10s end-to-end), `--minimal`
  read speed (874), collectors recover after kill (714), AGY rows (710, 868), store catch-up keeps agent
  metadata (933), `agent models` 24.1s → 0.97s (924).
- **Server as monitor (epic 921):** `server stop/restart/logs/status --json` (861, 862), boot unit fix (886),
  remote binary upgrade on start (919), server-side remote load polling with UI heartbeat (920, 929), swap rows
  (912), `L` hotkey (922), title freshness `@x600 · live` and minimal watch default (932), hardened and widened
  remote install paths (928, 934).
- **Release safety:** asset upload race (863), refuse with unreleased breaking rows (866), close refuses
  without a code commit naming the ticket (867).
- **Structure:** `cmd/harnez` split into internal packages (884), usage system as a reusable library (560),
  test catalog and lighter tests (881, 883, 907), sandbox-fixture lint (895), neus as optional dependency (897).
- **Skills and practice:** epic (900, 918), new-project (899), retro (878), jam (880), tui-check (852),
  lean-sprint rules for host decisions (840), one-run rerun rule (896), touched-package rerun (876), retros
  must name a safeguard (858), feature sub-packages (877, 890), per-repo ticket cap (882), init wizard (855,
  parked last time, now done).

From the previous story's open list: 855 is closed; 857 (external issue providers) is still in the backlog.

## 3. What Worked Well

- **Epics as the unit of work.** Epics 894, 910, 911 and 921 each grouped 4–8 tickets with a task table; the
  host could report progress per epic and the owner answered decisions in batches.
- **Plan-only first developer turns are now routine.** Every sprint this period used them. Plans were changed
  before code at least five times (920 split into M1/M2, 928 shared validator scope, 934 doc placement, 919 M4
  unit change, 860 earlier).
- **Sol developers finish in one round more often.** The 932, 928, 933 and 934 developers each delivered a
  milestone in 1–6 minutes with green `make test-q1`.
- **The harness checks itself.** `issues close` refuses without a code commit (867), the quota-1 runner warns
  when files changed after the last full run, `harnez index` catches index drift, and the new `kind:` front
  matter lets `/story` find its predecessor. Several of this period's guards were filed from earlier slips.
- **Live host checks found what tests could not** (see section 4) — the practice from the previous story held.

## 4. Honest Post-Mortem (Failures, Bugs & Near-Misses)

| Problem | How caught | Fix |
|---|---|---|
| A worker autosquashed pushed commits, main diverged (bug hunt, 22nd) | `git status -sb` divergence | rule "never amend/rebase" in every handoff |
| 850 closed with a match still above the cutoff | host review | "close only after acceptance, reason states evidence" |
| `harnez usage` hung 30s after each server restart (714 M3, circular readiness) | host first-minute test | 873 bounded check; skill rule "test services in the first minute" |
| Remote upgrade failed inside the systemd sandbox (919) | live restart; tests used an injected runner | `-F ~/.ssh/config`; sandbox repro recipe in the 921 study |
| 920 closed while the box showed stale data | owner screenshot | 920 M3 and 929 (real `load-stream` sample with observation time) |
| All agents shown `installed: false` after store catch-up (933, P1) | owner report | b0356c2a; bisect pointed to a Sep 30 change |
| 932 one-shot test still asserted an hour-old sample | host full run | 932 M3 |
| 934 M1 shell-quoted the scp target; default upgrade broke | host live restart; tests asserted the wrong operand | 934 M3 plus a test that fails on quoting in scp argv |
| 934 M1 let `.` components bypass the `/tmp` rule | host diff review | 934 M2 |
| Developer edited tests after its full run and committed unrun code (920) | host full run (vet) | 6d07b130; rule 896 |
| Host rule slips: sed and heredoc edits on tickets (22nd–25th, at least four times) | self-noticed or owner | admitted each time; no tool-level guard yet |
| Status reports did not name the developer's model ("is Haiku slow?") | owner question | reports now name the model |

Pattern: almost every user-visible defect was caught by a live run or by the owner, not by tests. Tests asserted
what the developer believed (wrong scp operand, old label, injected runner without sandbox).

## 5. Quality & Invariants Audit

| Area | Verdict | Evidence |
|---|---|---|
| Architecture & separation | Improved | 884 moved command logic out of `cmd/harnez`; 560 made usage a library; remote polling moved from UI to server (−556 lines in watch code) |
| Idempotency | Stable | remote upgrade keeps equal/newer versions; `harnez init -d .` and `harnez index` leave clean trees |
| Backward compatibility | Stable | `--normal` kept as hidden alias; legacy remote versions upgrade; spec keys renamed only where no user config used them |
| Tests | Improved in volume, mixed in rigor | tests grew more than code (+8.8k vs +8.4k); `make test-q1` green at each close; but three closes this week needed a follow-up because the test encoded the bug |
| Lint | Diverged | `gokay check` lint and spec stages fail (927, open) |

Overall: quality improved structurally; correctness depends on live checks more than it should.

## 6. Efficiency & Velocity Assessment

- 345 commits and 61 closed tickets in about 3.5 days; roughly one closed ticket per hour of active time.
- Code churn is honest growth, not padding: production +3.4k net, tests +5.7k net. Docs grew modestly.
- Developer turns this session: 1–10 minutes; the slowest (920 M1+M2, Luna) 10m22s.
- Waste: one extra live cycle each for 919, 920, 932 and 934 (about 5–15 minutes each), and one wrong-repo
  ticket (reverted).

**Smoothness compared with the previous story.** Rising, but not evenly:

- Better: decisions batched once per epic; plan-first is default; developers rarely need a second round for
  scope; guards (867, quota-1 warning, index check) stop several earlier classes of slips automatically.
- Flat: the host still finds defects live at about the same rate (previous story: 2 of 3 real defects in scratch
  repos; this period: 919, 920, 932, 934 live). The cause is the same — tests mirror the developer's model.
- Worse in one point: host rule slips (shell edits) repeated across sessions; prose rules alone do not stop them.

## 7. Key Learnings & Evergreen Upstream

1. **A test that asserts an external contract must not take its expected value from the code under test.** scp
   operands, sample ages and sandbox behavior need a real-tool or real-time check. Proposed target:
   `docs/TestTiers.md` (canary/container tier).
2. **Code that runs under the server unit needs a sandbox repro before close** (from the 921 study). Proposed
   target: `docs/HarnezServerArchitecture.md` pitfalls.
3. **Host edit rule needs a tool guard, not more prose.** Proposed: a hook that flags `sed -i`/heredoc writes to
   tracked files in host sessions (owner decision).
4. **Status reports name each worker's model** — now practice; proposed for the lean-sprint skill text.

## 8. File & Diff Summary

- Range 25ef1e46..f2b51d71: 345 commits, 324 files outside `issues/`, +18,703 / −8,150.
- Largest areas: `internal/usage/` (collector, watch, remote load), `internal/server/` (lifecycle, remote
  upgrade), `cmd/harnez/` split into `internal/*`, `docs/commands/` (epic, new-project, retro, jam, tui-check,
  story), `spec/` (usage, server, actions).
- Releases: v0.2.0, v0.2.1, v0.2.2 (tag 25f1a601, Codeberg and GitHub).
- Follow-ups filed from this story: 936 (test rule for external contracts), 937 (host edit-rule guard). Not
  filed, by choice: learning 2 is already in the 921 study's proposal list; learning 4 is a one-line skill edit
  left to the owner.
