package usage

import (
	"fmt"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/rograph"
)

// Startup splash tuning (issue 164). splashSpinnerSequenceName reuses
// spec/indicators.yaml's existing "braille-classic-10" spinner rather than
// adding a new one -- it already fits a one-off "work in progress" glyph.
const (
	splashSpinnerSequenceName = "braille-classic-10"
	splashFrameInterval       = 90 * time.Millisecond
	splashBarSweepPeriod      = 1200 * time.Millisecond
	splashBarWidth            = 24
)

// splashSpinnerGlyph returns the animated spinner glyph for the startup
// splash, cycling through spec/indicators.yaml's "braille-classic-10"
// sequence on splashFrameInterval ticks.
func splashSpinnerGlyph(elapsed time.Duration) string {
	frames := namedSequenceFrames(splashSpinnerSequenceName, "spinner")
	idx := int(elapsed/splashFrameInterval) % len(frames)
	return frames[idx]
}

// splashBarCapPercent bounds the determinate splash bar (issue 168) below
// 100% until the fetch it is tracking actually completes, so the bar never
// visually "finishes" and then appears to stall while the dashboard is
// still waiting on real data.
const splashBarCapPercent = 95.0

// splashBarPercent computes the startup splash bar's fill percentage.
//
// When haveEstimate is true, it is determinate: pct = 100 * elapsed /
// estimate, capped at splashBarCapPercent, so the bar fills monotonically
// left-to-right and reaches ~95% around when the tracked fetch
// (CollectAll/CollectRemote) is expected to finish -- estimate comes from
// this project's own persisted fetch-duration history (fetchdurations.go),
// not a guess.
//
// When haveEstimate is false (a true cold start: no prior sample exists yet
// for this fetch kind), it falls back to the original issue-164 sweep --
// 0-100-0 over splashBarSweepPeriod -- reading as "indeterminate work in
// progress" rather than fabricating a determinate bar with no real basis.
func splashBarPercent(elapsed, estimate time.Duration, haveEstimate bool) float64 {
	if !haveEstimate || estimate <= 0 {
		return splashBarPercentSweep(elapsed)
	}
	pct := 100 * float64(elapsed) / float64(estimate)
	switch {
	case pct < 0:
		return 0
	case pct > splashBarCapPercent:
		return splashBarCapPercent
	default:
		return pct
	}
}

// splashBarPercentSweep sweeps 0-100-0 over splashBarSweepPeriod -- the
// original issue-164 indeterminate bar, kept as splashBarPercent's
// no-estimate-yet fallback.
func splashBarPercentSweep(elapsed time.Duration) float64 {
	phase := elapsed % splashBarSweepPeriod
	half := splashBarSweepPeriod / 2
	if phase < half {
		return 100 * float64(phase) / float64(half)
	}
	return 100 - 100*float64(phase-half)/float64(half)
}

// centerLine pads s with leading spaces so its visible content centers
// within width columns. Trailing padding is left to fit/paint's per-line
// erase-to-end-of-line.
func centerLine(s string, width int) string {
	if s == "" {
		return ""
	}
	pad := (width - visLen(s)) / 2
	if pad <= 0 {
		return s
	}
	return strings.Repeat(" ", pad) + s
}

// splashStatusMinDisplay is the minimum time each queued FetchStage event
// stays on screen before splashStatusAdvance lets the next one replace it.
// Local sub-fetches (claude/agy/codex) routinely complete in well under this
// project's splashFrameInterval paint cadence, so without pacing the splash
// only ever shows whichever event happened to be latest when the fetch
// finished -- the whole point of issue 169's status line (seeing each stage
// go by) was otherwise lost. 300ms is long enough to read a short line, short
// enough that a handful of queued events don't meaningfully delay handing
// control to the real dashboard once the fetch is actually done.
const splashStatusMinDisplay = 300 * time.Millisecond

// splashStatusEvent is one FetchStage transition queued for display.
type splashStatusEvent struct {
	source string
	stage  FetchStage
}

// splashBadge represents one completed (or failed) probe/collector badge (issue 252).
type splashBadge struct {
	source string
	ok     bool
}

// splashStatusState is the splash status line's queue of pending events plus
// which one is currently on screen. reportFetchStage (in RunWatchWithOptions)
// appends to queue; splashStatusAdvance pops from it on a minimum-display
// cadence so every event gets a turn instead of only the latest one.
type splashStatusState struct {
	queue   []splashStatusEvent
	current splashStatusEvent
	have    bool
	shownAt time.Time
	badges  []splashBadge
}

// splashStatusRecord records a new FetchStage event into st: appending the
// event to queue for the rolling single-line status log, and recording a
// completed/failed badge into st.badges when the event is terminal (FetchDone
// or FetchFailed).
func splashStatusRecord(st splashStatusState, source string, stage FetchStage) splashStatusState {
	if source == "" {
		return st
	}
	st.queue = append(st.queue, splashStatusEvent{source: source, stage: stage})
	if stage == FetchDone || stage == FetchFailed {
		ok := stage == FetchDone
		found := false
		for i, b := range st.badges {
			if b.source == source {
				st.badges[i].ok = ok
				found = true
				break
			}
		}
		if !found {
			st.badges = append(st.badges, splashBadge{source: source, ok: ok})
		}
	}
	return st
}

// splashStatusAdvance pops the next queued event into st.current once
// splashStatusMinDisplay has elapsed since the current one was shown (or
// immediately, if nothing has been shown yet). Pure function of (st, now) so
// it can be unit-tested without a real clock/goroutines.
func splashStatusAdvance(st splashStatusState, now time.Time, minDisplay time.Duration) splashStatusState {
	if len(st.queue) == 0 {
		return st
	}
	if st.have && now.Sub(st.shownAt) < minDisplay {
		return st
	}
	st.current = st.queue[0]
	st.queue = st.queue[1:]
	st.have = true
	st.shownAt = now
	return st
}

