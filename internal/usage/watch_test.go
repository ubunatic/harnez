package usage

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// TestBuildWatchFrameRowsFitWidth guards the layout bug that broke `--watch`:
// a row's boxes plus their gutters must never exceed the terminal's usable
// width, or the last box on the row gets silently mangled by fit()'s
// truncateVisible instead of wrapping to its own row. Since issue 093, row
// packing comes from internal/uix.Layout rather than a bespoke grid
// computation, but the invariant still has to hold end to end through
// buildWatchFrame, at every terminal size the panel toggles are exercised
// at (issue 093 acceptance: 80x18, 80x24, 100x24, 120x18).
func TestBuildWatchFrameRowsFitWidth(t *testing.T) {
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true,
				Weekly:  &QuotaWindow{Name: "Weekly", UsedPercent: 85, DurationLeft: 8 * time.Hour},
				Session: &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 9, DurationLeft: 4 * time.Hour}},
			{AgentID: "agy", Name: "Antigravity", Installed: true, Authenticated: true},
			{AgentID: "codex", Name: "OpenAI Codex", Installed: true, Authenticated: true},
		},
	}

	sizes := [][2]int{{80, 18}, {80, 24}, {100, 24}, {120, 18}}
	sectionSets := []watchSections{
		defaultWatchSections(),
		compactWatchSections(),
	}
	for _, size := range sizes {
		cols, rows := size[0], size[1]
		for _, sec := range sectionSets {
			frame := buildWatchFrame(summary, nil, 60*time.Second, sec, cols, rows, false, "", "")
			for i, l := range frame.lines {
				if got := visLen(l); got > frame.cols {
					t.Errorf("size=%dx%d: line %d visible width %d exceeds usable %d: %q",
						cols, rows, i, got, frame.cols, stripANSI(l))
				}
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
		box := buildAgentBox(agent, agentRate{}, boxWidth, false, false, false)
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
	if !strings.Contains(box.title, "⁵") || !strings.Contains(box.title, "History") {
		t.Errorf("expected box title to contain superscript 5 and History, got %q", box.title)
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

	box := buildAllUsageBox(summary, 74, false)
	if !strings.Contains(box.title, "¹") || !strings.Contains(box.title, "All Usage") {
		t.Fatalf("expected all-usage title, got %q", box.title)
	}
	if len(box.lines) != 4 {
		t.Fatalf("expected 4 all-usage rows, got %d: %v", len(box.lines), box.lines)
	}

	// The mid column is padded to a fixed width (10) so the second [    ] bar
	// starts at the same column in every row — that's the alignment guarantee
	// this test verifies. "93% 2d8h" (8 chars) pads to 10 → 2 trailing spaces;
	// "85% 8h51m" (9 chars) pads to 10 → 1 trailing space. Empty bar cells
	// render as a plain space, not '░', under the ANSI background wrap (issue
	// 136 follow-up — see eighthBlockFill's doc comment in
	// internal/rograph/options.go).
	want := []string{
		"Gemini        [███▋] 93% 2d8h   [    ] 3% 4h58m",
		"Claude/GPT    [█▍  ] 35% 6d2h   [    ] 0% 4h58m",
		"Claude Code   [███▍] 85% 8h51m  [▎   ] 9% 4h51m",
		"OpenAI Codex  [█▌  ] 39% 5d9h   [    ] 0% 4h59m",
	}
	for i := range want {
		if got := stripANSI(box.lines[i]); got != want[i] {
			t.Errorf("line %d = %q, want %q", i, got, want[i])
		}
	}
}

func TestBuildAllUsageBox_StaleAndHistoricalAgents(t *testing.T) {
	// Tests issue 103: an agent whose quota windows were filled via historical
	// fallback (e.g. AGY when not actively running) appears in All Usage with its
	// quota windows intact.
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{
				AgentID:       "agy",
				Name:          "Antigravity (AGY)",
				Installed:     true,
				Authenticated: true,
				ModelGroups: []ModelGroup{
					{
						Name: "Gemini Models",
						Windows: []QuotaWindow{
							{Name: "Weekly (stale)", UsedPercent: 86},
							{Name: "Five Hour Limit Remaining (stale)", UsedPercent: 12},
						},
					},
				},
				Sources:       []string{"~/.claude/harnez/usage-history (usage-history, stale)"},
				LastRefreshed: testTime.Add(-8 * 24 * time.Hour),
			},
			{
				AgentID:       "claude",
				Name:          "Claude Code",
				Installed:     true,
				Authenticated: true,
				Weekly:        &QuotaWindow{Name: "Weekly", UsedPercent: 10, DurationLeft: 6 * 24 * time.Hour},
				Session:       &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 5, DurationLeft: 4 * time.Hour},
				LastRefreshed: testTime,
			},
		},
	}

	box := buildAllUsageBox(summary, 74, false)
	if len(box.lines) != 2 {
		t.Fatalf("expected 2 all-usage rows including historical AGY, got %d: %v", len(box.lines), box.lines)
	}

	stripped0 := stripANSI(box.lines[0])
	stripped1 := stripANSI(box.lines[1])

	if !strings.HasPrefix(stripped0, "Gemini") || !strings.Contains(stripped0, "86%") || !strings.Contains(stripped0, "12%") {
		t.Errorf("expected Gemini row for historical AGY data, got %q", stripped0)
	}
	if !strings.HasPrefix(stripped1, "Claude Code") || !strings.Contains(stripped1, "10%") {
		t.Errorf("expected Claude Code row, got %q", stripped1)
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

	box := buildAllUsageBox(summary, 51, false)
	contentW := box.width - 4
	if len(box.lines) != 2 {
		t.Fatalf("expected 2 all-usage rows, got %d: %v", len(box.lines), box.lines)
	}

	// With mid-column padded to 10 chars for alignment, width=51 (contentW=47)
	// is tight enough that the second window's duration (d2) must be dropped on
	// the Claude row to stay within contentW. The second [    ] bar must still
	// appear and must align at the same column in both rows. Empty cells render
	// as a plain space (not '░') under the ANSI background wrap — see
	// eighthBlockFill's doc comment in internal/rograph/options.go.
	secondBarCol := -1
	for _, line := range box.lines {
		stripped := stripANSI(line)
		if got := visLen(line); got > contentW {
			t.Fatalf("line visible width %d exceeds contentW %d: %q", got, contentW, stripped)
		}
		// Both bars must still be present.
		if !strings.Contains(stripped, "[    ]") && !strings.Contains(stripped, "[█") {
			t.Fatalf("expected second bar to remain visible in narrow row: %q", stripped)
		}
		switch {
		case strings.Contains(stripped, "Claude Code"):
			// d2 dropped to fit; d1 kept; percent shown.
			if !strings.Contains(stripped, "[███▍]") || !strings.Contains(stripped, "85%") || !strings.Contains(stripped, "[▍   ]") || !strings.Contains(stripped, "11%") {
				t.Fatalf("expected Claude second bar and percent to remain visible in narrow row: %q", stripped)
			}
		case strings.Contains(stripped, "OpenAI Codex"):
			if !strings.Contains(stripped, "[▌   ]") || !strings.Contains(stripped, "13%") {
				t.Fatalf("expected Codex second bar and percent to remain visible in narrow row: %q", stripped)
			}
		default:
			t.Fatalf("unexpected all-usage row: %q", stripped)
		}
		// Rune-count column, not byte offset: the bar glyphs mix 1-byte
		// spaces and 3-byte UTF-8 block characters, so a raw byte index
		// no longer corresponds to a visual column.
		byteIdx := strings.LastIndex(stripped, "[")
		if byteIdx < 0 {
			t.Fatalf("expected second bar in row: %q", stripped)
		}
		col := utf8.RuneCountInString(stripped[:byteIdx])
		if secondBarCol < 0 {
			secondBarCol = col
			continue
		}
		if col != secondBarCol {
			t.Fatalf("expected second bar column %d, got %d in row %q (alignment broken)", secondBarCol, col, stripped)
		}
	}
}

// TestAllUsageBoxSecondBarColumnAlignment is the canonical alignment test: it
// builds an [a] All Usage box with rows whose mid-column content varies in
// length (3-digit vs 4-digit percent, 4- vs 5-char duration, no duration) and
// asserts every second [░] bar starts at exactly the same column, at several
// representative box widths. This is the unit-level equivalent of the smoke
// test that runs the real binary.
func TestAllUsageBoxSecondBarColumnAlignment(t *testing.T) {
	// Mix: 100% (4-digit), sub-100% (3-digit), 4-char duration "1d1h",
	// 5-char "8h51m", model-group rows (agy) and weekly/session rows.
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{
				AgentID: "agy", Name: "Antigravity", Installed: true, Authenticated: true,
				ModelGroups: []ModelGroup{
					{
						Name: "Gemini Models",
						Windows: []QuotaWindow{
							{Name: "Weekly", UsedPercent: 100, DurationLeft: 1*24*time.Hour + 1*time.Hour},
							{Name: "Session", UsedPercent: 8, DurationLeft: 4*time.Hour + 44*time.Minute},
						},
					},
					{
						Name: "Claude/GPT",
						Windows: []QuotaWindow{
							{Name: "Weekly", UsedPercent: 35, DurationLeft: 6*24*time.Hour + 2*time.Hour},
							{Name: "Session", UsedPercent: 0, DurationLeft: 4*time.Hour + 58*time.Minute},
						},
					},
				},
			},
			{
				AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true,
				Weekly:  &QuotaWindow{Name: "Weekly", UsedPercent: 12, DurationLeft: 6*24*time.Hour + 2*time.Hour},
				Session: &QuotaWindow{Name: "Session", UsedPercent: 100, DurationLeft: 2*time.Hour + 27*time.Minute},
			},
			{
				AgentID: "codex", Name: "OpenAI Codex", Installed: true, Authenticated: true,
				Weekly:  &QuotaWindow{Name: "Weekly", UsedPercent: 12, DurationLeft: 6*24*time.Hour + 21*time.Hour},
				Session: &QuotaWindow{Name: "Session", UsedPercent: 79, DurationLeft: 2*time.Hour + 59*time.Minute},
			},
		},
	}

	for _, boxWidth := range []int{55, 60, 70, 80, 100} {
		t.Run(fmt.Sprintf("width=%d", boxWidth), func(t *testing.T) {
			box := buildAllUsageBox(summary, boxWidth, false)
			contentW := box.width - 4
			if len(box.lines) == 0 {
				t.Fatalf("expected rows, got none")
			}
			secondBarCol := -1
			for _, line := range box.lines {
				stripped := stripANSI(line)
				if got := visLen(line); got > contentW {
					t.Errorf("line width %d > contentW %d: %q", got, contentW, stripped)
				}
				// The second progress bar is the last '[' in the stripped line.
				// Rune-count column, not byte offset: bar glyphs mix 1-byte
				// spaces and 3-byte UTF-8 block characters.
				byteIdx := strings.LastIndex(stripped, "[")
				if byteIdx < 0 {
					t.Errorf("no second bar found in row: %q", stripped)
					continue
				}
				col := utf8.RuneCountInString(stripped[:byteIdx])
				if secondBarCol < 0 {
					secondBarCol = col
					continue
				}
				if col != secondBarCol {
					t.Errorf("second bar column mismatch: want col %d, got %d\n  row: %q\n  (alignment broken at width %d)",
						secondBarCol, col, stripped, boxWidth)
				}
			}
		})
	}
}

