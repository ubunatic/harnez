# Roadmap

Working roadmap for the open backlog (updated 2026-09-17). Derived from each ticket's
appended `## Implementation Plan`, so scope calls here reflect the planning pass, not a fresh
re-derivation.

**Strategic Goal — OS-Agnostic & Cross-Platform Readiness (macOS first)**:
harnez is preparing to become fully OS-agnostic, targeting macOS (Darwin) as the primary non-Linux platform. This requires eliminating hardcoded GNU/Linux CLI tool dependencies (`stty`, `pactl`, `amixer`, `/proc/*`) in favor of standard Go cross-platform libraries (such as `golang.org/x/term`), OS build-tag splits for system telemetry, and platform audio/process abstractions.

**Guiding bias for ordering**: harnez's primary daily surface is a single workstation with
`harnez usage --watch` open in a terminal split, plus the issue tracker and the instruction
docs that every agent session loads. Work that makes that daily loop more correct, more
legible, and OS-agnostic outranks work that adds new capability surface.

Sequencing buckets:

- **Now** — actionable today, no blockers, high daily-loop value or already in flight.
- **Next** — actionable but either larger, lower daily value, or waiting on a Now item.
- **Later** — real but speculative, large, or dependent on decisions not yet made.
- **Close / Park** — should not be scheduled; see §9.

---


## 0. Shipped Recently

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

## 1. OS-Agnostic Readiness (macOS first) & Terminal Modernization

Laying the foundation to run seamlessly across operating systems (macOS / Darwin at first), eliminating brittle Linux-only subprocess forks in the interactive TUI and establishing portable system abstraction layers.
| Ticket | Scope | Bucket |
|---|---|---|
| 286 — promote `golang.org/x/term` for terminal operations in Go conventions & watch.go | S — add `x/term` carve-out to `docs/lang/Go.md`, replace `stty` subprocesses & raw-mode ioctls in `internal/usage/watch.go` with `x/term` | **Now** |
| (Architecture) — OS build-tag split for system telemetry (`/proc` vs Darwin `sysctl`/`mach_vm`) | M — decouple Linux `/proc/stat`, `/proc/meminfo`, `/proc/loadavg` behind OS-specific collectors | **Next** |
| (Architecture) — Cross-platform process detection (`ps` vs Darwin API/`sysctl`) | S/M — replace GNU-specific `ps -eo comm=` with portable detection | **Next** |
| (Architecture) — macOS CoreAudio / microphone level probe backend | M — platform backend counterpart to PipeWire/Pulse/ALSA | **Later** |

Rationale: 286 is immediate, high-leverage low-hanging fruit — it updates the Go convention docs, eliminates the `stty` dependency from `watch.go`, and cuts subprocess CPU overhead in one clean step. It directly unlocks running the watch TUI reliably on macOS and non-GNU environments without requiring coreutils `stty`.

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
| 219 — subtler usage-bar colors vs Braille charts | S — spec/colors.yaml ramp split | **Next** |
| 214 — 256-color heat palette option | M — third value for two existing presentation enums | **Next** |
| 143 — Git status in all agent status bars | M — shared collector + per-agent wiring | **Next** |
| 141 — running-agent count in Codex status bar | blocked: Codex status line is a closed item picker, not a command hook | **Later** |
| 146 — recent-subagent-activity watch box | L — needs a per-tool feasibility matrix first | **Later** |
| 247 — third mic graph (amplitude-over-time audiogram) | M/L — visual addition, evaluate after 262 | **Later** |
| 051 — multi-host monitoring + host navigation | L — introduces an "active host" concept the watch state has never had | **Later** |
| 160 — extract watch layout/UI into renderer-agnostic module | feasibility done; only the user's proceed/stage call remains | **Later** |
| 161 — collector absorbs remote-load host + Prometheus | M, but only pays off after 160/051 direction is set | **Later** |

Rationale: 210 → 201 → 139 → 105 → 262 → 255 is the shortest path to "the dashboard never shows a wrong or
silently stale number." 262 fixes a broken TUI box; 255 stops opaque errors. 105 is the one that stops the recurring class of incident (086, 103/104)
where a dead collector was only caught by hand-digging on disk. Cosmetics (219, 214) and the
status-bar work follow once the numbers are trustworthy.

