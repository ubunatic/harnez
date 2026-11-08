package usage

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CurrentCPULoad reads the system load averages and the instantaneous CPU
// usage. Load averages come from /proc/loadavg, falling back to the
// `uptime` command where /proc is unavailable (e.g. macOS, containers
// without procfs). CPU usage is delta-based off /proc/stat and has no
// non-Linux fallback: CPUPercentOk is false where /proc/stat is missing.
func CurrentCPULoad() CPULoad {
	return currentCPULoad(true)
}

func currentCPULoad(seed bool) CPULoad {
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
	load.CPUPercent, load.CPUPercentOk, load.PerCorePercent, load.PerCoreOk = currentCPUPercentsWithSeed(seed)
	load.PercentHistory = cpuHistory.snapshot()
	load.TempC, load.TempOk = readCPUTempFromSysfs()
	load.PowerW, load.PowerOk, load.RAPLReadable = readRAPLPower(raplSysRoot, time.Now)
	load.Memory = CurrentSystemMemory()
	return load
}

// CollectLoadSnapshot gathers the same CPU/GPU readings CurrentCPULoad and
// CurrentGPUs already produce, packaged as a LoadSnapshot for JSON transport
// (issue 090: carrying Load panel data through the remote `usage --json`
// payload so `--host` mode never has to open a second SSH connection at the
// Load panel's redraw cadence).
func CollectLoadSnapshot() LoadSnapshot {
	return LoadSnapshot{
		CPU:  CurrentCPULoad(),
		GPUs: CurrentGPUs(),
	}
}

