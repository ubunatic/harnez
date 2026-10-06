# 730 — Smoke test LMCODER_CACHE_DIR takes a list and uses the first available disk

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [#723 agent container](723-consolidated-agent-container-with-in-container-lmcoder-smoke-test.md)

---

## 1. Problem & Motivation
`make smoke-container` (`scripts/agent-canary/run.sh`) accepts one
`LMCODER_CACHE_DIR` for model downloads (fcf91ed7). The user's scratch disk
`/mnt/kdev` is an on-demand USB disk, often not mounted, so a single fixed
path fails or forces editing the variable per run.

/goal `LMCODER_CACHE_DIR="a,b,c"` uses the first listed directory that is
available and says which one it picked; when none is, the run falls back to
the podman cache volume with a notice. A single path keeps working as today.
Keep it simple: no free-space check in the script.

## 2. Technical Specification / Findings
- "Available": the directory exists or `mkdir -p` succeeds, and it is
  writable. An unmounted
  `/mnt/kdev` leaves a root-owned empty mount point, so `mkdir -p` under it
  fails and the entry is skipped.
- Decided: free space is not checked here. lmcoder's own 10 GiB minimum
  (`internal/llamahost/diskspace.go` in lmcoder) fails the run, by design.
- Workspace rule: experiment downloads go to `/mnt/kdev/scratch/` when mounted
  (`~/projects/AGENTS.md`, "Scratch disk for experiments").

## 3. Implementation & Verification Plan
- Parse the list in `run.sh`, log the chosen directory, fall back to the volume.
- Verify with `/mnt/kdev` mounted and with an unmountable or read-only first
  entry; document the variable in the Makefile help.
