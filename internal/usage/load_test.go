package usage

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
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
	readAMDGPUMemory(deviceDir, &g)

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
}

func TestReadAMDGPUMemoryGracefullyHandlesMissingGTT(t *testing.T) {
	deviceDir := t.TempDir()
	writeSysfsUint(t, deviceDir, "mem_info_vram_used", 2*1024*1024*1024)
	writeSysfsUint(t, deviceDir, "mem_info_vram_total", 4*1024*1024*1024)

	var g GPU
	readAMDGPUMemory(deviceDir, &g)

	if !g.HaveMem || !g.HaveVRAM {
		t.Fatalf("VRAM flags = HaveMem:%v HaveVRAM:%v, want true", g.HaveMem, g.HaveVRAM)
	}
	if g.HaveGTT {
		t.Fatalf("HaveGTT = true, want false for missing GTT files")
	}
	if g.MemUsedMiB != 2*1024 || g.MemTotalMiB != 4*1024 {
		t.Errorf("combined memory = %.1f/%.1f MiB, want 2048.0/4096.0", g.MemUsedMiB, g.MemTotalMiB)
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
