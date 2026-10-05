# Roadmap

Working roadmap for the open backlog (updated 2026-09-27, reconciled against 8a0b95f; previous
pass f90faf4). Derived from each ticket's appended `## Implementation Plan` or its `/goal` and
specification sections, so scope calls here reflect the planning pass, not a fresh re-derivation.

**Value axis.** harnez is the single source of truth for everything a coding agent reads:
`config.yaml` drives settings, hooks, instructions, skills and doc copies across Claude Code,
Codex, AGY and Prime Agent, and `apply` must stay idempotent and never touch user-managed
keys (README). On top of that core sit three surfaces used every day: `harnez agent` dispatch
(cheap developer agents driven by an orchestrator, now reachable over CLI **and** the `harnez
mcp` server), the issue tracker, and the telemetry that measures whether the harness actually
saves tokens and turns. Work that makes those surfaces more correct, more composable and cheaper
per session outranks work that adds new surface.

**What changed the axis this pass (2026-09-24 → 2026-09-27):**

1. **Dispatch got a second channel and got safer.** The MCP server (571, 572, 574, 575, 583,
   584) gives hosts async start plus `harnez_wait_agent`; auth preflight (596), a pre-prompt
   compaction gate (590, 591), a runtime token watchdog (592) and quota-aware `agent models` /
   `agent start` (603, 604) all shipped. The dispatch backlog is now mostly *discoverability in
   other repos* (539, 557, 581) and *failure visibility* (594, 595, 606), not missing features.
2. **The usage split changed shape.** 560 (P1, In Progress) turns usage data into a public Go
   library `ubunatic.com/harnez/usage` with an optional shared controller, and 607 adopts loom's
   view for `usage --compact --watch` behind a flag (strangler, from loom 103). So the rule is no
   longer "everything usage moves to loom": **loom owns views, harnez owns the usage data
   contract.** §2 is rewritten around that.
3. **Instruction layout gets a P1 owner.** 567 (P1) moves harnez-only rules out of AGENTS.md into
   `.harnez/rules/` with a tool-neutral AGENTS.md, and 568 splits the mixed copyable docs. This
   supersedes the layout half of 494 (`*.harnez.md`/`*.local.md`).

**Strategic goal kept — OS-agnostic readiness (macOS first).** Still valid for the harnez core
(hooks, shims, `exec`, CI). 560's open portability question (flock, socket) is the newest
instance.

Sequencing buckets:

- **Now** — actionable today, no blockers, high daily-loop value, or a prerequisite for other
  work.
- **Next** — actionable but larger, lower daily value, or waiting on a Now item.
- **Later** — real but speculative, large, or dependent on decisions not yet made.
- **Close / Park / Move** — should not be scheduled here; see §9.

---

## 0. Shipped Recently

**2026-09-30 (usage store and agent lifecycle):** **650-655** (usage store read seam, collector
registry, passive ingestion, session attribution, consolidation with archived legacy files),
**657** (compact shows Claude/Codex/AGY again), **660** (`usage --watch` fast start, `q` quits,
terminal restored), **661** (`agent stop` kills the process tree and reports truthfully). Invariants
are in `docs/UsageCollection.md` → "Status and invariants". Next: **659** (statusline latency),
**630** (session-background guides per host), **656** (opt-in `agent send`).

**Closed since the f90faf4 pass (2026-09-24 → 2026-09-27), grouped:**

- **Previously bucketed here:** **509** (tip leak in tests) and **511** (Quota-1 keeps full log)
  — both were **Now** in §7; **517** (Claude effort support, was **Now** §1b-q); **522** (reuse
  fresh quota readings, was **Next**-high §1b-q); **523** (lean-sprint host runs live checks, was
  **Next** §5); **035** (metering proxy M1–M3 delivered, was **Park** §9); **273** (superseded by
  537, was **Next** §13).
- **Dispatch & MCP:** 571 (`harnez mcp` server), 572 (async dispatch + `harnez_wait_agent`), 574,
  583, 584 (registration for AGY and Claude, schema fix), 575 (`harnez_command` tool), 578, 585
  (managed block and Subagent Policy teach MCP vs Bash), 540 (short aliases), 590, 591, 592
  (compaction gate, token watchdog), 596 (Codex auth preflight), 602 (`-p` on start; managed
  Start line without `--detach`), 603 (quota exhaustion in `agent models`/`start`), 604
  (flash38 escalation-only), 605 (`/goal` exit-clause rule).
