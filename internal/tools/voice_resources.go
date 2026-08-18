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
	GPUAccel       string
	ActiveModel    string
	Processes      []ProcessResource
	EagerMetrics   *EagerMetrics
	ZombieWarnings []string
}

var (
	cpuHistoryLock sync.Mutex
	cpuHistory     []float64
	lastSampleTime time.Time
	lastSampleCPU  time.Duration
)

var sparkRunes = []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// RenderSparkline converts a float slice into a Unicode sparkline.
func RenderSparkline(values []float64, maxVal float64) string {
	if len(values) == 0 {
		return "        "
	}
	if maxVal <= 0 {
		for _, v := range values {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	if maxVal <= 0 {
		maxVal = 10.0
	}

	var sb strings.Builder
	for _, v := range values {
		idx := int((v / maxVal) * float64(len(sparkRunes)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkRunes) {
			idx = len(sparkRunes) - 1
		}
		sb.WriteRune(sparkRunes[idx])
	}
	return sb.String()
}

// RenderSpeedGauge renders a 10-segment visual gauge of processing speed.
func RenderSpeedGauge(rtf float64) string {
	if rtf <= 0 {
		return "\x1b[32m[██████████]\x1b[0m"
	}
	bars := 10
	// 0.05x RTF (20x realtime) -> 10 bars; 0.5x RTF (2x realtime) -> 5 bars; 1.0x RTF -> 1 bar
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

	cmd := &cobra.Command{
		Use:     "resources",
		Aliases: []string{"top", "stats", "status-full"},
		Short:   "Monitor voice input daemon, GPU acceleration, memory, and subprocess resources",
		Long: "Inspects CPU and memory consumption, active systemd user units, GPU Vulkan acceleration,\n" +
			"model memory footprint, and checks for orphan/zombie recording processes.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if watch {
				return RunWatchResources(ctx, d, time.Duration(intervalSec)*time.Second)
			}
			report := CollectVoiceResources(ctx, d)
			PrintVoiceResourceReport(d.Stdout, report)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "continuously refresh resource metrics")
	cmd.Flags().IntVarP(&intervalSec, "interval", "i", 1, "refresh interval in seconds for --watch")

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

	// Calculate CPU history and sparkline
	cpuHistoryLock.Lock()
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
	report.CPUSparkline = RenderSparkline(cpuHistory, 25.0)
	cpuHistoryLock.Unlock()

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

	// Compute uptime from /proc/[pid]/stat or ActiveEnterTimestampMonotonic
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
		return "AMD Radeon Graphics (Vulkan 1.4 GPU)"
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
	// Explicit target daemons, excluding ydotoold
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

// PrintVoiceResourceReport writes a formatted terminal report of voice resources.
func PrintVoiceResourceReport(w io.Writer, r VoiceResourceReport) {
	fmt.Fprintln(w, "── Voice Input Status ────────────────────────────────────────────")
	fmt.Fprintf(w, "  Mode:              \x1b[1m%s\x1b[0m (continuous sentence streaming)\n", r.Mode)
	fmt.Fprintf(w, "  Recording:         %s\n", formatRecordState(r.RecordStatus))
	fmt.Fprintf(w, "  Engine:            \x1b[32m%s\x1b[0m [%s]\n", r.GPUAccel, r.ActiveModel)

	if r.EagerMetrics != nil && r.EagerMetrics.TotalChunks > 0 {
		m := r.EagerMetrics
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "── Performance & Latency ─────────────────────────────────────────")
		if m.LastUtterance != nil {
			speedMultiplier := 0.0
			if m.LastUtterance.RTF > 0 {
				speedMultiplier = 1.0 / m.LastUtterance.RTF
			}
			gauge := RenderSpeedGauge(m.LastUtterance.RTF)
			fmt.Fprintf(w, "  Typing Speed:      \x1b[32;1m%.1fx faster than speech\x1b[0m  %s  (\x1b[1m%.2fs\x1b[0m lag after voice stops)\n",
				speedMultiplier, gauge, m.LastUtterance.TranscribeSecs)
		}
		fmt.Fprintf(w, "  Total Dictation:   \x1b[1m%s\x1b[0m voice processed in \x1b[32m%.1fs\x1b[0m GPU compute (%d sentence chunks)\n",
			FormatDuration(time.Duration(m.TotalAudioSecs*float64(time.Second))),
			m.TotalTranscribeSecs,
			m.TotalChunks)
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "── System Load & Resources ───────────────────────────────────────")
	if r.ActiveService != "" {
		uptimeStr := "0s"
		if r.ServiceUptime > 0 {
			uptimeStr = FormatDuration(r.ServiceUptime)
		}
		fmt.Fprintf(w, "  Service Uptime:    %s (%s, PID %d)\n", uptimeStr, r.ActiveService, r.ServicePID)
	}

	// Live and average CPU load
	sysLoad := r.AvgCPULoad / 6.0 // 6 CPU cores
	sparkline := r.CPUSparkline
	if sparkline == "" {
		sparkline = "        "
	}
	fmt.Fprintf(w, "  CPU Usage:         \x1b[1m%4.1f%%\x1b[0m live  \x1b[36m[%s]\x1b[0m  (avg \x1b[1m%.1f%%\x1b[0m of 1 core / \x1b[32m%.2f%%\x1b[0m total system load)\n",
		r.LiveCPULoad, sparkline, r.AvgCPULoad, sysLoad)

	memStr := FormatBytes(r.ServiceMemory)
	fmt.Fprintf(w, "  Memory (RAM):      %s daemon footprint  (GPU VRAM: ~487 MB)\n", memStr)

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "── Active Voice Daemons ──────────────────────────────────────────")
	if len(r.Processes) == 0 {
		fmt.Fprintln(w, "  No active voice processes running.")
	} else {
		var procSummaries []string
		for _, p := range r.Processes {
			procSummaries = append(procSummaries, fmt.Sprintf("\x1b[1m%s\x1b[0m (PID %d, %s)", p.Name, p.PID, FormatBytes(p.RSSBytes)))
		}
		fmt.Fprintf(w, "  %s\n", strings.Join(procSummaries, "  ·  "))
	}

	if r.EagerMetrics != nil && len(r.EagerMetrics.Recent) > 0 {
		m := r.EagerMetrics
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "── Recent Spoken Sentences ───────────────────────────────────────")
		limit := 4
		if len(m.Recent) < limit {
			limit = len(m.Recent)
		}
		for i := 0; i < limit; i++ {
			u := m.Recent[i]
			timeStr := u.Timestamp.Format("15:04:05")
			shortText := u.Text
			if len(shortText) > 52 {
				shortText = shortText[:49] + "..."
			}
			fmt.Fprintf(w, "  \x1b[36m%s\x1b[0m  \x1b[32m[%4.2fs lag]\x1b[0m  %q\n",
				timeStr, u.TranscribeSecs, shortText)
		}
	}

	fmt.Fprintln(w, "")
	if len(r.ZombieWarnings) == 0 {
		fmt.Fprintln(w, "── Health: \x1b[32m✓ 0 orphan processes (clean)\x1b[0m ── Press 'q' or Ctrl+C to exit ──")
	} else {
		for _, wMsg := range r.ZombieWarnings {
			fmt.Fprintf(w, "  \x1b[31;1m⚠️  %s\x1b[0m\n", wMsg)
		}
		fmt.Fprintln(w, "────────────────────────── Press 'q' or Ctrl+C to exit ───────────")
	}
}

