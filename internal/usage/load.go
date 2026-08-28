package usage

import (
	"encoding/json"
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
	return load
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

// currentCPUPercents reports aggregate and per-core CPU busy% since the last
// call, computed from the delta between two /proc/stat frames (btop-style
// "real" CPU, distinct from the kernel's minute-scale load averages). The
// first call in a process has no prior sample to diff against, so it takes
// a second sample after a short sleep to still return an immediate reading;
// every later call in the same `--watch` process reuses the previous
// frame's sample instead.
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
		time.Sleep(150 * time.Millisecond)
		cur2, err := readCPUStatFrame("/proc/stat")
		if err != nil {
			return 0, false, nil, false
		}
		cpuFrameMu.Lock()
		lastCPUFrame = cur2
		cpuFrameMu.Unlock()
		return cpuFramePercents(cur, cur2)
	}
	return cpuFramePercents(prev, cur)
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
}

// CurrentGPUs probes for NVIDIA (nvidia-smi) or AMD GPUs and returns
// whatever readings are available. AMD is read directly from sysfs
// (kernel-cached counters, a plain read() per file) rather than shelling
// out to `rocm-smi`, which forks a subprocess and takes ~100ms+ per call;
// rocm-smi remains a fallback for AMD systems where sysfs lacks the
// expected attributes. Returns nil if nothing is present or everything
// fails (e.g. no GPU, or an integrated-only system).
func CurrentGPUs() []GPU {
	if gpus, err := readNvidiaSMI(); err == nil {
		return gpus
	}
	if gpus, err := readAMDSysfs(); err == nil {
		return gpus
	}
	if gpus, err := readROCmSMI(); err == nil {
		return gpus
	}
	return nil
}

// readNvidiaSMI shells out to `nvidia-smi` for CSV utilization/memory/temp readings.
func readNvidiaSMI() ([]GPU, error) {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=name,utilization.gpu,memory.used,memory.total,temperature.gpu",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return nil, err
	}

	var gpus []GPU
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 5 {
			continue
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		g := GPU{Name: fields[0]}
		if v, err := strconv.ParseFloat(fields[1], 64); err == nil {
			g.UtilPercent = v
		}
		memUsed, errUsed := strconv.ParseFloat(fields[2], 64)
		memTotal, errTotal := strconv.ParseFloat(fields[3], 64)
		if errUsed == nil && errTotal == nil && memTotal > 0 {
			g.MemUsedMiB = memUsed
			g.MemTotalMiB = memTotal
			g.MemPercent = memUsed / memTotal * 100
			g.HaveMem = true
		}
		if v, err := strconv.ParseFloat(fields[4], 64); err == nil {
			g.TempC = v
			g.HaveTemp = true
		}
		gpus = append(gpus, g)
	}
	if len(gpus) == 0 {
		return nil, fmt.Errorf("nvidia-smi returned no GPUs")
	}
	return gpus, nil
}

// readAMDSysfs reads AMD GPU utilization, VRAM, and temperature straight
// from the kernel's amdgpu sysfs attributes, avoiding a rocm-smi
// subprocess. Card enumeration (cardN) and hwmon enumeration (hwmonN) are
// both driver-assigned at runtime, so both are globbed rather than assumed.
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
		g := GPU{Name: cardName}

		if v, err := readSysfsUint(busyPath); err == nil {
			g.UtilPercent = float64(v)
		} else {
			continue // not a real GPU device (or unreadable): skip rather than report zeros
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

// readROCmSMI shells out to `rocm-smi` (AMD) for JSON utilization/VRAM%/temp readings.
func readROCmSMI() ([]GPU, error) {
	out, err := exec.Command("rocm-smi", "--showuse", "--showmemuse", "--showtemp", "--json").Output()
	if err != nil {
		return nil, err
	}

	var raw map[string]map[string]string
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}

	var gpus []GPU
	for card, fields := range raw {
		if !strings.HasPrefix(card, "card") {
			continue
		}
		g := GPU{Name: card}
		for key, val := range fields {
			switch {
			case strings.Contains(key, "GPU use"):
				if v, err := strconv.ParseFloat(val, 64); err == nil {
					g.UtilPercent = v
				}
			case strings.Contains(key, "Memory Allocated"):
				if v, err := strconv.ParseFloat(val, 64); err == nil {
					g.MemPercent = v
					g.HaveMem = true
				}
			case strings.Contains(key, "Temperature"):
				if v, err := strconv.ParseFloat(val, 64); err == nil {
					g.TempC = v
					g.HaveTemp = true
				}
			}
		}
		gpus = append(gpus, g)
	}
	if len(gpus) == 0 {
		return nil, fmt.Errorf("rocm-smi returned no GPUs")
	}
	return gpus, nil
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
