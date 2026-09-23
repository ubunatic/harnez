# Roadmap

Working roadmap for the open backlog (updated 2026-09-24, reconciled against b828dff). Derived
from each ticket's appended `## Implementation Plan` or its `/goal` and specification sections,
so scope calls here reflect the planning pass, not a fresh re-derivation.

**Value axis.** harnez is the single source of truth for everything a coding agent reads:
`config.yaml` drives settings, hooks, instructions, skills and doc copies across Claude Code,
Codex, AGY and Prime Agent, and `apply` must stay idempotent and never touch user-managed
keys (README). On top of that core sit three surfaces used every day: `harnez agent` dispatch
(cheap developer agents driven by an orchestrator), the issue tracker, and the telemetry that
measures whether the harness actually saves tokens and turns. Work that makes those surfaces
more correct, more composable and cheaper per session outranks work that adds new surface.

**What changed the axis this pass — two decisions from the component-system work (489, 490):**

1. **harnez becomes composable.** `docs/HarnezComponents.md` §8 designs a component selection
   (`full`, `docs-only`, …) so a user can take harnez's docs without its hooks, telemetry or
   dispatcher. That widens who harnez is useful for, and it forces a boundary between
   *capture* and *storage* of token data (495) that reshapes the cost-telemetry chain in §3.
