# Case Study: `harnez usage --watch` TUI and a Three-Theory Terminal Rendering Postmortem

**Date**: 2026-08-18
**Scope**: Live-refreshing btop-style dashboard for `harnez usage`, and the debugging saga that followed
**Related**: [Issue 023: `harnez usage`](../../issues/023-usage-command-token-quota-tracking.md), [Issue 030: AGY/Codex local token counts](../../issues/030-agy-codex-missing-local-token-counts.md)
**Status**: Feature implemented, user-confirmed fixed, **uncommitted at session end** (see §6)

---

## 1. What was built

Starting from the existing one-shot `harnez usage` report (`internal/usage/usage.go`), this session added `harnez usage --watch` (`internal/usage/watch.go`): a live-refreshing, btop-style grid of per-agent panels (Claude Code, Antigravity/AGY, OpenAI Codex), modeled directly on the resource-monitor TUI already proven out in the sibling `voxi` repo (`voxi/internal/monitor/monitor.go`).

Feature surface, in the order it was built:
1. `--watch` / `-w` flag with `--interval` (default 60s, floored at 30s) so the live view can't be pointed at live quota APIs faster than the data actually changes — the original ask, prompted by the user manually polling `harnez usage` in a loop and burning far more requests than intended.
2. A compact box-per-agent redraw (in place of reprinting the full, chatty one-shot report every tick), with a tokens/min sparkline computed from an in-memory `rateTracker` — no persistence, no extra API calls beyond the tick itself.
3. Interactive per-panel toggles (`c`/`g`/`o` for Claude/AGY/Codex, `t` for the token line, `a` for all, `q` to quit), with the toggle key embedded in each panel's own title bar rather than a legend at the bottom — explicitly requested to mirror btop's own convention.
4. Along the way, a real bug was found and fixed in `internal/usage/agy.go`: the Antigravity CLI listens on two loopback ports (one TLS-only, one plain-HTTP RPC), and the original quota-port discovery only TCP-connect-probed before picking one, so it would silently pick the wrong port about half the time and drop AGY's quota bars with no error. Fixed to try every candidate port until one actually answers the RPC.

Then the box broke, and stayed broken through three independent fix attempts.

---

## 2. The three wrong theories

Each fix was plausible, each was verified in isolation, and none of them fixed the user's actual bug. That pattern — verified-in-isolation-but-doesn't-fix-the-report — is the core lesson of this study.

| # | Theory | Fix applied | Verified how | Outcome |
|---|--------|-------------|---------------|---------|
| 1 | `terminalWidth()` discarded any real reading under 60 columns, substituting a hardcoded 90 | Trust real narrow readings, floor at 30 | Reasoned through the code | Did not fix it — never tested against the user's actual terminal |
| 2 | `stty -F /dev/tty size` itself was failing/unreliable in the user's environment | Query `TIOCGWINSZ` via ioctl directly on the output fd instead of shelling out | Synthetic `pty.openpty()` test at 55×20 — rendered correctly | Did not fix it — the synthetic test didn't reproduce the user's actual failure mode |
| 3 | Rounded Unicode box corners (`╭ ╮ ╰ ╯`) were missing from the user's terminal font and falling back to a wider substitute glyph | Switch to the plain square corners (`┌ ┐ └ ┘`) already used successfully by the one-shot report | Compared against the working `RenderText` renderer, which shares the same `─`/`│` glyphs | Did not fix it — user reported "same effect" |

By theory 3 failing, a pattern should have been named explicitly and wasn't: **every fix so far was reasoned about, none was reproduced.** Each theory was individually falsifiable and none was actually tested against a real, height-constrained pty running the exact toggle sequence the user performed (open with 3 panels, press two keys to drop to 1). That test — not more reasoning — is what found the real bug.

## 3. The actual root cause

The user's own framing — *"we may have a sonnet mess here, hand this to an opus subagent... use a robust box system"* — was the right call, and not just because a stronger model reasons better. The decisive difference was **methodology**: the handoff prompt explicitly asked the agent to reproduce the bug under a full VT100 emulator (`pyte`) driven by a real pty with the user's exact keystroke sequence, before proposing anything. That single instruction is what broke the guess-and-check loop.

