package main

import (
	"bytes"
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
	// - a tie on cost and rating (modelC and modelD have identical cost 4.0 and rating 3.0)
	// - a tie on rating with higher cost (modelE has rating 3.0 at cost 8.0, dominated by modelC/D at cost 4.0)
	// - a tie on cost with lower rating (modelB has rating 2.0 at cost 4.0, dominated by modelC/D at rating 3.0)
	fixture := []chartPoint{
		{spec: "modelA:low", cost: 1.0, rating: 1.5},
		{spec: "modelB:low", cost: 4.0, rating: 2.0},  // dominated by modelC/D
		{spec: "modelC:low", cost: 4.0, rating: 3.0},  // on frontier
		{spec: "modelD:low", cost: 4.0, rating: 3.0},  // tie on cost & rating with modelC -> on frontier
		{spec: "modelE:low", cost: 8.0, rating: 3.0},  // tie on rating with modelC/D, but higher cost -> dominated
		{spec: "modelF:low", cost: 10.0, rating: 4.0}, // on frontier
	}

	frontier := computeParetoFrontier(fixture)

	// A broken implementation (e.g. returning points[:1]) will fail here
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
