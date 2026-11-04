package usage

import (
	"strings"
	"testing"
	"time"
)

// useSharedPowerMeterForTest swaps the watch's shared meter for m until the
// test ends, so frame tests never touch the real state directory.
func useSharedPowerMeterForTest(t *testing.T, m *powerMeter) {
	t.Helper()
	sharedPowerMu.Lock()
	old := sharedPower
	sharedPower = m
	sharedPowerMu.Unlock()
	t.Cleanup(func() {
		sharedPowerMu.Lock()
		sharedPower = old
		sharedPowerMu.Unlock()
	})
}

func powerFrameSummary(load LoadSnapshot) UsageSummary {
	return UsageSummary{
		Timestamp: time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC),
		Agents:    []AgentUsage{{AgentID: "claude", Name: "Claude", Installed: true}},
		Load:      &load,
	}
}

func powerFrameText(sec watchSections, summary UsageSummary) string {
	frame := buildWatchFrame(summary, nil, time.Minute, sec, 140, 120, false)
	return stripANSI(strings.Join(frame.lines, "\n"))
}

func powerSectionsOn(on bool) watchSections {
	sec := initialWatchSections(WatchOptions{})
	sec.Power = on
	return sec
}

var powerFrameLoadWithPower = LoadSnapshot{
	CPU: CPULoad{PowerW: 12, PowerOk: true, RAPLReadable: true},
	GPUs: []GPU{{
		Name: "Navi 31", PowerW: 287, HavePower: true, Class: GPUClassDGPU,
		PCI: "0000:03:00.0",
	}},
}

func TestPowerBoxAppearsInWatchFrameWhenOnWithPower(t *testing.T) {
	m, _ := newTestPowerMeter(t)
	useSharedPowerMeterForTest(t, m)
	text := powerFrameText(powerSectionsOn(true), powerFrameSummary(powerFrameLoadWithPower))
	if !strings.Contains(text, "[W] Power") || !strings.Contains(text, "now · peak") {
		t.Fatalf("[W] box missing from frame with power on:\n%s", text)
	}
	if !strings.Contains(text, "gpu (Navi 31)") {
		t.Fatalf("dGPU row missing from [W] box:\n%s", text)
	}
}

func TestPowerBoxAbsentFromWatchFrameWhenToggledOff(t *testing.T) {
	m, _ := newTestPowerMeter(t)
	useSharedPowerMeterForTest(t, m)
	text := powerFrameText(powerSectionsOn(false), powerFrameSummary(powerFrameLoadWithPower))
	if strings.Contains(text, "Power ") {
		t.Fatalf("[W] box shown with the power section off:\n%s", text)
	}
}

func TestPowerBoxAbsentFromWatchFrameWithoutPowerSource(t *testing.T) {
	m, _ := newTestPowerMeter(t)
	useSharedPowerMeterForTest(t, m)
	load := LoadSnapshot{CPU: CPULoad{NumCPU: 8}, GPUs: []GPU{{Name: "x"}}}
	text := powerFrameText(powerSectionsOn(true), powerFrameSummary(load))
	if strings.Contains(text, "[W] Power") {
		t.Fatalf("[W] box shown with no power source:\n%s", text)
	}
}

// TestPowerTotalSurvivesPartialSample: once two sources have been seen the
// total row stays, and a sample missing a source does not record a total peak.
func TestPowerTotalSurvivesPartialSample(t *testing.T) {
	m, advance := newTestPowerMeter(t)
	start := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	cpu := powerSource{Key: "cpu", Label: "cpu", Watts: 10}
	gpu := powerSource{Key: "gpu:a", Label: "gpu (a)", Watts: 100}

	full := m.observe([]powerSource{cpu, gpu}, start)
	if full.Rows[2].Peak != 110 {
		t.Fatalf("full sample total peak = %v, want 110", full.Rows[2].Peak)
	}

	advance(2 * time.Second)
	partial := m.observe([]powerSource{{Key: "cpu", Label: "cpu", Watts: 500}}, start.Add(2*time.Second))
	if len(partial.Rows) != 2 || partial.Rows[1].Label != "total" {
		t.Fatalf("partial sample rows = %+v, want cpu and a kept total row", partial.Rows)
	}
	total := partial.Rows[1]
	if total.Now != 500 {
		t.Fatalf("partial total now = %v, want the partial sum 500", total.Now)
	}
	if total.Peak != 110 {
		t.Fatalf("partial sample changed total peak to %v, want kept 110", total.Peak)
	}

	advance(2 * time.Second)
	back := m.observe([]powerSource{cpu, gpu}, start.Add(4*time.Second))
	if back.Rows[2].Peak != 110 {
		t.Fatalf("full sample after partial: total peak = %v, want 110", back.Rows[2].Peak)
	}
}

func TestPowerSourcesRAPLCPUOnly(t *testing.T) {
	got := powerSourcesFrom(LoadSnapshot{CPU: CPULoad{PowerW: 38, PowerOk: true, RAPLReadable: true}})
	assertPowerSources(t, got, []powerSource{{Key: "cpu", Label: "cpu", Watts: 38}})
}

// TestPowerFlushWithoutUseDoesNotCreateMeter: an exit flush from a watch that
// never showed the box must not build the meter or touch the peak file.
func TestPowerFlushWithoutUseDoesNotCreateMeter(t *testing.T) {
	sharedPowerMu.Lock()
	old := sharedPower
	sharedPower = nil
	sharedPowerMu.Unlock()
	t.Cleanup(func() {
		sharedPowerMu.Lock()
		sharedPower = old
		sharedPowerMu.Unlock()
	})

	if err := flushPowerPeaks(); err != nil {
		t.Fatalf("flushPowerPeaks() = %v, want nil", err)
	}
	sharedPowerMu.Lock()
	created := sharedPower != nil
	sharedPowerMu.Unlock()
	if created {
		t.Fatal("flushPowerPeaks created the shared meter without any use")
	}
}
