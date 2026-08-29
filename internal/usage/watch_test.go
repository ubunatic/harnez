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

func TestFormatMemoryLines(t *testing.T) {
	ram := formatSystemMemoryLine(SystemMemory{
		UsedMiB:  8 * 1024,
		TotalMiB: 32 * 1024,
		Ok:       true,
	})
	if got, want := stripANSI(ram), "ram              8.0/32.0G 25%"; got != want {
		t.Errorf("formatSystemMemoryLine = %q, want %q", got, want)
	}

	gpu := formatGPUMemoryLines(GPU{
		MemUsedMiB:   6 * 1024,
		MemTotalMiB:  20 * 1024,
		MemPercent:   30,
		VRAMUsedMiB:  5 * 1024,
		VRAMTotalMiB: 8 * 1024,
		GTTUsedMiB:   1 * 1024,
		GTTTotalMiB:  12 * 1024,
		HaveMem:      true,
		HaveVRAM:     true,
		HaveGTT:      true,
	})
	want := []string{
		"gpu mem          6.0/20.0G 30%",
		"vram/gtt         v5.0/8.0 g1.0/12.0",
	}
	if len(gpu) != len(want) {
		t.Fatalf("formatGPUMemoryLines returned %d lines, want %d: %v", len(gpu), len(want), gpu)
	}
	for i := range want {
		if got := stripANSI(gpu[i]); got != want[i] {
			t.Errorf("formatGPUMemoryLines[%d] = %q, want %q", i, got, want[i])
		}
	}
}

func TestLoadBoxMemoryLineClipsToWidth(t *testing.T) {
	b := wbox{
		title: "\x1b[1m[L]\x1b[0m Load",
		lines: formatGPUMemoryLines(GPU{
			MemUsedMiB:   6 * 1024,
			MemTotalMiB:  20 * 1024,
			MemPercent:   30,
			VRAMUsedMiB:  5 * 1024,
			VRAMTotalMiB: 8 * 1024,
			GTTUsedMiB:   1 * 1024,
			GTTTotalMiB:  12 * 1024,
			HaveMem:      true,
			HaveVRAM:     true,
			HaveGTT:      true,
		}),
		width: minBoxWidth,
	}

	for i, line := range renderWBox(b) {
		if got := visLen(line); got != minBoxWidth {
			t.Errorf("line %d visible width = %d, want %d: %q", i, got, minBoxWidth, stripANSI(line))
		}
	}
	rendered := renderWBox(b)
	if got := stripANSI(rendered[1]); !strings.Contains(got, "gpu mem") {
		t.Errorf("expected total memory line, got %q", got)
	}
	if got := stripANSI(rendered[2]); !strings.Contains(got, "vram/gtt") {
		t.Errorf("expected split memory line, got %q", got)
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

	// In compact layout: 16(label) + 1(sp) + 6(bar) + 1(sp) + 4(percent) + 5(duration) = 33
	// Duration fits when contentW >= 33.
	durationThreshold := 33

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

		// When contentW >= durationThreshold, the compact duration string must appear.
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
					boxWidth, contentW, durationThreshold, "3d5h", box.lines)
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

func TestBuildAllUsageBox(t *testing.T) {
	weeklyReset := testTime.Add(2*24*time.Hour + 8*time.Hour)
	sessionReset := testTime.Add(4*time.Hour + 58*time.Minute)
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{
				AgentID:       "agy",
				Name:          "Antigravity",
				Installed:     true,
				Authenticated: true,
				ModelGroups: []ModelGroup{
					{
						Name: "Gemini Models",
						Windows: []QuotaWindow{
							{Name: "Weekly", UsedPercent: 93, ResetAt: &weeklyReset, DurationLeft: 2*24*time.Hour + 8*time.Hour},
							{Name: "Session (5-hour)", UsedPercent: 3, ResetAt: &sessionReset, DurationLeft: 4*time.Hour + 58*time.Minute},
						},
					},
					{
						Name: "Claude and GPT models",
						Windows: []QuotaWindow{
							{Name: "Weekly", UsedPercent: 35, ResetAt: &weeklyReset, DurationLeft: 6*24*time.Hour + 2*time.Hour},
							{Name: "Session (5-hour)", UsedPercent: 0, ResetAt: &sessionReset, DurationLeft: 4*time.Hour + 58*time.Minute},
						},
					},
				},
			},
			{
				AgentID:       "claude",
				Name:          "Claude Code",
				Installed:     true,
				Authenticated: true,
				Weekly:        &QuotaWindow{Name: "Weekly", UsedPercent: 85, DurationLeft: 8*time.Hour + 51*time.Minute},
				Session:       &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 9, DurationLeft: 4*time.Hour + 51*time.Minute},
			},
			{
				AgentID:       "codex",
				Name:          "OpenAI Codex",
				Installed:     true,
				Authenticated: true,
				Weekly:        &QuotaWindow{Name: "Weekly", UsedPercent: 39, DurationLeft: 5*24*time.Hour + 9*time.Hour},
				Session:       &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 0, DurationLeft: 4*time.Hour + 59*time.Minute},
			},
		},
	}

	box := buildAllUsageBox(summary, 74)
	if !strings.Contains(box.title, "[a]") || !strings.Contains(box.title, "All Usage") {
		t.Fatalf("expected all-usage title, got %q", box.title)
	}
	if len(box.lines) != 4 {
		t.Fatalf("expected 4 all-usage rows, got %d: %v", len(box.lines), box.lines)
	}

	want := []string{
		"Gemini        [███░] 93% 2d8h  [░░░░] 3% 4h58m",
		"Claude/GPT    [█░░░] 35% 6d2h  [░░░░] 0% 4h58m",
		"Claude Code   [███░] 85% 8h51m [░░░░] 9% 4h51m",
		"OpenAI Codex  [█░░░] 39% 5d9h  [░░░░] 0% 4h59m",
	}
	for i := range want {
		if got := stripANSI(box.lines[i]); got != want[i] {
			t.Errorf("line %d = %q, want %q", i, got, want[i])
		}
	}
}

