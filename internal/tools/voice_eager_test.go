package tools

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestComputeAudioRMS(t *testing.T) {
	// Silence frame
	silence := make([]byte, 640)
	if rms := ComputeAudioRMS(silence); rms != 0 {
		t.Errorf("ComputeAudioRMS(silence) = %d, want 0", rms)
	}

	// Constant amplitude frame (e.g. 1000)
	sine := make([]byte, 640)
	for i := 0; i < len(sine); i += 2 {
		binary.LittleEndian.PutUint16(sine[i:i+2], uint16(1000))
	}
	rms := ComputeAudioRMS(sine)
	if math.Abs(float64(rms-1000)) > 5 {
		t.Errorf("ComputeAudioRMS(constant 1000) = %d, want ~1000", rms)
	}
}

func TestWriteWAVAudio(t *testing.T) {
	tmpDir := t.TempDir()
	wavPath := filepath.Join(tmpDir, "test_audio.wav")

	dummyPCM := make([]byte, 3200) // 100ms at 16kHz 16-bit mono
	if err := WriteWAVAudio(wavPath, dummyPCM, 16000); err != nil {
		t.Fatalf("WriteWAVAudio failed: %v", err)
	}

	info, err := os.Stat(wavPath)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	// 44 bytes header + 3200 bytes data = 3244 bytes
	if info.Size() != 3244 {
		t.Errorf("WAV file size = %d, want 3244", info.Size())
	}
}

func TestStripANSI(t *testing.T) {
	colored := "\x1b[2m2026-08-18T06:45:43.303459Z\x1b[0m \x1b[32mINFO\x1b[0m Model loaded"
	clean := StripANSI(colored)
	expected := "2026-08-18T06:45:43.303459Z INFO Model loaded"
	if clean != expected {
		t.Errorf("StripANSI = %q, want %q", clean, expected)
	}
}

