package usage

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// pcm16LE encodes signed 16-bit samples as little-endian bytes, matching
// what `parec --format=s16le` streams and what audiolevel.AmplitudeFromPCM16LE
// consumes. The pure amplitude/ballistics/meter math this used to exercise
// directly now lives (and is tested) in ubunatic.com/voxi/audiolevel; the
// tests kept here instead cover this package's own backend-selection wiring
// (resolveMicBackend -> which capture command startMicLiveManager builds).
func pcm16LE(samples []int16) []byte {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	return buf
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

// writeFakeAudioBinary writes an executable shell script into dir named
// name that dumps payload to stdout and exits (clean EOF, no lingering
// process) — a stand-in for `parec`/`pw-record` that lets
// audiolevel.RunCapture's real exec.Command/StdoutPipe/io.ReadFull
// machinery run against a known byte stream instead of a real audio device.
func writeFakeAudioBinary(t *testing.T, dir, name string, payload []byte) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("fake audio binary script assumes a POSIX shell (linux CI)")
	}
	payloadPath := filepath.Join(dir, name+".pcm")
	if err := os.WriteFile(payloadPath, payload, 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	scriptPath := filepath.Join(dir, name)
	script := "#!/bin/sh\ncat " + payloadPath + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake binary %s: %v", name, err)
	}
	return scriptPath
}

// TestStartMicLiveManagerSelectsPipeWireCaptureFn documents startMicLiveManager's
// backend->capture-command wiring (issue 265): on the pipewire backend with
// pw-record on PATH, it must actually dispatch to a pw-record-based
// audiolevel.Manager (not silently no-op the way pre-265 code did for every
// non-pactl backend). Proven by observing a real onSample callback fire from
// the spawned goroutine's fake `pw-record` output, rather than only checking
// the returned *micLiveManager is non-nil (which is always true regardless
// of whether a goroutine was actually started).
func TestStartMicLiveManagerSelectsPipeWireCaptureFn(t *testing.T) {
	origPactl, origPipeWire, origAmixer := probePactlDefaultSourceFn, probePipeWireReachableFn, probeAmixerCaptureFn
	t.Cleanup(func() {
		probePactlDefaultSourceFn, probePipeWireReachableFn, probeAmixerCaptureFn = origPactl, origPipeWire, origAmixer
		resetMicBackendCacheForTest()
	})
	probePactlDefaultSourceFn = func() string { return "" }
	probePipeWireReachableFn = func() bool { return true }
	probeAmixerCaptureFn = func() bool { return false }
	resetMicBackendCacheForTest()

	samples := make([]int16, 400)
	for i := range samples {
		samples[i] = 550
	}
	dir := t.TempDir()
	fakeBin := writeFakeAudioBinary(t, dir, "pw-record", pcm16LE(samples))
	t.Setenv("PATH", filepath.Dir(fakeBin)+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	samplesCh := make(chan micLiveReading, 1)
	mgr := startMicLiveManager(ctx, func(r micLiveReading) {
		select {
		case samplesCh <- r:
		default:
		}
	})
	t.Cleanup(mgr.Stop)

	select {
	case r := <-samplesCh:
		if !r.Available || r.Level <= 0 {
			t.Errorf("expected a real available, non-zero reading from the fake pw-record binary, got %+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for onSample: startMicLiveManager did not dispatch to the pipewire capture path")
	}
}
