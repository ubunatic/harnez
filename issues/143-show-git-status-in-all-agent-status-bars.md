# 143 — Show Git status in all agent status bars

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[095-restore-cwd-after-shell-tool-use-and-statusline-cwd-display]], [[141-codex-status-bar-agent-count]], `internal/statusline`, `internal/claude/apply.go`

---

## 1. Problem & Motivation

Agent status bars do not show the Git state of the active workspace. Users
need a compact, consistent indication of branch and working-tree status
across all supported agent integrations, so uncommitted work and repository
context are visible without a separate terminal command.

## 2. Technical Specification / Findings

- Define the supported status-bar integration surface for Claude, Codex, AGY,
  and other managed agents before assuming a shared protocol.
- Render useful Git state compactly: repository/branch identity and whether
  tracked or untracked changes are present; handle detached HEAD, non-Git
  directories, and Git command failures without breaking the status bar.
- Reuse one shared Git-status implementation so each agent surface cannot
  drift in formatting or semantics.

## 3. Implementation & Verification Plan

- Inventory every supported agent status-bar integration and document any
  platform limitations or fallback behavior.
- Add shared Git-status collection and compact rendering, with bounded
  latency suitable for frequent status-bar refreshes.
- Wire the renderer into every supported agent status bar while preserving
  existing status-bar content and graceful degradation.
- Add focused tests for clean, modified, untracked, detached-HEAD, non-repo,
  and Git-error states; verify each available integration manually.

---

## Implementation Plan

### Inventory of status-bar surfaces (the ticket's step 1, answered)