func TestAllUsageBoxNarrowKeepsSecondQuotaVisible(t *testing.T) {
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{
				AgentID:       "claude",
				Name:          "Claude Code",
				Installed:     true,
				Authenticated: true,
				Weekly:        &QuotaWindow{Name: "Weekly", UsedPercent: 85, DurationLeft: 8*time.Hour + 39*time.Minute},
				Session:       &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 11, DurationLeft: 4*time.Hour + 44*time.Minute},
			},
			{
				AgentID:       "codex",
				Name:          "OpenAI Codex",
				Installed:     true,
				Authenticated: true,
				Weekly:        &QuotaWindow{Name: "Weekly", UsedPercent: 41, DurationLeft: 5*24*time.Hour + 8*time.Hour},
				Session:       &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 13},
			},
		},
	}

	box := buildAllUsageBox(summary, 51)
	contentW := box.width - 4
	if len(box.lines) != 2 {
		t.Fatalf("expected 2 all-usage rows, got %d: %v", len(box.lines), box.lines)
	}

	secondBarCol := -1
	for _, line := range box.lines {
		stripped := stripANSI(line)
		if got := visLen(line); got > contentW {
			t.Fatalf("line visible width %d exceeds contentW %d: %q", got, contentW, stripped)
		}
		switch {
		case strings.Contains(stripped, "Claude Code"):
			if !strings.Contains(stripped, "Claude Code   [███░] 85% 8h39m [░░░░] 11% 4h44m") {
				t.Fatalf("expected Claude second bar and percent to remain visible in narrow row: %q", stripped)
			}
		case strings.Contains(stripped, "OpenAI Codex"):
			if !strings.Contains(stripped, "[░░░░] 13%") {
				t.Fatalf("expected Codex second bar and percent to remain visible in narrow row: %q", stripped)
			}
		default:
			t.Fatalf("unexpected all-usage row: %q", stripped)
		}
		col := strings.LastIndex(stripped, "[")
		if col < 0 {
			t.Fatalf("expected second bar in row: %q", stripped)
		}
		if secondBarCol < 0 {
			secondBarCol = col
			continue
		}
		if col != secondBarCol {
			t.Fatalf("expected second bar column %d, got %d in row %q", secondBarCol, col, stripped)
		}
	}
}

func TestCompactWatchSectionsAndAllUsageToggle(t *testing.T) {
	sec := initialWatchSections(WatchOptions{Compact: true, ShowProcesses: true})
	if !sec.AllUsage || !sec.Load {
		t.Fatalf("compact initial sections should show all-usage and load: %+v", sec)
	}
	if sec.Claude || sec.AGY || sec.Codex || sec.History || sec.Processes {
		t.Fatalf("compact initial sections should hide all other panels: %+v", sec)
	}

	if !applyWatchSectionKey(&sec, 'a') || sec.AllUsage {
		t.Fatalf("lowercase a should hide all-usage: %+v", sec)
	}
	if !applyWatchSectionKey(&sec, 'a') || !sec.AllUsage {
		t.Fatalf("lowercase a should show all-usage: %+v", sec)
	}
	if !applyWatchSectionKey(&sec, 'A') {
		t.Fatalf("uppercase A should be handled")
	}
	want := defaultWatchSections()
	if sec != want {
		t.Fatalf("uppercase A reset = %+v, want %+v", sec, want)
	}
}