Two concrete, independent bugs were found this way:

1. **Stale-content bleed.** The redraw loop wrote `\x1b[H` (cursor home) + frame content + `\x1b[J` (erase-cursor-to-end-of-screen) each tick. `\x1b[J` only clears *below* wherever the cursor ends up — it does nothing to a line that gets *shorter* than what it's replacing, and a blank line (a bare `"\n"`) clears nothing at all. Shrinking from 3 panels to 1 left literal debris from the taller frame interleaved with the new, shorter one. Captured mid-bug at 100×30 after pressing `c`, `g`:
   ```
    6|└──────────────────────────────────────────────┘|
    7|│|                                                    <- stale from old frame
    8|refresh every 30s   [a]ll  [q]uit 49.3% · 5d 22h │ │ Weekly  [████████████] 100.0%
   ```
   This is exactly "top border, then the footer, body and bottom border gone" — the body wasn't missing, it was overwritten by / merged with leftovers from a previous, larger frame.

2. **Gutter off-by-one in the width math.** `boxWidth = total / columns` never accounted for the single-space gutter `combineRow` inserts between side-by-side boxes. At 100 columns: 2 boxes × 50 + 1 gutter = 101 cells in a 100-column terminal — every multi-column row wrapped onto a second physical row and sprayed stray `┐`/`│` fragments, which then compounded into the scroll-drift symptom. Single-panel lines, not needing a gutter, still landed at *exactly* 100 columns — the "deferred wrap" danger zone some terminals (VTE included) treat specially at the very last cell.

None of the three earlier theories were *wrong* to worry about — Unicode box-drawing and block characters (`·`, `…`, `█`, `░`, and the box corners themselves) genuinely are East-Asian "Ambiguous" width and can render two cells wide under some font/locale configurations, and it was reasonable to suspect glyph width first, since that is the classic version of this bug class. They were just not *this* bug. The fix that landed:
- A `screenFrame` type doing position-independent, per-line painting (`\x1b[0m\x1b[K` after every line clears that line specifically, regardless of what was there before).
- `gridColumns()` that subtracts gutters before dividing, plus a 1-column safety margin that never writes the terminal's last cell.
- Height-aware layout: `terminalSize()` now returns rows as well as columns (the previous `ttyColumns` silently discarded `ws.Row` from the same ioctl call), and panels that don't fit vertically are dropped with a "N panel(s) hidden — terminal too short" note instead of corrupting the frame.
- The alternate screen buffer (`\x1b[?1049h` / `\x1b[?1049l`) plus `SIGWINCH` handling, so a live resize re-lays-out cleanly instead of drifting.
- Four regression tests (`internal/usage/watch_test.go`) pinning the gutter math across widths/panel-counts, per-line box width, viewport clipping, and the no-stale-content painting contract.

---

## 4. Honest post-mortem

### What I'd do differently

**I had already read the answer and didn't apply it.** At the very start of this session I read `voxi/internal/monitor/monitor.go` — the sibling repo's own resource-monitor TUI — as the explicit reference for "the monitor TUI we already built." That file implements `RuneDisplayWidth`/`StringDisplayWidth`, which measure actual terminal *display* columns (accounting for double-width Unicode) rather than `utf8.RuneCountInString`. `harnez usage --watch`'s `visLen` was written as a plain `utf8.RuneCountInString` over ANSI-stripped text — the exact naive approach voxi's own commit history had already moved past. Worse: `voxi/docs/studies/2026-08-18-continuous-eager-streaming-and-resource-monitor.md` and `2026-08-18-modifier-key-gating-and-system-daemon.md` document this *as a named lesson* ("Terminal Layouts Require Cell Display Width Math" — never use rune-length for box padding when wide characters may be present). Those studies used to live in this repo and were deliberately migrated to `voxi` a few commits ago (`2206b95 docs: remove historical voice issues and case studies`); migrating the docs out doesn't migrate the lesson out of the codebase's institutional memory if the next session's agent doesn't go re-read the sibling repo's docs before reimplementing a very similar TUI from scratch. I should have grepped `../voxi/docs/studies/` for prior art before writing `watch.go`'s width math, not just skimmed `monitor.go` for structure.