2. **The `usage` TUI is leaving harnez for a `../loom` app** (`HarnezComponents.md` §4 item 3,
   §8.2). The previous passes' guiding bias ("the primary daily surface is `harnez usage
   --watch` in a terminal split") is therefore retired. The watch-dashboard backlog in §2/§2b
   stops competing for harnez's **Now** slots and becomes a move-to-loom set (§9). The contract
   harnez keeps is the `harnez usage --json` snapshot schema, which agent dispatch still needs
   for quota-aware model choice (449/485).

**What changed the axis this pass — model economics became measurable.** The 2026-09-24
model research (`docs/ModelResearch.md`, issue 514) replaced list-price guesses with COST as
*plan quota per turn* in `spec/agent.yaml` (luna = 1, astra = 100), measured from our own
quota history for ChatGPT Plus and Claude Pro. That makes "which model for which role" a
data question, and exposed the data gaps that now gate it: telemetry scattered across decoy
stores (515), no per-turn quota snapshots (507), and unmeasured agy and gpt-6-sol rows
(516, 518). These form the new §1b-q cluster and take one **Now** slot ahead of further
dispatch UX work.

**Strategic goal kept — OS-agnostic readiness (macOS first).** Still valid for the harnez core
(hooks, shims, `exec`, CI). Most of the Linux-specific probes it named (`pactl`, `amixer`,
`/proc/*`, `ps -eo comm=`) live in `internal/usage` and move with loom, so §1/§11 shrink to the
core half.

Sequencing buckets:

- **Now** — actionable today, no blockers, high daily-loop value, or a prerequisite for other
  work.
- **Next** — actionable but larger, lower daily value, or waiting on a Now item.
- **Later** — real but speculative, large, or dependent on decisions not yet made.
- **Close / Park / Move** — should not be scheduled here; see §9.

---

## 0. Shipped Recently

**Closed since the 67f6c3a pass (2026-09-22 → 2026-09-24):**

- **514** (repeatable model research plan): `docs/ModelResearch.md` with family researchers,
  a claim ledger, local quota analytics (step 2b) and `docs/ModelTrials` for what web
  research can't settle. Follow-up code: the `harnez agent models` table with roles, COST,
  EFF and Go/TUI/SQL skill columns, then COST re-based on measured plan quota (663e619).
  Its dry-run findings filed **510**, **512**, **515–518**.
- **506** (exec agent-timeout exemption missed `bash -c`): widened to a general `HTO`/
  `HARNEZ_TIMEOUT` timeout-intent prefix with hook forwarding.
- **504** (onboard GPT-6 Codex models) and **500** (agy driver live-tested on
  flash37/flash38/sonnet/opus).
- Untracked but relevant: `harnez usage --loom` integration (PR #2, first step of the loom
  move in §2), distill/readcard ANSI fast paths (PRs #5, #6), `:med` tier specs listed.
- New tickets placed this pass: **501–503**, **505**, **507–513**, **515–518**.

**Closed on 2026-09-22 (lean sprints, after this pass):**

- **496** (gofmt cleanup) and **488** (hermetic Quota-1 state): the 30 files formatted, a
  `gofmt -l` gate in `make check`, and a `.git` dir counts as a repo only with `HEAD`.
- **289** (go.work fixture `go.mod`): already fixed in d602096; closed after a stale-ticket
  check.
- **497** (`harnez agent` model tiers): the Codex tier reaches `exec` and resume, and aliases
  moved to `spec/agent.yaml` with `terra` and the current Claude aliases.
- **491** (component selection in `apply`): `--components`, `components:`/`requires:`, one
  settings write, removal driven by `requires:`, Codex/AGY by `telemetry`, a CLI e2e test,
  and the MVP deleted. §1's keystone is done; 492, 493 and 495 are unblocked.
- New: **498** (Claude sessions can't resume inside Claude Code, P1) and **499** (bake the
  model-aware orchestration approach into skills). Model data: `docs/ModelAdvisoryEval.md`,
  practice: `docs/practices/ModelRoles.md`.

**Closed since the e61f74c pass (this pass):**

- **489** — component separation analysis: coupling, use-case coverage, boundaries and
  independent deployability in `docs/HarnezComponents.md` §1–7. Concluded that coupling is a
  `cmd/` and runtime-contract problem, not an `internal/` refactor, and that `usage` leaves for
  `../loom`.
- **490** — component system design (`HarnezComponents.md` §8, paths A–E, recommendation A
  now with B/C/D triggers) and an MVP in `internal/components` that wraps `apply` from the
  outside. Integration split to **491**, persistence to **492**; the design's decisions filed
  **493** (dispatch mode), **494** (init selection) and **495** (shared token capture).
- **Unified agent CLI epic 479** with children **480–484** and **478**, **486**: one flag set
  (`--name`/`--model`/`-d`, prompt files, `--` tail), a runnable root form with slash
  commands, truthful resume state and a RESUME column, attributable bare `resume`/`-c`,
  `default_model` and bare aliases in `spec/agent.yaml`, and enforced roles
  (orchestrator/developer/reviewer/advisor). Follow-ups split out as **476**, **477**, **485**.
- **464**, **470** — Codex resume runs with the same sandbox bypass as start (1c3f49a); the
  first orchestrated sprint (`docs/OrchestratedAgentFlow.md`) ran end to end on it.
- **467**, **469** — plan-first initial prompt guidance and the evergreen trigger rule, in the
  sprint and AgenticLoop docs.
- **268** — bounded `harnez exec` timeouts with process-group kills, repo precedence and
  agent-turn exemptions. Was **Next** in §10.
- **156** — Codex `spawn_agent` Agents-view delegation documented. Was **Next** in §5.
- **135** — closed as obsolete: since 415 the Tool Feedback Protocol is delivered only by its
  skill, so there is nothing to split. Was **Next** in §5.
- **291** — closed as absorbed by 481/484. It was a §9 close candidate; the tracker now agrees.
- **462** — telemetry review leftovers delivered (`warn_condition` dropped, tip skip
  documented, test renamed). Was **Later** in §3.

**Earlier passes (kept for reference):**

- **457** — canonical telemetry analytics queries and `harnez stats --quality`; live run all
  PASS. Its finding (no `model` column on `tool_calls`) is **461**.
- **127** (remainder **458**), **341** (fixed on Linux, macOS unverified; reopen if 338's CI
  shows lost rows), **424**, **425** (remainder **459**), **428** (remainder **460**).
- **454**, **315**, **442**, **374**, **149**, **417**, **030** (AGY half deferred to 034),
  **114**, **429–434**, **166**, **399**, **443**, **448**, **452**, **455**, **456**.
- **006, 018, 030, 042, 045, 046, 056, 070, 071, 072, 095 pt.1, 096, 105, 108, 123, 125, 126,
  139, 201, 209, 210, 216, 217, 222, 229, 249, 261, 262, 263, 290, 292, 299, 301, 303**.

## 1. Component system (`apply` composability) — new section, leads this pass

The design is done and an MVP exists; what remains is putting it where users can reach it.
491 is the keystone: 492, 493's clamp, 495's peer detection and (through 495) 446 all consume
its resolved selection.

| Ticket | Scope | Bucket |
|---|---|---|
| 491 — integrate component selection into `apply` | M — `components:`/`requires:` keys, `apply --components`, one settings write instead of a second removal pass, selection-aware `diff`/`status`, telemetry schema gated on `telemetry`, Codex/AGY apply-or-remove. Retarget the MVP tests at `claude.ApplyAll*`, then delete `internal/components` | **Now** |
| 493 — `mixed` subagent dispatch mode, dispatch-mode-aware sprint skills | M — `mixed` in `agentpolicy` (native for the host's vendor, `harnez agent` for others), interception honours it, sprint skills stop hard-coding `harnez agent start`. The `agents`-disabled ⇒ `native` clamp needs 491 | **Next** (parallel with 495, after 491) |
| 495 — shared token-capture package with stable API and `spec/` schemas | M — capture/parse per provider, a versioned `Record` in `spec/`, no storage; peer detection needs 491. 446 becomes its first consumer | **Next** (parallel with 493, after 491) |
| 492 — persist selection in `~/.config/harnez/local.yaml` | S/M — move the local-config loader out of `internal/usage` into a neutral package, add `components:` and `apply --save`. Do it before the loom move lands, because the loader currently lives in the package that is leaving | **Next** (after 491) |
| 494 — project-level selection in `init` with `*.harnez.md` / `*.local.md` | M/L — repo-type commit policy, move harnez-specific managed blocks out of AGENTS.md. Independent of 491 (init is not apply, see `docs/CLIDesign.md`), but it rewrites managed blocks, so it should follow 355 (pruning) and absorb the layout question in 413 | **Next** (independent; after 355) |

Rationale: this is the only cluster that changes *who* harnez is useful for, and its design
decisions are fresh. 491 goes first because four tickets read its output; shipping any of
them against the MVP wrapper would mean doing the settings-write and `diff`/`status` work
twice. 493 and 495 have no dependency on each other and can run in parallel once 491 lands.
492 is P3 but time-boxed by the loom move. 494 is independent of `apply` and can start any
time, but it is the larger migration and is best done once 355 has settled how managed blocks
are removed.

## 1a. Cross-agent dispatch (`harnez agent`)

The 479 epic shipped this pass, and the first orchestrated sprint (luna:med orchestrator,
luna:low developers, three tickets) ran on it. Its findings reorder this section: blocking
`harnez agent` calls worked for 2–6 minutes without the host ever polling, every failure was
review depth rather than model capability, and the open gaps are async mode, attribution and
report contracts.

| Ticket | Scope | Bucket |
|---|---|---|
| 476 — explicit `--sync`/`--async`, immediate start feedback, `--plan yes\|no\|inline` | M — `--plan yes\|no` and `-d` shipped in 479; the remainder is async mode, the first-call tip and `inline` planning. Prerequisite for 477: there is no async path to notify from yet | **Next** |
| 477 — hook-driven background task completion instead of polling | M/L — P1 in the tracker, **placed Next behind 476**: the sprint showed synchronous calls do not poll, so the hazard this fixes only appears once 476 adds async mode. Design both together | **Next** (after 476) |
| 449 — spec-driven chat model selection and aliases | **partially delivered by 484** (`default_model`, bare aliases). The remainder is provider-only/tier-only resolution and quota-aware eligibility, which is the same concern as 485. **Moved Now → Next** and merged with 485 in intent; both read quota through the `usage --json` contract that survives the loom move | **Next** |
| 485 — autodetect the default model from 5h and weekly usage | S/M — spec-driven preference list plus demotion thresholds; explicit `--model` always wins. Land as one change with 449's quota half | **Next** (with 449) |
| 306 — quarantine Codex sessions unusable after usage limits | S/M — P1; 482 left the quarantine hook point in bare `resume`/`-c` attribution, so this is now a small, well-placed change | **Next** |
| 463 — `--escalated --reason` on start/resume | S — P2; same flag surface as 476, land together. The ticket file carries a stray `Status: Draft / Reserved placeholder` trailer that a tracker pass should remove | **Next** (with 476) |
| 144 — Codex subagent model-selection policy | S — the sprint recorded that luna:low handled docs, code and config tickets; the fallback default now lives in `spec/agent.yaml`. The per-task-type choice remains | **Next** |
| 383 — disable queued question-tool prompts in Codex sessions | S — dispatch papercut | **Next** |
| 435 — native-subagent interception + A/B telemetry | **rescope again**: 493 now defines interception semantics (`mixed` passes same-vendor requests back to native). What stays unique is the A/B telemetry, which needs 487's role attribution and 495/446's cost records | **Later** (after 493, 487, 495) |
| 450 — hosted chat input line after terminal resize | P1, blocked on `../loom` | **Park** (§9) |
| 288 / 342 — external-agent handoff skill, cross-agent MVP | delivered by 417 + 479; see §9 | **Close / rescope** |

Rationale: ordered by what a wrong dispatch costs. 306 and 449/485 decide *where* work goes;
476 → 477 decide *how the host waits*. 449 leaves **Now** because its core promise (explicit
selection wins, a spec default exists) shipped in 484, and its remaining half is a duplicate of
485. 477 keeps its P1 label in the tracker, but its trigger (async workers) does not exist
until 476, so it cannot be verified before then.

## 1b-q. Quota & model economics — new section this pass

The model table now drives role choice in every sprint skill, so a wrong COST row directly
wastes plan quota. The chain is *find the data → record it per turn → measure the gaps →
act on it*.

| Ticket | Scope | Bucket |
|---|---|---|
| 515 — one discoverable home for all telemetry data | M — P1. One root with per-component stores, decoys (`telemetry.sqlite`, `~/.local/share/harnez/telemetry.db`) removed or migrated, `usage history timeline` reading `quota-history.jsonl`. Absorbs 487's "one canonical DB path" item; 495's storage side should target this layout | **Now** |
| 517 — check `claude:` effort support | S — the table says EFFORT "no" for every Claude row while Anthropic documents low/medium/high; a spec/driver check with a live canary | **Now** (cheap, fixes a wrong row) |
| 507 — quota snapshots at `agent start`/`resume` | M — before/after readings per turn via the shared cache (087) and collector, best-effort ≤ 2s. Replaces grepping `quota-history.jsonl` by hand | **Next** (high; after 515 fixes where it writes) |
| 508 — reject agent calls on confirmed-exhausted quota | S/M — reads the same snapshot; the pre-flight twin of 306's quarantine | **Next** (after 507, with 306) |
| 516 / 518 — measure agy (Google Pro) and gpt-6-sol plan-quota COST | S each — measurement, not code; needs 507's per-turn deltas to be cheap | **Next** (after 507) |
| 501 — `agent models` marks interactive-only providers | S — stops hosts dispatching batch work to chat-only models | **Next** |
| 512 — check docs list only aliases defined in `spec/agent.yaml` | S — a test gate; the drift it prevents already happened once | **Next** |
| 510 — web-only researcher role | S — the advisor role misfit web research in the 514 sweep; belongs in the role rules next to 176 | **Next** (with 176) |

Rationale: 515 leads because every later measurement (507, 516, 518, 485's demotion
thresholds) reads telemetry, and the 514 run showed agents concluding "no data" from decoy
stores. 485 and 449 in §1a now consume 507's snapshots instead of the `usage --json` poll
alone, so 507 moves ahead of them.

## 2. Usage watch TUI — moving to `../loom`

The watch dashboard, its collector and the mic/hardware boxes are leaving harnez
(`HarnezComponents.md` §4 item 3). Scheduling them here would build features into a package
that is about to be extracted. They are listed in §9 as **Move to loom**, with the one
exception that still matters to harnez:

| Ticket | Scope | Bucket |
|---|---|---|
| 143 — Git status in all agent status bars | M — `internal/statusline` is installed by `apply` and may stay in harnez even if the usage TUI leaves; decide its owner as part of the loom split before building | **Later** (owner decision first) |
| 152 — move `agent-collector` under `usage` | moot: the collector moves with loom, and its systemd unit becomes loom's (the first manifest candidate in §8.2 path D) | **Close** (§9) |
| 255, 256, 085, 295, 264, 250, 251, 253, 278, 404, 219, 214, 146, 247, 051, 160, 161, 330, 172, 141 | watch/collector/mic/splash/status features | **Move to loom** (§9) |

Rationale: 255 was the last **Now** item here last pass. It moves out not because it lost
value but because its value now accrues to loom. The previous "never show a wrong number"
chain (210 → 201 → 139 → 105 → 262) shipped, so the dashboard leaves in a trustworthy state.

## 2b. Collector pipeline & token sources

| Ticket | Scope | Bucket |
|---|---|---|
| 034 — hook-triggered token extraction (AGY, Claude) | **rescope into 495**: the capture half is exactly what 495's shared package inventories; the display half goes with loom. Keep the AGY-protobuf research, drop the watch-specific plumbing | **Next** (as a 495 consumer) |
| 111 — per-agent collector cadence/timeout/cancellation | collector internals | **Move to loom** (§9) |
| 113 — per-collector roundtrip times, `usage --meta` | collector internals | **Move to loom** (§9) |
| 084 — aggregate quota-window box | blocked (no capacity field) and a watch box | **Move to loom** (§9) |
| 035 — transparent HTTPS proxy sidecar | only rate-limit headers remain unique | **Park** (§9) |

## 3. Telemetry data layer & analytics

The hardening chain (341 → 127 → 457) landed in earlier passes and `harnez stats --quality`
guards the store. This pass changes the *extend* half: 490's design separates capture from
storage (495), and the orchestrated sprint showed that the store cannot say which agent role
issued which call (487).

| Ticket | Scope | Bucket |
|---|---|---|
| 446 — persist cost fields reported by agents | S/M — **moved Now → Next (after 495)**: the ticket now records that capture moves to the shared package, agents store cost in their own session records, and telemetry receives it only when active. Building it before 495 would couple agents to sqlite, which the component design forbids | **Next** (after 495) |
| 445 — counterfactual API rate cards in `spec/` | M — still after 446 (measured before modelled) | **Next** (after 446) |
| 487 — record agent role and parent; attribute nested `harnez` subcommands | M — `agent_role`/`parent_session_id` columns, the real subcommand behind `exec`, `stats --role`. The canonical-DB-path item moves to **515** (§1b-q), which generalises it to every store. **New and Next-high**: every orchestrated-sprint question ("which commands did each agent run?") is unanswerable without it | **Next** |
| 461 — `model` column on `tool_calls` | S/M — same migration shape as 487; do them as one attribution change | **Next** (with 487) |
| 124 + 296 — PostToolUse tool-call capture + always compute distill savings | M + S/M — the two halves of one efficiency number; 124 is canary-gated | **Next** |
| 468 — compact `make test-q1` output via `harnez distill` | S/M — new, P2. Quota-1 allows one run, and hosts truncate the output and miss late failures. Grows distill with a Go-test mode, so it belongs with 178 and 296 | **Next** |
| 178 — distill smart mode, error-pattern preservation | M — pairs with 468 (same distill growth); keep `internal/distill` free of `internal/telemetry` | **Next** (after 468) |
| 225 → 215 — classifier baseline, then LLM backfill | S → M — baseline before backfill, unchanged | **Next** |
| 458 — remaining telemetry SQL into `spec/` | S/M — opportunistic, alongside 487/461 which touch the same migrations | **Later** |
| 421 — telemetry for harnez commands and feature usage | M — partly subsumed by 487's subcommand attribution; re-read after 487 lands and reduce to what is left | **Later** |
| 208 — SQLite format for `harnez usage export` | S/M — the export builders live in `internal/telemetry`, but the command is under `usage`; decide which binary owns `export` in the loom split | **Later** |

Rationale: 446 was **Now** and now waits on 495, which the component design made a
prerequisite; the reason is architectural, not a priority drop. 487 enters high because the
first real multi-agent run exposed an attribution gap the quality checks cannot see: rows are
present, only unattributable. 468 is the cheapest way to finally put distill to use and
removes a real Quota-1 failure mode.

## 4. Issue tracking & tracker tooling

| Ticket | Scope | Bucket |
|---|---|---|
| 279 — persistent `issues/README.md` lock sidecar in working trees | S — **moved Now → Later**: the model advisors recommended cutting it (design-heavy, low value) and no new friction was reported | **Later** |
| 426 + 475 — text-first `find` output; "PNG card" wording | S + S — one pass over `find` output; 475 is new and a string change | **Now** |
| 340 — label/project/category filters in `harnez find issues` | S/M — this pass again read the full open list (≈170 tickets) for want of it | **Next** |
| 283 — reassess mandatory fresh-subagent issue filing | S — policy decision | **Next** |
| 246 — `/commit` and `/publish` skills | M | **Next** |
| 241 — session-tree tool-call efficiency audit | M — needs 487's attribution to mean anything | **Later** (after 487) |

## 5. Agent instructions & practice docs

| Ticket | Scope | Bucket |
|---|---|---|
| 471 — root doc copies drift from copyable sources (Bash, Make, IssueTracking, Spec, GoRelease) | S/M — new, P2. Every `harnez init` in this repo rewrites ~1000 lines of root docs, so any agent following the repo rules has to revert them by hand. Per-hunk reconcile with the source-wins rule; find GoRelease's source first | **Next** (high) |
| 176 — capped subagent completion-report contract | S — the sprint supplied the content: 8–12-line replies with named fields worked; the cap belongs in the role rules in `spec/agent.yaml`, with a mandatory exact-test-result field | **Next** |
| 499 — assess baking the model-aware orchestration approach (model eval, role assignment by capability and cost, quota watching, strong host) into skills | S/M — new. Decide per technique: skill, doc, code (485) or drop. Feeds 145 | **Next** (before 145) |
| 145 — orchestrator-session skill/command | M — the sprint recorded exactly what it must encode (role start command, preflight, two follow-up turns max, review checklist, helper cleanup). Its last prerequisite is 176. Absorbs the remaining scope of 288 | **Next** (after 176) |
| 465 — adopt loom's lean-sprint field notes into AgenticLoop | S — decision ticket; do it with 176/145 so the practice docs change once | **Next** (with 176) |
| 502 — AGENTS.md documentation and constant-uniqueness guidance | S — new, P2; the missing-evergreen-doc gap it names recurs with low-tier developers | **Next** (with 176) |
| 505 — copyable-doc reference rule undiscoverable in CLIDesign | S — new, P3; fold into 471's reconcile pass | **Next** (with 471) |
| 221 — Go-first for scripts, demote ad-hoc Python | S | **Next** |
| 151 — on-demand lookup vs. materialized instructions (research) | S/M — also informs 494's split into AGENTS.md vs `*.harnez.md` | **Next** |
| 128 — full system-prompt self-audit for repetition | research, 2 of 4 ACs met | **Next** |
| 134 — ConciseMode trigger Skill | S | **Next** |
| 473 — study MiniMax CLI harness efficiency | M research, P2 | **Later** |
| 472 — Dream-RSI replay and exploration practices | research, P3; needs 487's attributable history to replay | **Later** |
| 053 — stale LSP diagnostics detect/toggle | research first | **Later** |

Rationale: 471 leads because it is a live defect in the doc pipeline that is harnez's core
promise, and 494 will change the same `init` paths. 176 and 145 moved from "waiting on a
convention" to "content known": the orchestrated sprint is their acceptance evidence.

## 6. Config & doc management (`apply` / `init` core)

| Ticket | Scope | Bucket |
|---|---|---|
| 289 — `init` go.work reconciliation hard-fails on fixture `go.mod` files | S — hard failure in any repo with Go fixtures | **Now** |
| 355 — `apply`/`init` don't prune removed AGENTS.md sections | S/M — **raised in importance**: 491 (component removal) and 494 (moving blocks to `AGENTS.harnez.md`) both need a correct prune | **Next** (high; before 494) |
| 005 — permissions are grow-only | M — needs a managed-permission state sidecar. 491's "nil value = remove if harnez-owned" in `applyMerge` is the same ownership idea; design them together | **Next** (with/after 491) |
| 474 — `init --docs man` alias for `manpages` | S — new; small, test-covered alias | **Next** |
| 413 — manage local context-link policy through `init` | M — re-read against 494's file layout before building; it may become one of 494's topic files | **Next** (after 494 decides layout) |
| 015 — teach AGENTS.md about `uman` | S — under 494 this is a `*.harnez.md`/`*.local.md` candidate, not an AGENTS.md block | **Next** |
| 016 — `make smoke` convention in Make.md | S | **Next** |
| 224 — website rules auto-install | S; split the Android scaffold to 230 | **Next** |
| 092 — latest-release links / README install | S — linter probe only | **Next** |
| 369 / 370 / 371 — Go init profile: shape detection, agent capabilities, systemd guidance | S–M each | **Next** |
| 230 — Android direct-release scaffold | M | **Later** |
| 170 — modularize `cmd/harnez/main.go` | M — `HarnezComponents.md` §7 names `cmd/` as the real coupling point; revisit once 491 shows which command files need to split | **Later** |
| 009 — `diff`/`clean` for Makefile targets | belongs on `init --dry-run` | **Later** |
| 013 — `promote` command | L | **Later** |

## 7. Testing, canary & build hygiene

| Ticket | Scope | Bucket |
|---|---|---|
| 496 — clean up gofmt drift across the repo | S — new. One mechanical `gofmt -w` commit plus a `gofmt -l` gate in `make check`. **Now, and before 491**: 491 edits `internal/claude` and `cmd/harnez`, both on the drift list, and every review has to filter formatting noise by hand | **Now** |
| 488 — Quota-1 state trusts any ancestor `.git`; tests not hermetic | S — new, P3 but high leverage: a stray `/tmp/.git` broke two tests for every developer on the machine and likely explains the unexplained exec-hook failures workers reported | **Now** |
| 509 — session tip leaks into `cmd/harnez` tests run inside an agent session | S — new, P2. Makes `make test-q1` fail only for agents, which burns the single Quota-1 run | **Now** |
| 511 — `make test-q1` keeps the full log and prints its path on failure | S — new, P2. Same failure class as 468 (truncated output wastes the one run); land first, 468 compacts on top | **Now** |
| 513 — real-terminal (TTY) run before a CLI feature is done | S/M — practice plus a helper; catches display-width and pager bugs tests can't | **Next** |
| 503 — release publishes a stale versioned source archive | S/M — new, P2, cause unknown; wrong artifacts on a public release are a correctness bug | **Next** (high) |
| 010 — smoke-test that agents see installed skills/commands | unblocked | **Next** |
| 007 — thin test coverage, `stripComments` `/* */` gap | M | **Next** |
| 177 — lean post-edit build check | M | **Later** |
| 073 — credentialed cloud-agent canary | blocked on a user decision | **Park** (§9) |

Rationale: 509 and 511 are this pass's cheap noise removers, like 496/488 last pass: both
make agents' single Quota-1 run fail for reasons unrelated to the change. 496 and 488 were both small and both remove noise from every sprint that follows;
cheap work that de-risks the Now keystone (491) goes first.

## 8. Local-LLM support

| Ticket | Scope | Bucket |
|---|---|---|
| 231 — local-compact doc profile | unblocked since 149; still pays off only once dispatch routes to local models | **Next** |
| 165 — two-phase architect/patch harness | tracking-only | **Later** |
| 167 — local runtime targets & telemetry | register runtimes as agent IDs; no `--agent local` | **Later** |

## 9. Close, park, move, or split

This roadmap is read-only against `issues/`; the tracker actions below are recommendations for a
separate pass.

**Close — work is done or the premise is gone:**

- **291** — ✅ closed by the tracker this pass (absorbed by 481/484). Removed from this list.
- **302** — the comparison study exists at
  `docs/studies/2026-09-10-agent-harness-plugin-systems-and-self-modification.md`. Still open
  after three passes. **Tracker action:** close as delivered, study in `Related`.
- **342** — the MVP it describes is 417 plus the 479 epic. **Tracker action:** close as
  superseded, or reduce it to one AGY-host dispatch canary if that half is unverified.
- **152** — moving `agent-collector` under `usage` is moot once the collector moves to loom.
  **Tracker action:** close as superseded by the loom move, or transfer to loom.

**Rescope — the premise moved under the ticket:**

- **288** — the CLI half is done (417, 479), and 145 now carries the handoff contract with
  evidence from the orchestrated sprint. **Tracker action:** fold into 145 and close, or rename
  around the remaining skill contract only.
- **435** — interception semantics move to 493's `mixed` mode. **Tracker action:** keep open,
  reduce to A/B telemetry, depend on 493, 487 and 495.
- **034** — the capture half is 495's inventory; the display half goes to loom. **Tracker
  action:** relate to 495 and drop the watch-specific plumbing.
- **449** — the default-model and alias half shipped in 484; the quota half duplicates 485.
  **Tracker action:** mark 484 as delivering part of it and merge or cross-link with 485.

**Move to `../loom` — the usage TUI leaves harnez (`HarnezComponents.md` §4 item 3):**

- Watch/collector: **255** (partial: retries and a basic `l` overlay shipped), **256**,
  **085**, **111**, **113**, **160**, **161**, **051**, **146**, **295** (splash).
- Boxes and presentation: **404**, **219**, **214**, **084**, **172**.
- Mic and audio: **250** (research done, canaries pending), **251**, **253**, **264**,
  **278**, **247**, **330**.
- Platform probes that live in `internal/usage`: **339** (hardware/mic gating on macOS),
  **334** (`ps -eo comm=` in `internal/usage/process.go`), and the `watch.go` half of **286**.
- **141** (Codex status-bar agent count) — blocked upstream either way; moves if the status
  line goes with loom, see 143 in §2.

**Tracker action for the move set:** file them in loom (or tag them `move: loom`) when the
extraction starts, then close here with a pointer. Until then they stay open but unscheduled.
Note the loom-side dependencies harnez must keep: 492 extracts the local-config loader first,
and the `usage --json` snapshot schema stays the contract for 449/485.

**Park — blocked on something outside this repo:**

- **450** — P1, blocked on `../loom`'s resize/raw-mode handling. With the usage TUI moving
  there too, loom is now the natural owner of terminal input for both.
- **073** — needs a user decision on credential-mounting posture.
- **035** — reduce to a rate-limit-header canary.
- **231** is not parked (§8); **165**/**167** stay tracking-only on their own terms.

**Split (unchanged):** **224** — the Android scaffold is **230**.

## 10. Agent execution & planning hardening

| Ticket | Scope | Bucket |
|---|---|---|
| ~~268~~ — bounded `harnez exec` timeout | ✅ closed this pass; see §0 | **Done** |
| 466 — detect and break Claude Stop-hook loops | M — new, P2. A `/goal` judge loop burned 10–20 full-transcript turns in a loom session; harnez already owns the hook layer where detection belongs | **Next** |
| 274 — session-start harness health checks | M — should report the component selection once 491 lands | **Next** (after 491) |
| 281 — opt-in advisor discovery | S | **Next** |
| 285 — durable-note wording contract | S | **Next** |
| 293 — recoverable roadmap synthesis | M | **Next** |
| 297 — linked language subdocuments | M | **Next** |
| 298 — commit checkpoint/file-granularity guidance | S | **Next** |
| 300 — raw-mode and PTY input guidance | S — pairs with 286's `x/term` convention | **Next** |
| 451 + 453 — low-cost roadmap-skill default; `opus:low` vs `astra:low` guidance | S + S — one cost-discipline docs batch | **Next** |

## 11. macOS & OS-agnostic core

Reduced to the harnez-core half; the usage probes move to loom (§9).

| Ticket | Scope | Bucket |
|---|---|---|
| 286 — `golang.org/x/term` convention in `docs/lang/Go.md` | S — **moved Now → Next**: the `watch.go` `stty` refactor that made it urgent is loom's now; the doc convention is still useful for `harnez agent chat` and any core TUI | **Next** |
| 338 — macOS CI via the GitHub mirror | push/PR trigger and CLI smoke test remain; also the only way to verify 341 on macOS | **Next** |
| 335 — `afplay` audio notifications in default hooks | S | **Next** |
| 336 — cross-platform shell shim patterns | research | **Next** |
| 337 — macOS permissions and CLI whitelist for the config template | research | **Next** |
| 310 / 313 — ambient `go.work` checks in status/lint; canary `GOWORK` probe | S each | **Later** |

## 12. Multimodal & visual context

`harnez read -I` / `find -I` / `issues show -I` cards stay in harnez (`readcard` is core), so
this section is unaffected by the loom move.

| Ticket | Scope | Bucket |
|---|---|---|
| 444 — Dot8 renderer cell pitch for mixed glyphs | S — P1 and the root of the Dot8 chain | **Hold** |
| 447 — `B`/`#` as matrix colour symbols | S — land with 444 | **Hold** |
| 459 — Braille glyph cell margins | S — with 444 | **Hold** |
| 427 → 460 — preserve ANSI colours in stdin render; 256/truecolor SGR | S → S/M | **Next** |
| 441 — Dot8 PNG card reader | M — after 444 | **Hold** |
| 436 — `--dot8` dense cards for codex and agy | M — after 444 | **Hold** |
| 403 — hook interception and distill adapter for multi-slice `harnez read` | M | **Next** |
| 402 — move config diff below status, free `harnez diff` | M — after 426 | **Next** |
| 440 — card header in PNG metadata | S/M — try the inline-note channel first | **Hold** |

