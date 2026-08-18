package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// EagerOptions holds configuration parameters for Continuous Eager Sentence Streaming dictation.
type EagerOptions struct {
	ThresholdRMS  int
	SilenceMs     int
	PreRollMs     int
	MinSpeechMs   int
	MaxWindowMs   int
	TypeOutput    bool
	RecordHistory bool
	Daemon        bool
}

// DefaultEagerOptions returns standard defaults for eager sentence streaming dictation.
func DefaultEagerOptions() EagerOptions {
	return EagerOptions{
		ThresholdRMS:  250,
		SilenceMs:     700,
		PreRollMs:     300,
		MinSpeechMs:   250,
		MaxWindowMs:   8000,
		TypeOutput:    true,
		RecordHistory: true,
		Daemon:        false,
	}
}

// NewVoiceInputEagerCommand returns the `harnez tools voice-input eager` command.
func NewVoiceInputEagerCommand(d Dependencies) *cobra.Command {
	opts := DefaultEagerOptions()

	cmd := &cobra.Command{
		Use:   "eager",
		Short: "Continuous eager sentence streaming dictation into focused window",
		Long: "Continuously captures audio from the microphone with a circular pre-roll buffer.\n" +
			"Segments speech on natural conversational pauses (silence > 700ms) or rolling windows,\n" +
			"transcribes completed phrases immediately with local Whisper, and types finalized sentences\n" +
			"directly into the active application via dotool with zero dropped words across pauses.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunEagerDictation(cmd.Context(), d, opts)
		},
	}

	cmd.Flags().IntVar(&opts.ThresholdRMS, "threshold", opts.ThresholdRMS, "audio RMS energy threshold to trigger speech detection (default: 250)")
	cmd.Flags().IntVar(&opts.SilenceMs, "silence", opts.SilenceMs, "silence duration in ms to finalize an utterance chunk (default: 700)")
	cmd.Flags().IntVar(&opts.PreRollMs, "pre-roll", opts.PreRollMs, "pre-speech circular buffer duration in ms to preserve starting phonemes (default: 300)")
	cmd.Flags().IntVar(&opts.MinSpeechMs, "min-speech", opts.MinSpeechMs, "minimum speech duration in ms to ignore noise (default: 250)")
	cmd.Flags().IntVar(&opts.MaxWindowMs, "max-window", opts.MaxWindowMs, "maximum window length in ms before forcing a phrase chunk (default: 8000)")
	cmd.Flags().BoolVar(&opts.TypeOutput, "type", opts.TypeOutput, "type transcribed sentences directly into the focused window via dotool")
	cmd.Flags().BoolVar(&opts.RecordHistory, "history", opts.RecordHistory, "record transcribed utterances into local dictation history")
	cmd.Flags().BoolVar(&opts.Daemon, "daemon", opts.Daemon, "run as background systemd daemon listening for toggle control")

	return cmd
}

// AudioSegmenter processes incoming PCM frames (16kHz mono S16_LE) and identifies speech segments.
type AudioSegmenter struct {
	opts                EagerOptions
	frameBytes          int
	preRollFrames       int
	silenceFramesNeeded int
	minSpeechFrames     int
	maxWindowFrames     int

	preRollBuffer      [][]byte
	speechFrames       [][]byte
	isSpeaking         bool
	consecutiveSilence int
}

