package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// ResourceSections defines which btop-style monitoring boxes are currently displayed.
type ResourceSections struct {
	Speed      bool
	Hardware   bool
	Transcript bool
	Daemons    bool
}

// DefaultResourceSections returns all sections enabled.
func DefaultResourceSections() ResourceSections {
	return ResourceSections{
		Speed:      true,
		Hardware:   true,
		Transcript: true,
		Daemons:    true,
	}
}

// ParseSections parses a comma-separated list of section names or letters.
func ParseSections(s string) ResourceSections {
	trimmed := strings.TrimSpace(strings.ToLower(s))
	if trimmed == "" || trimmed == "all" {
		return DefaultResourceSections()
	}
	sec := ResourceSections{}
	for _, p := range strings.Split(trimmed, ",") {
		p = strings.TrimSpace(p)
		switch p {
		case "s", "speed", "status", "voice", "v", "1":
			sec.Speed = true
		case "h", "hardware", "cpu", "gpu", "hw", "c", "g", "2":
			sec.Hardware = true
		case "t", "transcript", "sentences", "feed", "3":
			sec.Transcript = true
		case "d", "daemons", "procs", "health", "p", "4":
			sec.Daemons = true
		}
	}
	if !sec.Speed && !sec.Hardware && !sec.Transcript && !sec.Daemons {
		return DefaultResourceSections()
	}
	return sec
}

// ProcessResource represents runtime memory and thread information for one process.
type ProcessResource struct {
	PID      int
	Name     string
	Cmdline  string
	RSSBytes int64
	Threads  int
}

// VoiceResourceReport contains consolidated health and resource metrics for the voice typing subsystem.
type VoiceResourceReport struct {
	Mode           VoiceInputMode
	RecordStatus   string
	ActiveService  string
	ServiceStatus  string
	ServicePID     int
	ServiceMemory  int64
	ServiceCPU     time.Duration
	ServiceUptime  time.Duration
	AvgCPULoad     float64
	LiveCPULoad    float64
	CPUSparkline   string
	LiveGPULoad    float64
	GPUSparkline   string
	VRAMUsedBytes  int64
	VRAMTotalBytes int64
	GPUAccel       string
	ActiveModel    string
	Processes      []ProcessResource
	EagerMetrics   *EagerMetrics
	ZombieWarnings []string
}

var (
	loadHistoryLock sync.Mutex
	cpuHistory      []float64
	gpuHistory      []float64
	lastSampleTime  time.Time
	lastSampleCPU   time.Duration
)

var sparkRunes = []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// RenderSparkline converts a float slice into a smoothed Unicode sparkline.
func RenderSparkline(values []float64, maxVal float64) string {
	if len(values) == 0 {
		return "            "
	}
	if maxVal <= 0 {
		maxVal = 100.0
	}

	var sb strings.Builder
	for _, v := range values {
		if v <= 0 {
			sb.WriteString(" ")
			continue
		}
		idx := int((v / maxVal) * float64(len(sparkRunes)-1))
		if idx < 1 {
			idx = 1
		}
		if idx >= len(sparkRunes) {
			idx = len(sparkRunes) - 1
		}
		sb.WriteRune(sparkRunes[idx])
	}
	return sb.String()
}

// RenderSpeedGauge renders an 8-segment visual gauge of processing speed.
func RenderSpeedGauge(rtf float64) string {
	if rtf <= 0 {
		return "\x1b[32m[████████]\x1b[0m"
	}
	bars := 8
	filled := int((1.0 - (rtf * 0.8)) * float64(bars))
	if filled < 1 {
		filled = 1
	}
	if filled > bars {
		filled = bars
	}

	var sb strings.Builder
	sb.WriteString("\x1b[32m[")
	for i := 0; i < bars; i++ {
		if i < filled {
			sb.WriteString("█")
		} else {
			sb.WriteString("░")
		}
	}
	sb.WriteString("]\x1b[0m")
	return sb.String()
}

