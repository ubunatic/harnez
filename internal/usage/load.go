package usage

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CPULoad holds the system load averages, the instantaneous CPU usage, and
// the core count needed to render the watch grid's Load panel.
type CPULoad struct {
	Load1  float64
	Load5  float64
	Load15 float64
	NumCPU int
	Ok     bool

	// CPUPercent is the fraction of CPU time spent busy (0-100) since the
	// previous sample, as opposed to Load1/5/15's kernel-smoothed averages.
	// It is only meaningful once a prior sample exists (see cpuStatCache);
	// CPUPercentOk reports whether that happened.
	CPUPercent   float64
	CPUPercentOk bool

	// PerCorePercent is CPUPercent broken out per core (cpu0, cpu1, ...),
	// same delta technique, same freshness caveat as CPUPercentOk.
	PerCorePercent []float64
	PerCoreOk      bool

	// PercentHistory is a short rolling window of recent CPUPercent
	// samples, oldest first, for the Load box's timeline sparkline. A
	// spatial per-core snapshot barely moves frame to frame; a trend over
	// the last ~10 samples is more informative.
	PercentHistory []float64

	// TempC is the CPU package temperature, read from the first recognized
	// hwmon driver (k10temp/zenpower on AMD, coretemp on Intel). TempOk is
	// false where none of those drivers is present.
	TempC  float64
	TempOk bool
}

// CurrentCPULoad reads the system load averages and the instantaneous CPU
// usage. Load averages come from /proc/loadavg, falling back to the
// `uptime` command where /proc is unavailable (e.g. macOS, containers
// without procfs). CPU usage is delta-based off /proc/stat and has no
// non-Linux fallback: CPUPercentOk is false where /proc/stat is missing.
func CurrentCPULoad() CPULoad {
	load, err := readLoadavgFromProc("/proc/loadavg")
	if err != nil {
		load, err = readLoadavgFromUptime()
	}
	if err != nil {
		load = CPULoad{}
	} else {
		load.Ok = true
	}
	load.NumCPU = runtime.NumCPU()
	load.CPUPercent, load.CPUPercentOk, load.PerCorePercent, load.PerCoreOk = currentCPUPercents()
	load.PercentHistory = cpuHistory.snapshot()
	load.TempC, load.TempOk = readCPUTempFromSysfs()
	return load
}

// cpuTempHwmonDrivers are the hwmon driver names known to report a CPU
// package/die temperature (as opposed to battery, NVMe, Wi-Fi, etc. hwmons
// that also live under /sys/class/hwmon).
var cpuTempHwmonDrivers = map[string]bool{
	"k10temp":     true, // AMD (Zen 1-5)
	"zenpower":    true, // AMD (Zen, community driver)
	"coretemp":    true, // Intel
	"cpu_thermal": true, // ARM SoCs
}

// cpuTempPreferredLabels are the sensor labels, in priority order, that
// best represent "the" CPU temperature within a multi-sensor hwmon device
// (e.g. coretemp exposes one temp per core plus a package sensor).
var cpuTempPreferredLabels = []string{"Tctl", "Tdie", "Package id 0", "CPU"}

// readCPUTempFromSysfs finds the first recognized CPU hwmon driver under
// /sys/class/hwmon and reads its package temperature, preferring a sensor
// labeled Tctl/Tdie/"Package id 0"/CPU over an arbitrary temp1_input.
func readCPUTempFromSysfs() (float64, bool) {
	nameFiles, err := filepath.Glob("/sys/class/hwmon/hwmon*/name")
	if err != nil {
		return 0, false
	}
	for _, nameFile := range nameFiles {
		data, err := os.ReadFile(nameFile)
		if err != nil || !cpuTempHwmonDrivers[strings.TrimSpace(string(data))] {
			continue
		}
		dir := filepath.Dir(nameFile)

		labelFiles, _ := filepath.Glob(filepath.Join(dir, "temp*_label"))
		for _, want := range cpuTempPreferredLabels {
			for _, labelFile := range labelFiles {
				label, err := os.ReadFile(labelFile)
				if err != nil || strings.TrimSpace(string(label)) != want {
					continue
				}
				inputFile := strings.TrimSuffix(labelFile, "_label") + "_input"
				if milliC, err := readSysfsUint(inputFile); err == nil {
					return float64(milliC) / 1000, true
				}
			}
		}

		inputFiles, _ := filepath.Glob(filepath.Join(dir, "temp*_input"))
		for _, inputFile := range inputFiles {
			if milliC, err := readSysfsUint(inputFile); err == nil {
				return float64(milliC) / 1000, true
			}
		}
	}
	return 0, false
}