// NewAudioSegmenter creates a new AudioSegmenter with the given configuration.
func NewAudioSegmenter(opts EagerOptions) *AudioSegmenter {
	const (
		sampleRate = 16000
		frameMs    = 20
		frameBytes = (sampleRate * frameMs / 1000) * 2 // 640 bytes (320 samples @ 16-bit)
	)

	preRollFrames := (opts.PreRollMs + frameMs - 1) / frameMs
	if preRollFrames < 1 {
		preRollFrames = 1
	}

	silenceFramesNeeded := (opts.SilenceMs + frameMs - 1) / frameMs
	if silenceFramesNeeded < 1 {
		silenceFramesNeeded = 1
	}

	minSpeechFrames := (opts.MinSpeechMs + frameMs - 1) / frameMs
	if minSpeechFrames < 1 {
		minSpeechFrames = 1
	}

	maxWindowFrames := (opts.MaxWindowMs + frameMs - 1) / frameMs

	return &AudioSegmenter{
		opts:                opts,
		frameBytes:          frameBytes,
		preRollFrames:       preRollFrames,
		silenceFramesNeeded: silenceFramesNeeded,
		minSpeechFrames:     minSpeechFrames,
		maxWindowFrames:     maxWindowFrames,
		preRollBuffer:       make([][]byte, 0, preRollFrames),
		speechFrames:        make([][]byte, 0, 200),
		isSpeaking:          false,
		consecutiveSilence:  0,
	}
}

// ProcessFrame ingests a 20ms frame of S16_LE PCM audio.
// Returns a non-nil speech segment slice if an utterance chunk has finished/triggered.
func (s *AudioSegmenter) ProcessFrame(frame []byte) (speechSegment []byte, speechStarted bool, isSpeaking bool) {
	if len(frame) < s.frameBytes {
		return nil, false, s.isSpeaking
	}

	frameCopy := make([]byte, s.frameBytes)
	copy(frameCopy, frame[:s.frameBytes])

	rms := ComputeAudioRMS(frameCopy)

	if rms >= s.opts.ThresholdRMS {
		if !s.isSpeaking {
			s.isSpeaking = true
			speechStarted = true
			s.consecutiveSilence = 0
			s.speechFrames = make([][]byte, 0, len(s.preRollBuffer)+100)
			// Prepend circular pre-roll buffer to retain initial consonants
			s.speechFrames = append(s.speechFrames, s.preRollBuffer...)
			s.speechFrames = append(s.speechFrames, frameCopy)
		} else {
			s.speechFrames = append(s.speechFrames, frameCopy)
			s.consecutiveSilence = 0
		}
	} else {
		if s.isSpeaking {
			s.speechFrames = append(s.speechFrames, frameCopy)
			s.consecutiveSilence++

			if s.consecutiveSilence >= s.silenceFramesNeeded {
				// Completed utterance on silence pause
				s.isSpeaking = false
				s.consecutiveSilence = 0
				actualSpeechLen := len(s.speechFrames) - s.silenceFramesNeeded

				if actualSpeechLen >= s.minSpeechFrames {
					trimmedFrames := s.speechFrames[:actualSpeechLen]
					speechSegment = FlattenAudioFrames(trimmedFrames)
				}
				s.speechFrames = nil
			}
		} else {
			// Maintain rolling pre-roll buffer
			if len(s.preRollBuffer) >= s.preRollFrames {
				s.preRollBuffer = s.preRollBuffer[1:]
			}
			s.preRollBuffer = append(s.preRollBuffer, frameCopy)
		}
	}

	// Safety check: if an utterance is running continuously longer than MaxWindowMs, force a chunk
	if s.isSpeaking && s.maxWindowFrames > 0 && len(s.speechFrames) >= s.maxWindowFrames {
		speechSegment = FlattenAudioFrames(s.speechFrames)
		// Reset speech frames but stay in speaking state with a small overlap
		overlapFrames := s.preRollFrames
		if len(s.speechFrames) < overlapFrames {
			overlapFrames = len(s.speechFrames)
		}
		overlap := make([][]byte, overlapFrames)
		copy(overlap, s.speechFrames[len(s.speechFrames)-overlapFrames:])
		s.speechFrames = overlap
	}

	return speechSegment, speechStarted, s.isSpeaking
}

// Flush forces any currently buffered speech into a segment (e.g. upon session termination).
func (s *AudioSegmenter) Flush() []byte {
	if s.isSpeaking && len(s.speechFrames) >= s.minSpeechFrames {
		res := FlattenAudioFrames(s.speechFrames)
		s.speechFrames = nil
		s.isSpeaking = false
		return res
	}
	s.speechFrames = nil
	s.isSpeaking = false
	return nil
}

