package usage

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// MicStatus is a single poll of the default system microphone input: its
// current gain/level, mute state, and whether anything is actively
// capturing from it right now (issue 244). Available reports whether any
// supported audio-server interface exists on this machine at all —
// callers must check it before trusting the other fields, exactly like
// CPULoad.Ok/SystemMemory.Ok elsewhere in this package.
type MicStatus struct {
	// Level is the default input source's configured gain, 0-100. This is
	// the source volume (what the mixer is set to), not a true real-time
	// RMS/peak signal meter — PipeWire/PulseAudio only expose true peak
	// monitoring through a subscribed streaming API, not a one-shot CLI
	// call, and this box polls at the watch redraw cadence rather than
	// holding an audio stream open. See issues/244's Resolution section
	// for why this tradeoff was made instead of streaming peak data.
	Level float64
	// Muted reports the default source's hardware/software mute switch.
	Muted bool
	// Recording reports whether some process currently holds an active
	// capture stream open on the default source. Only observable on the
	// pactl backend (PipeWire/PulseAudio's source-outputs list); the plain
	// ALSA amixer fallback has no equivalent generic concept, so Recording
	// is always false there — Backend tells the box renderer which case
	// it's in so it can say "n/a" instead of implying "off".
	Recording bool
	// Available is false when no supported audio-server interface could be
	// found or its canary probe failed (no PipeWire/PulseAudio socket, no
	// ALSA capture control, or simply no microphone). The box hides itself
	// entirely when this is false rather than rendering an error.
	Available bool
	// Backend names which interface produced this reading: "pactl" or
	// "amixer". Empty when Available is false.
	Backend string
	// LiveLevel is a genuine live signal reading (0-100, RMS-based) of the
	// actual incoming sound hitting the default source right now — issue
	// 245's answer to "the volume slider is always at 100%, that's not the
	// real noise". Unlike Level (the configured gain, unmoving unless the
	// user changes it), this comes from a held-open low-rate audio capture
	// (see miclive.go) and changes continuously with real ambient sound.
	// Meaningless when LiveAvailable is false.
	LiveLevel float64
	// LiveAvailable reports whether LiveLevel is a real, currently-flowing
	// reading. False while the capture subprocess is (re)connecting,
	// permanently false on the amixer backend or when no capture binary
	// (`parec`) is installed — the box renders "n/a" for the live meter in
	// that case rather than a fabricated 0.
	LiveAvailable bool
}

// micBackendKind is the audio-server interface CurrentMicStatus reads from,
// resolved once per process (mirroring amdGPUCodename's cache in load.go):
// probing involves a real subprocess per candidate, and which interface is
// installed doesn't change over a watch session's lifetime.
type micBackendKind int

const (
	micBackendNone micBackendKind = iota
	micBackendPactl
	micBackendAmixer
)

var (
	micBackendMu    sync.Mutex
	micBackendValue micBackendKind
	micBackendKnown bool
)

// resolveMicBackend canary-probes, in preference order, PipeWire/PulseAudio
// (`pactl`, the modern Linux default per this repo's kernel-standard-
// metrics-sourcing policy of preferring the most portable interface) and
// then plain ALSA (`amixer`) as the fallback for systems with no sound
// server. Each candidate is a real read-only invocation, not just a
// LookPath check (docs/practices/Canary.md) — a binary can be installed
// but non-functional (no default source configured, no capture control on
// this card).
func resolveMicBackend() micBackendKind {
	micBackendMu.Lock()
	if micBackendKnown {
		v := micBackendValue
		micBackendMu.Unlock()
		return v
	}
	micBackendMu.Unlock()

	kind := micBackendNone
	switch {
	case probePactlDefaultSource() != "":
		kind = micBackendPactl
	case probeAmixerCapture():
		kind = micBackendAmixer
	}

	micBackendMu.Lock()
	micBackendValue, micBackendKnown = kind, true
	micBackendMu.Unlock()
	return kind
}

// MicAvailable reports whether resolveMicBackend found any usable audio
// interface, without doing the (slightly heavier) live level/recording
// read. Used by the watch TUI to decide whether the Mic box's toggle key
// should count as "hidden" or simply doesn't apply on this machine.
func MicAvailable() bool {
	return resolveMicBackend() != micBackendNone
}

