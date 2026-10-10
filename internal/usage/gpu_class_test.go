package usage

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyAMDGPU(t *testing.T) {
	tests := []struct {
		name      string
		external  bool
		apu       bool
		wantClass GPUClass
	}{
		{"external wins over APU metrics", true, true, GPUClassEGPU},
		{"APU metrics mark an iGPU", false, true, GPUClassIGPU},
		{"discrete metrics mark a dGPU", false, false, GPUClassDGPU},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyAMDGPU(tt.external, tt.apu); got != tt.wantClass {
				t.Errorf("classifyAMDGPU = %v, want %v", got, tt.wantClass)
			}
		})
	}
}

// Regression (issue 765 pre-work): a large BIOS carve-out must not demote an
// APU. The local Cezanne reports 8 GiB VRAM and is still an iGPU.
func TestAPUWithLargeCarveoutStaysIGPU(t *testing.T) {
	root := t.TempDir()
	dev := fakePCIDevice(t, root, "", "0000:07:00.0")
	writeGPUMetrics(t, dev, 2)
	writeSysfsUint(t, dev, "mem_info_vram_total", 8*1024*1024*1024)
	if got := classifyAMDGPU(false, isAPUGPU(dev)); got != GPUClassIGPU {
		t.Errorf("8 GiB carve-out APU class = %v, want iGPU", got)
	}
}

func TestIsAPUGPUReadsMetricsFormat(t *testing.T) {
	root := t.TempDir()
	apu := fakePCIDevice(t, root, "", "0000:07:00.0")
	writeGPUMetrics(t, apu, 2)
	dgpu := fakePCIDevice(t, root, "", "0000:08:00.0")
	writeGPUMetrics(t, dgpu, 1)
	missing := fakePCIDevice(t, root, "", "0000:09:00.0")

	if !isAPUGPU(apu) {
		t.Errorf("format 2 metrics not reported as APU")
	}
	if isAPUGPU(dgpu) {
		t.Errorf("format 1 metrics reported as APU")
	}
	if isAPUGPU(missing) {
		t.Errorf("missing gpu_metrics reported as APU")
	}
}

// writeGPUMetrics writes a gpu_metrics header: structure_size 128 (u16 LE),
// format_revision, content_revision 2.
func writeGPUMetrics(t *testing.T, dir string, format byte) {
	t.Helper()
	header := []byte{128, 0, format, 2}
	if err := os.WriteFile(filepath.Join(dir, "gpu_metrics"), header, 0o444); err != nil {
		t.Fatal(err)
	}
}

