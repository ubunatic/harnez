package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeRAPLRoot writes a powercap tree with the given energy_uj and, when
// maxRange is non-empty, max_energy_range_uj. It returns the class root.
func fakeRAPLRoot(t *testing.T, energy, maxRange string) string {
	t.Helper()
	root := t.TempDir()
	zone := filepath.Join(root, raplZone)
	if err := os.MkdirAll(zone, 0o755); err != nil {
		t.Fatal(err)
	}
	if energy != "" {
		if err := os.WriteFile(filepath.Join(zone, "energy_uj"), []byte(energy+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if maxRange != "" {
		if err := os.WriteFile(filepath.Join(zone, "max_energy_range_uj"), []byte(maxRange+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// resetRAPLState clears the package-level RAPL baseline so each test starts
// from "no previous sample".
func resetRAPLState(t *testing.T) {
	t.Helper()
	raplMu.Lock()
	lastRAPL, haveRAPL = raplSample{}, false
	raplMu.Unlock()
	t.Cleanup(func() {
		raplMu.Lock()
		lastRAPL, haveRAPL = raplSample{}, false
		raplMu.Unlock()
	})
}

func TestReadRAPLPowerFromTwoSamples(t *testing.T) {
	resetRAPLState(t)
	t0 := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

	root := fakeRAPLRoot(t, "1000000", "")
	if _, ok := readRAPLPower(root, t0); ok {
		t.Fatal("first sample: ok = true, want no reading")
	}

	// 30 J over 2 s is 15 W.
	writeRAPLEnergy(t, root, "31000000")
	w, ok := readRAPLPower(root, t0.Add(2*time.Second))
	if !ok || w != 15 {
		t.Errorf("delta reading: got %v/%v, want 15/true", w, ok)
	}
}

func TestReadRAPLPowerWrapAround(t *testing.T) {
	resetRAPLState(t)
	t0 := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

	// Range is 100 J. Counter goes 90 J -> 10 J: 10 J to the wrap plus
	// 10 J after it is 20 J in 1 s, so 20 W.
	root := fakeRAPLRoot(t, "90000000", "100000000")
	readRAPLPower(root, t0)
	writeRAPLEnergy(t, root, "10000000")
	w, ok := readRAPLPower(root, t0.Add(time.Second))
	if !ok || w != 20 {
		t.Errorf("wrap reading: got %v/%v, want 20/true", w, ok)
	}
}

func TestReadRAPLPowerWrapWithoutRangeGivesNoReading(t *testing.T) {
	resetRAPLState(t)
	t0 := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

	root := fakeRAPLRoot(t, "90000000", "")
	readRAPLPower(root, t0)
	writeRAPLEnergy(t, root, "10000000")
	if _, ok := readRAPLPower(root, t0.Add(time.Second)); ok {
		t.Error("counter reset without max_energy_range_uj: ok = true, want false")
	}
	// The reset still becomes the baseline, so the next delta is valid.
	writeRAPLEnergy(t, root, "12000000")
	if w, ok := readRAPLPower(root, t0.Add(2*time.Second)); !ok || w != 2 {
		t.Errorf("after reset: got %v/%v, want 2/true", w, ok)
	}
}

func TestReadRAPLPowerUnreadableIsSilentAndKeepsBaseline(t *testing.T) {
	resetRAPLState(t)
	t0 := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

	root := fakeRAPLRoot(t, "1000000", "")
	readRAPLPower(root, t0)

	unreadable := fakeRAPLRoot(t, "", "")
	if _, ok := readRAPLPower(unreadable, t0.Add(time.Second)); ok {
		t.Error("unreadable energy_uj: ok = true, want false")
	}

	// The failed read must not replace the baseline: the next readable
	// sample is measured from t0, not from the failed attempt.
	writeRAPLEnergy(t, root, "11000000")
	if w, ok := readRAPLPower(root, t0.Add(2*time.Second)); !ok || w != 5 {
		t.Errorf("after unreadable: got %v/%v, want 5/true (10 J over 2 s)", w, ok)
	}
}

func TestReadRAPLPowerMissingZoneIsSilent(t *testing.T) {
	resetRAPLState(t)
	if _, ok := readRAPLPower(t.TempDir(), time.Now()); ok {
		t.Error("missing powercap tree: ok = true, want false")
	}
}

func TestReadRAPLPowerNonPositiveStepGivesNoReading(t *testing.T) {
	resetRAPLState(t)
	t0 := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

	root := fakeRAPLRoot(t, "1000000", "")
	readRAPLPower(root, t0)
	writeRAPLEnergy(t, root, "2000000")
	if _, ok := readRAPLPower(root, t0); ok {
		t.Error("zero time step: ok = true, want false")
	}
}

func writeRAPLEnergy(t *testing.T, root, energy string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, raplZone, "energy_uj"), []byte(energy+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// apuLoadSnapshot is an iGPU whose power1 label is PPT (the whole package)
// alongside an optional dGPU, with the CPU's RAPL state given by load.
func apuLoadSnapshot(load CPULoad, extra ...GPU) *LoadSnapshot {
	load.NumCPU = 8
	load.TempC, load.TempOk = 42, true
	igpu := GPU{Name: "Cezanne", Class: GPUClassIGPU, UtilPercent: 3, HaveTemp: true, TempC: 55,
		PowerW: 15.4, HavePower: true, PowerLabel: apuPackagePowerLabel}
	return &LoadSnapshot{CPU: load, GPUs: append([]GPU{igpu}, extra...)}
}

func plainLoadLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, stripANSI(l))
	}
	return out
}

func TestAPUPPTMovesToCPULineWithoutRAPL(t *testing.T) {
	lines := plainLoadLines(buildLoadBoxLines("x600", apuLoadSnapshot(CPULoad{})))

	var cpu, gpu string
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "cpu ("):
			cpu = l
		case strings.HasPrefix(l, "gpu ("):
			gpu = l
		}
	}
	if !strings.HasSuffix(cpu, " (42°C/15W)") {
		t.Errorf("cpu line = %q, want suffix \" (42°C/15W)\" (PPT moved to CPU)", cpu)
	}
	if !strings.HasSuffix(gpu, " (55°C)") {
		t.Errorf("iGPU line = %q, want PPT power dropped and temperature kept as \" (55°C)\"", gpu)
	}
}

func TestAPURAPLPreferredOverPPT(t *testing.T) {
	lines := plainLoadLines(buildLoadBoxLines("x600", apuLoadSnapshot(CPULoad{PowerW: 9, PowerOk: true})))

	var cpu, gpu string
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "cpu ("):
			cpu = l
		case strings.HasPrefix(l, "gpu ("):
			gpu = l
		}
	}
	if !strings.HasSuffix(cpu, " (42°C/9W)") {
		t.Errorf("cpu line = %q, want RAPL 9W, not PPT 15W", cpu)
	}
	if !strings.HasSuffix(gpu, " (55°C)") {
		t.Errorf("iGPU line = %q, want PPT still dropped and temperature kept", gpu)
	}
}

