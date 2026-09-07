package usage

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
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
	// (`parec`, or `pw-record` on the pipewire backend) is installed — the
	// box renders "n/a" for the live meter in that case rather than a
	// fabricated 0.
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
	// micBackendPipeWire (issue 265) is a PipeWire-native fallback for
	// systems running PipeWire without the `pulseaudio-utils` compat
	// package (no `pactl`/`parec` on PATH) — common on modern desktops
	// where PipeWire is the default sound server but the PulseAudio-compat
	// CLI shim is an optional extra package. Gain/mute/recording reads stay
	// out of scope (issue 265 §3 — those still fall through to amixer, or
	// degrade to zero values below); only the live-capture path in
	// miclive.go actually uses this backend.
	micBackendPipeWire
	micBackendAmixer
)

var (
	micBackendMu    sync.Mutex
	micBackendValue micBackendKind
	micBackendKnown bool
)

// probePactlDefaultSourceFn/probePipeWireReachableFn/probeAmixerCaptureFn
// are the real probes behind package-level variables (mirroring
// runAGYUsageCmdFn in agy.go) so tests can stub each candidate's outcome
// independently and exercise resolveMicBackend's preference order without a
// real pactl/PipeWire/ALSA rig on the test machine.
var (
	probePactlDefaultSourceFn = probePactlDefaultSource
	probePipeWireReachableFn  = probePipeWireReachable
	probeAmixerCaptureFn      = probeAmixerCapture
)

// runWpctlGetVolumeFn is the real `wpctl get-volume @DEFAULT_AUDIO_SOURCE@`
// invocation behind a package-level variable (issue 270), mirroring the
// probe*Fn seams above, so mic_test.go can stub its raw output/error
// independently without a real wpctl/WirePlumber rig on the test machine.
var runWpctlGetVolumeFn = runWpctlGetVolume

// resolveMicBackend canary-probes, in preference order: PipeWire/PulseAudio
// via `pactl` (the modern Linux default per this repo's kernel-standard-
// metrics-sourcing policy of preferring the most portable interface); then,
// on PipeWire systems missing the `pulseaudio-utils` compat package (issue
// 265), PipeWire-native `pw-record`/`pw-cli`; then plain ALSA (`amixer`) as
// the final fallback for systems with no sound server at all. Each
// candidate is a real read-only invocation, not just a LookPath check
// (docs/practices/Canary.md) — a binary can be installed but non-functional
// (no default source configured, no capture control on this card, no
// reachable PipeWire socket).
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
	case probePactlDefaultSourceFn() != "":
		kind = micBackendPactl
	case probePipeWireReachableFn():
		kind = micBackendPipeWire
	case probeAmixerCaptureFn():
		kind = micBackendAmixer
	}

	micBackendMu.Lock()
	micBackendValue, micBackendKnown = kind, true
	micBackendMu.Unlock()
	return kind
}

// resetMicBackendCacheForTest clears resolveMicBackend's cached result so
// tests can re-probe with a freshly stubbed probePactlDefaultSourceFn/
// probePipeWireReachableFn/probeAmixerCaptureFn. Test-only; production code
// never needs to invalidate the cache mid-process (see resolveMicBackend's
// doc comment: the installed interface doesn't change over a watch
// session's lifetime).
func resetMicBackendCacheForTest() {
	micBackendMu.Lock()
	micBackendKnown = false
	micBackendMu.Unlock()
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

// pwCliProbeTimeout bounds the `pw-cli info 0` canary invocation below —
// this runs on every cold resolveMicBackend() call on a pactl-less machine,
// so it must never hang the caller if PipeWire is installed but its socket
// is wedged (docs/practices/Canary.md: a real read-only call, not just a
// LookPath check, but still bounded).
const pwCliProbeTimeout = 2 * time.Second

// probePipeWireReachable reports whether this machine can use the
// PipeWire-native live-capture backend (issue 265): `pw-record` (the binary
// miclive.go's captureMicLiveOnceViaPipeWire actually shells out to) must be
// on PATH, and a real `pw-cli info 0` call against the running PipeWire
// daemon must succeed — confirming an actual reachable socket, not just
// that pipewire-bin happens to be installed. Only reached when
// probePactlDefaultSource already failed, so this never overrides the
// existing, more mature pactl path.
func probePipeWireReachable() bool {
	if _, err := exec.LookPath("pw-record"); err != nil {
		return false
	}
	if _, err := exec.LookPath("pw-cli"); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), pwCliProbeTimeout)
	defer cancel()
	return exec.CommandContext(ctx, "pw-cli", "info", "0").Run() == nil
}

