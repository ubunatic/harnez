# Roadmap

Working roadmap for the open backlog (77 tickets as of 2026-09-04). Derived from each ticket's
appended `## Implementation Plan`, so scope calls here reflect the planning pass, not a fresh
re-derivation.

**Guiding bias for ordering**: harnez's primary daily surface is a single workstation with
`harnez usage --watch` open in a terminal split, plus the issue tracker and the instruction
docs that every agent session loads. Work that makes that daily loop more correct or more
legible outranks work that adds new capability surface.

Sequencing buckets:

- **Now** — actionable today, no blockers, high daily-loop value or already in flight.
- **Next** — actionable but either larger, lower daily value, or waiting on a Now item.
- **Later** — real but speculative, large, or dependent on decisions not yet made.
- **Close / Park** — should not be scheduled; see §9.

---

## 1. Usage watch TUI — correctness & legibility

The `--watch` dashboard is the tool's front door. Everything that makes it lie, misalign, or
hide a failed collector belongs at the front of the queue.

| Ticket | Scope | Bucket |
|---|---|---|
| 210 — low nonzero Braille load values invisible | S, in progress, P1 | **Now** |
| 201 — doubled sparkline resolution + Braille | done, tracker closure only | **Now** |
| 139 — Codex quota window rollover keeps stale limit | M — three root causes, all identified in-plan (frozen `DurationLeft`, cache, renderers) | **Now** |
| 105 — per-collector fetch status in usage UI | M — introduces `FetchMode` enum, retires `strings.Contains(source,"stale")` | **Now** |
| 085 — show collector-daemon status in watch | S — no plan written yet; small sibling of 105, land with it | **Next** |
| 172 — AGY single-window row alignment | rendering bug fixed; remainder needs a live capped account | **Park** (§9) |
| 219 — subtler usage-bar colors vs Braille charts | S — spec/colors.yaml ramp split | **Next** |
| 214 — 256-color heat palette option | M — third value for two existing presentation enums | **Next** |
| 143 — Git status in all agent status bars | M — shared collector + per-agent wiring | **Next** |
| 141 — running-agent count in Codex status bar | blocked: Codex status line is a closed item picker, not a command hook | **Later** |
| 146 — recent-subagent-activity watch box | L — needs a per-tool feasibility matrix first | **Later** |
| 051 — multi-host monitoring + host navigation | L — introduces an "active host" concept the watch state has never had | **Later** |
| 160 — extract watch layout/UI into renderer-agnostic module | feasibility done; only the user's proceed/stage call remains | **Later** |
| 161 — collector absorbs remote-load host + Prometheus | M, but only pays off after 160/051 direction is set | **Later** |

Rationale: 210 → 201 → 139 → 105 is the shortest path to "the dashboard never shows a wrong or
silently stale number." 105 is the one that stops the recurring class of incident (086, 103/104)
where a dead collector was only caught by hand-digging on disk. Cosmetics (219, 214) and the
status-bar work follow once the numbers are trustworthy.

## 2. Collector pipeline & token sources

| Ticket | Scope | Bucket |
|---|---|---|
| 030 — AGY/Codex have no local token counts | **Codex half now unblocked** — `~/.codex/sessions/**/rollout-*.jsonl` carries plain-JSON `token_count`; AGY half still protobuf | **Now** (Codex only) |
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
| 217 — `-n` limit (default 10) and `--all` on `find issues` | S | **Now** |
| 108 — sequential subagent dispatch + number-allocation race guard | **`Reserve` does not actually close the race** — `O_EXCL` keys on number *plus slug*, so two agents can both own one number (the 179/180 pairs) | **Now** |
| 229 — `/issue` skill for filing tickets | S/M — config.yaml command+skill registration, pipeline tests | **Now** |
| 126 — document `Closed — resolved in <commit>` | S, docs-only; evidence says converge on Option B (bare `Closed`) | **Now** |

Rationale: 108 is a live data-integrity bug in the tracker — duplicate ticket numbers have
already happened twice. 217 and 229 are small and directly reduce daily friction.

## 5. Agent instructions & practice docs

Two sub-clusters: a batch of small `AgenticLoop.md` edits, and a larger question about how
instructions are delivered at all.

**Docs batch — land as one or two commits** (all touch `docs/practices/AgenticLoop.md`, several
also `commands/sprint.md`; landing them separately guarantees conflicts):

- 042 — repro-before-fix + one Status field per ticket (Phase 2 / Phase 5)
- 045 — root-cause vs. symptom review-checklist item (Phase 3)
- 046 — end-of-work commit checkpoint (the 044 gap that recurred)
- 056 — "piping long-running output through a buffering filter" anti-pattern
- 125 — chat-visible empty polling anti-pattern
- 222 — prefer `git -C` / `make -C` over `cd` (also `docs/lang/Bash.md`)
- 095 part 1 — restore cwd after shell tool use (2–3 lines in the global section, rationale in Bash.md)

