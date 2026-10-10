# 767 — usage: show streamed bytes in Remote Load title as 'streaming <num>k|M'

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 110 (remote load batch vs streaming), 114 (stream drop to batch), internal/usage/watch.go, internal/usage/loadstream.go

---

## Problem & Motivation

The Remote Load box title shows only the mode, e.g. `[R] Remote Load (@x600 · streaming)`
(`watch.go:~1030-1037`). The user wants to see how much data the stream has moved, to judge its
cost and that it is alive.

## Goal

/goal While streaming, the title reads `streaming <num><k|M>` with the total bytes received from
the remote stream so far (e.g. `@x600 · streaming 812k`, `@x600 · streaming 3.4M`); `batch` stays
unchanged; the counter is unit-tested (formatting at the k/M boundaries, zero bytes, reset on
reconnect or fallback to batch) and checked live in `harnez usage --minimal --watch`; or stop and
report when blocked on a user decision or a denied permission.

## Notes

- Check live code first. Count bytes where `loadstream.go` reads the stream, not by re-encoding.
- Open: unit base (1000 vs 1024) and whether the count resets on reconnect; pick the obvious
  choice, state it in the commit, and note it here.
- The title width must still fit the box; check the minimal layout.
