package usage

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"ubunatic.com/harnez/internal/rograph"
)

// The Power box ([W]) shows every power reading the host reports as a row:
// one sparkline, the current watts ("now"), and the highest watts seen since
// boot ("peak"). The sparkline is scaled to that boot peak, so it is always
// 0-100% of the source's own peak and reuses the Load box's percent sparkline.
//
// Sources, with no double counting:
//   - CPU package power from RAPL ("cpu"), or the APU's PPT reading when RAPL
//     is unreadable ("cpu+igpu (PPT)"). PPT already contains the iGPU, so it
//     never appears on an iGPU row.
//   - Each dGPU/eGPU with a power reading ("gpu (<name>)").
//
// A total row ("total") appears only with two or more sources. Its value is
// the sum of the shown rows at each sample, and its peak is the peak of that
// sum, not the sum of the per-row peaks.

// powerSource is one reading the Power box can show for this sample.
type powerSource struct {
	Key   string // stable identity, also the peak-state key
	Label string // row label, e.g. "cpu", "gpu (Navi 31)"
	Watts float64
}

// powerRowView is one rendered row: the current reading, the recent watts
// window (oldest first, sampled at minLoadSampleInterval), and the boot peak.
type powerRowView struct {
	Label  string
	Now    float64
	Series []float64
	Peak   float64
}

// powerView is everything the Power box draws for one frame.
type powerView struct {
	Rows []powerRowView
}

// powerSourcesFrom lists the power readings in snap. It applies the same APU
// PPT split the Load box uses, so an iGPU's PPT never counts twice.
func powerSourcesFrom(snap LoadSnapshot) []powerSource {
	load, gpus := splitAPUPackagePower(snap.CPU, snap.GPUs)
	var out []powerSource
	if load.PowerOk {
		label := "cpu"
		if !load.RAPLReadable {
			// splitAPUPackagePower only sets PowerOk from PPT when RAPL is
			// unreadable, so this reading is the APU package.
			label = "cpu+igpu (PPT)"
		}
		out = append(out, powerSource{Key: "cpu", Label: label, Watts: load.PowerW})
	}
	for _, g := range gpus {
		if !g.HavePower || (g.Class != GPUClassDGPU && g.Class != GPUClassEGPU) {
			continue
		}
		key := "gpu:" + g.PCI
		if g.PCI == "" {
			key = "gpu:" + g.Name
		}
		out = append(out, powerSource{Key: key, Label: fmt.Sprintf("gpu (%s)", g.Name), Watts: g.PowerW})
	}
	return out
}

// powerMeter turns per-frame readings into a powerView: it keeps a short
// watts history per source, and the boot peaks in peaks.
type powerMeter struct {
	mu    sync.Mutex
	peaks *powerPeakStore
	hist  map[string]*sampleHistory
	total sampleHistory
	// maxSources is the most sources seen in one sample. The total row is
	// kept once it reaches 2, and a sample with fewer sources is partial.
	maxSources int
}

func newPowerMeter(peaks *powerPeakStore) *powerMeter {
	return &powerMeter{peaks: peaks, hist: map[string]*sampleHistory{}}
}

// observe records one sample of sources at now and returns the rows to draw.
// History advances at most once per minLoadSampleInterval, as in the Load box.
// Per-source peaks update on every call.
//
// The total row stays once two sources have been seen. A sample with fewer
// sources than the most seen is partial: its sum is drawn, but it does not
// record a total peak, so a missing source cannot inflate the peak.
func (m *powerMeter) observe(sources []powerSource, now time.Time) powerView {
	m.mu.Lock()
	defer m.mu.Unlock()
	var view powerView
	sum := 0.0
	for _, src := range sources {
		h := m.hist[src.Key]
		if h == nil {
			h = &sampleHistory{}
			m.hist[src.Key] = h
		}
		series := h.appendAt(src.Watts, now, minLoadSampleInterval)
		view.Rows = append(view.Rows, powerRowView{
			Label:  src.Label,
			Now:    src.Watts,
			Series: series,
			Peak:   m.peaks.Observe(src.Key, src.Watts),
		})
		sum += src.Watts
	}
	if len(sources) > m.maxSources {
		m.maxSources = len(sources)
	}
	if m.maxSources >= 2 {
		complete := len(sources) == m.maxSources
		series := m.total.appendAt(sum, now, minLoadSampleInterval)
		peak := m.peaks.Peak("total")
		if complete {
			peak = m.peaks.Observe("total", sum)
		}
		view.Rows = append(view.Rows, powerRowView{
			Label:  "total",
			Now:    sum,
			Series: series,
			Peak:   peak,
		})
	}
	return view
}

