# 093 — Usage TUI layout planner and sizing policy

**Status**: Closed — resolved in internal/usage/watch.go integration of internal/uix (see commits below)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: issues/083-usage-tui-self-hiding-auto-discovery.md, issues/084-aggregate-quota-window-box-assessment.md, issues/088-load-panel-ram-vram-gtt-memory.md, issues/089-load-panel-combine-gpu-vram-gtt-row.md

---

## 1. Problem & Motivation

`harnez usage` has accumulated enough panels and modes that the layout rules now feel mostly correct but brittle. The standard summary layout is acceptable, but interactive watch toggles can produce awkward panel packing: a box may stretch far past its content width, while other panel combinations still truncate or wrap unpredictably. The issue is less about any one panel's text and more about the absence of a small layout manager that sizes and wraps boxes consistently.

Observed commands:

- `~/go/bin/harnez usage --summary`
- `~/go/bin/harnez usage --summary --proc`
- `COLUMNS=80 LINES=24 ~/go/bin/harnez usage --summary --proc`
- `stty cols 100 rows 24` then `~/go/bin/harnez usage --watch --compact`
- `stty cols 100 rows 24` then `~/go/bin/harnez usage --watch --compact --proc`
- `stty cols 80 rows 18` then `~/go/bin/harnez usage --watch --proc`
- `stty cols 120 rows 18` then `~/go/bin/harnez usage --watch --compact --proc`
- Interactive toggles in compact watch: `A`, `p`, `l`, `a`, `q`

## 2. Findings

- At 80 columns with `--summary --proc`, quota rows fit only by dropping some time labels, while load rows truncate values such as `24...` and `7%...`. That is acceptable as a fallback, but the policy is implicit and panel-specific.
- At 80x18 with `--watch --proc`, the UI hides two panels as "terminal too short"; the hint is useful, but it only reports a count and does not identify which visible-by-default panels were omitted.
- `--watch --compact --proc` starts with hidden state `[C] [G] [O] [H] [P]`, so the process panel remains hidden even though `--proc` explicitly requested it.
- The standard summary layout is acceptable, but toggled watch states expose odd stretch behavior: hiding usage after entering compact mode can leave the Load box stretched across most of the screen even though its content is narrow.
- Mixed toggle states can make `All Usage` stretch across a long row while the individual agent boxes below form a reasonable three-column row. The content looks tabular, but the box sizing does not respect content/preferred widths.
- Toggling `A`, `p`, `l`, and `a` works, but the packing result depends on special-case state: all-usage can become full-width, processes can be manually added, and hidden-panel hints then switch between explicit panel letters and generic "N panel(s) hidden" height overflow messages.

## 3. Observed Output

ANSI escapes removed for readability; content and box shape are from the tool output.

`~/go/bin/harnez usage --summary`:

```text
Agentic usage  11:43:22 CEST   history: 2 files (1.6 MB)   hidden: [P] [a]

┌─ [C] Claude Code ────────────────────────┐ ┌─ [G] Antigravity (AGY) ──────────────────┐
│ active session · Pro                     │ │ u***l@gmail.com · Consumer               │
│ Wk / 5h    [███░] 86% 7h16m [░░░░] 24%   │ │ model: Gemini 3.7 Flash (Low)            │
│ [T] tok: 2,591,837,184 total             │ │ Gemini     [███░] 97% 2d6h [█░░░] 27%    │
└──────────────────────────────────────────┘ │ Claude/GPT [█░░░] 35% 6d [░░░░] 0% 4h59m │
                                             └──────────────────────────────────────────┘
┌─ [O] OpenAI Codex ───────────────────────┐ ┌─ [H] History ────────────────────────────┐
│ u***l@gmail.com · Plus                   │ │ 2 files · 1.6 MB                         │
│ model: gpt-5.5                           │ │ ~/.claude/harnez/usage-history           │
│ Wk / 5h    [█░░░] 49% 5d7h [██░░] 59%    │ │ [     █] · +0 used · 0/hr                │
└──────────────────────────────────────────┘ └──────────────────────────────────────────┘
┌─ [L] Load ───────────────────────────────┐
│ cpu (12 cores)   [▁▁▁▁▁▁▁▁▁▁] 11% (59°C) │
│ ram              4.6/23.3G 20%           │
│ gpu (Cezanne)    [▁▁▁▁▁▁▁▁▁▁] 3% (48°C)  │
│ gpu mem          1.0/19.6G 5%            │
│ vram/gtt         v0.9/8.0 g0.1/11.6      │
└──────────────────────────────────────────┘
```

