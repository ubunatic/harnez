package usage

import (
	"context"
	"encoding/binary"
	"io"
	"math"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Issue 245: a genuine live peak/RMS meter of the real signal hitting the
// default microphone right now, shown alongside MicStatus.Level's configured
// gain (issue 244). PipeWire/PulseAudio only expose true peak/RMS through a
// subscribed streaming API, not a one-shot poll (see mic.go's MicStatus.Level
// doc comment and issue 244's Resolution §3), so this holds a low-rate raw
// PCM capture open on the default source for as long as the Mic box is
// visible and computes a rolling RMS amplitude from it in memory — it never
// writes captured audio anywhere.
//
// Canary-probed by hand on a PipeWire 17.0 dev machine (2026-09-05, see
// issue 245's Resolution section for the full transcript): both `parec
// --raw --format=s16le --rate=8000 --channels=1 -d @DEFAULT_SOURCE@` and the
// `pw-cat -r --target <source> --format s16 --rate 8000 --channels 1 -`
// equivalent produced live, changing amplitude data and terminated cleanly
// on SIGTERM. `parec` was chosen: it already matches this file's pactl-only
// backend gate (issue 244 only builds the streaming-capable Recording field
// on the pactl backend, never amixer) and needs no numeric sink/source id
// resolution — `@DEFAULT_SOURCE@` tracks source changes on its own, the same
// symbolic name mic.go's pactl calls already use.
const (
	// micLiveSampleRate is deliberately low: this box only needs a coarse
	// amplitude reading, not audio fidelity, and a low rate keeps the
	// subprocess's CPU/bandwidth footprint negligible for a status box that
	// may sit open for a whole watch session.
	micLiveSampleRate = 8000
	// micLiveChunkBytes is ~50ms of mono s16le audio at micLiveSampleRate
	// (8000 samples/sec * 2 bytes/sample * 0.05s = 800 bytes) — frequent
	// enough (20 Hz) to provide smooth, real-time VU meter response during
	// speech while keeping RMS computation trivial (issue 258).
	micLiveChunkBytes = 800
	// micLiveRetryInterval paces reconnect attempts after the capture
	// subprocess exits early (source unplugged, PipeWire restarted) or
	// never starts (permission denied) — mirrors remoteLoadRetryInterval's
	// role for the analogous remote-load streaming manager.
	micLiveRetryInterval = 5 * time.Second
	// micLiveMinDBFS is the calibrated noise-floor cutoff for the live input
	// meter (issue 257). Digital full-scale is 0 dBFS (RMS 32768). Conversational
	// speech recorded with standard ADC gain typically sits at -46 to -35 dBFS
	// (RMS ~150-600). Mapping [-60 dBFS, 0 dBFS] to [0, 100] places conversational
	// voice in the 25%-45% range on the visual TUI bar while suppressing ambient
	// room silence (< -60 dBFS, RMS < 33).
	micLiveMinDBFS = -60.0
	// micLiveDecayCutoff is the floor (0.5%) below which decayed level snaps cleanly
	// to zero (issue 258, 260).
	micLiveDecayCutoff = 0.5
	// micGracePeriod is the duration high-frequency redraws continue after
	// sound/speech returns to silence (issue 258) to allow smooth ballistics
	// decay release before idling back to standard 1s cadence.
	micGracePeriod = 800 * time.Millisecond
)

// micLiveApplyBallistics updates previous level with the target level:
// instant attack on rises, smooth exponential release on falls (issue 258, 260).
// When target level >= prev level, it snaps instantly to target level.
// When target level < prev level, it decays exponentially over elapsed dt with time constant tau:
//   decayed = prev * exp(-dt / tau)
// Clamped to not drop below target level, and snaps to 0.0 below micLiveDecayCutoff (0.5%).
// If decayDuration <= 0, it falls back to instant update (no decay).
func micLiveApplyBallistics(prev, target float64, dt, decayDuration time.Duration) float64 {
	if decayDuration <= 0 {
		return target
	}
	if target >= prev {
		return target
	}
	if dt <= 0 {
		return prev
	}
	tau := decayDuration.Seconds()
	decayed := prev * math.Exp(-dt.Seconds()/tau)
	if decayed < target {
		decayed = target
	}
	if decayed < micLiveDecayCutoff {
		return 0.0
	}
	return decayed
}

// micLiveReading is one published sample from the background capture
// goroutine: Level is 0-100 like MicStatus.Level, and Available reports
// whether a real reading is currently flowing (false while (re)connecting,
// permanently false when capture can't work on this machine at all).
type micLiveReading struct {
	Level     float64
	Available bool
}

type micSample struct {
	ts    time.Time
	level float64
}

// micLiveMeter is the mutex-guarded handoff point between the background
// capture goroutine (the only writer) and the watch redraw loop (the only
// reader) — same shape as runRemoteLoadManager's lastRemoteLoad/remoteLoadMu
// pair, just scoped to this one value instead of a whole LoadSnapshot.
type micLiveMeter struct {
	mu             sync.Mutex
	reading        micLiveReading
	samples        []micSample
	lastUpdate     time.Time
	displayedLevel float64
}

func (m *micLiveMeter) set(r micLiveReading) {
	m.mu.Lock()
	m.reading = r
	m.mu.Unlock()
}

func (m *micLiveMeter) update(rawLevel float64, available bool, now time.Time, metric MicLiveValueMetric, window time.Duration, decay time.Duration) micLiveReading {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !available {
		m.samples = nil
		m.lastUpdate = time.Time{}
		m.displayedLevel = 0
		m.reading = micLiveReading{Available: false, Level: 0}
		return m.reading
	}

	m.samples = append(m.samples, micSample{ts: now, level: rawLevel})

	cutoff := now.Add(-window)
	start := 0
	for start < len(m.samples) && m.samples[start].ts.Before(cutoff) {
		start++
	}
	if start > 0 {
		m.samples = m.samples[start:]
	}

	var targetLevel float64
	if len(m.samples) == 0 {
		targetLevel = rawLevel
	} else {
		switch metric {
		case MicLiveValueLive:
			targetLevel = rawLevel
		case MicLiveValueMin:
			targetLevel = m.samples[0].level
			for _, s := range m.samples[1:] {
				if s.level < targetLevel {
					targetLevel = s.level
				}
			}
		case MicLiveValueAvg:
			var sum float64
			for _, s := range m.samples {
				sum += s.level
			}
			targetLevel = sum / float64(len(m.samples))
		case MicLiveValueMax:
			fallthrough
		default:
			targetLevel = m.samples[0].level
			for _, s := range m.samples[1:] {
				if s.level > targetLevel {
					targetLevel = s.level
				}
			}
		}
	}

	var dt time.Duration
	if !m.lastUpdate.IsZero() {
		dt = now.Sub(m.lastUpdate)
	}
	m.displayedLevel = micLiveApplyBallistics(m.displayedLevel, targetLevel, dt, decay)
	m.lastUpdate = now

	m.reading = micLiveReading{Level: m.displayedLevel, Available: true}
	return m.reading
}

func (m *micLiveMeter) snapshot() micLiveReading {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reading
}

// micLiveManager owns the lifecycle of one background capture goroutine.
// Start it when the Mic box becomes visible, Stop it when the box is
// toggled off or the watch loop exits — never leave one running unobserved
// (Zero Zombie Guarantee, docs/practices/AgenticLoop.md).
type micLiveManager struct {
	meter    micLiveMeter
	cancel   context.CancelFunc
	onSample func(micLiveReading)
}

// startMicLiveManager begins capturing, deriving its own lifetime from
// parent (the watch loop's sigCtx) so a Ctrl-C/SIGTERM that ends the whole
// --watch process also tears this down even if Stop is never called
// explicitly. If onSample is non-nil, it is invoked on every newly processed
// audio chunk. Returns nil immediately, without spawning anything, when live
// capture structurally can't work here: no pactl backend (issue 244 already
// requires pactl's streaming-capable Recording field for this scope; the
// plain-ALSA amixer fallback has no monitor-stream equivalent, matching
// buildMicBoxLines' existing "n/a" treatment of amixer's Recording field),
// or no `parec` binary installed. This is the "degrade gracefully" path
// (issue 245 requirement 3): the caller renders LiveAvailable: false, never
// an error.
func startMicLiveManager(parent context.Context, onSample func(micLiveReading)) *micLiveManager {
	ctx, cancel := context.WithCancel(parent)
	mgr := &micLiveManager{cancel: cancel, onSample: onSample}
	if resolveMicBackend() != micBackendPactl {
		return mgr
	}
	if _, err := exec.LookPath("parec"); err != nil {
		return mgr
	}
	go runMicLiveManager(ctx, &mgr.meter, mgr.onSample)
	return mgr
}

// Stop tears down the capture subprocess (if any) and its reader goroutine.
// Safe to call on a nil *micLiveManager (mirrors the nil-receiver-safe
// pattern used elsewhere in this package) and safe to call more than once.
func (m *micLiveManager) Stop() {
	if m == nil {
		return
	}
	m.cancel()
}

// Snapshot returns the most recently published reading. Safe on nil.
func (m *micLiveManager) Snapshot() micLiveReading {
	if m == nil {
		return micLiveReading{}
	}
	return m.meter.snapshot()
}

// runMicLiveManager holds capture open for the life of ctx, restarting the
// subprocess with micLiveRetryInterval backoff whenever it exits early —
// mirrors runRemoteLoadManager's reconnect loop for the same reason: a
// source can come and go mid-session (USB mic unplugged, PipeWire
// restarted) without that being a reason to give up on the box for the
// rest of the run.
func runMicLiveManager(ctx context.Context, m *micLiveMeter, onSample func(micLiveReading)) {
	for ctx.Err() == nil {
		captureMicLiveOnce(ctx, m, onSample)
		if ctx.Err() != nil {
			return
		}
		spec := watchMicLiveSpec()
		m.update(0, false, time.Now(), spec.ValueMetric(), spec.WindowDuration(), spec.DecayDuration())
		if onSample != nil {
			onSample(micLiveReading{Available: false})
		}
		waitOrDone(ctx, micLiveRetryInterval)
	}
}

// captureMicLiveOnce runs one `parec` invocation until it exits or ctx is
// canceled, publishing a rolling amplitude reading from its stdout as it
// arrives. cmd.Cancel sends SIGTERM (not Go's default SIGKILL) so the
// subprocess gets a chance to release the audio device cleanly, and
// WaitDelay bounds how long shutdown can take if it ignores that signal —
// together these are what makes ctx cancellation (tied to the watch loop's
// own sigCtx, or to Stop() on box-toggle-off) an actual clean-teardown path
// rather than an abandoned pipe.
func captureMicLiveOnce(ctx context.Context, m *micLiveMeter, onSample func(micLiveReading)) {
	cmd := exec.CommandContext(ctx, "parec",
		"--raw",
		"--format=s16le",
		"--rate="+strconv.Itoa(micLiveSampleRate),
		"--channels=1",
		"--latency-msec=20",
		"--process-time-msec=20",
		"-d", "@DEFAULT_SOURCE@",
	)
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 2 * time.Second

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}

	buf := make([]byte, micLiveChunkBytes)
	for ctx.Err() == nil {
		n, err := io.ReadFull(stdout, buf)
		if n > 0 {
			rawLevel := micLiveAmplitudeFromPCM16LE(buf[:n])
			spec := watchMicLiveSpec()
			reading := m.update(rawLevel, true, time.Now(), spec.ValueMetric(), spec.WindowDuration(), spec.DecayDuration())
			if onSample != nil {
				onSample(reading)
			}
		}
		if err != nil {
			break
		}
	}
	_ = cmd.Wait()
}