// probePactlDefaultSource returns the default source's name, or "" if
// `pactl` isn't installed or reports no configured default source.
func probePactlDefaultSource() string {
	if _, err := exec.LookPath("pactl"); err != nil {
		return ""
	}
	out, err := exec.Command("pactl", "get-default-source").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// probeAmixerCapture reports whether ALSA's simple mixer has a 'Capture'
// control at all (the common name for the primary capture control; systems
// with a differently-named capture control fall through to Available:
// false rather than guessing at names — kept deliberately narrow per this
// ticket's "one focused box, not a general audio subsystem" scope).
func probeAmixerCapture() bool {
	if _, err := exec.LookPath("amixer"); err != nil {
		return false
	}
	out, err := exec.Command("amixer", "get", "Capture").Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// CurrentMicStatus polls whichever audio backend resolveMicBackend found
// and returns the default microphone's current level/mute/recording state.
// Returns a zero-value (Available: false) MicStatus when no backend is
// available or a live read fails — never an error the caller has to check
// separately, matching CurrentCPULoad/CurrentGPUs's probe-and-degrade shape.
func CurrentMicStatus() MicStatus {
	switch resolveMicBackend() {
	case micBackendPactl:
		return currentMicStatusPactl()
	case micBackendAmixer:
		return currentMicStatusAmixer()
	default:
		return MicStatus{}
	}
}

func currentMicStatusPactl() MicStatus {
	volOut, err := exec.Command("pactl", "get-source-volume", "@DEFAULT_SOURCE@").Output()
	if err != nil {
		// The default source can disappear mid-session (a USB mic
		// unplugged, PipeWire restarted) — degrade to unavailable for this
		// frame rather than showing a stale or zeroed-out "live" reading.
		return MicStatus{}
	}
	level, _ := parsePactlVolumePercent(string(volOut))
	st := MicStatus{Available: true, Backend: "pactl", Level: level}

	if muteOut, err := exec.Command("pactl", "get-source-mute", "@DEFAULT_SOURCE@").Output(); err == nil {
		st.Muted = parsePactlMute(string(muteOut))
	}
	if soOut, err := exec.Command("pactl", "list", "short", "source-outputs").Output(); err == nil {
		st.Recording = strings.TrimSpace(string(soOut)) != ""
	}
	return st
}

func currentMicStatusAmixer() MicStatus {
	out, err := exec.Command("amixer", "get", "Capture").Output()
	if err != nil {
		return MicStatus{}
	}
	level, _ := parseAmixerCapturePercent(string(out))
	return MicStatus{
		Available: true,
		Backend:   "amixer",
		Level:     level,
		Muted:     !parseAmixerCaptureOn(string(out)),
		// Recording stays false: plain ALSA has no generic "who's holding
		// this capture device open" signal the way PipeWire/PulseAudio's
		// source-outputs list does. buildMicBoxLines renders this as
		// "n/a", not as "off", so it isn't misread as a real observation.
	}
}

// percentRe matches the first "NN%" occurring in pactl's
// "front-left: 65536 / 100% / 0,00 dB" volume output, or amixer's
// "Front Left: Capture 65536 [100%] [on]" line — both formats put the
// channel's percentage right after the raw integer, before the unit that
// differs between the two tools (dB text vs. a bracketed on/off switch).
var percentRe = regexp.MustCompile(`(\d+)%`)

func parsePactlVolumePercent(out string) (float64, bool) {
	m := percentRe.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	return v, err == nil
}

func parsePactlMute(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "Mute:"); ok {
			return strings.TrimSpace(rest) == "yes"
		}
	}
	return false
}

func parseAmixerCapturePercent(out string) (float64, bool) {
	m := percentRe.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	return v, err == nil
}

// parseAmixerCaptureOn reports whether amixer's Capture control shows at
// least one channel switched on. A control with every channel off is
// treated as not capturing; a control with only some channels off (an
// unusual asymmetric setup) is treated as on, since any open channel means
// the mic isn't fully closed.
func parseAmixerCaptureOn(out string) bool {
	return strings.Contains(out, "[on]")
}
