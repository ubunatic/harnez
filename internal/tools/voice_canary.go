//go:build debug

package tools

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// CanaryOptions holds tuning parameters for the Voxtype streaming canary.
type CanaryOptions struct {
	ChunkSecs        float64
	LeftContextSecs  float64
	RightContextSecs float64
	VAD              bool
	VADThreshold     float64
	VADBackend       string
}

// DefaultCanaryOptions returns standard Parakeet streaming parameters.
func DefaultCanaryOptions() CanaryOptions {
	return CanaryOptions{
		ChunkSecs:        0.48,
		LeftContextSecs:  1.60,
		RightContextSecs: 0.48,
		VAD:              false,
		VADThreshold:     0.5,
		VADBackend:       "energy",
	}
}

// ValidateMelMultiple checks if the duration in seconds is a valid multiple of 0.08s (8 mel frames @ 100fps).
func ValidateMelMultiple(name string, val float64) error {
	frames := math.Round(val * 100.0)
	if int(frames)%8 != 0 {
		return fmt.Errorf("%s (%.2fs = %.0f mel frames) must be a multiple of 0.08s (8 mel frames @ 100fps) for Parakeet ONNX", name, val, frames)
	}
	return nil
}

// NewVoiceInputCanaryCommand returns the `harnez tools voice-input canary` command.
func NewVoiceInputCanaryCommand(d Dependencies) *cobra.Command {
	opts := DefaultCanaryOptions()

	cmd := &cobra.Command{
		Use:   "canary",
		Short: "Interactive real-time streaming canary to observe token delivery and tune parameters",
		Long: "Spawns an isolated Voxtype streaming daemon with custom parameters, captures real-time tokens\n" +
			"emitted during live speech, measures inter-word pause gaps, and detects dropped chunks.\n\n" +
			"Press Enter to start recording, speak with deliberate pauses, then press Enter (or Ctrl-C) to finish.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunStreamingCanary(cmd.Context(), d, opts)
		},
	}

	cmd.Flags().Float64Var(&opts.ChunkSecs, "chunk", opts.ChunkSecs, "streaming chunk duration in seconds (must be multiple of 0.08)")
	cmd.Flags().Float64Var(&opts.LeftContextSecs, "left-context", opts.LeftContextSecs, "streaming left context in seconds (must be multiple of 0.08)")
	cmd.Flags().Float64Var(&opts.RightContextSecs, "right-context", opts.RightContextSecs, "streaming right context in seconds (must be multiple of 0.08)")
	cmd.Flags().BoolVar(&opts.VAD, "vad", opts.VAD, "enable voice activity detection (VAD)")
	cmd.Flags().Float64Var(&opts.VADThreshold, "vad-threshold", opts.VADThreshold, "VAD speech threshold (0.0 to 1.0)")
	cmd.Flags().StringVar(&opts.VADBackend, "vad-backend", opts.VADBackend, "VAD backend (energy or whisper)")

	return cmd
}

// TokenRecord holds a single captured token event.
type TokenRecord struct {
	Elapsed float64
	Delta   float64
	Text    string
	IsKey   bool
}

