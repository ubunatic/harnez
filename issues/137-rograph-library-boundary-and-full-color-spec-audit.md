# 137 — Harden `rograph` as a standalone library; move every ANSI color/style code into spec/

**Status**: Closed — resolved in 6c05b52 (code); docs closed in this commit (best guess, per issue 126)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Refactor
**Related**: [[136-bar-ansi-background-bracket-leak-and-color-spec]] (bootstraps `spec/colors.yaml` with a first `panel-bg` entry and the `internal/usage/colorsspec.go` loader — implement this ticket after 136 lands; both touch `internal/rograph/options.go` and the new spec file, so running them concurrently would race), [[132-watch-spec-driven-superscript-hotkeys]] (established the `spec/` bootstrap + embed/parse/validate pattern this ticket reuses), [[078-rograph-library-shared-bar-sparkline-renderer]] (original rograph package), `internal/rograph/*.go`, `internal/usage/watch.go`, `internal/usage/usage.go`, `internal/claude/maketargets.go`

## Problem

Two related gaps, found while fixing issue 136:

1. **Color/style codes are hardcoded throughout, not just in one place.**
   A grep for raw ANSI SGR escapes across the Go source turns up roughly 15+
   call sites beyond the one `"100"` background 136 addresses — bold
   (`\x1b[1m`), dim/muted (`\x1b[90m`, `\x1b[2m`), and reset (`\x1b[0m`)
   literals scattered through `internal/usage/watch.go` (box titles, hint
   lines, "no data" placeholders, the `bold`/`dim` helper closures around
   `watch.go:1282-1283`, footer text) and `internal/usage/usage.go:349`
   (`\033[2m`). `internal/claude/maketargets.go:360-365` also hardcodes a
   cyan `\033[36m` for generated Makefile output — a different subsystem
   (Makefile template generation, not the usage TUI/rograph), included here
   for completeness but may turn out to be a deliberate exception; decide
   and document rather than silently skip it.
2. **`rograph` isn't cleanly bounded as a library yet.** It's currently
   dependency-free (good), but there's no explicit rule stopping a future
   change from reaching into `spec/`-loading machinery or `internal/usage`
   types directly from inside `internal/rograph`. Issue 136's color-spec
   work had to make an explicit layering call (resolve spec values in
   `internal/usage`, pass plain strings/options into `rograph`) — that
   decision should be written down as a package-doc rule in
   `internal/rograph`, not left as an implicit convention only visible in
   one commit message.

## Scope

1. **Package boundary rule**: Add an explicit statement to
   `internal/rograph`'s package doc (`bar.go`'s existing doc comment, or a
   new `doc.go`) that the package must stay dependency-free and
   spec-agnostic — no `//go:embed`, no `spec/` awareness, no `internal/usage`
   imports. All spec-driven values (colors, defaults) are resolved by
   callers and passed in via existing `Options` structs. Verify this holds
   after issue 136 lands (it should, per that ticket's stated layering
   approach) and add it as a durable rule other contributors/agents will
   see, not just infer from history.
2. **Full color/style audit**: Extend `spec/colors.yaml` (bootstrapped by
   136) with named entries for every distinct SGR code actually in use —
   at minimum bold (`1`), dim (`2` and/or `90` — confirm whether these are
   used interchangeably or for different purposes and name them
   accordingly), and the existing panel background (`100`). Replace the
   hardcoded `\x1b[NNm` literals at the call sites listed above with lookups
   against the spec (via `internal/usage/colorsspec.go`, extended as
   needed — reuse 136's loader rather than adding a second one).
3. **Terminal control sequences stay out of scope.** Cursor positioning,
   screen clear/alt-screen, and cursor visibility sequences (`\x1b[H`,
   `\x1b[J`, `\x1b[K`, `\033[?1049h`/`l`, `\033[?25l`/`h`) are terminal
   *control*, not color/shade, and must NOT be moved into the color spec —
   only SGR color/style codes are in scope.
4. **Decide and document `maketargets.go`'s cyan.** Either fold it into the
   same color spec (if it's reasonable for a Makefile-generation subsystem
   to share `spec/colors.yaml`) or explicitly leave it and record why in
   this ticket's Resolution section — don't leave it silently unaddressed
   without a stated reason.

## Acceptance Criteria

- [x] `internal/rograph` has a written package-boundary rule: no spec/
      embedding, no `internal/usage` imports, stays a generic,
      dependency-free rendering library.
- [x] Every SGR color/style literal in `internal/usage/watch.go` and
      `internal/usage/usage.go` is replaced with a named lookup against
      `spec/colors.yaml`, with no behavior change to the actual rendered
      colors (a refactor, not a redesign) unless a specific difference is
      deliberately called out.
- [x] `maketargets.go`'s cyan is either included in the spec or explicitly
      exempted with a documented reason.
- [x] Terminal control sequences are unaffected — confirm none were
      accidentally swept into the color spec.
- [x] `go build ./...`, `go vet ./...` clean.
- [x] `go test -race ./internal/rograph/... ./internal/usage/... ./internal/claude/...`
      passes clean.
- [x] `harnez status` confirms tracker sync after filing/closing.

## Resolution

Landed in three commits: `8868937` (rograph package-boundary doc),
`675daf7` (spec/colors.yaml + colorsspec.go extension), `6c05b52`
(call-site sweep + maketargets.go decision).

**Named spec/colors.yaml entries** (final set): `panel-bg` (SGR 100,
from issue 136), `bold` (SGR 1), `dim-grey` (SGR 90, "bright black"
foreground), `dim-faint` (SGR 2, the faint/decreased-intensity
attribute). `\x1b[0m` reset stays a raw literal at call sites, per
issue 136's existing convention — it's the universal SGR closer, not a
color/style choice, so it doesn't get a spec entry.

**`dim`/`2` vs `90` naming decision**: kept as two distinct spec
entries rather than one. SGR 90 ("bright black") is a foreground
*color* substitution; SGR 2 ("faint") is a rendering *attribute*
applied on top of whatever foreground color is already active. They
can look similar in some terminals but are mechanically different and
not always interchangeable (e.g. a terminal that ignores faint
entirely still renders SGR 90's grey). `dim-grey` (90) is used
throughout the watch TUI (hint lines, placeholders, footer text);
`dim-faint` (2) is used only for the agent-sources list in
`harnez usage`'s non-watch output — genuinely distinct call sites in
distinct rendering contexts, so distinct names.

**`maketargets.go`'s cyan decision**: left as a raw `\033[36m` literal,
not folded into `spec/colors.yaml`, with the reasoning recorded inline
at `ensureColorVariables`. It's Make variable *content* generated for
other projects' `make help`-style output — a different subsystem,
audience, and color language than harnez's own `usage --watch` TUI.
Coupling an external Makefile's help-text color convention to harnez's
internal TUI palette would be a layering mistake for no real benefit,
so it stays a documented, deliberate exception rather than a silent
gap.

**Other fix along the way**: `internal/rograph/sparkline.go`'s
`PercentSparkline` independently hardcoded `"\x1b[100m"` instead of
using `DefaultBackgroundANSI` — the same duplication issue 136 fixed
in `RenderBar`. Fixed in the same sweep since it's the exact bug
pattern this ticket exists to close out, and it stays fully inside
`internal/rograph`'s own boundary (uses the package's own exported var,
no spec-loading machinery added).

Verification: `go build ./...`, `go vet ./...` clean; full `go test
./...` and targeted `go test -race ./internal/rograph/...
./internal/usage/... ./internal/claude/...` both pass. `make install`
run after the code changes.