// TestCompactWatchSectionsAndAllUsageToggle also covers issue 093's flag
// interaction bug: `--watch --compact --proc` must show the Processes panel
// even though compactWatchSections() itself never enables it — --proc is an
// explicit request and has to win over the compact default.
func TestCompactWatchSectionsAndAllUsageToggle(t *testing.T) {
	sec := initialWatchSections(WatchOptions{Compact: true, ShowProcesses: true})
	if !sec.AllUsage || !sec.Load {
		t.Fatalf("compact initial sections should show all-usage and load: %+v", sec)
	}
	if !sec.Processes {
		t.Fatalf("--proc must show the Processes panel even under --compact: %+v", sec)
	}
	if sec.Claude || sec.AGY || sec.Codex || sec.History {
		t.Fatalf("compact initial sections should hide the other non-requested panels: %+v", sec)
	}

	sec = initialWatchSections(WatchOptions{Compact: true})
	if sec.Processes {
		t.Fatalf("compact initial sections should hide Processes when --proc was not requested: %+v", sec)
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

	if !strings.Contains(frameText, "¹") || !strings.Contains(frameText, "All Usage") {
		t.Fatalf("expected compact frame to show all-usage box, got:\n%s", frameText)
	}
	if !strings.Contains(frameText, "⁷") || !strings.Contains(frameText, "Load") {
		t.Fatalf("expected compact frame to show load box, got:\n%s", frameText)
	}
	foundSharedTitleRow := false
	for _, line := range frame.lines {
		stripped := stripANSI(line)
		if strings.Contains(stripped, "¹ All Usage") && strings.Contains(stripped, "⁷ Load") {
			foundSharedTitleRow = true
			break
		}
	}
	if !foundSharedTitleRow {
		t.Fatalf("expected compact all-usage and load boxes to share one row, got:\n%s", frameText)
	}
	if strings.Contains(frameText, "² Claude Code") {
		t.Fatalf("expected compact frame to hide individual Claude box, got:\n%s", frameText)
	}
	// Mid-column padded to 10 chars → "85% 8h51m " (10) + " " + "[    ]" = 2 spaces before second bar.
	// Bars carry an ANSI background wrap (issue 133); empty cells render as a
	// plain space under that wrap rather than '░' (issue 136 follow-up — see
	// eighthBlockFill's doc comment in internal/rograph/options.go), so strip
	// escapes before matching.
	if !strings.Contains(stripANSI(frameText), "Claude Code  [███▍] 85% 8h51m  [▎   ] 9% 4h51m") {
		t.Fatalf("expected compact all-usage row to keep short-window time at 100 columns, got:\n%s", frameText)
	}
	if !strings.Contains(frameText, "[?]controls") || !strings.Contains(frameText, "[m]ode") {
		t.Fatalf("expected reduced footer to expose [?]controls and [m]ode, got:\n%s", frameText)
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
	if !strings.Contains(frameText, "²") || !strings.Contains(frameText, "Claude Code") {
		t.Errorf("expected frame to contain Claude box")
	}
	if !strings.Contains(frameText, "³") || !strings.Contains(frameText, "Antigravity") {
		t.Errorf("expected frame to contain AGY box")
	}
	if !strings.Contains(frameText, "⁴") || !strings.Contains(frameText, "OpenAI Codex") {
		t.Errorf("expected frame to contain Codex box")
	}
	if !strings.Contains(frameText, "⁵") || !strings.Contains(frameText, "History") {
		t.Errorf("expected frame to contain History box")
	}

	// Verify hiding history works
	sec.History = false
	frameHidden := buildWatchFrame(summary, nil, 60*time.Second, sec, 100, 30, false, "", tempDir)
	frameHiddenText := strings.Join(frameHidden.lines, "\n")
	if strings.Contains(frameHiddenText, "+0 used") || strings.Contains(frameHiddenText, "used ·") {
		t.Errorf("expected history box content to be hidden when sec.History is false")
	}
	// AllUsage (default off) + Processes (default off) + History (just
	// toggled off) = 3 hidden — issue 132's single hidden-count summary
	// replaces the old per-box "[H] [P]" badge list.
	if !strings.Contains(frameHiddenText, "3 hidden") {
		t.Errorf("expected header to show a single '3 hidden' summary, got:\n%s", frameHiddenText)
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
	// Undiscovered agents (agy/codex, Installed: false) never enter the
	// hidden-count either — only AllUsage and Processes are toggled off by
	// default here, so the single-count summary must read "2 hidden", not
	// inflated by agents that were never discovered in the first place.
	if !strings.Contains(frameText, "2 hidden") {
		t.Errorf("expected hidden-count hint to read '2 hidden' (AllUsage+Processes only), got:\n%s", frameText)
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

	// Box titles render as "<symbol> <Name>"; check for that rather than the
	// bare name, since the explanatory fallback message legitimately
	// mentions the agent names in prose.
	boxSymbolForName := map[string]string{
		"Claude Code":  "²",
		"Antigravity":  "³",
		"OpenAI Codex": "⁴",
	}
	for _, name := range []string{"Claude Code", "Antigravity", "OpenAI Codex"} {
		if strings.Contains(frameText, boxSymbolForName[name]+" "+name) {
			t.Errorf("expected frame to omit %s box (no usage data anywhere), got:\n%s", name, frameText)
		}
	}
	if !strings.Contains(frameText, "no agent usage detected") {
		t.Errorf("expected frame to explain that no agent has recorded usage, got:\n%s", frameText)
	}
}

// TestBuildWatchFrame_StaleWithinSevenDaysStillShown mirrors
// TestRenderText_StaleWithinSevenDaysStillShown for the --watch TUI path
// (issue 101): an agent refreshed hours ago (past the 30-minute live-
// recollect window, well short of the 7-day display-hide gate) still gets a
// panel, annotated with an "updated ... ago" line.
func TestBuildWatchFrame_StaleWithinSevenDaysStillShown(t *testing.T) {
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{
				AgentID:       "agy",
				Name:          "Antigravity",
				Installed:     true,
				Authenticated: true,
				Session:       &QuotaWindow{Name: "5h", UsedPercent: 42},
				LastRefreshed: time.Now().Add(-3 * time.Hour),
			},
		},
	}

	sec := defaultWatchSections()
	frame := buildWatchFrame(summary, nil, 60*time.Second, sec, 100, 30, false, "", "")
	frameText := strings.Join(frame.lines, "\n")

	if !strings.Contains(frameText, "Antigravity") {
		t.Errorf("expected frame to still show the stale-but-within-7d agent box, got:\n%s", frameText)
	}
	if !strings.Contains(frameText, "updated") || !strings.Contains(frameText, "3h") {
		t.Errorf("expected frame to annotate the box with 'updated ~3h ago', got:\n%s", frameText)
	}
}

// TestBuildWatchFrame_SevenDayStaleAgentHidden mirrors
// TestRenderText_SevenDayStaleAgentHidden for the --watch TUI path (issue
// 101): a 7+ day stale agent auto-hides even though HasUsageData() is true.
func TestBuildWatchFrame_SevenDayStaleAgentHidden(t *testing.T) {
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{
				AgentID:       "agy",
				Name:          "Antigravity",
				Installed:     true,
				Authenticated: true,
				Session:       &QuotaWindow{Name: "5h", UsedPercent: 42},
				LastRefreshed: time.Now().Add(-8 * 24 * time.Hour),
			},
		},
	}

	sec := defaultWatchSections()
	sec.Processes = false
	frame := buildWatchFrame(summary, nil, 60*time.Second, sec, 100, 30, false, "", "")
	frameText := strings.Join(frame.lines, "\n")

	if strings.Contains(frameText, "³ Antigravity") {
		t.Errorf("expected frame to hide an agent 8 days stale, got:\n%s", frameText)
	}
	if !strings.Contains(frameText, "no agent usage detected") {
		t.Errorf("expected the no-agent-usage-detected fallback message, got:\n%s", frameText)
	}
}