## 2. Collector pipeline & token sources
| Ticket | Scope | Bucket |
|---|---|---|
| 034 — hook-triggered token extraction | plan revised: hook infra now exists (`agy-hooks`, `codex-hook`), scope shrank | **Next** |
| 113 — per-collector roundtrip times, `usage --meta` | S/M — extend existing `internal/usage/fetchdurations.go`, do not build a second timing store | **Next** |
| 111 — per-agent cadence/timeout/cancellation | premise corrected: collection is already concurrent; reduced to cadence + timeout + cancel | **Next** |
| 035 — transparent HTTPS proxy sidecar | superseded in most of its value; only rate-limit headers remain unique | **Park** (§9) |
| 084 — aggregate quota-window box | blocked: `QuotaWindow` has no capacity field, and 030's token data is missing for 2 of 3 agents | **Later** (after 030) |

Rationale: doing the Codex half of 030 first is nearly free and unblocks the token column for a
second agent. 113 before 111 — you want the measurements before tuning the cadence they'd inform.

## 3. Telemetry data layer & analytics
| Ticket | Scope | Bucket |
|---|---|---|
| 127 — move `internal/telemetry` SQL into `spec/` | M — 10 statements, not the 5 the ticket lists; needs a clean tree | **Next** |
| 215 — LLM backfill/reclassification of tool notes | M — new `activity_category` column + migration + CLI | **Next** |
| 225 — local SLM classifier reliability/accuracy | S — cache versioning first, then an accuracy baseline | **Next** |
| 178 — distill smart mode, error-pattern preservation | M — key constraint: do not import `internal/telemetry` from `internal/distill` | **Next** |
| 208 — SQLite export format | S/M — depends on 204's scrubbed record slices | **Later** |

Rationale: 225's cache versioning is a prerequisite for any honest accuracy experiment, so it
precedes 215's backfill. 127 is pure architecture hygiene — schedule it into a quiet slot, on a
clean tree.

## 4. Issue tracking & tracker tooling
| Ticket | Scope | Bucket |
|---|---|---|
| 246 — add /commit and /publish Skills | M — multi-project staged commit ownership | **Next** |

Rationale: 108 is a live data-integrity bug in the tracker — duplicate ticket numbers have
already happened twice. 217 is small and directly reduces daily friction. 246 extends the skill set.

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
| 144 — Codex subagent model selection policy | S once 149 exists (its content migrates into a profile) | **Next** (after 149) |
| 151 — on-demand lookup vs. materialized instructions (research) | research doc; item 4 depends on 149's design | **Next** (after 149) |
| 128 — full system-prompt self-audit for repetition | research; 2 of 4 ACs met, needs the with-project-docs pass | **Next** |
| 135 — Tool Feedback Protocol delivered twice | S — measure with `harnez stats --overhead` first, differentiate only if warranted | **Next** |
| 134 — ConciseMode trigger Skill | S — the `mode` skill already exists; only its *description* needs to become trigger-shaped | **Next** |
| 145 — orchestrator-session skill/command | M — best written after 149/144/176 settle the conventions it would encode | **Later** |
| 053 — stale LSP diagnostics detect/toggle | research first (is the toggle even exposed?); disagreement-detector explicitly rejected — harnez has no observation point | **Later** |

Rationale: 149 is the keystone here. 144, 151, 145, and the local-LLM profile work (165/166B)
all either depend on it or would invent a competing mechanism if they land first. 128 and 135
are the measurement side of the same problem — do them near 149 so the audit informs the design.

## 6. Config & doc management (`apply` / `init` core)
| Ticket | Scope | Bucket |
|---|---|---|
| 005 — permissions are grow-only | M — needs a managed-permission state sidecar so user/Claude-Code additions survive | **Next** |
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
| 124 — PostToolUse auto-capture of tool-call counts | canary-gated: run the payload probe before writing code | **Later** |
| 073 — credentialed cloud-agent canary | blocked on a user decision about credential-mounting posture | **Park** (§9) |