`COLUMNS=80 LINES=24 ~/go/bin/harnez usage --summary --proc`:

```text
Agentic usage  11:43:31 CEST   history: 2 files (1.6 MB)   hidden: [a]

┌─ [C] Claude Code ───────────────────┐ ┌─ [G] Antigravity (AGY) ─────────────┐
│ active session · Pro                │ │ u***l@gmail.com · Consumer          │
│ Wk / 5h    [███░] 86% [░░░░] 24%    │ │ model: Gemini 3.7 Flash (Low)       │
│ [T] tok: 2,591,837,184 total        │ │ Gemini     [███░] 97% [█░░░] 27%    │
└─────────────────────────────────────┘ │ Claude/GPT [█░░░] 35% 6d [░░░░] 0%  │
                                        └─────────────────────────────────────┘
┌─ [O] OpenAI Codex ──────────────────┐ ┌─ [H] History ───────────────────────┐
│ u***l@gmail.com · Plus              │ │ 2 files · 1.6 MB                    │
│ model: gpt-5.5                      │ │ ~/.claude/harnez/usage-history      │
│ Wk / 5h    [█░░░] 49% [██░░] 60%    │ │ [     █] · +0 used · 0/hr           │
└─────────────────────────────────────┘ └─────────────────────────────────────┘
┌─ [P] Processes ─────────────────────┐ ┌─ [L] Load ──────────────────────────┐
│ 3 active processes                  │ │ cpu (12 cores)   [▁▁▁▁▁▁▁▁▁▁] 5%... │
│ claude: 1  agy: 1  codex: 1         │ │ ram              4.6/23.3G 20%      │
└─────────────────────────────────────┘ │ gpu (Cezanne)    [▁▁▁▁▁▁▁▁▁▁] 2%... │
                                        │ gpu mem          1.0/19.6G 5%       │
                                        │ vram/gtt         v0.9/8.0 g0.1/11.6 │
                                        └─────────────────────────────────────┘
```

Interactive watch toggle example: enter compact mode, press `A`, then hide usage so only Load remains. The Load panel stretches far beyond the content it needs:

```text
┌─ [L] Load ───────────────────────────────────────────────────────────────────────────────────────────────┐
│ cpu (12 cores)   [          ] 5% (55°C)                                                                  │
│ ram              4.8/23.3G 21%                                                                           │
│ gpu (Cezanne)    [          ] 0% (46°C)                                                                  │
│ gpu mem          1.2/19.6G 6%                                                                            │
│ vram/gtt         v1.1/8.0 g0.1/11.6                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

Interactive watch toggle example with `All Usage` plus individual agent panels. `All Usage` stretches across the row while the agent boxes below are compact:

```text
┌─ [a] All Usage ─────────────────────────────────────────────────────────────────────────────────────────┐
│ Claude Code   [    ] 86% 7h10m [    ] 24% 3h10m                                                         │
│ Gemini        [    ] 97% 2d6h  [    ] 28% 3h15m                                                         │
│ Claude/GPT    [    ] 35% 6d    [    ] 0% 4h57m                                                          │
│ OpenAI Codex  [    ] 49% 5d7h  [    ] 62% 3h24m                                                         │
└─────────────────────────────────────────────────────────────────────────────────────────────────────────┘