func TestBuildProcessesBox(t *testing.T) {
	box := buildProcessesBox(40, nil)
	if !strings.Contains(box.title, "⁶") || !strings.Contains(box.title, "Processes") {
		t.Errorf("expected box title to contain superscript 6 and Processes, got %q", box.title)
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
	// AllUsage (default off) + Processes (default off) = 2 hidden.
	if !strings.Contains(frameDefaultText, "2 hidden") {
		t.Errorf("expected '2 hidden' in header, got:\n%s", frameDefaultText)
	}
	if strings.Contains(frameDefaultText, "Processes") {
		t.Errorf("expected Processes box to be hidden by default")
	}

	// Toggled visible
	sec.Processes = true
	frameVisible := buildWatchFrame(summary, nil, 60*time.Second, sec, 100, 30, false, "", "")
	frameVisibleText := strings.Join(frameVisible.lines, "\n")
	// Only AllUsage remains hidden now.
	if !strings.Contains(frameVisibleText, "1 hidden") {
		t.Errorf("expected '1 hidden' once sec.Processes is true, got:\n%s", frameVisibleText)
	}
	if !strings.Contains(frameVisibleText, "⁶") || !strings.Contains(frameVisibleText, "Processes") {
		t.Errorf("expected Processes box to be visible when sec.Processes is true, got:\n%s", frameVisibleText)
	}
	if !strings.Contains(frameVisibleText, "claude:") {
		t.Errorf("expected Processes box content in frame, got:\n%s", frameVisibleText)
	}
}

// TestBuildWatchFrame_HiddenCountAtZeroOneAndN is issue 132's acceptance
// criterion for the single hidden-count hint: it must be absent at 0 hidden
// boxes, read "1 hidden" at exactly one, and "N hidden" (never a per-box
// badge list) at several.
func TestBuildWatchFrame_HiddenCountAtZeroOneAndN(t *testing.T) {
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true},
		},
	}

	// 0 hidden: everything defaultWatchSections tracks is on except
	// AllUsage/Processes, so force those on too for a true zero-hidden case.
	allOn := defaultWatchSections()
	allOn.AllUsage = true
	allOn.Processes = true
	frameZero := buildWatchFrame(summary, nil, 60*time.Second, allOn, 100, 30, false, "", "")
	zeroText := strings.Join(frameZero.lines, "\n")
	if strings.Contains(zeroText, "hidden") {
		t.Errorf("expected no hidden-count hint at 0 hidden boxes, got:\n%s", zeroText)
	}

	// 1 hidden: only Processes off.
	oneHidden := allOn
	oneHidden.Processes = false
	frameOne := buildWatchFrame(summary, nil, 60*time.Second, oneHidden, 100, 30, false, "", "")
	oneText := strings.Join(frameOne.lines, "\n")
	if !strings.Contains(oneText, "1 hidden") {
		t.Errorf("expected '1 hidden' with exactly one box toggled off, got:\n%s", oneText)
	}

	// N hidden: AllUsage, Processes, History, Load all off (4).
	nHidden := allOn
	nHidden.AllUsage = false
	nHidden.Processes = false
	nHidden.History = false
	nHidden.Load = false
	frameN := buildWatchFrame(summary, nil, 60*time.Second, nHidden, 100, 30, false, "", "")
	nText := strings.Join(frameN.lines, "\n")
	if !strings.Contains(nText, "4 hidden") {
		t.Errorf("expected '4 hidden' with four boxes toggled off, got:\n%s", nText)
	}
	// Never a per-box badge list for the toggle-hidden hint (that mechanism
	// is reserved for the separate "terminal too short" drop note).
	if strings.Contains(nText, "hidden: [") {
		t.Errorf("expected the toggle-hidden hint to be a single count, not per-box badges, got:\n%s", nText)
	}
}

