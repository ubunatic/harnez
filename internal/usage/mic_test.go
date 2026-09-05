package usage

import (
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
	if len(lines) != 1 {
		t.Fatalf("expected a single line, got %v", lines)
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

func TestBuildMicBoxLinesMuted(t *testing.T) {
	lines := buildMicBoxLines(MicStatus{Available: true, Backend: "pactl", Level: 0, Muted: true})
	if got := lines[0]; !containsAll(got, "muted") {
		t.Errorf("buildMicBoxLines muted = %q, want it to mention muted", got)
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
