# Roadmap

Working roadmap for the open backlog (updated 2026-09-21). Derived from each ticket's
appended `## Implementation Plan`, so scope calls here reflect the planning pass, not a fresh
re-derivation.

**Strategic Goal — OS-Agnostic & Cross-Platform Readiness (macOS first)**:
harnez is preparing to become fully OS-agnostic, targeting macOS (Darwin) as the primary non-Linux platform. This requires eliminating hardcoded GNU/Linux CLI tool dependencies (`stty`, `pactl`, `amixer`, `/proc/*`) in favor of standard Go cross-platform libraries (such as `golang.org/x/term`), OS build-tag splits for system telemetry, and platform audio/process abstractions.

**Guiding bias for ordering**: harnez's primary daily surface is a single workstation with
`harnez usage --watch` open in a terminal split, plus the issue tracker and the instruction
docs that every agent session loads. Work that makes that daily loop more correct, more
legible, and OS-agnostic outranks work that adds new capability surface.

**New this pass — telemetry data management leads.** The dispatch-correctness cluster that led the
previous pass has largely landed (454, 315, 442, 374, 424 all closed; see §0), and the stated focus
is now **better, cleaner, more robust telemetry data management**. §3 therefore moves to the front
of the suggested order of attack: the store is the substrate every cost, efficiency and A/B number
in this backlog is read from, and 424 demonstrated that a schema defect can sit in it unnoticed
until somebody happens to look. The ordering inside §3 was deliberately *reproduce-first*: 341
(concurrent writers losing rows), then 127 (SQL into `spec/`), then 457 (queries as tests plus a
live data-quality check). **That chain has landed**: 457 is closed, 127 is closed (remainder: 458),
and 341 is closed (fixed on Linux, macOS unverified). What is left in §3 is the *extend* half (446, 445, 124+296,
225, 215).

**Previous lead, now mostly delivered.** `harnez agent`
(`start`/`resume`/`chat`/`stop`/`list`/`models`/`enable`/`disable`) is shipped and in use, and 454's
explicit-selection rule is enforced. §1a's remaining P1 (450) is blocked on external work in
`../loom`, so the cluster no longer heads the order.

Sequencing buckets:

- **Now** — actionable today, no blockers, high daily-loop value or already in flight.
- **Next** — actionable but either larger, lower daily value, or waiting on a Now item.
- **Later** — real but speculative, large, or dependent on decisions not yet made.
- **Close / Park** — should not be scheduled; see §9.

---


## 0. Shipped Recently

**Closed since the a600641 pass (latest shipped set):**

- **457** — canonical telemetry analytics queries and live data-quality checks: spec-defined checks
  with fixture tests (9bedd31) and `harnez stats --quality` (`--json`, `--strict`, 19e0895). The live
  run is all PASS. **Model gap recorded in the ticket:** `tool_calls` has no `model` column, so
  per-model call analytics are impossible; noted in §3 as a model-attribution item next to 446/445
  (no ticket filed).
- **424** — confirmed closed earlier (see below).

**Closed, remainders split into follow-up tickets:**

- **127** — closed: all schema, insert and query SQL lives in `spec/telemetry.yaml`. The rest
  (`telemetry.go` migration SQL, `classify.go`, `sanitize_cache.go`, `issuesnapshot.go`,
  `export.go`, `economics_query.go`) is now **458**.
- **341** — closed: fixed and regression-tested on Linux (fbaa511, `BEGIN IMMEDIATE`
  serialization). macOS is not tested soon; reopen if 338's CI shows lost rows.

**Closed since the 2026-09-21 pass:**

- **454** — explicit `luna` selection is enforced; no silent host-provider fallback. The
  cost-substitution bug that led §1a last pass is fixed, which is why that section no longer
  heads the order of attack.
- **315** — `init` no longer drops previously opted-in docs on re-run. The destructive half of
  the `apply`/`init` family is closed; 289 and 355 remain.
- **442** — `harnez issues open --commit` now commits a newly created ticket, so the documented
  `/issue` workflow ends in a commit instead of leaving the tracker dirty.
- **374** — README CLI coverage and the website link are refreshed against the current command
  surface, including the `harnez agent` tree.
- **424** — telemetry compaction events insert with the model column; fresh, legacy and
  current-version DBs all self-heal, with regression tests (a1ade4a). This was the **Now** item
  at the head of §3 and it is the reason §3's remaining work is now about *keeping* the store
  honest rather than repairing it.

**Closed, remainders split into follow-up tickets:**

- **425** — closed: migration logging and schema-version drift landed with 235d7da. The Braille
  glyph-spacing half is now **459** (§12, with 444); review leftovers are **462**.
- **428** — closed: the telemetry migration test coverage landed in the same commit. The ANSI
  256-color / 24-bit truecolor half is now **460** (§12, behind 427).

- **030, 071, 096, 105, 108, 126, 139, 149, 201, 209, 210, 217, 261, 262, 290, 292, 299** — shipped/closed
- **006** — `fix(status): check all managed settings keys`
- **018** — bundled marker backfill + guard test
- **042**, **045**, **046**, **056**, **125**, **222**, **095 pt.1** — AgenticLoop & practice docs improvements
- **229** — `/issue` skill
- **249** — portable copyable-doc contract
- **070**, **072** — local-agent canary and cross-agent distill research/verification
- **216** — local SLM telemetry classifier endpoint
- **301** — cross-agent Docup testing skill
- **303** — prose-first reusable advisor skill with four-target distribution

**Closed since the 2026-09-17 pass:**

- **429, 430, 431, 432, 433** — the pixel-font cluster behind `harnez read -I` visual context
  cards: glyph/unicode mappings moved into `internal/readcard/spec/` YAML, the `%` glyph fixed,
  golden visual assets generated, dead profiles dropped, and official upstream BDFs (Tom Thumb
  3x5, Spleen 6x12/8x16, X11 misc-fixed 7x13) adopted for every non-default size with only the
  5x8 font staying hand-tuned. Remaining upstream gaps are documented in
  `third_party/fonts/README.md`; the follow-through is 434 (see §12).