→ **Now.** Zero code, high per-session value, and they collide with each other if deferred
individually.

| Ticket | Scope | Bucket |
|---|---|---|
| 221 — Go-first for scripts, demote ad-hoc Python | S — do *not* add a new `docs/practices/Languages.md` (130's precedent) | **Next** |
| 176 — capped subagent completion-report contract | S — new `docs/practices/SubagentReporting.md` + skill refs | **Next** |
| 156 — document `collaboration.spawn_agent` in Codex Agents | S, docs-only | **Next** |
| 149 — agent-specific instruction profiles (Codex async-wait) | M — **key finding: there is no per-agent instruction file today**; `~/AGENTS.md` symlinks to `~/.claude/CLAUDE.md`, so this needs a new agent-owned target | **Next** |
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
| 006 — status checks only the `model` key | S — iterate `managedSettingsKeys`, gate on presence in the built doc | **Now** |
| 018 — mark bundled docs in front matter | S — marker convention already load-bearing; 10 of 20 copyable docs unmarked, backfill + lint | **Now** |
| 005 — permissions are grow-only | M — needs a managed-permission state sidecar so user/Claude-Code additions survive | **Next** |
| 209 — move `agy-hooks` into `harnez apply` | S/M — Codex hooks are the exact precedent; also fixes the `status` gap | **Next** |
| 152 — move `agent-collector` under `usage` | S, **but** the systemd unit hardcodes `ExecStart … agent-collector`; needs an alias + migration, not a rename | **Next** |
| 015 — teach AGENTS.md about `uman` | S — mirror the `repo_modes` opt-in mechanism, not a global section | **Next** |
| 016 — `make smoke` convention in Make.md | S, docs + template | **Next** |
| 224 — website rules auto-install | S — `Website.md` is *already* copyable; make it self-install for website-capable projects. **Split the Android release scaffold into its own ticket.** | **Next** |
| 092 — latest-release links / README install | S — Option B (linter probe) only; explicitly reject the managed-README-block option | **Next** |
| 096 — `has_releases` 401 warning lacks the fix hint | S — thread token provenance out of `GetForgeToken` | **Next** |
| 170 — modularize `cmd/harnez/main.go` | M refactor — every command closes over two shared vars | **Later** |
| 009 — `diff`/`clean` don't cover Makefile targets | scope corrected: belongs on `init --dry-run`, not on the global-only `DiffAll`/`CleanAll` | **Later** |
| 013 — `promote` command | L, new command with an agent-invocation surface | **Later** |

Rationale: 006 and 018 are ~an hour each and remove two classes of silent wrongness (status
reporting "ok" for keys it never checks; agents hand-editing docs that get overwritten). 005 is
the real one but needs the state sidecar designed carefully so `apply` never deletes a
permission the user approved interactively.

## 7. Testing, canary & agent visibility

| Ticket | Scope | Bucket |
|---|---|---|
| 010 — smoke-test that agents see installed skills/commands | **unblocked** — wayreel#11 landed, `verifyContains` exists | **Next** |
| 007 — thin test coverage | M — one test file per package; `stripComments` genuinely lacks `/* */` support, which silently yields an empty map | **Next** |
| 177 — lean post-edit build check for control-flow edits | M — hook option chosen; prototype the heuristic against real past edits first | **Later** |
| 124 — PostToolUse auto-capture of tool-call counts | canary-gated: run the payload probe before writing code | **Later** |
| 071 — agent canary architecture | delivered; nothing left to implement | **Close** (§9) |
| 073 — credentialed cloud-agent canary | blocked on a user decision about credential-mounting posture | **Park** (§9) |

Rationale: 007's `/* */` gap is the sharp edge — a hand-written JSONC config with block comments
currently parses to nothing and `apply` treats that as "nothing applied." Worth doing even if the
rest of 007's table-test sweep waits.

## 8. Local-LLM support

Coherent cluster, all P3, all gated on §5's profile mechanism.

| Ticket | Scope | Bucket |
|---|---|---|
| 166 Part A — Go rune/display-width invariants in `docs/lang/Go.md` | S, independent of everything | **Now** |
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

---

## Suggested order of attack

1. **Dashboard truth**: 210 → 201 (close) → 139 → 105 (+085)
2. **Tracker integrity**: 108 → 217 → 229 → 126
3. **Docs batch, one commit**: 042, 045, 046, 056, 125, 222, 095(pt.1) — plus 166 Part A
4. **Cheap `apply` correctness**: 006, 018, 030 (Codex half)
5. **Instruction architecture**: 149 → 144, 151, 128, 135, 134
6. **Collector depth**: 113 → 111 → 034
7. Everything in **Later**, reprioritized once 149 and 160 have decisions attached.