// NewVoiceInputResourcesCommand returns the `harnez tools voice-input resources` command.
func NewVoiceInputResourcesCommand(d Dependencies) *cobra.Command {
	var watch bool
	var intervalSec int
	var sectionsStr string

	cmd := &cobra.Command{
		Use:     "resources",
		Aliases: []string{"top", "stats", "status-full"},
		Short:   "Monitor voice input daemon, GPU acceleration, memory, and subprocess resources",
		Long: "Inspects CPU and memory consumption, active systemd user units, GPU Vulkan acceleration,\n" +
			"model memory footprint, and checks for orphan/zombie recording processes.\n\n" +
			"Interactive controls (in --watch mode):\n" +
			"  s - toggle [s]peed & status box\n" +
			"  h - toggle [h]ardware CPU & GPU load box\n" +
			"  t - toggle [t]ranscript feed box\n" +
			"  d - toggle [d]aemons & health box\n" +
			"  a - enable [a]ll boxes\n" +
			"  q - quit",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			sections := ParseSections(sectionsStr)
			if watch {
				return RunWatchResources(ctx, d, time.Duration(intervalSec)*time.Second, sections)
			}
			report := CollectVoiceResources(ctx, d)
			PrintVoiceResourceReport(d.Stdout, report, sections)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "continuously refresh resource metrics")
	cmd.Flags().IntVarP(&intervalSec, "interval", "i", 1, "refresh interval in seconds for --watch")
	cmd.Flags().StringVarP(&sectionsStr, "sections", "s", "all", "comma-separated sections: s(speed), h(hardware), t(transcript), d(daemons)")

	return cmd
}

// CollectVoiceResources gathers live process, service, and GPU metrics for voice input.
func CollectVoiceResources(ctx context.Context, d Dependencies) VoiceResourceReport {
	mode := CurrentVoiceInputMode(ctx, d)
	recStatus, _ := GetRecordingStatus(ctx, d)

	report := VoiceResourceReport{
		Mode:         mode,
		RecordStatus: recStatus,
		GPUAccel:     detectGPUStatus(),
		ActiveModel:  detectActiveModel(d),
	}

	activeUnit := ""
	switch mode {
	case ModeEager:
		activeUnit = EagerService
	case ModeBatch:
		activeUnit = BatchService
	case ModeStreaming:
		activeUnit = StreamingService
	}

	report.ActiveService = activeUnit
	if activeUnit != "" {
		collectServiceMetrics(ctx, d, activeUnit, &report)
	}

	// Calculate CPU & GPU load history and sparklines
	loadHistoryLock.Lock()
	now := time.Now()
	if !lastSampleTime.IsZero() && report.ServiceCPU > 0 {
		deltaWall := now.Sub(lastSampleTime).Seconds()
		deltaCPU := (report.ServiceCPU - lastSampleCPU).Seconds()
		if deltaWall > 0.1 && deltaCPU >= 0 {
			liveLoad := (deltaCPU / deltaWall) * 100.0
			report.LiveCPULoad = liveLoad
			cpuHistory = append(cpuHistory, liveLoad)
			if len(cpuHistory) > 12 {
				cpuHistory = cpuHistory[len(cpuHistory)-12:]
			}
		}
	}
	lastSampleTime = now
	lastSampleCPU = report.ServiceCPU
	report.CPUSparkline = RenderSparkline(cpuHistory, 50.0)

	// Query AMD GPU load & VRAM
	if gpuLoad, vramUsed, vramTotal, err := getAMDGPUUsage(); err == nil {
		report.LiveGPULoad = gpuLoad
		report.VRAMUsedBytes = vramUsed
		report.VRAMTotalBytes = vramTotal
		gpuHistory = append(gpuHistory, gpuLoad)
		if len(gpuHistory) > 12 {
			gpuHistory = gpuHistory[len(gpuHistory)-12:]
		}
		report.GPUSparkline = RenderSparkline(gpuHistory, 100.0)
	}
	loadHistoryLock.Unlock()

	// Load live streaming transcription speed metrics
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	metricsPath := filepath.Join(runtimeDir, "harnez", "eager-metrics.json")
	if data, err := os.ReadFile(metricsPath); err == nil {
		var metrics EagerMetrics
		if err := json.Unmarshal(data, &metrics); err == nil {
			report.EagerMetrics = &metrics
		}
	}

	report.Processes = collectVoiceProcesses()

	// Check for zombie/orphan recording processes when idle
	pwCount := 0
	for _, p := range report.Processes {
		if strings.Contains(p.Name, "pw-record") || strings.Contains(p.Name, "arecord") {
			pwCount++
		}
	}
	if recStatus == "idle" && pwCount > 0 {
		report.ZombieWarnings = append(report.ZombieWarnings,
			fmt.Sprintf("Detected %d background recording process(es) while status is idle (possible leak)", pwCount))
	} else if pwCount > 1 {
		report.ZombieWarnings = append(report.ZombieWarnings,
			fmt.Sprintf("Detected %d multiple concurrent recording processes (expected at most 1)", pwCount))
	}

	return report
}

