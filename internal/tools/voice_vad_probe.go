//go:build debug

package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

// VADProbeOptions holds configuration parameters for the VAD-segmented voice dictation probe.
type VADProbeOptions struct {
	ThresholdRMS int
	SilenceMs    int
	PreRollMs    int
	MinSpeechMs  int
	TypeOutput   bool
}

// DefaultVADProbeOptions returns standard defaults for VAD segmentation.
func DefaultVADProbeOptions() VADProbeOptions {
	return VADProbeOptions{
		ThresholdRMS: 250,
		SilenceMs:    700,
		PreRollMs:    300,
		MinSpeechMs:  250,
		TypeOutput:   false,
	}
}

// NewVoiceInputVADProbeCommand returns the `harnez tools voice-input vad-probe` command.
func NewVoiceInputVADProbeCommand(d Dependencies) *cobra.Command {
	opts := DefaultVADProbeOptions()

	cmd := &cobra.Command{
		Use:   "vad-probe",
		Short: "Prototype VAD-segmented sentence-by-sentence dictation with audio pre-roll buffer",
		Long: "Listens to the microphone with a circular pre-roll buffer. When you speak, it buffers audio\n" +
			"with zero dropped initial consonants. When you pause (silence > 700ms), it transcribes that sentence\n" +
			"immediately with Whisper and outputs/types it in near real-time.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunVADProbe(cmd.Context(), d, opts)
		},
	}

	cmd.Flags().IntVar(&opts.ThresholdRMS, "threshold", opts.ThresholdRMS, "audio RMS energy threshold to trigger speech (default: 500)")
	cmd.Flags().IntVar(&opts.SilenceMs, "silence", opts.SilenceMs, "silence duration in ms to commit an utterance (default: 600)")
	cmd.Flags().IntVar(&opts.PreRollMs, "pre-roll", opts.PreRollMs, "pre-speech circular buffer duration in ms (default: 250)")
	cmd.Flags().IntVar(&opts.MinSpeechMs, "min-speech", opts.MinSpeechMs, "minimum speech duration in ms to ignore noise (default: 300)")
	cmd.Flags().BoolVar(&opts.TypeOutput, "type", opts.TypeOutput, "type transcribed sentences into the active window via dotool")

	return cmd
}

