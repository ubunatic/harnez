package main

import (
	"bytes"
	"context"
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
