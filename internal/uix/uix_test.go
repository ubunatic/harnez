package uix

import (
	"strings"
	"testing"
)

func TestLayoutSkipsDisabledBoxes(t *testing.T) {
	plan := Layout([]Box{
		{Title: "Hidden", MinWidth: 10, PrefWidth: 10, Enabled: false},
		{Title: "Visible", MinWidth: 10, PrefWidth: 10, Enabled: true},
	}, Options{Width: 80, Gap: 1})

	if len(plan.Rows) != 1 || len(plan.Rows[0].Boxes) != 1 {
		t.Fatalf("got %#v, want one visible box", plan.Rows)
	}
	if got := plan.Rows[0].Boxes[0].Title; got != "Visible" {
		t.Errorf("visible box = %q, want Visible", got)
	}
}

func TestLayoutWrapsAtPreferredWidths(t *testing.T) {
	boxes := []Box{
		{Title: "One", MinWidth: 20, PrefWidth: 30, Enabled: true},
		{Title: "Two", MinWidth: 20, PrefWidth: 30, Enabled: true},
		{Title: "Three", MinWidth: 20, PrefWidth: 30, Enabled: true},
	}
	plan := Layout(boxes, Options{Width: 64, Gap: 1})

	if got, want := len(plan.Rows), 2; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	if got, want := len(plan.Rows[0].Boxes), 2; got != want {
		t.Errorf("first row boxes = %d, want %d", got, want)
	}
	if got, want := len(plan.Rows[1].Boxes), 1; got != want {
		t.Errorf("second row boxes = %d, want %d", got, want)
	}
}

func TestLayoutDoesNotStretchSingleBoxUnlessStretchable(t *testing.T) {
	compact := Layout([]Box{
		{Title: "Load", MinWidth: 28, PrefWidth: 34, MaxWidth: 42, Enabled: true},
	}, Options{Width: 100, Gap: 1})
	if got, want := compact.Rows[0].Boxes[0].Width, 34; got != want {
		t.Errorf("non-stretch single box width = %d, want %d", got, want)
	}

	stretched := Layout([]Box{
		{Title: "Timeline", MinWidth: 28, PrefWidth: 34, MaxWidth: 50, Stretch: true, Enabled: true},
	}, Options{Width: 100, Gap: 1})
	if got, want := stretched.Rows[0].Boxes[0].Width, 50; got != want {
		t.Errorf("stretch single box width = %d, want %d", got, want)
	}
}

func TestLayoutDistributesExtraOnlyToStretchBoxes(t *testing.T) {
	plan := Layout([]Box{
		{Title: "Fixed", MinWidth: 10, PrefWidth: 20, MaxWidth: 30, Enabled: true},
		{Title: "Stretch", MinWidth: 10, PrefWidth: 20, MaxWidth: 35, Stretch: true, Enabled: true},
	}, Options{Width: 60, Gap: 1})

	if got, want := plan.Rows[0].Boxes[0].Width, 20; got != want {
		t.Errorf("fixed width = %d, want %d", got, want)
	}
	if got, want := plan.Rows[0].Boxes[1].Width, 35; got != want {
		t.Errorf("stretch width = %d, want %d", got, want)
	}
}

func TestRenderLinesUsePlannedWidths(t *testing.T) {
	plan := Layout([]Box{
		{Title: "Short", Lines: []string{"ok", strings.Repeat("x", 100)}, MinWidth: 12, PrefWidth: 16, Enabled: true},
		{Title: "Tall", Lines: []string{"a", "b", "c"}, MinWidth: 12, PrefWidth: 14, Enabled: true},
	}, Options{Width: 80, Gap: 2})
	out := Render(plan)

	for i, line := range strings.Split(out, "\n") {
		if got, want := runeLen(line), 32; got != want {
			t.Errorf("line %d width = %d, want %d: %q", i, got, want, line)
		}
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("render output contains ANSI escapes: %q", out)
	}
}

func TestIssue093DemoLoadOnlyStaysCompact(t *testing.T) {
	plan := Layout(issue093LoadOnlyBoxes(), Options{Width: 100, Gap: 1})

	if got, want := plan.Rows[0].Boxes[0].Width, 34; got != want {
		t.Fatalf("load-only width = %d, want %d", got, want)
	}
	assertGolden(t, Render(plan), issue093LoadOnlyDemo)
}

func TestIssue093DemoAllUsageAndLoadShareRow(t *testing.T) {
	plan := Layout(issue093AllUsageLoadBoxes(), Options{Width: 90, Gap: 1})

	if got, want := len(plan.Rows), 1; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	assertGolden(t, Render(plan), issue093AllUsageLoadDemo)
}

