package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/decide"
)

func TestCompactCmd_SummaryAndJSONL(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "transcript.jsonl")
	content := `{"role":"user","content":"Initial goal"}
{"role":"tool","tool_name":"ls","tool_output":"file1\nfile2\nfile3"}
{"role":"assistant","content":"Recent response"}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	callTrue := 0.90
	resultFalse := 0.10
	mock := &testMockBackend{
		fn: func(ctx context.Context, req *decide.Request) (*decide.Response, error) {
			return &decide.Response{
				Model: "jev-test",
				Answers: map[string]decide.Answer{
					"cand_0_keep_call":   {Type: decide.TypeNoul, Noul: &callTrue},
					"cand_0_keep_result": {Type: decide.TypeNoul, Noul: &resultFalse},
				},
			}, nil
		},
	}

	// 1. Test summary mode
	var buf bytes.Buffer
	opts := compactOpts{
		transcript: transcriptPath,
		format:     "summary",
		pinRecent:  1,
	}

	err := runCompact(context.Background(), &buf, mock, &opts)
	if err != nil {
		t.Fatalf("runCompact summary failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Compaction Summary") {
		t.Errorf("expected header in summary output, got: %s", out)
	}
	if !strings.Contains(out, "Original entries:   3") {
		t.Errorf("expected original entries in summary, got: %s", out)
	}

	// 2. Test JSONL mode with output file
	outPath := filepath.Join(dir, "compacted.jsonl")
	buf.Reset()
	opts = compactOpts{
		transcript: transcriptPath,
		output:     outPath,
		format:     "jsonl",
		pinRecent:  1,
	}

	err = runCompact(context.Background(), &buf, mock, &opts)
	if err != nil {
		t.Fatalf("runCompact jsonl failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if !strings.Contains(string(data), "Initial goal") {
		t.Errorf("expected initial goal in output file, got: %s", string(data))
	}

	// 3. Test Summary with Output File
	summaryOutPath := filepath.Join(dir, "summary.txt")
	opts = compactOpts{
		transcript: transcriptPath,
		output:     summaryOutPath,
		format:     "summary",
		pinRecent:  1,
	}
	err = runCompact(context.Background(), &buf, mock, &opts)
	if err != nil {
		t.Fatalf("runCompact summary to file failed: %v", err)
	}
	summaryData, err := os.ReadFile(summaryOutPath)
	if err != nil {
		t.Fatalf("failed to read summary output file: %v", err)
	}
	if !strings.Contains(string(summaryData), "Compaction Summary") {
		t.Errorf("expected header in summary file, got: %s", string(summaryData))
	}

	// 4. Test In-Place mode
	inPlacePath := filepath.Join(dir, "inplace.jsonl")
	if err := os.WriteFile(inPlacePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	opts = compactOpts{
		transcript: inPlacePath,
		format:     "jsonl",
		inPlace:    true,
		pinRecent:  1,
	}
	err = runCompact(context.Background(), &buf, mock, &opts)
	if err != nil {
		t.Fatalf("runCompact in-place failed: %v", err)
	}
	inPlaceData, err := os.ReadFile(inPlacePath)
	if err != nil {
		t.Fatalf("failed to read in-place file: %v", err)
	}
	if !strings.Contains(string(inPlaceData), "Initial goal") {
		t.Errorf("expected content in in-place file, got: %s", string(inPlaceData))
	}
}

func TestCompactCmd_FlagValidation(t *testing.T) {
	cmd := newCompactCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// In-place combined with output should fail
	cmd.SetArgs([]string{"--in-place", "--output", "out.jsonl", "input.jsonl"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "cannot combine") {
		t.Errorf("expected cannot combine error, got: %v", err)
	}

	// Out-of-bounds threshold (> 1.0) should fail
	cmd = newCompactCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--threshold", "1.5", "input.jsonl"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "out of bounds") {
		t.Errorf("expected out of bounds error, got: %v", err)
	}

	// Out-of-bounds threshold (< 0.0) should fail
	cmd = newCompactCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--threshold", "-0.2", "input.jsonl"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "out of bounds") {
		t.Errorf("expected out of bounds error, got: %v", err)
	}

	// Extra positional arguments should fail
	cmd = newCompactCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"file1.jsonl", "file2.jsonl"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error for extra positional arguments, got nil")
	}

	// In-place with stdin should fail
	mock := &testMockBackend{}
	buf.Reset()
	opts := compactOpts{
		transcript: "-",
		inPlace:    true,
	}
	if err := runCompact(context.Background(), &buf, mock, &opts); err == nil || !strings.Contains(err.Error(), "cannot use --in-place") {
		t.Errorf("expected in-place stdin error, got: %v", err)
	}

	// Default empty format should default to jsonl and produce output
	dir := t.TempDir()
	sampleTranscript := filepath.Join(dir, "sample.jsonl")
	if err := os.WriteFile(sampleTranscript, []byte(`{"role":"user","content":"ping"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	opts = compactOpts{
		transcript: sampleTranscript,
		format:     "", // empty format should default to jsonl
	}
	if err := runCompact(context.Background(), &buf, mock, &opts); err != nil {
		t.Fatalf("runCompact with empty format failed: %v", err)
	}
	if !strings.Contains(buf.String(), `"content":"ping"`) {
		t.Errorf("expected jsonl output with default format, got: %s", buf.String())
	}
}
