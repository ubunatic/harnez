# 141 — Show the running-agent count in the Codex status bar

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `internal/statusline`, `internal/usage/process.go`, [[049-running-agent-processes-watch-panel]], [[095-restore-cwd-after-shell-tool-use-and-statusline-cwd-display]]

---

## 1. Problem & Motivation

Codex's status bar does not show how many coding agents are currently
running. A compact live count would make concurrent-agent activity visible
without opening the usage dashboard.

## 2. Technical Specification / Findings

- `internal/usage/process.go` already exposes `CountRunningAgentProcesses`
  and `AgentProcessCount.Total()` for Claude, AGY, and Codex processes.
- The current `harnez statusline` renderer is Claude Code-specific and only
  renders the working directory. Establish the supported Codex status-bar
  integration point before changing it; do not assume Claude's statusLine
  JSON protocol is accepted by Codex.
- The display should be compact and tolerate a failed process probe by
  retaining the rest of the status bar rather than failing the renderer.

## 3. Implementation & Verification Plan

- Determine and document the supported Codex status-bar customization path.
- Add the total running-agent count to the Codex status bar, using the shared
  process-count implementation rather than a second process scanner.
- Cover count formatting and failure handling with focused tests.
- Verify the count against live agent processes and confirm it changes as
  agents start or exit; run the affected Go tests and `harnez status`.

---

## Implementation Plan

### Finding first: Codex's status line is a closed item picker, not a command hook

The ticket's precondition ("establish the supported Codex status-bar integration
point before changing it") resolves negatively. Evidence from the installed
Codex CLI (`~/.codex/packages/standalone/releases/0.153.0-.../bin/codex`,
v0.153.0):

- A `/statusline` slash command exists, described as *"configure which items
  appear in the status line"* — an **item picker over a fixed built-in set**,
  not a shell-command escape hatch like Claude Code's `statusLine` settings key.
- The persisted keys are `status_line` and `status_line_use_colors` (namespaced
  `codex.status_line`), stored in Codex's TUI settings, not in
  `~/.codex/config.toml` (which harnez already manages for hooks via
  `internal/codex`).
- The built-in item set is visible in the TUI event names:
  `StatusLineBranchUpdated`, `StatusLineGitSummaryUpdated`,
  `StatusLineWorkspaceHeadlineUpdated`. All are Codex-internal data sources.
- Codex's hook surface (`[hooks.<name>]`, issues 199/200) is `PreToolUse`-shaped
  only — there is no status-line hook.

**Conclusion: there is no supported way for harnez to inject a running-agent
count into the Codex status bar today.** Implementing one would mean writing
into Codex's private TUI settings store with a value it has no item for, which
is exactly the kind of unsupported coupling `internal/codex/hooks.go` was
careful to avoid.

### Recommended steps

1. **Record the finding** (this section) and add it as a short subsection to
   `docs/` — the same place issue 095's Claude-statusLine decision record lives —
   so the next agent does not re-derive it. One paragraph, plus the version
   probed (0.153.0) so it can be re-checked after a Codex upgrade.
2. **Re-scope the ticket to the surface harnez does own**: add the running-agent
   count to the **Claude Code** status line, which harnez already renders
   (`internal/statusline/statusline.go`, wired by `internal/claude/apply.go:173`
   via the `status_line: true` config key).
   - `internal/statusline/statusline.go`: extend `Render` to append a compact
     count segment after the cwd, e.g. `~/projects/harnez · 3 agents`.
     Call `usage.CountRunningAgentProcesses()` (`internal/usage/process.go`) —
     the shared implementation, per the ticket's "no second process scanner"
     requirement. Guard the import direction: `internal/statusline` importing
     `internal/usage` is a new edge; if that pulls in too much (usage imports
     net/http, sqlite-adjacent code), extract `CountRunningAgentProcesses` and
     `AgentProcessCount` into a small `internal/agentproc` package and have both
     `internal/usage` and `internal/statusline` depend on that instead. Prefer
     the extraction — it also unblocks [[143]].
   - Failure tolerance: `CountRunningAgentProcesses` already degrades to `ps`
     and then to a zero-valued struct; on `Total() == 0` omit the segment
     entirely rather than printing `0 agents`, so the status line never grows a
     useless field.
   - Subtract 1 from the count for the Claude process rendering the status line
     itself? **No** — the count is "agents running", and the host is one. Show
     the raw total; document it.
3. **Keep the Codex path covered by what already works**: `harnez usage --watch
   --proc` already renders the Processes box with per-agent counts
   (`buildProcessesBox`, `internal/usage/watch.go:639`). Note in the ticket that
   this is the Codex answer until Codex ships a custom status-line item.
4. **Tests** (`internal/statusline/statusline_test.go`): inject the count via a
   function field or an explicit parameter rather than calling the real
   `/proc` scanner — table cases for count 0 (segment omitted), 1 (`1 agent`),
   N (`N agents`), and a probe that returns a zero struct (renderer still
   returns the cwd line unchanged).
5. **Verify**: `go test ./...`, `make install`, then `scripts/smoke-test.sh` to
   confirm `harnez apply` still writes the `statusLine` settings block
   idempotently. Manually confirm the count changes as agents start/exit.

### Design decisions / tradeoffs

- Renaming/decoupling: extracting `internal/agentproc` is the one structural
  choice here. It is justified by two consumers ([[141]] and [[143]]) rather
  than speculation, and keeps `internal/statusline` free of the usage package's
  HTTP/collector dependencies (status lines run on every prompt — import weight
  is latency).
- Deliberately **not** writing to Codex's TUI settings store. Unsupported, and
  it would silently break on the next Codex release.

### Risks / open questions

- The Codex finding is from binary-string inspection of one version, not upstream
  docs. Worth one confirmation pass (`/statusline` in a live Codex TUI, or Codex
  release notes) before closing the "no integration point" conclusion. If Codex
  later adds a custom/command item, this ticket reopens as a small config change.
- Status-line latency: `CountRunningAgentProcesses` walks all of `/proc` on every
  Claude Code prompt render. Measure it (`go test -bench` or a timing loop) before
  shipping; if it exceeds a few milliseconds, add a short in-process/TTL cache
  keyed on time (e.g. 2s) rather than dropping the feature.

### Scope

**Small** for the re-scoped Claude-side work (one package, one extraction, ~4
test cases). **Zero/blocked** for the literal Codex status bar until upstream
provides a hook — the ticket title should be amended, or the Codex half split
into a blocked follow-up.