// powerBoxFor returns the frame's Power view. ok is false when no source has
// a reading, which hides the box entirely.
func powerBoxFor(m *powerMeter, sources []powerSource, now time.Time) (powerView, bool) {
	if len(sources) == 0 {
		return powerView{}, false
	}
	return m.observe(sources, now), true
}

// powerSparkPercent scales watts to 0-100 against peak, the sparkline's
// range. A zero peak gives all zeros rather than a division by zero.
func powerSparkPercent(series []float64, peak float64) []float64 {
	out := make([]float64, len(series))
	if peak <= 0 {
		return out
	}
	for i, w := range series {
		pct := w / peak * 100
		switch {
		case pct < 0:
			pct = 0
		case pct > 100:
			pct = 100
		}
		out[i] = pct
	}
	return out
}

// powerChartWidth is the sparkline's glyph count, excluding the brackets.
const powerChartWidth = rograph.MaxWidth

// The value fields: "now" (watts) then one space then "peak" (watts), both
// right-aligned, so the peak ends one column before the border.
const (
	powerNowWidth  = 5
	powerPeakWidth = 6
	// powerHeadTail is the header text drawn over the value columns.
	powerHeadTail = "now   peak"
)

// powerLeftWidth is the label and sparkline column: label, a space, the
// bracketed sparkline, and one column of gap before the values.
func powerLeftWidth() int {
	return loadLabelWidth + 1 + powerChartWidth + 3
}

// powerContentWidth is the width of one row's content, excluding the borders.
func powerContentWidth() int {
	return powerLeftWidth() + powerNowWidth + 1 + powerPeakWidth
}

// powerBoxWidth is the box's fixed width: content plus "│ " and " │".
// It is independent of the planner's width, so the header and the values
// stay aligned and no spare columns are added to the right.
func powerBoxWidth() int {
	return powerContentWidth() + 4
}

// buildPowerBox renders the Power box. Its width is fixed (powerBoxWidth), so
// the layout planner measures and draws the same box.
//
// renderWBox draws the top as "┌─ " + title + " " + dashes + "┐", with at
// least one dash, so the title can be at most powerContentWidth()-2 wide.
// Its "peak" therefore ends one column before the last digit of the values.
func buildPowerBox(view powerView) wbox {
	head := watchBoxSymbol("power") + " Power"
	fill := powerContentWidth() - 2 - (visLen(head) + 1) - 1 - len(powerHeadTail)
	if fill < 1 {
		fill = 1
	}
	title := head + " " + strings.Repeat("─", fill) + " " + powerHeadTail

	lines := make([]string, 0, len(view.Rows))
	for _, r := range view.Rows {
		lines = append(lines, powerRowLine(r, powerLeftWidth()))
	}
	return wbox{title: title, lines: lines, width: powerBoxWidth()}
}

// powerRowLine renders "label [spark]" padded to leftW, then the right-aligned
// now and peak values.
func powerRowLine(r powerRowView, leftW int) string {
	pct := powerSparkPercent(r.Series, r.Peak)
	nowPct := 0.0
	if r.Peak > 0 {
		nowPct = min(max(r.Now/r.Peak*100, 0), 100)
	}
	pct = padHistory(pct, nowPct, loadHistoryLen)
	mode := mustIndicators().LoadCharts.CPUMode()
	if strings.HasPrefix(r.Label, "gpu") {
		mode = mustIndicators().LoadCharts.GPUMode()
	}
	left := padLoadLabel(r.Label) + " [" + watchPercentSparkline(pct, powerChartWidth, mode) + "]"
	if pad := leftW - visLen(left); pad > 0 {
		left += strings.Repeat(" ", pad)
	}
	now := fmt.Sprintf("%*s", powerNowWidth, fmt.Sprintf("%.0fW", r.Now))
	peak := fmt.Sprintf("%*s", powerPeakWidth, fmt.Sprintf("%.0fW", r.Peak))
	return left + now + " " + peak
}

// Shared meter for the live watch. It is created on first use, so a process
// that never shows the box never touches the peak file. The mutex guards the
// pointer so the exit flush can check for a meter without creating one.
var (
	sharedPowerMu sync.Mutex
	sharedPower   *powerMeter
)

func sharedPowerMeter() *powerMeter {
	sharedPowerMu.Lock()
	defer sharedPowerMu.Unlock()
	if sharedPower == nil {
		sharedPower = newPowerMeter(newPowerPeakStore(powerPeaksPath(), readBootID(powerBootIDPath), time.Now))
	}
	return sharedPower
}

// flushPowerPeaks writes any pending boot peak. The watch calls it on exit so
// a peak from the last few seconds is not lost to the save throttle. If the
// box was never used there is no meter, so nothing is created or written.
func flushPowerPeaks() error {
	sharedPowerMu.Lock()
	m := sharedPower
	sharedPowerMu.Unlock()
	if m == nil {
		return nil
	}
	return m.peaks.Flush()
}
