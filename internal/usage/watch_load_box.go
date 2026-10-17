package usage

import (
	"fmt"

	"ubunatic.com/harnez/internal/rograph"
)

// buildLoadBox renders a compact panel showing CPU and GPU load, styled
// like the agent quota lines: "label (detail) [spark] avg% (temp)".
//
// remoteHost and snapshot together select the data source (issue 090): with
// remoteHost == "", this is local mode and CurrentCPULoad()/CurrentGPUs()
// are called directly, giving the 1Hz-redraw live behavior local mode has
// always had. With remoteHost != "", this is remote mode: the panel renders
// exclusively from snapshot (last fetched at the remote collection
// interval, not the 1Hz redraw cadence — no local /proc or /sys read is
// substituted, and no SSH call happens here). A nil snapshot in remote mode
// renders an explicit "remote load unavailable" placeholder rather than
// silently falling back to local telemetry.
func buildLoadBox(width int, remoteHost string, snapshot *LoadSnapshot) wbox {
	title := watchBoxSymbol("load") + " Load"
	if remoteHost != "" {
		title = fmt.Sprintf("%s Load %s(@%s)\x1b[0m", watchBoxSymbol("load"), ansiDimGrey, remoteHost)
	}
	return wbox{title: title, lines: buildLoadBoxLines(remoteHost, snapshot), width: width}
}

// buildRemoteLoadBox renders issue 110's independent remote Load box: the
// CPU/GPU load of load.watch_host, a separate panel from the existing local
// (or --host-retitled) [L] Load box above — not merged into it (Decision
// §5). Shares buildLoadBox's row-rendering via buildLoadBoxLines; only the
// title/panel key differ. host must be non-empty — callers only build this
// panel at all when load.watch_host is configured (Acceptance Criterion 1).
//
// The title also carries a "streaming"/"batch" label so it's always visible
// whether the current data arrived over the persistent --watch-only stream
// or a one-shot poll (issue 110's follow-up UI request) — never hidden
// state the user has to infer from update cadence.
func buildRemoteLoadBox(width int, host string, snapshot *LoadSnapshot, streaming bool) wbox {
	mode := "batch"
	if streaming {
		mode = "streaming"
	}
	title := fmt.Sprintf("%s[R]\x1b[0m Remote Load %s(@%s · %s)\x1b[0m", ansiBold, ansiDimGrey, host, mode)
	return wbox{title: title, lines: buildLoadBoxLines(host, snapshot), width: width}
}

// buildLoadBoxLines renders the shared CPU/GPU row content for both
// buildLoadBox and buildRemoteLoadBox.
//
// remoteHost and snapshot together select the data source (issue 090): with
// remoteHost == "", this is local mode and CurrentCPULoad()/CurrentGPUs()
// are called directly, giving the 1Hz-redraw live behavior local mode has
// always had. With remoteHost != "", this is remote mode: the panel renders
// exclusively from snapshot (last fetched at the remote collection
// interval, not the 1Hz redraw cadence — no local /proc or /sys read is
// substituted, and no SSH call happens here). A nil snapshot in remote mode
// renders an explicit "remote load unavailable" placeholder rather than
// silently falling back to local telemetry.
func buildLoadBoxLines(remoteHost string, snapshot *LoadSnapshot) []string {
	if remoteHost != "" && snapshot == nil {
		return []string{ansiDimGrey + "remote load unavailable\x1b[0m"}
	}

	var load CPULoad
	var gpus []GPU
	if snapshot != nil {
		load = snapshot.CPU
		gpus = snapshot.GPUs
	} else {
		load = CurrentCPULoad()
		gpus = CurrentGPUs()
	}

	var lines []string
	if load.NumCPU > 0 {
		lines = append(lines, formatCPULine(load))
	}
	if load.Memory.Ok {
		lines = append(lines, formatSystemMemoryLine(load.Memory))
	}

	if len(gpus) == 0 {
		lines = append(lines, ansiDimGrey+"gpu n/a\x1b[0m")
	} else {
		for _, g := range gpus {
			lines = append(lines, formatGPULine(g))
			if g.HaveMem {
				lines = append(lines, formatGPUMemoryLines(g)...)
			}
		}
	}
	if len(lines) == 0 {
		lines = []string{ansiDimGrey + "load data unavailable\x1b[0m"}
	}
	return lines
}