// ComputeAudioRMS calculates the Root Mean Square amplitude of 16-bit PCM audio frame.
func ComputeAudioRMS(frame []byte) int {
	var sumSquares int64
	numSamples := len(frame) / 2
	if numSamples == 0 {
		return 0
	}

	for i := 0; i < len(frame); i += 2 {
		sample := int16(binary.LittleEndian.Uint16(frame[i : i+2]))
		sumSquares += int64(sample) * int64(sample)
	}

	meanSquare := sumSquares / int64(numSamples)
	return int(math.Sqrt(float64(meanSquare)))
}

// FlattenAudioFrames concatenates multiple audio frames into a contiguous byte slice.
func FlattenAudioFrames(frames [][]byte) []byte {
	totalLen := 0
	for _, f := range frames {
		totalLen += len(f)
	}
	res := make([]byte, 0, totalLen)
	for _, f := range frames {
		res = append(res, f...)
	}
	return res
}

// RenderAudioLevelMeter generates an ASCII audio level bar for console output.
func RenderAudioLevelMeter(rms int, threshold int) string {
	bars := 10
	level := (rms * bars) / (threshold * 2)
	if level > bars {
		level = bars
	}
	var sb strings.Builder
	for i := 0; i < bars; i++ {
		if i < level {
			sb.WriteString("■")
		} else {
			sb.WriteString("·")
		}
	}
	return sb.String()
}

// WriteWAVAudio writes 16kHz 16-bit mono PCM data with a standard RIFF/WAVE header.
func WriteWAVAudio(path string, pcmData []byte, sampleRate int) error {
	var buf bytes.Buffer

	// RIFF header
	buf.WriteString("RIFF")
	totalSize := uint32(36 + len(pcmData))
	_ = binary.Write(&buf, binary.LittleEndian, totalSize)
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(16)) // subchunk1 size
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))  // PCM
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))  // 1 channel (mono)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	byteRate := uint32(sampleRate * 1 * 2)
	_ = binary.Write(&buf, binary.LittleEndian, byteRate)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(2))  // block align
	_ = binary.Write(&buf, binary.LittleEndian, uint16(16)) // bits per sample

	// data chunk
	buf.WriteString("data")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(pcmData)))
	buf.Write(pcmData)

	return os.WriteFile(path, buf.Bytes(), 0600)
}

var (
	ansiEscapeRe   = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	rfc3339TimeRe  = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`)
	quoteExtractRe  = regexp.MustCompile(`Transcription completed in [^:]+:\s*"([^"]*)"`)
	hallucinationRe = regexp.MustCompile(`(?i)^\s*(thank you for watching|thanks for watching|thank you\.|thanks for listening|please subscribe|subscribe to my channel|see you next time|see you in the next video|subtitles by.*|translated by.*|like and subscribe|mcrun|mbc)\s*[.!]?\s*$`)
)

