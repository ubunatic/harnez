package usage

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestParseMeminfoUsesMemAvailable(t *testing.T) {
	mem, err := parseMeminfo(`MemTotal:       32768000 kB
MemFree:         1000000 kB
MemAvailable:   24576000 kB
Buffers:          100000 kB
Cached:          2000000 kB
`)
	if err != nil {
		t.Fatalf("parseMeminfo returned error: %v", err)
	}
	if !mem.Ok {
		t.Fatalf("mem.Ok = false, want true")
	}
	if mem.TotalMiB != 32000 {
		t.Errorf("TotalMiB = %.1f, want 32000.0", mem.TotalMiB)
	}
	if mem.AvailableMiB != 24000 {
		t.Errorf("AvailableMiB = %.1f, want 24000.0", mem.AvailableMiB)
	}
	if mem.UsedMiB != 8000 {
		t.Errorf("UsedMiB = %.1f, want 8000.0", mem.UsedMiB)
	}
}

func TestParseMeminfoFallbackAvailability(t *testing.T) {
	mem, err := parseMeminfo(`MemTotal:       8192000 kB
MemFree:        1024000 kB
Buffers:         512000 kB
Cached:         2048000 kB
SReclaimable:    512000 kB
Shmem:           256000 kB
`)
	if err != nil {
		t.Fatalf("parseMeminfo returned error: %v", err)
	}
	if mem.AvailableMiB != 3750 {
		t.Errorf("AvailableMiB = %.1f, want 3750.0", mem.AvailableMiB)
	}
	if mem.UsedMiB != 4250 {
		t.Errorf("UsedMiB = %.1f, want 4250.0", mem.UsedMiB)
	}
}

func TestReadAMDGPUMemoryAggregatesVRAMAndGTT(t *testing.T) {
	deviceDir := t.TempDir()
	writeSysfsUint(t, deviceDir, "mem_info_vram_used", 5*1024*1024*1024)
	writeSysfsUint(t, deviceDir, "mem_info_vram_total", 8*1024*1024*1024)
	writeSysfsUint(t, deviceDir, "mem_info_gtt_used", 1*1024*1024*1024)
	writeSysfsUint(t, deviceDir, "mem_info_gtt_total", 12*1024*1024*1024)

	var g GPU
	readAMDGPUMemory(deviceDir, "card0", &g)

	if !g.HaveMem || !g.HaveVRAM || !g.HaveGTT {
		t.Fatalf("memory flags = HaveMem:%v HaveVRAM:%v HaveGTT:%v, want all true", g.HaveMem, g.HaveVRAM, g.HaveGTT)
	}
	if g.MemUsedMiB != 6*1024 {
		t.Errorf("MemUsedMiB = %.1f, want %.1f", g.MemUsedMiB, float64(6*1024))
	}
	if g.MemTotalMiB != 20*1024 {
		t.Errorf("MemTotalMiB = %.1f, want %.1f", g.MemTotalMiB, float64(20*1024))
	}
	if g.MemPercent != 30 {
		t.Errorf("MemPercent = %.1f, want 30.0", g.MemPercent)
	}
	if len(g.VRAMPercentHistory) == 0 || g.VRAMPercentHistory[len(g.VRAMPercentHistory)-1] != 62.5 {
		t.Errorf("VRAMPercentHistory = %v, want last sample 62.5", g.VRAMPercentHistory)
	}
	if len(g.GTTPercentHistory) == 0 || math.Abs(g.GTTPercentHistory[len(g.GTTPercentHistory)-1]-(100.0/12.0)) > 1e-4 {
		t.Errorf("GTTPercentHistory = %v, want last sample approx %.2f", g.GTTPercentHistory, 100.0/12.0)
	}
}

func TestReadAMDGPUMemoryGracefullyHandlesMissingGTT(t *testing.T) {
	deviceDir := t.TempDir()
	writeSysfsUint(t, deviceDir, "mem_info_vram_used", 2*1024*1024*1024)
	writeSysfsUint(t, deviceDir, "mem_info_vram_total", 4*1024*1024*1024)

	var g GPU
	readAMDGPUMemory(deviceDir, "card1", &g)

	if !g.HaveMem || !g.HaveVRAM {
		t.Fatalf("VRAM flags = HaveMem:%v HaveVRAM:%v, want true", g.HaveMem, g.HaveVRAM)
	}
	if g.HaveGTT {
		t.Fatalf("HaveGTT = true, want false for missing GTT files")
	}
	if g.MemUsedMiB != 2*1024 || g.MemTotalMiB != 4*1024 {
		t.Errorf("combined memory = %.1f/%.1f MiB, want 2048.0/4096.0", g.MemUsedMiB, g.MemTotalMiB)
	}
	if len(g.VRAMPercentHistory) == 0 || g.VRAMPercentHistory[len(g.VRAMPercentHistory)-1] != 50 {
		t.Errorf("VRAMPercentHistory = %v, want last sample 50.0", g.VRAMPercentHistory)
	}
}

