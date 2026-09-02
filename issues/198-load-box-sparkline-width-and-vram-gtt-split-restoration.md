# 198 — Load Box RAM/VRAM Sparkline Width & VRAM/GTT Split Restoration

**Status**: Closed
**Priority**: P1 (High)
**Severity**: Minor
**Category**: TUI / Hardware Monitoring
**Related**: [[089-load-panel-combine-gpu-vram-gtt-row]], [[090-load-panel-ram-vram-gtt-memory]], [[191-load-box-ram-chart-hardware-label-and-chart-mode-spec]], `internal/usage/watch.go`, `internal/usage/load.go`, `spec/indicators.yaml`

---

## 1. Problem & Visual Regression

In `harnez usage --watch` and `--compact`, setting RAM and VRAM/GTT charts to `sparkline` caused two severe visual regressions (see screenshot):

```text
┌─ [R] Remote Load (@x600 · streaming) ──────┐
│ cpu (16 cores)   [▁▁▁▁▁▁▁▁▁▁] 1% (27°C)   │
│ ram (45G)        [▄] 19.9/45.1G 44%        │
│ gpu (Phoenix1)   [▁▁▁▁▁▁▁▁▁▁] 0% (26°C)   │
│ gpu vram/gtt     [▁] 1.5/23.1G 6%          │
└────────────────────────────────────────────┘
```

1. **Single-Character Sparklines & Column Misalignment**:
   - `cpu` and `gpu` are 10 characters wide (`[▁▁▁▁▁▁▁▁▁▁]`).
   - `ram` and `gpu vram/gtt` shrunk to a single-character sparkline (`[▄]` and `[▁]`), breaking all vertical column alignment.
   - Root cause: `formatSystemMemoryLine` and `formatGPUMemoryLines` passed `min(rograph.MaxWidth, len(series))` where `len(series) == 1` instead of rendering a full fixed-width sparkline (or seeding the history buffer on startup like CPU/GPU).

2. **Killed VRAM / GTT Dual Split**:
   - Issue 089 explicitly combined VRAM and GTT into two adjacent 4-cell bracketed bars (`[███ ][▏   ]` = 10 chars).
   - The sparkline fallback collapsed them into a single aggregate value `g.MemPercent`, losing the separate VRAM and GTT visual split entirely.

---

## 2. Technical Specification

### 1. Fixed-Width Sparklines Across All Rows (10 Columns)
- All load box sparklines must occupy a fixed width (10 columns matching `rograph.MaxWidth` / `[..........]`).
- When history has fewer than 10 samples (or when starting up / on remote streams):
  - Pad/repeat or burst-seed history (as done for CPU/GPU in `burstSeedGPUHistory`), or render the sparkline over `rograph.MaxWidth` width so the bracket `[...]` is always 10 characters wide.
  - RAM history must maintain `ramHistory` and pad to width 10 so `ram (45G) [<10 chars>]` aligns perfectly with `cpu (16 cores) [<10 chars>]`.

### 2. VRAM & GTT Dual Split Preservation
- In bar mode: keep the adjacent dual 4-cell bars `[VRAM][GTT]` (total 10 chars with brackets, e.g. `[███ ][▏   ]`).
- In sparkline mode: render adjacent dual 4-cell sparkline timelines `[<VRAM-spark-timeline>][<GTT-spark-timeline>]` (total 10 chars with brackets, e.g. `[▁▁▁▁][▁▁▁▁]`).
- Track rolling percent histories for both VRAM and GTT on the GPU struct (`VRAMPercentHistory []float64`, `GTTPercentHistory []float64`).
- If only one of VRAM or GTT is present, render single 10-char sparkline or bar. If both are present, render adjacent 4-cell brackets `[VRAM][GTT]`.

---

## 3. Verification & Resolution

1. Updated `internal/usage/load.go`:
   - Added `VRAMPercentHistory` and `GTTPercentHistory` to `GPU` struct.
   - Added `vramHistory` and `gttHistory` rolling sample histories per card.
   - Populated both in `readAMDGPUMemory`.
2. Updated `internal/usage/watch.go`:
   - Added `padHistory` helper to ensure sparklines always render with exact requested cell width even on initial sample.
   - Fixed `formatSystemMemoryLine` to render width-10 RAM sparklines (`[▁▁▁▁▁▁▁▁▁▁]`).
   - Fixed `formatGPUMemoryLines` to render dual `[....][....]` 4-cell sparkline and bar timelines when both VRAM and GTT are present, aligning all load row columns to 12 visible chars.
3. Added and updated tests in `watch_test.go` and `load_test.go`.
4. Verified with `make test`, `make install`, and live verification via `harnez usage --compact`.
