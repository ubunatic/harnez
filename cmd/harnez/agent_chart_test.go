package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

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

func TestModelChartNarrowWidthRefusal(t *testing.T) {
	entries, err := subagent.KnownModelEntriesE()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = renderModelsChart(&out, entries, 50, "raw", false)
	if err == nil || !strings.Contains(err.Error(), "terminal width 50 is too narrow for --chart (minimum 60 columns)") {
		t.Fatalf("expected narrow width refusal, got: %v", err)
	}
}

func TestModelChartNoColorAndUnplotted(t *testing.T) {
	entries, err := subagent.KnownModelEntriesE()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := renderModelsChart(&out, entries, 80, "raw", false); err != nil {
		t.Fatalf("renderModelsChart failed: %v", err)
	}
	res := out.String()

	// Verify NO ANSI escape codes when colorEnabled is false
	if strings.Contains(res, "\x1b[") {
		t.Fatalf("output contains ANSI escape sequences when color is disabled: %q", res)
	}

	// Verify unplotted models are listed below
	if !strings.Contains(res, "Models not plotted") {
		t.Fatalf("expected 'Models not plotted' section, got:\n%s", res)
	}
	for _, unplotted := range []string{"jules:jules:low", "lmcoder:coder:low", "strata:qwen"} {
		if !strings.Contains(res, unplotted) {
			t.Errorf("expected unplotted list to contain %q, got:\n%s", unplotted, res)
		}
	}

	// Verify Pareto frontier is present and lists models
	if !strings.Contains(res, "Pareto frontier: ") {
		t.Fatalf("expected Pareto frontier summary, got:\n%s", res)
	}
	if !strings.Contains(res, "codex:luna:low") {
		t.Errorf("expected codex:luna:low on Pareto frontier")
	}
}

func TestModelChartEffectiveCost(t *testing.T) {
	entries, err := subagent.KnownModelEntriesE()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := renderModelsChart(&out, entries, 80, "effective", false); err != nil {
		t.Fatalf("renderModelsChart effective failed: %v", err)
	}
	res := out.String()
	if !strings.Contains(res, "effective cost (subscription-adjusted)") {
		t.Fatalf("expected effective cost indicator in output, got:\n%s", res)
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
	want, err := os.ReadFile(goldenFile)
	if err != nil {
		// If golden doesn't exist yet, we create it
		t.Fatalf("read golden %s: %v", goldenFile, err)
	}
	if got != string(want) {
		t.Fatalf("output does not match golden %s:\nGOT:\n%s\nWANT:\n%s", goldenFile, got, string(want))
	}
}
