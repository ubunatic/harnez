# 2026-08-31 — Bar-Rendering Visual Debugging, the `spec/` Bootstrap, and the `rograph` Boundary

Case study for the sprint that produced issues [[129]], [[131]], [[132]], [[133]],
[[136]], [[137]], [[138]] — `harnez usage --watch`'s collector parallelization,
hotkey overhaul, debug overlay, and a four-round visual-fidelity debugging chain
on the "All Usage" progress bars. Captures what a purely screenshot-driven bug
hunt looks like when the agent cannot see a live terminal, and the layering
decisions made along the way that don't otherwise have a home outside ticket text.

---

## 1. What shipped

- **129** — `collectAll`'s three sequential per-agent collectors (Claude/AGY/Codex)
  parallelized via goroutines + `sync.WaitGroup`. Worst-case ~10-12s reduced to
  ~3-4s. Low-risk, TDD-verified with an artificial-delay test asserting wall time.
- **132** — first use of `spec/` in this project (`spec/actions.yaml` +
  `spec/schemas/actions.schema.json`): every `--watch` hotkey, its action name, and
  its display symbol moved out of a hardcoded Go switch into a spec-driven registry.
  Landed btop-style superscript-numbered box toggles (`¹`-`⁷`) replacing the old
  ad hoc letter keys (`C`/`G`/`O`/`H`/`P`/`L`), and collapsed per-box "hidden"
  badges into a single count line.
- **131** — a `!`-toggled per-agent debug overlay: a 3-character countdown gauge
  that steals the last 3 characters of each agent row's label, showing time until
  the next background-collector tick, resetting full on `LastRefreshed` update.
- **133/136/137/138** — see §2 below; the eighth-block sub-character bar precision
  feature and three successive rounds of fixing its color rendering.

All landed on `main` directly (no worktrees, per explicit repo convention —
[[Worktrees]]), interleaved with an unrelated concurrent session touching
`AGENTS.md`/`canary-*` files. Every commit staged files explicitly
(`git add <files>`, never `-A`) and verified via `git status --short` before
committing, to avoid collateral-staging the other session's in-flight work.

## 2. The four-round bar-color debugging chain

This is the part worth a case study: a purely visual bug, diagnosed entirely from
four user-supplied terminal screenshots, with no way for the agent to render or
inspect the actual pixels itself.

**Round 0 (133)**: replaced whole-character bar snapping with Unicode eighth-block
glyphs (`▏▎▍▌▋▊▉█`) so the narrow 4-char "All Usage" bars show sub-character fill
precision. Landed clean, but the user's very next message clarified a requirement
that had been implicit until then: the bars must use the *same ANSI-background*
convention as the CPU/GPU load-box sparklines, not a dim foreground glyph — because
a bare `░` reads differently depending on terminal theme, while an explicit SGR
background is deterministic.

**Round 1 (136), image #1/#2**: user screenshots showed the new bars looking "wild"
next to the clean two-tone CPU/GPU bars. Root cause: `RenderBar`'s `ANSI` option
wrapped the *entire* rendered string — brackets, glyphs, and percent label — in the
background escape, whereas the CPU/GPU sparkline path (`RenderSparkline`) only ever
wrapped the glyph run itself. The brackets bled the escape's background outside
the intended bar area. Fixed by wrapping only the glyph substring; bootstrapped
`spec/colors.yaml` with a first named entry (`panel-bg` = SGR 100) so the `"100"`
literal wasn't hardcoded twice.

**Round 2 (137)**: a full audit pass, filed as its own ticket and *explicitly
sequenced after 136* to avoid a `spec/colors.yaml` file race — both tickets
touched the same new file and `internal/rograph/options.go`. Found ~15 more
hardcoded SGR literals scattered through `watch.go` and `usage.go` (bold, two
different "dim" codes), moved them all into the spec, and — the first time in
this project — wrote down `internal/rograph`'s package boundary as an explicit
rule (dependency-free, no `spec/` awareness, no `internal/usage` imports; a
caller's `init()` overwrites a plain exported var like `rograph.DefaultBackgroundANSI`
instead of the library importing spec-loading machinery). Also caught
`PercentSparkline` independently hardcoding the same `"100"` — the exact
duplication bug pattern 136 existed to close, found by the audit rather than
another screenshot round.

