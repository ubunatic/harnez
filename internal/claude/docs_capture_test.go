package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureDocsDriftReportsChangedMissingExtraAndIdentical(t *testing.T) {
	sourceDir := t.TempDir()
	repoDir := t.TempDir()
	writeCaptureFile(t, sourceDir, "docs/Go.md", "source go\n")
	writeCaptureFile(t, sourceDir, "docs/Make.md", "source make\n")
	writeCaptureFile(t, sourceDir, "docs/Git.md", "same\n")
	writeCaptureFile(t, repoDir, "docs/Go.md", "local go\n")
	writeCaptureFile(t, repoDir, "docs/Git.md", "same\n")
	writeCaptureFile(t, repoDir, "docs/Extra.md", "<!-- harnez:bundled -->\nextra\n")

	cfg := &Config{
		Dir:  sourceDir,
		FS:   os.DirFS(sourceDir),
		Docs: []string{"go", "make", "git"},
		AgentsMD: AgentsMD{Languages: map[string]Language{
			"go":   {Source: "docs/Go.md", Local: "docs/Go.md"},
			"make": {Source: "docs/Make.md", Local: "docs/Make.md"},
			"git":  {Source: "docs/Git.md", Local: "docs/Git.md"},
		}},
	}
	out := filepath.Join(repoDir, "issues", "inbox", "drift.md")
	path, changed, err := CaptureDocsDrift(repoDir, out, cfg)
	if err != nil {
		t.Fatalf("CaptureDocsDrift failed: %v", err)
	}
	if path != out || !changed {
		t.Fatalf("CaptureDocsDrift = (%q, %v), want (%q, true)", path, changed, out)
	}
	report, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(report)
	for _, want := range []string{
		"source_repo: '" + sourceDir + "'",
		"  - 'docs/Go.md'",
		"## `docs/Go.md` (changed)",
		"## `docs/Make.md` (missing)",
		"## `docs/Git.md` (identical)",
		"## `docs/Extra.md` (extra)",
		"Summary: 1 changed, 1 missing, 1 extra, 1 identical.",
		"-source go",
		"+local go",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("report does not contain %q:\n%s", want, text)
		}
	}
}

func TestCaptureDocsDriftIdenticalReturnsFalse(t *testing.T) {
	sourceDir := t.TempDir()
	repoDir := t.TempDir()
	writeCaptureFile(t, sourceDir, "docs/Go.md", "same\n")
	writeCaptureFile(t, repoDir, "docs/Go.md", "same\n")
	cfg := &Config{
		Dir:  sourceDir,
		FS:   os.DirFS(sourceDir),
		Docs: []string{"go"},
		AgentsMD: AgentsMD{Languages: map[string]Language{
			"go": {Source: "docs/Go.md", Local: "docs/Go.md"},
		}},
	}
	_, changed, err := CaptureDocsDrift(repoDir, filepath.Join(repoDir, "report.md"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("identical docs reported as changed")
	}
}

func TestCaptureDocsDriftDefaultRequiresInbox(t *testing.T) {
	sourceDir := t.TempDir()
	repoDir := t.TempDir()
	writeCaptureFile(t, sourceDir, "docs/Go.md", "same\n")
	cfg := &Config{
		Dir:  sourceDir,
		FS:   os.DirFS(sourceDir),
		Docs: []string{"go"},
		AgentsMD: AgentsMD{Languages: map[string]Language{
			"go": {Source: "docs/Go.md", Local: "docs/Go.md"},
		}},
	}
	if _, _, err := CaptureDocsDrift(repoDir, "", cfg); err == nil || !strings.Contains(err.Error(), "use --out") {
		t.Fatalf("expected explicit output guidance, got %v", err)
	}
}

func TestCaptureDocsDriftEmbeddedConfigUsesSiblingHarnezRepo(t *testing.T) {
	projectsDir := t.TempDir()
	sourceDir := filepath.Join(projectsDir, "harnez")
	repoDir := filepath.Join(projectsDir, "app")
	writeCaptureFile(t, sourceDir, "go.mod", "module ubunatic.com/harnez\n")
	writeCaptureFile(t, sourceDir, "config.yaml", "target_dir: ~/.claude\n")
	if err := os.MkdirAll(filepath.Join(sourceDir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeCaptureFile(t, sourceDir, "docs/Go.md", "source go\n")
	writeCaptureFile(t, repoDir, "docs/Go.md", "local go\n")

	cfg := &Config{
		Dir: ".",
		FS:  os.DirFS(sourceDir),
		Docs: []string{
			"go",
		},
		AgentsMD: AgentsMD{Languages: map[string]Language{
			"go": {Source: "docs/Go.md", Local: "docs/Go.md"},
		}},
	}
	path, _, err := CaptureDocsDrift(repoDir, "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := filepath.Join(sourceDir, "issues", "inbox", "managed-docs-drift-")
	if !strings.HasPrefix(path, wantPrefix) {
		t.Fatalf("default report path = %q, want prefix %q", path, wantPrefix)
	}
	report, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "source_repo: '"+sourceDir+"'") {
		t.Fatalf("report did not use sibling harnez repo as source:\n%s", string(report))
	}
}

func TestCaptureDocsDriftEmbeddedConfigFallsBackToLabel(t *testing.T) {
	sourceDir := t.TempDir()
	repoDir := t.TempDir()
	writeCaptureFile(t, sourceDir, "docs/Go.md", "same\n")
	writeCaptureFile(t, repoDir, "docs/Go.md", "same\n")
	cfg := &Config{
		Dir:  ".",
		FS:   os.DirFS(sourceDir),
		Docs: []string{"go"},
		AgentsMD: AgentsMD{Languages: map[string]Language{
			"go": {Source: "docs/Go.md", Local: "docs/Go.md"},
		}},
	}
	out := filepath.Join(repoDir, "report.md")
	if _, _, err := CaptureDocsDrift(repoDir, out, cfg); err != nil {
		t.Fatal(err)
	}
	report, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "source_repo: 'embedded harnez config'") {
		t.Fatalf("report did not use embedded fallback label:\n%s", string(report))
	}
}

func writeCaptureFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
