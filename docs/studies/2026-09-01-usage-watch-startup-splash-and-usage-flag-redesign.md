---
title: "Usage --watch Startup Splash, CLI Flag Redesign, and a Context-Cost Audit"
weight: 100
---

# Case Study: `usage --watch` Startup Splash, `harnez usage` Flag Redesign, and a Context-Cost Audit

**Scope**: issues 164–172; `internal/usage/watch.go`, `internal/usage/usage.go`,
`internal/usage/agy.go`, `cmd/harnez/main.go`

## 1. What Happened

One continuous session (main orchestrator + four dispatched dev subagents) closed a full arc on
`harnez usage --watch`'s startup experience, then pivoted into a CLI flag redesign of `harnez
usage` itself, then filed a bug report. In order:

1. **164 — startup splash**: `--watch` blocked ~3s on the first live quota fetch before painting
   anything. Fixed by entering the alt-screen immediately and painting a splash (spinner + bar)
   while the fetch runs in a background goroutine; Esc aborts the wait without quitting the app
   (a deliberate deviation from Esc's normal quit behavior — scoped to the splash only).
2. **168 — determinate bar**: the splash's bar was an indeterminate 0→100→0 sweep. Replaced with a
   real left-to-right fill driven by a persisted rolling (EWMA) duration estimate per fetch kind
   (local vs. `CollectRemote`-per-host), reusing the existing on-disk quota-cache locking machinery
   rather than inventing new storage. Capped at 95% so it never visually "finishes" before the real
   fetch does; falls back to the old sweep on a true cold start with no estimate yet.
3. **169 — fetch-stage status line**: added a single status line under the bar reporting which
   per-source fetch (claude/agy/codex) is in flight, via an additive `FetchProgressFunc` threaded
   through `CollectAllProgress`/`CollectRemoteProgress` (nil callback for every existing non-watch
   caller, zero cost there).
4. **Same-session follow-up fix (no new ticket number needed — fixed inline)**: the status line
   from 169 only ever showed the *last* event ("agy done") in practice, because local sub-fetches
   often complete faster than one splash paint tick. Root cause was two-fold: (a) events overwrote
   each other with no minimum display time, and (b) the splash loop exited the instant the whole
   fetch finished, regardless of what was still queued. Fixed by queuing events with a
   `splashStatusMinDisplay` pacing (`splashStatusAdvance`) and by splitting `renderFrame` into
   `fetchAndUpdate` (data only) + `draw()` (paint only), so the real dashboard's paint could be
   deliberately delayed until the status queue drained — otherwise a fast fetch's `draw()` call
   would land mid-queue and then get silently overwritten by further splash repaints, leaving the
   dashboard invisible until the next periodic tick.
5. **171 — `harnez usage` flag redesign**: separate thread, started from a plain bug report
   (`--compact` alone errored). The fix escalated through two rounds of user correction: first
   "don't make `--compact` require `--watch`", then "don't even make it imply `--watch` — just
   show the compact output" — which led to the actual final shape: the compact one-shot dashboard
   (previously reached only via `--summary`) is now the **default** for a bare `harnez usage`,
   `--summary` was **removed outright** (solo/hobby repo, no deprecation shim — see
   [Git.md](../lang/Git.md)/Repo Setup convention), and a new `--raw`/`-r` flag opts into the old
   flat/detailed per-field report that used to be the unconditional default.
6. **172 — bug report only, not implemented**: a screenshot showed AGY's "Claude/GPT" row missing
   its second (5-hour) window at 100% weekly usage, visibly breaking the All Usage box's grid
   alignment. Traced to two independent causes: a confirmed harnez rendering bug (single-window
   rows in `formatCompactGroupLineWithLabelWidth` aren't padded to the two-bar column width other
   rows use) and an unconfirmed upstream hypothesis (AGY's own CLI may stop printing the five-hour
   line once weekly is fully consumed — `parseAGYUsageOutput` has no fallback/synthesis for a
   missing line, since it's scraping AGY's CLI text output, not a versioned API).

## 2. Agentic Workflow Notes

- **Dispatch pattern that worked**: each of 164/168/169 was a self-contained, well-scoped ticket
  handed to a fresh dev subagent with explicit file/line pointers, a hard spec-driven-styling
  requirement (reuse `spec/colors.yaml`/`spec/indicators.yaml`, no hardcoded ANSI/glyphs), and an
  explicit pty-based-repro verification requirement (per this project's standing rule that
  reasoning-only fixes for ANSI/terminal bugs have failed repeatedly before). All three subagents
  came back green with real pty repro evidence, not just "tests pass" claims.
- **Sequential-write discipline held**: 169's dispatch was explicitly delayed until 168's subagent
  had landed and freed `internal/usage/watch.go`, per this project's no-worktree-isolation
  convention (sequential agents on the same checked-out branch, not concurrent writers).
- **A concurrent unrelated session was running in parallel** (issue 170, a `main.go` modularization
  ticket, appeared as an untracked file and later a modified `issues/README.md` mid-session,
  neither of which this session created). Handled correctly per standing guidance: left untouched,
  not reverted, and `issues/README.md` was committed as a merged whole rather than trying to
  surgically split one shared index file's diff across two sessions' work.
- **A real self-inflicted mistake**: one `Edit` call on `cmd/harnez/main.go` used an `old_string`
  that matched a smaller span than intended, and the `new_string` didn't correctly close out the
  function — this left ~45 lines of dead/duplicated code in the file (visible only on the next
  full read, not from the edit tool's own success response). Recovered by reading the affected
  region and doing a second corrective edit; verified with `go build` immediately after. The lesson
  worth generalizing: **a multi-hunk `Edit` on a control-flow-heavy function is higher-risk than it
  looks** — the tool reports success as soon as the string match succeeds, which says nothing about
  whether the *surrounding* code is still structurally valid. A `go build`/`go vet` immediately
  after any edit that reshapes function bodies (not just changes a line) would have caught this one
  edit sooner than the eventual full-file read did.
- **User corrections compounded rather than restarted work**: the `--compact` fix went through two
  rounds of user redirection (imply `--watch` → don't imply anything, use `--summary`'s behavior)
  without re-deriving prior findings — each correction was applied as a targeted diff against the
  same in-progress edit, not a restart, matching this project's general no-worktree-isolation
  stance for sequential work: keep working the same code state, don't throw away and redo.

## 3. Context-Cost Assessment (for `harnez distill` / `harnez rate`)

Requested explicitly for this session: which tool calls put a disproportionate amount of text into
the orchestrator's context, as input for improving `harnez distill`'s autopipe coverage and
`harnez rate`'s signal. **Caveat up front**: harnez currently has no per-tool-call token-count
telemetry surfaced back to the model mid-session (see [[142-disable-rate-feedback-and-measure-overhead]],
which already flags this same gap for `harnez rate` specifically) — the ranking below is a
qualitative read of this session's transcript shape, not measured token counts. That measurement
gap is itself the top recommendation below.

Ranked by estimated context contribution, highest first:

1. **Subagent completion-report payloads (the `task-notification` `<result>` blocks).** Each of the
   four dispatched dev subagents (164/168/169, plus the earlier splash-implementation agent)
   returned a multi-paragraph free-text summary — file lists, mechanism explanations, verification
   narratives — that lands verbatim in the orchestrator's context on completion. These were the
   single largest recurring contributor this session, and **neither `harnez distill` nor `harnez
   rate` currently has any reach here**: `distill`'s autopipe hooks rewrite *Bash tool calls*
   ([[066-native-go-command-output-distillation]], [[070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode]]),
   not agent-to-agent task-completion messages, which aren't a tool call at all in the sense those
   tickets scope to. This looks like a genuinely uncovered gap, not an extension of existing work —
   worth a fresh ticket rather than folding into 066/070's scope. Concretely: subagent prompts in
   this session already asked for "report back: what you changed, test results, friction" in
   free-form prose; a structured/capped report format (e.g. a fixed-field summary capped at N
   lines, expanded only on request) would cut this contributor significantly without losing the
   information that actually gets used (file paths, commit hashes, pass/fail).
2. **Full-file `Read` calls on `cmd/harnez/main.go` after the self-inflicted edit mistake (§2).**
   Two ~200-230-line reads of the same file within a few turns, purely to recover from an edit that
   went wrong. This is the clearest "avoidable" cost in the session: a `go build` check immediately
   after the first bad edit would have surfaced the syntax break with a short compiler error instead
   of requiring a full-file read to diagnose visually.
3. **Broad `grep -rn` sweeps across `issues/*.md` and `docs/**/*.md` for `--summary` references**
   (done once, ahead of the flag-redesign implementation, to confirm scope before a breaking
   change). Returned ~80 matches. This was a deliberate, one-time due-diligence check before a
   breaking CLI change, not a repeated pattern — high-value despite the size, so not a strong
   candidate for distillation, but worth noting as a case where a narrower `grep -l` (file list
   only) then targeted reads might have been cheaper for the same confidence level.
4. **Verbose `go test -v` output** (used a few times to confirm specific new tests by name).
   Consistently piped through `tail -N` or `-run <pattern>` to bound it — this project's existing
   convention worked as intended and didn't show up as a real cost driver this session.
5. **pty-repro script output** (both subagents' and this session's own direct one for the AGY/status
   line fix) — kept deliberately small (printed only the extracted status-line sightings and one
   frame), suggesting this pattern is already reasonably self-disciplined; no action needed.

### Recommendation for the user to decide on

Filing a new issue for item 1 above (subagent report-payload verbosity) rather than bundling it
into [[066-native-go-command-output-distillation]]/[[070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode]],
since those are scoped to Bash tool-call rewriting specifically and this is a different
mechanism (agent-to-agent task completion messages). Left for the user to confirm before filing,
since it implies a new interface contract for subagent report prompts across this project's whole
dispatch pattern, not a local fix.

## 4. What I'd Do Differently

- Run `go build ./...` immediately after any `Edit` that reshapes a function body (adds/removes a
  closing brace, splits a function, moves a `return`), not just after the full batch of changes —
  would have caught the `main.go` mistake in one turn instead of two.
- For the flag redesign (171), state the three-way flag semantics (`--raw` vs `--json` vs default)
  as a truth table in the ticket *before* writing code, rather than deriving it inline while
  editing — would have made the two rounds of user correction cheaper to apply (smaller diffs
  against a clearer starting structure).

## 5. Where I Might Have Missed Project Status

- Did not check whether issue 170 (the concurrent modularization ticket touching `cmd/harnez/main.go`)
  has since landed or is still in flight; the flag-redesign work in this session (171) touched the
  same file and could conflict with 170's restructuring if it lands with stale line references.
  Worth a status check before either is picked up again.
- Did not verify against a live AGY account with a fully-consumed weekly window for issue 172 —
  filed as a hypothesis with an explicit "needs live confirmation" caveat, not a verified root
  cause.