// splashStatusDrained reports whether the status queue is empty and the
// currently-shown event (if any) has been up long enough that advancing
// again would be a no-op -- i.e. there is nothing left worth waiting to
// display. RunWatchWithOptions uses this to decide when it's safe to leave
// the splash after the underlying fetch has finished: without it, a fetch
// that completes faster than its own queued events can be paced through
// would cut the display short exactly as before this fix.
func splashStatusDrained(st splashStatusState, now time.Time, minDisplay time.Duration) bool {
	return len(st.queue) == 0 && (!st.have || now.Sub(st.shownAt) >= minDisplay)
}

// splashStatusLine renders one FetchStage event (issue 169) as the single
// status line shown under the startup splash's bar, reporting which
// per-source fetch CollectAll/CollectRemote most recently started, finished,
// or failed. have is false before the first event has arrived (e.g. the
// whole fetch is being served from a fresh cache with no live sub-fetch at
// all), in which case the caller renders no status line.
func splashStatusLine(source string, stage FetchStage, have bool) string {
	if !have || source == "" {
		return ""
	}
	switch stage {
	case FetchStarted:
		return fmt.Sprintf("fetching %s...", source)
	case FetchDone:
		return fmt.Sprintf("%s done", source)
	case FetchFailed:
		return fmt.Sprintf("%s failed", source)
	default:
		return ""
	}
}

// splashSourceGlyph maps a probe source name to its brand/source glyph (issue 254).
func splashSourceGlyph(source string) string {
	switch source {
	case "agy":
		return "Λ"
	case "claude":
		return "✳"
	case "codex":
		return "֍"
	case "gemini":
		return "✦"
	case "mic":
		return "●"
	case "gpu", "load":
		return "⚙"
	default:
		return "●"
	}
}

// splashBadgesLine renders the cumulative list of completed/failed source badges
// (issues 252, 254) shown under the single rolling fetch status log line on the
// startup splash screen. Each source renders with its brand-specific glyph
// (e.g. "Λ agy", "✳ claude"), styled in chart-green on success or chart-warm
// on failure, with the source label in dim grey. Returns empty string when no
// badges are present.
func splashBadgesLine(badges []splashBadge) string {
	if len(badges) == 0 {
		return ""
	}
	items := make([]string, len(badges))
	for i, b := range badges {
		glyph := splashSourceGlyph(b.source)
		var color string
		if b.ok {
			color = "chart-green"
		} else {
			color = "chart-warm"
		}
		items[i] = ansiWrap(color, glyph) + " " + ansiWrap("dim-grey", b.source)
	}
	return strings.Join(items, "  ")
}

// buildSplashFrame paints the startup splash (issue 164): a spinner, an
// indeterminate progress bar (rendered via internal/rograph, the same bar
// renderer the rest of the dashboard uses), and a short hint line, centered
// in the terminal. RunWatchWithOptions shows this immediately after entering
// the alt-screen instead of leaving it blank while the first live fetch
// (CollectAll/CollectRemote) is in flight.
//
// animate controls the spinner/bar motion: true while the splash is timing
// out waiting for the fetch, false once Esc has skipped the wait. At that
// point the frame freezes -- it deliberately does not read lastSummary/
// lastProcs/currentHost, which the still-running background fetch goroutine
// owns exclusively until it finishes and calls draw() itself; painting the
// live dashboard from two goroutines at once would race on those vars.
//
// estimate/haveEstimate (issue 168) are the persisted fetch-duration
// estimate for the fetch kind this splash is waiting on, loaded once by
// RunWatchWithOptions before the splash loop starts; they drive
// splashBarPercent's determinate-vs-sweep choice. See splashBarPercent for
// the calculation itself.
//
// statusText (issue 169) is the latest rendered FetchStage event (see
// splashStatusLine) -- e.g. "fetching codex..." then "codex done" -- shown as
// exactly one line under the bar, or omitted entirely when empty (no event
// has arrived yet). It deliberately carries only the single latest event,
// never a scrolling history, per the ticket's "single line" scope.
//
// badgesText (issues 252, 254) is the cumulative completed source badges line
// (see splashBadgesLine) -- e.g. "Λ agy  ✳ claude  ● mic" -- shown directly
// under the status line, or omitted entirely when empty.
func buildSplashFrame(cols, rows int, elapsed time.Duration, animate bool, estimate time.Duration, haveEstimate bool, statusText, badgesText string) screenFrame {
	spinner := splashSpinnerGlyph(elapsed)
	hint := "Esc to skip"
	if !animate {
		spinner = splashSpinnerGlyph(0)
		hint = "waiting for first update..."
	}

	barOpts := watchBarOptions()
	barOpts.Width = splashBarWidth
	pct := splashBarPercent(elapsed, estimate, haveEstimate)
	if !animate {
		pct = 0
	}
	bar := rograph.RenderBar(pct, barOpts)

	title := ansiWrap("bold", spinner+"  harnez usage")
	hintLine := ansiWrap("dim-grey", hint)

	content := []string{title, "", bar}
	if animate {
		if statusText != "" {
			content = append(content, "", ansiWrap("dim-grey", statusText))
		}
		if badgesText != "" {
			if statusText == "" {
				content = append(content, "")
			}
			content = append(content, badgesText)
		}
	}
	content = append(content, "", hintLine)
	top := (rows - len(content)) / 2
	if top < 0 {
		top = 0
	}

	lines := make([]string, 0, rows)
	for i := 0; i < top; i++ {
		lines = append(lines, "")
	}
	for _, line := range content {
		lines = append(lines, centerLine(line, cols))
	}
	return fit(lines, cols, rows)
}
