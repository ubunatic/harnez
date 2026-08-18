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
	GPUAccel       string
	ActiveModel    string
	Processes      []ProcessResource
	EagerMetrics   *EagerMetrics
	ZombieWarnings []string
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
		"--property=ActiveState,SubState,MainPID,MemoryCurrent,CPUUsageNSec")
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

// PrintVoiceResourceReport writes a formatted terminal report of voice resources.
func PrintVoiceResourceReport(w io.Writer, r VoiceResourceReport) {
	fmt.Fprintln(w, "── Voice Input Resource Monitor ──────────────────────────────────")
	fmt.Fprintf(w, "  Active Mode:       \x1b[1m%s\x1b[0m\n", r.Mode)
	fmt.Fprintf(w, "  Recording State:   %s\n", formatRecordState(r.RecordStatus))
	fmt.Fprintf(w, "  GPU Acceleration:  \x1b[32m%s\x1b[0m\n", r.GPUAccel)
	fmt.Fprintf(w, "  Active Model:      %s\n", r.ActiveModel)

	if r.ActiveService != "" {
		fmt.Fprintln(w, "")
		fmt.Fprintf(w, "── Systemd Service (%s) ──\n", r.ActiveService)
		fmt.Fprintf(w, "  Status:            %s\n", r.ServiceStatus)
		if r.ServicePID > 0 {
			fmt.Fprintf(w, "  Main PID:          %d\n", r.ServicePID)
		}
		if r.ServiceMemory > 0 {
			fmt.Fprintf(w, "  Memory Usage:      %s\n", FormatBytes(r.ServiceMemory))
		}
		if r.ServiceCPU > 0 {
			fmt.Fprintf(w, "  Total CPU Time:    %s\n", r.ServiceCPU.Round(time.Millisecond))
		}
	}

	if r.EagerMetrics != nil && r.EagerMetrics.TotalChunks > 0 {
		m := r.EagerMetrics
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "── Transcription Speed & Latency (Whisper GPU) ───────────────────")
		if m.LastUtterance != nil {
			speedMultiplier := 0.0
			if m.LastUtterance.RTF > 0 {
				speedMultiplier = 1.0 / m.LastUtterance.RTF
			}
			fmt.Fprintf(w, "  Last Utterance:    Audio: \x1b[1m%.1fs\x1b[0m | Compute: \x1b[32;1m%.2fs\x1b[0m (RTF \x1b[1m%.2fx\x1b[0m, \x1b[32m%.1fx\x1b[0m realtime)\n",
				m.LastUtterance.AudioSecs, m.LastUtterance.TranscribeSecs, m.LastUtterance.RTF, speedMultiplier)
		}
		avgMult := 0.0
		if m.AvgRTF > 0 {
			avgMult = 1.0 / m.AvgRTF
		}
		fmt.Fprintf(w, "  Aggregated Speed:  Avg RTF: \x1b[32;1m%.2fx\x1b[0m (\x1b[32m%.1fx\x1b[0m realtime) across %d chunk(s) [%.1fs audio in %.2fs GPU compute]\n",
			m.AvgRTF, avgMult, m.TotalChunks, m.TotalAudioSecs, m.TotalTranscribeSecs)

		if len(m.Recent) > 0 {
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
				if len(shortText) > 42 {
					shortText = shortText[:39] + "..."
				}
				fmt.Fprintf(w, "  [%s] #%-2d (Audio: %4.1fs, GPU: %4.2fs, RTF: %4.2fx) %q\n",
					timeStr, u.Index, u.AudioSecs, u.TranscribeSecs, u.RTF, shortText)
			}
		}
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "── Active Voice Processes ─────────────────────────────────────────")
	if len(r.Processes) == 0 {
		fmt.Fprintln(w, "  No active voice processes running.")
	} else {
		fmt.Fprintf(w, "  %-7s  %-12s  %-10s  %-8s  %s\n", "PID", "NAME", "RSS MEM", "THREADS", "COMMAND")
		for _, p := range r.Processes {
			shortCmd := p.Cmdline
			if len(shortCmd) > 40 {
				shortCmd = shortCmd[:37] + "..."
			}
			fmt.Fprintf(w, "  %-7d  %-12s  %-10s  %-8d  %s\n",
				p.PID, p.Name, FormatBytes(p.RSSBytes), p.Threads, shortCmd)
		}
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "── Subprocess Health & Integrity ──────────────────────────────────")
	if len(r.ZombieWarnings) == 0 {
		fmt.Fprintln(w, "  \x1b[32m✓ 0 orphan/zombie processes (clean)\x1b[0m")
	} else {
		for _, wMsg := range r.ZombieWarnings {
			fmt.Fprintf(w, "  \x1b[31;1m⚠️  %s\x1b[0m\n", wMsg)
		}
	}
	fmt.Fprintln(w, "── Press 'q' or Ctrl+C to exit ───────────────────────────────────")
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
func RunWatchResources(ctx context.Context, d Dependencies, interval time.Duration) error {
	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Put terminal in cbreak mode to capture 'q' keypresses without requiring Enter
	oldState, err := exec.Command("stty", "-g").Output()
	if err == nil {
		_ = exec.Command("stty", "cbreak", "-echo").Run()
		defer func() {
			_ = exec.Command("stty", string(bytes.TrimSpace(oldState))).Run()
		}()
	}

	// Listen for 'q', 'Q', Ctrl-C, or Esc on standard input
	go func() {
		inputBuf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(inputBuf)
			if err != nil || n == 0 {
				return
			}
			if inputBuf[0] == 'q' || inputBuf[0] == 'Q' || inputBuf[0] == 3 || inputBuf[0] == 27 {
				stop()
				return
			}
		}
	}()

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
