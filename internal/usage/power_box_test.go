package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestPowerMeter(t *testing.T) (*powerMeter, func(time.Duration)) {
	t.Helper()
	now, advance := fakePowerClock(time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC))
	store := newPowerPeakStore(filepath.Join(t.TempDir(), "power-peaks.json"), "boot-a", now)
	return newPowerMeter(store), advance
}

func TestPowerSourcesRAPLCPUAndDGPU(t *testing.T) {
	load := CPULoad{PowerW: 12.4, PowerOk: true, RAPLReadable: true}
	gpus := []GPU{{
		Name: "Navi 31", PowerW: 287, HavePower: true, Class: GPUClassDGPU,
		PCI: "0000:03:00.0", PowerLabel: "",
	}}
	got := powerSourcesFrom(LoadSnapshot{CPU: load, GPUs: gpus})
	want := []powerSource{
		{Key: "cpu", Label: "cpu", Watts: 12.4},
		{Key: "gpu:0000:03:00.0", Label: "gpu (Navi 31)", Watts: 287},
	}
	assertPowerSources(t, got, want)
}

func TestPowerSourcesAPUPPTStandsInWithoutRAPL(t *testing.T) {
	load := CPULoad{NumCPU: 16}
	gpus := []GPU{{
		Name: "Phoenix", PowerW: 13, HavePower: true, PowerLabel: apuPackagePowerLabel,
		Class: GPUClassIGPU, PCI: "0000:c4:00.0",
	}}
	got := powerSourcesFrom(LoadSnapshot{CPU: load, GPUs: gpus})
	assertPowerSources(t, got, []powerSource{{Key: "cpu", Label: "cpu+igpu (PPT)", Watts: 13}})
}

func TestPowerSourcesPPTNeverDoubleCountsWithRAPL(t *testing.T) {
	load := CPULoad{PowerW: 9, PowerOk: true, RAPLReadable: true}
	gpus := []GPU{{
		Name: "Phoenix", PowerW: 13, HavePower: true, PowerLabel: apuPackagePowerLabel,
		Class: GPUClassIGPU, PCI: "0000:c4:00.0",
	}}
	got := powerSourcesFrom(LoadSnapshot{CPU: load, GPUs: gpus})
	assertPowerSources(t, got, []powerSource{{Key: "cpu", Label: "cpu", Watts: 9}})
}

func TestPowerSourcesReadableRAPLWithoutDeltaShowsNoCPUPPT(t *testing.T) {
	load := CPULoad{RAPLReadable: true}
	gpus := []GPU{{
		Name: "Phoenix", PowerW: 13, HavePower: true, PowerLabel: apuPackagePowerLabel,
		Class: GPUClassIGPU, PCI: "0000:c4:00.0",
	}}
	if got := powerSourcesFrom(LoadSnapshot{CPU: load, GPUs: gpus}); len(got) != 0 {
		t.Fatalf("sources = %+v, want none while RAPL has no delta", got)
	}
}

func TestPowerSourcesIGPUWithoutPPTLabelIsIgnored(t *testing.T) {
	gpus := []GPU{{Name: "Phoenix", PowerW: 4, HavePower: true, Class: GPUClassIGPU, PCI: "0000:c4:00.0"}}
	if got := powerSourcesFrom(LoadSnapshot{GPUs: gpus}); len(got) != 0 {
		t.Fatalf("sources = %+v, want none for a non-PPT iGPU reading", got)
	}
}

func TestPowerBoxHiddenWhenNoSourceHasPower(t *testing.T) {
	m, _ := newTestPowerMeter(t)
	now := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	if _, ok := powerBoxFor(m, powerSourcesFrom(LoadSnapshot{CPU: CPULoad{NumCPU: 8}, GPUs: []GPU{{Name: "x"}}}), now); ok {
		t.Fatal("power box shown with no readable power source")
	}
}

func TestPowerMeterTotalOnlyWithTwoRows(t *testing.T) {
	m, advance := newTestPowerMeter(t)
	cpu := powerSource{Key: "cpu", Label: "cpu", Watts: 10}
	gpu := powerSource{Key: "gpu:a", Label: "gpu (Navi 31)", Watts: 100}

	one := m.observe([]powerSource{cpu}, time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC))
	if len(one.Rows) != 1 || one.Rows[0].Label != "cpu" {
		t.Fatalf("one source: rows = %+v, want only cpu", one.Rows)
	}

	advance(2 * time.Second)
	two := m.observe([]powerSource{cpu, gpu}, time.Date(2026, 11, 4, 9, 0, 0, 2, time.UTC))
	if len(two.Rows) != 3 || two.Rows[2].Label != "total" {
		t.Fatalf("two sources: rows = %+v, want cpu, gpu, total", two.Rows)
	}
	if two.Rows[2].Now != 110 {
		t.Fatalf("total now = %v, want 110", two.Rows[2].Now)
	}
}