func writeSysfsUint(t *testing.T, dir, name string, value uint64) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(formatUint(value)+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func formatUint(value uint64) string {
	return strconv.FormatUint(value, 10)
}

func TestFormatRAMLabel(t *testing.T) {
	tests := []struct {
		name string
		mem  SystemMemory
		want string
	}{
		{
			name: "explicit dual channel geometry",
			mem:  SystemMemory{Geometry: "2x16G", TotalMiB: 32768, Ok: true},
			want: "ram (2x16G)     ",
		},
		{
			name: "fallback total GiB geometry",
			mem:  SystemMemory{TotalMiB: 46182.4, Ok: true},
			want: "ram (45G)       ",
		},
		{
			name: "fallback standard total GiB",
			mem:  SystemMemory{TotalMiB: 32768, Ok: true},
			want: "ram (32G)       ",
		},
		{
			name: "empty total without geometry",
			mem:  SystemMemory{Ok: false},
			want: "ram             ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatRAMLabel(tt.mem)
			if got != tt.want {
				t.Errorf("FormatRAMLabel() = %q, want %q", got, tt.want)
			}
			if len(got) != loadLabelWidth {
				t.Errorf("FormatRAMLabel() length = %d, want %d", len(got), loadLabelWidth)
			}
		})
	}
}

func TestReadRAMGeometryFromEDAC(t *testing.T) {
	tempDir := t.TempDir()

	// 2 DIMMs of 16384 MiB each (16G)
	dimm0Dir := filepath.Join(tempDir, "mc0", "dimm0")
	dimm1Dir := filepath.Join(tempDir, "mc0", "dimm1")
	if err := os.MkdirAll(dimm0Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dimm1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSysfsUint(t, dimm0Dir, "size", 16384)
	writeSysfsUint(t, dimm1Dir, "size", 16384)

	pattern := filepath.Join(tempDir, "mc*", "dimm*", "size")
	got := readRAMGeometryFromEDACPattern(pattern, "")
	if got != "2x16G" {
		t.Errorf("readRAMGeometryFromEDACPattern = %q, want %q", got, "2x16G")
	}

	// 4 DIMMs of 8192 MiB each (8G)
	tempDir4 := t.TempDir()
	for i := 0; i < 4; i++ {
		d := filepath.Join(tempDir4, "mc0", "dimm"+strconv.Itoa(i))
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		writeSysfsUint(t, d, "size", 8192)
	}
	got4 := readRAMGeometryFromEDACPattern(filepath.Join(tempDir4, "mc*", "dimm*", "size"), "")
	if got4 != "4x8G" {
		t.Errorf("readRAMGeometryFromEDACPattern (4x8G) = %q, want %q", got4, "4x8G")
	}

	// Mismatched DIMMs: 16G + 32G -> 48G
	tempDirMixed := t.TempDir()
	d1 := filepath.Join(tempDirMixed, "mc0", "dimm0")
	d2 := filepath.Join(tempDirMixed, "mc0", "dimm1")
	_ = os.MkdirAll(d1, 0o755)
	_ = os.MkdirAll(d2, 0o755)
	writeSysfsUint(t, d1, "size", 16384)
	writeSysfsUint(t, d2, "size", 32768)
	gotMixed := readRAMGeometryFromEDACPattern(filepath.Join(tempDirMixed, "mc*", "dimm*", "size"), "")
	if gotMixed != "48G" {
		t.Errorf("readRAMGeometryFromEDACPattern (mixed) = %q, want %q", gotMixed, "48G")
	}
}

func TestReadRAMGeometryFromDMI(t *testing.T) {
	tempDir := t.TempDir()
	e0 := filepath.Join(tempDir, "17-0")
	e1 := filepath.Join(tempDir, "17-1")
	_ = os.MkdirAll(e0, 0o755)
	_ = os.MkdirAll(e1, 0o755)

	// Build raw Type 17 payload: type=17 at byte 0, length >= 14, size at byte 12-13 (16384 = 0x4000)
	raw0 := make([]byte, 32)
	raw0[0] = 17
	raw0[1] = 32
	raw0[12] = 0x00
	raw0[13] = 0x40 // 16384 MB = 16 GB

	raw1 := make([]byte, 32)
	raw1[0] = 17
	raw1[1] = 32
	raw1[12] = 0x00
	raw1[13] = 0x40 // 16384 MB = 16 GB

	if err := os.WriteFile(filepath.Join(e0, "raw"), raw0, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e1, "raw"), raw1, 0o644); err != nil {
		t.Fatal(err)
	}

	got := readRAMGeometryFromDMIPattern(filepath.Join(tempDir, "17-*", "raw"))
	if got != "2x16G" {
		t.Errorf("readRAMGeometryFromDMIPattern = %q, want %q", got, "2x16G")
	}
}