// CurrentSystemMemory reads Linux memory pressure from /proc/meminfo.
func CurrentSystemMemory() SystemMemory {
	mem, err := readMeminfo("/proc/meminfo")
	if err != nil {
		return SystemMemory{}
	}
	mem.Geometry = readRAMGeometry(mem.TotalMiB)
	pct := percent(mem.UsedMiB, mem.TotalMiB)
	mem.PercentHistory = ramHistory.append(pct)
	return mem
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

// raplSysRoot is the powercap class directory and raplPackagePrefix the
// name-file prefix of a CPU package zone. Both are kernel conventions, not
// configuration. AMD CPUs expose their package counter under the same names.
const (
	raplSysRoot       = "/sys/class/powercap"
	raplPackagePrefix = "package-"
)

// raplRateInterval is the shortest span between RAPL baselines. Closer calls
// reuse the last computed power, so frequent callers do not produce noisy
// readings from tiny energy deltas.
const raplRateInterval = time.Second

// raplSample is one reading of the package energy counter and its clock.
type raplSample struct {
	energyUJ uint64
	at       time.Time
}

var (
	raplMu        sync.Mutex
	lastRAPL      raplSample
	haveRAPL      bool
	lastRAPLWatts float64
	haveRAPLWatts bool
)

// findRAPLPackageZone returns the top-level package zone under root: the first
// intel-rapl:N in sorted order (subzones such as intel-rapl:N:M are skipped)
// whose name file starts with "package-".
func findRAPLPackageZone(root string) (string, bool) {
	zones, err := filepath.Glob(filepath.Join(root, "intel-rapl:*"))
	if err != nil {
		return "", false
	}
	sort.Strings(zones)
	for _, zone := range zones {
		if strings.Count(filepath.Base(zone), ":") != 1 {
			continue
		}
		if strings.HasPrefix(readSysfsLabel(filepath.Join(zone, "name")), raplPackagePrefix) {
			return zone, true
		}
	}
	return "", false
}

// readRAPLPower returns the CPU package power in watts from the energy delta
// between two samples of the package zone under root (the powercap class dir).
//
// readable reports whether the zone and its energy_uj could be read at all;
// it is what decides between RAPL and the APU PPT fallback. ok reports whether
// a power value exists. The first sample has no delta, so ok is false.
//
// Calls less than raplRateInterval after the stored baseline do not advance
// it; they return the last computed watts (ok only if one exists). The clock
// and energy_uj are read under the mutex, so stored samples stay in order.
// A counter that went backwards is one wrap at max_energy_range_uj; without
// that file the sample only resets the baseline. Failures are silent by
// design: RAPL is root-only on many systems.
func readRAPLPower(root string, now func() time.Time) (watts float64, ok, readable bool) {
	zone, found := findRAPLPackageZone(root)
	if !found {
		return 0, false, false
	}

	raplMu.Lock()
	defer raplMu.Unlock()
	at := now()
	energy, err := readSysfsUint(filepath.Join(zone, "energy_uj"))
	if err != nil {
		return 0, false, false
	}
	cur := raplSample{energyUJ: energy, at: at}
	if !haveRAPL {
		lastRAPL, haveRAPL = cur, true
		return 0, false, true
	}
	dt := at.Sub(lastRAPL.at)
	if dt < raplRateInterval {
		return lastRAPLWatts, haveRAPLWatts, true
	}
	prev := lastRAPL
	lastRAPL = cur

	var deltaUJ uint64
	if energy >= prev.energyUJ {
		deltaUJ = energy - prev.energyUJ
	} else {
		maxRange, err := readSysfsUint(filepath.Join(zone, "max_energy_range_uj"))
		if err != nil || maxRange <= prev.energyUJ {
			haveRAPLWatts = false
			return 0, false, true
		}
		deltaUJ = (maxRange - prev.energyUJ) + energy
	}
	lastRAPLWatts, haveRAPLWatts = float64(deltaUJ)/1e6/dt.Seconds(), true
	return lastRAPLWatts, true, true
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
// Keep the startup seed near one second even though the doubled timeline now
// collects twenty observations.
const historyBurstInterval = 50 * time.Millisecond

func currentCPUPercentsWithSeed(seed bool) (float64, bool, []float64, bool) {
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
		if !seed {
			return 0, false, nil, false
		}
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
			cpuHistory.forceAppend(aggPct)
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

func readMeminfo(path string) (SystemMemory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SystemMemory{}, err
	}
	return parseMeminfo(string(data))
}

func parseMeminfo(text string) (SystemMemory, error) {
	values := map[string]uint64{}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		valueKiB, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return SystemMemory{}, err
		}
		values[key] = valueKiB
	}

	total, ok := values["MemTotal"]
	if !ok || total == 0 {
		return SystemMemory{}, fmt.Errorf("unexpected /proc/meminfo format: missing MemTotal")
	}
	available, ok := values["MemAvailable"]
	if !ok {
		free := values["MemFree"]
		buffers := values["Buffers"]
		cached := values["Cached"]
		reclaimable := values["SReclaimable"]
		shmem := values["Shmem"]
		available = free + buffers + cached + reclaimable
		if available >= shmem {
			available -= shmem
		}
	}
	if available > total {
		available = total
	}
	swapTotal, haveSwapTotal := values["SwapTotal"]
	swapFree, haveSwapFree := values["SwapFree"]
	swapKnown := haveSwapTotal && haveSwapFree
	swapUsed := uint64(0)
	if swapKnown && swapFree < swapTotal {
		swapUsed = swapTotal - swapFree
	}
	return SystemMemory{
		UsedMiB:      float64(total-available) / 1024,
		TotalMiB:     float64(total) / 1024,
		AvailableMiB: float64(available) / 1024,
		SwapUsedMiB:  float64(swapUsed) / 1024,
		SwapTotalMiB: float64(swapTotal) / 1024,
		SwapKnown:    swapKnown,
		Ok:           true,
		Geometry:     fallbackRAMGeometry(float64(total) / 1024),
	}, nil
}

// readLoadavgFromUptime parses the trailing "load averages: 1.23 1.45 1.67"
// (or "load average:") segment of `uptime` output.

// loadHistoryLen is the doubled internal resolution of a width-10 Load box
// timeline: each rendered cell consumes two chronological samples.
const loadHistoryLen = 20

// minLoadSampleInterval is the minimum spacing between consecutive appends
// to rolling hardware load histories (CPU, GPU, RAM). When the TUI redraws
// at high FPS (e.g. 20 Hz during speech-activated mic activity), hardware
// timeline charts must maintain a steady 1s temporal resolution instead of
// scrolling violently.
const minLoadSampleInterval = 1 * time.Second