┌─ [C] Claude Code ─────────────────┐ ┌─ [G] Antigravity (AGY) ─────────┐ ┌─ [O] OpenAI Codex ────────────┐
│ active session · Pro              │ │ u***l@gmail.com · Consumer      │ │ u***l@gmail.com · Plus        │
│ Wk / 5h    [    ] 86% [    ] 24%  │ │ model: Gemini 3.7 Flash (Low)   │ │ model: gpt-5.5                │
│ [T] tok: 2,591,837,184 total ·... │ │ Gemini     [    ] 97% [    ] 28%│ │ Wk / 5h    [   ] 49% [   ] 62%│
└───────────────────────────────────┘ │ Claude/GPT [   ] 35% 6d [   ] 0%│ └───────────────────────────────┘
                                      └─────────────────────────────────┘
```

## 4. Suggested Direction

Add a small layout planner / sizing policy for usage panels rather than a broad TUI rewrite. This can be simple: more like a terminal flex/wrap manager than a full UI framework. Each panel should declare:

- minimum width, preferred/content width, maximum useful width, and whether it can stretch;
- minimum height and optional collapsed height;
- priority when terminal height is constrained;
- whether a CLI flag makes the panel requested even in compact mode;
- a short hidden reason/label for footer hints.

The renderer can then plan rows/columns once, allocate spare width deliberately, and make compact/all-usage behavior a policy decision instead of a cluster of local special cases. A practical target would be:

- prefer concise boxes close to their content width;
- place boxes left-to-right until the next box no longer fits;
- wrap remaining boxes to the next row;
- use two or three columns when that better matches available width;
- avoid stretching a single visible box across the whole terminal unless the panel explicitly benefits from stretch.

## 5. Acceptance Criteria

- `--watch --compact --proc` shows the process panel or clearly documents/renames the flag interaction.
- At 80-column summary/watch widths, truncation is deliberate and consistent across agent, all-usage, process, history, and load panels.
- Height overflow hints identify omitted panel keys when possible, not only the number of hidden panels.
- Compact mode distributes width so `All Usage` does not starve `Load` when both are visible.
- Toggled states with only `Load`, or with `All Usage` plus several agent boxes, keep boxes near useful widths instead of stretching them across arbitrary screen space.
- Layout tests or golden frame checks cover at least 80x18, 80x24, 100x24, and 120x18 with `--summary`, `--summary --proc`, `--watch --compact`, and `--watch --compact --proc`.

## 6. Prototype Layout Library

`internal/uix` now provides a standalone, non-integrated layout prototype for future usage/watch work. It supports enabled/disabled boxes, min/preferred/max useful widths, stretchable boxes, stable priority/order sorting, greedy left-to-right row packing, wrapping, and plain ASCII rendering for tests and demos.

Load-only stays compact in a 100-column terminal because the box is not stretchable:

```text
+ [L] Load ----------------------+
| cpu (12 cores)   [    ] 5%     |
| ram              4.8/23.3G 21% |
| gpu mem          1.2/19.6G 6%  |
+--------------------------------+
```

All Usage and Load share one row when their useful widths fit:

```text
+ [a] All Usage -------------------------------+ + [L] Load ----------------------+
| Claude Code   [    ] 86% 7h10m               | | cpu (12 cores)   [    ] 5%     |
| Gemini        [    ] 97% 2d6h                | | ram              4.8/23.3G 21% |
| OpenAI Codex  [    ] 49% 5d7h                | +--------------------------------+
+----------------------------------------------+
```

All Usage plus three agent boxes avoids a full-terminal All Usage stretch and wraps predictably into one useful-width All Usage row plus a compact three-agent row:

```text
+ [a] All Usage -------------------------------------------------------------------+
| Claude Code   [    ] 86% 7h10m [    ] 24%                                        |
| Gemini        [    ] 97% 2d6h  [    ] 28%                                        |
| OpenAI Codex  [    ] 49% 5d7h  [    ] 62%                                        |
+----------------------------------------------------------------------------------+

