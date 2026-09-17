# 404 — Add HDD/SSD storage and I/O usage metrics to usage watch TUI

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: #343

---

## 1. Problem Statement & Motivation

The `harnez usage --watch` split-screen telemetry TUI provides real-time monitoring for host CPU cores, memory utilization, GPU compute activity, and VRAM/GTT memory allocations. However, host disk storage capacity and disk I/O rates (reads/writes per second or throughput) are currently missing from the usage monitor.

When running resource-intensive tasks such as continuous agentic loops, containerized guest workloads (e.g. `dockur/macos`, Docker dev environments), VM image builds, or large-scale dataset ingestion, disk space exhaustion and I/O saturation can severely degrade performance or cause sudden catastrophic failures. Having high-visibility storage metrics alongside CPU/RAM/GPU in the `harnez usage --watch` dashboard allows operators and agent harnesses to detect storage bottlenecks and capacity limits early.

---

## 2. Scope & Target Experience

In `harnez usage --watch`:
1. **Disk Capacity Meter**:
   - Display mount point storage usage (e.g., `/` root filesystem or primary storage drives).
   - Format: Used / Total capacity with percentage progress bar (e.g. `[⣿⣿⣿⣿⣿⣿⣀⣀⣀⣀] 420.5/953.8G 44%`).
2. **Disk I/O Activity**:
   - Display active read and write throughput (e.g. `IO R: 12.4 MB/s | W: 85.2 MB/s` or IOPS).
3. **Visual Alignment**:
   - Render storage stats cleanly within the existing Usage / Load telemetry panel layout, respecting terminal width and responsive compact layouts.

---

## 3. Technical Specification & Implementation Plan

### 3.1 Data Collection Backend
- **Linux**:
  - Filesystem Capacity: Use `syscall.Statfs` / `golang.org/x/sys/unix` `Statvfs` on configured / detected mount points (e.g. `/`).
  - Disk I/O: Parse `/proc/diskstats` or `/sys/block/<dev>/stat` across sample intervals to compute differential read/write sectors and convert to throughput (KB/s, MB/s) and IOPS.
- **Darwin (macOS)**:
  - Filesystem Capacity: Use `unix.Statfs` on `/` or primary volumes.
  - Disk I/O: Use `sysctl` (`hw.diskstats` / `iostat` Mach kernel statistics) or graceful fallback when detailed I/O counters are unavailable without cgo.
- **Graceful Degradation**:
  - Non-Linux/macOS platforms or environments with restricted `/proc` / filesystem access gracefully omit I/O or fall back to standard `statvfs` capacity meters without crashing the TUI.

### 3.2 TUI Integration
- Update usage telemetry collector in `internal/usage` (or equivalent telemetry packages).
- Add disk usage visual meters to the usage dashboard rendering engine, matching the existing sparkline/progress bar formatting standards (runes, display width safety, color-coded thresholds).

---

## 4. Acceptance Criteria

- [ ] `harnez usage --watch` displays local disk storage usage (used / total / percent) and active I/O throughput.
- [ ] Disk metrics update reliably on each tick of the usage watch loop.
- [ ] Cross-platform compilation and runtime compatibility for Linux (primary) and Darwin (macOS).
- [ ] Unit and telemetry collector tests verify parsing and calculation logic.