- **Exec & Quota-1 hardening:** 524, 532, 533 (run records, `clean` reaps stuck processes and
  Quota-1 state), 541 (exec busy-wait 94% → 0.3% CPU), 543 (`read -I` OOM bounded; re-enable is
  546), 551 (shell unwrap for tool inference), 553, 554 (tests isolated from real `~/.local/bin`;
  Quota-1 in read-only bwrap), 576.
- **AGY:** 527 (quota probe bound), 529, 530 (stats reliability labels), 536 (statusline), 537
  (agy shell routing through `harnez exec`), 550, 558, 559 (launcher and meter env).
- **Telemetry & bench:** 598 (telemetry, history and quota caches moved to XDG paths — a large
  part of 515), 561, 562, 565, 570, 573 (`harnez bench`).
- **Tracker, docs, skills:** 525, 528, 577, 586, 588 (issue flow and lint), 579, 593 (`.ansi` TUI
  mockup workflow), 582 (pluggable `find code|docs` finders), 597 (peer-assistant skill), 599
  (`/harnez-status`), 600 (`/next`).

**Earlier passes (kept for reference):** 519, 507, 498, 520, 521 (2026-09-24); 514, 506, 504, 500;
496, 488, 289, 497, 491 (2026-09-22); 489, 490, the 479 epic (480–484, 478, 486), 464, 470, 467,
469, 268, 156, 135, 291, 462; 457, 127, 341, 424, 425, 428, 454, 315, 442, 374, 149, 417, 030,
114, 429–434, 166, 399, 443, 448, 452, 455, 456; 006, 018, 042, 045, 046, 056, 070, 071, 072, 095
pt.1, 096, 105, 108, 123, 125, 126, 139, 201, 209, 210, 216, 217, 222, 229, 249, 261, 262, 263,
290, 292, 299, 301, 303.

## 1. Component system (`apply` composability)

491 (the keystone) shipped last pass. What remains consumes its selection.

| Ticket | Scope | Bucket |
|---|---|---|
| 493 — `mixed` subagent dispatch mode, dispatch-mode-aware sprint skills | M — `mixed` in `agentpolicy`, interception honours it, sprint skills stop hard-coding `harnez agent start`. The MCP route (578, 585) is now a third dispatch form the mode must name | **Next** |
| 495 — shared token-capture package with stable API and `spec/` schemas | M — capture/parse per provider, a versioned `Record` in `spec/`, no storage. **Align with 560's public `usage` package**: both are "stable data contract for outside consumers"; decide whether token records live beside usage snapshots before building | **Next** (after 560's schema decision) |
| 492 — persist selection in `~/.config/harnez/local.yaml` | S/M — move the local-config loader out of `internal/usage`. Still needed; less time-pressured now that usage stays as a library rather than leaving wholesale | **Next** |
| 494 — project-level selection in `init` | **rescope**: the file-layout half is now 567 (`.harnez/rules/`). What stays is component selection for `init` | **Next** (after 567) |

Rationale: unchanged in spirit — 493 and 495 are independent of each other. 495 moves behind a
design check with 560 because shipping two separate "stable public data" packages would be
the duplication the component design warns against.

## 1a. Cross-agent dispatch (`harnez agent`, `harnez mcp`)