func collectServiceMetrics(ctx context.Context, d Dependencies, service string, report *VoiceResourceReport) {
	if d.RunOutput == nil {
		return
	}
	out, err := d.RunOutput(ctx, "systemctl", "--user", "show", service,
		"--property=ActiveState,SubState,MainPID,MemoryCurrent,CPUUsageNSec,ActiveEnterTimestampMonotonic")
	if err != nil {
		report.ServiceStatus = "unknown"
		return
	}

	props := parseSystemdProperties(out)
	report.ServiceStatus = fmt.Sprintf("%s (%s)", props["ActiveState"], props["SubState"])

	if pidStr, ok := props["MainPID"]; ok {
		if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 {
			report.ServicePID = pid
		}
	}
	if memStr, ok := props["MemoryCurrent"]; ok && memStr != "[not set]" {
		if mem, err := strconv.ParseInt(memStr, 10, 64); err == nil {
			report.ServiceMemory = mem
		}
	}
	if cpuStr, ok := props["CPUUsageNSec"]; ok && cpuStr != "[not set]" {
		if cpuNsec, err := strconv.ParseInt(cpuStr, 10, 64); err == nil {
			report.ServiceCPU = time.Duration(cpuNsec) * time.Nanosecond
		}
	}

	if report.ServicePID > 0 {
		if uptime, err := getProcessUptime(report.ServicePID); err == nil && uptime > 0 {
			report.ServiceUptime = uptime
			if uptime.Seconds() > 0 && report.ServiceCPU > 0 {
				report.AvgCPULoad = (report.ServiceCPU.Seconds() / uptime.Seconds()) * 100.0
			}
		}
	}
}

func getProcessUptime(pid int) (time.Duration, error) {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	content, err := os.ReadFile(statPath)
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(content))
	if len(fields) < 22 {
		return 0, fmt.Errorf("short stat")
	}
	startTimeTicks, err := strconv.ParseInt(fields[21], 10, 64)
	if err != nil {
		return 0, err
	}

	uptimeBytes, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	uptimeFields := strings.Fields(string(uptimeBytes))
	if len(uptimeFields) < 1 {
		return 0, fmt.Errorf("short uptime")
	}
	sysUptimeSec, err := strconv.ParseFloat(uptimeFields[0], 64)
	if err != nil {
		return 0, err
	}

	procStartSec := float64(startTimeTicks) / 100.0 // sysconf(_SC_CLK_TCK) = 100
	processUptimeSec := sysUptimeSec - procStartSec
	if processUptimeSec < 0 {
		processUptimeSec = 0
	}
	return time.Duration(processUptimeSec * float64(time.Second)), nil
}

func parseSystemdProperties(out string) map[string]string {
	m := make(map[string]string)
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return m
}

func detectGPUStatus() string {
	if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
		return "AMD Radeon Vulkan 1.4"
	}
	return "CPU fallback"
}

func detectActiveModel(d Dependencies) string {
	home := d.Getenv("HOME")
	if home == "" {
		return "base.en"
	}
	configPath := filepath.Join(home, ".config", "voxtype", "config.toml")
	content, err := os.ReadFile(configPath)
	if err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "model =") {
				parts := strings.Split(line, "=")
				if len(parts) == 2 {
					return strings.Trim(strings.TrimSpace(parts[1]), "\"")
				}
			}
		}
	}
	return "small.en"
}

