package usage

import (
	"errors"
	"strings"
	"testing"
)

// Fixtures below are real output captured from `pactl 17.0`/PipeWire and
// ALSA `amixer` on a live dev machine while implementing issue 244 (see its
// Resolution section) — not hand-written approximations of the format.

const pactlVolumeFixture = `Volume: front-left: 65536 / 100% / 0,00 dB,   front-right: 65536 / 100% / 0,00 dB
        balance 0,00
`

const pactlVolumeHalfFixture = `Volume: mono: 32768 /  50% / -18,06 dB
`

const pactlMuteNoFixture = "Mute: no\n"
const pactlMuteYesFixture = "Mute: yes\n"

const amixerCaptureFixture = `Simple mixer control 'Capture',0
  Capabilities: cvolume cswitch cswitch-joined
  Capture channels: Front Left - Front Right
  Limits: Capture 0 - 65536
  Front Left: Capture 65536 [100%] [on]
  Front Right: Capture 65536 [100%] [on]
`

const amixerCaptureMutedFixture = `Simple mixer control 'Capture',0
  Capabilities: cvolume cswitch cswitch-joined
  Capture channels: Front Left - Front Right
  Limits: Capture 0 - 65536
  Front Left: Capture 32768 [50%] [off]
  Front Right: Capture 32768 [50%] [off]
`

func TestParsePactlVolumePercent(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want float64
		ok   bool
	}{
		{"stereo 100%", pactlVolumeFixture, 100, true},
		{"mono 50%", pactlVolumeHalfFixture, 50, true},
		{"no match", "garbage\n", 0, false},
		{"empty", "", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parsePactlVolumePercent(c.in)
			if ok != c.ok || got != c.want {
				t.Errorf("parsePactlVolumePercent(%q) = (%v, %v), want (%v, %v)", c.in, got, ok, c.want, c.ok)
			}
		})
	}
}

