# 187 — Periodic "Summary: X tool calls in `<dur>`" reminder (spec-first)

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[179-harnez-rate-ok-heartbeat]] (the `harnez rate --ok` call this reminder
points agents at), [[186-time-based-gap-tip-alongside-call-count]] (the time-dimension
this reminder's trigger condition likely shares), [[183-session-state-assessment-and-reminders]]
(`internal/sessionstate` — existing gap-tip mechanism this composes with)

## Problem

The existing gap-tips (issues 183/179/186) are short single-line nudges
("no rate call in N calls/T time — consider `harnez rate --ok`"). The user
wants a distinct, occasional **summary-style** reminder instead of/alongside
that — something closer to:

> Summary: X tool calls in `<dur>`; call `harnez rate --ok` to confirm
> healthy tool usage.

i.e. giving the agent a concrete count + duration, not just "it's been a
while," to make the ask more concrete and the agent's compliance easier to
self-check ("have I actually confirmed the last X calls or not").

## Task — spec first, then implement

This ticket is deliberately spec-first: **before writing implementation
code, draft a first spec** for this reminder and get it into the `spec/`
directory following this repo's existing spec conventions (`docs/other/Spec.md`
— YAML authoring + a companion JSON Schema under `spec/schemas/`, embedded
into the binary, no hardcoded shadow copies in Go source). Look at the
existing `spec/actions.yaml` / `spec/colors.yaml` / `spec/indicators.yaml` +
their schemas for the house style before drafting a new one (e.g.
`spec/reminders.yaml` or fold into an existing file if genuinely a better
fit — your call, document the reasoning).

The spec should define, at minimum:
- The exact condition(s) under which this summary reminder fires — "X
  unconfirmed calls" needs a concrete definition: is X the same threshold
  as issue 179/186's gap-tips, a distinct (larger?) one, or does this
  reminder replace the plain gap-tip's wording once a bigger threshold is
  hit? Decide and document the reasoning in the spec and the ticket's
  Resolution Note.
- The message template itself (with placeholders for call count and
  duration), so wording changes don't require a Go recompile-and-redeploy
  cycle for pure copy edits.
- How it composes with the opt-out (issue 142's `RateFeedbackDisabled`) and
  the existing gap-tip cooldown (`tipCooldown`) so it doesn't create a third
  independent nag channel that stacks with the other two.

Then wire `sessionTipHook`/`sessionstate.GapTip` (or a clearly-scoped new
function, your call) to read the spec and render this summary reminder per
its defined condition, computing X (call count since last
rate/heartbeat) and `<dur>` (wall-clock elapsed since last
rate/heartbeat — reuse issue 186's timestamp if that ticket has already
landed when you pick this up; otherwise compute your own) from
`internal/sessionstate.State`.

## Acceptance Criteria

1. `spec/` gains a validated (schema-backed) definition of this reminder's
   condition and message template — not hardcoded Go string literals for
   the wording/threshold.
2. The reminder fires per the spec's defined condition, with the actual
   call count and duration correctly substituted.
3. It does not stack with/duplicate the existing plain and heartbeat-specific
   gap-tips in the same nudge slot — composes as one coherent nudge system.
4. Opt-out (issue 142) suppresses this reminder too.
5. Unit tests cover: spec loading/validation, condition-trigger logic,
   message rendering with real numbers, and opt-out suppression.
6. `go test ./...` passes.

## Resolution Note

**Spec shape**: a new `spec/reminders.yaml` + `spec/schemas/reminders.schema.json`
(house style: YAML authoring, JSON Schema validation, embedded via the existing
root `//go:embed spec` directive in `embed.go` — no new embed target needed).
Chose a new file over folding into `spec/actions.yaml`/`colors.yaml`: those two
are about the `usage --watch` TUI specifically (hotkeys, ANSI colors); this is
about the CLI-wide session-tip hook, a different consumer and lifecycle, so a
sibling top-level spec file keeps the "one file per concern" pattern those two
already establish rather than overloading either with an unrelated schema.

The spec defines a `reminders` map (currently one entry, `summary`) with
`title`, `description`, a `message` template (`{calls}`/`{duration}`
placeholders, substituted by `internal/sessionstate.renderReminder`), and a
`threshold_multiplier` + `replaces` pair that encode the condition/composition
decision below. Loader/validator: `internal/sessionstate/remindersspec.go`,
mirroring `internal/usage/actionsspec.go`'s parse-then-index pattern
(`parseRemindersYAML` fails clearly on bad YAML/schema violations, never
panics; `mustRemindersSpec()` is the build-time invariant used only by tests
and available for future callers, while `GapTip` itself calls the
error-tolerant `summaryReminder()` so a corrupted embedded spec degrades to
the pre-existing plain/heartbeat tips instead of ever panicking out of the
"never block a real command" `sessionTipHook` path).

**Threshold/condition decision**: the summary reminder **replaces** the
plain and heartbeat gap-tips' wording once its own, strictly larger threshold
is crossed — it does not stack as a third independent nag. Concretely,
`threshold_multiplier: 4` in the spec is applied against the *existing* base
units (`rateGapThreshold` calls / `rateGapIdleDuration()`) already defined in
`internal/sessionstate/sessionstate.go`, giving an 80-call / 80-minute
summary threshold by default — exactly 2x `heartbeatGapThreshold`'s own 2x
multiple of the plain threshold. Rationale for "replaces, not stacks":
issue 179's heartbeat tip already established that regressing to multiple
concurrent nag channels for the same underlying gap (unconfirmed tool
activity) reintroduces the chattiness issue 181 deliberately narrowed away
from. A session that's ignored both the plain and heartbeat tips doesn't need
a *third* differently-worded reminder stacked on top — it needs the
next-more-specific one to replace what would otherwise repeat. Expressing the
threshold as a multiplier of the existing base units (rather than a new
absolute constant) also means retuning `HARNEZ_RATE_GAP_IDLE_MINUTES` (issue
186) automatically keeps the three-tier ladder's relative spacing intact
without a second, disconnected constant to update in lockstep.

**Composition**: `GapTip` checks the summary condition first (most specific,
since it only fires at the largest gap), then heartbeat, then plain, same as
before issue 187 — all three still share one `tipCooldown` gate, one
`feedbackDisabled` opt-out check, and return at most one line per invocation.
`internal/sessionstate/remindersspec.go`'s `summaryReminder()`/`renderReminder()`
plus `sessionstate.go`'s `GapTip` doc comment spell out the ordering; the
schema's `replaces` enum (`plain_rate_gap`/`heartbeat_rate_gap`) documents the
relationship in the spec itself (informational — `GapTip`'s checking order is
what actually enforces it) so the composition intent isn't only a Go comment.

**Tests**: `internal/sessionstate/remindersspec_test.go` covers spec
load/validation (embedded-spec integrity test mirroring
`TestEmbeddedActionsSpecIsValid`, malformed-YAML and missing/invalid-field
cases), `formatReminderDuration`/`renderReminder` output with concrete
numbers, and `GapTip` condition-trigger/composition/opt-out behavior
(summary supersedes heartbeat once its threshold is crossed, time-only
trigger parity with issue 186, `feedbackDisabled` suppresses it same as the
other two). `go build ./...`, `go vet ./...`, and `go test ./...` all pass.