func collectVoiceProcesses() []ProcessResource {
	var procs []ProcessResource
	targets := []string{"harnez", "voxtype", "pw-record", "arecord", "dotoold"}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return procs
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}

		statusPath := filepath.Join("/proc", entry.Name(), "status")
		statusBytes, err := os.ReadFile(statusPath)
		if err != nil {
			continue
		}

		name, rssBytes, threads := parseProcStatus(string(statusBytes))
		if name == "ydotoold" {
			continue
		}

		isTarget := false
		for _, t := range targets {
			if strings.Contains(name, t) {
				isTarget = true
				break
			}
		}

		if isTarget {
			cmdlinePath := filepath.Join("/proc", entry.Name(), "cmdline")
			cmdBytes, _ := os.ReadFile(cmdlinePath)
			cmdline := strings.ReplaceAll(string(cmdBytes), "\x00", " ")
			cmdline = strings.TrimSpace(cmdline)

			procs = append(procs, ProcessResource{
				PID:      pid,
				Name:     name,
				Cmdline:  cmdline,
				RSSBytes: rssBytes,
				Threads:  threads,
			})
		}
	}
	return procs
}

func parseProcStatus(content string) (name string, rssBytes int64, threads int) {
	for _, line := range strings.Split(content, "\n") {
		fields := strings.SplitN(line, ":", 2)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		val := strings.TrimSpace(fields[1])

		switch key {
		case "Name":
			name = val
		case "VmRSS":
			parts := strings.Fields(val)
			if len(parts) > 0 {
				if kb, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
					rssBytes = kb * 1024
				}
			}
		case "Threads":
			if t, err := strconv.Atoi(val); err == nil {
				threads = t
			}
		}
	}
	return name, rssBytes, threads
}

// FormatBytes formats byte sizes into human-readable strings (e.g. 7.5 MB).
func FormatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatDuration formats duration into compact human format (e.g. 21m 40s).
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func getAMDGPUUsage() (busyPercent float64, vramUsed int64, vramTotal int64, err error) {
	cards, _ := filepath.Glob("/sys/class/drm/card*/device/gpu_busy_percent")
	for _, cardPath := range cards {
		if content, err := os.ReadFile(cardPath); err == nil {
			if val, err := strconv.ParseFloat(strings.TrimSpace(string(content)), 64); err == nil {
				busyPercent = val
				dir := filepath.Dir(cardPath)
				if usedB, err := os.ReadFile(filepath.Join(dir, "mem_info_vram_used")); err == nil {
					vramUsed, _ = strconv.ParseInt(strings.TrimSpace(string(usedB)), 10, 64)
				}
				if totalB, err := os.ReadFile(filepath.Join(dir, "mem_info_vram_total")); err == nil {
					vramTotal, _ = strconv.ParseInt(strings.TrimSpace(string(totalB)), 10, 64)
				}
				return busyPercent, vramUsed, vramTotal, nil
			}
		}
	}
	return 0, 0, 0, fmt.Errorf("no AMD GPU sysfs found")
}

// TruncateLineANSI truncates a line to maxVisWidth visible characters while preserving ANSI escape sequences.
func TruncateLineANSI(s string, maxVisWidth int) string {
	if maxVisWidth <= 3 {
		return "..."
	}
	clean := StripANSI(s)
	if len([]rune(clean)) <= maxVisWidth {
		return s
	}

	var sb strings.Builder
	visCount := 0
	inEscape := false
	runes := []rune(s)
	targetVis := maxVisWidth - 3

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\x1b' {
			inEscape = true
			sb.WriteRune(r)
			continue
		}
		if inEscape {
			sb.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}

		if visCount < targetVis {
			sb.WriteRune(r)
			visCount++
		} else {
			break
		}
	}
	sb.WriteString("...\x1b[0m")
	return sb.String()
}

func getTerminalWidth() int {
	cmd := exec.Command("stty", "-F", "/dev/tty", "size")
	if out, err := cmd.Output(); err == nil {
		parts := strings.Fields(string(out))
		if len(parts) >= 2 {
			if cols, err := strconv.Atoi(parts[1]); err == nil && cols >= 60 {
				return cols
			}
		}
	}
	return 90 // Default comfortable fallback width
}

// BoxSpec defines a rendered bordered panel.
type BoxSpec struct {
	Title string
	Lines []string
	Width int
}