// sampleHistory is a small mutex-protected rolling window of recent 0-100%
// samples, used to render a Load box timeline sparkline.
type sampleHistory struct {
	mu         sync.Mutex
	samples    []float64
	lastSample time.Time
}

// append records pct as the latest sample if at least minLoadSampleInterval has
// elapsed since the previous sample (or if no sample exists yet), and returns a
// copy of the trailing loadHistoryLen-sample window (oldest first). If called
// within minLoadSampleInterval, it updates the latest sample in place and returns
// the snapshot without advancing the historical time-series ring buffer.
func (h *sampleHistory) append(pct float64) []float64 {
	return h.appendAt(pct, time.Now(), minLoadSampleInterval)
}

// appendAt records pct at a specific timestamp and minimum interval threshold.
func (h *sampleHistory) appendAt(pct float64, now time.Time, minInterval time.Duration) []float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.lastSample.IsZero() || now.Sub(h.lastSample) >= minInterval {
		h.samples = append(h.samples, pct)
		if len(h.samples) > loadHistoryLen {
			h.samples = h.samples[len(h.samples)-loadHistoryLen:]
		}
		h.lastSample = now
	} else if len(h.samples) > 0 {
		h.samples[len(h.samples)-1] = pct
	}
	out := make([]float64, len(h.samples))
	copy(out, h.samples)
	return out
}

// forceAppend unconditionally appends pct to the history window, bypassing the
// rate throttle (used e.g. during startup burst seeding and unit tests).
func (h *sampleHistory) forceAppend(pct float64) []float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.samples = append(h.samples, pct)
	if len(h.samples) > loadHistoryLen {
		h.samples = h.samples[len(h.samples)-loadHistoryLen:]
	}
	h.lastSample = time.Now()
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

// ramHistory is the single rolling window for CurrentSystemMemory's utilization %.
var ramHistory sampleHistory

