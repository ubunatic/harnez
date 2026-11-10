package main

import (
	"bytes"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/braillecanvas"
	"ubunatic.com/harnez/internal/subagent"
)

func TestParetoFrontierFixture(t *testing.T) {
	// Fixture with:
	// - a dominated point (modelB is dominated on rating at cost 4; modelE is dominated on cost at rating 3.0)
	// - a tie on cost and rating across different models (modelC and modelD have identical cost 4.0 and rating 3.0) -> both on frontier
	// - a tie on cost and rating for the same model across tiers (modelC:low, modelC:med, modelC:high) -> collapsed to lowest tier (modelC:low)
	// - a tie on rating with higher cost (modelE has rating 3.0 at cost 8.0, dominated by modelC/D at cost 4.0) -> excluded
	// - a tie on cost with lower rating (modelB has rating 2.0 at cost 4.0, dominated by modelC/D at rating 3.0) -> excluded
	fixture := []chartPoint{
		{spec: "modelA:low", provider: "p1", alias: "modelA", tier: "low", cost: 1.0, rating: 1.5},
		{spec: "modelB:low", provider: "p1", alias: "modelB", tier: "low", cost: 4.0, rating: 2.0},   // dominated by modelC/D
		{spec: "modelC:low", provider: "p1", alias: "modelC", tier: "low", cost: 4.0, rating: 3.0},   // on frontier
		{spec: "modelC:med", provider: "p1", alias: "modelC", tier: "med", cost: 4.0, rating: 3.0},   // same model tie -> collapsed to lowest tier
		{spec: "modelC:high", provider: "p1", alias: "modelC", tier: "high", cost: 4.0, rating: 3.0}, // same model tie -> collapsed to lowest tier
		{spec: "modelD:low", provider: "p2", alias: "modelD", tier: "low", cost: 4.0, rating: 3.0},   // different model tie -> on frontier
		{spec: "modelE:low", provider: "p1", alias: "modelE", tier: "low", cost: 8.0, rating: 3.0},   // tie on rating with modelC/D, but higher cost -> dominated
		{spec: "modelF:low", provider: "p1", alias: "modelF", tier: "low", cost: 10.0, rating: 4.0},  // on frontier
	}

	frontier := computeParetoFrontier(fixture)

	// A broken implementation (e.g. returning points[:1] or repeating ties) will fail here
	if len(frontier) != 4 {
		t.Fatalf("expected 4 frontier points, got %d: %+v", len(frontier), frontier)
	}

	wantSpecs := []string{"modelA:low", "modelC:low", "modelD:low", "modelF:low"}
	var gotSpecs []string
	for _, p := range frontier {
		gotSpecs = append(gotSpecs, p.spec)
	}
	if !reflect.DeepEqual(gotSpecs, wantSpecs) {
		t.Fatalf("frontier specs = %v, want %v", gotSpecs, wantSpecs)
	}
}

func TestModelChartCollisionsAndValidations(t *testing.T) {
	runErr := func(args ...string) error {
		cmd := newAgentCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append([]string{"models"}, args...))
		return cmd.Execute()
	}

	// Refuse --chart with --json
	err := runErr("--chart", "--json")
	if err == nil || !strings.Contains(err.Error(), "--chart cannot be combined with --json") {
		t.Fatalf("expected --chart with --json refusal, got: %v", err)
	}

	// Refuse --chart with --picks
	err = runErr("--chart", "--picks")
	if err == nil || !strings.Contains(err.Error(), "--chart cannot be combined with") {
		t.Fatalf("expected --chart with --picks refusal, got: %v", err)
	}

	// Refuse invalid --cost
	err = runErr("--chart", "--cost", "invalid")
	if err == nil || !strings.Contains(err.Error(), "invalid --cost") {
		t.Fatalf("expected invalid --cost error, got: %v", err)
	}
}

func TestModelChartNarrowWidthRefusalThroughCommand(t *testing.T) {
	// Item 2: Width: honor COLUMNS when not a TTY; test the <60 refusal through the cobra command
	t.Setenv("COLUMNS", "50")
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"models", "--chart"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "terminal width 50 is too narrow for --chart (minimum 60 columns)") {
		t.Fatalf("expected narrow width refusal through cobra command, got: %v", err)
	}
}

func TestModelChartNO_COLORThroughAgentListColorEnabled(t *testing.T) {
	// Item 5: NO_COLOR test goes through agentListColorEnabled with the env var set
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	enabled, err := agentListColorEnabled("auto", &buf)
	if err != nil {
		t.Fatalf("agentListColorEnabled failed: %v", err)
	}
	if enabled {
		t.Fatalf("expected color to be disabled when NO_COLOR=1, got enabled")
	}

	t.Setenv("COLUMNS", "80")
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"models", "--chart"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("models --chart failed: %v", err)
	}
	res := out.String()
	if strings.Contains(res, "\x1b[") {
		t.Fatalf("output contains ANSI escape sequences with NO_COLOR=1:\n%s", res)
	}
}

