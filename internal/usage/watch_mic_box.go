package usage

import (
	"fmt"

	"ubunatic.com/harnez/internal/rograph"
)

// buildMicBox renders issue 244's compact panel: the default system
// microphone's input level and whether anything is currently recording
// from it. st is resolved once per redraw frame by the caller (see the
// micStatus comment in buildWatchFrameAt) so the panel-measure/panel-render
// double-build that every panel gets doesn't spawn the underlying pactl/
// amixer subprocess twice.
func buildMicBox(width int, st MicStatus) wbox {
	title := watchBoxSymbol("mic") + " Mic"
	return wbox{title: title, lines: buildMicBoxLines(st), width: width}
}

// buildMicBoxLines renders MicStatus as two lines — the configured-gain bar
// (issue 244) and, below it, the live peak/RMS reading (issue 245) or an
// "n/a" placeholder when live capture isn't available — or a single dim
// "unavailable" placeholder when no audio interface was found at all
// (Available == false) — the box itself is never added to the panel list
// in that case (see buildWatchFrameAt), but this keeps buildMicBoxLines
// safe to call standalone, e.g. from tests.
//
// Issue 262: the live line's "n/a" placeholder distinguishes the amixer
// backend's permanent structural limitation (no streaming-capable
// equivalent to parec — see miclive.go's startMicLiveManager) from the
// generic transient case (pactl backend still (re)connecting, or no parec
// binary installed) so an amixer-only user isn't left guessing whether the
// live meter is broken or simply unsupported on their setup.
func buildMicBoxLines(st MicStatus) []string {
	if !st.Available {
		return []string{ansiDimGrey + "mic unavailable\x1b[0m"}
	}

	bar := rograph.RenderBar(st.Level, rograph.BarOptions{IncludePercent: true, PercentPrecision: 0})

	recordingWord := "off"
	if st.Recording {
		recordingWord = "on"
	}
	if st.Backend == "amixer" || st.Backend == "pipewire" {
		// ALSA has no generic "who's holding this device open" signal the
		// way PipeWire/PulseAudio's source-outputs list does (see
		// currentMicStatusAmixer) — say so rather than implying "off" is an
		// actual observation. The pipewire backend (issue 265) doesn't probe
		// this either — it's scoped to the live-capture path only (see
		// currentMicStatusPipeWire) — so it gets the same "n/a" treatment.
		recordingWord = "n/a"
	}

	line := fmt.Sprintf("%s   recording %s", bar, recordingWord)
	if st.Muted {
		line += ansiDimGrey + "  (muted)\x1b[0m"
	}

	// Issue 245: a second line for the genuine live signal reading,
	// alongside (not replacing) the gain bar above — "gain" answers "is the
	// mic turned up", "live" answers "is sound actually reaching it right
	// now". Rendered as "n/a" rather than a fabricated bar when the capture
	// subprocess isn't available (no parec, amixer backend, or still
	// (re)connecting) — see micLiveManager's doc comment for when that is.
	var liveLine string
	switch {
	case st.LiveAvailable:
		liveBar := rograph.RenderBar(st.LiveLevel, rograph.BarOptions{IncludePercent: true, PercentPrecision: 0})
		liveLine = fmt.Sprintf("%s   live", liveBar)
	case st.Backend == "amixer":
		// Issue 262/265: the amixer/plain-ALSA fallback has no
		// streaming-capable equivalent to parec or pw-record (see
		// miclive.go's startMicLiveManager doc comment) — this is a
		// permanent, structural limitation of the backend, not a transient
		// "still connecting" state, so say so explicitly instead of the
		// generic "n/a" that could as easily mean "reconnecting" on a pactl
		// or pipewire system. Both a live-capture-capable backend (pactl,
		// parec) and this PipeWire-native one (issue 265, pw-record) are
		// tried before falling back to amixer, so the message names both.
		liveLine = ansiDimGrey + "live n/a (needs pactl or PipeWire w/ pw-record)" + "\x1b[0m"
	default:
		liveLine = ansiDimGrey + "live n/a" + "\x1b[0m"
	}

	return []string{line, liveLine}
}