**Round 3 (138), images #3/#4**: even after 136+137, the user still saw more than
two tones. Image #3: "I still see three shades — does the darkest shade have any
meaning?" Image #4, after a probing question, delivered a *fourth* observed tone
and a precise breakdown: darkest = default terminal BG, brightest = default FG
(also the level markers), a bright shade (filled-cell FG), and a dark shade
(fractional-cell BG). At this point the agent proposed an `AskUserQuestion`
clarifying prompt — **the user rejected it outright ("No!")** and instead wrote
the exact fix themselves:

> 1. Use std. FG color for the full and half/quarter/third block FG chars
> 2. Use the same BG shade as used by GPU/CPU graph for chars between "[]"
> to me this yields two colors and ignores the default BG of the terminal
> the default BG of the terminal is already 100% overwritten in the GPU/CPU case, afaic

Root cause: `'░'` (light-shade block) is not "no ink" — it's its own low-density
stipple pattern drawn in the *foreground* color, so laid on top of a colored
background it reads as a third, unintended tone distinct from both the solid
`'█'` fill and the flat background. Fix: `eighthBlockFill` gained an `emptyRune`
parameter; `RenderBar` passes a plain space (not `'░'`) for empty cells whenever
`opts.ANSI` is true, since the background escape already covers the whole run and
a space renders as flat, zero-ink `panel-bg`. Plain-text callers (no ANSI) keep
`'░'` so the bar shape stays visible without color support.

## 3. What this debugging chain shows about agentic UI work

- **The agent's own theory of "done" was wrong three times in a row**, each time
  passing its own tests and its own mental model, before the user's screenshots
  proved otherwise. Root cause depth kept increasing: bracket-wrap bug (visible in
  one glance) → systemic hardcoded-literal sprawl (needed a grep audit) → a subtle
  semantic property of `'░'` itself (needed the user's own color-theory reasoning
  to surface). None of the first two rounds were *wrong* — they were real bugs and
  worth fixing — but neither was *sufficient*, and the agent had no way to know
  that without another round of user-supplied visual ground truth.
- **The user's rejected clarifying question was the more useful signal than the
  question itself.** The agent asked "should sub-character precision be dropped?"
  when the user had already stated, two turns earlier, that "the default BG of the
  terminal is already 100% overwritten in the GPU/CPU case" — a hint that fully
  determined the answer (keep sub-char ink, flatten only the background) without
  needing to ask. See [[feedback_verify_system_claims_empirically]] in memory for
  the general pattern; this is a visual-domain instance of the same lesson —
  a concrete instruction from the user beats a multiple-choice guess when the user
  has clearly already done the diagnosis themselves.
- **Sequencing tickets that touch the same file** (136 before 137, both touching
  `spec/colors.yaml` and `internal/rograph/options.go`) avoided a race without
  needing worktree isolation — just an explicit "Sequencing Note" in the ticket
  body plus running them one dev-subagent at a time on `main`.
- **A latent test-methodology bug surfaced only because the fix changed byte
  width.** Two `watch_test.go` alignment tests computed a visual column via
  `strings.LastIndex(...)`'s **byte** offset, which only "worked" because every
  glyph previously in play (`'█'`, `'░'`, all eighth-block chars) was a uniform
  3-byte UTF-8 sequence. Introducing a 1-byte space broke the coincidence and
  exposed the bug; fixed by switching to `utf8.RuneCountInString`. The real
  rendered TUI alignment was never wrong — only the test's own measurement was.
  Worth remembering as a general trap: any test comparing terminal-rendered string
  positions via raw byte indices is silently relying on all in-play runes being
  equal-width in UTF-8, and that assumption can break invisibly on the next glyph
  change.

## 4. Related docs

- [[Spec]] — the general `spec/` convention this session's `spec/actions.yaml` and
  `spec/colors.yaml` were the second and third instances of (first was in an
  earlier session — `spec/` itself didn't exist in this project before 132).
- [[2026-08-18-usage-watch-tui-terminal-rendering-postmortem]] — an earlier
  terminal-rendering bug in the same TUI that needed pty+VT100 repro rather than
  reasoning-only fixes; this session's bug was solvable from screenshots alone
  because the defect was in color/glyph *choice*, not layout/redraw timing.
- [[136-bar-ansi-background-bracket-leak-and-color-spec]],
  [[137-rograph-library-boundary-and-full-color-spec-audit]],
  [[138-bar-empty-glyph-flat-background-under-ansi-wrap]] — the three tickets
  chaining the fix; each documents its own root cause and resolution in full.
