package usage

import (
	"encoding/binary"
	"math"
	"testing"
)

// pcm16LE encodes signed 16-bit samples as little-endian bytes, matching
// what `parec --format=s16le` streams and what micLiveAmplitudeFromPCM16LE
// consumes.
func pcm16LE(samples []int16) []byte {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	return buf
}

func TestMicLiveAmplitudeFromPCM16LESilence(t *testing.T) {
	buf := pcm16LE(make([]int16, 800))
	if got := micLiveAmplitudeFromPCM16LE(buf); got != 0 {
		t.Errorf("silence = %v, want 0", got)
	}
}

func TestMicLiveAmplitudeFromPCM16LEEmpty(t *testing.T) {
	if got := micLiveAmplitudeFromPCM16LE(nil); got != 0 {
		t.Errorf("empty = %v, want 0", got)
	}
	// An odd trailing byte (a chunk cut mid-sample) must not panic or skew
	// the result — it's simply dropped from the sample count.
	if got := micLiveAmplitudeFromPCM16LE([]byte{0x01}); got != 0 {
		t.Errorf("single odd byte = %v, want 0", got)
	}
}

func TestMicLiveAmplitudeFromPCM16LEFullScale(t *testing.T) {
	samples := make([]int16, 800)
	for i := range samples {
		if i%2 == 0 {
			samples[i] = math.MaxInt16
		} else {
			samples[i] = math.MinInt16
		}
	}
	got := micLiveAmplitudeFromPCM16LE(pcm16LE(samples))
	if got < 99 || got > 100 {
		t.Errorf("full-scale square wave = %v, want ~100", got)
	}
}

func TestMicLiveAmplitudeFromPCM16LESineWaveOrdering(t *testing.T) {
	// A quiet sine wave must read a lower amplitude than a loud one — the
	// function need not be perceptually calibrated (see its own doc
	// comment), but it must at least preserve relative loudness ordering,
	// which is all buildMicBoxLines' live bar actually needs to be useful.
	sine := func(amplitude int16) []byte {
		samples := make([]int16, 800)
		for i := range samples {
			samples[i] = int16(float64(amplitude) * math.Sin(2*math.Pi*float64(i)/40))
		}
		return pcm16LE(samples)
	}

	quiet := micLiveAmplitudeFromPCM16LE(sine(1000))
	loud := micLiveAmplitudeFromPCM16LE(sine(20000))

	if quiet <= 0 {
		t.Errorf("quiet sine amplitude = %v, want > 0", quiet)
	}
	if loud <= quiet {
		t.Errorf("loud sine amplitude (%v) should exceed quiet (%v)", loud, quiet)
	}
	if loud > 100 {
		t.Errorf("loud sine amplitude = %v, want <= 100", loud)
	}
}

func TestMicLiveAmplitudeFromPCM16LE_CalibratedVectors(t *testing.T) {
	constantBuffer := func(val int16) []byte {
		samples := make([]int16, 400)
		for i := range samples {
			samples[i] = val
		}
		return pcm16LE(samples)
	}

	tests := []struct {
		name    string
		buf     []byte
		wantMin float64
		wantMax float64
	}{
		{
			name:    "absolute silence (zeroes)",
			buf:     pcm16LE(make([]int16, 400)),
			wantMin: 0.0,
			wantMax: 0.0,
		},
		{
			name:    "below noise floor (RMS 20 < -60 dBFS)",
			buf:     constantBuffer(20),
			wantMin: 0.0,
			wantMax: 0.0,
		},
		{
			name:    "quiet conversational speech (RMS 180 ~ -45.2 dBFS)",
			buf:     constantBuffer(180),
			wantMin: 24.0,
			wantMax: 25.5,
		},
		{
			name:    "normal conversational speech (RMS 550 ~ -35.5 dBFS)",
			buf:     constantBuffer(550),
			wantMin: 40.0,
			wantMax: 42.0,
		},
		{
			name:    "loud speech peak (RMS 2700 ~ -21.7 dBFS)",
			buf:     constantBuffer(2700),
			wantMin: 63.0,
			wantMax: 65.0,
		},
		{
			name:    "full scale clipping (RMS 32767 ~ -0.0 dBFS)",
			buf:     constantBuffer(32767),
			wantMin: 99.9,
			wantMax: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := micLiveAmplitudeFromPCM16LE(tt.buf)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("micLiveAmplitudeFromPCM16LE() = %v, want in [%v, %v]", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestMicLiveManagerNilSafe documents the nil-receiver-safe contract Stop/
// Snapshot rely on (mirrored from other nil-safe helpers in this package):
// draw() in watch.go calls both unconditionally on whatever micLiveMgr
// currently holds, including a nil *micLiveManager before the Mic box is
// ever toggled on.
func TestMicLiveManagerNilSafe(t *testing.T) {
	var mgr *micLiveManager
	mgr.Stop() // must not panic
	got := mgr.Snapshot()
	if got != (micLiveReading{}) {
		t.Errorf("nil Snapshot() = %+v, want zero value", got)
	}
}