| Ticket | Scope | Bucket |
|---|---|---|
| 594 — Codex compaction never acknowledged; over-limit resume always fails | M — P1, In Progress. Plan: set `model_auto_compact_token_limit`, read context from the rollout's `last_token_usage`, verify after the turn. A stuck worker is the worst dispatch failure | **Now** (finish) |
| 539 — agents in other projects don't know `harnez agent` or short model aliases | S/M — P1. 585 and 578 put a `harnez agent` section into the managed block and 540 added short aliases, so most of the fix may already be out. **Verify in cati after `harnez init`**, then close or reduce to what is missing | **Now** (verify first) |
| 557 — agent docs: descriptive names vs real commands (`agent run`/`loop` do not exist) | S — same discoverability fix as 539; land together | **Now** (with 539) |
| 595 — Codex start failure hides the real 401 | S/M — 596's auth preflight catches the common case; the remainder is surfacing the rollout's `task_complete.error.message` when start still fails | **Next** (high) |
| 477 — hook-driven background task completion instead of polling | M/L — **moved up**: last pass it waited on 476's async mode; 572 shipped async start + `harnez_wait_agent`, so the trigger now exists and can be verified | **Next** (high) |
| 587 — MCP tool calls block and hit the client timeout | S/M — async exists (572); what is left is early-detach guidance or an automatic switch when a sync call will exceed the client limit | **Next** (with 477) |
| 476 — explicit `--sync`/`--async`, start feedback, `--plan inline` | M — **reduced**: async now exists over MCP (572); the remainder is the CLI flag parity, first-call tip and `inline` planning | **Next** |
| 538 — report leftover processes at turn end, reap only stopped ones | M — P2; `internal/subagent` starts workers without their own process group. 533 fixed the exec side | **Next** |
| 547 / 548 / 549 — `HTO=0` for agent sessions; direct `harnez agent` commands only; short Bash calls and prompt files | S each — one agent-ergonomics batch; 549 overlaps 581's "visible to the user" goal | **Next** |
| 526 — `agent delete` warns about unrated sessions | S — ratings feed model comparison | **Next** |
| 449 + 485 — quota-aware default model | S/M — 603 now marks exhausted providers; what remains is choosing a default from 5h/weekly headroom. Explicit `--model` always wins | **Next** |
| 306 — quarantine Codex sessions unusable after usage limits | S/M — P1 in the tracker; re-read against 603 (exhaustion) and 596 (auth preflight) before building, the remaining gap may be small | **Next** |
| 463 — `--escalated --reason` on start/resume | S — with 476's flag work | **Next** (with 476) |
| 144 / 383 — Codex subagent model policy; disable queued question prompts | S each | **Next** |
| 435 — native-subagent interception + A/B telemetry | reduced to A/B telemetry; needs 493, 487, 495 | **Later** |
| 581 / 606 / 450 / 288 / 342 | see §9 | **Park / Close** |

Rationale: last pass ordered by "where does work go" (quota) then "how does the host wait".
603 settled most of the first half, and 572 delivered async, so the order is now: **don't lose
a worker** (594), **let hosts in other repos find dispatch at all** (539, 557), then **make
failures visible** (595) and **stop polling** (477, 587).

## 1b-q. Quota & model economics

| Ticket | Scope | Bucket |
|---|---|---|
| 515 — one discoverable home for all telemetry data | M → S/M — P1. **Partly delivered by 598** (XDG paths with locked migration). Re-read and reduce to what is left: decoy stores, `usage history timeline` reading `quota-history.jsonl`, one documented root | **Now** (re-scope, then finish) |
| 508 — reject agent calls on confirmed-exhausted quota | **likely delivered by 603** (`agent models` and `agent start` honour provider exhaustion). See §9 | **Close candidate** |
| 516 / 518 — measure agy (Google Pro) and gpt-6-sol plan-quota COST | S each — measurement only; 522 removed the extra agy polls and 527 bounded the probe, so the blocker from last pass is gone | **Next** |
| 501 — `agent models` marks interactive-only providers | S | **Next** |
| 512 — docs list only aliases defined in `spec/agent.yaml` | S — test gate; 540 added more aliases | **Next** |
| 510 — web-only researcher role | S | **Next** (with 176) |

## 2. Usage data vs usage views (560, 607) — rewritten this pass

Last pass said the whole usage TUI leaves for `../loom`. 560 refines that: harnez keeps and
publishes the **usage data contract** (public `usage` package, optional single controller),
and loom supplies **views**, which harnez adopts behind a flag (607). Tickets are placed by
which half they touch.