func TestParsePactlMute(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"not muted", pactlMuteNoFixture, false},
		{"muted", pactlMuteYesFixture, true},
		{"garbage", "nonsense\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parsePactlMute(c.in); got != c.want {
				t.Errorf("parsePactlMute(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestParseAmixerCapturePercent(t *testing.T) {
	got, ok := parseAmixerCapturePercent(amixerCaptureFixture)
	if !ok || got != 100 {
		t.Errorf("parseAmixerCapturePercent(active) = (%v, %v), want (100, true)", got, ok)
	}
	got, ok = parseAmixerCapturePercent(amixerCaptureMutedFixture)
	if !ok || got != 50 {
		t.Errorf("parseAmixerCapturePercent(muted) = (%v, %v), want (50, true)", got, ok)
	}
}

func TestParseAmixerCaptureOn(t *testing.T) {
	if !parseAmixerCaptureOn(amixerCaptureFixture) {
		t.Error("expected active fixture to report on")
	}
	if parseAmixerCaptureOn(amixerCaptureMutedFixture) {
		t.Error("expected muted fixture to report off")
	}
}

// wpctlVolumeFixture/wpctlVolumeMutedFixture are real `wpctl get-volume
// @DEFAULT_AUDIO_SOURCE@` output captured on a live PipeWire-native dev
// machine (no pactl/parec on PATH) while implementing issue 270 — not a
// hand-written approximation, per docs/practices/Canary.md.
const wpctlVolumeFixture = "Volume: 1.00\n"
const wpctlVolumeMutedFixture = "Volume: 1.00 [MUTED]\n"

func TestParseWpctlVolumePercent(t *testing.T) {
	got, ok := parseWpctlVolumePercent(wpctlVolumeFixture)
	if !ok || got != 100 {
		t.Errorf("parseWpctlVolumePercent(%q) = (%v, %v), want (100, true)", wpctlVolumeFixture, got, ok)
	}
	got, ok = parseWpctlVolumePercent("Volume: 0.59\n")
	if !ok || got != 59 {
		t.Errorf("parseWpctlVolumePercent(0.59) = (%v, %v), want (59, true)", got, ok)
	}
	if _, ok := parseWpctlVolumePercent("garbage\n"); ok {
		t.Error("expected no match on garbage input")
	}
}

func TestParseWpctlMuted(t *testing.T) {
	if parseWpctlMuted(wpctlVolumeFixture) {
		t.Error("expected unmuted fixture to report false")
	}
	if !parseWpctlMuted(wpctlVolumeMutedFixture) {
		t.Error("expected [MUTED] fixture to report true")
	}
}

// Issue 270: currentMicStatusPipeWire now reads real gain/mute via wpctl
// (stubbed here through runWpctlGetVolumeFn, mirroring the probe*Fn seams
// used by resolveMicBackend's tests below) instead of the issue 265
// hardcoded-zero placeholder.
func TestCurrentMicStatusPipeWireReadsWpctl(t *testing.T) {
	orig := runWpctlGetVolumeFn
	defer func() { runWpctlGetVolumeFn = orig }()

	runWpctlGetVolumeFn = func() (string, error) { return wpctlVolumeMutedFixture, nil }
	got := currentMicStatusPipeWire()
	want := MicStatus{Available: true, Backend: "pipewire", Level: 100, Muted: true}
	if got != want {
		t.Errorf("currentMicStatusPipeWire() = %+v, want %+v", got, want)
	}
}

// Issue 270: when wpctl isn't installed or the call fails, degrade to the
// pre-270 zero-value gain reading rather than fabricating one — the box
// stays Available (the live-capture path doesn't depend on wpctl).
func TestCurrentMicStatusPipeWireDegradesWithoutWpctl(t *testing.T) {
	orig := runWpctlGetVolumeFn
	defer func() { runWpctlGetVolumeFn = orig }()

	runWpctlGetVolumeFn = func() (string, error) { return "", errors.New("exec: \"wpctl\": executable file not found in $PATH") }
	got := currentMicStatusPipeWire()
	want := MicStatus{Available: true, Backend: "pipewire"}
	if got != want {
		t.Errorf("currentMicStatusPipeWire() = %+v, want %+v", got, want)
	}
}

func TestCurrentMicStatusPactlUsesRealCommands(t *testing.T) {
	// currentMicStatusPactl/currentMicStatusAmixer shell out directly and
	// have no seam for fixture injection (mirroring CurrentGPUs/
	// CurrentCPULoad's own live-probe functions in load.go, which are
	// likewise untested directly and instead covered by testing the pure
	// parsers above). This test only documents that MicAvailable degrades
	// to a bool without panicking or erroring on whatever this test
	// machine actually has, so `go test ./...` never depends on a real
	// audio device being present.
	_ = MicAvailable()
	st := CurrentMicStatus()
	if !st.Available && (st.Level != 0 || st.Recording || st.Backend != "") {
		t.Errorf("expected zero-value MicStatus when unavailable, got %+v", st)
	}
}

func TestBuildMicBoxLinesHidesWhenUnavailable(t *testing.T) {
	lines := buildMicBoxLines(MicStatus{Available: false})
	if len(lines) != 1 {
		t.Fatalf("expected a single placeholder line, got %v", lines)
	}
}

func TestBuildMicBoxLinesPactlRecording(t *testing.T) {
	lines := buildMicBoxLines(MicStatus{Available: true, Backend: "pactl", Level: 80, Recording: true})
	if len(lines) != 2 {
		t.Fatalf("expected a gain line and a live line, got %v", lines)
	}
	if got := lines[0]; !containsAll(got, "80", "recording on") {
		t.Errorf("buildMicBoxLines recording=true = %q, want it to mention 80%% and recording on", got)
	}
}

func TestBuildMicBoxLinesAmixerRecordingUnknown(t *testing.T) {
	lines := buildMicBoxLines(MicStatus{Available: true, Backend: "amixer", Level: 100})
	if got := lines[0]; !containsAll(got, "recording n/a") {
		t.Errorf("buildMicBoxLines amixer backend = %q, want it to mention recording n/a", got)
	}
}

// Issue 265: the pipewire backend, like amixer, has no source-outputs-style
// signal for "who's holding this device open" — currentMicStatusPipeWire
// leaves Recording at its zero value, and the box must say "n/a" rather
// than imply "off" is a real observation, exactly like the amixer case
// above.
func TestBuildMicBoxLinesPipeWireRecordingUnknown(t *testing.T) {
	lines := buildMicBoxLines(MicStatus{Available: true, Backend: "pipewire", Level: 0})
	if got := lines[0]; !containsAll(got, "recording n/a") {
		t.Errorf("buildMicBoxLines pipewire backend = %q, want it to mention recording n/a", got)
	}
}

// Issue 270: once currentMicStatusPipeWire reads a real gain from wpctl,
// buildMicBoxLines must render that real percentage in the bar — not the
// pre-270 hardcoded 0% — while Recording (still unimplemented for this
// backend, see currentMicStatusPipeWire's doc comment) keeps rendering
// "n/a", not "off".
func TestBuildMicBoxLinesPipeWireRealGain(t *testing.T) {
	lines := buildMicBoxLines(MicStatus{Available: true, Backend: "pipewire", Level: 59})
	if got := lines[0]; !containsAll(got, "59", "recording n/a") {
		t.Errorf("buildMicBoxLines pipewire real gain = %q, want it to mention 59%% and recording n/a", got)
	}
	if containsAll(lines[0], "0%") {
		t.Errorf("buildMicBoxLines pipewire real gain = %q, must not render the pre-270 hardcoded 0%%", lines[0])
	}
}

func TestBuildMicBoxLinesMuted(t *testing.T) {
	lines := buildMicBoxLines(MicStatus{Available: true, Backend: "pactl", Level: 0, Muted: true})
	if got := lines[0]; !containsAll(got, "muted") {
		t.Errorf("buildMicBoxLines muted = %q, want it to mention muted", got)
	}
}

// Issue 245: buildMicBoxLines' second line renders the live peak/RMS
// reading, or an "n/a" placeholder when the capture subprocess isn't
// available (no parec, amixer backend, still (re)connecting) — distinct
// from Available==false, which hides the whole box (tested above).
func TestBuildMicBoxLinesLiveAvailable(t *testing.T) {
	lines := buildMicBoxLines(MicStatus{Available: true, Backend: "pactl", Level: 50, LiveAvailable: true, LiveLevel: 37})
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %v", lines)
	}
	if got := lines[1]; !containsAll(got, "37", "live") {
		t.Errorf("buildMicBoxLines live line = %q, want it to mention 37%% and live", got)
	}
}