- **166** — both parts are now closed, not just Part A: the Go rune/display-width invariants
  landed in `docs/lang/Go.md`, and the compact local-LLM doc profile no longer tracks here
  (see §8).
- **399** — `harnez read` image enhancements (configurable line-number cadence, AST-safe
  whitespace compression); removed from §12.
- **071**, **123**, **201** — previously listed in §9 as "close now"; the tracker now records
  them closed, so they are no longer close candidates.

**Closed since the 2026-09-19 pass:**

- **149** — agent-specific profiles + the Codex async-wait instruction. This was named the
  "keystone" of §5 in the last two passes; the profile mechanism and its Codex content are
  shipped, reviewed, and committed. Everything that was written as **Next (after 149)** —
  144, 151, 231 — is now unblocked, and 145 no longer has to wait for a convention that does
  not exist yet.
- **417** — `harnez subagent` MVP, delivered as the `harnez agent` command tree. It carries
  `start`, `resume`, `chat`, `stop`, `delete`, `compact`, `status`, `list`, `models`, and the
  `enable`/`disable` pair that sets `subagent_mode`. This closure supersedes most of 291 and 342
  and shrinks 435 (see §9).
- **030** — Codex rollout token aggregation is wired. The AGY protobuf half is explicitly
  deferred into 034, so 084's gate is now "034", not "030".
- **114** — remote-load stream stdin-EOF false trigger; recorded Resolved.
- **434** — closed, as the previous pass predicted; the glyph-spec-per-font-size work and the
  `scripts/import-bdf-font.go` removal are both done.
- **443**, **448**, **452**, **455**, **456** — filed and closed inside this cycle: `issues list`
  aliases and completion, the known-agent-models listing, `agent stop --all`/`delete --all` with
  lineage-safe behaviour, agent-name shell completion with prompt descriptions, and the
  synchronous agent-lifecycle documentation.

## 1. OS-Agnostic Readiness (macOS first) & Terminal Modernization

Laying the foundation to run seamlessly across operating systems (macOS / Darwin at first), eliminating brittle Linux-only subprocess forks in the interactive TUI and establishing portable system abstraction layers.
| Ticket | Scope | Bucket |
|---|---|---|
| 286 — promote `golang.org/x/term` for terminal operations in Go conventions & watch.go | S — add `x/term` carve-out to `docs/lang/Go.md`, replace `stty` subprocesses & raw-mode ioctls in `internal/usage/watch.go` with `x/term` | **Now** |
| 339 — gate hardware telemetry and mic probes on macOS | M — owns the OS build-tag split, Darwin/fallback collectors, graceful TUI degradation, and acceptance checks | **Next** |
| 334 — research OS-agnostic process inspection | S/M — owns the `ps -eo comm=` replacement research and the follow-up implementation-ticket decision | **Next** |

Rationale: 286 is immediate, high-leverage low-hanging fruit — it updates the Go convention docs, eliminates the `stty` dependency from `watch.go`, and cuts subprocess CPU overhead in one clean step. It directly unlocks running the watch TUI reliably on macOS and non-GNU environments without requiring coreutils `stty`.

## 1a. Cross-agent dispatch & hosted agent chat (`harnez agent`) — new section

`harnez agent` shipped with 417 and is now how work is handed to a cheaper or different model.
**454 closed this pass**, so the highest-cost failure mode here — silently substituting the
expensive host model for the low-cost tier the user asked for — is fixed. What remains is one
blocked P1 and a set of follow-ons, which is why this cluster no longer leads the order of attack.

| Ticket | Scope | Bucket |
|---|---|---|
| 450 — hosted chat input line breaks after terminal resize | S/M — P1, **blocked on `../loom`**: the resize/raw-mode handling this needs lives in that external dependency, so no amount of work here closes it. Revisit when loom lands; pair it with 286's `x/term` work at that point | **Park** (blocked) |
| 449 — spec-driven chat model selection and aliases | M — P1; provider-only/tier-only/no-arg forms resolved through an editable spec, with quota-aware selection (skip agents ≥80% of their 5h window). **Unblocked by 454**: the "explicit selection wins" rule is now enforced, so defaults can be layered on top of it safely. Shares the spec surface 445 touches | **Now** |
| 451 — roadmap skill defaults to low-cost models when unspecified | S — docs/skill-only; the same cost-discipline defect as 454, one layer up in the skill rather than the CLI | **Next** |
| 453 — clarify `opus:low` vs `astra:low` cost guidance in sprint docs | S — docs-only; land together with 451 as one cost-discipline pass | **Next** |
| 435 — native-subagent interception + A/B telemetry switch | **rescoped by 417**: the `subagent_mode` switch now exists as `harnez agent enable`/`disable`. What remains is the interception/redirection hooks for native `invoke_subagent`/`spawn_agent` and the comparative telemetry. The hardening chain (341 → 127 → 457) has largely landed, so the telemetry half is now unblocked once 341's macOS CI confirmation arrives and 446/445 supply cost numbers | **Next** |
| 306 — quarantine Codex subagents unusable after usage limits | S/M — P1; a dispatcher that keeps routing to a dead session wastes a whole turn per attempt. Follows 449, which introduces the eligibility notion this would extend | **Next** |
| 383 — disable queued question-tool prompts in Codex sessions | S — moved up from §14; it is a dispatch-surface papercut, not misc | **Next** |
| 144 — Codex subagent model-selection policy | S — **unblocked by 149**; its content now migrates into a real profile instead of waiting for one | **Next** |
| 288 / 291 / 342 — external-agent handoff, standardized dispatch, cross-agent MVP | largely delivered by 417; see §9 for the close/rescope call | **Close / rescope** |

Rationale: this cluster is ordered by *what a wrong answer costs the user*. With 454 closed, 449 is
the remaining piece of "send work to the model the user actually asked for" and stays **Now**. 450
is the interactive half of the same surface but is externally blocked, so it moves to **Park**
rather than sitting in **Now** as an item nobody can start. 435's A/B telemetry is still held
behind §3's cost work (446/445); 457's live checks now guard the
store, so the earlier reason for holding it is largely gone.