| Ticket | Scope | Bucket |
|---|---|---|
| 560 — reusable Usage System and Go library | L — P1, In Progress; M1 (`bf9e275`, snapshot reader) shipped. M2 (controller lock, IPC) is **blocked on design decisions** recorded in its §7: supported OSes and lock/socket abstraction, owner idle lifetime, forced-refresh semantics, schema independence | **Now** (decide the §7 questions; code M2 after) |
| 564 — capture session status-line usage data | M — Claude's status-line payload has rate-limit percentages and context tokens; 560 names it as input to the public schema before M4 | **Next** (before 560 M4) |
| 556 — basic context numbers in the Claude status line | S — In Progress; same payload as 564 | **Now** (finish) |
| 534 — capture agent session tokens continuously | S/M — live sessions show 0 tokens; the status-line payload (564) is one source | **Next** (with 564) |
| 535 — agent-collector lifecycle: install, auto-start, keep current | M — 560 M3/M5 replace the collector with an opt-in controller; do the lifecycle there, not twice | **Next** (fold into 560 M3/M5) |
| 563 — meter Claude and Codex traffic like agy | M — P3; 035's proxy works for agy | **Later** |
| 607 — adopt Loom view for `usage --compact --watch` behind a flag | M — P3, new. Strangler step from loom 103; plain and Loom views share one data model | **Later** (after 560 M4 settles the data client) |
| 542 — idle CPU of `usage --compact --watch` (~8% of a core) | S/M — P2; fix in the plain view now, or let 607's Loom view replace it. Measure first | **Next** |
| 143 — Git status in all agent status bars | M — `internal/statusline` stays in harnez | **Later** |
| 152 | see §9 | **Close** |
| 255, 256, 085, 295, 264, 250, 251, 253, 278, 404, 219, 214, 146, 247, 051, 160, 161, 330, 172, 141, 111, 113, 084 | view, collector and mic/box features | **Move to loom / re-sort after 560** (§9) |

Rationale: 560 is the only P1 in the usage area and four open tickets (564, 534, 535, 607) read
its contract. Its M2 cannot start until §7's questions are answered, so the Now item is a
decision, not code. The collector tickets (111, 113, 535) likely become 560 M3 work rather than
loom work; re-sort them when M3 is planned.

## 2b. Token sources

| Ticket | Scope | Bucket |
|---|---|---|
| 034 — hook-triggered token extraction (AGY, Claude) | rescope into 495 (capture half) | **Next** (as a 495 consumer) |

## 3. Telemetry data layer & analytics

| Ticket | Scope | Bucket |
|---|---|---|
| 446 — persist cost fields reported by agents | S/M — after 495 | **Next** (after 495) |
| 445 — counterfactual API rate cards in `spec/` | M — after 446 | **Next** (after 446) |
| 487 + 461 — agent role/parent attribution; `model` column on `tool_calls` | M — one migration. 537 M7 added attribution for agy shim rows, which is a start | **Next** (high) |
| 124 + 296 — PostToolUse capture + always compute distill savings | M + S/M | **Next** |
| 468 → 178 — compact `make test-q1` output via distill; distill smart mode | S/M → M — 511 shipped the log-path half; 468 compacts on top | **Next** |
| 225 → 215 — classifier baseline, then LLM backfill | S → M | **Next** |
| 552 — fast LLM classifier for exec command names | discovery, P3; 551's rule fix covers known shapes | **Later** |
| 458 / 421 / 208 — telemetry SQL into `spec/`; command usage telemetry; SQLite export | as before | **Later** |

## 4. Issue tracking & tracker tooling

| Ticket | Scope | Bucket |
|---|---|---|
| 426 + 475 — text-first `find` output; "PNG card" wording | S + S | **Next** (moved from Now: 582 reworked `find` just now; do this on top of it) |
| 340 — label/project/category filters in `find issues` | S/M — this pass again read ~190 open tickets by hand | **Next** |
| 283 / 246 / 279 / 241 | as before (241 after 487) | **Next / Later** |
| 601 — `/wrap` session-close skill | a `wrap` skill is installed in `~/.claude/skills`; see §9 | **Close candidate** |

## 5. Agent instructions & practice docs

| Ticket | Scope | Bucket |
|---|---|---|
| 567 — move harnez-only rules into `.harnez/rules/`, tool-neutral AGENTS.md | M/L — P1, new. Supersedes 494's layout half; 351–353, 015, 413 become rule files instead of AGENTS.md blocks. Needs 355 (prune) so the old blocks can be removed | **Next** (high; after 355) |
| 568 — split IssueTracking, AgenticLoop, GoRelease into generic docs + harnez rules | M — follows 567 | **Next** (after 567) |
| 471 (+505, 323, 324) — root doc copies drift from sources | S/M — still P2 and still hit by every `harnez init` here; do before 568 so the split starts from reconciled sources | **Next** (high) |
| 353 — Claude-only instruction-file rule | S — P1 per 544; M1 committed (746418b). Finish and close | **Now** (finish) |
| 545 — lean-sprint guardrails (history preflight, CPU check, `agent start --dry-run`, host-only close) | S/M — P2, new; each item came from a real incident in the 2026-09-24 retro | **Next** |
| 546 — `harnez init` toggles the `read -I` recommendation | S/M — P2, new; 543 bounded the OOM, so re-enabling is now a per-project choice | **Next** |
| 176 (+465, 502, 510) → 499 → 145 | report contract → orchestration assessment → orchestrator skill | **Next** |
| 221, 151, 128, 134 | as before | **Next** |
| 473, 472, 053 | research | **Later** |