// cpuStatSample is one reading of a "cpu"/"cpuN" line in /proc/stat:
// cumulative jiffies since boot, split into idle and total.
type cpuStatSample struct {
	idle  uint64
	total uint64
}

// cpuStatFrame is one full /proc/stat sample: the aggregate "cpu" line plus
// the per-core "cpuN" lines beneath it, read together so aggregate and
// per-core percentages always come from the same two points in time.
type cpuStatFrame struct {
	aggregate cpuStatSample
	perCore   []cpuStatSample
}

var (
	cpuFrameMu   sync.Mutex
	lastCPUFrame cpuStatFrame
	haveCPUFrame bool
)

// historyBurstInterval is the spacing between samples when seeding a
// timeline's history at startup (see burstSeedCPUHistory,
// burstSeedGPUHistory). loadHistoryLen samples at this interval take
// ~loadHistoryLen*historyBurstInterval of real wall-clock time — there's no
// way to read the past from a kernel counter that only tracks a
// cumulative total, so filling the chart immediately still costs real time,
// just compressed from the ~10s it'd otherwise take at the 1s redraw
// cadence down to about 1s.
const historyBurstInterval = 100 * time.Millisecond

// currentCPUPercents reports aggregate and per-core CPU busy% since the last
// call, computed from the delta between two /proc/stat frames (btop-style
// "real" CPU, distinct from the kernel's minute-scale load averages). The
// first call in a process has no prior sample to diff against, so it burst-
// samples (see historyBurstInterval) to both get an immediate reading and
// seed cpuHistory with a full timeline instead of growing one glyph per
// redraw; every later call in the same `--watch` process reuses the
// previous frame's sample instead.
func currentCPUPercents() (float64, bool, []float64, bool) {
	cur, err := readCPUStatFrame("/proc/stat")
	if err != nil {
		return 0, false, nil, false
	}

	cpuFrameMu.Lock()
	prev := lastCPUFrame
	had := haveCPUFrame
	lastCPUFrame = cur
	haveCPUFrame = true
	cpuFrameMu.Unlock()

	if !had {
		return burstSeedCPUHistory(cur)
	}
	aggPct, aggOk, perCore, perCoreOk := cpuFramePercents(prev, cur)
	if aggOk {
		cpuHistory.append(aggPct)
	}
	return aggPct, aggOk, perCore, perCoreOk
}

// burstSeedCPUHistory takes loadHistoryLen rapid /proc/stat samples,
// historyBurstInterval apart, appending each delta's aggregate % to
// cpuHistory so the Load box's CPU timeline starts already filled. Returns
// the final sample's aggregate and per-core percentages, same shape as
// currentCPUPercents' normal path.
func burstSeedCPUHistory(first cpuStatFrame) (float64, bool, []float64, bool) {
	prev := first
	var aggPct float64
	var aggOk bool
	var perCore []float64
	var perCoreOk bool
	for i := 0; i < loadHistoryLen; i++ {
		time.Sleep(historyBurstInterval)
		cur, err := readCPUStatFrame("/proc/stat")
		if err != nil {
			break
		}
		aggPct, aggOk, perCore, perCoreOk = cpuFramePercents(prev, cur)
		if aggOk {
			cpuHistory.append(aggPct)
		}
		cpuFrameMu.Lock()
		lastCPUFrame = cur
		cpuFrameMu.Unlock()
		prev = cur
	}
	return aggPct, aggOk, perCore, perCoreOk
}

func cpuFramePercents(prev, cur cpuStatFrame) (float64, bool, []float64, bool) {
	aggPct, aggOk := cpuPercentFromSamples(prev.aggregate, cur.aggregate)

	n := len(cur.perCore)
	if len(prev.perCore) < n {
		n = len(prev.perCore)
	}
	if n == 0 {
		return aggPct, aggOk, nil, false
	}
	perCore := make([]float64, n)
	for i := 0; i < n; i++ {
		pct, _ := cpuPercentFromSamples(prev.perCore[i], cur.perCore[i])
		perCore[i] = pct
	}
	return aggPct, aggOk, perCore, true
}

