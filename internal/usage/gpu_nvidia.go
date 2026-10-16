package usage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NVIDIA GPUs are read through nvidia-smi (issue 765). This is a deliberate
// exception to the kernel-standard-only policy (docs/studies/2026-08-28-
// kernel-standard-metrics-sourcing-policy.md, superseded for NVIDIA by 765).
// NVIDIA's driver exposes nothing through sysfs, so the proprietary CLI is the
// only source. Its cost is why the reader below is gated, timed out, and cached.

// nvidiaSMIArgs is the nvidia-smi query. parseNVIDIASMI expects its columns in
// this order: name, pci.bus_id, utilization.gpu, memory.used, memory.total,
// temperature.gpu. Memory is in MiB (nounits).
var nvidiaSMIArgs = []string{
	"--query-gpu=name,pci.bus_id,utilization.gpu,memory.used,memory.total,temperature.gpu",
	"--format=csv,noheader,nounits",
}

// Seams for tests. nvidiaPCIRoot is the flat PCI device list in sysfs.
var (
	nvidiaPCIRoot     = "/sys/bus/pci/devices"
	nvidiaClock       = time.Now
	nvidiaSMILookPath = exec.LookPath
	nvidiaSMIRun      = func(ctx context.Context) ([]byte, error) {
		return exec.CommandContext(ctx, "nvidia-smi", nvidiaSMIArgs...).Output()
	}
)

// nvidiaCache holds the last nvidia-smi reading for the TTL from the spec.
// Failures are cached too, so a broken nvidia-smi is not retried on every
// redraw. The mutex makes the refresh single-flight: concurrent callers wait
// for one call instead of forking several.
var (
	nvidiaMu       sync.Mutex
	nvidiaCachedAt time.Time
	nvidiaCached   []GPU
)

func resetNVIDIACache() {
	nvidiaMu.Lock()
	defer nvidiaMu.Unlock()
	nvidiaCachedAt = time.Time{}
	nvidiaCached = nil
}

// currentNVIDIAGPUs returns the NVIDIA GPUs of this host, or nil when there
// are none or nvidia-smi is unusable. It never returns an error: an NVIDIA
// card that cannot be read is left out, as the AMD path leaves out a card it
// cannot read.
func currentNVIDIAGPUs() []GPU {
	timeout, ttl, err := nvidiaSMIPolicy()
	if err != nil {
		return nil
	}
	nvidiaMu.Lock()
	defer nvidiaMu.Unlock()
	now := nvidiaClock()
	if !nvidiaCachedAt.IsZero() && now.Sub(nvidiaCachedAt) < ttl {
		return nvidiaCached
	}
	nvidiaCached = refreshNVIDIAGPUs(timeout)
	nvidiaCachedAt = now
	return nvidiaCached
}

// refreshNVIDIAGPUs runs nvidia-smi once. It gates on hardware first: no
// NVIDIA PCI function means no fork at all, and neither does a missing binary.
func refreshNVIDIAGPUs(timeout time.Duration) []GPU {
	if !nvidiaPresent() {
		return nil
	}
	if _, err := nvidiaSMILookPath("nvidia-smi"); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := nvidiaSMIRun(ctx)
	if err != nil {
		return nil
	}
	rows, err := parseNVIDIASMI(out)
	if err != nil {
		return nil
	}
	gpus := make([]GPU, 0, len(rows))
	for _, row := range rows {
		g := row.GPU
		// Histories are sampled once per refresh, not per redraw, so a
		// reading reused from the cache does not inflate the timeline.
		g.UtilHistory = appendGPUHistory(row.PCI, g.UtilPercent)
		if g.HaveVRAM {
			g.VRAMPercentHistory = appendVRAMHistory(row.PCI, percent(g.VRAMUsedMiB, g.VRAMTotalMiB))
		}
		external := isExternalGPU(filepath.Join(nvidiaPCIRoot, row.PCI))
		g.Class = classifyGPU(external, false)
		gpus = append(gpus, g)
	}
	return gpus
}

// nvidiaPresent reports whether the kernel lists an NVIDIA PCI function
// (vendor 0x10de). This is the hardware gate: nvidia-smi is never run on a
// host without NVIDIA hardware.
func nvidiaPresent() bool {
	vendors, err := filepath.Glob(filepath.Join(nvidiaPCIRoot, "*", "vendor"))
	if err != nil {
		return false
	}
	for _, v := range vendors {
		if data, err := os.ReadFile(v); err == nil && strings.TrimSpace(string(data)) == "0x10de" {
			return true
		}
	}
	return false
}

// nvidiaSMIRow is one parsed nvidia-smi line plus its PCI address, the key
// for its histories and its sysfs path.
type nvidiaSMIRow struct {
	GPU GPU
	PCI string
}

// parseNVIDIASMI parses the CSV from nvidiaSMIArgs. Lines with the wrong
// column count are skipped. A "[N/A]" or other non-numeric field is left
// absent, not zero. If no line is usable, it returns an error.
func parseNVIDIASMI(out []byte) ([]nvidiaSMIRow, error) {
	var rows []nvidiaSMIRow
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Split(line, ",")
		if len(f) != 6 {
			continue
		}
		for i := range f {
			f[i] = strings.TrimSpace(f[i])
		}
		name := nvidiaShortName(f[0])
		pci := nvidiaPCIAddress(f[1])
		if name == "" || pci == "" {
			continue
		}
		g := GPU{Name: name}
		if u, err := strconv.ParseFloat(f[2], 64); err == nil {
			g.UtilPercent = u
		}
		used, errUsed := strconv.ParseFloat(f[3], 64)
		total, errTotal := strconv.ParseFloat(f[4], 64)
		if errUsed == nil && errTotal == nil && total > 0 {
			g.VRAMUsedMiB, g.VRAMTotalMiB, g.HaveVRAM = used, total, true
			g.MemUsedMiB, g.MemTotalMiB = used, total
			g.MemPercent = percent(used, total)
			g.HaveMem = true
		}
		if t, err := strconv.ParseFloat(f[5], 64); err == nil {
			g.TempC, g.HaveTemp = t, true
		}
		rows = append(rows, nvidiaSMIRow{GPU: g, PCI: pci})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("nvidia-smi returned no usable GPU rows")
	}
	return rows, nil
}

// nvidiaShortName turns "NVIDIA GeForce RTX 3060" into "RTX 3060".
func nvidiaShortName(product string) string {
	name := strings.TrimPrefix(product, "NVIDIA GeForce ")
	name = strings.TrimPrefix(name, "NVIDIA ")
	return strings.TrimSpace(name)
}

// nvidiaPCIAddress converts nvidia-smi's "00000000:0A:00.0" into the sysfs
// form "0000:0a:00.0". It returns "" if the value is not a PCI address.
func nvidiaPCIAddress(bus string) string {
	bus = strings.ToLower(strings.TrimSpace(bus))
	if len(bus) != len("00000000:0a:00.0") {
		return ""
	}
	return bus[4:]
}