// StripANSI removes all ANSI escape sequences from s.
func StripANSI(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

// IsSafeToType validates that a text string contains genuine speech and no diagnostic log leakage or hallucination.
func IsSafeToType(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if rfc3339TimeRe.MatchString(trimmed) {
		return false
	}
	if strings.Contains(trimmed, "INFO ") || strings.Contains(trimmed, "DEBUG ") ||
		strings.Contains(trimmed, "WARN ") || strings.Contains(trimmed, "ERROR ") ||
		strings.Contains(trimmed, "whisper_") || strings.Contains(trimmed, "ggml-") {
		return false
	}
	if strings.HasPrefix(trimmed, "Loading audio file:") || strings.HasPrefix(trimmed, "Audio format:") ||
		strings.HasPrefix(trimmed, "Processing ") || strings.HasPrefix(trimmed, "Model loaded in") {
		return false
	}
	// Reject known Whisper silence hallucinations
	if hallucinationRe.MatchString(trimmed) {
		return false
	}
	return true
}

// StripTrailingHallucinations cleans trailing YouTube / silence artifact tokens.
func StripTrailingHallucinations(text string) string {
	clean := text
	patterns := []string{
		`(?i)\s*thank you for watching[.!]*`,
		`(?i)\s*thanks for watching[.!]*`,
		`(?i)\s*please subscribe[.!]*`,
		`(?i)\s*mcrun[.!]*`,
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		clean = re.ReplaceAllString(clean, "")
	}
	return strings.TrimSpace(clean)
}

// CleanWhisperTranscript extracts only valid human speech from Voxtype transcribe output,
// discarding ANSI escape codes, diagnostic logs, timestamps, model metadata, and hallucinations.
func CleanWhisperTranscript(output string) string {
	clean := StripANSI(output)

	// Strategy 1: Look for Voxtype's explicit canonical summary line:
	// 'Transcription completed in 1.25s: "the quick brown fox"'
	if m := quoteExtractRe.FindStringSubmatch(clean); len(m) > 1 {
		candidate := strings.TrimSpace(m[1])
		candidate = StripTrailingHallucinations(candidate)
		if IsSafeToType(candidate) {
			return candidate
		}
	}

	// Strategy 2: Line-by-line filtering of trailing text
	lines := strings.Split(clean, "\n")
	var resultLines []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		// Discard known log lines and metadata
		if strings.HasPrefix(trimmed, "Loading audio file:") ||
			strings.HasPrefix(trimmed, "Audio format:") ||
			strings.HasPrefix(trimmed, "Processing ") ||
			strings.HasPrefix(trimmed, "whisper_") ||
			strings.Contains(trimmed, "INFO ") ||
			strings.Contains(trimmed, "DEBUG ") ||
			strings.Contains(trimmed, "WARN ") ||
			strings.Contains(trimmed, "ERROR ") ||
			strings.Contains(trimmed, "TRACE ") ||
			strings.Contains(trimmed, "Model loaded in ") ||
			strings.Contains(trimmed, "Using local whisper") ||
			strings.Contains(trimmed, "Loading whisper model") ||
			strings.Contains(trimmed, "Transcription completed in ") ||
			rfc3339TimeRe.MatchString(trimmed) {
			continue
		}
		trimmed = StripTrailingHallucinations(trimmed)
		if IsSafeToType(trimmed) {
			resultLines = append(resultLines, trimmed)
		}
	}
	return strings.Join(resultLines, " ")
}

func writeVoxtypeState(state string) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	stateDir := filepath.Join(runtimeDir, "voxtype")
	_ = os.MkdirAll(stateDir, 0755)
	stateFile := filepath.Join(stateDir, "state")
	_ = os.WriteFile(stateFile, []byte(state+"\n"), 0644)

	// Also write to harnez state
	hDir := filepath.Join(runtimeDir, "harnez")
	_ = os.MkdirAll(hDir, 0755)
	_ = os.WriteFile(filepath.Join(hDir, "voice-state"), []byte(state+"\n"), 0644)
}

