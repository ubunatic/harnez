package tools

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
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

// EagerOptions holds configuration parameters for Continuous Eager Sentence Streaming dictation.
type EagerOptions struct {
	ThresholdRMS  int
	SilenceMs     int
	PreRollMs     int
	MinSpeechMs   int
	MaxWindowMs   int
	TypeOutput    bool
	RecordHistory bool
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

	cmd.Flags().IntVar(&opts.ThresholdRMS, "threshold", opts.ThresholdRMS, "audio RMS energy threshold to trigger speech detection (default: 500)")
	cmd.Flags().IntVar(&opts.SilenceMs, "silence", opts.SilenceMs, "silence duration in ms to finalize an utterance chunk (default: 600)")
	cmd.Flags().IntVar(&opts.PreRollMs, "pre-roll", opts.PreRollMs, "pre-speech circular buffer duration in ms to preserve starting phonemes (default: 250)")
	cmd.Flags().IntVar(&opts.MinSpeechMs, "min-speech", opts.MinSpeechMs, "minimum speech duration in ms to ignore noise (default: 300)")
	cmd.Flags().IntVar(&opts.MaxWindowMs, "max-window", opts.MaxWindowMs, "maximum window length in ms before forcing a phrase chunk (default: 8000)")
	cmd.Flags().BoolVar(&opts.TypeOutput, "type", opts.TypeOutput, "type transcribed sentences directly into the focused window via dotool")
	cmd.Flags().BoolVar(&opts.RecordHistory, "history", opts.RecordHistory, "record transcribed utterances into local dictation history")

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

// CleanWhisperTranscript filters out diagnostic logs from Voxtype transcribe output.
func CleanWhisperTranscript(output string) string {
	lines := strings.Split(output, "\n")
	var resultLines []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" ||
			strings.HasPrefix(trimmed, "Loading audio file:") ||
			strings.HasPrefix(trimmed, "Audio format:") ||
			strings.HasPrefix(trimmed, "Processing ") ||
			strings.HasPrefix(trimmed, "whisper_") ||
			strings.Contains(trimmed, "INFO Model loaded") ||
			strings.Contains(trimmed, "INFO Using local whisper") ||
			strings.Contains(trimmed, "INFO Loading whisper model") ||
			strings.Contains(trimmed, "INFO Transcription completed") {
			continue
		}
		resultLines = append(resultLines, trimmed)
	}
	return strings.Join(resultLines, " ")
}

// RunEagerDictation orchestrates continuous audio capture, rolling phrase segmentation,
// Whisper transcription, instant text typing via dotool, and history appending.
func RunEagerDictation(ctx context.Context, d Dependencies, opts EagerOptions) error {
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

	sigCtx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	recCmd := exec.CommandContext(sigCtx, recCmdName, recArgs...)
	audioOut, err := recCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("audio pipe: %w", err)
	}
	recCmd.Stderr = os.Stderr

	if err := recCmd.Start(); err != nil {
		return fmt.Errorf("start audio capture: %w", err)
	}
	defer func() {
		if recCmd.Process != nil {
			_ = recCmd.Process.Kill()
			_ = recCmd.Wait()
		}
	}()

	const (
		sampleRate  = 16000
		bytesPerSec = sampleRate * 2
		frameMs     = 20
		frameBytes  = (sampleRate * frameMs / 1000) * 2 // 640 bytes
	)

	segmenter := NewAudioSegmenter(opts)

	type TranscribeJob struct {
		Index    int
		Audio    []byte
		Duration float64
	}

	jobChan := make(chan TranscribeJob, 16)
	var transWg sync.WaitGroup
	var fullTranscript strings.Builder
	var transLock sync.Mutex
	utteranceCount := 0

	historyPath := ""
	if opts.RecordHistory {
		if home := d.Getenv("HOME"); home != "" {
			historyPath = HistoryPath(home)
		}
	}

	// Start sequential transcription worker to ensure in-order typing and logging
	transWg.Add(1)
	go func() {
		defer transWg.Done()
		for job := range jobChan {
			transStart := time.Now()
			wavPath := filepath.Join(tmpDir, fmt.Sprintf("utt_%03d.wav", job.Index))
			if err := WriteWAVAudio(wavPath, job.Audio, sampleRate); err != nil {
				fmt.Fprintf(d.Stdout, "Error writing utterance audio: %v\n", err)
				continue
			}

			cmd := exec.CommandContext(context.Background(), voxtypePath, "transcribe", wavPath)
			outBytes, err := cmd.CombinedOutput()
			_ = os.Remove(wavPath)
			transDuration := time.Since(transStart).Seconds()

			text := CleanWhisperTranscript(string(outBytes))
			if err == nil && text != "" {
				transLock.Lock()
				if fullTranscript.Len() > 0 {
					fullTranscript.WriteString(" ")
				}
				fullTranscript.WriteString(text)
				transLock.Unlock()

				fmt.Fprintf(d.Stdout, "  #%d [Audio: %.1fs, Transcribe: %.2fs] -> \x1b[32;1m%q\x1b[0m\n",
					job.Index, job.Duration, transDuration, text)

				if opts.TypeOutput {
					_ = TypeText(context.Background(), d, text+" ")
				}

				if historyPath != "" {
					_, _ = AppendHistory(historyPath, text, DefaultHistoryLimit, time.Now())
				}
			}
		}
	}()

	buf := make([]byte, frameBytes)
	frameIndex := 0

	fmt.Fprintln(d.Stdout, "🟢 Listening for voice activity...")

	for {
		if sigCtx.Err() != nil {
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
		if speechStarted {
			fmt.Fprintf(d.Stdout, "\r🎙️  [Speaking... buffering]                                  \n")
		} else if frameIndex%10 == 0 && !isSpeaking {
			rms := ComputeAudioRMS(buf)
			meter := RenderAudioLevelMeter(rms, opts.ThresholdRMS)
			fmt.Fprintf(d.Stdout, "\r  Level: %s RMS: %4d   ", meter, rms)
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

	fmt.Fprintln(d.Stdout, "\n\n── Dictation Complete ──────────────────────────────────────────")
	fmt.Fprintf(d.Stdout, "Total Utterances Transcribed: %d\n", utteranceCount)
	fmt.Fprintf(d.Stdout, "Full Consolidated Transcript:\n\n\x1b[1m%s\x1b[0m\n", fullTranscript.String())
	fmt.Fprintln(d.Stdout, "────────────────────────────────────────────────────────────────")

	return nil
}
