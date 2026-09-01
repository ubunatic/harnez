# 160 — Feasibility: Split `usage --watch` Viewer From App-Logic Server

**Status**: Open — feasibility assessment, no implementation decision yet
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Architecture
**Related**: [[082-agent-usage-collector-daemon]] (existing precedent: collection already runs as an
  independent, always-on process writing snapshots the TUI reads), [[110-remote-load-batch-vs-streaming-collection-modes]]
  (existing precedent: a long-lived side-channel — SSH `ControlMaster` — managed alongside the
  `--watch` process lifecycle), `internal/usage/watch.go` (`screenFrame`, `RunWatchWithOptions`)

## Problem

Developing `harnez` itself (its own usage/watch/collector code) currently requires restarting
`harnez usage --watch` every time the binary is rebuilt, because `--watch` is one monolithic
process: terminal I/O (raw mode via `stty`, SIGWINCH resize, key input), the redraw loop, and all
app logic (collectors, frame layout, remote streaming) are compiled into a single binary and run
in one process (`RunWatchWithOptions`, `watch.go:1718`). A code change to any of it means killing
and relaunching the terminal session that's watching it.

Desired: keep the terminal viewer attached (or reconnecting) across `harnez` rebuilds, so
iterating on the app logic doesn't interrupt the live dashboard.

## Findings

1. **The render output is already a clean, serializable boundary.** `screenFrame`
   (`watch.go:330-334`) is just `{lines []string, cols, rows int}` with a `paint(out io.Writer)`
   method (`watch.go:347-358`) that writes ANSI cursor-home + per-line content + erase-to-EOL. A
   `screenFrame` is trivially serializable (newline-joined text, or a JSON array of lines) and
   `paint` doesn't care whether `out` is a local terminal or a `net.Conn` — this is the natural
   seam for a client/server split.

2. **A "server" precedent already exists.** Issue 082 shipped `harnez agent-collector`: a
   long-running process that collects and writes atomic JSON snapshots
   (`~/.local/state/harnez/agents/usage/<agent>.json`) independent of any TUI being open. It
   already decouples *data collection* from *display* — but not *frame building* (layout,
   `buildWatchFrameAt`, section state, hotkeys) from *display*, which is the piece this ticket is
   about.

3. **`RunWatchWithOptions` currently owns everything in one goroutine tree**: `stty` raw-mode
   setup/teardown, SIGWINCH-driven resize, a `fetchChan`/`redrawChan` pair, key-dispatch
   (`dispatchWatchKey`), section-preset state, the remote-Load streaming manager (issue 110's
   SSH `ControlMaster`, already a long-lived side-process managed via the same lifecycle/defer
   pattern this ticket would need to generalize), and finally `renderFrame()` → `screenFrame.paint`.
   Splitting cleanly means drawing a line between "owns terminal, owns keys, owns local state
   like section presets/scroll position" (viewer) and "owns collectors, owns layout, computes
   `screenFrame`" (server).

4. **Local-only, not a network service.** This only needs a Unix domain socket (or the existing
   snapshot-file directory, polled) — no auth/remote-exposure design burden, unlike issue 035's
   proxy sidecar idea. A `harnez usage --watch` invocation would: try to connect to a local
   socket; if nothing's listening, spawn (or instruct the user to run) `harnez usage serve` and
   connect; render frames pushed from the server; on disconnect, show a "reconnecting…" frame
   and retry rather than exiting.

5. **What stays in the viewer vs. moves to the server** (rough split, to be firmed up if this
   proceeds to a real spec):
   - Viewer (should change rarely, so restarting the server doesn't require restarting this):
     raw-mode terminal setup, SIGWINCH → terminal size, keypress capture, reconnect loop,
     `screenFrame.paint`.
   - Server (restartable independently during development): collectors, `buildWatchFrameAt`,
     section-preset/hotkey state (or: hotkeys stay client-side and are sent to the server as
     small messages — needs a decision, since some state like debug-overlay toggle is purely
     cosmetic and could live in either place).
   - Ambiguous, needs a decision during spec: remote-Load SSH streaming (issue 110) — probably
     belongs on the server side since it's collection, not rendering.

## Feasibility Verdict

Feasible, moderate-sized refactor, no fundamentally new mechanism required — reuses two patterns
already proven in this codebase (082's always-on background process, 110's persistent side-channel
managed across the watch process's lifetime). Main design work is genuinely deciding the
viewer/server message boundary (frame-only push, vs. also forwarding keys/hotkeys to the server)
and the reconnect/spawn UX, not proving the split is possible.

## Desired Outcome (if pursued)

A follow-up implementation ticket, once the message-boundary question above is settled, covering:
- `harnez usage serve` (or folded into `agent-collector`) exposing `screenFrame`s over a local
  Unix socket.
- `harnez usage --watch` becomes a thin client: terminal I/O + reconnect loop + `paint`.
- Rebuilding/restarting the server does not kill the viewer; the viewer shows a reconnecting
  state and resumes automatically once the server is back.

## Architecture Decision (2026-09-01)

Considered folding the frame-building "server" (and issue 110's remote-Load `ControlMaster`,
currently owned by the `--watch` process) into `agent-collector` (082) to avoid running a third
long-lived process. **Decided against it** — keep collection (082), remote streaming (110), and
the new watch-server concerns separate rather than converging them into one daemon. 082 was
deliberately scoped as collection-only, not display; keeping the split preserves that boundary
instead of growing the collector into a presentation-adjacent process. A follow-up implementation
ticket for the viewer/server split should treat the watch-server as its own process, not an
extension of `agent-collector`.

## Acceptance Criteria (for this feasibility ticket)

- [x] Confirm whether a clean viewer/server seam exists in the current code (yes — `screenFrame`).
- [x] Identify what precedent already exists for a long-lived, independently-restartable process
      (yes — issue 082's collector daemon, issue 110's SSH `ControlMaster` manager).
- [x] Identify the open design questions a real implementation ticket would need to resolve
      (message boundary for keys/hotkeys; where remote-Load streaming lives).
- [ ] Decide whether to proceed to an implementation ticket (user call, not made here).
