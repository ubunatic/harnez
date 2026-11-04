package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func fakePowerClock(start time.Time) (func() time.Time, func(d time.Duration)) {
	t := start
	return func() time.Time { return t }, func(d time.Duration) { t = t.Add(d) }
}

func loadPowerPeakFileForTest(t *testing.T, path string) powerPeakFile {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var f powerPeakFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return f
}

func TestPowerPeakSavesNewPeak(t *testing.T) {
	path := filepath.Join(t.TempDir(), "power-peaks.json")
	now, _ := fakePowerClock(time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC))
	s := newPowerPeakStore(path, "boot-a", now)

	if got := s.Observe("cpu", 38); got != 38 {
		t.Fatalf("Observe returned %v, want 38", got)
	}
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	f := loadPowerPeakFileForTest(t, path)
	if f.BootID != "boot-a" || f.Peaks["cpu"] != 38 {
		t.Fatalf("file = %+v, want boot-a cpu=38", f)
	}
}

func TestPowerPeakLowerValueKeepsOldPeak(t *testing.T) {
	path := filepath.Join(t.TempDir(), "power-peaks.json")
	now, _ := fakePowerClock(time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC))
	s := newPowerPeakStore(path, "boot-a", now)

	s.Observe("cpu", 38)
	if got := s.Observe("cpu", 12); got != 38 {
		t.Fatalf("Observe(12) returned %v, want kept peak 38", got)
	}
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if f := loadPowerPeakFileForTest(t, path); f.Peaks["cpu"] != 38 {
		t.Fatalf("file cpu = %v, want 38", f.Peaks["cpu"])
	}
}

func TestPowerPeakSurvivesRestartWithSameBoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "power-peaks.json")
	now, _ := fakePowerClock(time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC))
	first := newPowerPeakStore(path, "boot-a", now)
	first.Observe("gpu:0000:03:00.0", 287)
	if err := first.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	second := newPowerPeakStore(path, "boot-a", now)
	if got := second.Peak("gpu:0000:03:00.0"); got != 287 {
		t.Fatalf("restarted peak = %v, want 287", got)
	}
}

func TestPowerPeakOtherBootIDResets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "power-peaks.json")
	now, _ := fakePowerClock(time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC))
	old := newPowerPeakStore(path, "boot-a", now)
	old.Observe("cpu", 38)
	if err := old.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	fresh := newPowerPeakStore(path, "boot-b", now)
	if got := fresh.Peak("cpu"); got != 0 {
		t.Fatalf("new boot peak = %v, want 0 after reset", got)
	}
	fresh.Observe("cpu", 5)
	if err := fresh.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	f := loadPowerPeakFileForTest(t, path)
	if f.BootID != "boot-b" || f.Peaks["cpu"] != 5 {
		t.Fatalf("file = %+v, want boot-b cpu=5 with the boot-a peak dropped", f)
	}
}

func TestPowerPeakWriteIsThrottled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "power-peaks.json")
	now, advance := fakePowerClock(time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC))
	s := newPowerPeakStore(path, "boot-a", now)

	s.Observe("cpu", 10) // first write is immediate
	if f := loadPowerPeakFileForTest(t, path); f.Peaks["cpu"] != 10 {
		t.Fatalf("first observe not saved: %+v", f)
	}

	advance(time.Second)
	s.Observe("cpu", 20) // new peak, but inside the 10s throttle window
	if f := loadPowerPeakFileForTest(t, path); f.Peaks["cpu"] != 10 {
		t.Fatalf("write inside throttle window: file cpu = %v, want 10", f.Peaks["cpu"])
	}
	if got := s.Peak("cpu"); got != 20 {
		t.Fatalf("in-memory peak = %v, want 20", got)
	}

	advance(10 * time.Second)
	s.Observe("cpu", 30)
	if f := loadPowerPeakFileForTest(t, path); f.Peaks["cpu"] != 30 {
		t.Fatalf("after throttle window: file cpu = %v, want 30", f.Peaks["cpu"])
	}
}

func TestPowerPeakFlushWritesPendingPeak(t *testing.T) {
	path := filepath.Join(t.TempDir(), "power-peaks.json")
	now, advance := fakePowerClock(time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC))
	s := newPowerPeakStore(path, "boot-a", now)
	s.Observe("cpu", 10)
	advance(time.Second)
	s.Observe("cpu", 20) // throttled
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if f := loadPowerPeakFileForTest(t, path); f.Peaks["cpu"] != 20 {
		t.Fatalf("Flush did not write pending peak: cpu = %v, want 20", f.Peaks["cpu"])
	}
}

func TestPowerPeakConcurrentWritersMergeMax(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "power-peaks.json")
	now, _ := fakePowerClock(time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC))

	const writers = 8
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := newPowerPeakStore(path, "boot-a", now)
			for step := 1; step <= 25; step++ {
				s.Observe("cpu", float64(i*100+step))
				s.Observe("total", float64(step))
				if err := s.Flush(); err != nil {
					t.Errorf("writer %d Flush: %v", i, err)
				}
			}
		}(i)
	}
	wg.Wait()

	f := loadPowerPeakFileForTest(t, path)
	if f.Peaks["cpu"] != float64((writers-1)*100+25) {
		t.Fatalf("cpu peak = %v, want max over all writers %d", f.Peaks["cpu"], (writers-1)*100+25)
	}
	if f.Peaks["total"] != 25 {
		t.Fatalf("total peak = %v, want 25", f.Peaks["total"])
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "power-peaks.json" && e.Name() != "power-peaks.json.lock" {
			t.Fatalf("leftover file after atomic writes: %s", e.Name())
		}
	}
}

// TestPowerPeakSaveIntervalComesFromSpec guards the throttle as a spec value:
// the embedded spec/usage.yaml must carry a positive power_peak_save_interval,
// and the store must use it rather than a Go constant.
func TestPowerPeakSaveIntervalComesFromSpec(t *testing.T) {
	d, err := powerPeakSaveIntervalFromSpec()
	if err != nil {
		t.Fatalf("embedded spec: %v", err)
	}
	if d != 10*time.Second {
		t.Fatalf("power_peak_save_interval = %v, want the spec value 10s", d)
	}
	if got := newPowerPeakStore("", "", time.Now).saveInterval; got != d {
		t.Fatalf("store saveInterval = %v, want %v from spec", got, d)
	}
}

func TestReadBootIDTrimsSysctlValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "boot_id")
	if err := os.WriteFile(path, []byte("0f1e2d3c-4b5a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readBootID(path); got != "0f1e2d3c-4b5a" {
		t.Fatalf("readBootID = %q", got)
	}
	if got := readBootID(filepath.Join(t.TempDir(), "missing")); got != "" {
		t.Fatalf("missing boot_id = %q, want empty", got)
	}
}