383 and 144 remain behind 451/453 and 306 deliberately: 451/453 are a small documentation batch
that fixes current cost guidance, while 306 prevents repeated dispatch to a known-dead session.
383 is a lower-impact prompt-queue papercut, and 144 is policy work whose empirical model-selection
check can follow the live dispatch correctness and cost-default fixes.

## 2. Usage watch TUI — correctness & legibility

The `--watch` dashboard is the tool's front door. Everything that makes it lie, misalign, or
hide a failed collector belongs at the front of the queue.
| Ticket | Scope | Bucket |
|---|---|---|
| 255 — collector resilience: retries & TUI logs | retry, structured details, and basic scrolling shipped; full key decoding/tests remain | **Now** |
| 085 — show collector-daemon status in watch | S — no plan written yet; small sibling of 105, land with it | **Next** |
| 264 — ALSA/arecord live-mic-level backend | M — follows 262 for amixer systems | **Next** |
| 250 — research desktop mic indicators | S, in progress — solves cross-desktop privacy UX | **Next** |
| 251 — suppress desktop mic indicators | M — depends on 250 | **Next** |
| 253 — mic view triggers desktop privacy indicator | S/M | **Next** |
| 256 — persist watch & collector launch logs | M — follows 255 | **Next** |
| 172 — AGY single-window row alignment | rendering bug fixed; remainder needs a live capped account | **Park** (§9) |
| 278 — mic box "recording n/a" wording + real recording-active probe | S/M — the box currently states something it cannot know; wording fix first, probe investigation second | **Next** |
| 404 — HDD/SSD storage and I/O metrics in the watch TUI | M — new metric family; land after the existing boxes are trustworthy | **Later** |
| 219 — subtler usage-bar colors vs Braille charts | S — spec/colors.yaml ramp split | **Next** |
| 214 — 256-color heat palette option | M — third value for two existing presentation enums | **Next** |
| 143 — Git status in all agent status bars | M — shared collector + per-agent wiring | **Next** |
| 141 — running-agent count in Codex status bar | blocked: Codex status line is a closed item picker, not a command hook | **Later** |
| 146 — recent-subagent-activity watch box | L — needs a per-tool feasibility matrix first | **Later** |
| 247 — third mic graph (amplitude-over-time audiogram) | M/L — visual addition, evaluate after 262 | **Later** |
| 051 — multi-host monitoring + host navigation | L — introduces an "active host" concept the watch state has never had | **Later** |
| 160 — extract watch layout/UI into renderer-agnostic module | feasibility done; only the user's proceed/stage call remains | **Later** |
| 161 — collector absorbs remote-load host + Prometheus | M, but only pays off after 160/051 direction is set | **Later** |

Rationale: 210 → 201 → 139 → 105 → 262 all shipped, so the "never show a wrong or silently stale
number" chain is complete except for 255, which finishes the opaque-error half and is the one
remaining **Now** item here. 105 was the one that stopped the recurring class of incident
(086, 103/104) where a dead collector was only caught by hand-digging on disk. 278 is next
because the mic box currently asserts a recording state it cannot actually observe — the same
category of defect the chain above just closed. Cosmetics (219, 214), new metric families (404),
and the status-bar work follow once the numbers are trustworthy.

## 2b. Collector pipeline & token sources
| Ticket | Scope | Bucket |
|---|---|---|
| 034 — hook-triggered token extraction | plan revised: hook infra now exists (`agy-hooks`, `codex-hook`), scope shrank | **Next** |
| 113 — per-collector roundtrip times, `usage --meta` | S/M — extend existing `internal/usage/fetchdurations.go`, do not build a second timing store | **Next** |
| 111 — per-agent cadence/timeout/cancellation | premise corrected: collection is already concurrent; reduced to cadence + timeout + cancel | **Next** |
| 035 — transparent HTTPS proxy sidecar | superseded in most of its value; only rate-limit headers remain unique | **Park** (§9) |
| 084 — aggregate quota-window box | blocked: `QuotaWindow` has no capacity field, and AGY token extraction remains in 034 | **Park** (after 034; §9) |

Rationale: doing the Codex half of 030 first is nearly free and unblocks the token column for a
second agent. 113 before 111 — you want the measurements before tuning the cadence they'd inform.

## 3. Telemetry data layer & analytics

The stated focus is better, cleaner, more robust telemetry data management. The order was
**reproduce-first, then consolidate, then keep honest, then extend**. The first three steps have
landed (341 fixed on Linux, 127 M1/M2, 457 closed), so the section now moves to the *extend* half,
with 457's `stats --quality` as the guard that flags regressions.