// gpuHistoryMu guards gpuHistory, a per-GPU (keyed by sysfs card name)
// rolling window, since a system can have more than one GPU.
var (
	gpuHistoryMu sync.Mutex
	gpuHistory   = map[string]*sampleHistory{}
	vramHistory  = map[string]*sampleHistory{}
	gttHistory   = map[string]*sampleHistory{}
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

func forceAppendGPUHistory(key string, pct float64) []float64 {
	gpuHistoryMu.Lock()
	h, ok := gpuHistory[key]
	if !ok {
		h = &sampleHistory{}
		gpuHistory[key] = h
	}
	gpuHistoryMu.Unlock()
	return h.forceAppend(pct)
}

func appendVRAMHistory(key string, pct float64) []float64 {
	gpuHistoryMu.Lock()
	h, ok := vramHistory[key]
	if !ok {
		h = &sampleHistory{}
		vramHistory[key] = h
	}
	gpuHistoryMu.Unlock()
	return h.append(pct)
}

func appendGTTHistory(key string, pct float64) []float64 {
	gpuHistoryMu.Lock()
	h, ok := gttHistory[key]
	if !ok {
		h = &sampleHistory{}
		gttHistory[key] = h
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
		hist = forceAppendGPUHistory(key, float64(v))
		if i < loadHistoryLen-1 {
			time.Sleep(historyBurstInterval)
		}
	}
	return hist
}

// CurrentGPUs returns every readable GPU of the host: AMD first (read from
// sysfs, kernel-cached counters), then NVIDIA (read via nvidia-smi, gated and
// cached, see gpu_nvidia.go). The 2026-08-28 kernel-standard-only ADR is
// superseded for NVIDIA by issue 765; AMD still has no vendor-tool path. A
// GPU that cannot be read is left out, and nil means no readable GPU at all.
// Issue 765 removed the earlier nvidia-smi reader for policy reasons and for
// its cost: one call takes ~123ms, so it now runs at most once per the
// spec's nvidia_smi_cache_ttl and only when NVIDIA hardware is present.
func CurrentGPUs() []GPU {
	return currentGPUs(true)
}

func currentGPUs(seed bool) []GPU {
	var gpus []GPU
	if amd, err := readAMDSysfsWithSeed(seed); err == nil {
		gpus = amd
	}
	// NVIDIA cards come after AMD; the probe is gated and cached in gpu_nvidia.go.
	return append(gpus, currentNVIDIAGPUs()...)
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

// amdCodenameByPCI memoizes each AMD GPU's codename by PCI address for the
// process lifetime: the lspci probe it may run is a subprocess, and the
// hardware it's describing can't change mid-session. Keyed per device so two
// AMD GPUs never share one name (issue 765).
var (
	amdCodenameMu    sync.Mutex
	amdCodenameByPCI = map[string]string{}
)

// amdGPUCodename resolves deviceDir's (a /sys/class/drm/cardN/device path)
// AMD codename, preferring the live system (`lspci`'s device string for this
// GPU's PCI address, e.g. "Cezanne") over the built-in fallback table, since
// the table is necessarily incomplete.
func amdGPUCodename(deviceDir string) string {
	pci := pciAddressOf(deviceDir)
	amdCodenameMu.Lock()
	if v, ok := amdCodenameByPCI[pci]; ok {
		amdCodenameMu.Unlock()
		return v
	}
	amdCodenameMu.Unlock()

	name := ""
	if lspciName, err := readAMDCodenameFromLspci(pci); err == nil && lspciName != "" {
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
	amdCodenameByPCI[pci] = name
	amdCodenameMu.Unlock()
	return name
}

// pciAddressOf returns the PCI address of a sysfs GPU device directory (e.g.
// "0000:c4:00.0"), the stable per-GPU key. Unlike the cardN name it does not
// change when another GPU is attached. If the symlink can't be resolved, the
// directory path itself is the key.
func pciAddressOf(deviceDir string) string {
	if resolved, err := filepath.EvalSymlinks(deviceDir); err == nil {
		return filepath.Base(resolved)
	}
	return deviceDir
}

// readAMDCodenameFromLspci shells out to `lspci -D -d 1002: -mm` (machine-
// readable, quoted fields, full PCI domain) and returns the codename of the
// AMD device at pci. Called at most once per GPU via amdGPUCodename's cache.
func readAMDCodenameFromLspci(pci string) (string, error) {
	out, err := exec.Command("lspci", "-D", "-d", "1002:", "-mm").Output()
	if err != nil {
		return "", err
	}
	return parseLspciCodename(out, pci)
}

// parseLspciCodename extracts the device string's codename (e.g. "Cezanne"
// from "Cezanne [Radeon Vega Series / ...]") for the VGA/3D/display-class
// function whose slot equals pci. Other AMD functions in the output are
// skipped, so each GPU gets its own name.
func parseLspciCodename(out []byte, pci string) (string, error) {
	for _, line := range strings.Split(string(out), "\n") {
		// lspci -mm quotes each field: Slot "Class" "Vendor" "Device" ...
		fields := strings.Split(line, `"`)
		if len(fields) < 6 || strings.TrimSpace(fields[0]) != pci {
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
	return "", fmt.Errorf("no AMD VGA/3D/display device %s found in lspci output", pci)
}

func readAMDSysfsWithSeed(seed bool) ([]GPU, error) {
	return readAMDSysfsFromGlobWithSeed("/sys/class/drm/card*/device/gpu_busy_percent", seed)
}

func readAMDSysfsFromGlobWithSeed(pattern string, seed bool) ([]GPU, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no amdgpu sysfs busy-percent attributes found")
	}

	var gpus []GPU
	for _, busyPath := range matches {
		deviceDir := filepath.Dir(busyPath)
		// Histories are keyed by PCI address, not cardN, which can renumber
		// when another GPU (e.g. an eGPU) is attached.
		key := pciAddressOf(deviceDir)
		g := GPU{Name: amdGPUCodename(deviceDir), PCI: key}

		if !seed || gpuHistorySeeded(key) {
			v, err := readSysfsUint(busyPath)
			if err != nil {
				continue // not a real GPU device (or unreadable): skip rather than report zeros
			}
			g.UtilPercent = float64(v)
			g.UtilHistory = appendGPUHistory(key, g.UtilPercent)
		} else {
			// First time seeing this card: burst-sample so its timeline
			// starts already filled instead of growing one glyph per
			// redraw (see historyBurstInterval).
			hist := burstSeedGPUHistory(key, busyPath)
			if len(hist) == 0 {
				continue
			}
			g.UtilHistory = hist
			g.UtilPercent = hist[len(hist)-1]
		}

		readAMDGPUMemory(deviceDir, key, &g)

		if hwmonMatches, err := filepath.Glob(filepath.Join(deviceDir, "hwmon", "hwmon*", "temp1_input")); err == nil && len(hwmonMatches) > 0 {
			if milliC, err := readSysfsUint(hwmonMatches[0]); err == nil {
				g.TempC = float64(milliC) / 1000
				g.HaveTemp = true
			}
		}
		g.PowerW, g.PowerLabel, g.HavePower = readAMDGPUPower(deviceDir)

		g.Class = classifyGPU(isExternalGPU(deviceDir), isAPUGPU(deviceDir))
		gpus = append(gpus, g)
	}
	if len(gpus) == 0 {
		return nil, fmt.Errorf("amdgpu sysfs attributes present but unreadable")
	}
	return gpus, nil
}

// readAMDGPUPower reads the board power from the amdgpu hwmon in microwatts.
// power1_average is the smoothed value; power1_input (newer kernels and
// APUs) is the fallback. label is the same hwmon's power1_label (empty when
// absent). ok is false when neither file is readable.
func readAMDGPUPower(deviceDir string) (watts float64, label string, ok bool) {
	for _, name := range []string{"power1_average", "power1_input"} {
		matches, err := filepath.Glob(filepath.Join(deviceDir, "hwmon", "hwmon*", name))
		if err != nil || len(matches) == 0 {
			continue
		}
		if microW, err := readSysfsUint(matches[0]); err == nil {
			hwmon := filepath.Dir(matches[0])
			return float64(microW) / 1e6, readSysfsLabel(filepath.Join(hwmon, "power1_label")), true
		}
	}
	return 0, "", false
}

// readSysfsLabel returns the trimmed contents of a sysfs label file, or ""
// when it is missing or unreadable.
func readSysfsLabel(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// readAMDGPUMemory fills g's VRAM/GTT fields. key is the GPU's history key
// (its PCI address, see pciAddressOf).
func readAMDGPUMemory(deviceDir, key string, g *GPU) {
	vramUsed, errVRAMUsed := readSysfsUint(filepath.Join(deviceDir, "mem_info_vram_used"))
	vramTotal, errVRAMTotal := readSysfsUint(filepath.Join(deviceDir, "mem_info_vram_total"))
	if errVRAMUsed == nil && errVRAMTotal == nil && vramTotal > 0 {
		g.VRAMUsedMiB = float64(vramUsed) / (1024 * 1024)
		g.VRAMTotalMiB = float64(vramTotal) / (1024 * 1024)
		g.MemUsedMiB += g.VRAMUsedMiB
		g.MemTotalMiB += g.VRAMTotalMiB
		g.HaveVRAM = true
		vramPct := percent(g.VRAMUsedMiB, g.VRAMTotalMiB)
		g.VRAMPercentHistory = appendVRAMHistory(key, vramPct)
	}

	gttUsed, errGTTUsed := readSysfsUint(filepath.Join(deviceDir, "mem_info_gtt_used"))
	gttTotal, errGTTTotal := readSysfsUint(filepath.Join(deviceDir, "mem_info_gtt_total"))
	if errGTTUsed == nil && errGTTTotal == nil && gttTotal > 0 {
		g.GTTUsedMiB = float64(gttUsed) / (1024 * 1024)
		g.GTTTotalMiB = float64(gttTotal) / (1024 * 1024)
		g.MemUsedMiB += g.GTTUsedMiB
		g.MemTotalMiB += g.GTTTotalMiB
		g.HaveGTT = true
		gttPct := percent(g.GTTUsedMiB, g.GTTTotalMiB)
		g.GTTPercentHistory = appendGTTHistory(key, gttPct)
	}

	if g.MemTotalMiB > 0 {
		g.MemPercent = g.MemUsedMiB / g.MemTotalMiB * 100
		g.HaveMem = true
	}
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

// FormatRAMLabel formats the padded RAM label for the Load box, e.g. "ram (2x16G)" or "ram (45G)".
func FormatRAMLabel(mem SystemMemory) string {
	geom := mem.Geometry
	if geom == "" && mem.TotalMiB > 0 {
		geom = fallbackRAMGeometry(mem.TotalMiB)
	}
	if geom != "" {
		return padLoadLabel(fmt.Sprintf("ram (%s)", geom))
	}
	return padLoadLabel("ram")
}

func readRAMGeometry(totalMiB float64) string {
	if geom := readRAMGeometryFromEDAC(); geom != "" {
		return geom
	}
	if geom := readRAMGeometryFromDMI(); geom != "" {
		return geom
	}
	return fallbackRAMGeometry(totalMiB)
}

func readRAMGeometryFromEDAC() string {
	return readRAMGeometryFromEDACPattern("/sys/devices/system/edac/mc/mc*/dimm*/size", "/sys/devices/system/edac/mc/mc*/csrow*/size")
}

func readRAMGeometryFromEDACPattern(dimmPattern, csrowPattern string) string {
	matches, err := filepath.Glob(dimmPattern)
	if err != nil || len(matches) == 0 {
		if csrowPattern != "" {
			matches, err = filepath.Glob(csrowPattern)
		}
	}
	if err != nil || len(matches) == 0 {
		return ""
	}
	sizes := map[int]int{}
	totalCount := 0
	for _, p := range matches {
		val, err := readSysfsUint(p)
		if err != nil || val == 0 {
			continue
		}
		// EDAC reports size in Megabytes (MiB)
		gib := int(math.Round(float64(val) / 1024.0))
		if gib <= 0 {
			gib = 1
		}
		sizes[gib]++
		totalCount++
	}
	if totalCount == 0 {
		return ""
	}
	if len(sizes) == 1 {
		for gib, count := range sizes {
			if count > 1 {
				return fmt.Sprintf("%dx%dG", count, gib)
			}
			return fmt.Sprintf("%dG", gib)
		}
	}
	tot := 0
	for gib, count := range sizes {
		tot += gib * count
	}
	return fmt.Sprintf("%dG", tot)
}

func readRAMGeometryFromDMI() string {
	return readRAMGeometryFromDMIPattern("/sys/firmware/dmi/entries/17-*/raw")
}

func readRAMGeometryFromDMIPattern(pattern string) string {
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}
	sizes := map[int]int{}
	totalCount := 0
	for _, p := range matches {
		data, err := os.ReadFile(p)
		if err != nil || len(data) < 14 {
			continue
		}
		if data[0] != 17 { // Type 17: Memory Device
			continue
		}
		sizeVal := uint16(data[12]) | (uint16(data[13]) << 8)
		if sizeVal == 0 || sizeVal == 0xFFFF {
			continue
		}
		var mib int
		if sizeVal == 0x7FFF && len(data) >= 32 {
			extSize := uint32(data[28]) | (uint32(data[29]) << 8) | (uint32(data[30]) << 16) | (uint32(data[31]) << 24)
			mib = int(extSize)
		} else if sizeVal&0x8000 != 0 {
			mib = int(sizeVal&0x7FFF) / 1024
		} else {
			mib = int(sizeVal)
		}
		if mib <= 0 {
			continue
		}
		gib := int(math.Round(float64(mib) / 1024.0))
		if gib <= 0 {
			gib = 1
		}
		sizes[gib]++
		totalCount++
	}
	if totalCount == 0 {
		return ""
	}
	if len(sizes) == 1 {
		for gib, count := range sizes {
			if count > 1 {
				return fmt.Sprintf("%dx%dG", count, gib)
			}
			return fmt.Sprintf("%dG", gib)
		}
	}
	tot := 0
	for gib, count := range sizes {
		tot += gib * count
	}
	return fmt.Sprintf("%dG", tot)
}

func fallbackRAMGeometry(totalMiB float64) string {
	if totalMiB <= 0 {
		return ""
	}
	gib := int(math.Round(totalMiB / 1024.0))
	if gib <= 0 {
		return ""
	}
	return fmt.Sprintf("%dG", gib)
}
