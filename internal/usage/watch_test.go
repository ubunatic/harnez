package usage

import (
	"strings"
	"testing"
	"time"
)

// TestGridColumnsFitsWithGutters guards the layout bug that broke `--watch`:
// boxWidth was computed as total/columns, ignoring the boxGap columns that
// combineRow inserts between panels. At 100 columns that produced two 50-wide
// boxes plus a gutter = 101 cells, wrapping every box line onto a second
// physical row and corrupting the whole frame.
func TestGridColumnsFitsWithGutters(t *testing.T) {
	for usable := minTerminalWidth; usable <= 200; usable++ {
		for panels := 1; panels <= 6; panels++ {
			columns, boxWidth := gridColumns(usable, panels)
			if columns < 1 || columns > panels {
				t.Fatalf("usable=%d panels=%d: columns=%d out of range", usable, panels, columns)
			}
			if columns == 1 {
				continue
			}
			total := columns*boxWidth + (columns-1)*boxGap
			if total > usable {
				t.Errorf("usable=%d panels=%d: row width %d exceeds usable (columns=%d boxWidth=%d)",
					usable, panels, total, columns, boxWidth)
			}
		}
	}
}

// TestRenderWBoxWidth checks a rendered panel occupies exactly its declared
// width on every line, so side-by-side rows line up.
func TestRenderWBoxWidth(t *testing.T) {
	b := wbox{
		title: "\x1b[1m[C]\x1b[0m Claude Code",
		lines: []string{"short", strings.Repeat("x", 200), ""},
		width: 48,
	}
	for i, l := range renderWBox(b) {
		if got := visLen(l); got != b.width {
			t.Errorf("line %d: visible width %d, want %d (%q)", i, got, b.width, stripANSI(l))
		}
	}
}

// TestFitClipsToViewport ensures a frame can never exceed the terminal box.
// An overflowing frame scrolls the terminal, after which the `\x1b[H` starting
// each redraw no longer addresses the row the program assumes.
func TestFitClipsToViewport(t *testing.T) {
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = strings.Repeat("y", 300)
	}
	f := fit(lines, 60, 10)
	if len(f.lines) != 10 {
		t.Fatalf("got %d lines, want 10", len(f.lines))
	}
	for i, l := range f.lines {
		if visLen(l) > 60 {
			t.Errorf("line %d: visible width %d exceeds 60", i, visLen(l))
		}
	}
}

// TestPaintClearsEachLine guards the stale-content bug: `\x1b[H` + content +
// `\x1b[J` clears only *below* the new frame, so shrinking the frame left the
// previous, larger frame's tails on every shortened or blank line. Every line
// must therefore carry its own erase-to-end-of-line.
func TestPaintClearsEachLine(t *testing.T) {
	var sb strings.Builder
	fit([]string{"a", "", "c"}, 20, 5).paint(&sb)
	out := sb.String()

	if !strings.HasPrefix(out, "\x1b[H") {
		t.Errorf("frame must start by homing the cursor, got %q", out)
	}
	if n := strings.Count(out, "\x1b[K"); n != 3 {
		t.Errorf("got %d erase-to-end-of-line sequences, want one per line (3): %q", n, out)
	}
	if !strings.HasSuffix(out, "\x1b[J") {
		t.Errorf("frame must end by erasing below the content, got %q", out)
	}
	// A trailing newline after the final line would scroll a full-height frame.
	if strings.HasSuffix(strings.TrimSuffix(out, "\x1b[J"), "\n") {
		t.Errorf("frame must not end with a newline: %q", out)
	}
}

// testTime is a fixed timestamp used by watch_test helpers.
var testTime = time.Date(2026, 8, 17, 22, 0, 0, 0, time.UTC)