// fakePCIDevice builds a sysfs-like PCI tree under root:
// root/pci0000:00[/bridge]/<addr>, and returns the device directory.
func fakePCIDevice(t *testing.T, root, bridge, addr string) string {
	t.Helper()
	dir := filepath.Join(root, "pci0000:00")
	if bridge != "" {
		dir = filepath.Join(dir, bridge)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dir = filepath.Join(dir, addr)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestIsExternalGPU(t *testing.T) {
	root := t.TempDir()

	internal := fakePCIDevice(t, root, "", "0000:c4:00.0")
	if isExternalGPU(internal) {
		t.Errorf("internal GPU reported external")
	}

	bridge := fakePCIDevice(t, root, "0000:00:01.1", "0000:01:00.0")
	if err := os.WriteFile(filepath.Join(root, "pci0000:00", "0000:00:01.1", "removable"), []byte("removable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isExternalGPU(bridge) {
		t.Errorf("GPU behind a removable bridge not reported external")
	}

	own := fakePCIDevice(t, root, "", "0000:02:00.0")
	if err := os.WriteFile(filepath.Join(own, "removable"), []byte("removable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isExternalGPU(own) {
		t.Errorf("GPU with removable attribute not reported external")
	}
}

func TestParseLspciCodenameMatchesSlot(t *testing.T) {
	out := []byte(strings.Join([]string{
		`0000:c4:00.0 "VGA compatible controller" "Advanced Micro Devices, Inc. [AMD/ATI]" "Phoenix1 [Radeon 780M]" -rc1 -p00 "ASUS" "Phoenix1"`,
		`0000:01:00.0 "VGA compatible controller" "Advanced Micro Devices, Inc. [AMD/ATI]" "Navi 33 [Radeon RX 7600]" -rc1 -p00 "ASUS" "Navi 33"`,
	}, "\n"))

	if got, err := parseLspciCodename(out, "0000:01:00.0"); err != nil || got != "Navi 33" {
		t.Errorf("slot 0000:01:00.0 = %q, %v; want Navi 33", got, err)
	}
	if got, err := parseLspciCodename(out, "0000:c4:00.0"); err != nil || got != "Phoenix1" {
		t.Errorf("slot 0000:c4:00.0 = %q, %v; want Phoenix1", got, err)
	}
	if _, err := parseLspciCodename(out, "0000:ff:00.0"); err == nil {
		t.Errorf("unknown slot returned a name, want error")
	}
}

func TestReadAMDSysfsListsEveryGPUWithClassRows(t *testing.T) {
	root := t.TempDir()
	// An APU iGPU: 512 MiB carve-out, big GTT, APU metrics layout.
	addFakeAMDGPU(t, root, 0, fakePCIDevice(t, root, "", "0000:fe:00.0"), 2, 512, 100, 16384, 1024, 40)
	// A discrete card behind a removable bridge: VRAM 8 GiB, eGPU.
	bridge := fakePCIDevice(t, root, "0000:00:02.0", "0000:fd:00.0")
	if err := os.WriteFile(filepath.Join(root, "pci0000:00", "0000:00:02.0", "removable"), []byte("removable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	addFakeAMDGPU(t, root, 1, bridge, 1, 8192, 2048, 12288, 512, 80)

	gpus, err := readAMDSysfsFromGlobWithSeed(filepath.Join(root, "card*", "device", "gpu_busy_percent"), false)
	if err != nil {
		t.Fatalf("readAMDSysfsFromGlobWithSeed: %v", err)
	}
	if len(gpus) != 2 {
		t.Fatalf("got %d GPUs, want 2 (no cap, every card listed)", len(gpus))
	}
	if gpus[0].Class != GPUClassIGPU || !gpus[0].HaveGTT {
		t.Errorf("GPU 0 class = %v HaveGTT=%v, want iGPU with GTT", gpus[0].Class, gpus[0].HaveGTT)
	}
	if gpus[1].Class != GPUClassEGPU {
		t.Errorf("GPU 1 class = %v, want eGPU", gpus[1].Class)
	}
	// Histories are keyed by PCI address, so each GPU keeps its own VRAM sample.
	if got := gpus[0].VRAMPercentHistory[len(gpus[0].VRAMPercentHistory)-1]; math.Abs(got-100.0/512*100) > 1e-6 {
		t.Errorf("GPU 0 VRAM sample = %.4f, want %.4f", got, 100.0/512*100)
	}
	if got := gpus[1].VRAMPercentHistory[len(gpus[1].VRAMPercentHistory)-1]; math.Abs(got-25) > 1e-6 {
		t.Errorf("GPU 1 VRAM sample = %.4f, want 25", got)
	}
}

// addFakeAMDGPU creates cardN/device as a symlink to a PCI device directory
// and fills its amdgpu sysfs attributes (MiB values except busy percent).
// metricsFormat is the gpu_metrics format_revision (2 = APU, 1 = discrete).
func addFakeAMDGPU(t *testing.T, root string, card int, pciDir string, metricsFormat byte, vramTotalMiB, vramUsedMiB, gttTotalMiB, gttUsedMiB, busy uint64) {
	t.Helper()
	cardDir := filepath.Join(root, "card"+formatUint(uint64(card)))
	if err := os.MkdirAll(cardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(pciDir, filepath.Join(cardDir, "device")); err != nil {
		t.Fatal(err)
	}
	const mib = 1024 * 1024
	writeGPUMetrics(t, pciDir, metricsFormat)
	writeSysfsUint(t, pciDir, "gpu_busy_percent", busy)
	writeSysfsUint(t, pciDir, "mem_info_vram_total", vramTotalMiB*mib)
	writeSysfsUint(t, pciDir, "mem_info_vram_used", vramUsedMiB*mib)
	writeSysfsUint(t, pciDir, "mem_info_gtt_total", gttTotalMiB*mib)
	writeSysfsUint(t, pciDir, "mem_info_gtt_used", gttUsedMiB*mib)
}
