package usage

import (
	"context"
	"os/exec"
	"time"

	"ubunatic.com/voxi/audiolevel"
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
// The actual capture/RMS/dBFS/ballistics pipeline now lives in
// ubunatic.com/voxi/audiolevel (moved out so voxi's own tooling can share
// it instead of harnez maintaining a private copy — see docs/MicIndicators.md
// for the desktop-indicator research that shaped it). This file only keeps
// the harnez-specific bits: resolving which backend/binary to shell out to
// (mic.go's resolveMicBackend) and wiring that into audiolevel's Manager
// with this session's configured spec (indicatorsspec.go's
// watchMicLiveSpec).
//
// Canary-probed by hand on a PipeWire 17.0 dev machine (2026-09-05, see
// issue 245's Resolution section for the full transcript): both `parec
// --raw --format=s16le --rate=8000 --channels=1 -d @DEFAULT_SOURCE@` and the
// `pw-cat -r --target <source> --format s16 --rate 8000 --channels 1 -`
// equivalent produced live, changing amplitude data and terminated cleanly
// on SIGTERM. `parec` was chosen for the pactl backend: it already matches
// this file's pactl-only backend gate (issue 244 only builds the
// streaming-capable Recording field on the pactl backend, never amixer) and
// needs no numeric sink/source id resolution — `@DEFAULT_SOURCE@` tracks
// source changes on its own, the same symbolic name mic.go's pactl calls
// already use.
//
// Issue 265 added a second capture path for PipeWire systems missing the
// `pulseaudio-utils` compat package (no `pactl`/`parec` on PATH — see
// mic.go's micBackendPipeWire and probePipeWireReachable). It uses
// `pw-record` rather than `pw-cat`: `pw-record` is dedicated to recording
// (no `-r`/`--record` mode flag needed) and, like `pw-cat`, leaves
// `--target` at its default ("auto"), which tracks the current default
// capture source. Live-verified on this session's actual pactl-less
// PipeWire machine (see issue 265's Resolution section for the exact
// command and observed RMS values).

// micLiveReading is a type alias (not a distinct type) so existing call
// sites and tests across this package keep working unchanged after the
// meter's implementation moved to audiolevel.Reading.
type micLiveReading = audiolevel.Reading

// micLiveManager owns the lifecycle of one background capture goroutine.
// Start it when the Mic box becomes visible, Stop it when the box is
// toggled off or the watch loop exits — never leave one running unobserved
// (Zero Zombie Guarantee, docs/practices/AgenticLoop.md).
type micLiveManager struct {
	mgr *audiolevel.Manager
}

// startMicLiveManager begins capturing, deriving its own lifetime from
// parent (the watch loop's sigCtx) so a Ctrl-C/SIGTERM that ends the whole
// --watch process also tears this down even if Stop is never called
// explicitly. If onSample is non-nil, it is invoked on every newly processed
// audio chunk. Starts audiolevel.StartManager with a nil buildCmd (meaning
// live capture structurally can't work here) when neither the pactl backend
// (issue 244 already requires pactl's streaming-capable Recording field for
// this scope, and needs `parec` on PATH) nor the pipewire backend (issue
// 265, needs `pw-record` on PATH) is resolved — the plain-ALSA amixer
// fallback has no monitor-stream equivalent to either, matching
// buildMicBoxLines' existing "n/a" treatment of amixer's Recording field.
// This is the "degrade gracefully" path (issue 245 requirement 3): the
// caller renders LiveAvailable: false, never an error.
func startMicLiveManager(parent context.Context, onSample func(micLiveReading)) *micLiveManager {
	var buildCmd func(context.Context) *exec.Cmd
	switch resolveMicBackend() {
	case micBackendPactl:
		if _, err := exec.LookPath("parec"); err == nil {
			buildCmd = func(ctx context.Context) *exec.Cmd { return audiolevel.ParecCommand(ctx, audiolevel.DefaultSampleRate) }
		}
	case micBackendPipeWire:
		if _, err := exec.LookPath("pw-record"); err == nil {
			buildCmd = func(ctx context.Context) *exec.Cmd {
				return audiolevel.PwRecordCommand(ctx, audiolevel.DefaultSampleRate)
			}
		}
	}

	spec := func() (audiolevel.Metric, time.Duration, time.Duration, time.Duration) {
		s := watchMicLiveSpec()
		return audiolevel.Metric(s.ValueMetric()), s.WindowDuration(), s.AttackDuration(), s.DecayDuration()
	}
	mgr := audiolevel.StartManager(parent, buildCmd, audiolevel.DefaultChunkBytes, audiolevel.DefaultMinDBFS, micLiveRetryInterval, spec, onSample)
	return &micLiveManager{mgr: mgr}
}

// micLiveRetryInterval paces reconnect attempts after the capture
// subprocess exits early (source unplugged, PipeWire restarted) or never
// starts (permission denied) — mirrors remoteLoadRetryInterval's role for
// the analogous remote-load streaming manager.
const micLiveRetryInterval = 5 * time.Second

// micGracePeriod is the duration high-frequency redraws continue after
// sound/speech returns to silence (issue 258) to allow smooth ballistics
// decay release before idling back to standard 1s cadence. Redraw cadence
// is harnez-specific TUI behavior, so this stays here rather than moving
// into audiolevel with the rest of the capture pipeline.
const micGracePeriod = 800 * time.Millisecond

// Stop tears down the capture subprocess (if any) and its reader goroutine.
// Safe to call on a nil *micLiveManager (mirrors the nil-receiver-safe
// pattern used elsewhere in this package) and safe to call more than once.
func (m *micLiveManager) Stop() {
	if m == nil {
		return
	}
	m.mgr.Stop()
}

// Snapshot returns the most recently published reading. Safe on nil.
func (m *micLiveManager) Snapshot() micLiveReading {
	if m == nil {
		return micLiveReading{}
	}
	return m.mgr.Snapshot()
}