func cpuPercentFromSamples(prev, cur cpuStatSample) (float64, bool) {
	totalDelta := cur.total - prev.total
	if totalDelta == 0 {
		return 0, false
	}
	idleDelta := cur.idle - prev.idle
	busy := totalDelta - idleDelta
	return float64(busy) / float64(totalDelta) * 100, true
}

// readCPUStatFrame parses the aggregate "cpu  user nice system idle iowait
// irq softirq steal guest guest_nice" line and the per-core "cpuN ..." lines
// beneath it at the top of /proc/stat. idle covers idle+iowait per the usual
// convention (e.g. htop).
func readCPUStatFrame(path string) (cpuStatFrame, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cpuStatFrame{}, err
	}

	var frame cpuStatFrame
	sawAggregate := false
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || !strings.HasPrefix(fields[0], "cpu") {
			if sawAggregate {
				break // per-core lines are contiguous right after "cpu"
			}
			continue
		}
		sample, err := parseCPUStatFields(fields[1:])
		if err != nil {
			return cpuStatFrame{}, err
		}
		if fields[0] == "cpu" {
			frame.aggregate = sample
			sawAggregate = true
		} else {
			frame.perCore = append(frame.perCore, sample)
		}
	}
	if !sawAggregate {
		return cpuStatFrame{}, fmt.Errorf("unexpected /proc/stat format: no aggregate cpu line")
	}
	return frame, nil
}

func parseCPUStatFields(fields []string) (cpuStatSample, error) {
	var sample cpuStatSample
	for i, f := range fields {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return cpuStatSample{}, err
		}
		sample.total += v
		if i == 3 || i == 4 { // idle, iowait
			sample.idle += v
		}
	}
	return sample, nil
}

func readLoadavgFromProc(path string) (CPULoad, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CPULoad{}, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return CPULoad{}, fmt.Errorf("unexpected /proc/loadavg format: %q", string(data))
	}
	l1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return CPULoad{}, err
	}
	l5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return CPULoad{}, err
	}
	l15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return CPULoad{}, err
	}
	return CPULoad{Load1: l1, Load5: l5, Load15: l15}, nil
}

// readLoadavgFromUptime parses the trailing "load averages: 1.23 1.45 1.67"
// (or "load average:") segment of `uptime` output.
// GPU holds one GPU's utilization, VRAM usage, and temperature.
type GPU struct {
	Name        string
	UtilPercent float64
	MemUsedMiB  float64
	MemTotalMiB float64
	MemPercent  float64
	TempC       float64
	HaveMem     bool
	HaveTemp    bool

	// UtilHistory is a short rolling window of recent UtilPercent samples,
	// oldest first, populated by the AMD sysfs path (cheap enough to poll
	// every redraw).
	UtilHistory []float64
}

// loadHistoryLen is how many recent samples are kept for a Load box
// timeline sparkline (CPU aggregate %, or one per GPU's utilization %).
const loadHistoryLen = 10

// sampleHistory is a small mutex-protected rolling window of recent 0-100%
// samples, used to render a Load box timeline sparkline.
type sampleHistory struct {
	mu      sync.Mutex
	samples []float64
}

// append records pct as the latest sample and returns a copy of the
// trailing loadHistoryLen-sample window (oldest first).
func (h *sampleHistory) append(pct float64) []float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.samples = append(h.samples, pct)
	if len(h.samples) > loadHistoryLen {
		h.samples = h.samples[len(h.samples)-loadHistoryLen:]
	}
	out := make([]float64, len(h.samples))
	copy(out, h.samples)
	return out
}

// snapshot returns a copy of the current window without appending to it.
func (h *sampleHistory) snapshot() []float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]float64, len(h.samples))
	copy(out, h.samples)
	return out
}

// cpuHistory is the single rolling window for CurrentCPULoad's aggregate %.
var cpuHistory sampleHistory

// gpuHistoryMu guards gpuHistory, a per-GPU (keyed by sysfs card name)
// rolling window, since a system can have more than one GPU.
var (
	gpuHistoryMu sync.Mutex
	gpuHistory   = map[string]*sampleHistory{}
)

// appendGPUHistory records pct as the latest sample for the GPU identified
// by key (its sysfs card name) and returns a copy of its trailing window.
func appendGPUHistory(key string, pct float64) []float64 {
	gpuHistoryMu.Lock()
	h, ok := gpuHistory[key]
	if !ok {
		h = &sampleHistory{}
		gpuHistory[key] = h
	}
	gpuHistoryMu.Unlock()
	return h.append(pct)
}