// TestBuildAgentBoxBarFitsContentW verifies that the adaptive progress bar
// never causes a quota line to exceed the box's content width (width-4),
// and that the duration string is preserved when the box is wide enough.
func TestBuildAgentBoxBarFitsContentW(t *testing.T) {
	// 3 days + 5 hours → FormatDuration → "3d 5h"; resetStr = " · 3d 5h" (visLen=9)
	dur := 3*24*time.Hour + 5*time.Hour
	resetAt := testTime.Add(dur)
	agent := AgentUsage{
		AgentID:       "claude",
		Name:          "Claude Code",
		Installed:     true,
		Authenticated: true,
		Account:       "u***@example.com",
		Session: &QuotaWindow{
			Name:         "Session",
			UsedPercent:  75.0,
			ResetAt:      &resetAt,
			DurationLeft: dur,
		},
	}

	// Compute the threshold above which duration must appear.
	// Layout: 16(label) + 1(sp) + (barW+2)(bar) + 1(sp) + 6(percent) + visLen(resetStr)
	// = 26 + barW + visLen(resetStr)
	// Duration fits when barW >= 1: contentW >= 26 + 1 + visLen(resetStr)
	resetStr := " · " + FormatDuration(dur)
	durationThreshold := 26 + 1 + visLen(resetStr) // contentW at which duration must appear

	for boxWidth := minBoxWidth; boxWidth <= 80; boxWidth++ {
		box := buildAgentBox(agent, agentRate{}, boxWidth, false, false)
		contentW := boxWidth - 4

		for _, l := range box.lines {
			lw := visLen(l)
			if lw > contentW {
				t.Errorf("boxWidth=%d: line visible width %d exceeds contentW %d: %q",
					boxWidth, lw, contentW, stripANSI(l))
			}
		}

		// When contentW >= durationThreshold, there is room for a 1-char bar + duration.
		// The duration string must appear in one of the quota lines.
		if contentW >= durationThreshold {
			found := false
			for _, l := range box.lines {
				if strings.Contains(l, "3d") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("boxWidth=%d (contentW=%d, threshold=%d): expected duration %q in quota line, lines=%v",
					boxWidth, contentW, durationThreshold, resetStr, box.lines)
			}
		}
	}
}

func TestBuildHistoryBox(t *testing.T) {
	tempDir := t.TempDir()
	_ = AppendHistory(tempDir, UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{
				AgentID:       "claude",
				Name:          "Claude Code",
				Installed:     true,
				Authenticated: true,
				Tokens: &TokenBreakdown{
					TotalTokens: 1000,
				},
			},
		},
	})
	_ = AppendHistory(tempDir, UsageSummary{
		Timestamp: testTime.Add(10 * time.Minute),
		Agents: []AgentUsage{
			{
				AgentID:       "claude",
				Name:          "Claude Code",
				Installed:     true,
				Authenticated: true,
				Tokens: &TokenBreakdown{
					TotalTokens: 2500,
				},
			},
		},
	})

	box := buildHistoryBox("", tempDir, 40)
	if !strings.Contains(box.title, "[H]") || !strings.Contains(box.title, "History") {
		t.Errorf("expected box title to contain [H] and History, got %q", box.title)
	}

	rendered := strings.Join(box.lines, "\n")
	if !strings.Contains(rendered, "1 file") {
		t.Errorf("expected box to contain '1 file', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "+1,500 used") {
		t.Errorf("expected box to contain '+1,500 used', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "9,000/hr") {
		t.Errorf("expected box to contain '9,000/hr', got:\n%s", rendered)
	}
}

func TestBuildWatchFrame_HistoryHeaderAnd4Boxes(t *testing.T) {
	tempDir := t.TempDir()
	_ = AppendHistory(tempDir, UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true},
		},
	})

	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true},
			{AgentID: "agy", Name: "Antigravity", Installed: true, Authenticated: true},
			{AgentID: "codex", Name: "OpenAI Codex", Installed: true, Authenticated: true},
		},
	}

	sec := defaultWatchSections()
	frame := buildWatchFrame(summary, nil, 60*time.Second, sec, 100, 30, false, "", tempDir)
	frameText := strings.Join(frame.lines, "\n")

	// Verify header contains history counter
	if !strings.Contains(frameText, "history: 1 file") {
		t.Errorf("expected header to contain 'history: 1 file', got:\n%s", frameText)
	}

	// Verify all 4 boxes exist: Claude, AGY, Codex, and History
	if !strings.Contains(frameText, "[C]") || !strings.Contains(frameText, "Claude Code") {
		t.Errorf("expected frame to contain Claude box")
	}
	if !strings.Contains(frameText, "[G]") || !strings.Contains(frameText, "Antigravity") {
		t.Errorf("expected frame to contain AGY box")
	}
	if !strings.Contains(frameText, "[O]") || !strings.Contains(frameText, "OpenAI Codex") {
		t.Errorf("expected frame to contain Codex box")
	}
	if !strings.Contains(frameText, "[H]") || !strings.Contains(frameText, "History") {
		t.Errorf("expected frame to contain History box")
	}

	// Verify hiding history works
	sec.History = false
	frameHidden := buildWatchFrame(summary, nil, 60*time.Second, sec, 100, 30, false, "", tempDir)
	frameHiddenText := strings.Join(frameHidden.lines, "\n")
	if strings.Contains(frameHiddenText, "+0 used") || strings.Contains(frameHiddenText, "used ·") {
		t.Errorf("expected history box content to be hidden when sec.History is false")
	}
	if !strings.Contains(frameHiddenText, "hidden:") || !strings.Contains(frameHiddenText, "[H]") || !strings.Contains(frameHiddenText, "[P]") {
		t.Errorf("expected header to show hidden with [H] and [P], got:\n%s", frameHiddenText)
	}
}

