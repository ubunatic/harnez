package assess

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input string
		min   int
		max   int
	}{
		{"", 0, 0},
		{"hello", 1, 3},
		{strings.Repeat("a", 400), 90, 110},
	}

	for _, tt := range tests {
		got := EstimateTokens([]byte(tt.input))
		if got < tt.min || got > tt.max {
			t.Errorf("EstimateTokens(%q) = %d; want between %d and %d", tt.input, got, tt.min, tt.max)
		}
	}
}

func TestCountLines(t *testing.T) {
	content := []byte("line 1\n\nline 2\n   \nline 3\n")
	lines := CountLines(content)
	if lines != 3 {
		t.Errorf("CountLines() = %d; want 3", lines)
	}
}

func TestClassifyFile(t *testing.T) {
	tests := []struct {
		path     string
		wantCat  FileCategory
		wantLang string
		isTest   bool
	}{
		{"main.go", CategoryCode, "Go", false},
		{"main_test.go", CategoryTests, "Go", true},
		{"scripts/run.sh", CategoryCode, "Bash", false},
		{"docs/README.md", CategoryDocs, "Markdown", false},
		{"AGENTS.md", CategoryDocs, "Markdown", false},
		{"config.yaml", CategoryConfig, "YAML", false},
	}

	for _, tt := range tests {
		cat, lang, isTest, _, _ := ClassifyFile(tt.path)
		if cat != tt.wantCat {
			t.Errorf("ClassifyFile(%q) cat = %v; want %v", tt.path, cat, tt.wantCat)
		}
		if lang != tt.wantLang {
			t.Errorf("ClassifyFile(%q) lang = %v; want %v", tt.path, lang, tt.wantLang)
		}
		if isTest != tt.isTest {
			t.Errorf("ClassifyFile(%q) isTest = %v; want %v", tt.path, isTest, tt.isTest)
		}
	}
}

func TestAssessPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	goCode := strings.Repeat("fmt.Println(\"test\")\n", 50)
	goTest := strings.Repeat("t.Log(\"test\")\n", 30)
	bashOversized := "#!/bin/bash\n" + strings.Repeat("echo \"step\"\n", 160)
	doc := strings.Repeat("# Documentation\nDetails\n", 20)

	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(goCode), 0644)
	os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte(goTest), 0644)
	os.WriteFile(filepath.Join(tmpDir, "tool.sh"), []byte(bashOversized), 0755)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte(doc), 0644)

	report, err := AssessPath(tmpDir)
	if err != nil {
		t.Fatalf("AssessPath error: %v", err)
	}

	if report.TotalFiles != 4 {
		t.Errorf("TotalFiles = %d; want 4", report.TotalFiles)
	}

	// Check bash warning triggered
	hasBashWarning := false
	for _, w := range report.Warnings {
		if strings.Contains(w, "Bash script exceeds") {
			hasBashWarning = true
			break
		}
	}
	if !hasBashWarning {
		t.Errorf("expected Bash warning in report warnings: %v", report.Warnings)
	}

	// Render text test
	text := RenderText(report)
	if !strings.Contains(text, "Repository Assessment") {
		t.Errorf("RenderText output missing header: %s", text)
	}

	// Render JSON test
	jsonOut, err := RenderJSON(report)
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	if !strings.Contains(jsonOut, `"total_files": 4`) {
		t.Errorf("RenderJSON output unexpected: %s", jsonOut)
	}
}

func TestAssessSingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "script.sh")
	os.WriteFile(filePath, []byte("#!/bin/bash\necho ok\n"), 0755)

	report, err := AssessPath(filePath)
	if err != nil {
		t.Fatalf("AssessPath single file error: %v", err)
	}
	if !report.IsFile {
		t.Errorf("expected IsFile to be true")
	}
	if report.TotalLines != 2 {
		t.Errorf("TotalLines = %d; want 2", report.TotalLines)
	}

	text := RenderText(report)
	if !strings.Contains(text, "File Assessment: script.sh") {
		t.Errorf("RenderText single file unexpected: %s", text)
	}
}