func TestBuildWatchFrame_CompactShowsOnlyAllUsageAndLoad(t *testing.T) {
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{
				AgentID:       "claude",
				Name:          "Claude Code",
				Installed:     true,
				Authenticated: true,
				Weekly:        &QuotaWindow{Name: "Weekly", UsedPercent: 85, DurationLeft: 8*time.Hour + 51*time.Minute},
				Session:       &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 9, DurationLeft: 4*time.Hour + 51*time.Minute},
			},
		},
	}

	frame := buildWatchFrame(summary, nil, 60*time.Second, compactWatchSections(), 100, 30, true, "", "")
	frameText := strings.Join(frame.lines, "\n")

	if !strings.Contains(frameText, "[a]") || !strings.Contains(frameText, "All Usage") {
		t.Fatalf("expected compact frame to show all-usage box, got:\n%s", frameText)
	}
	if !strings.Contains(frameText, "[L]") || !strings.Contains(frameText, "Load") {
		t.Fatalf("expected compact frame to show load box, got:\n%s", frameText)
	}
	if strings.Contains(frameText, "] Claude Code") {
		t.Fatalf("expected compact frame to hide individual Claude box, got:\n%s", frameText)
	}
	if !strings.Contains(frameText, "Claude Code  [███░] 85% 8h51m [░░░░] 9% 4h51m") {
		t.Fatalf("expected compact all-usage row to keep short-window time at 100 columns, got:\n%s", frameText)
	}
	if !strings.Contains(frameText, "[a]usage") || !strings.Contains(frameText, "[⇧a]ll") {
		t.Fatalf("expected footer to expose [a]usage and shifted [⇧a]ll, got:\n%s", frameText)
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

// TestBuildWatchFrame_SelfHidesAgentsWithoutUsageData covers issue 083: an
// agent with real recorded usage renders its box, an agent with no usage
// data (Installed: false) is filtered out entirely — no empty box, and no
// "hidden: [x]" toggle hint either, since the user never hid it.
func TestBuildWatchFrame_SelfHidesAgentsWithoutUsageData(t *testing.T) {
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true},
			{AgentID: "agy", Name: "Antigravity", Installed: false},
			{AgentID: "codex", Name: "OpenAI Codex", Installed: false},
		},
	}

	sec := defaultWatchSections()
	frame := buildWatchFrame(summary, nil, 60*time.Second, sec, 100, 30, false, "", "")
	frameText := strings.Join(frame.lines, "\n")

	if !strings.Contains(frameText, "Claude Code") {
		t.Errorf("expected frame to contain the Claude box (has usage data), got:\n%s", frameText)
	}
	if strings.Contains(frameText, "Antigravity") {
		t.Errorf("expected frame to omit the AGY box (no usage data), got:\n%s", frameText)
	}
	if strings.Contains(frameText, "OpenAI Codex") {
		t.Errorf("expected frame to omit the Codex box (no usage data), got:\n%s", frameText)
	}
	if strings.Contains(frameText, "[G]") || strings.Contains(frameText, "[O]") {
		t.Errorf("expected no 'hidden: [G]'/'[O]' toggle hints for agents with no usage data, got:\n%s", frameText)
	}
}

// TestBuildWatchFrame_AllAgentsAbsent covers issue 083's "don't render a
// silently empty screen" requirement for the --watch TUI: when no agent has
// any real recorded usage, the frame must say so explicitly.
func TestBuildWatchFrame_AllAgentsAbsent(t *testing.T) {
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{AgentID: "claude", Name: "Claude Code", Installed: false},
			{AgentID: "agy", Name: "Antigravity", Installed: false},
			{AgentID: "codex", Name: "OpenAI Codex", Installed: false},
		},
	}

	sec := defaultWatchSections()
	sec.Processes = false
	frame := buildWatchFrame(summary, nil, 60*time.Second, sec, 100, 30, false, "", "")
	frameText := strings.Join(frame.lines, "\n")

	// Box titles render as "[X] <Name>"; check for that rather than the bare
	// name, since the explanatory fallback message legitimately mentions the
	// agent names in prose.
	for _, name := range []string{"Claude Code", "Antigravity", "OpenAI Codex"} {
		if strings.Contains(frameText, "] "+name) {
			t.Errorf("expected frame to omit %s box (no usage data anywhere), got:\n%s", name, frameText)
		}
	}
	if !strings.Contains(frameText, "no agent usage detected") {
		t.Errorf("expected frame to explain that no agent has recorded usage, got:\n%s", frameText)
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