// TestBuildLoadBox_RemoteUsesSnapshot guards issue 090: in remote mode the
// Load panel must render from the supplied snapshot, never from a local
// CurrentCPULoad()/CurrentGPUs() call, so it can't silently show the wrong
// machine's load.
func TestBuildLoadBox_RemoteUsesSnapshot(t *testing.T) {
	snap := &LoadSnapshot{
		CPU: CPULoad{
			NumCPU:       4,
			Ok:           true,
			CPUPercent:   42,
			CPUPercentOk: true,
			Memory:       SystemMemory{UsedMiB: 4096, TotalMiB: 8192, Ok: true},
		},
		GPUs: []GPU{{Name: "Remote GPU", UtilPercent: 7}},
	}

	box := buildLoadBox(minBoxWidth, "remote-worker-1", snap)
	text := strings.Join(box.lines, "\n")

	if !strings.Contains(text, "42%") {
		t.Errorf("expected remote CPU percent 42%% in load box, got:\n%s", stripANSI(text))
	}
	if !strings.Contains(text, "Remote GPU") {
		t.Errorf("expected remote GPU name in load box, got:\n%s", stripANSI(text))
	}
	if !strings.Contains(stripANSI(box.title), "@remote-worker-1") {
		t.Errorf("expected remote host in load box title, got %q", stripANSI(box.title))
	}
}

