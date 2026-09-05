# 160 — Feasibility: Extract `usage --watch` Layout/UI/Keyboard Into a Renderer-Agnostic Module

**Status**: Open — feasibility assessment, no implementation decision yet
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Architecture
**Related**: [[082-agent-usage-collector-daemon]] (existing precedent: collection already runs as an
  independent, always-on process writing snapshots the TUI reads), [[161-collector-remote-control-host-and-prometheus-exposition]]
  (sibling ticket: opt-in HTTP `/metrics` exposition on `agent-collector` — the HTTP-serving
  precedent this ticket's HTML idea would reuse), `internal/rograph/*` (bar/sparkline renderers —
  currently bake ANSI directly into returned strings), `internal/usage/watch.go` (`screenFrame`,
  `buildWatchFrameAt`, `dispatchWatchKey`, `RunWatchWithOptions`)

## Rescope (2026-09-01)

Originally scoped as a client/server *process* split (thin terminal viewer + a server owning
collectors/frame-building, connected over a local socket, purely to survive `harnez` rebuilds
without killing the terminal session). Rescoped, per discussion, to something broader: extract
layout, UI structure, and keyboard-control handling into a module that is renderer-agnostic —
not committed to terminal/ANSI output — so that module *could* run inside a server and be
rendered as HTML (e.g. custom elements for bars/sparklines) and served over HTTP, the same way
161's collector will serve Prometheus metrics. The original "just keep the terminal viewer alive
across rebuilds" goal is still in scope, but now as one possible renderer/transport (terminal)
rather than the whole design.

## Problem

Two compounding problems, not just one:

1. Developing `harnez` itself requires restarting `usage --watch` on every rebuild, because
   viewer (terminal I/O) and app logic (collectors, layout, keyboard dispatch) are one process
   (`RunWatchWithOptions`, `watch.go:1718`) — this is the original 160 problem, still valid.
2. The dashboard is terminal-only. There's no way to view the same data in a browser, and no
   shared representation that a hypothetical HTML view and the terminal view could both render
   from — today "layout" and "ANSI rendering" are the same code, not two separable steps.

## Findings

1. **Layout and ANSI presentation are NOT currently separable — this changes the original
   feasibility finding.** The prior version of this ticket noted `screenFrame`
   (`watch.go:330-334`, `{lines []string, cols, rows}`) as "already a clean serializable
   boundary." That's true only for a terminal-to-terminal split. `screenFrame.lines` are fully
   ANSI-baked strings by the time they exist: `rograph.RenderBar`/`RenderSparkline`
   (`internal/rograph/options.go`) embed raw `\x1b[...m` escape sequences directly into the
   returned string, and `watch.go`'s line-builders (`formatGPULine`, `formatGPUMemoryLines`,
   `formatAllUsageTableLine`, etc.) `fmt.Sprintf` those ANSI-laden bar strings together with
   labels and text in one step. There is no intermediate structured form (e.g. a widget tree of
   "label + bar(value, max, color) + trailing text") — layout and terminal-specific styling
   happen together, at construction time, throughout `watch.go`. An HTML renderer cannot reuse
   any of this as-is; it would need to parse ANSI back out, which is the wrong direction.

2. **What a renderer-agnostic module would actually require**: introducing a structured
   intermediate representation — a small widget/row model (e.g. `Row{Label string, Bars []Bar,
   Sparkline []float64, Trailing string}`, mirroring what `internal/rograph` already computes
   numerically before it stringifies to ANSI) that `buildWatchFrameAt` produces instead of
   `screenFrame`. Two independent renderers then consume that model: the existing ANSI terminal
   painter (today's `rograph` + `screenFrame.paint`), and a new HTML renderer (could use plain
   `<div>`/`<progress>`-style markup, or custom elements like `<harnez-bar value="63" max="100">`
   for the browser to style/animate client-side). This is a real refactor of `internal/rograph`
   and every `watch.go` line-builder, not a thin wrapper — bigger than the original process-split
   scope.

3. **Keyboard control has the same shape problem.** `dispatchWatchKey` (`watch.go:474`) takes a
   raw input byte read from the terminal (`stty cbreak` mode) and mutates `watchKeyState`
   in-process. Serving the same controls over HTTP (e.g. a browser view with clickable
   panel-toggle buttons) means keyboard dispatch also needs an transport-agnostic input event
   (already true in spirit — `dispatchWatchKey` is already a "pure function of key + state", per
   its own doc comment at `watch.go:432` — the gap is only that its *source* is hardcoded to a
   local terminal read loop, not that its logic is coupled to the terminal).