Rationale: 007's `/* */` gap is the sharp edge — a hand-written JSONC config with block comments
currently parses to nothing and `apply` treats that as "nothing applied." Worth doing even if the
rest of 007's table-test sweep waits.

## 8. Local-LLM support

Coherent cluster, all P3, all gated on §5's profile mechanism.
| Ticket | Scope | Bucket |
|---|---|---|
| 166 Part A — Go rune/display-width invariants in `docs/lang/Go.md` | ✅ shipped; tracker closed | **Now** |
| 166 Part B — compact local-LLM doc profile | blocked on 149/151 | **Later** |
| 165 — two-phase architect/patch harness | tracking-only by the ticket's own instruction; decide "profile or workflow?" on paper first | **Later** |
| 167 — local runtime targets & telemetry (Ollama, llama.cpp, vLLM) | design skeleton; register runtimes as ordinary agent IDs, **do not ship `--agent local`** | **Later** |

Rationale: Part A of 166 is a plain docs win with no dependency and should be split out now. The
rest waits — building a local-model profile before 149 defines the profile mechanism means
building it twice.

## 9. Close, park, or split

**Close now — work is done or the premise is disproven:**

- **071** — every architectural decision is realized in `scripts/agent-canary/`; the
  implementation was split to 072 (closed) and 073. Nothing to implement.
- **123** (`harnez run <agent>` supervisor) — the premise failed in the opposite direction:
  *every* target agent turned out to have a native hook surface, and all are wired. A PTY
  supervisor would buy nothing.
- **201** — implementation complete; only the tracker entry is open.
- **302** — the requested comparison study now exists at `docs/studies/2026-09-10-agent-harness-plugin-systems-and-self-modification.md`; close after confirming the tracker record reflects that deliverable.

**Park — blocked on something no amount of work here resolves:**

- **073** — needs a user decision on credential-mounting posture. Cheap pre-work: confirm that
  AGY/Codex have no pre-exec rewrite hook, which would collapse this to Claude-Code-only and make
  the decision much easier.
- **172** — the rendering half is fixed; the remainder needs a live 100%-capped AGY account (or a
  captured fixture) to verify against.
- **035** — reduce to a scoped canary rather than building it. Token counts no longer need a
  proxy (Claude aggregates; Codex now writes plain-JSON rollouts). The only unique remaining
  capability is rate-limit response headers — a narrow payoff for a MITM CA plus TLS trust
  injection into three runtimes.
- **084** — cannot produce an honest aggregate: `QuotaWindow` has no capacity field, and
  percentages of unknown unequal denominators don't sum. Revisit after 030 fills tokens for all
  three agents.
- **141** — Codex's status line is a closed item picker with no command hook; parked until an
  upstream customization path exists.

**Split:**

- **224** — website-rules auto-install is a two-line change against existing machinery; the
  direct Android release scaffold shares no code path with it and deserves its own ticket.
- **166** — Part A (Go rune/width docs) ships now; Part B stays blocked on 149/151.

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
| 288 → 291 — external-agent handoff, then standardized CLI dispatch | M/L combined; settle the prose handoff before adding the command/adapters | **Next** |
| 293 — recoverable roadmap synthesis | M, improves this planning workflow; depends on an explicit safe recovery location | **Next** |
| 295 — actionable startup splash status | M, makes usage failures legible after the core dashboard fixes | **Next** |
| 296 — always compute distill savings | S/M, improves honest efficiency reporting; follow the existing telemetry model | **Next** |
| 297 — linked language subdocuments | M, extends the proven copyable-doc pipeline without bloating core docs | **Next** |
| 298 — commit checkpoint/file-granularity guidance | S, documentation first; split any hook enforcement into a separate design | **Next** |
| 300 — raw-mode and PTY input guidance | S, verified documentation gap with a low implementation cost | **Next** |
| 302 — plugin/self-modification research | study complete; close candidate in §9, with any Harnez design as a separately reviewed follow-up | **Close candidate** |