// loadLabelWidth is the fixed column width of the "cpu (...)"/"gpu (...)"
// label in the Load box, so every line's "[spark]" starts at the same
// column regardless of core count or GPU name length.
const loadLabelWidth = 16

// padLoadLabel truncates label to loadLabelWidth (with a trailing "…") if
// it's too long, then pads it to exactly that width. Thin wrapper around
// the shared rograph.PadLabel helper.
func padLoadLabel(label string) string {
	return rograph.PadLabel(label, loadLabelWidth)
}

// formatCPULine renders the "cpu (N cores) [<chart>] avg% (temp)" line. The
// spark is a timeline of recent aggregate real-time % samples, not a
// per-core snapshot — a spatial snapshot barely changes frame to frame,
// while a trend over the last ~10 samples actually shows something moving.
func formatCPULine(load CPULoad) string {
	series := load.PercentHistory
	if len(series) == 0 {
		val := 0.0
		if load.CPUPercentOk {
			val = load.CPUPercent
		}
		series = []float64{val}
	}
	series = padHistory(series, load.CPUPercent, loadHistoryLen)
	avgPart := "n/a"
	if load.CPUPercentOk {
		avgPart = watchLoadPercent(load.CPUPercent)
	}
	tempPart := ""
	if load.TempOk {
		tempPart = fmt.Sprintf(" (%.0f°C)", load.TempC)
	}
	label := padLoadLabel(fmt.Sprintf("cpu (%d cores)", load.NumCPU))

	var chart string
	mode := mustIndicators().LoadCharts.CPUMode()
	if mode == LoadChartBar {
		val := 0.0
		if load.CPUPercentOk {
			val = load.CPUPercent
		}
		chart = rograph.RenderBar(val, watchLoadBarOptions(val))
	} else {
		chart = fmt.Sprintf("[%s]", watchPercentSparkline(series, rograph.MaxWidth, mode))
	}
	return fmt.Sprintf("%s %s %s%s", label, chart, avgPart, tempPart)
}

func padHistory(series []float64, current float64, width int) []float64 {
	if len(series) >= width {
		return series
	}
	fillVal := current
	if len(series) > 0 {
		fillVal = series[0]
	}
	res := make([]float64, width)
	padCount := width - len(series)
	for i := 0; i < padCount; i++ {
		res[i] = fillVal
	}
	copy(res[padCount:], series)
	return res
}

func formatSystemMemoryLine(mem SystemMemory) string {
	label := FormatRAMLabel(mem)
	pct := percent(mem.UsedMiB, mem.TotalMiB)

	var chart string
	mode := mustIndicators().LoadCharts.RAMMode()
	if mode != LoadChartBar {
		series := padHistory(mem.PercentHistory, pct, rograph.MaxWidth*2)
		chart = fmt.Sprintf("[%s]", watchPercentSparkline(series, rograph.MaxWidth, mode))
	} else {
		chart = rograph.RenderBar(pct, watchLoadBarOptions(pct))
	}

	return fmt.Sprintf("%s %s %s/%sG %s", label, chart, formatGiB(mem.UsedMiB), formatGiB(mem.TotalMiB), watchLoadPercent(pct))
}

// formatGPULine renders the "gpu (name) [<chart>] avg% (temp)" line. The
// sparkline shows recent history (UtilHistory), or degenerates to a single
// current-value glyph if no history has been collected yet.
func formatGPULine(g GPU) string {
	series := g.UtilHistory
	if len(series) == 0 {
		series = []float64{g.UtilPercent}
	}
	series = padHistory(series, g.UtilPercent, loadHistoryLen)
	label := padLoadLabel(fmt.Sprintf("gpu (%s)", g.Name))
	tempPart := ""
	if g.HaveTemp {
		tempPart = fmt.Sprintf(" (%.0f°C)", g.TempC)
	}

	var chart string
	mode := mustIndicators().LoadCharts.GPUMode()
	if mode == LoadChartBar {
		chart = rograph.RenderBar(g.UtilPercent, watchLoadBarOptions(g.UtilPercent))
	} else {
		chart = fmt.Sprintf("[%s]", watchPercentSparkline(series, rograph.MaxWidth, mode))
	}
	return fmt.Sprintf("%s %s %s%s", label, chart, watchLoadPercent(g.UtilPercent), tempPart)
}