// RunStreamingCanary runs the live canary loop with an isolated voxtype daemon and clean terminal I/O.
func RunStreamingCanary(ctx context.Context, d Dependencies, opts CanaryOptions) error {
	if err := ValidateMelMultiple("chunk", opts.ChunkSecs); err != nil {
		fmt.Fprintf(d.Stdout, "Warning: %v\n", err)
	}
	if err := ValidateMelMultiple("left-context", opts.LeftContextSecs); err != nil {
		fmt.Fprintf(d.Stdout, "Warning: %v\n", err)
	}
	if err := ValidateMelMultiple("right-context", opts.RightContextSecs); err != nil {
		fmt.Fprintf(d.Stdout, "Warning: %v\n", err)
	}

	voxtypePath, err := d.LookPath("voxtype")
	if err != nil {
		return fmt.Errorf("voxtype binary not found on PATH: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "harnez-voice-canary-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	fifoPath := filepath.Join(tmpDir, "dotool.pipe")
	configPath := filepath.Join(tmpDir, "config.toml")
	logFile := filepath.Join(tmpDir, "voxtype.log")
	binDir := filepath.Join(tmpDir, "bin")

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}

	// Create FIFO for DOTOOL_PIPE
	if err := syscall.Mkfifo(fifoPath, 0600); err != nil {
		return fmt.Errorf("create FIFO: %w", err)
	}

	// Create mock dotool/dotoolc executables as fallback
	mockScript := fmt.Sprintf("#!/bin/sh\ncat >> %q\n", fifoPath)
	_ = os.WriteFile(filepath.Join(binDir, "dotool"), []byte(mockScript), 0755)
	_ = os.WriteFile(filepath.Join(binDir, "dotoolc"), []byte(mockScript), 0755)

	// Generate valid config with all required sections
	configContent := fmt.Sprintf(`engine = "parakeet"
state_file = "auto"

[hotkey]
key = "SCROLLLOCK"
mode = "toggle"
enabled = false

[audio]
device = "default"
sample_rate = 16000
max_duration_secs = 120

[whisper]
model = "base.en"
language = "en"

[output]
mode = "type"
fallback_to_clipboard = false
driver_order = ["dotool"]
language_to_layout = {}
type_delay_ms = 0

[output.notification]
on_recording_start = false
on_recording_stop = false
on_transcription = false

[vad]
enabled = %t
threshold = %.2f
min_speech_duration_ms = 100
vad_backend = %q

[osd]
enabled = false

[parakeet]
model = "parakeet-unified-en-0.6b"
streaming = true
streaming_chunk_secs = %.2f
streaming_left_context_secs = %.2f
streaming_right_context_secs = %.2f
`, opts.VAD, opts.VADThreshold, opts.VADBackend, opts.ChunkSecs, opts.LeftContextSecs, opts.RightContextSecs)

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("write test config: %w", err)
	}

	fmt.Fprintln(d.Stdout, "── Voxtype Streaming Tuning Canary ─────────────────────────────")
	fmt.Fprintf(d.Stdout, "  Engine:                parakeet (parakeet-unified-en-0.6b)\n")
	fmt.Fprintf(d.Stdout, "  Chunk size:            %.2f s\n", opts.ChunkSecs)
	fmt.Fprintf(d.Stdout, "  Left context:          %.2f s\n", opts.LeftContextSecs)
	fmt.Fprintf(d.Stdout, "  Right context:         %.2f s\n", opts.RightContextSecs)
	fmt.Fprintf(d.Stdout, "  VAD enabled:           %t (threshold: %.2f, backend: %s)\n", opts.VAD, opts.VADThreshold, opts.VADBackend)
	fmt.Fprintln(d.Stdout, "────────────────────────────────────────────────────────────────")

	// Temporarily stop systemd services if active
	var restoredService string
	if d.Run(ctx, "systemctl", "--user", "is-active", "--quiet", "voxtype-streaming.service") == nil {
		fmt.Fprintln(d.Stdout, "  Temporarily stopping voxtype-streaming.service for canary isolation...")
		_ = d.Run(ctx, "systemctl", "--user", "stop", "voxtype-streaming.service")
		restoredService = "voxtype-streaming.service"
	} else if d.Run(ctx, "systemctl", "--user", "is-active", "--quiet", "voxtype.service") == nil {
		fmt.Fprintln(d.Stdout, "  Temporarily stopping voxtype.service for canary isolation...")
		_ = d.Run(ctx, "systemctl", "--user", "stop", "voxtype.service")
		restoredService = "voxtype.service"
	}

	defer func() {
		if restoredService != "" {
			fmt.Fprintf(d.Stdout, "\n  Restoring %s...\n", restoredService)
			_ = d.Run(context.Background(), "systemctl", "--user", "start", restoredService)
		}
	}()

	// Signal handling context for clean interrupt (Ctrl-C)
	sigCtx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	// Open FIFO non-blocking / read-write mode so reader doesn't block open
	fifoF, err := os.OpenFile(fifoPath, os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open FIFO: %w", err)
	}
	defer fifoF.Close()

	// Launch Voxtype daemon in background
	logF, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("create daemon log: %w", err)
	}
	defer logF.Close()

	daemonCmd := exec.CommandContext(sigCtx, voxtypePath, "-c", configPath, "daemon")
	daemonCmd.Stdout = logF
	daemonCmd.Stderr = logF
	daemonCmd.Env = append(os.Environ(),
		"DOTOOL_PIPE="+fifoPath,
		"PATH="+binDir+":"+os.Getenv("PATH"),
	)

	if err := daemonCmd.Start(); err != nil {
		return fmt.Errorf("start voxtype daemon: %w", err)
	}

	defer func() {
		if daemonCmd.Process != nil {
			_ = daemonCmd.Process.Signal(syscall.SIGTERM)
			_ = daemonCmd.Wait()
		}
	}()

	// Wait for daemon to initialize model
	ready := false
	for i := 0; i < 40; i++ {
		time.Sleep(150 * time.Millisecond)
		// Check if daemon exited prematurely
		if daemonCmd.ProcessState != nil && daemonCmd.ProcessState.Exited() {
			break
		}
		data, _ := os.ReadFile(logFile)
		logStr := string(data)
		if strings.Contains(logStr, "Model loaded") || strings.Contains(logStr, "Daemon started") || strings.Contains(logStr, "Using audio device") || strings.Contains(logStr, "Parakeet streaming model loaded") {
			ready = true
			break
		}
	}
	if !ready {
		data, _ := os.ReadFile(logFile)
		return fmt.Errorf("voxtype daemon failed to initialize:\n%s", string(data))
	}

	fmt.Fprintln(d.Stdout, "\nReady! Instructions:")
	fmt.Fprintln(d.Stdout, "  1. Press Enter to START streaming dictation.")
	fmt.Fprintln(d.Stdout, "  2. Speak your test sentence with pauses.")
	fmt.Fprintln(d.Stdout, "  3. Press Enter (or Ctrl-C) to STOP and inspect timing breakdown.")
	fmt.Fprintln(d.Stdout, "")
	fmt.Fprint(d.Stdout, "Press Enter to START: ")

	// Read trigger from stdin
	stdinScanner := bufio.NewScanner(d.Stdin)
	stdinScanner.Scan()

	// Trigger Voxtype recording start
	_ = daemonCmd.Process.Signal(syscall.SIGUSR1)
	fmt.Fprintf(d.Stdout, "\n🔴 Recording ACTIVE... Speak now!\n\n")

	startTime := time.Now()
	lastTime := startTime

	tokensChan := make(chan TokenRecord, 100)
	stopReading := make(chan struct{})
	var wg sync.WaitGroup

	// FIFO Reader goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(fifoF)
		for scanner.Scan() {
			line := scanner.Text()
			now := time.Now()
			delta := now.Sub(lastTime).Seconds()
			elapsed := now.Sub(startTime).Seconds()

			if strings.HasPrefix(line, "type ") {
				text := strings.TrimPrefix(line, "type ")
				tokensChan <- TokenRecord{
					Elapsed: elapsed,
					Delta:   delta,
					Text:    text,
					IsKey:   false,
				}
				lastTime = now
			} else if strings.HasPrefix(line, "key ") {
				keyName := strings.TrimPrefix(line, "key ")
				tokensChan <- TokenRecord{
					Elapsed: elapsed,
					Delta:   delta,
					Text:    "[" + keyName + "]",
					IsKey:   true,
				}
				lastTime = now
			}
		}
	}()

	// User input stop watcher
	stopSignal := make(chan struct{})
	go func() {
		if stdinScanner.Scan() {
			close(stopSignal)
		}
	}()

	var allTokens []TokenRecord
	var fullTextBuilder strings.Builder

	// Token printing loop
	printDone := make(chan struct{})
	go func() {
		defer close(printDone)
		for rec := range tokensChan {
			allTokens = append(allTokens, rec)
			if !rec.IsKey {
				fullTextBuilder.WriteString(rec.Text)
			}

			var pauseTag string
			if rec.Delta >= 2.0 {
				pauseTag = fmt.Sprintf("  \x1b[31;1m[LONG GAP / POSSIBLE LOSS: %.2fs]\x1b[0m", rec.Delta)
			} else if rec.Delta >= 1.0 {
				pauseTag = fmt.Sprintf("  \x1b[33;1m[PAUSE GAP: %.2fs]\x1b[0m", rec.Delta)
			}

			if rec.IsKey {
				fmt.Fprintf(d.Stdout, "+%6.2fs (Δ %4.2fs)        -> \x1b[34m%s\x1b[0m%s\n",
					rec.Elapsed, rec.Delta, rec.Text, pauseTag)
			} else {
				fmt.Fprintf(d.Stdout, "+%6.2fs (Δ %4.2fs) [%2d chars] -> \x1b[32m%q\x1b[0m%s\n",
					rec.Elapsed, rec.Delta, len(rec.Text), rec.Text, pauseTag)
			}
		}
	}()

	// Wait for user Enter or OS signal
	select {
	case <-stopSignal:
	case <-sigCtx.Done():
		fmt.Fprintln(d.Stdout, "\nInterrupted via signal.")
	}

	// Stop recording in Voxtype
	_ = daemonCmd.Process.Signal(syscall.SIGUSR2)

	// Wait for final transcription flush
	time.Sleep(500 * time.Millisecond)

	close(stopReading)
	fifoF.Close()
	wg.Wait()
	close(tokensChan)
	<-printDone

	fmt.Fprintln(d.Stdout, "\n── Dictation Complete ──────────────────────────────────────────")
	fmt.Fprintf(d.Stdout, "Total Duration:  %.2f s\n", time.Since(startTime).Seconds())
	fmt.Fprintf(d.Stdout, "Total Chunks:    %d\n", len(allTokens))
	fmt.Fprintf(d.Stdout, "Full Transcript:\n\n\x1b[1m%s\x1b[0m\n", fullTextBuilder.String())
	fmt.Fprintln(d.Stdout, "────────────────────────────────────────────────────────────────")

	return nil
}