func TestBuildProcessesBox(t *testing.T) {
	box := buildProcessesBox(40, nil)
	if !strings.Contains(box.title, "[P]") || !strings.Contains(box.title, "Processes") {
		t.Errorf("expected box title to contain [P] and Processes, got %q", box.title)
	}
	if len(box.lines) != 2 {
		t.Fatalf("expected 2 lines in processes box, got %d: %v", len(box.lines), box.lines)
	}
	if !strings.Contains(box.lines[0], "active process") {
		t.Errorf("expected line 0 to contain 'active process', got %q", box.lines[0])
	}
	if !strings.Contains(box.lines[1], "claude:") || !strings.Contains(box.lines[1], "agy:") || !strings.Contains(box.lines[1], "codex:") {
		t.Errorf("expected line 1 to contain per-agent breakdown, got %q", box.lines[1])
	}

	// With explicit counts
	customCounts := &AgentProcessCount{Claude: 3, AGY: 2, Codex: 1}
	boxCustom := buildProcessesBox(40, customCounts)
	if !strings.Contains(boxCustom.lines[0], "6 active processes") {
		t.Errorf("expected 6 active processes, got %q", boxCustom.lines[0])
	}
	if !strings.Contains(boxCustom.lines[1], "claude: 3  agy: 2  codex: 1") {
		t.Errorf("expected custom counts line, got %q", boxCustom.lines[1])
	}
}

func TestBuildWatchFrame_ProcessesBox(t *testing.T) {
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true},
		},
	}

	// Hidden by default
	sec := defaultWatchSections()
	if sec.Processes {
		t.Errorf("expected sec.Processes to be false by default")
	}
	frameDefault := buildWatchFrame(summary, nil, 60*time.Second, sec, 100, 30, false, "", "")
	frameDefaultText := strings.Join(frameDefault.lines, "\n")
	if !strings.Contains(frameDefaultText, "hidden: [P]") {
		t.Errorf("expected hidden [P] in header, got:\n%s", frameDefaultText)
	}
	if strings.Contains(frameDefaultText, "Processes") {
		t.Errorf("expected Processes box to be hidden by default")
	}

	// Toggled visible
	sec.Processes = true
	frameVisible := buildWatchFrame(summary, nil, 60*time.Second, sec, 100, 30, false, "", "")
	frameVisibleText := strings.Join(frameVisible.lines, "\n")
	if strings.Contains(frameVisibleText, "hidden: [P]") {
		t.Errorf("expected [P] to not be in hidden hint when sec.Processes is true, got:\n%s", frameVisibleText)
	}
	if !strings.Contains(frameVisibleText, "[P]") || !strings.Contains(frameVisibleText, "Processes") {
		t.Errorf("expected Processes box to be visible when sec.Processes is true, got:\n%s", frameVisibleText)
	}
	if !strings.Contains(frameVisibleText, "claude:") {
		t.Errorf("expected Processes box content in frame, got:\n%s", frameVisibleText)
	}
}

func TestBuildWatchFrame_RemoteHost(t *testing.T) {
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true},
		},
	}

	sec := defaultWatchSections()
	frame := buildWatchFrame(summary, nil, 60*time.Second, sec, 100, 30, true, "", "", WatchOptions{
		Host: "remote-worker-1",
	})
	frameText := strings.Join(frame.lines, "\n")

	if !strings.Contains(frameText, "Agentic usage (@remote-worker-1)") {
		t.Errorf("expected remote host in header, got:\n%s", frameText)
	}
	if !strings.Contains(frameText, "[r]emote") {
		t.Errorf("expected [r]emote in footer, got:\n%s", frameText)
	}
}


