package quota

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

	// PowerW is the CPU package power in watts, from RAPL (Intel, or the AMD
	// counterpart under the same powercap name). It is a delta between two
	// samples, so PowerOk is false until a second sample exists.
	// RAPLReadable is true when the package zone's energy_uj could be read
	// (root-only on many systems). It alone decides whether an APU's PPT
	// reading may stand in for CPU power (see splitAPUPackagePower).
	PowerW       float64
	PowerOk      bool
	RAPLReadable bool

	Memory SystemMemory
}

// GPU holds one GPU's utilization, VRAM usage, and temperature.
type GPU struct {
	Name string
	// PCI is the device's PCI address (its history key), used to give each
	// card a stable identity in the Power box even when cardN renumbers.
	PCI          string
	UtilPercent  float64
	MemUsedMiB   float64
	MemTotalMiB  float64
	MemPercent   float64
	VRAMUsedMiB  float64
	VRAMTotalMiB float64
	GTTUsedMiB   float64
	GTTTotalMiB  float64
	TempC        float64
	PowerW       float64
	HaveMem      bool
	HaveVRAM     bool
	HaveGTT      bool
	HaveTemp     bool
	HavePower    bool

	// PowerLabel is the hwmon power1_label the power reading came from
	// ("PPT" on an APU means the whole package, not the iGPU; see
	// splitAPUPackagePower). Empty when the label is absent.
	PowerLabel string

	// Class is the internal iGPU/dGPU/eGPU kind. It selects the memory rows
	// and is never shown (issue 765).
	Class GPUClass

	// UtilHistory is a short rolling window of recent UtilPercent samples,
	// oldest first, populated by the AMD sysfs path (cheap enough to poll
	// every redraw).
	UtilHistory []float64

	// VRAMPercentHistory and GTTPercentHistory track rolling usage % windows
	// for VRAM and GTT timelines.
	VRAMPercentHistory []float64
	GTTPercentHistory  []float64
}

// GPUClass is the internal kind of a GPU (issue 765). It only decides which
// memory rows the Load box draws; it is never printed. The zero value is
// GPUClassUnknown, so a snapshot from an older remote binary without the
// field keeps the pre-765 rendering (vram/gtt row whenever GTT is present).
type GPUClass int

// SystemMemory holds RAM usage parsed from /proc/meminfo and hardware geometry.
type SystemMemory struct {
	UsedMiB        float64
	TotalMiB       float64
	AvailableMiB   float64
	SwapUsedMiB    float64
	SwapTotalMiB   float64
	SwapKnown      bool
	Ok             bool
	Geometry       string
	PercentHistory []float64
}