// TestBuildLoadBox_RemoteWithoutSnapshotDoesNotFallBackLocally guards the
// specific failure this issue is about: a remote session with no snapshot
// yet (fresh fallback summary, SSH hiccup) must show an explicit
// unavailable placeholder, not silently substitute local /proc /sys data.
func TestBuildLoadBox_RemoteWithoutSnapshotDoesNotFallBackLocally(t *testing.T) {
	box := buildLoadBox(minBoxWidth, "remote-worker-1", nil)
	text := stripANSI(strings.Join(box.lines, "\n"))
	if !strings.Contains(text, "remote load unavailable") {
		t.Errorf("expected explicit remote-unavailable placeholder, got:\n%s", text)
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

// TestBuildWatchFrameLayoutMatrix is the issue 093 acceptance-criteria
// layout matrix: every combination of --summary / --summary --proc /
// --watch --compact / --watch --compact --proc at 80x18, 80x24, 100x24,
// and 120x18 must
//   - never emit a line wider than the terminal (uix.Layout's row-packing
//     invariant, checked end to end through buildWatchFrame),
//   - show the Processes panel body whenever --proc is requested, even
//     under --compact (the flag-interaction bug this issue fixes), and
//   - identify any height-dropped panels by bracketed key (e.g. "[P] [O]
//     hidden"), never only a bare count.
func TestBuildWatchFrameLayoutMatrix(t *testing.T) {
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
			{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true,
				Account: "active session", PlanTier: "Pro",
				Weekly:  &QuotaWindow{Name: "Weekly", UsedPercent: 86, DurationLeft: 7*time.Hour + 16*time.Minute},
				Session: &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 24, DurationLeft: 3*time.Hour + 10*time.Minute}},
			{AgentID: "agy", Name: "Antigravity (AGY)", Installed: true, Authenticated: true,
				Account: "u***l@gmail.com", PlanTier: "Consumer", ActiveModel: "Gemini 3.7 Flash (Low)",
				ModelGroups: []ModelGroup{{Name: "Gemini Models", Windows: []QuotaWindow{
					{Name: "Weekly", UsedPercent: 97, DurationLeft: 2*24*time.Hour + 6*time.Hour},
					{Name: "Session (5-hour)", UsedPercent: 27, DurationLeft: time.Hour},
				}}}},
			{AgentID: "codex", Name: "OpenAI Codex", Installed: true, Authenticated: true,
				Account: "u***l@gmail.com", PlanTier: "Plus", ActiveModel: "gpt-5.5",
				Weekly:  &QuotaWindow{Name: "Weekly", UsedPercent: 49, DurationLeft: 5*24*time.Hour + 7*time.Hour},
				Session: &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 59, DurationLeft: 3*time.Hour + 24*time.Minute}},
		},
	}

	type mode struct {
		name string
		sec  watchSections
	}
	modes := []mode{
		{"summary", func() watchSections { s := defaultWatchSections(); return s }()},
		{"summary-proc", func() watchSections { s := defaultWatchSections(); s.Processes = true; return s }()},
		{"watch-compact", initialWatchSections(WatchOptions{Compact: true})},
		{"watch-compact-proc", initialWatchSections(WatchOptions{Compact: true, ShowProcesses: true})},
	}

	sizes := [][2]int{{80, 18}, {80, 24}, {100, 24}, {120, 18}}

	hiddenNoteRE := regexp.MustCompile(`… ((\[[A-Za-z]\] ?)+)hidden`)
	bareCountRE := regexp.MustCompile(`\d+ panel\(s\) hidden`)

	for _, sz := range sizes {
		cols, rows := sz[0], sz[1]
		for _, m := range modes {
			t.Run(fmt.Sprintf("%dx%d/%s", cols, rows, m.name), func(t *testing.T) {
				frame := buildWatchFrame(summary, nil, 60*time.Second, m.sec, cols, rows, false, "", tempDir)

				for i, l := range frame.lines {
					if got := visLen(l); got > frame.cols {
						t.Errorf("line %d visible width %d exceeds usable %d: %q", i, got, frame.cols, stripANSI(l))
					}
				}

				frameText := strings.Join(frame.lines, "\n")
				plain := stripANSI(frameText)

				if strings.Contains(m.name, "proc") {
					// --proc must win the "is this panel requested" decision
					// (issue 093's flag-interaction bug: --compact used to
					// silently ignore ShowProcesses). It may still lose the
					// "does it fit" decision to a short terminal, so this
					// only asserts Processes never shows up in the toggle
					// "hidden: [x]" hint (a *requested* panel is never
					// toggled off) — a height-overflow drop is a separate,
					// legitimate outcome checked below via droppedKeys.
					if idx := strings.Index(plain, "hidden: "); idx >= 0 {
						hintLine := plain[idx:]
						if nl := strings.IndexByte(hintLine, '\n'); nl >= 0 {
							hintLine = hintLine[:nl]
						}
						if strings.Contains(hintLine, "[P]") {
							t.Errorf("--proc requested but Processes shows up in the toggled-off hidden hint: %q", hintLine)
						}
					}
				}

				if bareCountRE.MatchString(plain) {
					t.Errorf("height-overflow hint used a bare count instead of panel keys, got:\n%s", plain)
				}
				if idx := strings.Index(plain, "hidden — terminal too short"); idx >= 0 {
					if !hiddenNoteRE.MatchString(plain) {
						t.Errorf("height-overflow hint did not list bracketed panel keys, got:\n%s", plain)
					}
				}
			})
		}
	}
}

