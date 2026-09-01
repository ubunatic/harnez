# Retrospective: Time-gauge ANSI styling must follow label padding

**Date**: 2026-09-01
**Context**: Issue 150 compact debug time-gauge color parity.

The first implementation put the foreground/background SGR sequences into the
freshness label before `rograph.PadLabel` processed it. That helper counts
runes, not visible terminal columns, so the escape sequences consumed the
compact label budget and corrupted the rendered countdown. Applying styling to
the leading gauge glyph after formatting preserves both ANSI-stripped text and
layout width.

The independent review also improved the regression test: it now checks the
exact styled gauge, its single foreground occurrence, and retention of an
existing graph's `panel-bg` SGR sequence. For ANSI changes in fixed-width TUI
rows, tests must cover the raw escape sequence and the stripped geometry.
