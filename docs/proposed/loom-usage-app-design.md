# Loom Usage Monitor (`harnez usage --loom`) Specification & Design

## 1. Overview & Objectives

The goal is to provide a `--loom` option for `harnez usage` that runs the usage monitor as a `codeberg.org/ubunatic/loom` TUI application.
- **Default View**: When `--loom` is passed, the monitor defaults exclusively to the **compact view** (`¹ All Usage` and `⁷ Load` boxes).
- **One-shot vs. Interactive TUI**:
  - `harnez usage --loom` (without `--watch`): Performs a single collection pass and prints the compact Loom widget once to stdout.
  - `harnez usage --loom --watch`: Opens an interactive Loom `Pane` rendering the live TUI watch monitor.
- **Legacy Fallback**: When `--loom` is omitted, the full legacy `harnez usage` application remains active as the default behavior.

---

## 2. Requirements & Expected UI

### 2.1 UI Layout
The `--loom` compact layout renders the header and two side-by-side boxes across the terminal width (`r.W`):

```
Agentic usage  22:36:48 CEST   history: 3 files (1.7 MB)   5 hidden (press ? for controls)

┌─ ¹ All Usage ────────────────────────────────────┐ ┌─ ⁷ Load ────────────────────────────────────┐
│ Claude Code   [⣿⣿  ] 60% 3d20h   [⣿⣿  ] 59% 1h3m │ │ cpu (12 cores)   [⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀] 2% (60°C)     │
│ Gemini        [⣿⣿⣿⡇] 97% 12h46m  [    ] 0% 4h59m │ │ ram (23G)        [⣤⣤⣤⣤⣤⣤⣤⣤⣤⣤] 5.9/23.3G 25% │
│ Claude/GPT    [⣿⣿⣿⡇] 92% 4d21h   [    ] 0% 4h59m │ │ gpu (Cezanne)    [⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀] 0% (47°C)     │
│ OpenAI Codex  [⣿⣿⣿⡇] 99% 22h5m   [    ] 4% 4h19m │ │ gpu vram/gtt     [⣤⣤⣤⣤][⣀⣀⣀⣀] 3.8/27.0G 14% │
└──────────────────────────────────────────────────┘ └─────────────────────────────────────────────┘
```

### 2.2 Functional Requirements
1. **Responsive Box Flow**:
   - `UsageLoomWidget.Draw` must use the canvas region width (`r.W`) to allow boxes to flow naturally across the full screen width.
2. **Animated Splash Screen**:
   - In watch mode (`--watch`), startup displays an animated progress splash screen while the initial data collection takes place.
3. **Interactive Hotkey Support**:
   - Keypresses in the Loom TUI pass through `dispatchWatchKey` to support all standard controls:
     - `?`: Open/close controls overlay.
     - `m` / `M`: Cycle view presets.
     - `r` / `R`: Toggle remote/local host.
     - `1`–`8` / `a`, `c`, `g`, `o`, `h`, `p`, `l`, `m`: Toggle individual boxes.
     - `!`: Toggle freshness debug overlay.
     - `l`: Open fetch diagnostics.
     - `q`, `Esc`, `Ctrl-C`: Quit.
4. **Live Refreshes & Tickers**:
   - Background data collection updates `UsageSummary` and rate history (`rateTracker`), triggering redraws on Loom cadence ticks (`loom.Cadence`).

---

## 3. Architecture & Implementation Changes Needed

### 3.1 Dependencies
- Add Loom dependency in `go.mod`:
  `codeberg.org/ubunatic/loom v0.2.5`

### 3.2 CLI Flag Wiring (`cmd/harnez/main.go`)
- Register `--loom` flag on `usageCmd`:
  `usageCmd.Flags().BoolVar(&usageLoom, "loom", false, "start usage monitor as loom app (compact view)")`
- Update `validateUsageFlags` to reject invalid flag combinations:
  - `--loom` with `--raw` (rejected: `--loom has no effect with --raw`).
  - `--loom` with `--json` (rejected: `--loom and --json cannot be combined`).
- Dispatching in `usageCmd.RunE`:
  ```go
  if usageLoom {
      loadOpt := usage.WatchOptions{
          Compact:        true,
          ShowProcesses:  usageProcesses,
          ShowMic:        usageMic,
          RemoteLoadHost: loadWatchHost,
          Host:           usageHost,
      }
      if usageWatch {
          return usage.RunLoomWatch(ctx, "", client, cmd.OutOrStdout(), usageInterval, "", loadOpt)
      }
      return usage.RenderLoomPrint(ctx, "", client, cmd.OutOrStdout(), loadOpt)
  }
  ```

### 3.3 Loom Widget (`internal/usage/loom.go`)
- Implement `UsageLoomWidget` implementing `loom.Widget`:
  - Fields: `summary UsageSummary`, `rates map[string]agentRate`, `st watchKeyState`, `Opts WatchOptions`, `HomeDir string`, `Now time.Time`, `Live bool`, `splashActive bool`, `splashElapsed time.Duration`, `OnFetch func()`.
  - `Draw(c *loom.Canvas, r loom.Rect)`:
    - If `splashActive`, renders `buildSplashFrame(r.W, r.H, ...)` to `c`.
    - Otherwise, renders `buildWatchFrameAt` with `w.st.sec` and width `r.W`, writing each line to `c.WriteANSI(r.X, r.Y+i, line)`.
  - `HandleKey(e loom.KeyEvent) bool`:
    - Converts `e` to hotkey byte.
    - During splash: `dispatchSplashKey(b)` (Esc skips splash, Ctrl-C quits).
    - During watch: `dispatchWatchKey(w.st, b, w.Opts.ShowProcesses)` updates `w.st` and triggers `OnFetch()` or quits.
  - `ContentHeight() int`:
    - Returns the content height required for the compact frame (e.g. 8 lines for static, 14 lines for watch).

### 3.4 Execution Functions
- `RenderLoomPrint(ctx, homeDir, client, out, opts)`:
  - Non-interactive one-shot print. Calls `loom.Render` on `UsageLoomWidget` and writes output lines to `out`.
- `RunLoomWatch(ctx, homeDir, client, out, interval, historyDir, opts)`:
  - Interactive watch mode. Runs initial collection with animated splash screen, opens `loom.New(height)` pane, and drives collection/redraw via `pane.RunWatch(ctx, widget, cadence, collectFunc)`.

---

## 4. Testing Strategy

1. **Widget UI Verification (`internal/usage/loom_test.go`)**:
   - Construct a mock `UsageSummary` with Claude, Antigravity (Gemini & Claude/GPT model groups), and OpenAI Codex.
   - Render with `loom.Render(widget, cols, rows)`.
   - Verify rendered ANSI text contains `Agentic usage`, `¹ All Usage`, `⁷ Load`, all model group names, and CPU/RAM rows.
2. **Hotkey Handling Unit Tests**:
   - Test `HandleKey` with `?` (toggles overlay), `q` (quits), `m` (cycles preset), and splash dismissal.
3. **CLI Integration Tests (`cmd/harnez/usage_test.go`)**:
   - Verify `validateUsageFlags` handles `--loom` combinations correctly.
   - Test `harnez usage --loom` Cobra command execution in offline mode.