Rationale: 290 blocks the normal build/test loop and therefore precedes feature work. 292 and 299
are similarly cheap correctness repairs in the tracker and documentation surfaces. The next group
hardens the agent-facing execution and planning loop; 288 and 291 stay together because a handoff
contract without a tested dispatch surface, or a dispatcher without that contract, would create
another incompatible workflow. 302 remains outside implementation sequencing until its research
recommendations have been reviewed.

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
| 341 — Concurrent SQLite telemetry writers lose rows on macOS | - | **Next** |

## 12. Multimodal & Visual Context

| Ticket | Scope | Bucket |
|---|---|---|
| 374 — Refresh README CLI coverage and website link | - | **Next** |
| 399 — harnez read image enhancements: configurable line-number cadence and AST-safe whitespace compression | - | **Next** |
| 402 — Move config diff below status and free top-level harnez diff for visual git diff | - | **Next** |
| 403 — Transparent hook interception and distill adapter for multi-slice file reads via harnez read | - | **Next** |

## 13. Agent Instructions & Tooling (Newer)

| Ticket | Scope | Bucket |
|---|---|---|
| 306 — Detect and quarantine Codex subagents that remain unusable after usage limits | - | **Next** |
| 315 — init drops previously opted-in docs on re-run; Canary.md hard-references opt-in PrototypingFeatures.md | - | **Next** |
| 322 — Evolve /story skill with optional focus areas, tooling fit, and human-steering divergence analysis | - | **Next** |
| 323 — docs/lang/Bash.md hard-references docs/practices/AgenticLoop.md instead of @docs/AgenticLoop.md alias | - | **Next** |
| 342 — MVP: /harnez-agent skill and CLI dispatch for agy host to codex:sol subagent | - | **Next** |
| 351 — Add AGENTS.md pointer: check config.yaml/Spec.md before hardcoding named business-value lists in Go | - | **Next** |
| 352 — Tell agents unrelated untracked files from parallel sessions are expected, not a fabrication concern | - | **Next** |
| 353 — Tell only Claude: never propose CLAUDE.md changes; check repo docs/AGENTS.md before any instruction-file change | - | **Next** |
| 355 — harnez apply/init do not prune agentsmd sections removed from config.yaml, leaving orphaned managed blocks | - | **Next** |
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
| 340 — Support label, project, and category query filters in harnez find issues | - | **Later** |
| 361 — harnez docs variant --check CLI verb for lite-doc structural gate | - | **Later** |
| 378 — Fleet-wide multi-repo git history sparks and token attribution matrix for uman | - | **Later** |
| 383 — Disable queued question tool prompts in Codex sessions | - | **Later** |
| 384 — harnez assess directory validation and -d/--dir flag support | - | **Later** |
| 385 — Tokens command to count tokens in files and directories | - | **Later** |


Rationale for newer backlog sequencing:
- macOS Porting (Section 11) is elevated to Next to fulfill the primary OS-Agnostic Readiness objective.
- Multimodal/Visual Context (Section 12) is Next because visual debugging has proven critical for resolving TUI alignment and rendering bugs.
- Agent Instructions (Section 13) are scheduled as Next because they prevent context leaks and improve the daily agentic workflow correctness.
- Telemetry/Misc (Section 14) are placed in Later to ensure the core execution loops are hardened first.

## Suggested order of attack

1. **OS-Agnostic terminal foundation & dev loop**: 286 (Go conventions + `x/term` watch.go refactor) → 290 → 210 → 201 (close) → 139 → 105 → 262 → 255
2. **Repair tracker and install/documentation correctness**: 292 → 299 → 108 → 217 → 126 → 209
3. **Finish the current docs batch**: 263 → 166 Part A → 297 → 300
4. **Harden agent execution and planning**: 268 → 274 → 281 → 285 → 293 → 295
5. **Instruction and dispatch architecture**: 149 → 144, 151, 128, 135, 134 → 288 → 291
6. **Mic indicators and audio UX**: 250 → 251 → 264 → 253
7. **Collector depth and efficiency evidence**: 030 → 113 → 111 → 034 → 296
8. Revisit **Later** items after 149 and 160 have decisions attached; close 302 once its study is
   accepted, and keep any plugin implementation outside this roadmap until separately scoped.
