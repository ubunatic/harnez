package usage

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Intel GPUs are read from the i915 driver's sysfs (issue 1219). The kernel
// publishes no busy-percent attribute for i915, and the engine-busy perf PMU
// (/sys/bus/event_source/devices/i915) needs CAP_PERFMON: an unprivileged
// perf_event_open returns EACCES. So the load figure is derived from the GT's
// RC6 residency counter (gt/gt0/rc6_residency_ms, world-readable): the share
// of wall time the GT was NOT in its power-gated idle state. This is a proxy
// for "GPU busy", not an engine counter. Only i915 is read; xe exposes its
// idle counters under a different path and is not claimed.

// Seams for tests. intelDRMRoot is the DRM class directory, intelSleep and
// intelClock let the seed window run without real time.
var (
	intelDRMRoot = "/sys/class/drm"
	intelClock   = time.Now
	intelSleep   = time.Sleep
	intelLspci   = readIntelNameFromLspci
)

const (
	intelVendorID = "0x8086"
	intelDriver   = "i915"
	// intelIGPUPCI is the display function of the CPU's integrated graphics
	// on Intel client platforms. Other Intel display functions are dGPUs.
	intelIGPUPCI = "0000:00:02.0"
	// intelSeedWindow is the first-sample wait: one RC6 delta needs two reads.
	// It is half a second, not the Load box's one-second seed budget, because
	// the dashboard's first frame waits for it (TestWatchPTYLifecycle wants
	// the first frame under 1s). RC6 counts in whole ms, so 0.2% resolution.
	intelSeedWindow = 500 * time.Millisecond
)

// intelRC6Sample is the last RC6 residency reading for one GPU and the time
// it was taken.
type intelRC6Sample struct {
	residencyMS uint64
	at          time.Time
}

var (
	intelMu        sync.Mutex
	intelLast      = map[string]intelRC6Sample{}
	intelNameByPCI = map[string]string{}
)

// currentIntelGPUs returns each i915-bound Intel display GPU of the host. A
// GPU without a readable RC6 counter is left out, so the box shows no row
// rather than a made-up load. A GPU seen for the first time costs one
// intelSeedWindow of wall time, so one-shot `harnez usage` (a fresh process
// with no earlier reading) shows the GPU too.
func currentIntelGPUs() []GPU {
	cards, err := filepath.Glob(filepath.Join(intelDRMRoot, "card[0-9]*"))
	if err != nil {
		return nil
	}
	var gpus []GPU
	for _, card := range cards {
		if !isCardNode(filepath.Base(card)) {
			continue
		}
		deviceDir := filepath.Join(card, "device")
		if !isIntelI915(deviceDir) {
			continue
		}
		key := pciAddressOf(deviceDir)
		busy, ok := intelBusyPercent(key, filepath.Join(card, "gt", "gt0", "rc6_residency_ms"))
		if !ok {
			continue
		}
		g := GPU{
			Name:        intelGPUName(key),
			PCI:         key,
			UtilPercent: busy,
			UtilHistory: appendGPUHistory(key, busy),
			// The iGPU's memory is system RAM, so the Load box has no VRAM/GTT
			// split to show for it (see the open memory-row question in issue 1219).
			Class: classifyGPU(isExternalGPU(deviceDir), key == intelIGPUPCI),
		}
		gpus = append(gpus, g)
	}
	return gpus
}

// intelBusyPercent returns the GPU's busy share over the window since its
// previous reading. Its first reading seeds by sleeping one intelSeedWindow
// and reading again. A counter that moved backwards (suspend or reset) is
// skipped once.
func intelBusyPercent(key, rc6Path string) (float64, bool) {
	cur, err := readSysfsUint(rc6Path)
	if err != nil {
		return 0, false
	}
	now := intelClock()
	intelMu.Lock()
	prev, have := intelLast[key]
	intelLast[key] = intelRC6Sample{residencyMS: cur, at: now}
	intelMu.Unlock()
	if !have {
		intelSleep(intelSeedWindow)
		return intelBusyPercent(key, rc6Path)
	}
	wallMS := float64(now.Sub(prev.at)) / float64(time.Millisecond)
	if wallMS <= 0 || cur < prev.residencyMS {
		return 0, false
	}
	idleMS := float64(cur - prev.residencyMS)
	busy := 100 * (1 - idleMS/wallMS)
	if busy < 0 {
		busy = 0
	}
	if busy > 100 {
		busy = 100
	}
	return busy, true
}

// isIntelI915 reports whether deviceDir is an Intel display-class PCI
// function bound to the i915 driver.
func isIntelI915(deviceDir string) bool {
	vendor, err := os.ReadFile(filepath.Join(deviceDir, "vendor"))
	if err != nil || strings.TrimSpace(string(vendor)) != intelVendorID {
		return false
	}
	class, err := os.ReadFile(filepath.Join(deviceDir, "class"))
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(class)), "0x03") {
		return false
	}
	driver, err := filepath.EvalSymlinks(filepath.Join(deviceDir, "driver"))
	return err == nil && filepath.Base(driver) == intelDriver
}

// isCardNode reports whether name is a DRM card node ("card0", "card1"),
// not a connector such as "card1-DP-1".
func isCardNode(name string) bool {
	rest, ok := strings.CutPrefix(name, "card")
	if !ok || rest == "" {
		return false
	}
	_, err := strconv.Atoi(rest)
	return err == nil
}

// intelGPUName returns the short marketing name of the GPU at pci, for example
// "UHD 730", resolved through lspci and memoized per PCI address. Without
// lspci it falls back to "Intel GPU".
func intelGPUName(pci string) string {
	intelMu.Lock()
	if v, ok := intelNameByPCI[pci]; ok {
		intelMu.Unlock()
		return v
	}
	intelMu.Unlock()

	name := "Intel GPU"
	if device, err := intelLspci(pci); err == nil {
		if short := intelShortName(device); short != "" {
			name = short
		}
	}

	intelMu.Lock()
	intelNameByPCI[pci] = name
	intelMu.Unlock()
	return name
}

// readIntelNameFromLspci returns lspci's full device string for the Intel
// display function at pci.
func readIntelNameFromLspci(pci string) (string, error) {
	out, err := exec.Command("lspci", "-D", "-d", "8086:", "-mm").Output()
	if err != nil {
		return "", err
	}
	return lspciDeviceField(out, pci)
}

// intelShortName turns lspci's "Alder Lake-S GT1 [UHD Graphics 730]" into
// "UHD 730": the bracketed marketing name without the word "Graphics", so it
// fits the Load box label. Without brackets the device string is kept.
func intelShortName(device string) string {
	name := device
	if i := strings.Index(device, "["); i >= 0 {
		if j := strings.LastIndex(device, "]"); j > i {
			name = device[i+1 : j]
		}
	}
	return strings.TrimSpace(strings.ReplaceAll(name, " Graphics", ""))
}