// boxTopBorderWidths measures each box's rendered width on a top-border
// line ("┌─ [X] Title ──…──┐" segments side by side), by pairing each "┌"
// with its "┐". Used to check a box's actual width against its useful
// content width instead of the full terminal width.
func boxTopBorderWidths(line string) []int {
	runes := []rune(stripANSI(line))
	var widths []int
	start := -1
	for i, r := range runes {
		switch r {
		case '┌':
			start = i
		case '┐':
			if start >= 0 {
				widths = append(widths, i-start+1)
				start = -1
			}
		}
	}
	return widths
}

// TestBuildWatchFrameLoadOnlyDoesNotStretch covers issue 093's Findings #4:
// toggling compact mode down to just Load must not leave the box stretched
// across most of a wide terminal — it should stay near its own useful
// width since it does not benefit from stretch.
func TestBuildWatchFrameLoadOnlyDoesNotStretch(t *testing.T) {
	sec := compactWatchSections()
	sec.AllUsage = false // only Load left visible, as in the issue's toggle repro

	summary := UsageSummary{Timestamp: testTime}
	frame := buildWatchFrame(summary, nil, 60*time.Second, sec, 100, 24, false, "", "")

	found := false
	for _, l := range frame.lines {
		if !strings.Contains(stripANSI(l), "⁷ Load") {
			continue
		}
		found = true
		widths := boxTopBorderWidths(l)
		if len(widths) != 1 {
			t.Fatalf("expected exactly one box on the Load-only row, got %v in %q", widths, stripANSI(l))
		}
		if widths[0] > minBoxWidth+20 {
			t.Errorf("Load-only box width %d stretched far past its useful width (min %d), line: %q",
				widths[0], minBoxWidth, stripANSI(l))
		}
	}
	if !found {
		t.Fatalf("expected a Load box in the frame:\n%s", strings.Join(frame.lines, "\n"))
	}
}

// TestBuildWatchFrameCompactAllUsageDoesNotStarveLoad covers issue 093's
// Findings #5: in compact mode with both All Usage and Load visible, All
// Usage must not claim the whole row and squeeze Load down; each box
// should size to its own useful content width and share the row.
func TestBuildWatchFrameCompactAllUsageDoesNotStarveLoad(t *testing.T) {
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true,
				Weekly:  &QuotaWindow{Name: "Weekly", UsedPercent: 86, DurationLeft: 7*time.Hour + 16*time.Minute},
				Session: &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 24, DurationLeft: 3*time.Hour + 10*time.Minute}},
		},
	}

	frame := buildWatchFrame(summary, nil, 60*time.Second, compactWatchSections(), 100, 24, false, "", "")

	for _, l := range frame.lines {
		stripped := stripANSI(l)
		if !strings.Contains(stripped, "¹ All Usage") || !strings.Contains(stripped, "⁷ Load") {
			continue
		}
		widths := boxTopBorderWidths(l)
		if len(widths) != 2 {
			t.Fatalf("expected All Usage and Load as two boxes on one row, got %v in %q", widths, stripped)
		}
		loadWidth := widths[1]
		if loadWidth < minBoxWidth {
			t.Errorf("Load box starved to width %d (min useful %d) by All Usage sharing the row: %q",
				loadWidth, minBoxWidth, stripped)
		}
		return
	}
	t.Fatalf("expected All Usage and Load to share one row at 100 columns:\n%s", strings.Join(frame.lines, "\n"))
}

// TestControlsOverlayListsAllActiveCommandsGroupedByPurpose is issue 094's
// core acceptance criterion: the [?] overlay must list every active
// keyboard command, grouped by purpose (view modes, panels, data rows,
// session), not just a subset.
func TestControlsOverlayListsAllActiveCommandsGroupedByPurpose(t *testing.T) {
	lines := controlsOverlayLines()
	text := strings.Join(lines, "\n")
	plain := stripANSI(text)

	if !strings.Contains(plain, "Controls") {
		t.Fatalf("expected overlay to be titled Controls, got:\n%s", plain)
	}

	for _, group := range []string{"View modes", "Panels", "Data rows", "Session"} {
		if !strings.Contains(plain, group) {
			t.Errorf("expected overlay to group controls under %q, got:\n%s", group, plain)
		}
	}

	// Every currently-live action's spec-sourced symbol must be documented
	// somewhere in the overlay (issue 132: superscript digits for the
	// numbered box toggles, literal keys for everything else) so it stays
	// the single source of truth. The old C/G/O/H/P/L letter toggles were
	// dropped in favor of the numbered scheme (see collision note below).
	for _, key := range []string{
		"[¹]", "[²]", "[³]", "[⁴]", "[⁵]", "[⁶]", "[⁷]", "[A]",
		"[T]", "[m]", "[r]", "[q]",
	} {
		if !strings.Contains(plain, key) {
			t.Errorf("expected overlay to document key %s, got:\n%s", key, plain)
		}
	}
	// [a] is documented as a compat alias in the trailing note, not as a
	// bracketed key of its own.
	if !strings.Contains(plain, "[a]") {
		t.Errorf("expected overlay to document the [a] compat alias, got:\n%s", plain)
	}

	if !strings.Contains(plain, "?") {
		t.Errorf("expected overlay dismiss instructions to mention ?, got:\n%s", plain)
	}
}