// RunVADProbe runs the interactive VAD segmentation dictation probe.
func RunVADProbe(ctx context.Context, d Dependencies, opts VADProbeOptions) error {
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

	tmpDir, err := os.MkdirTemp("", "harnez-vad-probe-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Fprintln(d.Stdout, "── VAD Sentence-by-Sentence Dictation Probe ────────────────────")
	fmt.Fprintf(d.Stdout, "  Audio Source:          %s (16kHz mono S16_LE)\n", recCmdName)
	fmt.Fprintf(d.Stdout, "  RMS Threshold:         %d\n", opts.ThresholdRMS)
	fmt.Fprintf(d.Stdout, "  Silence Cutoff:        %d ms\n", opts.SilenceMs)
	fmt.Fprintf(d.Stdout, "  Pre-roll Buffer:       %d ms (preserves initial consonants)\n", opts.PreRollMs)
	fmt.Fprintf(d.Stdout, "  Type into window:      %t\n", opts.TypeOutput)
	fmt.Fprintln(d.Stdout, "────────────────────────────────────────────────────────────────")
	fmt.Fprintln(d.Stdout, "Instructions:")
	fmt.Fprintln(d.Stdout, "  Speak sentences naturally with pauses. Each sentence will be transcribed")
	fmt.Fprintln(d.Stdout, "  and emitted right after each pause. Press Ctrl-C to finish.")
	fmt.Fprintln(d.Stdout, "")

	sigCtx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	// Launch audio capture
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
		sampleRate    = 16000
		bytesPerSec   = sampleRate * 2 // 16-bit = 2 bytes per sample
		frameMs       = 20
		frameBytes    = (sampleRate * frameMs / 1000) * 2 // 640 bytes (320 samples)
		framesPerSec  = 1000 / frameMs                    // 50 frames/sec
	)

	preRollFrames := (opts.PreRollMs + frameMs - 1) / frameMs
	preRollBuffer := make([][]byte, 0, preRollFrames)

	silenceFramesNeeded := (opts.SilenceMs + frameMs - 1) / frameMs
	minSpeechFrames := (opts.MinSpeechMs + frameMs - 1) / frameMs

	isSpeaking := false
	speechFrames := [][]byte{}
	consecutiveSilence := 0
	utteranceCount := 0

	var fullTranscript strings.Builder
	var transLock sync.Mutex

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

		frameCopy := make([]byte, frameBytes)
		copy(frameCopy, buf[:frameBytes])

		rms := computeRMS(frameCopy)

		frameIndex++
		if frameIndex%10 == 0 && !isSpeaking {
			// Print subtle live level meter every 200ms when idle
			meter := RenderAudioLevelMeter(rms, opts.ThresholdRMS)
			fmt.Fprintf(d.Stdout, "\r  Level: %s RMS: %4d   ", meter, rms)
		}

		if rms >= opts.ThresholdRMS {
			if !isSpeaking {
				// Transition to SPEECH
				isSpeaking = true
				consecutiveSilence = 0
				speechFrames = make([][]byte, 0, len(preRollBuffer)+100)
				// Prepend circular pre-roll buffer to retain starting consonant
				speechFrames = append(speechFrames, preRollBuffer...)
				speechFrames = append(speechFrames, frameCopy)
				fmt.Fprintf(d.Stdout, "\r🎙️  [Speech detected... buffering]                      \n")
			} else {
				speechFrames = append(speechFrames, frameCopy)
				consecutiveSilence = 0
			}
		} else {
			if isSpeaking {
				speechFrames = append(speechFrames, frameCopy)
				consecutiveSilence++

				if consecutiveSilence >= silenceFramesNeeded {
					// End of Utterance!
					isSpeaking = false
					consecutiveSilence = 0
					actualSpeechLen := len(speechFrames) - silenceFramesNeeded

					if actualSpeechLen >= minSpeechFrames {
						utteranceCount++
						uttNum := utteranceCount
						// Trim trailing silence frames
						trimmedFrames := speechFrames[:actualSpeechLen]
						speechAudio := flattenFrames(trimmedFrames)
						audioDuration := float64(len(speechAudio)) / float64(bytesPerSec)

						go func(num int, audioData []byte, duration float64) {
							transStart := time.Now()
							wavPath := filepath.Join(tmpDir, fmt.Sprintf("utt_%03d.wav", num))
							if err := writeWAVFile(wavPath, audioData, sampleRate); err != nil {
								fmt.Fprintf(d.Stdout, "Error saving audio: %v\n", err)
								return
							}
							defer os.Remove(wavPath)

							// Transcribe via voxtype
							cmd := exec.CommandContext(context.Background(), voxtypePath, "-q", "transcribe", wavPath)
							cmd.Env = append(os.Environ(), "NO_COLOR=1", "RUST_LOG=error")
							var outBuf bytes.Buffer
							cmd.Stdout = &outBuf
							cmd.Stderr = io.Discard
							err := cmd.Run()
							transDuration := time.Since(transStart).Seconds()

							text := CleanWhisperTranscript(outBuf.String())
							if err == nil && text != "" && IsSafeToType(text) {
								transLock.Lock()
								if fullTranscript.Len() > 0 {
									fullTranscript.WriteString(" ")
								}
								fullTranscript.WriteString(text)
								transLock.Unlock()

								fmt.Fprintf(d.Stdout, "  #%d [Audio: %.1fs, Transcribe: %.2fs] -> \x1b[32;1m%q\x1b[0m\n",
									num, duration, transDuration, text)

								if opts.TypeOutput {
									_ = TypeText(context.Background(), d, text+" ")
								}
							}
						}(uttNum, speechAudio, audioDuration)
					}
					speechFrames = nil
				}
			} else {
				// Maintain rolling pre-roll buffer
				if len(preRollBuffer) >= preRollFrames {
					preRollBuffer = preRollBuffer[1:]
				}
				preRollBuffer = append(preRollBuffer, frameCopy)
			}
		}
	}

	fmt.Fprintln(d.Stdout, "\n\n── VAD Session Complete ────────────────────────────────────────")
	fmt.Fprintf(d.Stdout, "Total Utterances Transcribed: %d\n", utteranceCount)
	fmt.Fprintf(d.Stdout, "Full Consolidated Transcript:\n\n\x1b[1m%s\x1b[0m\n", fullTranscript.String())
	fmt.Fprintln(d.Stdout, "────────────────────────────────────────────────────────────────")

	return nil
}

// computeRMS delegates to ComputeAudioRMS.
func computeRMS(frame []byte) int {
	return ComputeAudioRMS(frame)
}

func flattenFrames(frames [][]byte) []byte {
	return FlattenAudioFrames(frames)
}

func writeWAVFile(path string, pcmData []byte, sampleRate int) error {
	return WriteWAVAudio(path, pcmData, sampleRate)
}

func extractTranscribeOutput(output string) string {
	return CleanWhisperTranscript(output)
}