func TestIsSafeToType(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"valid clean speech", "the quick brown fox jumps over the yellow cat", true},
		{"valid short phrase", "One, two, three", true},
		{"empty string", "", false},
		{"whitespace only", "   \t\n", false},
		{"timestamp log line", "2026-08-18T06:45:43.303459Z Model loaded", false},
		{"contains INFO log", "INFO Using local whisper", false},
		{"contains DEBUG log", "DEBUG Processing audio chunk", false},
		{"contains whisper_ prefix", "whisper_init_state: compute buffer", false},
		{"contains audio format prefix", "Audio format: 16000 Hz", false},
		{"contains ggml model path", "Loading /home/uwe/ggml-base.en.bin", false},
		{"hallucination thank you for watching", "Thank you for watching.", false},
		{"hallucination please subscribe", "Please subscribe", false},
		{"hallucination mcrun", "mcrun", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSafeToType(tt.text); got != tt.want {
				t.Errorf("IsSafeToType(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestStripTrailingHallucinations(t *testing.T) {
	input := "Let's see if all the sentences are completed at MCRUN. Thank you for watching."
	cleaned := StripTrailingHallucinations(input)
	expected := "Let's see if all the sentences are completed at"
	if cleaned != expected {
		t.Errorf("StripTrailingHallucinations = %q, want %q", cleaned, expected)
	}
}

func TestCleanWhisperTranscript(t *testing.T) {
	t.Run("with standard diagnostic logs", func(t *testing.T) {
		rawOutput := `Loading audio file: "/tmp/foo.wav"
Audio format: 16000 Hz, 1 channel(s), Int
Processing 32000 samples (2.00s)...
2026-08-18T00:10:40.508521Z  INFO Using local whisper transcription mode
2026-08-18T00:10:40.508558Z  INFO Loading whisper model from "/path"
2026-08-18T00:10:40.665277Z  INFO Model loaded in 0.16s
2026-08-18T00:10:41.952335Z  INFO Transcription completed in 0.29s: "The quick brown fox"
The quick brown fox
`
		text := CleanWhisperTranscript(rawOutput)
		if text != "The quick brown fox" {
			t.Errorf("CleanWhisperTranscript = %q, want %q", text, "The quick brown fox")
		}
	})

	t.Run("with trailing hallucination", func(t *testing.T) {
		rawOutput := `Transcription completed in 0.89s: "Hello world. Thank you for watching."
Hello world. Thank you for watching.
`
		text := CleanWhisperTranscript(rawOutput)
		if text != "Hello world." {
			t.Errorf("CleanWhisperTranscript(trailing hallucination) = %q, want %q", text, "Hello world.")
		}
	})

	t.Run("with ANSI escape sequences and mixed logs", func(t *testing.T) {
		dirtyOutput := "\x1b[2m2026-08-18T06:45:43.303459Z\x1b[0m \x1b[32mINFO\x1b[0m Using local whisper transcription mode\n" +
			"\x1b[2m2026-08-18T06:45:43.303492Z\x1b[0m \x1b[32mINFO\x1b[0m Loading whisper model from \"/home/uwe/.local/share/voxtype/models/ggml-base.en.bin\"\n" +
			"\x1b[2m2026-08-18T06:45:43.420324Z\x1b[0m \x1b[32mINFO\x1b[0m Model loaded in 0.12s\n" +
			"\x1b[2m2026-08-18T06:45:44.673038Z\x1b[0m \x1b[32mINFO\x1b[0m Transcription completed in 1.25s: \"the quick brown fox jumps over the yellow\"\n" +
			"the quick brown fox jumps over the yellow\n"

		text := CleanWhisperTranscript(dirtyOutput)
		if text != "the quick brown fox jumps over the yellow" {
			t.Errorf("CleanWhisperTranscript(dirtyOutput) = %q, want %q", text, "the quick brown fox jumps over the yellow")
		}
	})

	t.Run("empty transcript with only logs", func(t *testing.T) {
		onlyLogs := `Loading audio file: "/tmp/empty.wav"
Audio format: 16000 Hz, 1 channel(s), Int
Processing 16000 samples (1.00s)...
2026-08-18T06:48:03.493611Z  INFO Using local whisper transcription mode
2026-08-18T06:48:05.183630Z  INFO Transcription completed in 1.54s: ""
`
		text := CleanWhisperTranscript(onlyLogs)
		if text != "" {
			t.Errorf("CleanWhisperTranscript(onlyLogs) = %q, want empty string", text)
		}
	})
}

func TestAudioSegmenter(t *testing.T) {
	opts := EagerOptions{
		ThresholdRMS:  500,
		SilenceMs:     60,  // 3 frames @ 20ms
		PreRollMs:     40,  // 2 frames @ 20ms
		MinSpeechMs:   40,  // 2 frames @ 20ms
		MaxWindowMs:   200, // 10 frames @ 20ms
		TypeOutput:    false,
		RecordHistory: false,
	}

	segmenter := NewAudioSegmenter(opts)

	silenceFrame := make([]byte, 640)
	speechFrame := make([]byte, 640)
	for i := 0; i < len(speechFrame); i += 2 {
		binary.LittleEndian.PutUint16(speechFrame[i:i+2], uint16(1000))
	}

	// 1. Send 5 frames of silence -> no segment, not speaking
	for i := 0; i < 5; i++ {
		seg, started, speaking := segmenter.ProcessFrame(silenceFrame)
		if len(seg) > 0 || started || speaking {
			t.Fatalf("expected idle silence, got seg=%d, started=%v, speaking=%v", len(seg), started, speaking)
		}
	}

	// 2. Send 4 frames of speech -> started on frame 1, speaking = true, no segment yet
	for i := 0; i < 4; i++ {
		seg, started, speaking := segmenter.ProcessFrame(speechFrame)
		if len(seg) > 0 {
			t.Fatalf("unexpected segment during speech")
		}
		if i == 0 && !started {
			t.Fatalf("expected speechStarted on first speech frame")
		}
		if !speaking {
			t.Fatalf("expected speaking = true during speech frames")
		}
	}

	// 3. Send 3 frames of silence (SilenceMs = 60ms = 3 frames)
	// Frame 1 and 2 of silence: speaking = true, no segment
	for i := 0; i < 2; i++ {
		seg, started, speaking := segmenter.ProcessFrame(silenceFrame)
		if len(seg) > 0 || started || !speaking {
			t.Fatalf("expected buffering during initial pause frames, got seg=%d, started=%v, speaking=%v", len(seg), started, speaking)
		}
	}

	// Frame 3 of silence: triggers segment finalization!
	seg, started, speaking := segmenter.ProcessFrame(silenceFrame)
	if started || speaking {
		t.Fatalf("expected speaking to finish, got started=%v, speaking=%v", started, speaking)
	}
	if len(seg) == 0 {
		t.Fatalf("expected non-empty speech segment on pause threshold")
	}

	// Length should include pre-roll (2 frames = 1280 bytes) + 4 speech frames (2560 bytes) = 3840 bytes
	expectedBytes := (2 + 4) * 640
	if len(seg) != expectedBytes {
		t.Errorf("segment length = %d bytes, want %d bytes (pre-roll + speech)", len(seg), expectedBytes)
	}
}

func TestAudioSegmenterFlush(t *testing.T) {
	opts := EagerOptions{
		ThresholdRMS: 500,
		SilenceMs:    200,
		PreRollMs:    40,
		MinSpeechMs:  40,
		MaxWindowMs:  1000,
	}
	segmenter := NewAudioSegmenter(opts)

	speechFrame := make([]byte, 640)
	for i := 0; i < len(speechFrame); i += 2 {
		binary.LittleEndian.PutUint16(speechFrame[i:i+2], uint16(1000))
	}

	// Ingest 3 speech frames
	for i := 0; i < 3; i++ {
		segmenter.ProcessFrame(speechFrame)
	}

	// Flush mid-utterance
	flushed := segmenter.Flush()
	if len(flushed) == 0 {
		t.Fatalf("expected non-empty flushed segment")
	}

	// After flush, segmenter is idle
	empty := segmenter.Flush()
	if len(empty) != 0 {
		t.Fatalf("expected empty second flush")
	}
}