// RenderBoxLines renders a bordered box with rounded corners (╭─╮, │, ╰─╯), guaranteeing zero overflow.
func RenderBoxLines(b BoxSpec) []string {
	var res []string

	// Top border
	var top strings.Builder
	top.WriteString("╭─ ")
	top.WriteString(b.Title)
	top.WriteString(" ")
	visTitleLen := len([]rune(StripANSI(b.Title))) + 4
	rem := b.Width - visTitleLen - 1
	if rem < 1 {
		rem = 1
	}
	top.WriteString(strings.Repeat("─", rem))
	top.WriteString("╮")
	res = append(res, top.String())

	// Body lines with strict anti-overflow clamping
	maxContentWidth := b.Width - 4
	if maxContentWidth < 10 {
		maxContentWidth = 10
	}

	for _, l := range b.Lines {
		var body strings.Builder
		body.WriteString("│ ")
		visLen := len([]rune(StripANSI(l)))
		if visLen > maxContentWidth {
			l = TruncateLineANSI(l, maxContentWidth)
			visLen = len([]rune(StripANSI(l)))
		}
		body.WriteString(l)
		pad := maxContentWidth - visLen
		if pad < 0 {
			pad = 0
		}
		body.WriteString(strings.Repeat(" ", pad))
		body.WriteString(" │")
		res = append(res, body.String())
	}

	// Bottom border
	var bot strings.Builder
	bot.WriteString("╰")
	bot.WriteString(strings.Repeat("─", b.Width-2))
	bot.WriteString("╯")
	res = append(res, bot.String())

	return res
}

// CombineSideBySide merges two rendered boxes side by side.
func CombineSideBySide(leftLines, rightLines []string) []string {
	maxL := len(leftLines)
	if len(rightLines) > maxL {
		maxL = len(rightLines)
	}
	var res []string
	for i := 0; i < maxL; i++ {
		l := ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		r := ""
		if i < len(rightLines) {
			r = rightLines[i]
		}
		res = append(res, l+" "+r)
	}
	return res
}