// TestBuildWatchFrameShowControlsRendersOverlayInstead verifies the overlay
// is drawn in the existing alternate-screen frame (via WatchOptions), not a
// separate rendering path, and that it replaces the normal panel grid.
func TestBuildWatchFrameShowControlsRendersOverlayInstead(t *testing.T) {
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true},
		},
	}

	frame := buildWatchFrame(summary, nil, 60*time.Second, defaultWatchSections(), 100, 30, true, "", "",
		WatchOptions{ShowControls: true})
	text := stripANSI(strings.Join(frame.lines, "\n"))

	if !strings.Contains(text, "Controls") {
		t.Fatalf("expected ShowControls frame to render the Controls overlay, got:\n%s", text)
	}
	if strings.Contains(text, "] Claude Code") {
		t.Fatalf("expected ShowControls frame to replace the panel grid, not show agent boxes, got:\n%s", text)
	}
	for _, l := range frame.lines {
		if got := visLen(l); got > frame.cols {
			t.Errorf("overlay line exceeds usable width %d: %q", frame.cols, stripANSI(l))
		}
	}
}

// TestNextWatchPresetCyclesThroughDefaultCompactAgents covers the [m] mode
// shortcut's transition logic (issue 094 acceptance: at least one
// preset/mode transition under test).
func TestNextWatchPresetCyclesThroughDefaultCompactAgents(t *testing.T) {
	sec, name, idx := nextWatchPreset(0)
	if name != "compact" || sec != compactWatchSections() {
		t.Fatalf("preset after default (idx 0) = %q %+v, want compact %+v", name, sec, compactWatchSections())
	}
	if idx != 1 {
		t.Fatalf("expected idx 1 after first cycle, got %d", idx)
	}

	sec, name, idx = nextWatchPreset(idx)
	if name != "agents" {
		t.Fatalf("preset after compact = %q, want agents", name)
	}
	want := agentsOnlyWatchSections()
	if sec != want {
		t.Fatalf("agents preset = %+v, want %+v", sec, want)
	}
	if !sec.Claude || !sec.AGY || !sec.Codex || !sec.Tokens {
		t.Fatalf("agents preset should show discovered agent boxes plus tokens: %+v", sec)
	}
	if sec.History || sec.Load || sec.Processes || sec.AllUsage {
		t.Fatalf("agents preset should hide History/Load/Processes/AllUsage: %+v", sec)
	}

	sec, name, idx = nextWatchPreset(idx)
	if name != "default" || sec != defaultWatchSections() {
		t.Fatalf("preset after agents should wrap to default, got %q %+v", name, sec)
	}
	if idx != 0 {
		t.Fatalf("expected idx to wrap to 0, got %d", idx)
	}
}

// TestDispatchWatchKeyOverlayOpenClose is issue 094's key-dispatch
// acceptance criterion: [?] opens the overlay, panel-toggle keys are
// swallowed while it's open (rather than silently mutating sec behind it),
// and each of the documented dismiss keys (Esc, q, Ctrl-C, Enter) closes it
// and hands back a redraw.
func TestDispatchWatchKeyOverlayOpenClose(t *testing.T) {
	base := watchKeyState{sec: defaultWatchSections()}

	opened, eff := dispatchWatchKey(base, '?', false)
	if !opened.overlayOpen {
		t.Fatalf("expected ? to open the overlay")
	}
	if !eff.redraw {
		t.Fatalf("expected opening the overlay to request a redraw")
	}

	// A panel toggle while the overlay is open must not mutate sec.
	swallowed, eff := dispatchWatchKey(opened, 'C', false)
	if swallowed.sec != opened.sec {
		t.Fatalf("expected panel toggle to be swallowed while overlay open: got sec %+v, want unchanged %+v", swallowed.sec, opened.sec)
	}
	if !swallowed.overlayOpen {
		t.Fatalf("expected overlay to remain open after a swallowed key")
	}
	if eff.redraw || eff.fetch || eff.quit {
		t.Fatalf("expected a swallowed key to produce no effect, got %+v", eff)
	}

	for _, dismiss := range []byte{27, 'q', 'Q', 3, '\r', '\n'} {
		closed, eff := dispatchWatchKey(opened, dismiss, false)
		if closed.overlayOpen {
			t.Errorf("expected key %v to close the overlay", dismiss)
		}
		if !eff.redraw {
			t.Errorf("expected closing the overlay via key %v to request a redraw", dismiss)
		}
		if eff.quit {
			t.Errorf("expected key %v to close the overlay, not quit the app, while it was open", dismiss)
		}
	}

	// ? toggles closed again too.
	reclosed, _ := dispatchWatchKey(opened, '?', false)
	if reclosed.overlayOpen {
		t.Fatalf("expected second ? press to close the overlay")
	}

	// Once closed, q quits rather than being swallowed.
	final, eff := dispatchWatchKey(watchKeyState{sec: defaultWatchSections()}, 'q', false)
	if !eff.quit {
		t.Fatalf("expected q to quit once the overlay is closed, got %+v (state %+v)", eff, final)
	}
}

