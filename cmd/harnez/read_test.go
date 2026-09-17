package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/readcard"
)

func TestReadCmd_TextMode(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hello.go")
	content := `package main

import "fmt"

func main() {
	fmt.Println("hello from test")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// 1. Basic text read with line numbers
	cmd := newReadCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"-n", testFile})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("read command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "1 │ package main") {
		t.Errorf("expected line numbers in output, got:\n%s", out)
	}
	if !strings.Contains(out, "func main()") {
		t.Errorf("expected func main in output, got:\n%s", out)
	}
}

func TestReadCmd_LineRange(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "lines.txt")
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, "line "+string(rune('A'+i-1)))
	}
	if err := os.WriteFile(testFile, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	cmd := newReadCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--lines", "5:8", "-n", testFile})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("read command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "5 │ line E") {
		t.Errorf("expected line 5 in output, got:\n%s", out)
	}
	if strings.Contains(out, "1 │ line A") || strings.Contains(out, "10 │ line J") {
		t.Errorf("output should be restricted to lines 5-8, got:\n%s", out)
	}
}

func TestReadCmd_ImageMode(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sample.go")
	imgOut := filepath.Join(tmpDir, "sample.png")

	content := `package sample

// Add returns the sum of two integers.
func Add(a, b int) int {
	return a + b
}
`
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	cmd := newReadCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"-I", "-o", imgOut, "--columns=1", testFile})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("read -I failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Rendered:") || !strings.Contains(out, imgOut) {
		t.Errorf("expected render notification, got:\n%s", out)
	}
	if !strings.Contains(out, "Token Breakdown:") {
		t.Errorf("expected token breakdown, got:\n%s", out)
	}

	if _, err := os.Stat(imgOut); err != nil {
		t.Errorf("output image not found at %s: %v", imgOut, err)
	}
}

func TestReadCmd_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "json_sample.py")
	imgOut := filepath.Join(tmpDir, "json_sample.png")

	content := `def compute(x: int) -> int:
    return x * 2
`
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	cmd := newReadCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"-I", "-o", imgOut, "--json", testFile})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("read -I --json failed: %v", err)
	}

	var res readcard.RenderResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse JSON output: %v, raw:\n%s", err, buf.String())
	}

	if len(res.Files) == 0 || res.Files[0] != imgOut {
		t.Errorf("expected files to contain %s, got %+v", imgOut, res.Files)
	}
	if res.TotalLines != 2 {
		t.Errorf("expected 2 total lines, got %d", res.TotalLines)
	}
}

func TestReadCmd_FontFlags(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "font_test.go")
	content := `package main
func main() {
	println("testing retro pixel fonts")
}`
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	for _, fontName := range []string{"pixel", "3x5", "5x8", "6x12", "standard", "8x16", "7x13"} {
		imgOut := filepath.Join(tmpDir, "out_"+fontName+".png")
		cmd := newReadCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"-I", "--font", fontName, "-o", imgOut, testFile})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("read -I --font=%s failed: %v", fontName, err)
		}

		if _, err := os.Stat(imgOut); err != nil {
			t.Errorf("expected generated image at %s for font %s: %v", imgOut, fontName, err)
		}
	}
}