// RunEagerDictation orchestrates continuous audio capture, rolling phrase segmentation,
// Whisper transcription, instant text typing via dotool, and history appending.
func RunEagerDictation(ctx context.Context, d Dependencies, opts EagerOptions) error {
	if opts.Daemon {
		return runEagerDaemon(ctx, d, opts)
	}

	recCmdName := ""
	var recArgs []string

	if _, err := d.LookPath("pw-record"); err == nil {
		recCmdName = "pw-record"
		recArgs = []string{"--rate", "16000", "--channels", "1", "--format", "s16", "-"}
	} else if _, err := d.LookPath("arecord"); err == nil {
		recCmdName = "arecord"
		recArgs = []string{"-r", "16000", "-c", "1", "-f", "S16_LE", "-t", "raw", "-q", "-"}
	} else {
		return fmt.Errorf("neither pw-record nor arecord found on PATH")
	}

	voxtypePath, err := d.LookPath("voxtype")
	if err != nil {
		return fmt.Errorf("voxtype not found on PATH: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "harnez-voice-eager-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Fprintln(d.Stdout, "── Continuous Eager Sentence Streaming Dictation ───────────────")
	fmt.Fprintf(d.Stdout, "  Audio Source:          %s (16kHz mono S16_LE)\n", recCmdName)
	fmt.Fprintf(d.Stdout, "  RMS Threshold:         %d\n", opts.ThresholdRMS)
	fmt.Fprintf(d.Stdout, "  Silence Cutoff:        %d ms\n", opts.SilenceMs)
	fmt.Fprintf(d.Stdout, "  Pre-roll Buffer:       %d ms (preserves initial phonemes)\n", opts.PreRollMs)
	fmt.Fprintf(d.Stdout, "  Type into window:      %t\n", opts.TypeOutput)
	fmt.Fprintf(d.Stdout, "  Record history:        %t\n", opts.RecordHistory)
	fmt.Fprintln(d.Stdout, "────────────────────────────────────────────────────────────────")
	fmt.Fprintln(d.Stdout, "Instructions:")
	fmt.Fprintln(d.Stdout, "  Speak naturally with conversational pauses. Sentences will transcribe")
	fmt.Fprintln(d.Stdout, "  and type immediately upon each pause with zero dropped words.")
	fmt.Fprintln(d.Stdout, "  Press Ctrl-C (or cancel context) to stop dictation.")
	fmt.Fprintln(d.Stdout, "")

	return runEagerCaptureSession(ctx, d, opts, tmpDir, voxtypePath, recCmdName, recArgs, false)
}

func runEagerCaptureSession(ctx context.Context, d Dependencies, opts EagerOptions, tmpDir string, voxtypePath string, recCmdName string, recArgs []string, isDaemon bool) error {
	const (
		sampleRate  = 16000
		bytesPerSec = sampleRate * 2
		frameMs     = 20
		frameBytes  = (sampleRate * frameMs / 1000) * 2 // 640 bytes
	)

	segmenter := NewAudioSegmenter(opts)
	historyPath := HistoryPath(d.Getenv("HOME"))

	type TranscribeJob struct {
		Index    int
		Audio    []byte
		Duration float64
	}

	jobChan := make(chan TranscribeJob, 10)
	var transWg sync.WaitGroup
	var fullTranscript strings.Builder
	var transLock sync.Mutex
	utteranceCount := 0

	writeVoxtypeState("recording")
	defer writeVoxtypeState("idle")

	// Start sequential transcription worker
	transWg.Add(1)
	go func() {
		defer transWg.Done()
		for job := range jobChan {
			transStart := time.Now()
			wavPath := filepath.Join(tmpDir, fmt.Sprintf("utt_%03d.wav", job.Index))
			if err := WriteWAVAudio(wavPath, job.Audio, sampleRate); err != nil {
				if !isDaemon {
					fmt.Fprintf(d.Stdout, "Error writing utterance audio: %v\n", err)
				}
				continue
			}

			writeVoxtypeState("transcribing")
			cmd := exec.CommandContext(context.Background(), voxtypePath, "-q", "transcribe", wavPath)
			cmd.Env = append(os.Environ(), "NO_COLOR=1", "RUST_LOG=error")
			var outBuf bytes.Buffer
			cmd.Stdout = &outBuf
			cmd.Stderr = io.Discard
			err := cmd.Run()
			_ = os.Remove(wavPath)
			transDuration := time.Since(transStart).Seconds()
			writeVoxtypeState("recording")

			text := CleanWhisperTranscript(outBuf.String())
			if err == nil && text != "" && IsSafeToType(text) {
				transLock.Lock()
				if fullTranscript.Len() > 0 {
					fullTranscript.WriteString(" ")
				}
				fullTranscript.WriteString(text)
				transLock.Unlock()

				if !isDaemon {
					fmt.Fprintf(d.Stdout, "  #%d [Audio: %.1fs, Transcribe: %.2fs] -> \x1b[32;1m%q\x1b[0m\n",
						job.Index, job.Duration, transDuration, text)
				}

				if opts.TypeOutput {
					_ = TypeText(context.Background(), d, text+" ")
				}

				if historyPath != "" {
					_, _ = AppendHistory(historyPath, text, DefaultHistoryLimit, time.Now())
				}
			}
		}
	}()

	recCmd := exec.CommandContext(ctx, recCmdName, recArgs...)
	audioOut, err := recCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("audio pipe: %w", err)
	}
	recCmd.Stderr = io.Discard

	if err := recCmd.Start(); err != nil {
		return fmt.Errorf("start audio capture: %w", err)
	}
	defer func() {
		if recCmd.Process != nil {
			_ = recCmd.Process.Kill()
			_ = recCmd.Wait()
		}
	}()

	buf := make([]byte, frameBytes)
	frameIndex := 0

	if !isDaemon {
		fmt.Fprintln(d.Stdout, "🟢 Listening for voice activity...")
	}

	for {
		if ctx.Err() != nil {
			break
		}

		n, err := io.ReadFull(audioOut, buf)
		if err != nil {
			break
		}
		if n < frameBytes {
			continue
		}

		speechSegment, speechStarted, isSpeaking := segmenter.ProcessFrame(buf)

		frameIndex++
		if !isDaemon {
			if speechStarted {
				fmt.Fprintf(d.Stdout, "\r🎙️  [Speaking... buffering]                                  \n")
			} else if frameIndex%10 == 0 && !isSpeaking {
				rms := ComputeAudioRMS(buf)
				meter := RenderAudioLevelMeter(rms, opts.ThresholdRMS)
				fmt.Fprintf(d.Stdout, "\r  Level: %s RMS: %4d   ", meter, rms)
			}
		}

		if len(speechSegment) > 0 {
			utteranceCount++
			audioDur := float64(len(speechSegment)) / float64(bytesPerSec)
			jobChan <- TranscribeJob{
				Index:    utteranceCount,
				Audio:    speechSegment,
				Duration: audioDur,
			}
		}
	}

	// Flush remaining speech upon exit
	if finalSegment := segmenter.Flush(); len(finalSegment) > 0 {
		utteranceCount++
		audioDur := float64(len(finalSegment)) / float64(bytesPerSec)
		jobChan <- TranscribeJob{
			Index:    utteranceCount,
			Audio:    finalSegment,
			Duration: audioDur,
		}
	}

	close(jobChan)
	transWg.Wait()

	if !isDaemon {
		fmt.Fprintln(d.Stdout, "\n\n── Dictation Complete ──────────────────────────────────────────")
		fmt.Fprintf(d.Stdout, "Total Utterances Transcribed: %d\n", utteranceCount)
		fmt.Fprintf(d.Stdout, "Full Consolidated Transcript:\n\n\x1b[1m%s\x1b[0m\n", fullTranscript.String())
		fmt.Fprintln(d.Stdout, "────────────────────────────────────────────────────────────────")
	}

	return nil
}

// runEagerDaemon runs as a background service listening on a Unix control socket.
func runEagerDaemon(ctx context.Context, d Dependencies, opts EagerOptions) error {
	runtimeDir := d.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	sockDir := filepath.Join(runtimeDir, "harnez")
	_ = os.MkdirAll(sockDir, 0755)
	sockPath := filepath.Join(sockDir, "eager.sock")
	_ = os.Remove(sockPath)

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("listen unix socket %s: %w", sockPath, err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(sockPath)
	}()

	recCmdName := ""
	var recArgs []string
	if _, err := d.LookPath("pw-record"); err == nil {
		recCmdName = "pw-record"
		recArgs = []string{"--rate", "16000", "--channels", "1", "--format", "s16", "-"}
	} else if _, err := d.LookPath("arecord"); err == nil {
		recCmdName = "arecord"
		recArgs = []string{"-r", "16000", "-c", "1", "-f", "S16_LE", "-t", "raw", "-q", "-"}
	} else {
		return fmt.Errorf("audio capture tool (pw-record or arecord) missing")
	}

	voxtypePath, err := d.LookPath("voxtype")
	if err != nil {
		return fmt.Errorf("voxtype missing: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "harnez-voice-eager-daemon-*")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	writeVoxtypeState("idle")
	defer writeVoxtypeState("inactive")

	var mu sync.Mutex
	var activeCancel context.CancelFunc
	isRecording := false

	stopCurrent := func() {
		mu.Lock()
		defer mu.Unlock()
		if activeCancel != nil {
			activeCancel()
			activeCancel = nil
		}
		isRecording = false
		writeVoxtypeState("idle")
	}

	startRecording := func() {
		mu.Lock()
		defer mu.Unlock()
		if activeCancel != nil {
			activeCancel()
		}
		sessCtx, cancel := context.WithCancel(ctx)
		activeCancel = cancel
		isRecording = true
		go func() {
			_ = runEagerCaptureSession(sessCtx, d, opts, tmpDir, voxtypePath, recCmdName, recArgs, true)
			mu.Lock()
			if activeCancel != nil {
				activeCancel = nil
			}
			isRecording = false
			mu.Unlock()
			writeVoxtypeState("idle")
		}()
	}

	toggleRecording := func() string {
		mu.Lock()
		rec := isRecording
		mu.Unlock()
		if rec {
			stopCurrent()
			return "Recording stopped"
		}
		startRecording()
		return "Recording started"
	}

	// Handle USR1 (toggle) and USR2 (stop)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for sig := range sigChan {
			if sig == syscall.SIGUSR1 {
				toggleRecording()
			} else if sig == syscall.SIGUSR2 {
				stopCurrent()
			}
		}
	}()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
		stopCurrent()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			scanner := bufio.NewScanner(c)
			if scanner.Scan() {
				cmd := strings.TrimSpace(scanner.Text())
				switch cmd {
				case "toggle":
					msg := toggleRecording()
					_, _ = fmt.Fprintln(c, msg)
				case "start":
					startRecording()
					_, _ = fmt.Fprintln(c, "Recording started")
				case "stop":
					stopCurrent()
					_, _ = fmt.Fprintln(c, "Recording stopped")
				case "status":
					mu.Lock()
					rec := isRecording
					mu.Unlock()
					if rec {
						_, _ = fmt.Fprintln(c, "recording")
					} else {
						_, _ = fmt.Fprintln(c, "idle")
					}
				default:
					_, _ = fmt.Fprintln(c, "unknown command")
				}
			}
		}(conn)
	}

	return nil
}

