package mode_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/markdown"
	"ubunatic.com/harnez/internal/mode"
)

func TestParseTier(t *testing.T) {
	tests := []struct {
		input    string
		expected mode.Tier
		wantErr  bool
	}{
		{"lite", mode.TierLite, false},
		{"1", mode.TierLite, false},
		{"level1", mode.TierLite, false},
		{"L1", mode.TierLite, false},
		{"std", mode.TierStandard, false},
		{"standard", mode.TierStandard, false},
		{"2", mode.TierStandard, false},
		{"ultra", mode.TierUltra, false},
		{"3", mode.TierUltra, false},
		{"off", mode.TierOff, false},
		{"reset", mode.TierOff, false},
		{"default", mode.TierOff, false},
		{"none", mode.TierOff, false},
		{"0", mode.TierOff, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		got, err := mode.ParseTier(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseTier(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.expected {
			t.Errorf("ParseTier(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestSetMode_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	agentsFile := filepath.Join(tmpDir, "AGENTS.md")

	initialContent := "# Project\n\nSome introductory text.\n"
	if err := os.WriteFile(agentsFile, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Switch to Lite
	res, err := mode.SetMode(mode.TierLite, mode.Options{FilePath: agentsFile})
	if err != nil {
		t.Fatalf("SetMode(TierLite) failed: %v", err)
	}
	if !res.Changed {
		t.Errorf("Expected Changed=true on initial set")
	}
	if !strings.Contains(res.Directive, "Concise Lite (Level 1)") {
		t.Errorf("Directive missing Concise Lite: %s", res.Directive)
	}
	content, _ := os.ReadFile(agentsFile)
	if !markdown.ContainsSection(agentsFile, mode.ConciseModeSection) {
		t.Errorf("AGENTS.md missing Concise Mode section:\n%s", string(content))
	}
	if !strings.Contains(string(content), "Concise Lite (Level 1)") {
		t.Errorf("AGENTS.md missing Lite tier line:\n%s", string(content))
	}

	// 2. Switch to Standard (update)
	res, err = mode.SetMode(mode.TierStandard, mode.Options{FilePath: agentsFile})
	if err != nil {
		t.Fatalf("SetMode(TierStandard) failed: %v", err)
	}
	if !res.Changed {
		t.Errorf("Expected Changed=true on tier switch")
	}
	if !strings.Contains(res.Directive, "Concise Standard (Level 2)") {
		t.Errorf("Directive missing Concise Standard: %s", res.Directive)
	}
	content, _ = os.ReadFile(agentsFile)
	if !strings.Contains(string(content), "Concise Standard (Level 2)") {
		t.Errorf("AGENTS.md missing Standard tier line:\n%s", string(content))
	}
	if strings.Contains(string(content), "Concise Lite") {
		t.Errorf("AGENTS.md still contains old Lite tier line:\n%s", string(content))
	}

	// 3. Switch to Ultra
	res, err = mode.SetMode(mode.TierUltra, mode.Options{FilePath: agentsFile})
	if err != nil {
		t.Fatalf("SetMode(TierUltra) failed: %v", err)
	}
	if !res.Changed {
		t.Errorf("Expected Changed=true on tier switch")
	}
	content, _ = os.ReadFile(agentsFile)
	if !strings.Contains(string(content), "Concise Ultra (Level 3)") {
		t.Errorf("AGENTS.md missing Ultra tier line:\n%s", string(content))
	}

	// 4. Dry run to Off
	res, err = mode.SetMode(mode.TierOff, mode.Options{FilePath: agentsFile, DryRun: true})
	if err != nil {
		t.Fatalf("SetMode(TierOff, DryRun=true) failed: %v", err)
	}
	if !strings.Contains(res.FileUpdate, "would remove") {
		t.Errorf("Expected dry run note, got: %s", res.FileUpdate)
	}
	// Verify file was NOT modified
	content, _ = os.ReadFile(agentsFile)
	if !markdown.ContainsSection(agentsFile, mode.ConciseModeSection) {
		t.Errorf("AGENTS.md section was removed during dry run!")
	}

	// 5. Turn Off (remove section)
	res, err = mode.SetMode(mode.TierOff, mode.Options{FilePath: agentsFile})
	if err != nil {
		t.Fatalf("SetMode(TierOff) failed: %v", err)
	}
	if !res.Changed {
		t.Errorf("Expected Changed=true when removing section")
	}
	if !strings.Contains(res.Directive, "ConciseMode OFF") {
		t.Errorf("Directive missing OFF notice: %s", res.Directive)
	}
	content, _ = os.ReadFile(agentsFile)
	if markdown.ContainsSection(agentsFile, mode.ConciseModeSection) {
		t.Errorf("AGENTS.md still contains Concise Mode section after off:\n%s", string(content))
	}
	if !strings.Contains(string(content), "Some introductory text.") {
		t.Errorf("Base content in AGENTS.md was corrupted:\n%s", string(content))
	}
}