func TestBuildMicBoxLinesLiveUnavailable(t *testing.T) {
	lines := buildMicBoxLines(MicStatus{Available: true, Backend: "amixer", Level: 100, LiveAvailable: false})
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %v", lines)
	}
	if got := lines[1]; !containsAll(got, "live", "n/a") {
		t.Errorf("buildMicBoxLines live-unavailable line = %q, want it to mention live n/a", got)
	}
}

// Issue 262: the amixer backend can never stream a live reading (issue
// 244/245 — no parec-equivalent for plain ALSA), a permanent structural
// limitation distinct from a pactl system that is merely still
// (re)connecting. The live line must say so explicitly rather than reusing
// the same bare "n/a" for both cases, so an amixer-only user doesn't read
// the live meter as broken.
func TestBuildMicBoxLinesLiveUnavailableAmixerExplainsWhy(t *testing.T) {
	amixerLines := buildMicBoxLines(MicStatus{Available: true, Backend: "amixer", Level: 100, LiveAvailable: false})
	if got := amixerLines[1]; !containsAll(got, "live", "n/a", "pactl") {
		t.Errorf("buildMicBoxLines amixer live-unavailable line = %q, want it to explain pactl/PipeWire is required", got)
	}
	// Issue 265: amixer is only reached once *both* live-capable backends
	// (pactl/parec and the new pipewire/pw-record path) have failed to
	// resolve — the "n/a" explanation must name both, not just pactl, or an
	// amixer-only user on a PipeWire-less machine is told to install
	// PipeWire tooling that would never help them anyway.
	if got := amixerLines[1]; !containsAll(got, "PipeWire") {
		t.Errorf("buildMicBoxLines amixer live-unavailable line = %q, want it to also mention PipeWire/pw-record", got)
	}

	pactlLines := buildMicBoxLines(MicStatus{Available: true, Backend: "pactl", Level: 100, LiveAvailable: false})
	if got := pactlLines[1]; strings.Contains(got, "pactl") {
		t.Errorf("buildMicBoxLines pactl live-unavailable line = %q, should not claim pactl is missing", got)
	}
	if got := pactlLines[1]; !containsAll(got, "live", "n/a") {
		t.Errorf("buildMicBoxLines pactl live-unavailable line = %q, want it to mention live n/a", got)
	}

	// Issue 265: a pipewire-backend reading that is transiently unavailable
	// (still (re)connecting the pw-record subprocess) must not fall into the
	// amixer branch's permanent "needs pactl/PipeWire" message — PipeWire is
	// exactly what this backend already found.
	pipeWireLines := buildMicBoxLines(MicStatus{Available: true, Backend: "pipewire", Level: 0, LiveAvailable: false})
	if got := pipeWireLines[1]; strings.Contains(got, "needs pactl") {
		t.Errorf("buildMicBoxLines pipewire live-unavailable line = %q, should not claim pactl/PipeWire is missing", got)
	}
	if got := pipeWireLines[1]; !containsAll(got, "live", "n/a") {
		t.Errorf("buildMicBoxLines pipewire live-unavailable line = %q, want it to mention live n/a", got)
	}
}