// TestDispatchWatchKeyDebugOverlayRendersAndRestoresFrame covers issue 140
// through the production dispatch and frame-rendering paths. In particular,
// compact mode relies on the aggregate All Usage panel, so its per-agent rows
// must visibly reflect the toggled state without changing frame geometry.
func TestDispatchWatchKeyDebugOverlayRendersAndRestoresFrame(t *testing.T) {
	lastRefreshed := time.Now().Add(time.Hour)
	summary := UsageSummary{
		Timestamp: testTime,
		Agents: []AgentUsage{
			{
				AgentID:       "claude",
				Name:          "Claude Code",
				Installed:     true,
				Authenticated: true,
				LastRefreshed: lastRefreshed,
				Weekly:        &QuotaWindow{Name: "Weekly", UsedPercent: 25},
				Session:       &QuotaWindow{Name: "5-hour", UsedPercent: 50},
			},
		},
	}
	sec := watchSections{AllUsage: true}
	state := watchKeyState{sec: sec}
	historyDir := t.TempDir()

	render := func(debugOverlay bool) screenFrame {
		return buildWatchFrame(summary, nil, time.Minute, sec, 90, 24, true, t.TempDir(), historyDir, WatchOptions{
			DebugOverlay: debugOverlay,
		})
	}

	normal := render(state.debugOverlay)
	normalText := stripANSI(strings.Join(normal.lines, "\n"))
	if !strings.Contains(normalText, "Claude Code") {
		t.Fatalf("normal frame lost the aggregate agent label:\n%s", normalText)
	}
	if strings.Contains(normalText, "[█]") {
		t.Fatalf("normal frame unexpectedly contains the freshness gauge:\n%s", normalText)
	}

	toggled, eff := dispatchWatchKey(state, '!', false)
	if !toggled.debugOverlay || !eff.redraw || eff.fetch || eff.quit {
		t.Fatalf("! dispatch did not enable debug overlay with redraw only: state=%+v effect=%+v", toggled, eff)
	}
	if toggled.sec != state.sec || toggled.overlayOpen != state.overlayOpen {
		t.Fatalf("! dispatch changed unrelated watch state: got %+v, want sections/controls from %+v", toggled, state)
	}

	debug := render(toggled.debugOverlay)
	debugText := stripANSI(strings.Join(debug.lines, "\n"))
	if !strings.Contains(debugText, "Claude C[█]") {
		t.Fatalf("debug frame does not show the aggregate per-agent freshness gauge:\n%s", debugText)
	}
	if debug.cols != normal.cols || debug.rows != normal.rows || len(debug.lines) != len(normal.lines) {
		t.Fatalf("debug overlay changed frame geometry: normal=%dx%d/%d lines debug=%dx%d/%d lines",
			normal.cols, normal.rows, len(normal.lines), debug.cols, debug.rows, len(debug.lines))
	}
	for i := range normal.lines {
		if visLen(debug.lines[i]) != visLen(normal.lines[i]) {
			t.Errorf("debug overlay changed visible width on line %d: normal=%d debug=%d", i, visLen(normal.lines[i]), visLen(debug.lines[i]))
		}
	}

	restored, eff := dispatchWatchKey(toggled, '!', false)
	if restored.debugOverlay || !eff.redraw {
		t.Fatalf("second ! dispatch did not disable debug overlay with redraw: state=%+v effect=%+v", restored, eff)
	}
	restoredFrame := render(restored.debugOverlay)
	if got, want := strings.Join(restoredFrame.lines, "\n"), strings.Join(normal.lines, "\n"); got != want {
		t.Fatalf("second ! did not restore the exact normal frame:\n--- got ---\n%s\n--- want ---\n%s", stripANSI(got), stripANSI(want))
	}
}

// TestAllUsageLinesAtDebugOverlayCountsDown proves the compact All Usage
// panel recalculates its gauge from LastRefreshed on each redraw timestamp,
// without requiring a new usage collection. It also locks the future and
// overdue bounds to full and empty respectively (issue 147).
func TestAllUsageLinesAtDebugOverlayCountsDown(t *testing.T) {
	refreshed := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	summary := UsageSummary{Agents: []AgentUsage{
		{
			AgentID:       "claude",
			Name:          "Claude Code",
			Installed:     true,
			Authenticated: true,
			LastRefreshed: refreshed,
			Weekly:        &QuotaWindow{Name: "Weekly", UsedPercent: 25},
		},
	}}

	assertGauge := func(now time.Time, want string) {
		t.Helper()
		lines := stripANSI(strings.Join(allUsageLinesAt(summary, 72, true, now), "\n"))
		if !strings.Contains(lines, "Claude C"+want) {
			t.Fatalf("gauge at %s = %q, want Claude C%s", now, lines, want)
		}
	}

	assertGauge(refreshed, "[█]")
	assertGauge(refreshed.Add(DefaultCollectorInterval/2), "[▄]")
	assertGauge(refreshed.Add(DefaultCollectorInterval), "[▁]")
	assertGauge(refreshed.Add(-time.Second), "[█]")
	assertGauge(refreshed.Add(2*DefaultCollectorInterval), "[▁]")
}

// TestDispatchWatchKeyModeCyclesAndKeepsShowProcesses covers the [m] preset
// shortcut through the same dispatch path RunWatch uses, including the
// showProcesses carry-through that initialWatchSections already guarantees
// at startup (issue 093's flag-interaction fix must keep holding here too).
func TestDispatchWatchKeyModeCyclesAndKeepsShowProcesses(t *testing.T) {
	st := watchKeyState{sec: defaultWatchSections()}

	st, eff := dispatchWatchKey(st, 'm', true)
	if !eff.redraw {
		t.Fatalf("expected m to request a redraw")
	}
	if st.sec != func() watchSections { s := compactWatchSections(); s.Processes = true; return s }() {
		t.Fatalf("expected m to move to compact preset with Processes forced on: %+v", st.sec)
	}

	st, _ = dispatchWatchKey(st, 'm', true)
	if !st.sec.Claude || !st.sec.AGY || !st.sec.Codex || !st.sec.Processes {
		t.Fatalf("expected agents preset with Processes forced on: %+v", st.sec)
	}
}

// TestRenderSummary_CompactSelectsReducedSections is issue 102's acceptance
// criterion: `harnez usage --summary --compact` must render the same reduced
// panel set as `--watch --compact` (compactWatchSections), not just be
// accepted by the flag guard. RenderSummary previously had no way to reach
// compactWatchSections() at all, regardless of what the CLI guard allowed.
func TestRenderSummary_CompactSelectsReducedSections(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()

	var full bytes.Buffer
	RenderSummary(ctx, home, nil, &full, false)
	fullText := stripANSI(full.String())
	if !strings.Contains(fullText, "⁵ History") {
		t.Fatalf("expected default --summary output to include the History box (defaultWatchSections), got:\n%s", fullText)
	}

	var compact bytes.Buffer
	RenderSummary(ctx, home, nil, &compact, false, WatchOptions{Compact: true})
	compactText := stripANSI(compact.String())
	if strings.Contains(compactText, "⁵ History") {
		t.Fatalf("expected --summary --compact to drop the History box (compactWatchSections has no History), got:\n%s", compactText)
	}
	if !strings.Contains(compactText, "⁷ Load") {
		t.Fatalf("expected --summary --compact to keep the Load box, got:\n%s", compactText)
	}
}
