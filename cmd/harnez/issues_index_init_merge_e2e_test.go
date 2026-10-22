package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitInstallsIssueMergeDriverEndToEnd(t *testing.T) {
	skipShort(t, "builds the CLI and performs real Git merges")

	binDir := t.TempDir()
	bin := filepath.Join(binDir, "harnez")
	build := exec.Command("go", "build", "-o", bin, "./cmd/harnez")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build harnez: %v\n%s", err, out)
	}

	root := t.TempDir()
	home := t.TempDir()
	for _, dir := range []string{"issues", "docs/studies"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeMergeFixture(t, root, "go.mod", "module fixture\n\ngo 1.24\n")
	writeMergeFixture(t, root, "AGENTS.md", "# Fixture\n")
	writeMergeFixture(t, root, "issues/001-base.md", "# 001 — base\n\n**Status**: Open\n")
	writeMergeFixture(t, root, "issues/README.md", "# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n")
	writeMergeFixture(t, root, "docs/README.md", "<!-- managed by harnez index: generated sections are overwritten; only the studies table is generated; other docs tables and prose are hand-written -->\n# Docs\n\nHandwritten preface.\n\n**`docs/studies/`** — case studies\n\n| File | Topic |\n|------|-------|\n")
	writeMergeFixture(t, root, "docs/studies/a.md", "# Base study\n\n**Scope**: base study\n")

	gitRunE2E(t, root, nil, "init", "-q", "-b", "main")
	gitRunE2E(t, root, nil, "config", "user.name", "Harnez E2E")
	gitRunE2E(t, root, nil, "config", "user.email", "harnez-e2e@example.invalid")
	cli := func(args ...string) {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = root
		cmd.Env = e2eEnv(home, binDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("harnez %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	cli("init", "-d", ".")
	config := gitRunE2E(t, root, nil, "config", "--local", "--get", "merge.harnez-issues-index.driver")
	if config != "harnez issues merge-driver %O %A %B %P" {
		t.Fatalf("plain CLI init driver = %q", config)
	}
	attrs, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"issues/README.md merge=harnez-issues-index",
		"issues/BACKLOG.md merge=harnez-issues-index",
		"issues/CLOSED.md merge=harnez-issues-index",
		"docs/README.md merge=harnez-issues-index",
	} {
		if !strings.Contains(string(attrs), line) {
			t.Errorf("plain CLI init omitted %q from .gitattributes", line)
		}
	}
	cli("index", "-d", ".")
	gitRunE2E(t, root, e2eEnv(home, binDir), "add", "-A")
	gitRunE2E(t, root, e2eEnv(home, binDir), "commit", "-qm", "base")

	gitRunE2E(t, root, nil, "switch", "-c", "side")
	writeMergeFixture(t, root, "issues/002-side.md", "# 002 — side ticket\n\n**Status**: Open\n")
	writeMergeFixture(t, root, "issues/004-backlog.md", "# 004 — backlog ticket\n\n**Status**: Backlog\n")
	writeMergeFixture(t, root, "docs/studies/b.md", "# Side study\n\n**Scope**: side study\n")
	cli("index", "-d", ".")
	gitRunE2E(t, root, e2eEnv(home, binDir), "add", "-A")
	gitRunE2E(t, root, e2eEnv(home, binDir), "commit", "-qm", "side sources")

	gitRunE2E(t, root, nil, "switch", "main")
	writeMergeFixture(t, root, "issues/003-main.md", "# 003 — main ticket\n\n**Status**: Open\n")
	writeMergeFixture(t, root, "issues/005-closed.md", "# 005 — closed ticket\n\n**Status**: Closed\n")
	writeMergeFixture(t, root, "docs/studies/c.md", "# Main study\n\n**Scope**: main study\n")
	cli("index", "-d", ".")
	gitRunE2E(t, root, e2eEnv(home, binDir), "add", "-A")
	gitRunE2E(t, root, e2eEnv(home, binDir), "commit", "-qm", "main sources")
	gitRunE2E(t, root, e2eEnv(home, binDir), "merge", "--no-edit", "side")

	for path := range map[string]bool{
		"issues/README.md":  true,
		"issues/BACKLOG.md": true,
		"issues/CLOSED.md":  true,
		"docs/README.md":    true,
	} {
		data := gitShowE2E(t, root, "HEAD:"+path)
		if strings.Contains(string(data), "<<<<<<<") || strings.Contains(string(data), ">>>>>>>") || strings.Contains(string(data), "=======") {
			t.Errorf("merge commit %s retains conflict markers:\n%s", path, data)
		}
	}
	for path, want := range map[string][]string{
		"issues/README.md":  {"side ticket", "main ticket"},
		"issues/BACKLOG.md": {"backlog ticket"},
		"issues/CLOSED.md":  {"closed ticket"},
		"docs/README.md":    {"studies/b.md", "studies/c.md"},
	} {
		data := gitShowE2E(t, root, "HEAD:"+path)
		for _, value := range want {
			if !strings.Contains(string(data), value) {
				t.Errorf("merge commit %s missing merged source %q", path, value)
			}
		}
	}
	if data := gitShowE2E(t, root, "HEAD:issues/README.md"); len(data) == 0 || data[len(data)-1] != '\n' {
		t.Error("merge commit issues/README.md is missing a trailing newline")
	}
	if status := gitRunE2E(t, root, nil, "status", "--porcelain"); status != "" {
		t.Fatalf("post-merge tree is dirty: %s", status)
	}

	// Conflicts in docs/README.md's hand-written area must remain unresolved.
	gitRunE2E(t, root, nil, "switch", "-c", "handwritten")
	editPreface(t, root, "side handwritten edit")
	gitRunE2E(t, root, e2eEnv(home, binDir), "add", "docs/README.md")
	gitRunE2E(t, root, e2eEnv(home, binDir), "commit", "-qm", "side handwritten edit")
	gitRunE2E(t, root, nil, "switch", "main")
	editPreface(t, root, "main handwritten edit")
	gitRunE2E(t, root, e2eEnv(home, binDir), "add", "docs/README.md")
	gitRunE2E(t, root, e2eEnv(home, binDir), "commit", "-qm", "main handwritten edit")
	if got := gitRunE2E(t, root, nil, "check-attr", "merge", "--", "docs/README.md"); !strings.Contains(got, "harnez-issues-index") {
		t.Fatalf("docs README merge attribute = %q", got)
	}
	mergeBase := gitRunE2E(t, root, nil, "merge-base", "main", "handwritten")
	ancestorPath, oursPath, theirsPath := filepath.Join(root, "ancestor.fixture"), filepath.Join(root, "ours.fixture"), filepath.Join(root, "theirs.fixture")
	for path, revision := range map[string]string{
		ancestorPath: mergeBase + ":docs/README.md",
		oursPath:     "main:docs/README.md",
		theirsPath:   "handwritten:docs/README.md",
	} {
		data := gitRunE2E(t, root, nil, "show", revision)
		if err := os.WriteFile(path, []byte(data+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	driverCmd := exec.Command(bin, "issues", "merge-driver", ancestorPath, oursPath, theirsPath, "docs/README.md")
	driverCmd.Dir = root
	driverCmd.Env = e2eEnv(home, binDir)
	if out, err := driverCmd.CombinedOutput(); err == nil {
		t.Fatalf("direct merge-driver hand-written conflict returned success: %s", out)
	}
	cmd := exec.Command("git", "merge", "--no-edit", "handwritten")
	cmd.Dir = root
	cmd.Env = append(e2eEnv(home, binDir), "GIT_TRACE=1")
	mergeOutput, mergeErr := cmd.CombinedOutput()
	if mergeErr == nil {
		t.Fatalf("hand-written docs conflict unexpectedly merged cleanly:\n%s", mergeOutput)
	}
	data, err := os.ReadFile(filepath.Join(root, "docs/README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<<<<<<<") || !strings.Contains(string(data), "side handwritten edit") || !strings.Contains(string(data), "main handwritten edit") {
		t.Fatalf("hand-written conflict was silently resolved (merge output: %s):\n%s", mergeOutput, data)
	}
}

func writeMergeFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func editPreface(t *testing.T, root, value string) {
	t.Helper()
	path := filepath.Join(root, "docs/README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), "Handwritten preface.", value, 1)
	if updated == string(data) {
		t.Fatal("hand-written preface missing")
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func e2eEnv(home, binDir string) []string {
	env := os.Environ()
	filtered := env[:0]
	for _, entry := range env {
		if !strings.HasPrefix(entry, "HOME=") && !strings.HasPrefix(entry, "PATH=") {
			filtered = append(filtered, entry)
		}
	}
	return append(filtered, "HOME="+home, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"), "HARNEZ_DISABLE_RATE_FEEDBACK=1")
}

func gitRunE2E(t *testing.T, root string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func gitShowE2E(t *testing.T, root, revision string) []byte {
	t.Helper()
	cmd := exec.Command("git", "show", revision)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git show %s: %v\n%s", revision, err, out)
	}
	return out
}
