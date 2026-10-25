package usage

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/loom"
)

var updateWatchBoxGoldens = flag.Bool("update-watch-box-goldens", false, "capture current watch box ANSI output")

func TestWatchBoxBaselineGoldens(t *testing.T) {
	t.Setenv("LOOMCOLOR", "truecolor")
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	debugBox := buildUsageDebugBox(UsageSummary{Timestamp: now}, 0, now)
	debugBox.width = 48
	boxes := []struct {
		name string
		box  wbox
	}{
		{
			name: "all-usage",
			box: wbox{
				title: watchBoxSymbol("all_usage") + " All Usage",
				lines: []string{"Claude Code  \x1b[31m⣿⣿⣿⡇\x1b[0m 95% 19h35m"},
				width: 40,
			},
		},
		{
			name: "load-host",
			box: wbox{
				title: watchBoxSymbol("load") + " Load " + ansiDimGrey + "(@worker)\x1b[0m",
				lines: []string{"cpu (8 cores)   \x1b[34m⣀⣀⣀⣀\x1b[0m 0% (42°C)"},
				width: 42,
			},
		},
		{
			name: "remote-load-source",
			box: wbox{
				title: ansiBold + "[R]\x1b[0m Remote Load " + ansiDimGrey + "(@worker · server cache)\x1b[0m",
				lines: []string{"ram (32G)        \x1b[32m⣤⣤⣤⣤\x1b[0m 47%"},
				width: 50,
			},
		},
		{
			name: "debug",
			box:  debugBox,
		},
	}

	for _, tc := range boxes {
		t.Run(tc.name, func(t *testing.T) {
			gotLines := renderWBox(tc.box)
			path := filepath.Join("testdata", "watch-box-baseline", tc.name+".ansi")
			if *updateWatchBoxGoldens {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(strings.Join(gotLines, "\n")+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			wantData, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			wantLines := strings.Split(strings.TrimSuffix(string(wantData), "\n"), "\n")
			if len(gotLines) != len(wantLines) {
				t.Fatalf("line count = %d, want %d", len(gotLines), len(wantLines))
			}
			for y := range gotLines {
				gotCells, wantCells := loom.ParseANSI(gotLines[y]), loom.ParseANSI(wantLines[y])
				if len(gotCells) != len(wantCells) {
					t.Fatalf("row %d cell count = %d, want %d", y, len(gotCells), len(wantCells))
				}
				for x := range gotCells {
					got, want := gotCells[x], wantCells[x]
					if got.Text != want.Text || got.Continuation != want.Continuation ||
						got.Style.FG != want.Style.FG || got.Style.BG != want.Style.BG || got.Style.Bold != want.Style.Bold {
						t.Errorf("cell (%d,%d): got glyph=%q fg=%v bg=%v bold=%v; want glyph=%q fg=%v bg=%v bold=%v",
							x, y, got.Text, got.Style.FG, got.Style.BG, got.Style.Bold,
							want.Text, want.Style.FG, want.Style.BG, want.Style.Bold)
					}
				}
			}
		})
	}
}