func TestDGPUPowerKeptOnItsLine(t *testing.T) {
	dgpu := GPU{Name: "Navi 33", Class: GPUClassDGPU, UtilPercent: 5, HaveTemp: true, TempC: 61,
		PowerW: 22.6, HavePower: true, PowerLabel: apuPackagePowerLabel}
	lines := plainLoadLines(buildLoadBoxLines("x600", apuLoadSnapshot(CPULoad{PowerW: 9, PowerOk: true}, dgpu)))

	var dline string
	for _, l := range lines {
		if strings.HasPrefix(l, "gpu (Navi 33)") {
			dline = l
		}
	}
	if !strings.HasSuffix(dline, " (61°C/23W)") {
		t.Errorf("dGPU line = %q, want \" (61°C/23W)\" (dGPU power unchanged)", dline)
	}
}

func TestSplitAPUPackagePowerDoesNotMutateInput(t *testing.T) {
	gpus := []GPU{{Class: GPUClassIGPU, PowerW: 15, HavePower: true, PowerLabel: apuPackagePowerLabel}}
	load, out := splitAPUPackagePower(CPULoad{}, gpus)
	if !load.PowerOk || load.PowerW != 15 {
		t.Errorf("cpu power = %v/%v, want 15/true", load.PowerW, load.PowerOk)
	}
	if out[0].HavePower {
		t.Error("returned iGPU still has HavePower")
	}
	if !gpus[0].HavePower {
		t.Error("input slice was mutated")
	}
}