The Dot8 chain (444, 447, 459, 441, 436, 440) is on hold; see issue 444 for why and the
condition to resume.

## 13. Agent instructions & tooling (newer)

| Ticket | Scope | Bucket |
|---|---|---|
| 351 / 352 / 353 — AGENTS.md pointers: spec before hardcoding; parallel-session untracked files; Claude-only CLAUDE.md rule | S each — under 494, these are candidates for `AGENTS.harnez.md` rather than AGENTS.md | **Next** |
| 273 — opt-in old AGY PreToolUse hook | S | **Next** |
| 294 — cross-agent post-edit success hooks for `harnez rate` | S research | **Next** |
| 322 — evolve `/story` skill | S/M | **Next** |
| 323 / 324 — dangling doc references in copyable docs (Bash.md, Go.md) | S each — fix with 471's reconcile pass | **Next** (with 471) |
| 365 — `harnez release --init` | M | **Next** |
| 366 — automated doc compression with canary evaluation | M | **Later** |
| 382 — user shell shortcuts for all agents | M | **Next** |

## 14. Misc (later)

| Ticket | Scope | Bucket |
|---|---|---|
| 304 — advisor lifecycle CLI | M | **Later** |
| 305 — `stats --auto` failure rates vs observed outcomes | research; revisit after 487 | **Later** |
| 314 / 321 / 329 — release: auto-create minisign key, `--login`, REUSE gate | S–M each | **Later** |
| 317 — split `harnez-advisor` guidance into resources/ | S | **Later** |
| 333 — retire `docs/feedback/` into `docs/studies/` with labels | S/M | **Later** |
| 361 — `harnez docs variant --check` | S | **Later** |
| 378 — fleet-wide git history sparks for uman | M | **Later** |
| 384 — `harnez assess -d` | S | **Later** |
| 385 — `tokens` command | S — could reuse 495's parsers | **Later** |

