# 997 — Code box in harnez usage for configured repos PR counts and latest PR age

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `spec/actions.yaml`, `internal/usage/localconfig.go`, `internal/usage/watch_frame.go`

---

## 1. Problem & Motivation

When coordinating multi-repo agentic workflows or monitoring background developments, developers need fast situational awareness of pull requests across configured repositories without constantly switching to a web browser or running ad-hoc forge CLI commands.

`harnez usage` (and `harnez usage --watch`) already aggregates system metrics (All Usage, Processes, Load, Mic). Adding a dedicated **Code** box provides instant visibility into open PR volume and whether PRs are recent or growing stale.

## 2. Technical Specification & Design

### 2.1 Configuration (`~/.config/harnez/local.yaml`)
Machine-local configuration lives under a new `code:` section in `local.yaml`:
```yaml
code:
  poll_interval: 5m
  repos:
    - name: harnez
      path: ~/projects/harnez   # auto-resolves git remote origin to forge URL
    - name: uix
      path: ~/projects/uix
    - name: external-repo
      forge: github            # github | codeberg | forgejo
      repo: org/repo
```
*Auto-Hiding*: If `code.repos` is empty or unconfigured, the panel auto-hides completely (matching Mic/GPU auto-hiding).

### 2.2 Data Collection & Background Caching
To maintain the 100ms watch redraw loop without blocking on network I/O:
- Background worker/collector queries open PRs via `gh pr list --state open --json number,createdAt,title` (GitHub) or `fj pr list` / API (Codeberg/Forgejo) at `poll_interval` cadence (default 5m).
- Computes:
  - `Count`: number of open PRs.
  - `LatestAge`: relative age of the newest open PR (`time.Since(newest.CreatedAt)`).
  - `PRsURL`: web URL for the repo's PR list (e.g. `https://github.com/org/repo/pulls` or `https://codeberg.org/org/repo/pulls`).
- Persists to cached snapshot (`$XDG_STATE_HOME/harnez/code-prs-cache.json`).

### 2.3 Visual Layout & Clickable Hyperlinks
Panel layout in `buildCodeBox`:
```text
┌─ [C] Code ────────────────────────────────────────┐
│ harnez              2 PRs   latest: 4h ago        │
│ uix                 0 PRs   -                     │
│ website             1 PR    latest: 3d ago        │
└───────────────────────────────────────────────────┘
```
- **Clickable Hyperlinks**: If stdout supports OSC 8 hyperlinks, wrap the displayed repo name in `\x1b]8;;<PRsURL>\x1b\<name>\x1b]8;;\x1b\` so clicking the name opens the repo's PR list directly in the browser.
- **Coloring**: Dim grey for 0 PRs (`-`), amber highlight if latest PR > 2 days, dim amber for `auth req` / `offline`.

### 2.4 Spec & Hotkey Integration
- Action `toggle_code` registered in `spec/actions.yaml` (`symbol: "[C]"`, hotkey `c` / `C`).
- CLI flag `--code` to force visibility in table and watch modes.

---

## 3. Implementation Plan & Milestones

/goal Implement the Code box in harnez usage displaying configured repo PR counts and latest PR ages with clickable terminal hyperlinks, verified by tests, or stop and report when blocked on user decisions or forge auth limitations.

- **M1: Spec & Local Config**: Add schema to `spec/schemas/local.schema.json`, wire `CodeConfig` in `internal/usage/localconfig.go`, register `toggle_code` in `spec/actions.yaml`.
- **M2: Forge PR Collector & Cache**: Implement background fetcher for GitHub and Codeberg/Forgejo PR metadata with JSON caching and graceful offline handling.
- **M3: TUI Panel & Hyperlinks**: Implement `buildCodeBox` with OSC 8 hyperlink formatting, wire into `watch_frame.go`, and bind `c`/`C` toggle.
- **M4: Tests & CLI Verification**: Unit tests for collector, formatting, hyperlink rendering, golden help update for `--code`.