func TestPowerMeterTotalPeakIsPeakOfSum(t *testing.T) {
	m, advance := newTestPowerMeter(t)
	start := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	m.observe([]powerSource{{Key: "cpu", Label: "cpu", Watts: 30}, {Key: "gpu:a", Label: "gpu (a)", Watts: 0}}, start)
	advance(2 * time.Second)
	v := m.observe([]powerSource{{Key: "cpu", Label: "cpu", Watts: 0}, {Key: "gpu:a", Label: "gpu (a)", Watts: 300}}, start.Add(2*time.Second))

	total := v.Rows[2]
	if total.Peak != 300 {
		t.Fatalf("total peak = %v, want peak of the sum 300 (not the sum of peaks 330)", total.Peak)
	}
	if v.Rows[0].Peak != 30 || v.Rows[1].Peak != 300 {
		t.Fatalf("row peaks = %v, %v, want 30 and 300", v.Rows[0].Peak, v.Rows[1].Peak)
	}
}

func TestPowerMeterSeriesSamplesAtLoadCadence(t *testing.T) {
	m, advance := newTestPowerMeter(t)
	start := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	cpu := func(w float64) []powerSource { return []powerSource{{Key: "cpu", Label: "cpu", Watts: w}} }
	m.observe(cpu(5), start)
	advance(200 * time.Millisecond)
	v := m.observe(cpu(7), start.Add(200*time.Millisecond))
	if got := v.Rows[0].Series; len(got) != 1 || got[0] != 7 {
		t.Fatalf("series inside 1s = %v, want the latest value only [7]", got)
	}
	advance(time.Second)
	v = m.observe(cpu(9), start.Add(1200*time.Millisecond))
	if got := v.Rows[0].Series; len(got) != 2 || got[0] != 7 || got[1] != 9 {
		t.Fatalf("series after 1s = %v, want [7 9]", got)
	}
}

func TestPowerBoxSparklineScalesToBootPeak(t *testing.T) {
	values := powerSparkPercent([]float64{0, 19, 38, 76}, 38)
	want := []float64{0, 50, 100, 100}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("sparkline percent = %v, want %v", values, want)
		}
	}
	if got := powerSparkPercent([]float64{0, 5}, 0); got[1] != 0 {
		t.Fatalf("zero peak must not divide: got %v", got)
	}
}

func assertPowerSources(t *testing.T, got, want []powerSource) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("sources = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("source %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// powerBoxGoldenView is the fixed fake CPU (APU PPT), dGPU, and total view the
// golden test renders. Series are watts, oldest first.
func powerBoxGoldenView() powerView {
	series := []float64{1, 2, 2, 3, 5, 6, 8, 9, 11, 12, 13, 11, 9, 8, 7, 10, 12, 13, 13, 13}
	gpu := []float64{7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7}
	total := make([]float64, len(series))
	for i := range series {
		total[i] = series[i] + gpu[i]
	}
	return powerView{Rows: []powerRowView{
		{Label: "cpu+igpu (PPT)", Now: 13, Series: series, Peak: 38},
		{Label: "gpu (Navi 31)", Now: 7, Series: gpu, Peak: 287},
		{Label: "total", Now: 20, Series: total, Peak: 311},
	}}
}

func TestPowerBoxGolden(t *testing.T) {
	t.Setenv("LOOMCOLOR", "truecolor")
	box := buildPowerBox(48, powerBoxGoldenView())
	got := strings.Join(renderWBox(box), "\n") + "\n"
	path := filepath.Join("testdata", "watch-box-baseline", "power-box.ansi")
	if *updateWatchBoxGoldens {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update-watch-box-goldens)", path, err)
	}
	if got != string(want) {
		t.Fatalf("power box golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestPowerBoxHeaderAlignsWithValues(t *testing.T) {
	box := buildPowerBox(60, powerBoxGoldenView())
	rows := renderWBox(box)
	top := stripANSI(rows[0])
	valueRow := stripANSI(rows[1])
	// Columns are counted in runes. The title starts at col 3 ("┌─ "), the
	// content at col 2 ("│ "). The header's "peak" must end in the same column
	// as the peak value's last character.
	peakHeaderCol := len([]rune(top[:strings.Index(top, "peak")+len("peak")-1])) // rune index of 'k'
	peakValueCol := len([]rune(valueRow[:strings.LastIndex(valueRow, "W")]))     // rune index of the final 'W'
	if peakHeaderCol != peakValueCol {
		t.Fatalf("header 'peak' ends at col %d, peak values end at col %d\ntop: %s\nrow: %s",
			peakHeaderCol, peakValueCol, top, valueRow)
	}
}