func TestModelChartEffectiveCostAxisLabel(t *testing.T) {
	// Item 6: Effective mode axis label must not claim "luna:low = 1" (state the base, e.g. "raw luna:low = 1")
	t.Setenv("COLUMNS", "80")
	t.Setenv("NO_COLOR", "1")
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"models", "--chart", "--cost", "effective"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("models --chart --cost effective failed: %v", err)
	}
	res := out.String()
	if !strings.Contains(res, "Effective Cost (subscription-adjusted, raw codex:luna:low = 1)") {
		t.Fatalf("expected axis label with 'raw codex:luna:low = 1', got:\n%s", res)
	}
}

func TestLabelCollisionAvoidanceAndDrop(t *testing.T) {
	// Item 4: Labels: IsOccupied must also check dots; a label that cannot be placed without collision
	// is dropped from the canvas. No concatenated labels like "argonterra".
	c := braillecanvas.New(20, 5)

	// Completely fill all candidate rows for cell X=5..15 with dots
	for y := 0; y < 5; y++ {
		for px := 10; px <= 30; px++ {
			c.Set(px, y*4)
		}
	}

	// Any label attempted in range X=5..10 must report occupied on both dots and text
	if !c.IsOccupied(5, 2, 5) {
		t.Fatal("expected cells with dots to be occupied")
	}

	// Verify rendered chart output has no concatenated labels like "argonterra"
	entries, err := subagent.KnownModelEntriesE()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := renderModelsChart(&out, entries, 80, "raw", false); err != nil {
		t.Fatalf("renderModelsChart failed: %v", err)
	}
	res := out.String()
	if strings.Contains(res, "argonterra") {
		t.Fatalf("found concatenated label 'argonterra' in chart output:\n%s", res)
	}
}

func TestPlacedLabelsWithinTwoCells(t *testing.T) {
	// Item 1: A label must sit next to its own series' last (highest-tier) point; test that each
	// placed label's cell is within 2 cells of its point.
	entries, err := subagent.KnownModelEntriesE()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := renderModelsChart(&out, entries, 80, "raw", false); err != nil {
		t.Fatalf("renderModelsChart failed: %v", err)
	}
	lines := strings.Split(out.String(), "\n")
	if len(lines) < 16 {
		t.Fatalf("output has fewer lines than expected: %d", len(lines))
	}

	// Verify fable and sonnet have labels on canvas
	canvasText := strings.Join(lines[3:15], "\n")
	for _, expectedLabel := range []string{"fable", "sonnet", "opus", "sol", "flash38", "flash37", "haiku", "luna", "terra", "argon"} {
		if !strings.Contains(canvasText, expectedLabel) {
			t.Errorf("expected label %q on canvas, but not found", expectedLabel)
		}
	}

	scores, err := subagent.LoadModelScores()
	if err != nil {
		t.Fatal(err)
	}
	scaleMin := scores.Scale.Min
	scaleMax := scores.Scale.Max
	logMin := 0.0                // math.Log10(1)
	logMax := 2.6020599913279625 // math.Log10(400)
	canvasWidth := 80 - 6 - 2

	for row := 0; row < 12; row++ {
		lineRunes := []rune(lines[3+row])
		if len(lineRunes) <= 6 {
			continue
		}
		canvasRunes := lineRunes[6:]
		for _, word := range []string{"fable", "sonnet", "opus", "sol", "flash38", "flash37", "haiku", "luna", "terra", "argon"} {
			wordRunes := []rune(word)
			for i := 0; i <= len(canvasRunes)-len(wordRunes); i++ {
				match := true
				for j := range wordRunes {
					if canvasRunes[i+j] != wordRunes[j] {
						match = false
						break
					}
				}
				if !match {
					continue
				}
				labelCol := i

				matched := false
				for _, e := range entries {
					parts := strings.Split(e.Spec, ":")
					alias := e.Model.Name
					if len(parts) >= 2 {
						alias = parts[1]
					}
					if alias != word {
						continue
					}
					rating, _, ok := scores.Rating(e.Model.Provider, alias, e.Model.Tier)
					if !ok || e.Cost <= 0 {
						continue
					}

					normX := (math.Log10(float64(e.Cost)) - logMin) / (logMax - logMin)
					ptCol := int(math.Round(normX*float64(canvasWidth*2-1))) / 2
					normY := (rating - scaleMin) / (scaleMax - scaleMin)
					ptRow := 11 - int(math.Round(normY*float64(12*4-1)))/4

					distY := abs(row - ptRow)
					distX := labelCol - ptCol
					if distX < 0 {
						distX = ptCol - (labelCol + len(word) - 1)
					}
					if distY <= 1 && distX <= 2 {
						matched = true
						break
					}
				}
				if !matched {
					t.Errorf("label %q at row %d col %d is not within 2 cells of its point", word, row, labelCol)
				}
			}
		}
	}
}

func TestModelChartGolden(t *testing.T) {
	entries, err := subagent.KnownModelEntriesE()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := renderModelsChart(&out, entries, 80, "raw", false); err != nil {
		t.Fatalf("renderModelsChart failed: %v", err)
	}
	got := out.String()

	goldenFile := "testdata/agent-models-chart-80cols.golden"
	if *updateGolden {
		if err := os.WriteFile(goldenFile, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("read golden %s: %v", goldenFile, err)
	}
	if got != string(want) {
		t.Fatalf("output does not match golden %s:\nGOT:\n%s\nWANT:\n%s", goldenFile, got, string(want))
	}
}