// micLiveAmplitudeFromPCM16LE computes a 0-100 RMS amplitude from a buffer
// of signed 16-bit little-endian mono PCM samples — pure and subprocess-free
// so it's directly unit-testable against synthetic buffers (silence, a known
// sine wave, full-scale noise). Odd trailing bytes (a chunk cut mid-sample)
// are ignored. The RMS level is scaled logarithmically in dBFS across
// [micLiveMinDBFS, 0] dBFS mapped to [0, 100] (issue 257) so conversational
// speech registers visibly at 25%-45% instead of being crushed into the bottom
// 1-2% by linear math.
func micLiveAmplitudeFromPCM16LE(buf []byte) float64 {
	n := len(buf) / 2
	if n == 0 {
		return 0
	}
	var sumSq float64
	for i := 0; i < n; i++ {
		s := int16(binary.LittleEndian.Uint16(buf[i*2 : i*2+2]))
		v := float64(s)
		sumSq += v * v
	}
	rms := math.Sqrt(sumSq / float64(n))
	if rms <= 0 {
		return 0
	}
	dBFS := 20 * math.Log10(rms / 32768.0)
	level := (dBFS - micLiveMinDBFS) / (0 - micLiveMinDBFS) * 100.0
	if level > 100 {
		level = 100
	}
	if level < 0 || math.IsNaN(level) {
		level = 0
	}
	return level
}