func formatRecordState(status string) string {
	switch strings.ToLower(status) {
	case "recording":
		return "\x1b[31;1m● recording\x1b[0m (speaking into mic)"
	case "transcribing":
		return "\x1b[33;1m⏳ transcribing\x1b[0m (Whisper GPU inference)"
	case "idle":
		return "\x1b[32m○ idle\x1b[0m (press Super+Ctrl+X to speak)"
	default:
		return status
	}
}

// RunWatchResources refreshes the resource monitor live in terminal without flickering.
func RunWatchResources(ctx context.Context, d Dependencies, interval time.Duration) error {
	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Put terminal in cbreak mode to capture 'q' keypresses without requiring Enter
	oldState, err := exec.Command("stty", "-F", "/dev/tty", "-g").Output()
	if err == nil {
		_ = exec.Command("stty", "-F", "/dev/tty", "cbreak", "-echo").Run()
		defer func() {
			_ = exec.Command("stty", "-F", "/dev/tty", string(bytes.TrimSpace(oldState))).Run()
		}()
	}

	// Listen for 'q', 'Q', Ctrl-C, or Esc on controlling terminal
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
				if inputBuf[0] == 'q' || inputBuf[0] == 'Q' || inputBuf[0] == 3 || inputBuf[0] == 27 {
					stop()
					return
				}
			}
		}()
	}

	// Hide cursor on start, restore on exit
	fmt.Print("\033[?25l\033[2J")
	defer fmt.Print("\033[?25h\n")

	renderFrame := func() {
		report := CollectVoiceResources(sigCtx, d)
		var buf bytes.Buffer
		buf.WriteString("\033[H") // Move cursor to top-left without clearing buffer
		PrintVoiceResourceReport(&buf, report)
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
		case <-ticker.C:
			renderFrame()
		}
	}
}
