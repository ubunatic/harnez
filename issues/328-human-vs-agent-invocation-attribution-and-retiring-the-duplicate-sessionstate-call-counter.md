# 328 — Human vs agent invocation attribution and retiring the duplicate sessionstate call counter

**Status**: Closed — resolved in ef5dd30
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Refactor
**Related**: issues/326 (`cli_invocations`), issues/327 (`harnez log`), issue 183 (sessionstate), issue 121 (session resolution), issues 179/181/186/187/188 (gap-tip heuristics)

---

## 1. Problem & Motivation

The project owner's framing: *"we could track all harnez calls, including ones run directly
by the human user outside of an agent session — that would tell us what a user actually did,
not just what an agent did."*

**Good news from the code audit: this is mostly already solved, and cheaper than it sounds.**

- `resolve.Session` (`internal/resolve/resolve.go`) does **not** require an agent session
  id. After the `SessionEnvVars` lookup fails it falls back to a PPID-keyed lock file with a
  30-minute sliding inactivity window, minting ids shaped `ppid-<hash>-<unix>`. A bare human
  terminal invocation therefore already resolves a stable session id, and `sessionTipHook`
  already records it.
- `detectAgent` (`cmd/harnez/rate.go:43`) already returns the literal string `"unknown"`
  when neither `HARNEZ_AGENT` nor any `rateAgentEnvVars` entry is set — which is precisely
  the human-terminal case.

So attribution is two existing signals away, not a new identity concept. What is missing is
(a) treating `"unknown"` as a deliberate, documented `human` classification rather than an
accidental sentinel, and (b) resolving the resulting duplication with `sessionstate`.

## Why this needs real work, not a quick patch

- **`"unknown"` is currently a shrug, not a classification.** `detectAgent` returns it for
  "no agent env var found", which conflates *a human at a terminal* with *an unrecognised
  agent that simply doesn't export a session/agent env var*. The `SessionEnvVars` list itself
  documents that `CODEX_SESSION_ID` and `ANTIGRAVITY_SESSION_ID` are **unconfirmed guesses** —
  so a real Codex invocation today may well land in the same `"unknown"` bucket as a human
  one. Shipping `harnez log --human` on top of that sentinel would silently mislabel agent
  traffic as user traffic, which is exactly the statistic the owner wants to trust. A second,
  independent signal is needed to disambiguate: the `ppid-` session-id prefix (present only
  when no agent env var supplied an id) combined with TTY detection on stdin. Classify as
  `human` only when both agree, and keep a third `unknown` bucket for the disagreement case
  rather than forcing a binary.
- **Two stores would now count the same thing.** `internal/sessionstate` persists
  `map[subcommand]{Count, LastAt}` plus `Total`, `FirstCallAt`, `LastRateAt`,
  `TotalAtLastRate`, `TotalAtLastTip` per session into `~/.harnez/sessions/*.usage.json`. Once
  issue 326 lands, `cli_invocations` holds a strict superset of the *counting* half of that
  (`Total` and `Calls` become `SELECT count(*) ... GROUP BY command`). Leaving both is a
  guaranteed drift source. But sessionstate is **not** purely derivable: `TotalAtLastTip` is
  tip-emission bookkeeping that exists nowhere else, and `LastRateAt`/`TotalAtLastRate` are
  written by `Record`'s `subcommand == "rate"` branch. The refactor is therefore a genuine
  split, not a deletion.
- **The gap-tip heuristics are load-bearing and heavily tuned.** `GapTip` encodes five
  tickets' worth of tuning (179, 181, 186, 187, 188) across count thresholds, wall-clock idle
  thresholds, a spec-driven summary reminder, and a priority ordering its doc comment spells
  out in detail. Any change to where its inputs come from must be proven non-regressive
  against the existing `sessionstate_test.go` suite, unchanged. Notably, sourcing counts from
  SQLite would also give `sessionstate` a DB dependency it explicitly avoids today ("it has no
  DB dependency and stays a pure, easily-unit-tested package") — so counts must be passed in
  as plain values at the boundary, exactly as `unratedFailures` already is.

## 2. Technical Specification / Findings

### Attribution classification

Add a small, independently-tested classifier producing one of `agent:<id>` / `human` /
`unknown`:

| Agent env var present | Session id prefix | stdin is a TTY | Result |
|---|---|---|---|
| yes | (any) | (any) | `agent:<id>` |
| no | `ppid-` | yes | `human` |
| no | `ppid-` | no | `unknown` (non-interactive: script, CI, unrecognised agent) |
| no | other | (any) | `unknown` |

Stored in `cli_invocations.agent_id` (issue 326) using this vocabulary, so `harnez log
--human` / `--agent` filter on a documented value rather than a sentinel.

### sessionstate consolidation

- Keep `internal/sessionstate` as the owner of *tip bookkeeping* (`TotalAtLastTip`,
  `LastRateAt`, `TotalAtLastRate`) — the fields with no other home.
- Source `Total` / per-subcommand counts from `cli_invocations` and pass them into
  `sessionstate` as plain values at the `sessionTipHook` boundary, preserving the package's
  no-DB-dependency property.
- Retain graceful degradation: if the telemetry DB is unavailable, the tip logic must keep
  working off the JSON file alone. Per the project's resilient-resolution rule, nothing here
  may hard-error.

## 3. Implementation & Verification Plan

### Proposed scope

- Attribution classifier + unit tests, consumed by issue 326's write path.
- Document the `agent:<id>` / `human` / `unknown` vocabulary wherever `detectAgent` is
  documented, and note the `"unknown"`-sentinel ambiguity it resolves.
- Split `sessionstate` inputs as above; no change to `GapTip`'s thresholds or priority order.
- `harnez log --human` / `--agent` filters (implemented in issues/327, semantics defined here).

### Verification

- [ ] Classifier returns `agent:claude` when `CLAUDE_CODE_SESSION_ID` is set, regardless of
      TTY state.
- [ ] Classifier returns `human` only for a `ppid-`-prefixed session id **with** a TTY, and
      `unknown` for a `ppid-` id without one — asserts the Codex/CI conflation is avoided.
- [ ] A real `harnez status` run from a bare terminal produces a `cli_invocations` row
      classified `human` (live check, not only a fixture — per AgenticLoop's
      "Live/Real-Environment Verification for env-resolution features" gate).
- [ ] `sessionstate_test.go` passes **unchanged** after the input-sourcing refactor.
- [ ] With the telemetry DB deleted or unwritable, gap tips still fire correctly from the
      JSON file alone.
- [ ] `harnez log --human` and `--agent claude` partition the same unfiltered result set
      with no overlap and no lost rows (modulo `unknown`).
- [ ] `go test ./...` passes.