// formatGPUMemoryLines renders GPU memory as a single "gpu vram/gtt" row
// combining VRAM and GTT into adjacent compact bars/sparklines (issue 089, issue 198),
// replacing the earlier two plain-text rows (gpu mem / vram/gtt). It mirrors
// formatAllUsageTableLine's adjacent-dual-bar pattern rather than hand-rolling a new bar renderer.
//
// g.MemUsedMiB/MemTotalMiB/MemPercent already hold the VRAM+GTT combined
// totals (see readAMDGPUMemory in load.go, which accumulates into them from
// whichever of HaveVRAM/HaveGTT is set), so they're used directly here for
// the combined used/total and percentage rather than re-summing.
func formatGPUMemoryLines(g GPU) []string {
	if g.Class == GPUClassDGPU || g.Class == GPUClassEGPU {
		return formatDiscreteGPUMemoryLines(g)
	}
	label := padLoadLabel("gpu vram/gtt")

	var chart string
	if g.HaveVRAM && g.HaveGTT {
		vramPct := percent(g.VRAMUsedMiB, g.VRAMTotalMiB)
		gttPct := percent(g.GTTUsedMiB, g.GTTTotalMiB)
		mode := mustIndicators().LoadCharts.VRAMMode()
		if mode != LoadChartBar {
			vramSeries := padHistory(g.VRAMPercentHistory, vramPct, 8)
			gttSeries := padHistory(g.GTTPercentHistory, gttPct, 8)
			chart = fmt.Sprintf("[%s][%s]", watchPercentSparkline(vramSeries, 4, mode), watchPercentSparkline(gttSeries, 4, mode))
		} else {
			barOpts := watchLoadBarOptions(vramPct)
			barOpts.Width = compactBarWidth
			chart = rograph.RenderBar(vramPct, barOpts)
			barOpts.ForegroundANSI = ""
			if mustIndicators().chartPresentation() == LoadChartHeat {
				barOpts.ForegroundANSI = heatForegroundANSI(gttPct)
			}
			chart += rograph.RenderBar(gttPct, barOpts)
		}
	} else {
		var activePct float64
		var activeHist []float64
		if g.HaveVRAM {
			activePct = percent(g.VRAMUsedMiB, g.VRAMTotalMiB)
			activeHist = g.VRAMPercentHistory
		} else if g.HaveGTT {
			activePct = percent(g.GTTUsedMiB, g.GTTTotalMiB)
			activeHist = g.GTTPercentHistory
		} else {
			activePct = g.MemPercent
		}

		mode := mustIndicators().LoadCharts.VRAMMode()
		if mode != LoadChartBar {
			series := padHistory(activeHist, activePct, rograph.MaxWidth*2)
			chart = fmt.Sprintf("[%s]", watchPercentSparkline(series, rograph.MaxWidth, mode))
		} else {
			chart = rograph.RenderBar(activePct, watchLoadBarOptions(activePct))
		}
	}

	line := fmt.Sprintf("%s %s %s/%sG %s", label, chart, formatGiB(g.MemUsedMiB), formatGiB(g.MemTotalMiB), watchLoadPercent(g.MemPercent))
	return []string{line}
}

// formatDiscreteGPUMemoryLines renders a dGPU's or eGPU's memory as one
// "gpu vram" row showing VRAM only (issue 765). GTT is system RAM the card
// can borrow, not the card's own memory, so it is left out. Returns nothing
// when the card reports no VRAM.
func formatDiscreteGPUMemoryLines(g GPU) []string {
	if !g.HaveVRAM {
		return nil
	}
	label := padLoadLabel("gpu vram")
	pct := percent(g.VRAMUsedMiB, g.VRAMTotalMiB)

	var chart string
	mode := mustIndicators().LoadCharts.VRAMMode()
	if mode != LoadChartBar {
		series := padHistory(g.VRAMPercentHistory, pct, rograph.MaxWidth*2)
		chart = fmt.Sprintf("[%s]", watchPercentSparkline(series, rograph.MaxWidth, mode))
	} else {
		chart = rograph.RenderBar(pct, watchLoadBarOptions(pct))
	}

	line := fmt.Sprintf("%s %s %s/%sG %s", label, chart, formatGiB(g.VRAMUsedMiB), formatGiB(g.VRAMTotalMiB), watchLoadPercent(pct))
	return []string{line}
}

func formatGiB(mib float64) string {
	return fmt.Sprintf("%.1f", mib/1024)
}

func percent(used, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return used / total * 100
}
