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
}

func newPowerMeter(peaks *powerPeakStore) *powerMeter {
	return &powerMeter{peaks: peaks, hist: map[string]*sampleHistory{}}
}

// observe records one sample of sources at now and returns the rows to draw.
// History advances at most once per minLoadSampleInterval, as in the Load box.
// Peaks update on every call.
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
	if len(sources) >= 2 {
		series := m.total.appendAt(sum, now, minLoadSampleInterval)
		view.Rows = append(view.Rows, powerRowView{
			Label:  "total",
			Now:    sum,
			Series: series,
			Peak:   m.peaks.Observe("total", sum),
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

// powerValueWidth is the width of the "now" and "peak" value fields.
const (
	powerNowWidth  = 5
	powerPeakWidth = 6
)

// buildPowerBox renders the Power box at width. Its natural width comes from
// its content and does not depend on width, so the layout planner can measure
// it. The "now"/"peak" header is laid out to end over the peak values.
func buildPowerBox(width int, view powerView) wbox {
	leftW := loadLabelWidth + 1 + powerChartWidth + 2
	head := watchBoxSymbol("power") + " Power"
	// The header's "now" must start at leftW+2, so the title's head plus the
	// fill dashes take exactly that many columns.
	fill := leftW + 2 - (visLen(head) + 1)
	if fill < 1 {
		fill = 1
	}
	title := head + " " + strings.Repeat("─", fill) + "now   peak"

	lines := make([]string, 0, len(view.Rows))
	for _, r := range view.Rows {
		lines = append(lines, powerRowLine(r, leftW))
	}
	return wbox{title: title, lines: lines, width: width}
}

// powerRowLine renders "label [spark]" padded to leftW, then right-aligned
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
	return left + " " + now + " " + peak
}

// Shared meter for the live watch. It is created on first use, so a process
// that never shows the box never touches the peak file.
var (
	sharedPowerOnce sync.Once
	sharedPower     *powerMeter
)

func sharedPowerMeter() *powerMeter {
	sharedPowerOnce.Do(func() {
		sharedPower = newPowerMeter(newPowerPeakStore(powerPeaksPath(), readBootID(powerBootIDPath), time.Now))
	})
	return sharedPower
}

// flushPowerPeaks writes any pending boot peak. The watch calls it on exit so
// a peak from the last few seconds is not lost to the save throttle.
func flushPowerPeaks() error {
	return sharedPowerMeter().peaks.Flush()
}