4. **HTTP-serving precedent already exists as of 161.** `agent-collector`'s planned opt-in
   `/metrics` endpoint (161) establishes the pattern: an existing headless daemon gains an
   optional HTTP listener, off by default. A hypothetical HTML dashboard view would reuse that
   same opt-in-HTTP posture rather than inventing a new one — but per the 2026-09-01 decision
   below, it should NOT reuse the *same process* as the collector.

5. **Where this module would run**: still a server process, still separate from
   `agent-collector` (per the standing decision below), reading data the same way `--watch`
   does today (082's snapshot files / whatever 161 exposes) and producing the structured row
   model. Both a terminal client and an HTTP/HTML client would connect to it — terminal over a
   local socket (original scope, unaffected by the HTML idea), browser over HTTP.

## Feasibility Verdict

The terminal-only viewer/server split (original 160 scope) is still feasible as previously
assessed — unaffected by this rescope. The renderer-agnostic layout module needed to also support
an HTML view is a materially larger refactor: `internal/rograph` and most of `watch.go`'s line
formatters would need to stop baking ANSI at construction time and instead emit a structured row
model, with ANSI-terminal and HTML as two thin renderers over that model. Feasible, but should be
staged: (a) extract the structured model and keep only the terminal renderer working from it
first — this alone unblocks the original rebuild-without-restart goal — then (b) add an HTML
renderer and HTTP transport as a second phase, once the model has proven itself against the one
real consumer (the terminal) it needs to support today.

## Desired Outcome (if pursued)

Phase 1 (was the whole ticket before rescope): thin terminal viewer + server over a local socket,
using a new structured row model instead of pre-rendered `screenFrame` strings internally, but
still rendering ANSI as the only output for now.

Phase 2 (new, from this rescope): an HTML renderer consuming the same row model, served over
HTTP by the same server process (not `agent-collector` — see decision below), with keyboard-style
controls exposed as clickable/toggleable browser controls dispatched through the same
`dispatchWatchKey`-shaped, transport-agnostic input handling.

## Architecture Decision (2026-09-01)

Considered folding the frame-building "server" (and issue 110's remote-Load `ControlMaster`,
currently owned by the `--watch` process) into `agent-collector` (082) to avoid running a third
long-lived process. **Decided against it for the frame-building/display piece** — this ticket
(160) stays scoped to the viewer/layout/HTML-rendering split only, kept separate from
`agent-collector`, which stays collection-only, not display. This still holds after the rescope:
the HTML/HTTP rendering piece added here is a *display* concern (like the terminal renderer it
sits alongside), not a *collection* concern, so it stays out of `agent-collector` for the same
reason the original frame-building server did.

The remote-Load `ControlMaster` piece was reconsidered separately and **is** being merged into
`agent-collector`, since it's data-gathering, not display — tracked in
[[161-collector-remote-control-host-and-prometheus-exposition]], split out from this ticket so 160
doesn't mix rendering concerns with collector-architecture changes.

## Acceptance Criteria (for this feasibility ticket)

- [x] Confirm whether a clean viewer/server seam exists in the current code for the
      terminal-only split (yes — `screenFrame`, unaffected by the rescope).
- [x] Confirm whether that same seam supports a renderer-agnostic (e.g. HTML) view (no — ANSI is
      baked in at construction time throughout `rograph` and `watch.go`'s line-builders; a real
      structured row model would need to be introduced first).
- [x] Identify what precedent exists for opt-in HTTP serving on a headless daemon (161's planned
      `/metrics` endpoint).
- [x] Confirm the display-vs-collection process boundary decision still holds after the rescope
      (yes — HTML rendering is display, stays out of `agent-collector`).
- [ ] Decide whether to proceed to an implementation ticket, and whether to stage it (terminal
      split first, HTML renderer second) or attempt both together (user call, not made here).

---

## Implementation Plan

This is a feasibility ticket whose research ACs are already checked. The only open AC is the
**proceed/stage decision, which is explicitly a user call**. So this plan covers (a) closing the
assessment out and (b) the concrete staging sketch the user needs in order to make that call — not
an implementation to start now.

### Line-number corrections (2026-09-04 re-verification)

The Findings section's references have drifted; correct them when this ticket is next touched:

| Ticket says | Actual (`internal/usage/watch.go`, 2688 lines) |
|---|---|
| `screenFrame` at 330-334 | `type screenFrame` at **337**, `paint` at **354** |
| `dispatchWatchKey` at 474 | **481** |
| `RunWatchWithOptions` at 1718 | **2176** |
| — | `buildWatchFrameAt` at **1632** |

### One finding to add before deciding

`internal/rograph/row.go` is **already renderer-agnostic**: `PadLabel` and `RowLayout` are pure
width arithmetic returning `(barWidth, keepTrailing)` with no ANSI. The ANSI baking is confined to
`RenderBar` (`options.go:180`, escapes at 235) and `RenderSparkline` (`options.go:257`, escapes at
290-304). That narrows Finding 2's "real refactor of `internal/rograph`" claim considerably: the
*geometry* layer already separates cleanly; what is entangled is the glyph+color stringification
and `watch.go`'s ~15 `format*Line` builders that `Sprintf` those strings together. Worth recording,
because it lowers the estimated cost of Phase 1 relative to the current verdict.

### Steps to close this ticket

1. **Add the corrected line numbers and the `row.go` finding above** into the Findings section
   (or leave them here — they are recorded either way).
2. **Enumerate the actual blast radius** so the user's decision is cost-informed, not vibes-based:
   `grep -c "rograph\." internal/usage/watch.go` and a list of the `format*Line` functions that
   would need to emit a model instead of a string (`formatAllUsageTableLine` :762 and its two
   variants :766/:777, `formatGPULine` :1100, `formatGPUMemoryLines`, …). One paragraph, one list.
3. **Present the staged option to the user** as the ticket's Feasibility Verdict already
   recommends, with the two phases costed separately:
   - **Phase 1** — introduce a structured row model (`Row{Label string, Bars []Bar, Sparkline
     []float64, Trailing string}`) produced by `buildWatchFrameAt`, with the existing ANSI painter
     as its only renderer. `screenFrame` becomes `[]Row` + dims; `paint` becomes the ANSI renderer.
     Unblocks the original rebuild-without-restart goal (terminal viewer + server over a local
     socket, serializing `[]Row` instead of pre-baked strings). Independently valuable even if
     Phase 2 never happens.
   - **Phase 2** — an HTML renderer over the same model, served over HTTP by the same server
     process (not `agent-collector` — per the standing 2026-09-01 decision), with browser controls
     dispatched through the already-pure `dispatchWatchKey` (`watch.go:481`) fed from a
     transport-agnostic input event instead of a raw terminal byte.
4. **Record the decision in this ticket**, tick the last AC, and set Status to
   `Closed — assessment complete, implementation tracked in NNN` (filing the implementation
   ticket(s) only if the user says proceed). If the user defers, mark
   `Blocked — awaiting proceed decision` rather than leaving it plain Open.

### Design decisions / tradeoffs (for the eventual implementation ticket)

- **Stage it.** Phase 1 has one real consumer (the terminal) that can validate the model
  immediately; Phase 2 has zero consumers until it is built. Building both at once means designing
  a model against a hypothetical second renderer — the classic premature-abstraction trap.
- **Regression guard is the deciding practical risk**, not the refactor itself: `--watch` output is
  ANSI-exact and easy to break invisibly. There is already a pty canary (`canary-watch-pty`) and
  `internal/rograph` has real tests (`options_test.go`, `rograph_test.go`, `row_test.go`) — Phase 1
  should start by capturing golden ANSI frames through the existing pty harness, then refactor
  until the goldens still match byte-for-byte. Do that first, not last.
- **Do not fold into `agent-collector`** — decision already made and re-confirmed after the
  rescope; the implementation ticket must restate it so it is not silently relitigated.
- Order relative to [[161]]: 161 moves the remote-Load `ControlMaster` out of
  `RunWatchWithOptions`, which shrinks the very function Phase 1 restructures. **Do 161 first** —
  otherwise both tickets edit the same lifecycle wiring.

### Risks / open questions

- Sequencing conflict with [[161]] (above) — the single most actionable scheduling note here.
- `buildWatchFrameAt` takes 12 parameters (`watch.go:1632`); introducing a row model without also
  addressing that signature will make the refactor uglier than the current code. Decide up front
  whether to bundle the params into an options struct as part of Phase 1 or explicitly defer it.
- Phase 2's value is unvalidated — no one has asked for the browser view except as an idea. It may
  be worth building Phase 1 and stopping.

### Scope estimate

- Closing **this** ticket (steps 1-4): **small** — a research write-up and a user decision.
- Phase 1, if approved: **medium** (row model + ANSI renderer + golden-frame guards, ~15 line
  builders touched, one file).
- Phase 2, if approved: **large** (HTML renderer, HTTP transport, input-event abstraction, plus a
  second UI to keep in sync with the terminal one indefinitely).