// Issue 265: resolveMicBackend must prefer pw-record/PipeWire over amixer
// when pactl/parec are absent but a PipeWire socket is reachable, and must
// still prefer pactl over pipewire when both resolve — exercised via the
// swappable probe*Fn variables rather than real subprocesses, since none of
// pactl/pw-record/amixer are guaranteed present on the test machine.
func TestResolveMicBackendPrefersPipeWireOverAmixer(t *testing.T) {
	origPactl, origPipeWire, origAmixer := probePactlDefaultSourceFn, probePipeWireReachableFn, probeAmixerCaptureFn
	t.Cleanup(func() {
		probePactlDefaultSourceFn, probePipeWireReachableFn, probeAmixerCaptureFn = origPactl, origPipeWire, origAmixer
		resetMicBackendCacheForTest()
	})

	probePactlDefaultSourceFn = func() string { return "" } // no pactl/parec on PATH
	probePipeWireReachableFn = func() bool { return true }  // PipeWire socket reachable, pw-record on PATH
	probeAmixerCaptureFn = func() bool { return true }      // amixer also present, but must lose to pipewire
	resetMicBackendCacheForTest()

	if got := resolveMicBackend(); got != micBackendPipeWire {
		t.Errorf("resolveMicBackend() = %v, want micBackendPipeWire when pactl absent and PipeWire reachable", got)
	}
}

func TestResolveMicBackendPrefersPactlOverPipeWire(t *testing.T) {
	origPactl, origPipeWire, origAmixer := probePactlDefaultSourceFn, probePipeWireReachableFn, probeAmixerCaptureFn
	t.Cleanup(func() {
		probePactlDefaultSourceFn, probePipeWireReachableFn, probeAmixerCaptureFn = origPactl, origPipeWire, origAmixer
		resetMicBackendCacheForTest()
	})

	probePactlDefaultSourceFn = func() string { return "alsa_input.pci-0000_00_1f.3.analog-stereo" }
	probePipeWireReachableFn = func() bool { return true }
	probeAmixerCaptureFn = func() bool { return true }
	resetMicBackendCacheForTest()

	if got := resolveMicBackend(); got != micBackendPactl {
		t.Errorf("resolveMicBackend() = %v, want micBackendPactl when a default source is configured", got)
	}
}

func TestResolveMicBackendFallsBackToAmixerWhenNeitherPactlNorPipeWire(t *testing.T) {
	origPactl, origPipeWire, origAmixer := probePactlDefaultSourceFn, probePipeWireReachableFn, probeAmixerCaptureFn
	t.Cleanup(func() {
		probePactlDefaultSourceFn, probePipeWireReachableFn, probeAmixerCaptureFn = origPactl, origPipeWire, origAmixer
		resetMicBackendCacheForTest()
	})

	probePactlDefaultSourceFn = func() string { return "" }
	probePipeWireReachableFn = func() bool { return false }
	probeAmixerCaptureFn = func() bool { return true }
	resetMicBackendCacheForTest()

	if got := resolveMicBackend(); got != micBackendAmixer {
		t.Errorf("resolveMicBackend() = %v, want micBackendAmixer when neither pactl nor PipeWire resolve", got)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