| Agent | Custom status-bar hook? | Git already shown natively? |
|---|---|---|
| **Claude Code** | **Yes** — `statusLine` settings key, already owned by harnez (`internal/claude/apply.go:173`, `internal/statusline`, `status_line: true` in `config.yaml`) | No (harnez's renderer is cwd-only) |
| **Codex CLI** (v0.153.0) | **No** — `/statusline` is a picker over a fixed built-in item set; keys `status_line` / `status_line_use_colors` live in Codex's private TUI settings. No command escape hatch, and its hook surface (`[hooks.<name>]`) is `PreToolUse`-only. | **Yes, natively** — built-in items include branch and git summary (`StatusLineBranchUpdated`, `StatusLineGitSummaryUpdated`, `StatusLineWorkspaceHeadlineUpdated`) |
| **AGY / Antigravity** | Not established. Its harnez integration is `~/.gemini/config/hooks.json` (`internal/agy/hooks.go`), lifecycle hooks only — no status-bar surface found. | Unknown |

**Consequence:** "all agent status bars" reduces to *one* surface harnez can
actually write to (Claude Code), while Codex already solves this natively for
its own users. The realistic deliverable is: implement it for Claude Code,
document that Codex users enable it with `/statusline`, and record AGY as
unsupported pending a found integration point. Amend the ticket title
accordingly rather than leaving it implying three implementations.

### Steps

1. **Reuse `internal/gitstatus` — do not write a second git reader.** It already
   parses `git status --porcelain=v2 --branch` into `Status{Branch, Detached,
   Upstream, Ahead, Behind, Staged/Unstaged/Untracked/ConflictFiles}` with a
   documented `Quiet()` threshold and full test coverage
   (`internal/gitstatus/gitstatus.go`, built for `harnez repo-status`, issue
   154). Detached HEAD, no-upstream, and parse failures are already modeled.
2. `internal/gitstatus/gitstatus.go` — add one pure formatter:
   `func (s Status) Compact() string`, e.g.
   - clean, in sync: `main`
   - detached: `@a1b2c3d` (or `(detached)` — `Status` does not currently carry
     the short OID; either extend `Parse` to keep `branch.oid`'s first 7 chars,
     or print `(detached)` and skip the OID for v1 — prefer the latter, smaller)
   - dirty: `main *3` (changed = staged+unstaged) and `main *3 +2` (untracked)
   - divergence: `main ↑2` / `main ↓1` / `main ↕2/1`
   - conflicts: `main !2` (highest-priority marker, shown first)
   Keep it ASCII-safe with the arrow glyphs optional, and keep the output short
   — a status line competes for one row.
3. `internal/statusline/statusline.go` — extend `Render` to append the compact
   git segment after the cwd. Take the collector as an injectable dependency
   (a `func(dir string) (gitstatus.Status, error)` parameter or struct field) so
   tests never shell out. On any error, on a non-git directory, or on an empty
   `Compact()`, emit the cwd line unchanged — the status line must never fail or
   print an error.
   - Collect against the payload's `cwd`/`workspace.current_dir`, not the
     process cwd.
4. `cmd/harnez/statusline.go` — wire the real collector in; update the `Long`
   help text, which currently says *"MVP scope is cwd only — no git branch,
   model, or cost info"*.
5. **Latency guard.** `gitstatus.Collect` forks `git` on every Claude Code
   prompt render. Bound it: run the command with a context timeout (~300ms) and
   fall back to the cwd-only line on timeout. Add the timeout to `Collect` or
   add a `CollectContext(ctx, dir)` sibling and leave `Collect` as-is for
   `repo-status`.
6. **Docs**: extend the issue-095 statusline decision record in `docs/` with the
   surface-inventory table above (shared with [[141]] — write it once, both
   tickets reference it).
7. **Tests**
   - `internal/gitstatus/gitstatus_test.go`: table test for `Compact()` across
     clean / staged-only / unstaged / untracked / mixed / conflicts / detached /
     no-upstream / ahead / behind / diverged. Pure string assertions on a
     hand-built `Status` — no repo needed.
   - `internal/statusline/statusline_test.go`: injected collector returning
     (a) a normal status, (b) an error, (c) a non-git error — assert the line
     content exactly, and that (b)/(c) return the unchanged cwd line.
   - One integration test using the existing real-repo pattern already in
     `gitstatus_test.go`.
8. **Verify**: `go test ./...`, `make install`, `scripts/smoke-test.sh`, then
   look at a live Claude Code session's status line in a dirty and a clean repo.

### Design decisions / tradeoffs

- **One shared implementation, one shared formatter.** The ticket's anti-drift
  requirement is satisfied by putting `Compact()` on `gitstatus.Status` (next to
  `Quiet()`), not by duplicating formatting in `internal/statusline`.
- **Reuse `Quiet()`'s semantics, not `Quiet()` itself.** `Quiet()` deliberately
  treats "ahead of upstream" as boring for `repo-status`' reporting. A status
  line should still *show* `↑2` — it costs 3 characters and is exactly the
  context a status line exists for. Do not gate `Compact()` on `Quiet()`.
- **Coordinate with [[141]].** Both tickets append a segment to the same one-line
  renderer. Land whichever first, then have the second reuse the segment-joining
  helper; do not build two independent append paths. Suggested order: 143 first
  (git is the higher-value segment and needs no new package), then 141's count.

### Risks / open questions

- Segment ordering and total width once [[141]]'s agent count lands: three
  segments (`cwd · git · agents`) can overflow a narrow terminal. Claude Code
  gives no width hint in the statusLine payload, so decide a fixed priority
  order (git > agents > full cwd path) and shorten the cwd (basename only) before
  dropping segments.
- AGY status-bar surface is unresearched — either spend a bounded probe on it
  (mirroring the Codex binary/strings inspection done for [[141]]) or explicitly
  scope it out of this ticket.
- `git status` in a very large or NFS-backed repo can exceed the 300ms budget
  regularly; if observed, cache per-directory with a short TTL rather than
  raising the timeout.

### Scope

**Medium** — the git collection already exists, so the work is one formatter, one
renderer change, a timeout, a docs entry, and a solid test table. Larger only if
the AGY surface investigation is kept in scope.
