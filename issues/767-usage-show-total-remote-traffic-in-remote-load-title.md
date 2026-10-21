# 767 — usage: show total remote traffic in Remote Load title

**Status**: Backlog — grooming pass 843 M4 (2026-10-22); promote when needed
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 110 (remote load batch vs streaming), 114 (stream drop to batch), internal/usage/watch.go, internal/usage/loadstream.go

---

## Problem & Motivation

The Remote Load box title shows only the mode, e.g. `[R] Remote Load (@x600 · streaming)`
(`watch.go:~1030-1037`). On the go (mobile data), the user wants to keep an eye on how much
network traffic `harnez usage` causes to remote hosts.

## Goal

/goal The Remote Load title shows the total remote traffic this `harnez usage` process has caused,
e.g. `@x600 · streaming 812k` or `@x600 · batch 3.4M`, counting all of it: the load stream, batch
calls, and any other remote fetches (usage JSON, reconnects), in both directions, plus an estimated
SSH/TCP overhead; unit-tested (k/M formatting boundaries, zero, accumulation across reconnect and
stream-to-batch fallback) and checked live in `harnez usage --minimal --watch`; or stop and report
when blocked on a user decision or a denied permission.

## Milestones

- **M1 (Go-side count):** count the bytes harnez itself reads and writes on each remote channel
  (wrap the stdin/stdout of the ssh subprocess) and add an estimated overhead (SSH packet framing,
  TCP/IP headers, session setup per new ssh connection). Document the estimate's formula and its
  source in code and in this ticket. The number is cumulative for the process, not reset on reconnect.
- **M2 (later, OS stats):** read real numbers from the OS for full accuracy (e.g. per-process or
  per-socket counters of the ssh child), and compare them against the M1 estimate. Not part of M1.

## Notes

- Check live code first; find every place `harnez usage` talks to a remote host before counting.
- Unit base: pick one (1000 or 1024), state it in the commit and here.
- The title width must still fit the box; check the minimal layout.