+ [C] Claude Code -----------------+ + [G] Antigravity -------------------+ + [O] OpenAI Codex ----------------+
| active session - Pro             | | u***l@gmail.com - Consumer         | | u***l@gmail.com - Plus           |
| Wk / 5h    [    ] 86% [    ] 24% | | Gemini     [    ] 97% [    ] 28%   | | Wk / 5h    [   ] 49% [   ] 62%   |
+----------------------------------+ +------------------------------------+ +----------------------------------+
```

## 7. Verification Notes

Earlier review-only assessment used bounded watch runs with `timeout` and an interactive PTY session exited with `q`; no watch process was left running.

Prototype library verification:

- `go test ./internal/uix`

## 8. Integration (resolved)

`internal/uix` is now wired into `internal/usage/watch.go`'s `buildWatchFrame`, replacing the
ad-hoc `gridColumns`/`onlyAllUsageAndLoad` special-case packing:

- **Flag bug fixed**: `initialWatchSections` no longer returns early for `--compact` before
  checking `ShowProcesses`; `--watch --compact --proc` now shows the Processes panel. Verified
  via `TestCompactWatchSectionsAndAllUsageToggle` and a bounded PTY run at all four required
  terminal sizes.
- **Deliberate, consistent truncation**: every panel's box first measures its own natural
  (untruncated) content width at a generous width, then is capped at `maxPanelContentWidth`
  (80 columns) as a single shared policy — the same truncation path (`renderWBox`'s
  ellipsis-on-overflow) now applies uniformly to Claude/AGY/Codex, All Usage, History, Processes,
  and Load, instead of Load/All-Usage having their own bespoke logic.
- **Height-overflow hints identify panel keys**: `buildWatchFrame` now tracks which specific
  panels (by their `[X]` key) got dropped for lack of vertical room, e.g.
  `… [P] [O] hidden — terminal too short`, replacing the old `N panel(s) hidden` count-only note.
- **All Usage no longer starves Load**: both are sized to their own measured preferred width and
  packed left-to-right by `uix.Layout`, so in compact mode they naturally share one row without
  All Usage claiming the whole terminal width.
- **No more arbitrary stretch**: Load-only and All-Usage-plus-agent-boxes toggle states stay near
  their useful widths (no box in the current panel set declares `Stretch: true`); All Usage width
  is now driven by its own measured content (e.g. 84 columns when showing 2 quota windows per
  agent) rather than a hardcoded 55-column constant or a full-width special case.

Tests added/changed in `internal/usage/watch_test.go`:

- `TestBuildWatchFrameRowsFitWidth` — replaces the old `TestGridColumnsFitsWithGutters` (which
  tested the removed `gridColumns` function directly); asserts no rendered line exceeds the
  terminal's usable width at 80x18, 80x24, 100x24, 120x18 for both default and compact sections.
- `TestBuildWatchFrameLayoutMatrix` — the acceptance-criteria matrix: all four terminal sizes
  crossed with `--summary`, `--summary --proc`, `--watch --compact`, `--watch --compact --proc`
  (16 cases), asserting the row-fits-width invariant, that `--proc` never shows up in the
  toggled-off `hidden:` hint, and that any height-overflow note lists bracketed panel keys, never
  a bare count.
- `TestBuildWatchFrameLoadOnlyDoesNotStretch` and `TestBuildWatchFrameCompactAllUsageDoesNotStarveLoad`
  — direct regressions for Findings #4 and #5.
- `TestCompactWatchSectionsAndAllUsageToggle` updated to assert the fixed `--proc` + `--compact`
  interaction instead of the old (buggy) expectation.

Full verification run:

- `go build ./...`, `go vet ./...`, `go test ./...` — all pass.
- `make check` — passes.
- `make install` — rebuilt `~/go/bin/harnez`.
- Manual renders at `COLUMNS=80/100/120 LINES=18/24 harnez usage --summary[--proc]` — confirmed
  deliberate packing and keyed hidden-panel hints (e.g. `… [O] [H] [L] hidden — terminal too
  short`).
- Bounded PTY runs (Python `pty.fork`, explicit `TIOCSWINSZ`, `q` sent to exit, process confirmed
  reaped via `pgrep`) of `--watch --compact --proc` at all four required sizes, plus an
  interactive toggle sequence (`A`,`p`,`l`,`a`) reproducing the ticket's Findings #5 scenario
  (All Usage + individual agent boxes) — All Usage renders at its natural content width, agent
  boxes pack in a compact grid below, no stretch. No watch process left running after any check.

No deferred work: acceptance criteria in section 5 are fully met by this integration.