| Ticket | Scope | Bucket |
|---|---|---|
| ~~341~~ — concurrent SQLite telemetry writers lose rows (macOS) | ✅ **closed**: fixed and tested on Linux (fbaa511); macOS unverified, reopen if 338 CI shows lost rows | **Done** |
| ~~127~~ — move `internal/telemetry` SQL into `spec/` | ✅ **closed**: schema/insert/query SQL is in `spec/telemetry.yaml`. Remainder is **458**; finish opportunistically alongside 446/445, which touch the same spec surface | **Done** (458 **Later**) |
| ~~457~~ — canonical analytics queries as tests + live data-quality checks | ✅ **closed**: spec-defined checks, fixture tests, `harnez stats --quality`; live run all PASS. See §0 | **Done** |
| 461 — `model` column on `tool_calls` | S/M — finding from 457: per-model call analytics are impossible today. Sequence with 446/445, which need the same attribution | **Next** (with 446/445) |
| 462 — telemetry leftovers (`warn_condition`, test name, apply tip skip) | S — review cleanups from the sprints | **Later** |
| 446 — persist cost fields reported by agents | S/M — capture `cost`/`total_cost`/`currency` where a provider already reports them rather than discarding them. **Before 445**: a measured number is worth more than a modelled one, and it gives 445's estimates something to be checked against | **Now** |
| 445 — counterfactual API rate cards in `spec/`, surfaced in `usage`/`stats` | M — rate cards per model in `spec/`, then estimated pay-as-you-go spend and cache savings. Lands on top of 446's measured values and 127's spec surface | **Now** (after 446) |
| 124 — PostToolUse auto-capture of tool-call counts | M — canary-gated: run the payload probe before writing code. Moved up from §7; it is a telemetry *ingestion* ticket, and pairing it with 296 makes the efficiency numbers complete at the same time | **Next** |
| 296 — always compute distill savings for stats | S/M — pairs with 124: together they close the "efficiency reporting is only partly populated" gap, and 457's data-quality checks will flag exactly these columns as sparse until they do | **Next** |
| 225 — local SLM classifier reliability/accuracy | S — cache versioning first, then an accuracy baseline. **Before 215**: a backfill run against an unversioned cache cannot be evaluated, so the baseline has to exist before rows are rewritten at scale | **Next** |
| 215 — LLM backfill/reclassification of tool notes | M — new `activity_category` column + migration + CLI. Follows 225's baseline, and follows 457 so the backfill's effect is measurable as a data-quality delta rather than an assertion | **Next** |
| 178 — distill smart mode, error-pattern preservation | M — key constraint: do not import `internal/telemetry` from `internal/distill` | **Next** |
| 421 — extend telemetry to harnez commands and feature-usage analytics | M — new event family. Deliberately **later**: adding an event family before 341/127/457 means the new family inherits the same unverified write path and has no canonical queries covering it | **Later** |
| 208 — SQLite export format | S/M — depends on 204's scrubbed record slices, and exporting a store is only worth doing once its contents are known-good | **Later** |
| 459 / 460 (from 425 / 428) | telemetry halves shipped in 235d7da; the Braille-spacing (459) and 256/truecolor (460) follow-ups track in §12 | **→ §12** |

Rationale: the hardening chain did its job. 457 now makes data quality a reported property
(`harnez stats --quality`, all PASS live), 127's SQL is single-homed in `spec/telemetry.yaml`, and
341 is closed (fixed on Linux, macOS unverified). Its own finding is the next gap: `tool_calls` has no model
column, so per-model call analytics are impossible; that belongs with 446/445 as one
model-attribution theme. Everything that *adds* to the store (446/445 cost, 124/296 efficiency, 215 classification, 421 command analytics)
queues behind that chain, in each case pairing a measurement with the thing that validates it.
446 before 445 (measured before modelled), 225 before 215 (baseline before backfill), 124 with 296
(both halves of one efficiency number).

## 4. Issue tracking & tracker tooling
| Ticket | Scope | Bucket |
|---|---|---|
| 279 — persistent `issues/README.md` lock sidecar left in working trees | S — a stray file in every tree is visible friction in `git status` on a tracker used many times a day | **Now** |
| 283 — reassess mandatory fresh-subagent issue filing vs. direct host filing | S — a policy decision, not code; the current rule costs a full subagent spawn per ticket | **Next** |
| 241 — session-tree tool-call efficiency audit command and skill | M — needs the telemetry of §3 to be trustworthy before its numbers mean anything | **Later** |
| 246 — add /commit and /publish Skills | M — multi-project staged commit ownership | **Next** |
| 340 — label/project/category query filters in `harnez find issues` | S/M — moved up from §14; roadmap synthesis and triage both re-scan the whole backlog today | **Next** |
| ~~442~~ — `issues open --commit` does not commit a newly created ticket | ✅ **closed** this pass; the `/issue` workflow now ends in a commit. See §0 | **Done** |
| 426 — make `find` output text-first for human users | S — tracked in §12; listed here because the tracker CLI is its heaviest consumer | **Now** (see §12) |

Rationale: 108 and 217 are shipped (§0). Of the remaining work, 279 and 283 are cheap and remove
friction from the filing loop itself, while 246 extends the skill set. 340 moves up from **Later**
because every planning pass (including this one) currently re-reads the full open list for want of
a category filter. 442 shipped this pass, so the filing workflow now ends in a commit; 279 is the
last remaining tracker-hygiene defect and keeps its **Now** slot.

## 5. Agent instructions & practice docs

Two sub-clusters: a batch of small `AgenticLoop.md` edits, and a larger question about how
instructions are delivered at all.

**Docs batch — New** (Previous batch shipped: 042, 045, 046, 056, 125, 222, 095 part 1):

- 263 — `docs/practices/PrototypingFeatures.md`: ✅ shipped; tracker closed