func TestIssue093DemoAllUsageAndAgentsPackPredictably(t *testing.T) {
	plan := Layout(issue093AllUsageAgentBoxes(), Options{Width: 120, Gap: 1})

	if got, want := len(plan.Rows), 2; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	if got, want := plan.Rows[0].Boxes[0].Width, 84; got != want {
		t.Errorf("all-usage width = %d, want %d", got, want)
	}
	if got, want := len(plan.Rows[1].Boxes), 3; got != want {
		t.Errorf("agent row boxes = %d, want %d", got, want)
	}
	assertGolden(t, Render(plan), issue093AllUsageAgentsDemo)
}

func assertGolden(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("render mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func issue093LoadOnlyBoxes() []Box {
	return []Box{
		{
			Title:     "[L] Load",
			Lines:     []string{"cpu (12 cores)   [    ] 5%", "ram              4.8/23.3G 21%", "gpu mem          1.2/19.6G 6%"},
			MinWidth:  30,
			PrefWidth: 34,
			MaxWidth:  42,
			Enabled:   true,
		},
	}
}

func issue093AllUsageLoadBoxes() []Box {
	return []Box{
		{
			Title:     "[a] All Usage",
			Lines:     []string{"Claude Code   [    ] 86% 7h10m", "Gemini        [    ] 97% 2d6h", "OpenAI Codex  [    ] 49% 5d7h"},
			MinWidth:  42,
			PrefWidth: 48,
			MaxWidth:  60,
			Enabled:   true,
		},
		{
			Title:     "[L] Load",
			Lines:     []string{"cpu (12 cores)   [    ] 5%", "ram              4.8/23.3G 21%"},
			MinWidth:  30,
			PrefWidth: 34,
			MaxWidth:  42,
			Enabled:   true,
		},
	}
}

func issue093AllUsageAgentBoxes() []Box {
	return []Box{
		{
			Title:     "[a] All Usage",
			Lines:     []string{"Claude Code   [    ] 86% 7h10m [    ] 24%", "Gemini        [    ] 97% 2d6h  [    ] 28%", "OpenAI Codex  [    ] 49% 5d7h  [    ] 62%"},
			MinWidth:  42,
			PrefWidth: 84,
			MaxWidth:  84,
			Enabled:   true,
		},
		{
			Title:     "[C] Claude Code",
			Lines:     []string{"active session - Pro", "Wk / 5h    [    ] 86% [    ] 24%"},
			MinWidth:  32,
			PrefWidth: 36,
			MaxWidth:  44,
			Enabled:   true,
		},
		{
			Title:     "[G] Antigravity",
			Lines:     []string{"u***l@gmail.com - Consumer", "Gemini     [    ] 97% [    ] 28%"},
			MinWidth:  32,
			PrefWidth: 38,
			MaxWidth:  44,
			Enabled:   true,
		},
		{
			Title:     "[O] OpenAI Codex",
			Lines:     []string{"u***l@gmail.com - Plus", "Wk / 5h    [   ] 49% [   ] 62%"},
			MinWidth:  32,
			PrefWidth: 36,
			MaxWidth:  44,
			Enabled:   true,
		},
	}
}

const issue093LoadOnlyDemo = `+ [L] Load ----------------------+
| cpu (12 cores)   [    ] 5%     |
| ram              4.8/23.3G 21% |
| gpu mem          1.2/19.6G 6%  |
+--------------------------------+`

const issue093AllUsageLoadDemo = `+ [a] All Usage -------------------------------+ + [L] Load ----------------------+
| Claude Code   [    ] 86% 7h10m               | | cpu (12 cores)   [    ] 5%     |
| Gemini        [    ] 97% 2d6h                | | ram              4.8/23.3G 21% |
| OpenAI Codex  [    ] 49% 5d7h                | +--------------------------------+
+----------------------------------------------+                                   `

const issue093AllUsageAgentsDemo = `+ [a] All Usage -------------------------------------------------------------------+
| Claude Code   [    ] 86% 7h10m [    ] 24%                                        |
| Gemini        [    ] 97% 2d6h  [    ] 28%                                        |
| OpenAI Codex  [    ] 49% 5d7h  [    ] 62%                                        |
+----------------------------------------------------------------------------------+

+ [C] Claude Code -----------------+ + [G] Antigravity -------------------+ + [O] OpenAI Codex ----------------+
| active session - Pro             | | u***l@gmail.com - Consumer         | | u***l@gmail.com - Plus           |
| Wk / 5h    [    ] 86% [    ] 24% | | Gemini     [    ] 97% [    ] 28%   | | Wk / 5h    [   ] 49% [   ] 62%   |
+----------------------------------+ +------------------------------------+ +----------------------------------+`