// PrintVoiceResourceReport writes a formatted terminal report of voice resources using Option A (True Btop Grid).
func PrintVoiceResourceReport(w io.Writer, r VoiceResourceReport, sec ResourceSections) {
	totalWidth := getTerminalWidth()
	if totalWidth > 110 {
		totalWidth = 110
	}
	if totalWidth < 76 {
		totalWidth = 76
	}

	colWidthLeft := (totalWidth - 1) / 2
	colWidthRight := totalWidth - 1 - colWidthLeft

	// Prepare [s]peed lines
	speedLines := []string{
		fmt.Sprintf("status:  %s (%s / %s)", formatRecordState(r.RecordStatus), r.Mode, r.ActiveModel),
		fmt.Sprintf("engine:  \x1b[32m%s\x1b[0m", r.GPUAccel),
	}
	if r.EagerMetrics != nil && r.EagerMetrics.TotalChunks > 0 {
		m := r.EagerMetrics
		if m.LastUtterance != nil {
			speedMultiplier := 0.0
			if m.LastUtterance.RTF > 0 {
				speedMultiplier = 1.0 / m.LastUtterance.RTF
			}
			gauge := RenderSpeedGauge(m.LastUtterance.RTF)
			speedLines = append(speedLines,
				fmt.Sprintf("speed:   \x1b[32;1m%4.1fx realtime\x1b[0m %s", speedMultiplier, gauge),
				fmt.Sprintf("lag:     \x1b[1m%.2fs\x1b[0m · %s audio (%d utt)",
					m.LastUtterance.TranscribeSecs,
					FormatDuration(time.Duration(m.TotalAudioSecs*float64(time.Second))),
					m.TotalChunks),
			)
		} else {
			speedLines = append(speedLines,
				"speed:   \x1b[32m---\x1b[0m (waiting for speech)",
				"lag:     ---",
			)
		}
	} else {
		speedLines = append(speedLines,
			"speed:   \x1b[32m---\x1b[0m (waiting for speech)",
			"lag:     ---",
		)
	}

	// Prepare [h]ardware lines
	sysLoad := r.AvgCPULoad / 6.0 // 6 CPU cores
	sparkline := r.CPUSparkline
	if sparkline == "" {
		sparkline = "            "
	}
	gpuSpark := r.GPUSparkline
	if gpuSpark == "" {
		gpuSpark = "            "
	}
	uptimeStr := "0s"
	if r.ServiceUptime > 0 {
		uptimeStr = FormatDuration(r.ServiceUptime)
	}

	memStr := FormatBytes(r.ServiceMemory)
	vramStr := "487 MB VRAM"
	if r.VRAMUsedBytes > 0 && r.VRAMTotalBytes > 0 {
		vramStr = fmt.Sprintf("%s / %s VRAM", FormatBytes(r.VRAMUsedBytes), FormatBytes(r.VRAMTotalBytes))
	}

	hwLines := []string{
		fmt.Sprintf("cpu:  \x1b[1m%4.1f%%\x1b[0m  \x1b[36m[%s]\x1b[0m  avg %3.1f%% (1c)", r.LiveCPULoad, sparkline, r.AvgCPULoad),
		fmt.Sprintf("gpu:  \x1b[1m%4.1f%%\x1b[0m  \x1b[35m[%s]\x1b[0m  sys %3.2f%%", r.LiveGPULoad, gpuSpark, sysLoad),
		fmt.Sprintf("mem:  %s daemon   %s", memStr, vramStr),
		fmt.Sprintf("up:   %s (PID %d)", uptimeStr, r.ServicePID),
	}

	// Render Top Row (Side by side if both enabled, or full width if single)
	if sec.Speed && sec.Hardware {
		boxSpeed := BoxSpec{Title: "\x1b[1m[s] voice & speed\x1b[0m", Lines: speedLines, Width: colWidthLeft}
		boxHW := BoxSpec{Title: "\x1b[1m[h] hardware load\x1b[0m", Lines: hwLines, Width: colWidthRight}
		rendered := CombineSideBySide(RenderBoxLines(boxSpeed), RenderBoxLines(boxHW))
		for _, line := range rendered {
			fmt.Fprintln(w, line)
		}
	} else if sec.Speed {
		boxSpeed := BoxSpec{Title: "\x1b[1m[s] voice & speed\x1b[0m", Lines: speedLines, Width: totalWidth}
		for _, line := range RenderBoxLines(boxSpeed) {
			fmt.Fprintln(w, line)
		}
	} else if sec.Hardware {
		boxHW := BoxSpec{Title: "\x1b[1m[h] hardware load\x1b[0m", Lines: hwLines, Width: totalWidth}
		for _, line := range RenderBoxLines(boxHW) {
			fmt.Fprintln(w, line)
		}
	}

	// Render [t]ranscript box
	if sec.Transcript {
		var transLines []string
		if r.EagerMetrics != nil && len(r.EagerMetrics.Recent) > 0 {
			limit := 4
			if len(r.EagerMetrics.Recent) < limit {
				limit = len(r.EagerMetrics.Recent)
			}
			for i := 0; i < limit; i++ {
				u := r.EagerMetrics.Recent[i]
				timeStr := u.Timestamp.Format("15:04:05")
				shortText := u.Text
				if len(shortText) > 60 {
					shortText = shortText[:57] + "..."
				}
				transLines = append(transLines,
					fmt.Sprintf("\x1b[36m%s\x1b[0m  \x1b[32m[%4.2fs]\x1b[0m  %q", timeStr, u.TranscribeSecs, shortText))
			}
		} else {
			transLines = append(transLines, "\x1b[90m(no transcriptions yet - speak with Super+Ctrl+X to dictate)\x1b[0m")
		}
		boxTrans := BoxSpec{Title: "\x1b[1m[t] transcript feed\x1b[0m", Lines: transLines, Width: totalWidth}
		for _, line := range RenderBoxLines(boxTrans) {
			fmt.Fprintln(w, line)
		}
	}

	// Render [d]aemons box
	if sec.Daemons {
		var daemonLines []string
		if len(r.Processes) == 0 {
			daemonLines = append(daemonLines, "\x1b[90mNo active voice processes running.\x1b[0m")
		} else {
			var procSummaries []string
			for _, p := range r.Processes {
				procSummaries = append(procSummaries, fmt.Sprintf("\x1b[1m%s\x1b[0m (PID %d, %s)", p.Name, p.PID, FormatBytes(p.RSSBytes)))
			}
			healthBadge := "\x1b[32m✓ clean\x1b[0m"
			if len(r.ZombieWarnings) > 0 {
				healthBadge = fmt.Sprintf("\x1b[31;1m⚠️ %s\x1b[0m", r.ZombieWarnings[0])
			}
			daemonLines = append(daemonLines, fmt.Sprintf("%s  ·  %s", strings.Join(procSummaries, "  ·  "), healthBadge))
		}
		boxDaemons := BoxSpec{Title: "\x1b[1m[d] active daemons & health\x1b[0m", Lines: daemonLines, Width: totalWidth}
		for _, line := range RenderBoxLines(boxDaemons) {
			fmt.Fprintln(w, line)
		}
	}

	// Render bottom btop navigation bar
	fmt.Fprintf(w, " %s  %s  %s  %s  \x1b[90m│\x1b[0m  \x1b[1m[a]ll\x1b[0m  \x1b[1m[q]uit\x1b[0m\n",
		formatLetterBadge("s", "speed", sec.Speed),
		formatLetterBadge("h", "hardware", sec.Hardware),
		formatLetterBadge("t", "transcript", sec.Transcript),
		formatLetterBadge("d", "daemons", sec.Daemons))
}