→ **Now.** High per-session value.
| Ticket | Scope | Bucket |
|---|---|---|
| 221 — Go-first for scripts, demote ad-hoc Python | S — do *not* add a new `docs/practices/Languages.md` (130's precedent) | **Next** |
| 176 — capped subagent completion-report contract | S — new `docs/practices/SubagentReporting.md` + skill refs | **Next** |
| 156 — document `collaboration.spawn_agent` in Codex Agents | S, docs-only | **Next** |
| 144 — Codex subagent model selection policy | **unblocked — 149 shipped**; its content migrates into the profile mechanism that now exists. Tracked in §1a, where its cost-discipline siblings live | **Next** (→ §1a) |
| 151 — on-demand lookup vs. materialized instructions (research) | **unblocked — 149 shipped**; item 4's dependency on 149's design is satisfied, so the research can now be written against a real mechanism instead of a hypothetical one | **Next** |
| 128 — full system-prompt self-audit for repetition | research; 2 of 4 ACs met, needs the with-project-docs pass | **Next** |
| 135 — Tool Feedback Protocol delivered twice | S — measure with `harnez stats --overhead` first, differentiate only if warranted | **Next** |
| 134 — ConciseMode trigger Skill | S — the `mode` skill already exists; only its *description* needs to become trigger-shaped | **Next** |
| 145 — orchestrator-session skill/command | M — 149 and 417 have now settled two of the three conventions it would encode; 176's report contract is the last one outstanding, so this moves **Later → Next (after 176)** | **Next** (after 176) |
| 053 — stale LSP diagnostics detect/toggle | research first (is the toggle even exposed?); disagreement-detector explicitly rejected — harnez has no observation point | **Later** |

Rationale: **149 shipped, so this section's keystone is gone and its dependents are released.**
144 moves to §1a (it is model-selection policy, which is now a live cost concern, not a docs
concern). 151 can be written against the profile mechanism as built. 145 moves up a bucket because
two of its three prerequisites now exist — only 176's report contract remains, so it is sequenced
directly behind it rather than parked indefinitely. 128 and 135 stay the measurement side of the
same problem and are now the cheapest way to check whether 149's profiles actually reduced the
per-session instruction load they were built to reduce.

## 6. Config & doc management (`apply` / `init` core)
| Ticket | Scope | Bucket |
|---|---|---|
| 289 — `init` go.work reconciliation hard-fails on testdata/fixture `go.mod` files | S — a hard failure that blocks `init` in any repo with Go fixtures; bug, not polish | **Now** |
| 005 — permissions are grow-only | M — needs a managed-permission state sidecar so user/Claude-Code additions survive | **Next** |
| 355 — `apply`/`init` don't prune AGENTS.md sections removed from `config.yaml` | S/M — moved up from §13; orphaned managed blocks break the "single source of truth" promise the README makes | **Next** |
| 413 — manage local context-link policy through `init` | M — extends the managed-section mechanism; do after 355 settles pruning semantics | **Next** |
| 230 — Android direct-release scaffold (Makefile template, signing, checksums) | M — the split-out half of 224; independent of the website-rules work | **Later** |
| 152 — move `agent-collector` under `usage` | S, **but** the systemd unit hardcodes `ExecStart … agent-collector`; needs an alias + migration, not a rename | **Next** |
| 015 — teach AGENTS.md about `uman` | S — mirror the `repo_modes` opt-in mechanism, not a global section | **Next** |
| 016 — `make smoke` convention in Make.md | S, docs + template | **Next** |
| 224 — website rules auto-install | S — `Website.md` is *already* copyable; make it self-install for website-capable projects. **Split the Android release scaffold into its own ticket.** | **Next** |
| 092 — latest-release links / README install | S — Option B (linter probe) only; explicitly reject the managed-README-block option | **Next** |
| 170 — modularize `cmd/harnez/main.go` | M refactor — every command closes over two shared vars | **Later** |
| 009 — `diff`/`clean` don't cover Makefile targets | scope corrected: belongs on `init --dry-run`, not on the global-only `DiffAll`/`CleanAll` | **Later** |
| 013 — `promote` command | L, new command with an agent-invocation surface | **Later** |

Rationale: With 006 and 018 shipped, 209 moves up to fix a missing piece in `apply` (agy-hooks). 005 is
the real one but needs the state sidecar designed carefully so `apply` never deletes a
permission the user approved interactively.

## 7. Testing, canary & agent visibility
| Ticket | Scope | Bucket |
|---|---|---|
| 010 — smoke-test that agents see installed skills/commands | **unblocked** — wayreel#11 landed, `verifyContains` exists | **Next** |
| 007 — thin test coverage | M — one test file per package; `stripComments` genuinely lacks `/* */` support, which silently yields an empty map | **Next** |
| 177 — lean post-edit build check for control-flow edits | M — hook option chosen; prototype the heuristic against real past edits first | **Later** |
| 124 — PostToolUse auto-capture of tool-call counts | moved to §3 — it is telemetry ingestion, and it pairs with 296 there | **Next** (→ §3) |
| 073 — credentialed cloud-agent canary | blocked on a user decision about credential-mounting posture | **Park** (§9) |

Rationale: 007's `/* */` gap is the sharp edge — a hand-written JSONC config with block comments
currently parses to nothing and `apply` treats that as "nothing applied." Worth doing even if the
rest of 007's table-test sweep waits.

## 8. Local-LLM support

Coherent cluster, all P3, all gated on §5's profile mechanism.
| Ticket | Scope | Bucket |
|---|---|---|
| 231 — local-compact doc profile for small local models | **unblocked — 149 shipped**; the profile mechanism it was waiting for exists, so this is now ordinary work rather than tracking-only. Still behind §1a and §3, because a doc profile for local models pays off only once dispatch actually routes to them | **Next** |
| 165 — two-phase architect/patch harness | tracking-only by the ticket's own instruction; decide "profile or workflow?" on paper first | **Later** |
| 167 — local runtime targets & telemetry (Ollama, llama.cpp, vLLM) | design skeleton; register runtimes as ordinary agent IDs, **do not ship `--agent local`** | **Later** |

Rationale (revised): 149 shipped, so this cluster is no longer fully blocked — 231 is released and
is the one actionable item here. The previous pass's reasoning ("building a local-model doc profile
before 149 defines the profile mechanism means building it twice") has been satisfied rather than
overturned: the mechanism now exists, so the profile can be written once. 165 and 167 remain
tracking-only on their own terms, not on 149's.

## 9. Close, park, or split

**Still open in the tracker as of this pass.** 291, 342, 288, 435 and 302 were all named as
close/rescope candidates last pass and all five are still `Open`. They remain the cheapest items in
the backlog — five tickets leave it without writing a line of code — and 302 in particular only
needs its delivered study verified against its acceptance criteria before closing. This roadmap is
read-only against `issues/`, so the tracker actions below are recommendations for a separate pass.

**Close now — work is done or the premise is disproven:**

- **071**, **123**, **201** — ✅ resolved since the last pass; the tracker records all three as
  closed. Moved to §0.
- **302** — the requested comparison study now exists at `docs/studies/2026-09-10-agent-harness-plugin-systems-and-self-modification.md`. **Tracker action:** set `Status` to `Closed — research study delivered`; retain the study in `Related` as the acceptance artifact.
- **291** — "add `harnez agent`: standardized non-interactive dispatch to external coding-agent
  CLIs". `harnez agent start`/`resume`/`chat` exist and ship today; this ticket asks for the
  command that 417 delivered. **Close as superseded**, after confirming the acceptance criteria
  against the live `harnez agent --help` surface. **Tracker action:** set `Status` to `Closed — superseded by 417`; record any genuinely missing acceptance criterion in a new,
  narrow ticket rather than keeping a delivered feature request open.
- **342** — "MVP: `harnez agent` command and cross-agent dispatch from agy/claude host to
  codex/claude subagents". Same finding: the MVP it describes is the thing 417 shipped.
  **Close as superseded**, or — if the agy-host half is genuinely unverified — reduce it to a
  single verification canary and say so in the ticket. **Tracker action:** either set `Status` to
  `Closed — superseded by 417`, or replace the goal and acceptance criteria with that one AGY-host
  canary while keeping it `Open`. Do not leave it open as a feature request
  for a feature that exists.

**Rescope — the premise moved under the ticket:**

- **288** — "`/harnez-agent` skill: lean fresh-handoff sprint dispatching to an external agent
  CLI". The CLI half is done (417) and the skill surface has since grown `harnez-advisor`,
  `reverse-sprinter` and the documented synchronous lifecycle (456). What is left is narrower than
  the ticket describes: a handoff *contract*, not a dispatcher. Rewrite the scope before
  scheduling it, or it will be implemented twice. **Tracker action:** keep `Status: Open`, rename
  the ticket around the handoff contract, remove the stale dependency on 291, and replace the CLI
  acceptance criteria with the remaining skill/contract checks.
- **435** — the `subagent_mode: harnez|native` switch it asks for now exists as
  `harnez agent enable`/`disable`. Remaining real scope: native-tool interception/redirection
  hooks, and A/B telemetry. **Tracker action:** keep `Status: Open`, remove the delivered switch
  from the goal/acceptance criteria, and retain only interception/redirection plus A/B telemetry.

**Park — blocked on something no amount of work here resolves:**

- **450** — P1, but blocked on external work in `../loom`, which owns the resize/raw-mode handling
  the fix needs. Moved out of §1a's **Now** this pass so the bucket reflects what can actually be
  started. **Tracker action:** set the status reason to name the `../loom` dependency, and revisit
  alongside 286's `x/term` work once loom lands.
- **073** — needs a user decision on credential-mounting posture. Cheap pre-work: confirm that
  AGY/Codex have no pre-exec rewrite hook, which would collapse this to Claude-Code-only and make
  the decision much easier. **Tracker action:** keep it `Blocked` and name that user decision in
  the status reason.
- **172** — the rendering half is fixed; the remainder needs a live 100%-capped AGY account (or a
  captured fixture) to verify against. **Tracker action:** keep it `Open — deferred` and replace
  the completed rendering acceptance criteria with the capped-account verification only.
- **035** — reduce to a scoped canary rather than building it. Token counts no longer need a
  proxy (Claude aggregates; Codex now writes plain-JSON rollouts). The only unique remaining
  capability is rate-limit response headers — a narrow payoff for a MITM CA plus TLS trust
  injection into three runtimes. **Tracker action:** keep it `Open — deferred` and reduce its goal
  and acceptance criteria to the rate-limit-header canary.
- **084** — cannot produce an honest aggregate: `QuotaWindow` has no capacity field, and
  percentages of unknown unequal denominators don't sum. Gate corrected this pass: revisit after
  **034** (AGY tokens), not 030, which closed with only the Codex half delivered. **Tracker action:**
  keep it `Open — deferred`, replace the 030 dependency with 034, and state the missing-capacity
  decision as an acceptance prerequisite.
- **141** — Codex's status line is a closed item picker with no command hook; parked until an
  upstream customization path exists. **Tracker action:** keep it `Blocked` and name the missing
  upstream customization hook in the status reason.

**Split:**

- **224** — website-rules auto-install is a two-line change against existing machinery; the
  direct Android release scaffold shares no code path with it and deserves its own ticket.
- **166** — ✅ both parts closed; the local-LLM doc-profile thread continues as 231 (§8).

## 10. Newer backlog: build, harness, and documentation follow-through

These tickets were filed or materially clarified after the previous roadmap. They are ordered by
their effect on a working harnez session, then by the dependency they create for later agent
workflow work.
| Ticket | Scope | Bucket |
|---|---|---|
| 268 — bounded `harnez exec` timeout | M, protects the hook and agent command path from indefinite hangs | **Next** |
| 274 — session-start harness health checks | M, depends on stable cross-agent hook/shim status semantics | **Next** |
| 281 — opt-in advisor discovery | S, reduces routine context and quota cost; coordinate with the shipped advisor skill | **Next** |
| 285 — durable-note wording contract | S, closes a trust gap in normal agent conversations | **Next** |
| 293 — recoverable roadmap synthesis | M, improves this planning workflow; depends on an explicit safe recovery location | **Next** |
| 295 — actionable startup splash status | M, makes usage failures legible after the core dashboard fixes | **Next** |
| 296 — always compute distill savings | moved to §3 and paired with 124 — the two are halves of one efficiency number | **Next** (→ §3) |
| 297 — linked language subdocuments | M, extends the proven copyable-doc pipeline without bloating core docs | **Next** |
| 298 — commit checkpoint/file-granularity guidance | S, documentation first; split any hook enforcement into a separate design | **Next** |
| 300 — raw-mode and PTY input guidance | S, verified documentation gap with a low implementation cost | **Next** |

Rationale: 290, 292 and 299 have shipped, so this section is now purely forward-looking. The group
hardens the agent-facing execution and planning loop. The delivered or superseded 288/291 work and
the completed 302 research are handled only in §9 rather than scheduled here.

---


## 11. macOS Porting & OS-Agnostic Execution

| Ticket | Scope | Bucket |
|---|---|---|
| 334 — Research OS-agnostic process inspection across macOS and Linux | - | **Next** |
| 335 — Support afplay audio notifications on macOS in default hooks | - | **Next** |
| 336 — Research cross-platform shell shim patterns for macOS and Linux | - | **Next** |
| 337 — Research macOS system permissions and CLI whitelist for config template | - | **Next** |
| 338 — macOS CI verification via GitHub mirror | - | **Next** |
| 339 — Graceful degradation and gating of hardware telemetry and mic probes on macOS | - | **Next** |
| ~~341~~ — Concurrent SQLite telemetry writers lose rows on macOS | closed; fixed on Linux, macOS unverified. See §3 | **Done** (→ §3) |

## 12. Multimodal & Visual Context

Visual context cards (`harnez read -I`, `harnez find -I`, `harnez issues show -I`) are now a
first-class reading path for agents under Context Discipline, so their legibility is daily-loop
value, not cosmetics. The 429–434 pixel-font cluster shipped this cycle; the remaining rows are
separate follow-through rather than unfinished 434 milestones.

| Ticket | Scope | Bucket |
|---|---|---|
| 434 — one glyph spec per font size | ✅ closed: M1–M4 done, importer removed, text→PNG pipeline tests added | **Done** |
| 426 — make `find` output text-first for human users | S — the tracker's own CLI is read many times a day; visual-first output costs humans a step | **Now** |
| 427 — preserve ANSI colors in the stdin render path | S — colors are dropped today, which silently degrades piped render output | **Next** |
| 460 — ANSI 256-color/truecolor SGR support (from 428) | S/M — the telemetry half of 428 shipped in 235d7da; this is the ANSI remainder. Follows 427, which must first stop dropping colors at all | **Next** |
| 459 — Braille glyph cell margins (from 425) | S — the telemetry half of 425 shipped; this is the glyph-spacing remainder and belongs with the Dot8 pitch work below | **Next** (with 444) |
| 403 — transparent hook interception and distill adapter for multi-slice `harnez read` | M — makes the distill path apply to the read surface agents actually use | **Next** |
| 402 — move config diff below status, free top-level `harnez diff` for visual git diff | M — CLI surface change; do after 426 settles find/read output conventions | **Next** |
| ~~374~~ — refresh README CLI coverage and website link | ✅ **closed** this pass; the README now covers the current command surface including `harnez agent`. See §0 | **Done** |

**Dot8 experimental cluster (new this pass).** 436, 440, 441, 444 and 447 all landed after the
previous roadmap and form one dependency chain around the Braille/Dot8 card encoding.

| Ticket | Scope | Bucket |
|---|---|---|
| 444 — Dot8 renderer cell pitch for mixed glyphs | S — P1 and the chain's root: the renderer advances 3px per rune while the `Font3x5` fallback is 4px wide, so mixed lines overlap and clip. Nothing downstream can be measured honestly until the pixels are correct | **Now** |
| 436 — `--dot8` dense Braille cards for codex and agy | M — owner-rescoped to "implement the 3x4 geometry, then hand it over for manual testing"; the canary-first plan is explicitly superseded. Blocked in practice by 444 | **Next** |
| 441 — Dot8 PNG card reader (decode a card back to text) | M — the deterministic ground truth for legibility: if a program can decode the card, any agent failure is a vision limit, not a rendering bug. Its own ticket names 444 as the blocker | **Next** (after 444) |
| 440 — card header in PNG metadata instead of pixels | S/M — four candidate header channels (image band, PNG `tEXt`, sidecar file, inline note); the ticket's own analysis expects the *inline note* to be the one that actually works on codex and agy. Do the cheap channel first and only build the metadata path if measurement justifies it | **Later** |
| 447 — `B`/`#` as glyph-matrix colour symbols | S — naming-only groundwork for future colour channels, with an explicit "do not add colours now" constraint. Land it with 444 while the renderer is already open | **Next** |

Rationale: this cluster is explicitly experimental (three of its five tickets are P3) and does not
outrank §1a or the correctness bugs, with one exception — 444 is P1 and is the root of the chain,
so fixing it is what makes the rest of the cluster *decidable* rather than speculative. 440 drops
to **Later** despite being cheap, because its own ticket predicts the expensive channel is not the
one that will work; do the inline note, measure, and shelve the rest.

## 13. Agent Instructions & Tooling (Newer)

| Ticket | Scope | Bucket |
|---|---|---|
| ~~417~~ — harnez subagent MVP | ✅ **closed** — shipped as the `harnez agent` command tree; see §0 and §1a | **Done** |
| 273 — restore the old AGY PreToolUse hook as an opt-in configuration option | S — a regression for AGY users; opt-in keeps the default surface unchanged | **Next** |
| 294 — investigate cross-agent post-edit success hooks for the `harnez rate` pipeline | S — research; today the feedback pipeline only sees failures, which biases every stat built on it | **Next** |
| 306 — Detect and quarantine Codex subagents that remain unusable after usage limits | moved to §1a — it is a dispatch-surface concern now that dispatch is shipped | **Next** (→ §1a) |
| ~~315~~ — init drops previously opted-in docs on re-run | ✅ **closed** this pass — the destructive `init` re-run is fixed. Its siblings 289 and 355 remain open in §6 | **Done** |
| 322 — Evolve /story skill with optional focus areas, tooling fit, and human-steering divergence analysis | - | **Next** |
| 323 — docs/lang/Bash.md hard-references docs/practices/AgenticLoop.md instead of @docs/AgenticLoop.md alias | - | **Next** |
| 342 — MVP: /harnez-agent skill and CLI dispatch for agy host to codex:sol subagent | superseded by 417 — close candidate, see §9 | **Close** |
| 351 — Add AGENTS.md pointer: check config.yaml/Spec.md before hardcoding named business-value lists in Go | - | **Next** |
| 352 — Tell agents unrelated untracked files from parallel sessions are expected, not a fabrication concern | - | **Next** |
| 353 — Tell only Claude: never propose CLAUDE.md changes; check repo docs/AGENTS.md before any instruction-file change | - | **Next** |
| 365 — Add harnez release --init=<lang|mode> to bootstrap release scaffolding | - | **Next** |
| 366 — Automated doc compression command/skill with LLM canary evaluation loop | - | **Next** |
| 369 — Detect Go library CLI and TUI shape for init guidance | - | **Next** |
| 370 — Offer useful Go agent capabilities through harnez init | - | **Next** |
| 371 — Add optional systemd service guidance to Go init profile | - | **Next** |
| 382 — Make user shell shortcuts available to Codex, Claude, AGY, and other agents | - | **Next** |

## 14. Telemetry & Misc (Newer)

| Ticket | Scope | Bucket |
|---|---|---|
| 304 — Advisor lifecycle CLI and metadata tracking | - | **Later** |
| 305 — Investigate stats --auto failure rates disagreeing with observed tool outcomes | - | **Later** |
| 310 — Check for ambient enclosing go.work in harnez status and lint | - | **Later** |
| 313 — Canary convention: require go.work/GOWORK probe before trusting scratch-module dependency checks | - | **Later** |
| 314 — harnez release: auto-create minisign key if missing and no repo key defined | - | **Later** |
| 317 — split harnez-advisor per-harness guidance into resources/ reference files | - | **Later** |
| 321 — harnez release --login flag to authenticate or refresh forge credentials via fj auth login | - | **Later** |
| 324 — docs/lang/Go.md canonical-pattern reference to internal/usage is dangling in consumer repos | - | **Later** |
| 329 — Gate harnez release on REUSE Compliance, with --no-reuse and a Global Opt-Out | - | **Later** |
| 330 — Adopt voxi audiolevel's Rolling Braille Timeline Alongside the Existing Live Mic-Level Bar | - | **Later** |
| 333 — Retire docs/feedback/, consolidate into docs/studies/ with labels/categories for grouping and filtering | - | **Later** |
| 361 — harnez docs variant --check CLI verb for lite-doc structural gate | - | **Later** |
| 378 — Fleet-wide multi-repo git history sparks and token attribution matrix for uman | - | **Later** |
| 383 — Disable queued question tool prompts in Codex sessions | moved to §1a (dispatch-surface papercut) | **Next** (→ §1a) |
| 384 — harnez assess directory validation and -d/--dir flag support | - | **Later** |
| 385 — Tokens command to count tokens in files and directories | - | **Later** |


Rationale for newer backlog sequencing:
- Telemetry data management (Section 3) leads this pass by explicit focus: the store is the
  substrate every cost, efficiency and A/B number depends on, and 424 showed a defect can sit in it
  undetected. 341 → 127 → 457 has largely landed: write path fixed on Linux, SQL single-homed in
  `spec/`, data quality a reported property via `stats --quality`. The remaining work is extension.
- Cost telemetry (446/445) follows immediately, because cheap-tier routing is only a policy if
  something measures what it saved — and those measurements are only worth taking once the store
  they land in is trusted.
- Cross-agent dispatch (Section 1a) drops from the lead: 454 shipped and 450 is blocked on
  `../loom`, leaving 449 as the actionable remainder.
- macOS Porting (Section 11) is elevated to Next to fulfill the primary OS-Agnostic Readiness objective.
- Multimodal/Visual Context (Section 12) is Next because visual context cards are now a routine
  agent reading path under Context Discipline, not just a debugging aid — an unreadable glyph in a
  card is a wrong number on the daily surface.
- Agent Instructions (Section 13) are scheduled as Next because they prevent context leaks and improve the daily agentic workflow correctness.
- Telemetry/Misc (Section 14) are placed in Later to ensure the core execution loops are hardened first.

## Suggested order of attack

Refreshed this pass. The telemetry hardening chain landed (457 closed, 127 M1/M2, 341 fixed on
Linux), so the lead moves from hardening to extension.

1. **Telemetry hardening chain is closed** (341, 127, 424, 457 done; 341 unverified on macOS).
   Remaining SQL-to-spec work is 458, picked up opportunistically as later items touch those files.
2. **Give the cost story measured numbers**: 446 (cost fields agents already report) → 445
   (counterfactual rate cards in `spec/`), plus 461, the model-attribution gap from 457 (`tool_calls` has
   no `model` column) → 435's A/B telemetry half.
3. **Complete the efficiency numbers**: 124 (PostToolUse tool-call capture, canary-gated) + 296
   (always compute distill savings) as one pair; `stats --quality` will show when the columns fill.
4. **Classification quality**: 225 (cache versioning, then an accuracy baseline) → 215 (LLM
   backfill/reclassification), so the backfill's effect is measurable as a `stats --quality` delta.
   Then 421 and 208 last in this theme.
5. **Close the delivered-feature tickets** (nearly free, and it shrinks everything below): 291,
   342 → close as superseded by 417; 288 and 435 → rescope to what 417 did not deliver; 302 →
   verify the delivered study against its acceptance criteria, then close. Five tickets leave the
   backlog without writing code.
6. **Remaining live correctness bugs**: 289 (`init` go.work hard-fail) → 279 (tracker lock
   sidecar) → 255 (finish collector key decoding and diagnostics tests). 424, 315 and 442 are closed.
7. **Dispatch follow-ons**: 449 (spec-driven chat model selection, now unblocked by 454) → 306
   (quarantine dead Codex sessions) → 383 → 144. 450 is excluded until `../loom` lands.
8. **OS-agnostic terminal foundation**: 286 (Go conventions + `x/term` `watch.go` refactor) →
   the OS build-tag split → portable process detection → 334/336/337 research → 338/339. 341 is
   closed (fixed on Linux); macOS remains unverified until 338's CI runs.
9. **Finish the visual-context and public-doc surface**: 444 (P1 pitch bug) + 447 + 459 (Braille
   glyph spacing) → 426 → 427 → 460 (ANSI 256/truecolor) → 441 → 436. 374 is
   done; 440 stays out until the inline-note channel has been tried.
10. **Cash in the 149 dividend**: 144 (→ §1a) → 151 → 231 → 176 → 145. All were blocked on the
    profile mechanism; it exists now, and 145 has waited three passes.
11. **Harden agent execution and planning**: 268 → 274 → 281 → 285 → 293 → 295, plus the
    cost-discipline docs pass 451 + 453 as one small batch.
12. **Tracker ergonomics**: 340 (query filters) → 283 → 246, so the next planning pass costs less
    than this one did. 340 is still unbuilt, and this pass again read the whole open list for want
    of it.
13. **Mic indicators and audio UX**: 250 → 251 → 264 → 253 → 278.
14. **Collector depth**: 113 → 111 → 034 → 294. Note 034 now also gates 084, since 030 closed with
    only the Codex half; 296 has moved up into step 3.
15. Revisit **Later** items after 160 has a decision attached, and keep any plugin implementation
    outside this roadmap until separately scoped.
