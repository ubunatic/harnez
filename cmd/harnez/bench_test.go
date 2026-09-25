package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/bench"
)

func runBench(t *testing.T, args ...string) (string, error) {
	t.Helper()
	stdout, stderr, err := runBenchStreams(t, args...)
	return stdout + stderr, err
}

func runBenchStreams(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := newBenchCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return stdout.String(), stderr.String(), err
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
	if err != nil || strings.Count(out, "auto") != 6 || !strings.Contains(out, "3") {
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

func TestBenchRunCardModeForcesImageRead(t *testing.T) {
	t.Setenv("HARNEZ_BENCH_DIR", filepath.Join(t.TempDir(), "bench"))
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	benchRunner = func(_ context.Context, dir, _ string, _ ...string) ([]byte, error) {
		agents, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
		if err != nil || !strings.Contains(string(agents), "harnez read -I") {
			t.Errorf("card instruction = %q, %v", agents, err)
		}
		return []byte(`{"result":"17","usage":{}}`), nil
	}
	defer func() { benchRunner = nil }()
	if _, err := runBench(t, "--setup"); err != nil {
		t.Fatal(err)
	}
	out, err := runBench(t, "run", "--read", "card", "--task", "read-one-fact", "--repo", root)
	if err != nil || !strings.Contains(out, "card:") || !strings.Contains(out, "PASS") {
		t.Fatalf("card read run: %q %v", out, err)
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
	if err != nil || !strings.Contains(out, "text+yaml+multi5") || len(files) != 5 || !strings.HasSuffix(files[0], ".yaml") {
		t.Fatalf("bare --multi + --yaml: %q %v %v", out, err, files)
	}
	out, err = runBench(t, "run", "--read", "native", "--multi=3", "--task", "read-one-fact", "--repo", root)
	if err != nil || !strings.Contains(out, "native+multi3") || len(files) != 3 {
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

func TestBenchCardFlagNeedsReadAuto(t *testing.T) {
	t.Setenv("HARNEZ_BENCH_DIR", filepath.Join(t.TempDir(), "bench"))
	if _, err := runBench(t, "--setup"); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"run", "--card=--style=compact"}, {"run", "--read", "text", "--card=--style=compact"}} {
		if _, err := runBench(t, args...); err == nil {
			t.Errorf("%v accepted", args)
		}
	}
}

func TestBenchRunModelReadMatrixAndLiteDefault(t *testing.T) {
	t.Setenv("HARNEZ_BENCH_DIR", filepath.Join(t.TempDir(), "bench"))
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	var calls []string
	benchRunner = func(_ context.Context, _, name string, args ...string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if name == "codex" {
			return []byte("{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"17\"}}\n{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":7,\"output_tokens\":1}}\n"), nil
		}
		return []byte(`{"result":"17","num_turns":2,"usage":{"input_tokens":7}}`), nil
	}
	defer func() { benchRunner = nil }()
	if _, err := runBench(t, "--setup"); err != nil {
		t.Fatal(err)
	}
	out, err := runBench(t, "run", "--task", "read-one-fact", "--read", "text,card", "--model", "claude:haiku:low,codex:luna:low", "--repo", root)
	if err != nil {
		t.Fatalf("matrix run: %v", err)
	}
	if len(calls) != 4 || !strings.Contains(out, "model") || strings.Count(out, "PASS") != 4 || !strings.Contains(out, "input") || !strings.Contains(out, "duration") {
		t.Fatalf("matrix output/calls: %q %v", out, calls)
	}
	results, err := runBench(t, "results")
	if err != nil || !strings.Contains(results, "lite") {
		t.Errorf("--docs default is not lite: %q %v", results, err)
	}
	if !strings.Contains(calls[0], "--effort low") {
		t.Errorf("model tier not passed: %q", calls[0])
	}
}

func TestBenchRunSeparatesTableAndProgress(t *testing.T) {
	t.Setenv("HARNEZ_BENCH_DIR", filepath.Join(t.TempDir(), "bench"))
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	var calls int
	benchRunner = func(_ context.Context, _, name string, _ ...string) ([]byte, error) {
		calls++
		if name == "codex" {
			return []byte("{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"17\"}}\n{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":15600}}\n"), nil
		}
		return []byte(`{"result":"17","num_turns":2,"usage":{"input_tokens":15600}}`), nil
	}
	defer func() { benchRunner = nil }()
	if _, err := runBench(t, "--setup"); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runBenchStreams(t, "run", "--read", "text", "--task", "read-one-fact", "--model", "claude:haiku:low,codex:luna:low", "--repo", root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stdout, "model ") || strings.Contains(stdout, "fixture:") || strings.Contains(stdout, "[1/2]") {
		t.Errorf("stdout is not table-only: %q", stdout)
	}
	if !strings.Contains(stderr, "read-one-fact (find one deep fact") || !strings.Contains(stderr, "fixture: docs/RUNBOOK.md (") || !strings.Contains(stderr, "question: What is the retry limit") {
		t.Errorf("preamble missing details: %q", stderr)
	}
	if strings.Count(stderr, "fixture: docs/RUNBOOK.md") != 1 || strings.Count(stderr, "[1/2]") != 1 || strings.Count(stderr, "[2/2]") != 1 || strings.Count(stderr, " turns,") != 2 {
		t.Errorf("progress/preamble counts wrong: %q", stderr)
	}
	if calls != 2 {
		t.Errorf("fake runner calls = %d, want 2", calls)
	}
}

func TestReadLangSummaryPreambleShowsFixturesAndModePrompt(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	spec, err := bench.LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := spec.Select([]string{"read-lang-summary"})
	if err != nil {
		t.Fatal(err)
	}
	preamble, err := benchPreamble(spec, tasks[0], bench.Condition{Read: "native"}, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Go.md", "Make.md", "ManPages.md", "Bash.md", "Git.md", "Markdown.md"} {
		if !strings.Contains(preamble, "fixture: docs/lang/"+name+" (") {
			t.Errorf("preamble omits %s: %s", name, preamble)
		}
	}
	if got := strings.Count(preamble, "fixture:"); got != 6 {
		t.Errorf("preamble has %d fixture lines, want 6", got)
	}
	if !strings.Contains(preamble, "question: "+bench.TaskPrompt(tasks[0], "native")) {
		t.Errorf("preamble omits native prompt: %s", preamble)
	}
}

func TestBenchCardPreambleFailureDoesNotCallAgent(t *testing.T) {
	t.Setenv("HARNEZ_BENCH_DIR", filepath.Join(t.TempDir(), "bench"))
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	var calls int
	benchRunner = func(context.Context, string, string, ...string) ([]byte, error) {
		calls++
		return []byte(`{"result":"17"}`), nil
	}
	defer func() { benchRunner = nil }()
	oldRenderer := benchCardRenderer
	benchCardRenderer = func(string, string) (string, error) {
		return "", fmt.Errorf("fake card failure")
	}
	defer func() { benchCardRenderer = oldRenderer }()
	if _, err := runBench(t, "--setup"); err != nil {
		t.Fatal(err)
	}
	_, _, err := runBenchStreams(t, "run", "--read", "card", "--task", "read-one-fact", "--repo", root)
	if err == nil || !strings.Contains(err.Error(), "fake card failure") {
		t.Fatalf("card failure = %v", err)
	}
	if calls != 0 {
		t.Fatalf("runner called %d times after failed preamble", calls)
	}
}
