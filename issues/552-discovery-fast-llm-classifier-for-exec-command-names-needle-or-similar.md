# 552 — Discovery: fast LLM classifier for exec command names (Needle or similar)

**Status**: Open
**Priority**: P3
**Severity**: Low
**Category**: Discovery / Telemetry
**Related**: [[551-exec-bash-c-unwrap-misses-bin-bash-and-lc]], [[215-llm-backfill-and-reclassification-mode-for-telemetry-tool-notes]]

---

## Problem

`harnez exec` guesses the tool name of a command with fixed rules (`inferToolFromArgs`,
`cmd/harnez/exec.go`). Wrapped forms (`/bin/bash -c`, `bash -lc`, pipes, `cd x && go test`) are
missed or reduced to the shell name (551). The rule fix in 551 covers the known cases. The user
wants to know if a very fast LLM classifier could name the real command for any shape of input.

## /goal

A short findings section in this ticket that assesses 3 options able to classify a shell command
line into its main command name in under 1 s (target: far less, since it runs per agent command).
Candidates: Needle LLM (claimed ~2000 tokens/s) and two others (e.g. a small local model, a hosted
fast-inference API). For each: latency measured or documented, cost, offline use, setup, accuracy on
~20 sample commands taken from real agy/claude telemetry. End with a recommendation: adopt, use only
for offline backfill (see 215), or keep rules only.

## Notes

- Discovery only; no code in harnez. Canary-first (docs/Canary.md) if an option is probed.
- The classifier must not slow `harnez exec`; if it cannot run in the hot path, it could run after
  the command, asynchronously, when writing telemetry.
- Uncertain: Needle's exact product, license and API; verify before assessing.