Rationale: 567 is new and P1 and changes where every later instruction ticket lands, so the
instruction tickets that were queued behind 494 now queue behind 567.

## 6. Config & doc management (`apply` / `init` core)

| Ticket | Scope | Bucket |
|---|---|---|
| 355 — `apply`/`init` don't prune removed AGENTS.md sections | S/M — **raised to the head of this section**: 567 moves blocks out of AGENTS.md and cannot leave orphans | **Next** (high; before 567) |
| 005 — permissions are grow-only | M | **Next** |
| 474, 016, 224, 092, 369/370/371 | S–M each | **Next** |
| 413 / 015 | now 567 rule-file candidates | **Next** (after 567) |
| 230, 170, 009, 013 | as before | **Later** |

## 7. Testing, canary & build hygiene

| Ticket | Scope | Bucket |
|---|---|---|
| 555 — Quota-1 summary shows the real error when there are no FAIL lines | S — P2, new. Compile/vet errors currently need a log read; same class as 511, which shipped | **Now** |
| 531 — a run that stops at gofmt/vet should not use up the single run | S — P3 but it caused an untested commit in 530 M1 | **Now** (with 555) |
| 503 — release publishes a stale versioned source archive | S/M — correctness bug on public releases | **Next** (high) |
| 513 — real-terminal run before a CLI feature is done | S/M — 593's `.ansi` mockups cover design; this covers the live check | **Next** |
| 566 / 569 — bench: isolated agent config; live progress table | S/M each — 569 (P2) first | **Next** / **Later** |
| 010, 007 | as before | **Next** |
| 177 | | **Later** |
| 073 | user decision | **Park** (§9) |

Rationale: 555 and 531 are this pass's cheap Quota-1 noise removers, exactly like 509/511 last
pass: both make an agent's single run fail or get wasted for reasons unrelated to its change.

## 8. Local-LLM support

| Ticket | Scope | Bucket |
|---|---|---|
| 231 — local-compact doc profile | | **Next** |
| 165 / 167 | tracking-only | **Later** |

## 9. Close, park, move, or split

This roadmap is read-only against `issues/`; the tracker actions below are recommendations for a
separate pass.

**Close — work is done or the premise is gone:**

- **508** — new this pass: 603 made `agent models` and `agent start` honour provider quota
  exhaustion. **Tracker action:** verify 508's acceptance criteria against 603 and close.
- **601** — new this pass: a `wrap` skill ("close the current work session, preserve its state")
  is installed. **Tracker action:** confirm it came from 601 and close, or record what is missing.
- **544** — the 2026-09-24 handoff (P1). Of its nine threads, 543 and 540 closed; 353, 539, 538,
  542, 534/535 are tickets in their own right (placed above). **Tracker action:** close with a
  pointer to those tickets once the "roll out managed-block changes" item is confirmed done.
- **302**, **342**, **152** — unchanged from last pass (study exists; MVP is 417 + 479; collector
  question now belongs to 560 M3).

**Rescope:**

- **494** — layout half superseded by 567; keep only `init` component selection.
- **476** — async half delivered by 572 over MCP; keep CLI parity, first-call tip, `inline`.
- **515** — reduce to what 598 did not cover.
- **288** (fold into 145), **435** (A/B telemetry only), **034** (into 495), **449** (merge with
  485) — unchanged.

**Park:**

- **581** — new here. Everything but the "Case 1" warning notice shipped (602, 585) and three
  repos confirmed it; the notice was not asked for by any reporter. Keep open for the last
  runtime confirmation, do not schedule the notice.
- **606** — new, P3, unconfirmed: `agent status` said `completed` while a resume started with
  shell `&` was still running. Shell `&` is now forbidden in AgenticLoop, so the trigger is gone;
  reopen if seen with a native background shell.