func TestFallbackRAMGeometry(t *testing.T) {
	if got := fallbackRAMGeometry(46182.4); got != "45G" {
		t.Errorf("fallbackRAMGeometry(46182.4) = %q, want %q", got, "45G")
	}
	if got := fallbackRAMGeometry(32768); got != "32G" {
		t.Errorf("fallbackRAMGeometry(32768) = %q, want %q", got, "32G")
	}
	if got := fallbackRAMGeometry(0); got != "" {
		t.Errorf("fallbackRAMGeometry(0) = %q, want empty", got)
	}
}

func TestRAMHistoryAppendsAndSnapshots(t *testing.T) {
	var hist sampleHistory
	for i := 1; i <= loadHistoryLen+5; i++ {
		hist.forceAppend(float64(i * 5))
	}
	snap := hist.snapshot()
	if len(snap) != loadHistoryLen {
		t.Fatalf("snapshot length = %d, want %d", len(snap), loadHistoryLen)
	}
	// Last element should retain the newest sample.
	if want := float64((loadHistoryLen + 5) * 5); snap[len(snap)-1] != want {
		t.Errorf("last element = %.0f, want %.0f", snap[len(snap)-1], want)
	}
}

func TestSampleHistoryRateThrottling(t *testing.T) {
	var hist sampleHistory
	now := time.Now()

	// Initial append records the first sample.
	snap := hist.appendAt(10.0, now, minLoadSampleInterval)
	if len(snap) != 1 || snap[0] != 10.0 {
		t.Fatalf("initial append = %v, want [10.0]", snap)
	}

	// Rapid successive calls within minLoadSampleInterval (e.g. at +10ms, +50ms)
	// must update the latest sample in place without adding new slots.
	for i := 1; i <= 100; i++ {
		snap = hist.appendAt(float64(10+i), now.Add(time.Duration(i)*5*time.Millisecond), minLoadSampleInterval)
	}
	if len(snap) != 1 {
		t.Fatalf("rapid appends length = %d, want 1", len(snap))
	}
	if snap[0] != 110.0 {
		t.Errorf("in-place updated sample = %.1f, want 110.0", snap[0])
	}

	// Once minLoadSampleInterval (1s) has passed, append advances history.
	snap = hist.appendAt(200.0, now.Add(1100*time.Millisecond), minLoadSampleInterval)
	if len(snap) != 2 {
		t.Fatalf("append after 1.1s length = %d, want 2", len(snap))
	}
	if snap[0] != 110.0 || snap[1] != 200.0 {
		t.Errorf("samples = %v, want [110.0, 200.0]", snap)
	}

	// Another rapid burst at +1.2s updates only the latest slot.
	snap = hist.appendAt(250.0, now.Add(1200*time.Millisecond), minLoadSampleInterval)
	if len(snap) != 2 {
		t.Fatalf("burst append length = %d, want 2", len(snap))
	}
	if snap[1] != 250.0 {
		t.Errorf("updated sample = %.1f, want 250.0", snap[1])
	}

	// Next 1s window advances to 3 samples.
	snap = hist.appendAt(300.0, now.Add(2100*time.Millisecond), minLoadSampleInterval)
	if len(snap) != 3 {
		t.Fatalf("append after 2.1s length = %d, want 3", len(snap))
	}
	if snap[2] != 300.0 {
		t.Errorf("new sample = %.1f, want 300.0", snap[2])
	}
}

func TestCurrentCPULoadRateThrottling(t *testing.T) {
	// Prime the first reading so cpuHistory and cpuFrameMu have initial state.
	load1 := CurrentCPULoad()
	if !load1.Ok {
		t.Skip("skipping CPU load test: /proc/stat or /proc/loadavg unavailable")
	}

	initialCPULen := len(cpuHistory.snapshot())
	initialRAMLen := len(ramHistory.snapshot())

	// Rapidly call CurrentCPULoad 100 times in a tight loop.
	for i := 0; i < 100; i++ {
		_ = CurrentCPULoad()
	}

	// History buffers should not have advanced 100 times.
	finalCPULen := len(cpuHistory.snapshot())
	finalRAMLen := len(ramHistory.snapshot())

	if finalCPULen > initialCPULen+1 {
		t.Errorf("cpuHistory length grew from %d to %d after rapid calls, want at most %d",
			initialCPULen, finalCPULen, initialCPULen+1)
	}
	if finalRAMLen > initialRAMLen+1 {
		t.Errorf("ramHistory length grew from %d to %d after rapid calls, want at most %d",
			initialRAMLen, finalRAMLen, initialRAMLen+1)
	}
}