// gpuHistorySeeded reports whether the GPU identified by key already has a
// history window, i.e. whether it's been through burstSeedGPUHistory.
func gpuHistorySeeded(key string) bool {
	gpuHistoryMu.Lock()
	defer gpuHistoryMu.Unlock()
	_, ok := gpuHistory[key]
	return ok
}

// burstSeedGPUHistory takes loadHistoryLen rapid samples of busyPath
// (historyBurstInterval apart), appending each to the GPU's history window,
// so its timeline starts already filled. sysfs reads are cheap (~microsecond
// scale), so the cost here is entirely the intentional pacing, same
// approach as burstSeedCPUHistory.
func burstSeedGPUHistory(key, busyPath string) []float64 {
	var hist []float64
	for i := 0; i < loadHistoryLen; i++ {
		v, err := readSysfsUint(busyPath)
		if err != nil {
			break
		}
		hist = appendGPUHistory(key, float64(v))
		if i < loadHistoryLen-1 {
			time.Sleep(historyBurstInterval)
		}
	}
	return hist
}

// CurrentGPUs probes for AMD GPUs and returns whatever readings are
// available, reading directly from sysfs (kernel-cached counters, a plain
// read() per file) — no subprocess/vendor-tool fallback of any kind. See
// docs/studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md: this
// project calls no vendor CLI/SDK for device telemetry (that policy
// initially removed nvidia-smi but kept rocm-smi as a fallback; rocm-smi
// was removed in the same pass shortly after, once it was clear the policy
// should apply uniformly rather than only to NVIDIA). NVIDIA's proprietary
// driver exposes nothing through procfs/sysfs at all, so there's no
// kernel-standard path for it. Returns nil if nothing is present or
// everything fails (e.g. no GPU, an NVIDIA-only system, or an AMD system
// whose driver doesn't populate the expected sysfs attributes).
func CurrentGPUs() []GPU {
	if gpus, err := readAMDSysfs(); err == nil {
		return gpus
	}
	return nil
}

// amdCodenameByDeviceID is a best-effort fallback table mapping a PCI
// device ID (as read from /sys/.../device, lowercase hex with "0x" prefix)
// to its AMD GPU/APU codename, for systems without `lspci` available. Only
// entries verified against real hardware are included — extend as more are
// confirmed rather than guessing IDs, since a wrong entry silently mislabels
// the box while a missing one just falls through to a generic "AMD GPU".
var amdCodenameByDeviceID = map[string]string{
	"0x1638": "Cezanne", // Ryzen 5000 (Zen3) APU, e.g. Ryzen 7 5800U
}

// amdCodenameCache memoizes the local AMD GPU's codename for the process
// lifetime: the lspci probe it may run is a subprocess, and the hardware
// it's describing can't change mid-session.
var (
	amdCodenameMu    sync.Mutex
	amdCodenameValue string
	amdCodenameKnown bool
)

// amdGPUCodename resolves deviceDir's (a /sys/class/drm/cardN/device path)
// AMD codename, preferring the live system (`lspci`'s device string, e.g.
// "Cezanne") over the built-in fallback table, since the table is
// necessarily incomplete.
func amdGPUCodename(deviceDir string) string {
	amdCodenameMu.Lock()
	if amdCodenameKnown {
		v := amdCodenameValue
		amdCodenameMu.Unlock()
		return v
	}
	amdCodenameMu.Unlock()

	name := ""
	if lspciName, err := readAMDCodenameFromLspci(); err == nil && lspciName != "" {
		name = lspciName
	} else if deviceID, err := os.ReadFile(filepath.Join(deviceDir, "device")); err == nil {
		if v, ok := amdCodenameByDeviceID[strings.TrimSpace(string(deviceID))]; ok {
			name = v
		}
	}
	if name == "" {
		name = "AMD GPU"
	}

	amdCodenameMu.Lock()
	amdCodenameValue, amdCodenameKnown = name, true
	amdCodenameMu.Unlock()
	return name
}