- **450** — blocked on loom's terminal input handling.
- **073** — user decision on credential mounting.
- ~~035~~ — closed (metering proxy delivered).

**Move to `../loom` or re-sort after 560:** the view and box set in §2 (255, 256, 085, 295,
160, 161, 051, 146, 404, 219, 214, 084, 172, 141), the mic set (250, 251, 253, 264, 278, 247,
330) and platform probes in `internal/usage` (339, 334, `watch.go` half of 286). The collector
set (111, 113, 535) likely stays as 560 M3 work instead. **Tracker action:** tag views `move:
loom`, relate collector tickets to 560.

**Split (unchanged):** **224** — the Android scaffold is **230**.

## 10. Agent execution & planning hardening

| Ticket | Scope | Bucket |
|---|---|---|
| 466 — detect and break Claude Stop-hook loops | M — 605 added the `/goal` exit-clause rule, which removes the most common trigger; detection is still useful | **Next** |
| 274 — session-start harness health checks | M — now also a natural home for 539's "does this repo know `harnez agent`" check | **Next** |
| 281, 285, 293, 297, 298, 300, 451 + 453 | as before | **Next** |

## 11. macOS & OS-agnostic core

| Ticket | Scope | Bucket |
|---|---|---|
| 338 — macOS CI via the GitHub mirror | also the place to test 560's lock/socket choice | **Next** |
| 286, 335, 336, 337 | as before | **Next** |
| 310 / 313 | | **Later** |

## 12. Multimodal & visual context

| Ticket | Scope | Bucket |
|---|---|---|
| 444, 447, 459, 441, 436, 440 — Dot8 chain | on hold; 543 moved Dot8 behind a build tag | **Hold** |
| 427 → 460 — ANSI colours in stdin render; 256/truecolor SGR | S → S/M | **Next** |
| 403 — hook interception and distill adapter for multi-slice `harnez read` | M — 543 M2 added range reads; build on that | **Next** |
| 402 — move config diff below status, free `harnez diff` | M — after 426 | **Next** |

## 13. Agent instructions & tooling (newer)

| Ticket | Scope | Bucket |
|---|---|---|
| 351 / 352 | S each — now `.harnez/rules/` candidates (567) | **Next** (after 567) |
| 294, 322, 365, 382 | as before | **Next** |
| 366 | | **Later** |

## 14. Misc (later)

304, 305 (after 487), 314 / 321 / 329, 317, 333, 361, 378, 384, 385 — unchanged, **Later**.

## Suggested order of attack

Refreshed this pass. Dispatch features largely shipped; the lead is now **reliability and
discoverability of what shipped**, then the two P1 architecture items (560 data contract, 567
instruction layout).

1. **Finish in-progress (Now):** 594 (Codex compaction), 556 (status-line context), 353
   (Claude-only rule).
2. **Discoverability (Now):** verify 539 in another repo after `harnez init`, land 557 with it.
3. **Quota-1 noise (Now):** 555 + 531.
4. **Telemetry home (Now):** 515, reduced to what 598 left.
5. **Usage contract decision (Now):** answer 560 §7, then M2 → M3 (absorbing 535) → 564/534 →
   M4; 607 after M4.
6. **Instruction layout:** 471 (+505, 323, 324) → 355 → 567 → 568; 351/352/413/015 as rule files.
7. **Dispatch follow-ons:** 595 → 477 + 587 → 476 (+463) → 538 → 547/548/549 → 526 → 449 + 485
   → 306 → 144, 383.
8. **Component follow-ons:** 493; 495 after the 560 schema decision; 492; 494 (selection only).
9. **Cost and attribution:** 487 + 461 → 446 → 445; 516, 518 measurements; 501, 512.
10. **Report and orchestration contract:** 176 (+465, 502, 510) → 499 → 145; 545 guardrails.
11. **Distill and efficiency:** 468 → 178; 124 + 296; 225 → 215.
12. **Execution hardening:** 466, 274, then 281, 285, 293, 297, 298, 300, 451 + 453.
13. **Tracker ergonomics:** 426 + 475, 340, 283, 246.
14. **macOS core:** 338 → 335 → 286 → 336/337.
15. **Tracker pass (no code):** close 508, 601, 544, 302, 342, 152 after checks; rescope 494,
    476, 515; tag the §9 view set for loom and relate collector tickets to 560.
