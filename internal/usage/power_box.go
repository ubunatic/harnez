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

// The value columns follow the label and sparkline column. Both are
// left-aligned under their header words: the now value starts under the "n"
// of "now", the peak value under the "p" of "peak".
const (
	// powerNowHead is the header over the now column ("now" then " · ").
	powerNowHead = "now · "
	// powerPeakHead is the header over the peak column.
	powerPeakHead = "peak"
	// powerNowMin is the now column's minimum width: the header's "now · ".
	powerNowMin = 6
	// powerPeakMin is the peak column's minimum width: the header "peak".
	powerPeakMin = 4
)

// powerColumns holds the widths of the two value columns for one frame.
type powerColumns struct {
	nowW, peakW int
}

// powerColumnsFor sizes the value columns to the widest value in view. A
// now value keeps one space before the peak column, so a 5-character value
// ("1200W") fits the 6-wide now column, and a 6-character one widens it. A
// peak value widens the peak column when it is wider than "peak".
func powerColumnsFor(view powerView) powerColumns {
	c := powerColumns{nowW: powerNowMin, peakW: powerPeakMin}
	for _, r := range view.Rows {
		c.nowW = max(c.nowW, visLen(powerWatts(r.Now))+1)
		c.peakW = max(c.peakW, visLen(powerWatts(r.Peak)))
	}
	return c
}

// powerWatts formats one reading for a value column.
func powerWatts(w float64) string {
	return fmt.Sprintf("%.0fW", w)
}

// powerPad pads s with spaces on the right to visible width w.
func powerPad(s string, w int) string {
	if pad := w - visLen(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// powerLeftWidth is the label and sparkline column: label, a space, the
// bracketed sparkline, and one column of gap before the values.
func powerLeftWidth() int {
	return loadLabelWidth + 1 + powerChartWidth + 3
}

// buildPowerBox renders the Power box. Its width follows the widest value
// (see powerColumnsFor), so the header and every row share one set of columns.
//
// The top border is "┌─ " + title + " ┐" (titleToCorner), with no dash
// between the title and the corner. The title runs from column 3 to the
// column before the corner, so its "peak" ends right above the last column of
// the peak values' padding, and "now" starts at the column of the now values.
func buildPowerBox(view powerView) wbox {
	leftW := powerLeftWidth()
	cols := powerColumnsFor(view)
	head := watchBoxSymbol("power") + " Power"
	// Title visible width: head, a space, fill dashes, a space, then the
	// column header. Header "now" lands at column leftW of the content.
	fill := leftW - 3 - visLen(head)
	if fill < 1 {
		fill = 1
	}
	title := head + " " + strings.Repeat("─", fill) + " " +
		powerPad(powerNowHead, cols.nowW) + powerPad(powerPeakHead, cols.peakW)

	lines := make([]string, 0, len(view.Rows))
	for _, r := range view.Rows {
		lines = append(lines, powerRowLine(r, leftW, cols))
	}
	contentW := leftW + cols.nowW + cols.peakW
	return wbox{title: title, lines: lines, width: contentW + 4, titleToCorner: true}
}

// powerRowLine renders "label [spark]" padded to leftW, then the now and peak
// values, each left-aligned in its column.
func powerRowLine(r powerRowView, leftW int, cols powerColumns) string {
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
	return left + powerPad(powerWatts(r.Now), cols.nowW) + powerPad(powerWatts(r.Peak), cols.peakW)
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
