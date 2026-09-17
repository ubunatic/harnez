package subagent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/readcard"
	"ubunatic.com/harnez/internal/subagent"
)

func TestParseDocMode(t *testing.T) {
	tests := []struct {
		input   string
		want    subagent.DocMode
		wantErr bool
	}{
		{"vision", subagent.DocModeVision, false},
		{"img", subagent.DocModeVision, false},
		{"lite", subagent.DocModeLite, false},
		{"full", subagent.DocModeFull, false},
		{"", subagent.DocModeFull, false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		got, err := subagent.ParseDocMode(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseDocMode(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseDocMode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStageSubagentContext_VisionMode(t *testing.T) {
	tmpDir := t.TempDir()

	res, err := subagent.StageSubagentContext(tmpDir, subagent.StageOptions{
		DocMode: subagent.DocModeVision,
		Task:    "Implement deploy-check.sh",
		OutFile: "deploy-check.sh",
	})
	if err != nil {
		t.Fatalf("StageSubagentContext failed: %v", err)
	}

	if res.DocMode != subagent.DocModeVision {
		t.Errorf("expected DocModeVision, got %s", res.DocMode)
	}
	if !strings.Contains(res.Prompt, "STYLE_GUIDE.png") {
		t.Errorf("prompt missing STYLE_GUIDE.png reference: %s", res.Prompt)
	}
	if !strings.Contains(res.Prompt, "Implement deploy-check.sh") {
		t.Errorf("prompt missing task: %s", res.Prompt)
	}

	stagedCard := filepath.Join(tmpDir, "STYLE_GUIDE.png")
	if _, err := os.Stat(stagedCard); err != nil {
		t.Fatalf("expected STYLE_GUIDE.png in workspace, got error: %v", err)
	}

	checkRes, err := readcard.CheckCard(stagedCard, 1568)
	if err != nil {
		t.Fatalf("CheckCard on staged card failed: %v", err)
	}
	if !checkRes.Passed {
		t.Errorf("staged card failed check: %+v", checkRes)
	}
}

func TestStageSubagentContext_LiteAndFullMode(t *testing.T) {
	tmpDir := t.TempDir()

	res, err := subagent.StageSubagentContext(tmpDir, subagent.StageOptions{
		DocMode: subagent.DocModeLite,
		Task:    "Build runner",
		OutFile: "runner.go",
	})
	if err != nil {
		t.Fatalf("StageSubagentContext failed: %v", err)
	}

	if res.DocMode != subagent.DocModeLite {
		t.Errorf("expected DocModeLite, got %s", res.DocMode)
	}
	if strings.Contains(res.Prompt, "STYLE_GUIDE.png") {
		t.Errorf("lite mode prompt should not contain STYLE_GUIDE.png: %s", res.Prompt)
	}
	if !strings.Contains(res.Prompt, "Build runner") {
		t.Errorf("prompt missing task: %s", res.Prompt)
	}
}

func TestAutoDocProfilesAndExplicitOverrides(t *testing.T) {
	for _, tc := range []struct {
		provider readcard.Provider
		want     subagent.DocMode
	}{
		{readcard.ProviderClaude, subagent.DocModeVision}, {readcard.ProviderOpenAI, subagent.DocModeVision},
		{readcard.ProviderGemini, subagent.DocModeLite}, {readcard.ProviderLocal, subagent.DocModeLite}, {readcard.ProviderUnknown, subagent.DocModeLite},
	} {
		res, err := subagent.StageSubagentContext(t.TempDir(), subagent.StageOptions{DocMode: subagent.DocModeAuto, Provider: tc.provider, Task: "task", LiteRules: "Specific rule"})
		if err != nil {
			t.Fatal(err)
		}
		if res.DocMode != tc.want {
			t.Fatalf("%s got %s", tc.provider, res.DocMode)
		}
		if tc.want == subagent.DocModeLite && !strings.Contains(res.Prompt, "Specific rule") {
			t.Fatal("lite rules missing")
		}
		if !strings.Contains(res.Prompt, "harnez read --auto") {
			t.Fatal("read handoff missing")
		}
	}
	res, err := subagent.StageSubagentContext(t.TempDir(), subagent.StageOptions{DocMode: subagent.DocModeLite, Provider: readcard.ProviderClaude})
	if err != nil || res.DocMode != subagent.DocModeLite {
		t.Fatal("explicit override ignored")
	}
}
