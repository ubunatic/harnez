# 563 — Meter claude and codex traffic like agy (proxy), incl. hidden helper calls

**Status**: Open
**Priority**: P3
**Severity**: Low
**Category**: Telemetry
**Related**: [[035-transparent-proxy-quota-and-token-sidecar]], [[562-harnez-bench-agy-main-vs-helper-model-split-read-benchmark]]

## Background

The agy metering proxy revealed that agy sends extra helper-model requests per prompt that the
agy UI never shows. Claude Code and Codex likely do similar things (conversation summaries,
session titles such as Codex's status-line title), but without a proxy we only see what their
own usage reports include.

## Goal (parked)

Find out whether claude and codex can run behind a per-session metering proxy (HTTPS_PROXY plus a
custom CA, or a documented base-URL override), and if so record every request the same way as
the agy meter: main and helper models separated, full token totals per session. Canary first:
one tiny prompt per CLI, check that traffic goes through and nothing breaks.

## Later: cost attribution analytics

Tokens are not the cost; plan quota (percentages with fractions, from `harnez usage` and the agy
meter quota rows) is. Once helper calls are visible, analyse how much of the quota drain comes
from the main session versus background helper calls, per provider.

Parked until the read benchmark (562) is done.