// runWpctlGetVolume shells out to `wpctl get-volume @DEFAULT_AUDIO_SOURCE@`
// (issue 270) and returns its raw stdout. Only called from the pipewire
// backend, which already confirmed `wpctl`'s sibling PipeWire tooling
// (`pw-record`/`pw-cli`) is reachable via probePipeWireReachable — but
// `wpctl` ships in a separate `wireplumber` package on some distros, so a
// missing binary here degrades via the error return, not a panic.
func runWpctlGetVolume() (string, error) {
	out, err := exec.Command("wpctl", "get-volume", "@DEFAULT_AUDIO_SOURCE@").Output()
	return string(out), err
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
	case micBackendPipeWire:
		return currentMicStatusPipeWire()
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

// currentMicStatusPipeWire reports the pipewire backend's one-shot status.
// Issue 265 originally scoped configured-gain/mute/recording reads out of
// this backend entirely (§3: "out of scope here — this ticket is about the
// live streaming meter only"), reasoning that there was no
// `pactl get-source-volume`-equivalent one-shot PipeWire-native CLI call to
// reuse without a much larger parsing surface (`pw-cli`'s output is a
// verbose property dump, not a stable "NN%" line). That left Level/Muted
// hardcoded at zero, which issue 270 found actively misleading: a flat 0%
// gain bar reads as "this is the real level", not as "unsupported".
//
// Issue 270 closes the gain/mute half of that gap using WirePlumber's
// `wpctl get-volume @DEFAULT_AUDIO_SOURCE@` — canary-probed live on a real
// PipeWire-native machine (no pactl/parec on PATH) before this parser was
// written; real observed output:
//
//	Volume: 1.00
//	Volume: 1.00 [MUTED]
//
// Like `pw-record --target auto` (the live-capture path in miclive.go),
// `@DEFAULT_AUDIO_SOURCE@` resolves against PipeWire/WirePlumber's actual
// current default node, so this reading tracks whatever GNOME's sound
// settings currently has selected, not a stale/hardcoded device.
//
// Recording stays unimplemented here, mirroring 265 §3's own scoping
// language: `pactl list short source-outputs` (used by currentMicStatusPactl)
// has no direct one-shot `wpctl` equivalent — the closest substitute would
// be filtering `pw-dump`'s full node graph for stream nodes linked to the
// default source, a much larger parsing surface than this box's scope
// warrants for one boolean. buildMicBoxLines already renders "n/a" (not a
// fabricated "off") for this backend's Recording, so leaving it false here
// is a documented decision, not an oversight — a future ticket can revisit
// it if a simpler probe turns up.
func currentMicStatusPipeWire() MicStatus {
	st := MicStatus{Available: true, Backend: "pipewire"}

	out, err := runWpctlGetVolumeFn()
	if err != nil {
		// wpctl not installed, or the call failed (e.g. no default source
		// configured) — degrade to the pre-270 zero-value gain reading
		// rather than fabricating one; the box still renders (Available
		// stays true) since the pipewire backend's live-capture path
		// (pw-record) doesn't depend on wpctl at all.
		return st
	}
	if level, ok := parseWpctlVolumePercent(out); ok {
		st.Level = level
	}
	st.Muted = parseWpctlMuted(out)
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

// wpctlVolumeRe matches the fractional volume in `wpctl get-volume`'s
// output — real observed formats (issue 270 canary probe):
//
//	Volume: 1.00
//	Volume: 1.00 [MUTED]
var wpctlVolumeRe = regexp.MustCompile(`Volume:\s*([\d.]+)`)

// parseWpctlVolumePercent converts wpctl's 0.0-1.0(+) fraction to a 0-100
// percent, matching the scale of Level everywhere else in this package
// (pactl/amixer already report NN%). wpctl allows volumes above 1.00 (boosted
// gain) — that's passed through rather than clamped, same as pactl/amixer's
// own percent readings.
func parseWpctlVolumePercent(out string) (float64, bool) {
	m := wpctlVolumeRe.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return v * 100, true
}

// parseWpctlMuted reports whether wpctl's get-volume output carries the
// "[MUTED]" suffix observed in the issue 270 canary probe.
func parseWpctlMuted(out string) bool {
	return strings.Contains(out, "[MUTED]")
}

// parseAmixerCaptureOn reports whether amixer's Capture control shows at
// least one channel switched on. A control with every channel off is
// treated as not capturing; a control with only some channels off (an
// unusual asymmetric setup) is treated as on, since any open channel means
// the mic isn't fully closed.
func parseAmixerCaptureOn(out string) bool {
	return strings.Contains(out, "[on]")
}
