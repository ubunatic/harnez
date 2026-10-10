package usage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeNVIDIASMI swaps the nvidia-smi seams for one test and restores them.
// It returns a pointer to the call counter.
func fakeNVIDIASMI(t *testing.T, pciRoot string, out []byte, runErr error) *int {
	t.Helper()
	calls := 0
	origRun, origLook, origRoot, origClock := nvidiaSMIRun, nvidiaSMILookPath, nvidiaPCIRoot, nvidiaClock
	t.Cleanup(func() {
		nvidiaSMIRun, nvidiaSMILookPath, nvidiaPCIRoot, nvidiaClock = origRun, origLook, origRoot, origClock
		resetNVIDIACache()
	})
	nvidiaPCIRoot = pciRoot
	nvidiaSMILookPath = func(string) (string, error) { return "/usr/bin/nvidia-smi", nil }
	nvidiaSMIRun = func(ctx context.Context) ([]byte, error) {
		calls++
		if _, ok := ctx.Deadline(); !ok {
			t.Errorf("nvidia-smi ran without a timeout deadline")
		}
		return out, runErr
	}
	resetNVIDIACache()
	return &calls
}

// fakeNVIDIAPCIRoot builds a sysfs-like PCI tree with one NVIDIA function
// (vendor 0x10de) at 0000:0a:00.0 and an AMD one at 0000:c4:00.0.
func fakeNVIDIAPCIRoot(t *testing.T, nvidia bool) string {
	t.Helper()
	root := t.TempDir()
	for addr, vendor := range map[string]string{"0000:c4:00.0": "0x1002", "0000:0a:00.0": "0x10de"} {
		if addr == "0000:0a:00.0" && !nvidia {
			continue
		}
		dir := filepath.Join(root, addr)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "vendor"), []byte(vendor+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const nvidiaTwoGPUOutput = "NVIDIA GeForce RTX 3060, 00000000:0A:00.0, 7, 1200, 12288, 30\n" +
	"NVIDIA GeForce GTX 1650, 00000000:0B:00.0, [N/A], [N/A], 4096, [N/A]\n"

func TestParseNVIDIASMI(t *testing.T) {
	rows, err := parseNVIDIASMI([]byte(nvidiaTwoGPUOutput))
	if err != nil {
		t.Fatalf("parseNVIDIASMI: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	rtx := rows[0]
	if rtx.GPU.Name != "RTX 3060" || rtx.PCI != "0000:0a:00.0" {
		t.Errorf("row 0 = name %q pci %q, want RTX 3060 / 0000:0a:00.0", rtx.GPU.Name, rtx.PCI)
	}
	if rtx.GPU.UtilPercent != 7 || !rtx.GPU.HaveVRAM || rtx.GPU.VRAMUsedMiB != 1200 || rtx.GPU.VRAMTotalMiB != 12288 {
		t.Errorf("row 0 readings = %+v", rtx.GPU)
	}
	if !rtx.GPU.HaveTemp || rtx.GPU.TempC != 30 {
		t.Errorf("row 0 temp = %v/%v, want 30", rtx.GPU.HaveTemp, rtx.GPU.TempC)
	}
	// [N/A] fields are absent, not zero: no util, no temperature, no VRAM used.
	gtx := rows[1].GPU
	if gtx.HaveTemp || gtx.HaveVRAM {
		t.Errorf("row 1 with [N/A] fields has HaveTemp=%v HaveVRAM=%v, want both false", gtx.HaveTemp, gtx.HaveVRAM)
	}
}

func TestParseNVIDIASMIRejectsGarbage(t *testing.T) {
	for name, out := range map[string][]byte{
		"empty":        nil,
		"not csv":      []byte("Failed to initialize NVML: Driver/library version mismatch\n"),
		"short fields": []byte("RTX 3060, 1, 2\n"),
	} {
		if _, err := parseNVIDIASMI(out); err == nil {
			t.Errorf("%s: parseNVIDIASMI returned no error, want one", name)
		}
	}
}

func TestNVIDIAProbeGateSkipsWithoutNVIDIAHardware(t *testing.T) {
	root := fakeNVIDIAPCIRoot(t, false)
	calls := fakeNVIDIASMI(t, root, []byte(nvidiaTwoGPUOutput), nil)

	if got := currentNVIDIAGPUs(); got != nil {
		t.Errorf("AMD-only host returned NVIDIA GPUs: %v", got)
	}
	if *calls != 0 {
		t.Errorf("nvidia-smi ran %d times on an AMD-only host, want 0", *calls)
	}
}

func TestNVIDIAMissingBinarySkipsProbe(t *testing.T) {
	root := fakeNVIDIAPCIRoot(t, true)
	calls := fakeNVIDIASMI(t, root, []byte(nvidiaTwoGPUOutput), nil)
	nvidiaSMILookPath = func(string) (string, error) { return "", errors.New("not found") }
	resetNVIDIACache()

	if got := currentNVIDIAGPUs(); got != nil {
		t.Errorf("missing binary returned GPUs: %v", got)
	}
	if *calls != 0 {
		t.Errorf("nvidia-smi ran %d times with no binary, want 0", *calls)
	}
}

func TestNVIDIAFailuresYieldNoGPUsAndAreCached(t *testing.T) {
	root := fakeNVIDIAPCIRoot(t, true)
	cases := map[string]struct {
		out []byte
		err error
	}{
		"non-zero exit or timeout": {nil, errors.New("exit status 9")},
		"garbage output":           {[]byte("NVML error\n"), nil},
	}
	for name, tc := range cases {
		calls := fakeNVIDIASMI(t, root, tc.out, tc.err)
		if got := currentNVIDIAGPUs(); got != nil {
			t.Errorf("%s: returned GPUs %v, want none", name, got)
		}
		currentNVIDIAGPUs()
		if *calls != 1 {
			t.Errorf("%s: nvidia-smi ran %d times within the TTL, want 1 (failure cached)", name, *calls)
		}
	}
}

func TestNVIDIACacheReusesReadingWithinTTL(t *testing.T) {
	root := fakeNVIDIAPCIRoot(t, true)
	calls := fakeNVIDIASMI(t, root, []byte(nvidiaTwoGPUOutput), nil)
	clock := time.Unix(1_000_000, 0)
	nvidiaClock = func() time.Time { return clock }
	resetNVIDIACache()

	for i := 0; i < 50; i++ { // a burst of 100ms redraws inside one TTL
		if got := currentNVIDIAGPUs(); len(got) != 2 {
			t.Fatalf("redraw %d returned %d GPUs, want 2", i, len(got))
		}
	}
	if *calls != 1 {
		t.Errorf("nvidia-smi ran %d times inside one TTL, want 1", *calls)
	}

	clock = clock.Add(nvidiaCacheTTLForTest(t) + time.Millisecond)
	currentNVIDIAGPUs()
	if *calls != 2 {
		t.Errorf("nvidia-smi ran %d times after TTL expiry, want 2", *calls)
	}
}

func TestNVIDIAEGPUIsClassifiedExternal(t *testing.T) {
	// Mirror real sysfs: the flat /sys/bus/pci/devices entry is a symlink to
	// the device's real place in the tree, here behind a removable bridge.
	root := t.TempDir()
	dev := fakePCIDevice(t, root, "0000:00:02.0", "0000:0a:00.0")
	if err := os.WriteFile(filepath.Join(dev, "vendor"), []byte("0x10de\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pci0000:00", "0000:00:02.0", "removable"), []byte("removable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dev, filepath.Join(root, "0000:0a:00.0")); err != nil {
		t.Fatal(err)
	}
	fakeNVIDIASMI(t, root, []byte("NVIDIA GeForce RTX 3060, 00000000:0A:00.0, 0, 1200, 12288, 30\n"), nil)

	gpus := currentNVIDIAGPUs()
	if len(gpus) != 1 || gpus[0].Class != GPUClassEGPU {
		t.Fatalf("NVIDIA behind a removable bridge = %+v, want one eGPU", gpus)
	}
}

// nvidiaCacheTTLForTest returns the spec TTL the reader uses.
func nvidiaCacheTTLForTest(t *testing.T) time.Duration {
	t.Helper()
	_, ttl, err := nvidiaSMIPolicy()
	if err != nil || ttl <= 0 {
		t.Fatalf("nvidiaSMIPolicy ttl = %v, %v", ttl, err)
	}
	return ttl
}
