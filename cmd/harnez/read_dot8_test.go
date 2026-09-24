//go:build dot8

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadCmd_Dot8OnHoldByDefault(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "dot8_hold.md")
	if err := os.WriteFile(testFile, []byte("# Hello\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	for _, args := range [][]string{
		{"--dot8", testFile},
		{"--dot8-colors", "red-white", testFile},
		{"--dot8-pitch", "4", testFile},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := newReadCmd()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(args)

			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error for %v without HARNEZ_DOT8=1", args)
			}
			if !strings.Contains(err.Error(), "on hold") || !strings.Contains(err.Error(), "HARNEZ_DOT8") {
				t.Errorf("error %q does not mention on hold / HARNEZ_DOT8", err.Error())
			}
		})
	}
}

func TestReadCmd_Dot8FlagsHiddenAndPlainReadUnaffected(t *testing.T) {
	cmd := newReadCmd()
	for _, name := range []string{"dot8", "dot8-colors", "dot8-pitch"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("flag %q not registered", name)
		}
		if !f.Hidden {
			t.Errorf("flag %q should be hidden", name)
		}
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "plain.md")
	if err := os.WriteFile(testFile, []byte("# Hello\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	plainCmd := newReadCmd()
	var buf bytes.Buffer
	plainCmd.SetOut(&buf)
	plainCmd.SetErr(&buf)
	plainCmd.SetArgs([]string{testFile})
	if err := plainCmd.Execute(); err != nil {
		t.Fatalf("plain read failed: %v", err)
	}
	if !strings.Contains(buf.String(), "Hello") {
		t.Errorf("expected source text in output, got: %s", buf.String())
	}
}

func TestReadCmd_Dot8Flags(t *testing.T) {
	t.Setenv("HARNEZ_DOT8", "1")
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