// readAMDCodenameFromLspci shells out to `lspci -d 1002: -mm` (machine-
// readable, quoted fields) and extracts the device string's codename
// (e.g. "Cezanne" from "Cezanne [Radeon Vega Series / ...]") for the first
// VGA/3D/display-class AMD function. Called at most once per process via
// amdGPUCodename's cache.
func readAMDCodenameFromLspci() (string, error) {
	out, err := exec.Command("lspci", "-d", "1002:", "-mm").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		// lspci -mm quotes each field: Slot "Class" "Vendor" "Device" ...
		fields := strings.Split(line, `"`)
		if len(fields) < 6 {
			continue
		}
		class := fields[1]
		if !strings.Contains(class, "VGA") && !strings.Contains(class, "3D controller") && !strings.Contains(class, "Display controller") {
			continue
		}
		device := fields[5]
		if idx := strings.Index(device, " ["); idx >= 0 {
			device = device[:idx]
		}
		if device = strings.TrimSpace(device); device != "" {
			return device, nil
		}
	}
	return "", fmt.Errorf("no AMD VGA/3D/display device found in lspci output")
}

// readAMDSysfs reads AMD GPU utilization, VRAM, and temperature straight
// from the kernel's amdgpu sysfs attributes, with no subprocess involved.
// Card enumeration (cardN) and hwmon enumeration (hwmonN) are both
// driver-assigned at runtime, so both are globbed rather than assumed.
func readAMDSysfs() ([]GPU, error) {
	matches, err := filepath.Glob("/sys/class/drm/card*/device/gpu_busy_percent")
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no amdgpu sysfs busy-percent attributes found")
	}

	var gpus []GPU
	for _, busyPath := range matches {
		deviceDir := filepath.Dir(busyPath)
		cardName := filepath.Base(filepath.Dir(deviceDir))
		g := GPU{Name: amdGPUCodename(deviceDir)}

		if gpuHistorySeeded(cardName) {
			v, err := readSysfsUint(busyPath)
			if err != nil {
				continue // not a real GPU device (or unreadable): skip rather than report zeros
			}
			g.UtilPercent = float64(v)
			g.UtilHistory = appendGPUHistory(cardName, g.UtilPercent)
		} else {
			// First time seeing this card: burst-sample so its timeline
			// starts already filled instead of growing one glyph per
			// redraw (see historyBurstInterval).
			hist := burstSeedGPUHistory(cardName, busyPath)
			if len(hist) == 0 {
				continue
			}
			g.UtilHistory = hist
			g.UtilPercent = hist[len(hist)-1]
		}

		used, errUsed := readSysfsUint(filepath.Join(deviceDir, "mem_info_vram_used"))
		total, errTotal := readSysfsUint(filepath.Join(deviceDir, "mem_info_vram_total"))
		if errUsed == nil && errTotal == nil && total > 0 {
			g.MemUsedMiB = float64(used) / (1024 * 1024)
			g.MemTotalMiB = float64(total) / (1024 * 1024)
			g.MemPercent = float64(used) / float64(total) * 100
			g.HaveMem = true
		}

		if hwmonMatches, err := filepath.Glob(filepath.Join(deviceDir, "hwmon", "hwmon*", "temp1_input")); err == nil && len(hwmonMatches) > 0 {
			if milliC, err := readSysfsUint(hwmonMatches[0]); err == nil {
				g.TempC = float64(milliC) / 1000
				g.HaveTemp = true
			}
		}

		gpus = append(gpus, g)
	}
	if len(gpus) == 0 {
		return nil, fmt.Errorf("amdgpu sysfs attributes present but unreadable")
	}
	return gpus, nil
}

// readSysfsUint reads a sysfs file holding a single unsigned integer value.
func readSysfsUint(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
}

func readLoadavgFromUptime() (CPULoad, error) {
	out, err := exec.Command("uptime").Output()
	if err != nil {
		return CPULoad{}, err
	}
	text := string(out)
	idx := strings.LastIndex(text, "load average")
	if idx < 0 {
		return CPULoad{}, fmt.Errorf("unexpected uptime format: %q", text)
	}
	rest := text[idx:]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return CPULoad{}, fmt.Errorf("unexpected uptime format: %q", text)
	}
	fields := strings.FieldsFunc(rest[colon+1:], func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	if len(fields) < 3 {
		return CPULoad{}, fmt.Errorf("unexpected uptime format: %q", text)
	}
	l1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return CPULoad{}, err
	}
	l5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return CPULoad{}, err
	}
	l15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return CPULoad{}, err
	}
	return CPULoad{Load1: l1, Load5: l5, Load15: l15}, nil
}
