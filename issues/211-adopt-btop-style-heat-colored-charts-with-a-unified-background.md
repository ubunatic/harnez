# 211 — Adopt btop-style heat-colored charts with a unified background

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: Issue 201; Issue 210; `spec/indicators.yaml`; `internal/rograph`

---

## 1. Problem & Motivation

The btop reference dashboard uses a black chart field with a heat-oriented
foreground: low readings are cool blue, medium readings become green/yellow,
and high readings become warm red. The adjacent percentage or temperature
text uses the same value-derived color. This makes the narrow per-CPU/GPU
charts readable at a glance while retaining their compact shape.

Harnez charts currently choose background styling independently and render
their percentages separately, so the chart field and its text do not form one
visual language. The new Braille history renderer should support this pattern,
but color must remain an explicit all-or-nothing presentation choice rather
than leaking into otherwise monochrome output.

## 2. Design Requirements

1. Define one spec-owned chart background token/ANSI value. Every chart
   surface (horizontal usage bars, lower-block sparklines, and Braille
   sparklines) resolves its background from this shared setting; renderers do
   not invent per-chart background defaults.
2. Add a named color presentation option for load histories, alongside the
   existing monochrome presentation. The color option applies a value-derived
   btop-style heat ramp to each chart cell/column: cool at low utilization,
   progressing through green/yellow to warm at high utilization. The exact
   palette and threshold/interpolation policy must be specified and tested,
   with terminal-safe ANSI output.
3. Couple chart and label styling: when colored Braille (or other colored load
   history) is selected, its adjacent CPU/GPU/RAM/VRAM percentage label must
   use the same current-value heat color. When monochrome is selected, both
   chart and percentage remain monochrome.
4. Keep chart glyph/presentation independent from color: block sparkline and
   Braille select geometry; monochrome and heat-colored select foreground
   styling. Bars use the shared background and their applicable foreground
   policy without creating a separate background setting.
5. Preserve a clean black/background field as shown by btop's default visual.
   Respect terminals that do not visibly distinguish the configured background
   and never emit malformed or unbalanced SGR sequences.
6. No color-only information: numeric percentages remain present and readable;
   monochrome stays a supported deliberate configuration.

## 3. Scope and Likely Touchpoints

- Extend `spec/indicators.yaml` and `spec/schemas/indicators.schema.json`
  with a single chart-background declaration plus explicit monochrome/colored
  chart presentation settings.
- Resolve those settings centrally in `internal/usage`, then pass final ANSI
  foreground/background choices into the dependency-free `internal/rograph`
  renderers.
- Update load-line composition so the chart and its percentage share the
  current value's heat color only in the colored mode.
- Audit all watch-summary chart call sites, including compact VRAM/GTT dual
  charts, for the shared background rule.

## 4. Acceptance Criteria and Verification

- A spec validation test accepts supported monochrome and heat-colored modes,
  rejects unknown values, and proves all chart types resolve the same
  background token.
- Renderer tests cover low/mid/high values, foreground transitions, ANSI reset
  balance, and unchanged geometry/terminal width with styling stripped.
- Usage rendering tests prove colored mode colors both the history and its
  percentage consistently, while monochrome mode colors neither foreground.
- Visual check `harnez usage --watch` at normal and compact widths confirms a
  black unified chart field and the btop-like cool-to-warm signal without
  compromising labels or layout.
- Run `go test ./...`, `make check`, and `make install`; distinguish any
  independently reproducible baseline failure from regressions.

## 5. Non-goals

- Do not make colored output mandatory or recolor the entire application.
- Do not change Braille's four visible baseline-inclusive height bands from
  issue 210.
- Do not give each chart type its own background setting.
