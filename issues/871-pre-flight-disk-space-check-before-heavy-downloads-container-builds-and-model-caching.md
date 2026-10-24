# 871 — Pre-flight disk space check before heavy downloads, container builds, and model caching

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 343 (host disk usage meters in load telemetry), 730 (LMCODERCACHEDIR disk fallback), voxi#182, voxi#200

---

## 1. Problem & Motivation

Autonomous and subagent coding workflows frequently attempt to download multi-gigabyte assets (ASR/TTS/LLM model weights, PyTorch container images, benchmark datasets, deep caches) without verifying filesystem headroom beforehand.

On developer laptops or workstation partitions with constrained free space (e.g., < 10–15 GiB free on `/` or `/home`), unattended agent operations can cause:
1. Disk exhaustion (`ENOSPC`) mid-operation, leaving half-extracted, corrupt caches.
2. System instability, crashing desktop compositors, browser sessions, or user daemons.
3. Thrashing or swap lockup when builds exhaust both disk and memory simultaneously.

Furthermore, agents rarely check whether a dedicated fast scratch mount or secondary drive exists (such as `/mnt/kdev/scratch`, NVMe scratch pools, or repo-specified data directories) unless explicitly told.

## 2. Specification & Behavior

### A. Pre-Flight Storage Check Rule / Utility
- Provide a standardized disk space pre-flight check in `harnez` (e.g., `harnez disk check [path] [--min-gb N]`, or guidance in `.harnez/rules/Tools.md`).
- Default safety threshold: $\ge 10\text{ GiB}$ free headroom on the target partition before downloading model weights, pulling large Docker/Podman layers, or running uncompressed audio/video dataset expansions.
- Report partition status: mount point, filesystem, total, used, and available space.

### B. Scratch Mount & External Storage Discovery
- If target free space is below the threshold, inspect common fast secondary scratch locations (e.g. `/mnt/kdev/scratch`, `/scratch`, `$SCRATCH_DIR`, or repo-specified mounts).
- Advise the agent or prompt the user before allocating large files on the primary root/home partition when a secondary scratch mount is available.

### C. Agent Harness Guidance & Skills Integration
- Update agent guidance (and model onboarding/benchmark skills like `voxi#200`) so agents execute disk pre-flight checks as part of their preparation phase before invoking heavy downloads (`curl`, `hf-cli`, `podman build`, etc.).
- Advise pruning unused model artifacts or utilizing cache eviction (`harnez` or repo-specific prune commands) when space is tight.

## 3. Implementation & Verification Plan

1. **CLI / Helper**: Implement or document disk pre-flight probing logic (e.g. in `harnez` CLI or standard agent tool guidance).
2. **Configuration / Spec**: Allow configuring minimum disk headroom and preferred scratch mounts in `.harnez/config.yaml` or project spec if appropriate.
3. **Tests**:
   - Add test validating disk check logic across mocked mount points and capacity levels.
   - Verify that insufficient space triggers a clear, actionable warning or abort rather than silent failure.

## Decision (owner delegated, 2026-10-24)

Guidance only: a short rule (check `df -h` on the target before downloads or builds over a few GB; use the
scratch disk or ask when short). No CLI, spec or tests; a CLI becomes its own ticket if the rule proves too weak.