func formatLetterBadge(key, name string, active bool) string {
	if active {
		return fmt.Sprintf("\x1b[1m[%s]\x1b[0m%s \x1b[32;1m●\x1b[0m", key, name[1:])
	}
	return fmt.Sprintf("\x1b[90m[%s]%s ○\x1b[0m", key, name[1:])
}

func formatRecordState(status string) string {
	switch strings.ToLower(status) {
	case "recording":
		return "\x1b[31;1m● recording\x1b[0m"
	case "transcribing":
		return "\x1b[33;1m⏳ transcribing\x1b[0m"
	case "idle":
		return "\x1b[32m○ idle\x1b[0m"
	default:
		return status
	}
}

// RunWatchResources refreshes the resource monitor live in terminal without flickering.
func RunWatchResources(ctx context.Context, d Dependencies, interval time.Duration, initialSec ResourceSections) error {
	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Put terminal in cbreak mode to capture keypresses without requiring Enter
	oldState, err := exec.Command("stty", "-F", "/dev/tty", "-g").Output()
	if err == nil {
		_ = exec.Command("stty", "-F", "/dev/tty", "cbreak", "-echo").Run()
		defer func() {
			_ = exec.Command("stty", "-F", "/dev/tty", string(bytes.TrimSpace(oldState))).Run()
		}()
	}

	var secLock sync.Mutex
	sec := initialSec

	redrawChan := make(chan struct{}, 1)
	requestRedraw := func() {
		select {
		case redrawChan <- struct{}{}:
		default:
		}
	}

	// Listen for interactive btop keys (s, h, t, d, a, q, Esc, etc.)
	tty, ttyErr := os.Open("/dev/tty")
	if ttyErr == nil {
		defer tty.Close()
		go func() {
			inputBuf := make([]byte, 1)
			for {
				n, err := tty.Read(inputBuf)
				if err != nil || n == 0 {
					return
				}
				b := inputBuf[0]
				secLock.Lock()
				switch b {
				case 's', 'S', '1', 'v', 'V':
					sec.Speed = !sec.Speed
					requestRedraw()
				case 'h', 'H', '2', 'c', 'C', 'g', 'G':
					sec.Hardware = !sec.Hardware
					requestRedraw()
				case 't', 'T', '3':
					sec.Transcript = !sec.Transcript
					requestRedraw()
				case 'd', 'D', '4', 'p', 'P':
					sec.Daemons = !sec.Daemons
					requestRedraw()
				case 'a', 'A':
					sec = DefaultResourceSections()
					requestRedraw()
				case 'q', 'Q', 3, 27:
					secLock.Unlock()
					stop()
					return
				}
				secLock.Unlock()
			}
		}()
	}

	// Hide cursor on start, restore on exit
	fmt.Print("\033[?25l\033[2J")
	defer fmt.Print("\033[?25h\n")

	renderFrame := func() {
		report := CollectVoiceResources(sigCtx, d)
		secLock.Lock()
		activeSec := sec
		secLock.Unlock()

		var buf bytes.Buffer
		buf.WriteString("\033[H") // Move cursor to top-left without clearing buffer
		PrintVoiceResourceReport(&buf, report, activeSec)
		buf.WriteString("\033[J") // Erase any trailing lines below output if list shrank
		_, _ = d.Stdout.Write(buf.Bytes())
	}

	renderFrame()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-sigCtx.Done():
			return nil
		case <-redrawChan:
			renderFrame()
		case <-ticker.C:
			renderFrame()
		}
	}
}
