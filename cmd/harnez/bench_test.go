package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runBench(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newBenchCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return buf.String(), err
}

func TestBenchRequiresSetup(t *testing.T) {
	t.Setenv("HARNEZ_BENCH_DIR", filepath.Join(t.TempDir(), "bench"))
	for _, args := range [][]string{{"run"}, {"results"}} {
		_, err := runBench(t, args...)
		if err == nil || !strings.Contains(err.Error(), "harnez bench --setup") {
			t.Errorf("bench %v before setup: err = %v", args, err)
		}
	}
	if out, err := runBench(t, "tasks"); err != nil || !strings.Contains(out, "shell-conditional") {
		t.Errorf("tasks needs no setup: %q %v", out, err)
	}
}

func TestBenchSetupRunResults(t *testing.T) {
	t.Setenv("HARNEZ_BENCH_DIR", filepath.Join(t.TempDir(), "bench"))
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	var gotArgs []string
	benchRunner = func(_ context.Context, _, name string, args ...string) ([]byte, error) {
		gotArgs = append([]string{name}, args...)
		return []byte(`{"result":"ready","usage":{"input_tokens":7,"output_tokens":3}}`), nil
	}
	defer func() { benchRunner = nil }()

	if out, err := runBench(t, "--setup"); err != nil || !strings.Contains(out, "bench ready") {
		t.Fatalf("setup: %q %v", out, err)
	}
	out, err := runBench(t, "run", "--agent", "claude", "--docs", "lite", "--task", "hello", "--repo", root)
	if err != nil || !strings.Contains(out, "PASS") || gotArgs[0] != "claude" {
		t.Fatalf("run: %q %v %v", out, err, gotArgs)
	}
	out, err = runBench(t, "run", "--task", "shell-conditional", "--repo", root)
	if err != nil || !strings.Contains(out, "FAIL") {
		t.Fatalf("scoring failure not reported: %q %v", out, err)
	}
	out, err = runBench(t, "results", "--recent", "2")
	if err != nil || !strings.Contains(out, "haiku") || !strings.Contains(out, "lite") || !strings.Contains(out, "#2") {
		t.Fatalf("results: %q %v", out, err)
	}
	if _, err := runBench(t, "run", "--docs", "huge"); err == nil {
		t.Error("bad --docs accepted")
	}
	if _, err := runBench(t, "run", "--task", "nope"); err == nil {
		t.Error("unknown task accepted")
	}
}

func TestBenchRunReadModeRunsFixtureTasksAndReportsTurns(t *testing.T) {
	t.Setenv("HARNEZ_BENCH_DIR", filepath.Join(t.TempDir(), "bench"))
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	benchRunner = func(_ context.Context, dir, _ string, _ ...string) ([]byte, error) {
		if _, err := os.Stat(filepath.Join(dir, "docs", "RUNBOOK.md")); err != nil {
			t.Errorf("fixture not staged: %v", err)
		}
		return []byte(`{"result":"17","num_turns":3,"usage":{}}`), nil
	}
	defer func() { benchRunner = nil }()
	if _, err := runBench(t, "--setup"); err != nil {
		t.Fatal(err)
	}
	out, err := runBench(t, "run", "--read", "auto", "--repo", root)
	if err != nil || strings.Count(out, "read:auto") != 2 || !strings.Contains(out, "turns=3") {
		t.Fatalf("read run: %q %v", out, err)
	}
	if out, err = runBench(t, "results"); err != nil || !strings.Contains(out, "auto") || !strings.Contains(out, "3.0") {
		t.Fatalf("results lack read mode/turns: %q %v", out, err)
	}
	if _, err := runBench(t, "run", "--read", "auto", "--task", "hello", "--repo", root); err == nil {
		t.Error("docs task accepted under --read")
	}
	if _, err := runBench(t, "run", "--read", "bogus"); err == nil {
		t.Error("bad --read accepted")
	}
}

func TestBenchRunYamlAndMultiFlags(t *testing.T) {
	t.Setenv("HARNEZ_BENCH_DIR", filepath.Join(t.TempDir(), "bench"))
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	var files []string
	benchRunner = func(_ context.Context, dir, _ string, _ ...string) ([]byte, error) {
		entries, _ := os.ReadDir(filepath.Join(dir, "docs"))
		files = files[:0]
		for _, e := range entries {
			files = append(files, e.Name())
		}
		return []byte(`{"result":"17","num_turns":2,"usage":{}}`), nil
	}
	defer func() { benchRunner = nil }()
	if _, err := runBench(t, "--setup"); err != nil {
		t.Fatal(err)
	}
	out, err := runBench(t, "run", "--read", "text", "--yaml", "--multi", "--task", "read-one-fact", "--repo", root)
	if err != nil || !strings.Contains(out, "read:text+yaml+multi5") || len(files) != 5 || !strings.HasSuffix(files[0], ".yaml") {
		t.Fatalf("bare --multi + --yaml: %q %v %v", out, err, files)
	}
	out, err = runBench(t, "run", "--read", "native", "--multi=3", "--task", "read-one-fact", "--repo", root)
	if err != nil || !strings.Contains(out, "read:native+multi3") || len(files) != 3 {
		t.Fatalf("--multi=3: %q %v %v", out, err, files)
	}
	if out, err = runBench(t, "results"); err != nil || !strings.Contains(out, "text+yaml+multi5") {
		t.Fatalf("results lack variant: %q %v", out, err)
	}
	for _, args := range [][]string{{"run", "--yaml"}, {"run", "--multi=3"}, {"run", "--read", "text", "--multi=27"}} {
		if _, err := runBench(t, args...); err == nil {
			t.Errorf("%v accepted", args)
		}
	}
}
