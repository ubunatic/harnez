package usage

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testRAPLZone is the package zone fakeRAPLRoot creates.
const testRAPLZone = "intel-rapl:0"

// addRAPLZone writes one powercap zone directory with the given name file
// content and, when energy is non-empty, its energy_uj. maxRange, when
// non-empty, is written as max_energy_range_uj.
func addRAPLZone(t *testing.T, root, zone, name, energy, maxRange string) {
	t.Helper()
	dir := filepath.Join(root, zone)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"name": name, "energy_uj": energy, "max_energy_range_uj": maxRange}
	for file, val := range files {
		if val == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, file), []byte(val+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// fakeRAPLRoot returns a powercap class root with one package zone
// (intel-rapl:0, name "package-0"). Empty energy leaves energy_uj absent.
func fakeRAPLRoot(t *testing.T, energy, maxRange string) string {
	t.Helper()
	root := t.TempDir()
	addRAPLZone(t, root, testRAPLZone, "package-0", energy, maxRange)
	return root
}

func writeRAPLEnergy(t *testing.T, root, energy string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, testRAPLZone, "energy_uj"), []byte(energy+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// resetRAPLState clears the package-level RAPL baseline and last watts so
// each test starts from "no previous sample".
func resetRAPLState(t *testing.T) {
	t.Helper()
	clear := func() {
		raplMu.Lock()
		lastRAPL, haveRAPL = raplSample{}, false
		lastRAPLWatts, haveRAPLWatts = 0, false
		raplMu.Unlock()
	}
	clear()
	t.Cleanup(clear)
}

func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

var raplT0 = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

func TestReadRAPLPowerFromTwoSamples(t *testing.T) {
	resetRAPLState(t)
	root := fakeRAPLRoot(t, "1000000", "")
	if w, ok, readable := readRAPLPower(root, fixedClock(raplT0)); ok || !readable || w != 0 {
		t.Fatalf("first sample: got %v/%v/%v, want 0/false/true", w, ok, readable)
	}

	// 30 J over 2 s is 15 W.
	writeRAPLEnergy(t, root, "31000000")
	w, ok, readable := readRAPLPower(root, fixedClock(raplT0.Add(2*time.Second)))
	if !ok || !readable || w != 15 {
		t.Errorf("delta reading: got %v/%v/%v, want 15/true/true", w, ok, readable)
	}
}

func TestReadRAPLPowerZeroDeltaIsZeroWatts(t *testing.T) {
	resetRAPLState(t)
	root := fakeRAPLRoot(t, "5000000", "")
	readRAPLPower(root, fixedClock(raplT0))

	// Same energy, 1 s apart: an idle counter reads as 0 W, not as no reading.
	w, ok, readable := readRAPLPower(root, fixedClock(raplT0.Add(time.Second)))
	if !ok || !readable || w != 0 {
		t.Errorf("zero delta: got %v/%v/%v, want 0/true/true", w, ok, readable)
	}
}

func TestReadRAPLPowerWrapAround(t *testing.T) {
	resetRAPLState(t)
	// Range is 100 J. Counter goes 90 J -> 10 J: 10 J to the wrap plus
	// 10 J after it is 20 J in 1 s, so 20 W.
	root := fakeRAPLRoot(t, "90000000", "100000000")
	readRAPLPower(root, fixedClock(raplT0))
	writeRAPLEnergy(t, root, "10000000")
	w, ok, _ := readRAPLPower(root, fixedClock(raplT0.Add(time.Second)))
	if !ok || w != 20 {
		t.Errorf("wrap reading: got %v/%v, want 20/true", w, ok)
	}
}

func TestReadRAPLPowerWrapWithoutRangeGivesNoReading(t *testing.T) {
	resetRAPLState(t)
	root := fakeRAPLRoot(t, "90000000", "")
	readRAPLPower(root, fixedClock(raplT0))
	writeRAPLEnergy(t, root, "10000000")
	if _, ok, readable := readRAPLPower(root, fixedClock(raplT0.Add(time.Second))); ok || !readable {
		t.Errorf("counter reset without max_energy_range_uj: ok=%v readable=%v, want false/true", ok, readable)
	}
	// The reset became the baseline, so the next delta is valid: 2 J in 1 s.
	writeRAPLEnergy(t, root, "12000000")
	if w, ok, _ := readRAPLPower(root, fixedClock(raplT0.Add(2*time.Second))); !ok || w != 2 {
		t.Errorf("after reset: got %v/%v, want 2/true", w, ok)
	}
}

func TestReadRAPLPowerUnreadableIsSilentAndKeepsBaseline(t *testing.T) {
	resetRAPLState(t)
	root := fakeRAPLRoot(t, "1000000", "")
	readRAPLPower(root, fixedClock(raplT0))

	unreadable := fakeRAPLRoot(t, "", "")
	if _, ok, readable := readRAPLPower(unreadable, fixedClock(raplT0.Add(time.Second))); ok || readable {
		t.Errorf("unreadable energy_uj: ok=%v readable=%v, want false/false", ok, readable)
	}

	// The failed read must not replace the baseline: the next readable
	// sample is measured from t0, so 10 J over 2 s is 5 W.
	writeRAPLEnergy(t, root, "11000000")
	if w, ok, _ := readRAPLPower(root, fixedClock(raplT0.Add(2*time.Second))); !ok || w != 5 {
		t.Errorf("after unreadable: got %v/%v, want 5/true", w, ok)
	}
}

func TestReadRAPLPowerMissingZoneIsNotReadable(t *testing.T) {
	resetRAPLState(t)
	if _, ok, readable := readRAPLPower(t.TempDir(), fixedClock(raplT0)); ok || readable {
		t.Errorf("missing powercap tree: ok=%v readable=%v, want false/false", ok, readable)
	}
}

func TestReadRAPLPowerNonPositiveStepGivesNoReading(t *testing.T) {
	resetRAPLState(t)
	root := fakeRAPLRoot(t, "1000000", "")
	readRAPLPower(root, fixedClock(raplT0))
	writeRAPLEnergy(t, root, "2000000")
	// The clock going backwards is inside the rate interval: no new reading.
	if _, ok, _ := readRAPLPower(root, fixedClock(raplT0)); ok {
		t.Error("zero time step: ok = true, want false")
	}
}

func TestReadRAPLPowerRateLimitKeepsBaseline(t *testing.T) {
	resetRAPLState(t)
	root := fakeRAPLRoot(t, "1000000", "")
	readRAPLPower(root, fixedClock(raplT0))

	// 0.4 s later the call is rate-limited: no watts yet, and the baseline
	// must stay at t0.
	writeRAPLEnergy(t, root, "2000000")
	if _, ok, readable := readRAPLPower(root, fixedClock(raplT0.Add(400*time.Millisecond))); ok || !readable {
		t.Fatalf("rate-limited first delta: ok=%v readable=%v, want false/true", ok, readable)
	}

	// At 1 s the delta is measured from t0: 10 J over 1 s is 10 W. If the
	// baseline had advanced at 0.4 s this would be 1 J over 0.6 s instead.
	writeRAPLEnergy(t, root, "11000000")
	if w, ok, _ := readRAPLPower(root, fixedClock(raplT0.Add(time.Second))); !ok || w != 10 {
		t.Errorf("baseline kept: got %v/%v, want 10/true", w, ok)
	}
}

func TestReadRAPLPowerRateLimitReturnsLastWatts(t *testing.T) {
	resetRAPLState(t)
	root := fakeRAPLRoot(t, "0", "")
	readRAPLPower(root, fixedClock(raplT0))
	writeRAPLEnergy(t, root, "10000000")
	if w, ok, _ := readRAPLPower(root, fixedClock(raplT0.Add(time.Second))); !ok || w != 10 {
		t.Fatalf("first watts: got %v/%v, want 10/true", w, ok)
	}

	// Rate-limited: the last computed 10 W comes back, the baseline stays at 1 s.
	writeRAPLEnergy(t, root, "20000000")
	if w, ok, readable := readRAPLPower(root, fixedClock(raplT0.Add(1500*time.Millisecond))); !ok || !readable || w != 10 {
		t.Errorf("rate-limited: got %v/%v/%v, want 10/true/true", w, ok, readable)
	}

	// At 2 s the delta is from 1 s: 10 J over 1 s is 10 W, not 20 W from t0.
	writeRAPLEnergy(t, root, "30000000")
	if w, ok, _ := readRAPLPower(root, fixedClock(raplT0.Add(2*time.Second))); !ok || w != 20 {
		t.Errorf("after rate limit: got %v/%v, want 20/true", w, ok)
	}
}

func TestFindRAPLPackageZoneSkipsSubzonesAndNonPackageZones(t *testing.T) {
	root := t.TempDir()
	addRAPLZone(t, root, "intel-rapl:0", "core", "1", "")
	addRAPLZone(t, root, "intel-rapl:0:0", "package-0", "1", "")
	addRAPLZone(t, root, "intel-rapl:2", "package-2", "1", "")
	addRAPLZone(t, root, "intel-rapl:1", "package-1", "1", "")

	zone, ok := findRAPLPackageZone(root)
	if !ok || filepath.Base(zone) != "intel-rapl:1" {
		t.Errorf("package zone = %q/%v, want intel-rapl:1 (first sorted top-level package-*)", zone, ok)
	}

	noPackage := t.TempDir()
	addRAPLZone(t, noPackage, "intel-rapl:0", "core", "1", "")
	addRAPLZone(t, noPackage, "intel-rapl:0:0", "package-0", "1", "")
	if zone, ok := findRAPLPackageZone(noPackage); ok {
		t.Errorf("no top-level package zone: got %q, want none", zone)
	}
}

func TestReadRAPLPowerUsesDiscoveredZone(t *testing.T) {
	resetRAPLState(t)
	root := t.TempDir()
	addRAPLZone(t, root, "intel-rapl:0", "core", "99000000", "")
	addRAPLZone(t, root, "intel-rapl:1", "package-1", "1000000", "")
	readRAPLPower(root, fixedClock(raplT0))
	if err := os.WriteFile(filepath.Join(root, "intel-rapl:1", "energy_uj"), []byte("31000000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w, ok, _ := readRAPLPower(root, fixedClock(raplT0.Add(2*time.Second))); !ok || w != 15 {
		t.Errorf("discovered zone: got %v/%v, want 15/true (not the core zone)", w, ok)
	}
}

// TestReadRAPLPowerConcurrentCallers runs many goroutines against one shared
// baseline under -race. The energy counter is constant, so every computed
// reading must be 0 W; the clock advances 500 ms per call, so rate-limited and
// full-delta paths both run. The clock is called under the mutex, so its
// ticks are ordered.
func TestReadRAPLPowerConcurrentCallers(t *testing.T) {
	resetRAPLState(t)
	root := fakeRAPLRoot(t, "7000000", "100000000")

	var ticks atomic.Int64
	clock := func() time.Time {
		return raplT0.Add(time.Duration(ticks.Add(1)) * 500 * time.Millisecond)
	}

	const workers, calls = 8, 200
	var wg sync.WaitGroup
	errs := make(chan string, workers*calls)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < calls; j++ {
				w, ok, readable := readRAPLPower(root, clock)
				if !readable {
					errs <- "readable = false for a readable zone"
				}
				if ok && w != 0 {
					errs <- "nonzero watts from a constant counter"
				}
				if math.IsNaN(w) {
					errs <- "NaN watts"
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Error(msg)
	}
}

// apuLoadSnapshot is an iGPU whose power1 label is PPT (the whole package)
// alongside an optional dGPU; load supplies the CPU's power state.
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

// loadLinesByPrefix returns the first plain line starting with prefix.
func loadLinesByPrefix(lines []string, prefix string) string {
	for _, l := range plainLoadLines(lines) {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
	return ""
}

func TestAPUPPTMovesToCPULineWithoutRAPL(t *testing.T) {
	lines := buildLoadBoxLines("x600", apuLoadSnapshot(CPULoad{}))
	cpu := loadLinesByPrefix(lines, "cpu (")
	gpu := loadLinesByPrefix(lines, "gpu (")
	if !strings.HasSuffix(cpu, " (42°C/15W)") {
		t.Errorf("cpu line = %q, want suffix \" (42°C/15W)\" (PPT moved to CPU)", cpu)
	}
	if !strings.HasSuffix(gpu, " (55°C)") {
		t.Errorf("iGPU line = %q, want PPT power dropped and temperature kept as \" (55°C)\"", gpu)
	}
}

func TestAPURAPLPreferredOverPPT(t *testing.T) {
	lines := buildLoadBoxLines("x600", apuLoadSnapshot(CPULoad{PowerW: 9, PowerOk: true, RAPLReadable: true}))
	cpu := loadLinesByPrefix(lines, "cpu (")
	gpu := loadLinesByPrefix(lines, "gpu (")
	if !strings.HasSuffix(cpu, " (42°C/9W)") {
		t.Errorf("cpu line = %q, want RAPL 9W, not PPT 15W", cpu)
	}
	if !strings.HasSuffix(gpu, " (55°C)") {
		t.Errorf("iGPU line = %q, want PPT still dropped and temperature kept", gpu)
	}
}

// A readable RAPL zone with no delta yet must not fall back to PPT: the CPU
// line shows no power, and the iGPU line still drops PPT.
func TestAPUReadableRAPLWithoutDeltaShowsNoPowerNotPPT(t *testing.T) {
	lines := buildLoadBoxLines("x600", apuLoadSnapshot(CPULoad{RAPLReadable: true}))
	cpu := loadLinesByPrefix(lines, "cpu (")
	gpu := loadLinesByPrefix(lines, "gpu (")
	if !strings.HasSuffix(cpu, " (42°C)") {
		t.Errorf("cpu line = %q, want no power suffix (no source flip to PPT)", cpu)
	}
	if !strings.HasSuffix(gpu, " (55°C)") {
		t.Errorf("iGPU line = %q, want PPT still dropped", gpu)
	}
}

func TestDGPUPowerKeptOnItsLine(t *testing.T) {
	dgpu := GPU{Name: "Navi 33", Class: GPUClassDGPU, UtilPercent: 5, HaveTemp: true, TempC: 61,
		PowerW: 22.6, HavePower: true, PowerLabel: apuPackagePowerLabel}
	lines := buildLoadBoxLines("x600", apuLoadSnapshot(CPULoad{PowerW: 9, PowerOk: true, RAPLReadable: true}, dgpu))
	if got := loadLinesByPrefix(lines, "gpu (Navi 33)"); !strings.HasSuffix(got, " (61°C/23W)") {
		t.Errorf("dGPU line = %q, want \" (61°C/23W)\" (dGPU power unchanged)", got)
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
