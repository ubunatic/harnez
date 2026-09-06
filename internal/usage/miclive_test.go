package usage

import (
	"encoding/binary"
	"math"
	"testing"
	"time"
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

func TestMicLiveApplyBallistics_InstantAttack(t *testing.T) {
	// A sudden louder peak must immediately jump without lag (fast attack).
	tau := 150 * time.Millisecond
	dt := 50 * time.Millisecond
	if got := micLiveApplyBallistics(0.0, 45.0, dt, tau); got != 45.0 {
		t.Errorf("instant attack from 0 to 45 = %v, want 45", got)
	}
	if got := micLiveApplyBallistics(30.0, 75.0, dt, tau); got != 75.0 {
		t.Errorf("instant attack from 30 to 75 = %v, want 75", got)
	}
	if got := micLiveApplyBallistics(50.0, 50.0, dt, tau); got != 50.0 {
		t.Errorf("equal levels = %v, want 50", got)
	}
}

func TestMicLiveApplyBallistics_SmoothDecay(t *testing.T) {
	// A drop to silence should decay smoothly across consecutive chunks.
	level := 50.0
	tau := 150 * time.Millisecond
	dt := 50 * time.Millisecond
	expectedDecayFactor := math.Exp(-dt.Seconds() / tau.Seconds())
	for step := 1; step <= 5; step++ {
		next := micLiveApplyBallistics(level, 0.0, dt, tau)
		expected := level * expectedDecayFactor
		if math.Abs(next-expected) > 1e-6 {
			t.Fatalf("step %d decay: got %v, want %v", step, next, expected)
		}
		if next >= level {
			t.Fatalf("step %d: level did not decrease (%v -> %v)", step, level, next)
		}
		level = next
	}
}

func TestMicLiveApplyBallistics_CutoffToZero(t *testing.T) {
	// When decayed level falls below micLiveDecayCutoff (0.5), it must snap cleanly to 0.
	tau := 150 * time.Millisecond
	dt := 50 * time.Millisecond
	// 0.6 * exp(-50/150) = 0.6 * 0.7165 = 0.4299 < 0.5 cutoff -> 0.0
	got := micLiveApplyBallistics(0.6, 0.0, dt, tau)
	if got != 0.0 {
		t.Errorf("decay below cutoff = %v, want 0.0", got)
	}
}

func TestMicLiveApplyBallistics_ClampToTargetFloor(t *testing.T) {
	// When target level is lower than prev but higher than decayed, level settles at target.
	tau := 150 * time.Millisecond
	dt := 50 * time.Millisecond
	// prev=40, decayed = 40 * exp(-50/150) = 28.66. target=35 > 28.66 -> 35.0
	if got := micLiveApplyBallistics(40.0, 35.0, dt, tau); got != 35.0 {
		t.Errorf("decay with floor: got %v, want 35.0", got)
	}
}

func TestMicLiveApplyBallistics_DisabledDecay(t *testing.T) {
	dt := 50 * time.Millisecond
	if got := micLiveApplyBallistics(50.0, 20.0, dt, 0); got != 20.0 {
		t.Errorf("decayDuration=0 got %v, want 20.0", got)
	}
	if got := micLiveApplyBallistics(50.0, 20.0, dt, -1*time.Millisecond); got != 20.0 {
		t.Errorf("decayDuration<0 got %v, want 20.0", got)
	}
}

func TestMicLiveApplyBallistics_ZeroOrNegativeDT(t *testing.T) {
	tau := 150 * time.Millisecond
	if got := micLiveApplyBallistics(50.0, 20.0, 0, tau); got != 50.0 {
		t.Errorf("dt=0 got %v, want prev 50.0", got)
	}
	if got := micLiveApplyBallistics(50.0, 20.0, -10*time.Millisecond, tau); got != 50.0 {
		t.Errorf("dt<0 got %v, want prev 50.0", got)
	}
}

func TestMicLiveMeter_Update_WindowMetricsWithoutDecay(t *testing.T) {
	var m micLiveMeter
	t0 := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	window := 100 * time.Millisecond

	// 1. Initial sample with Max metric
	r1 := m.update(40.0, true, t0, MicLiveValueMax, window, 0)
	if !r1.Available || r1.Level != 40.0 {
		t.Errorf("update speech: got %+v, want {Level: 40, Available: true}", r1)
	}
	if snap := m.snapshot(); snap != r1 {
		t.Errorf("snapshot mismatch: got %+v, want %+v", snap, r1)
	}

	// 2. Add smaller sample within window: Max should remain 40.0
	r2 := m.update(20.0, true, t0.Add(20*time.Millisecond), MicLiveValueMax, window, 0)
	if r2.Level != 40.0 {
		t.Errorf("expected max level 40.0, got %v", r2.Level)
	}

	// 3. Test Avg metric across the two samples (40 and 20 -> 30)
	rAvg := m.update(20.0, true, t0.Add(40*time.Millisecond), MicLiveValueAvg, window, 0)
	// samples in window: 40, 20, 20 -> avg = 80/3 = 26.666...
	if rAvg.Level < 26.0 || rAvg.Level > 27.0 {
		t.Errorf("expected avg ~26.66, got %v", rAvg.Level)
	}

	// 4. Test Min metric across window (min = 20)
	rMin := m.update(25.0, true, t0.Add(50*time.Millisecond), MicLiveValueMin, window, 0)
	if rMin.Level != 20.0 {
		t.Errorf("expected min level 20.0, got %v", rMin.Level)
	}

	// 5. Test Live metric (instantaneous latest: 25.0)
	rLive := m.update(25.0, true, t0.Add(60*time.Millisecond), MicLiveValueLive, window, 0)
	if rLive.Level != 25.0 {
		t.Errorf("expected live level 25.0, got %v", rLive.Level)
	}

	// 6. Advance past window: older samples expired
	rExpired := m.update(15.0, true, t0.Add(200*time.Millisecond), MicLiveValueMax, window, 0)
	if rExpired.Level != 15.0 {
		t.Errorf("expected level 15.0 after window expiry, got %v", rExpired.Level)
	}

	// 7. Update when unavailable (subprocess exited/restarting)
	rUnavail := m.update(0.0, false, t0.Add(300*time.Millisecond), MicLiveValueMax, window, 0)
	if rUnavail.Available || rUnavail.Level != 0.0 {
		t.Errorf("update unavailable: got %+v, want {Level: 0, Available: false}", rUnavail)
	}
}

func TestMicLiveMeter_Update_WithDecayBallistics(t *testing.T) {
	var m micLiveMeter
	t0 := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	window := 50 * time.Millisecond
	decay := 150 * time.Millisecond

	// 1. Initial onset: jumps to 50.0 (fast attack)
	r1 := m.update(50.0, true, t0, MicLiveValueMax, window, decay)
	if r1.Level != 50.0 || !r1.Available {
		t.Fatalf("initial onset: got %+v, want {Level: 50.0, Available: true}", r1)
	}

	// 2. 50ms later, sound drops to silence (0.0): window max is 0.0, decay begins
	// expected = 50.0 * exp(-50/150) = 50.0 * 0.7165313 = ~35.826
	t1 := t0.Add(50 * time.Millisecond)
	r2 := m.update(0.0, true, t1, MicLiveValueLive, window, decay)
	want2 := 50.0 * math.Exp(-0.05/0.15)
	if math.Abs(r2.Level-want2) > 1e-4 {
		t.Errorf("step 1 decay: got %v, want %v", r2.Level, want2)
	}

	// 3. Another 50ms of silence: decays further
	// expected = r2.Level * exp(-50/150) = 35.826 * 0.7165313 = ~25.67
	t2 := t1.Add(50 * time.Millisecond)
	r3 := m.update(0.0, true, t2, MicLiveValueLive, window, decay)
	want3 := want2 * math.Exp(-0.05/0.15)
	if math.Abs(r3.Level-want3) > 1e-4 {
		t.Errorf("step 2 decay: got %v, want %v", r3.Level, want3)
	}

	// 4. Sudden loud sound: instant attack to 70.0
	t3 := t2.Add(50 * time.Millisecond)
	r4 := m.update(70.0, true, t3, MicLiveValueLive, window, decay)
	if r4.Level != 70.0 {
		t.Errorf("instant rise: got %v, want 70.0", r4.Level)
	}

	// 5. Continued silence steps until floor cutoff snaps to zero
	cur := 70.0
	curTime := t3
	for i := 0; i < 20; i++ {
		curTime = curTime.Add(50 * time.Millisecond)
		r := m.update(0.0, true, curTime, MicLiveValueLive, window, decay)
		if r.Level == 0.0 {
			// Successfully snapped to zero
			break
		}
		if r.Level >= cur {
			t.Fatalf("expected decreasing level, got %v -> %v", cur, r.Level)
		}
		cur = r.Level
	}
	rFinal := m.snapshot()
	if rFinal.Level != 0.0 {
		t.Errorf("expected clean snap to 0.0, got %v", rFinal.Level)
	}

	// 6. Subprocess disconnect and reconnect cycle
	m.update(0.0, false, curTime.Add(50*time.Millisecond), MicLiveValueLive, window, decay)
	if snap := m.snapshot(); snap.Available || snap.Level != 0.0 {
		t.Errorf("expected unavailable state, got %+v", snap)
	}

	// Reconnect with speech onset
	rRecon := m.update(45.0, true, curTime.Add(100*time.Millisecond), MicLiveValueLive, window, decay)
	if !rRecon.Available || rRecon.Level != 45.0 {
		t.Errorf("reconnected onset: got %+v, want {Level: 45.0, Available: true}", rRecon)
	}
}