// ControlEagerDaemon sends recording control actions to the eager daemon socket.
func ControlEagerDaemon(ctx context.Context, d Dependencies, action RecordAction) error {
	runtimeDir := d.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	sockPath := filepath.Join(runtimeDir, "harnez", "eager.sock")

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return fmt.Errorf("eager daemon not reachable at %s (is harnez-voice-eager.service active?): %w", sockPath, err)
	}
	defer conn.Close()

	_, err = fmt.Fprintf(conn, "%s\n", action)
	if err != nil {
		return fmt.Errorf("send %s to eager daemon: %w", action, err)
	}

	res, _ := bufio.NewReader(conn).ReadString('\n')
	if strings.TrimSpace(res) != "" {
		fmt.Fprintln(d.Stdout, strings.TrimSpace(res))
	}
	return nil
}

// GetEagerRecordingStatus queries current recording/idle status from the eager daemon.
func GetEagerRecordingStatus(ctx context.Context, d Dependencies) (string, error) {
	runtimeDir := d.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	sockPath := filepath.Join(runtimeDir, "harnez", "eager.sock")

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return "inactive", nil
	}
	defer conn.Close()

	_, _ = fmt.Fprintln(conn, "status")
	res, _ := bufio.NewReader(conn).ReadString('\n')
	status := strings.TrimSpace(res)
	if status == "" {
		return "idle", nil
	}
	return status, nil
}
