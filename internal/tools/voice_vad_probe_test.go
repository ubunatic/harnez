//go:build debug

package tools

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestComputeRMS(t *testing.T) {
	// Silence frame
	silence := make([]byte, 640)
	if rms := computeRMS(silence); rms != 0 {
		t.Errorf("computeRMS(silence) = %d, want 0", rms)
	}

	// Constant amplitude frame (e.g. 1000)
	sine := make([]byte, 640)
	for i := 0; i < len(sine); i += 2 {
		binary.LittleEndian.PutUint16(sine[i:i+2], uint16(1000))
	}
	rms := computeRMS(sine)
	if math.Abs(float64(rms-1000)) > 5 {
		t.Errorf("computeRMS(constant 1000) = %d, want ~1000", rms)
	}
}

func TestWriteWAVFile(t *testing.T) {
	tmpDir := t.TempDir()
	wavPath := filepath.Join(tmpDir, "test.wav")

	dummyPCM := make([]byte, 3200) // 100ms at 16kHz 16-bit mono
	if err := writeWAVFile(wavPath, dummyPCM, 16000); err != nil {
		t.Fatalf("writeWAVFile failed: %v", err)
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

func TestExtractTranscribeOutput(t *testing.T) {
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
}
