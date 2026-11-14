<!-- harnez:variant=lite -->
---
title: Agentic Loop Practices (Lite)
weight: 40
---

# Agentic Loop Practices — Lite

Rules only; normal engineering practice is assumed. Rationale and evidence: `harnez docs variant agentic-loop full`.

## 1. Core Invariants

1. **Parallel Read, Sequential Write** — many may read; one writer per workspace. Dispatch one subagent at a time unless the user asks for parallel work: even read-only helpers race on shared sequential resources such as the next ticket number.
2. **Canary & Test-Driven Verification** — run the real tests before saying done. Tests of an external contract (scp/ssh operands, sandbox behaviour, clocks, real CLI or hook JSON) take expected values from the real tool or a recorded sample, never from the code under test; add a canary or container check where unit tests cannot run the tool.
3. **Zero Zombie Guarantee** — track and stop every background process, timer and subagent before the session ends. Background jobs get `HTO=0`; then wait for the completion notification, no polling or `sleep` loops. Hung quota-1 tests: `harnez clean procs q1 --kill`.
4. **Responsive Host Orchestrator** — a handoff is no licence to block the chat: start workers in the host background runner, report the handoff, and wait only when the user asks or the next step needs the result.
5. **In-Repository Single Source of Truth** — tickets, decisions, retros and session notes live in git (`issues/`, `docs/studies/`, `docs/feedback/` when present, otherwise a ticket), not only in chat. Tickets carry the work; messages (`harnez agent send`) only point at them.
6. **Context Discipline** — never whole-file-read `AGENTS.md` or other files already in the prompt; for files of 100 lines or more use `harnez read -L <range>` or `-n`. Global instruction files cite docs as plain text, never `@docs/...`: Claude Code inlines those into every session.
7. **Media & Demo Verification Gate** — **always ask the user for explicit confirmation** of recorded output (reels, screenshots, terminal recordings) before publishing or embedding it.
8. **Deployment Transparency (remote-deploying projects only)** — **Local State**, **Deployed Artifact State** and **Active Daemon State** are independent. A local build or test proves only the first; probe the live host for the other two.
9. **One-Level Delegation** — roles are `orchestrator|developer|reviewer|advisor`. Only an orchestrator starts helpers; leaf workers never run `harnez agent`, native subagents or delegating skills (harnez enforces it). A named `provider:model:tier` is dispatched exactly; if it is unavailable, report and ask, never substitute. **Completion Evidence**: the host checks every "done" report with `git show --stat <commit>` and the built output.
10. **Disk Headroom** — before a download or build of a few GB, run `df -h <target>`; keep 10 GiB free or ask first.
11. **Study & Retro Ticket Extraction** — a study or retro is complete only when every actionable follow-up is a ticket listed in it by number, or a justified non-goal.
12. **Repository Scope & Product Primacy** — this repository is the sole product. Never offer or start implementation in another repository; there, only read for context or file a requested or blocking issue.
13. **Steered Model Escalation** — start developers on `--model lean`; escalate to `std` or `smart` on struggle, failed verification or ambiguity, then drop back. Ask for a read-only plan first and grant write authority on `resume`.
14. **Immediate Product-Issue Capture** — file larger product issues at once (high priority when sprint-blocking); fix small local ones directly.

- Decide low-risk, reversible choices yourself; record choice and reason under `## Decision` in the ticket and report where it is.
- Limit plans and promises to this session; turn a lasting lesson into a check, a rule line or a ticket and name where it lives.
- TUI layout work: a checked-in `.ansi` mockup under `docs/data/` is the visual spec (`TUIDesign.md`).
- Peer sessions: `/peer-assistant` handles explicit requests only; it is no always-on watcher.

## 2. The 5-Phase Sprint Loop (`/sprint`)

Discovery (parallel read-only advisors), Development (one writer), Review gate, Hygiene, Retrospective. These rules hold in every workflow:

- **Kickoff & Commit Policy** — settle who commits and whether you hold standing commit authority. Advisors cite `file:line` from the code; docs may be stale.
- **Repro-before-fix** — a bug, timing or race ticket needs a reproduction with a concrete baseline number before the fix.
- **Commit stale/failed work before discarding it** — commit or branch an abandoned attempt, then revert; skip only trivial one-liners.
- **Milestones** — commit each one when it verifies; collect non-blocking nuances in one refinement ticket.
- **Review gate** — read the diff and the assertions; check docs and ticket sync. After a developer's green full run, rerun only the touched packages.
- **Live/Real-Environment Verification** — hooks, global config and environment resolution need one real end-to-end check after a genuine restart or re-apply. Green unit tests alone are **Unit-Test-Only Confidence**.
- **Root Cause vs. Symptom** — before hardening defensive code, try to fix the source of the bad input. After every fix, re-run the real failing case, not only the new tests.
- **Exit condition** — commit, or ask the user to authorize the commit. Never move on with reviewed work uncommitted (**Reviewed-But-Uncommitted Carryover**).
- **Hygiene** — stop helpers and background tasks, then `harnez rate --ok "<note>" [<ticket>]`.
- **Retrospective** — friction goes into a durable doc or ticket; run `harnez index` and `harnez status`; keep one Status field per ticket. **Closing gate**: close shipped and verified tickets before the session ends. Run `git status` first. After a long goal or 5+ closed tickets, run `/evergreen`.

## 3. Lean and Reverse Sprints

The skills carry the steps.

- **`/lean-sprint`** (one ticket): the host does zero coding and no exploratory digging and reviews diff-first; a Cheap Searcher (`--role advisor --model cheap`) answers bounded read-only questions; a Focused Developer (`--role dev --model lean`, or `std`, `smart`, `fast`, `cheap`) works test-first with milestone commits. Running report in `docs/studies/` when present (date-prefixed `two-agent-lean-sprint-<slug>`), otherwise in the ticket. Legacy: `/lean-sprint-v1`.
- **`/reverse-sprint`**: a cheap developer leads and codes; a `smart` reviewer checks each milestone diff; a `smart` advisor is called only when stuck, with bounded context; compact every 100–150k tokens.

## 4. Calibrated Friction Reporting

Report friction only after substantive sessions, without repeating known quirks, and with an actionable root cause: blocker, failure mode, workaround tried, recommended fix.

**Name items in reports**: give each milestone or ticket a short name at least once ("M3 (single-write removal)", "498 (Claude resume)"), not a bare "M3" or "498".

## 5. Anti-Patterns to Avoid

- **Goals Without Exit Clauses** — a `/goal` without one deadlocks the Stop hook when progress needs the user. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report (tried, why it failed, resume point) in the ticket.
- **Shell `&` Detaching** — never `&`, `nohup`, `setsid` or `disown`. Run `HTO=0 harnez agent start|resume` as one session-background job per worker (Claude Code: Bash `run_in_background: true`, under 30 minutes); `harnez agent start --help` covers the other hosts.
- **Blocking `sleep` Waits** — no long or chained `sleep` to wait out CI, a deploy or a queue; use the completion notification or a scheduled wakeup matched to how fast the state changes.
- **Chat-Visible Empty Polling** — no short-interval "still running" checks; surface events, not heartbeats.
- **Buffered Long-Running Output** — a long command piped through `tail`, `grep`, `sort`, `wc` or `head` shows nothing until it exits; run it plain or `tee` to a log.
- **Chatty Watch Wrappers** — no poll-and-redraw tools (`gh run watch`, `docker logs -f`) in routine targets; query status and print one summary line.
- **`cd`-scoped commands** — use `git -C`, `make -C` or a subshell; see `Bash.md`, Directory scoping.
- **Stopping An Unverified Process** — signal a process only with a known start time; fail closed without one.
- **Baking Real Credentials In** — no real hostnames, MACs or credentials "temporarily"; example values and a secret scanner from the first commit.
- **Narrow String-Substitution Edits** — prefer `apply_patch` or whole-block replacement.
- **Prompt Micromanagement** — do not restate base rules or tool docs in a subagent prompt.

<!-- harnez:model-picks:begin -->
These are the shipped picks. User models.yaml overrides are listed by `harnez agent models --picks`.

| Role | Trait | Meaning | Chain | Default |
|---|---|---|---|---|
| advisor | cheap | Low-cost work. | luna:low, haiku:low | yes |
| advisor | smart | Difficult judgment. | opus:low, opus:med, sol:low |  |
| developer | ai | AI-oriented work. | unset |  |
| developer | cheap | Low-cost work. | luna:med |  |
| developer | fast | Fast work. | haiku:med |  |
| developer | gui | Graphical interfaces. | lean, std |  |
| developer | lean | Efficient standard work. | terra:low | yes |
| developer | smart | Difficult judgment. | terra:high, sol:med, opus:med |  |
| developer | std | Standard work. | terra:med, opus:low |  |
| developer | tui | Terminal interfaces. | smart |  |
| reviewer | smart | Difficult judgment. | sol:low | yes |
<!-- harnez:model-picks:end -->
