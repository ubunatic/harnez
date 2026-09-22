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
	if !strings.Contains(out, "See @") || !strings.Contains(out, imgOut) {
		t.Errorf("expected render notification, got:\n%s", out)
	}
	if strings.Contains(out, "Token Breakdown:") {
		t.Errorf("token breakdown should be hidden by default, got:\n%s", out)
	}

	cmd = newReadCmd()
	buf.Reset()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"-I", "--tokens", "-o", imgOut, "--columns=1", testFile})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("read -I --tokens failed: %v", err)
	}
	if !strings.Contains(buf.String(), "Token Breakdown:") {
		t.Errorf("expected token breakdown with --tokens, got:\n%s", buf.String())
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

func TestReadCmd_WrapFlag(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "wrap_test.go")
	content := `package main
func main() {
	println("` + strings.Repeat("A_very_long_string_literal_sequence_", 5) + `")
}`
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	for _, wrapMode := range []string{"soft", "truncate"} {
		imgOut := filepath.Join(tmpDir, "out_wrap_"+wrapMode+".png")
		cmd := newReadCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"-I", "--wrap", wrapMode, "-o", imgOut, testFile})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("read -I --wrap=%s failed: %v", wrapMode, err)
		}

		if _, err := os.Stat(imgOut); err != nil {
			t.Errorf("expected generated image at %s for --wrap=%s: %v", imgOut, wrapMode, err)
		}
	}
}

func TestReadCmd_Dot8Flags(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "dot8_test.md")
	content := `# Hello World
This is a test.
Lines 123.`
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	tests := []struct {
		name      string
		args      []string
		wantErr   bool
		checkFunc func(t *testing.T, out string)
	}{
		{
			name:    "bare --dot8 creates card",
			args:    []string{"-I", "--dot8", "-o", filepath.Join(tmpDir, "bare.png"), testFile},
			wantErr: false,
			checkFunc: func(t *testing.T, out string) {
				if !strings.Contains(out, ".png") {
					t.Errorf("expected PNG output, got: %s", out)
				}
			},
		},
		{
			name:    "--dot8=native with already-encoded input",
			args:    []string{"-I", "--dot8=native", "-o", filepath.Join(tmpDir, "native.png"), testFile},
			wantErr: false,
		},
		{
			name:    "--dot8=invalid should error",
			args:    []string{"-I", "--dot8=invalid", testFile},
			wantErr: true,
		},
		{
			name:    "--dot8 text mode returns source",
			args:    []string{"--dot8", testFile},
			wantErr: false,
			checkFunc: func(t *testing.T, out string) {
				if !strings.Contains(out, "Hello World") {
					t.Errorf("expected source text in output, got: %s", out)
				}
			},
		},
		{
			name:    "--dot8 with -L returns decoded text",
			args:    []string{"--dot8", "-L", "1:2", testFile},
			wantErr: false,
			checkFunc: func(t *testing.T, out string) {
				if !strings.Contains(out, "Hello World") {
					t.Errorf("expected source text in output, got: %s", out)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newReadCmd()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v, output: %s", err, tt.wantErr, buf.String())
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, buf.String())
			}

			// Check that files were created (for image output)
			for _, arg := range tt.args {
				if strings.HasSuffix(arg, ".png") {
					if _, err := os.Stat(arg); err != nil && !tt.wantErr {
						t.Errorf("expected PNG file %s to exist: %v", arg, err)
					}
				}
			}
		})
	}
}