**I iterated on reasoning instead of reproduction, three times in a row.** Each of the three failed fixes was a plausible theory checked by re-reading code or a synthetic test that didn't match the user's real failure conditions (wrong terminal size, wrong keystroke sequence, or just "does the string I compute have the right length" rather than "what does a real terminal do with these bytes"). The turning point wasn't a smarter guess, it was switching to `pyte`-driven VT100 emulation of the *exact* repro steps. That should have been the first move, not the fourth.

**I never flagged that the entire session's work was uncommitted.** See §6 — this surfaced only when running this `/evergreen` pass, not proactively during four rounds of user-facing "fixed it" claims that each turned out to be wrong. A user watching `git status` stay dirty through several "should be fixed now" replies would reasonably worry about losing work if something crashed mid-session.

### Where I likely failed to discover project status

- Did not check `../voxi/docs/studies/` for prior art on the exact same terminal-rendering bug class before implementing `watch.go`'s layout math (see above).
- Did not proactively surface `git status` (all changes uncommitted) at any point before this `/evergreen` pass, despite multiple rounds of "fixed" claims that would have been safer to checkpoint between.
- Did not notice, until the Opus subagent's cleanup pass reported it, that `internal/usage/claude.go` and `internal/usage/types.go` have pre-existing `gofmt` drift (struct field alignment only, from commit `753ee86`, unrelated to this session — not fixed here, flagged in Issue 030's sibling note below for someone to clean up with a plain `gofmt -w`).

---

## 5. Agentic coding insight: when and how to escalate

The concrete, reusable lesson for future sessions in this repo (and elsewhere): **for terminal-rendering / cursor-math bugs, reproduce under a full terminal emulator before proposing a fix, on the first attempt — don't spend iterations reasoning about ANSI byte sequences by eye.** A `pty.openpty()` + `pyte.Screen`/`pyte.Stream` harness (or equivalent) that replays the exact byte stream a program writes and reports the resulting *visible* screen state is cheap to set up and is the only reliable way to tell "my width math is correct" from "my width math produces the right output on an actual terminal." Reasoning about `\x1b[H`/`\x1b[J` semantics by hand is exactly the kind of task where a plausible-sounding theory (rounded corners! ambiguous-width glyphs!) survives multiple rounds of "verified, but the user says it's still broken" without ever being wrong enough to notice sooner.

A secondary, narrower lesson: when a sibling repo already solved a materially similar problem (voxi's `StringDisplayWidth`), grep its docs *and* its solved code before reimplementing the same category of problem, even when only borrowing "the pattern" and not the code directly.

---

## 6. Uncommitted work at session end

As of writing, `git status` in this repo shows:
```
 M cmd/harnez/main.go
 M internal/usage/agy.go
 M internal/usage/util.go
 M internal/usage/util_test.go
?? internal/usage/watch.go
?? internal/usage/watch_test.go
```
This entire session's work — the `--watch` feature, the AGY dual-port fix, the `FormatDuration` day/hour formatting, and this study/issue documentation — has not been committed. `go build`, `go vet`, `go test ./...`, and `make install` all pass on the working tree; the binary the user tested against is built from this uncommitted state. Per this repo's `docs/lang/Git.md` conventions (conventional commits, work on the default branch, don't push unless asked), this is left for the user to commit explicitly rather than committed automatically here — but it should happen before anything else touches these files, to avoid losing four rounds of hard-won fixes to an unrelated `git checkout` or `git clean`.

---

## 7. File & commit summary

- **Added**: `internal/usage/watch.go`, `internal/usage/watch_test.go`
- **Modified**: `cmd/harnez/main.go` (`--watch`/`--interval` flags), `internal/usage/agy.go` (dual-port quota RPC fix), `internal/usage/util.go` + `util_test.go` (`FormatDuration` day/hour formatting at ≥48h)
- **Not yet committed** — see §6
- **New issue**: [030-agy-codex-missing-local-token-counts.md](../../issues/030-agy-codex-missing-local-token-counts.md) — AGY and Codex have no local token-count source, discovered while wiring up the watch panels' token toggle
