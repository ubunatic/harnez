# 343 — Display host disk usage and storage meters in Load telemetry TUI

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature

---

## 1. Problem Statement & Motivation

The `harnez usage` / Load telemetry widget currently monitors CPU, RAM, GPU utilization, and GPU VRAM/GTT memory:

```
 ┌─ ⁷ Load ─────────────────────────────────────┐
 │ cpu (12 cores)   [⣀⣄⣀⣀⣀⣀⣀⣀⣀⣀] 18% (76°C)     │
 │ ram (23G)        [⣶⣶⣶⣶⣶⣶⣶⣶⣶⣶] 16.7/23.3G 72% │
 │ gpu (Cezanne)    [⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀] 2% (54°C)      │
 │ gpu vram/gtt     [⣿⣿⣿⣿][⣀⣀⣀⣀] 6.8/27.0G 25%  │
 └──────────────────────────────────────────────┘
```

However, host storage utilization (NVMe SSDs, SATA drives, virtual disk backends, and mounted USB storage) is not visible. When running heavy workloads such as containerized guest operating systems (`dockur/macos`), large sparse disk images (`data.img`), VM snapshots, or dataset processing, storage capacity can deplete without early warning.

---

## 2. Design Options & Layout Explorations

### Option A: Dedicated Row(s) Below GPU Meters (Single or Multi-Line)
Add one or more dedicated rows under GPU meters:
* **Single-line compact braille**:
  ```
  │ disks (nvme/hdd) [⣿⣿ ] nvme01 [⣿  ] hdd1     │
  ```
* **Single-line sparkline**:
  ```
  │ disks (2 dev)     ▂ nvme01 (42%)  ▅ hdd1 (85%) │
  ```
* **Adaptive multi-line**: Expand to 2+ rows dynamically when 3+ block devices/mountpoints are present.

### Option B: Two-Column Side-by-Side (Preserving 4-Row Height)
Split the Load box horizontally into a compute/memory column (left) and a storage/disk column (right). To fit within standard terminal widths, CPU/RAM/GPU history sparklines can be squeezed to 3 characters (6 half-width braille time buckets):

```
 ┌─ ⁷ Load ────────────────────────────────────────────────────────┐
 │ cpu (12 cores) [⣀⣄] 18% (76°C) │ nvme01 [⣿⣿⣿⣀]  42% (760G/1.8T) │
 │ ram (23G)      [⣶⣶] 16.7/23G │ hdd1   [⣿⣿⣿⣿]  85% (3.1T/3.6T) │
 │ gpu (Cezanne)  [⣀⣀] 2% (54°C)  │ backup [⣿⣀⣀⣀]  15% (120G/800G) │
 │ gpu vram/gtt   [⣿][⣀] 6.8/27G  │ usb0   [⣿⣿⣀⣀]  50% (256G/512G) │
 └─────────────────────────────────────────────────────────────────┘
```

### Option C: Adaptive Width Switching (Responsive)
* **Narrow view (< 60 columns)**: 4-row layout with a single summary disk line (`disks: nvme01 42% | hdd1 85%`).
* **Standard/Wide view (>= 60 columns)**: Two-column layout with individual device usage bars and capacity metrics on the right.

---

## 3. Data Collection Strategy & Portability

* **Linux**:
  * Discover active root / mount points via `/proc/mounts` or `/proc/self/mountinfo`, filtering out pseudo-filesystems (`tmpfs`, `devtmpfs`, `overlay`, `cgroup`, `proc`, `sysfs`).
  * Query block device stats and free/used bytes via `unix.Statfs(mountPath, &stat)` (`golang.org/x/sys/unix`).
  * Map mount points to friendly block device labels (e.g. `nvme0n1p2` &rarr; `nvme01` / `/`).
* **macOS (Darwin)**:
  * Query `getmntinfo()` or `unix.Statfs` on `/` and `/Volumes/*` to report APFS container and external disk consumption.

---

## 4. Acceptance Criteria

- [ ] `internal/usage` collects storage metrics for primary host block devices and user mount points.
- [ ] Disk metrics include total capacity, used capacity, percentage, and friendly device/mount names.
- [ ] TUI renderer supports displaying disk meters without breaking existing 4-row or compact views.
- [ ] Responsive fallback for wide vs narrow terminals.
- [ ] Unit tests in `internal/usage` verifying disk stat parsing and formatting across Linux and Darwin.