## Suggested order of attack

Refreshed this pass. The component design landed and the usage TUI is leaving, so the lead
moves from telemetry extension to **making `apply` composable**, with telemetry extension
re-sequenced behind the shared capture package it now depends on.

1. **Clear the ground (small, Now):** ~~496, 488, 289~~ done 2026-09-22. **498** (Claude
   resume inside Claude Code), **509** and **511** (Quota-1 runs wasted by tip leak and
   truncated logs), **517** (wrong Claude effort rows). 279 moved to Later.
1b. **Model economics:** **515** (one telemetry home) → 507 (per-turn quota snapshots) →
   508, 516, 518; 501 and 512 alongside. 507 feeds 485 in step 6.
2. **Component keystone:** ~~491~~ done 2026-09-22 — selection in `apply`, one settings write, selection-aware
   `diff`/`status`.
3. **In parallel after 491:** 493 (`mixed` dispatch mode, mode-aware sprint skills) and 495
   (shared token capture). 492 alongside, before the loom extraction starts.
4. **Cost numbers on the new substrate:** 446 as 495's first consumer → 445 rate cards.
   487 + 461 attribution as one migration in the same window.
5. **Independent track:** 355 (prune) → 494 (init selection, `*.harnez.md`/`*.local.md`),
   with 413, 015, 351–353 re-read against the new file layout; 471 (+323/324) reconciles the
   root doc copies first.
6. **Dispatch follow-ons:** 306 (+508) → 449 + 485 (on 507's snapshots) (quota-aware default) → 476 (+463) → 477 → 144,
   383. 435 last, reduced to A/B telemetry.
7. **Report and orchestration contract:** 176 (+465) → 145 (absorbing 288).
8. **Visual context:** 444 + 447 + 459 → 426 + 475 → 427 → 460 → 441 → 436.
9. **Distill and efficiency:** 468 → 178; 124 + 296; 225 → 215.
10. **Execution hardening:** 466, then 274 (after 491), 281, 285, 293, 297, 298, 300, and the
    451 + 453 docs batch.
11. **Tracker ergonomics:** 340 → 283 → 246.
12. **macOS core:** 338 → 335 → 286 (doc) → 336/337.
13. **Tracker pass (no code):** close 302, 342, 152; fold 288 into 145; rescope 435, 034,
    449; tag the §9 move set for loom.
14. Revisit **Later** items once 491 has shown where `cmd/` needs to split (170) and once the
    loom extraction has settled what stays behind (143, 208).
