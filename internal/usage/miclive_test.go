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
