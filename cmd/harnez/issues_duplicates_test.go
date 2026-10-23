package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func duplicateRepo(t *testing.T, count int) string {
	t.Helper()
	dir := repoInit(t)
	for _, sub := range []string{"issues", "docs"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"a", "b", "c"}[:count] {
		writeDuplicateFile(t, dir, "issues/096-"+name+".md", "# 096 — "+name+"\n\n**Status**: Open\n")
	}
	writeDuplicateFile(t, dir, "issues/README.md", "# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n")
	writeDuplicateFile(t, dir, ".harnez/repo.yaml", `version: 1
issues:
  tracker: harnez
  merge_driver: true
docs:
  studies: false
  copies: []
rules:
  quota: false
make:
  container_test: false
`)
	repoRunGit(t, dir, "add", ".")
	repoRunGit(t, dir, "commit", "-qm", "fixture")
	return dir
}

func writeDuplicateFile(t *testing.T, dir, path, content string) {
	t.Helper()
	fullPath := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicateNumbersRejectIndexAndStatus(t *testing.T) {
	for _, count := range []int{2, 3} {
		for _, mode := range []string{"index", "check", "status"} {
			t.Run(mode+string(rune('0'+count)), func(t *testing.T) {
				dir := duplicateRepo(t, count)
				before, _ := os.ReadFile(filepath.Join(dir, "issues/README.md"))
				var out bytes.Buffer
				var err error
				if mode == "status" {
					t.Chdir(dir)
					cmd := newStatusCmd()
					cmd.SetArgs([]string{"--target", t.TempDir()})
					cmd.SetOut(&out)
					cmd.SetErr(&out)
					err = cmd.Execute()
				} else {
					err = runIndex(&out, indexOptions{Dir: dir, Check: mode == "check", Stderr: &out})
				}
				if err == nil {
					t.Errorf("%s accepted duplicate numbers", mode)
				} else {
					for _, want := range append([]string{"harnez issues mv -d . <file>"}, []string{"issues/096-a.md", "issues/096-b.md", "issues/096-c.md"}[:count]...) {
						if !strings.Contains(err.Error(), want) {
							t.Errorf("error %q missing %q", err, want)
						}
					}
				}
				after, _ := os.ReadFile(filepath.Join(dir, "issues/README.md"))
				if !bytes.Equal(before, after) {
					t.Error("README changed on duplicate error")
				}
			})
		}
	}
}

func TestDuplicateMoveRewritesReferencesInCommit(t *testing.T) {
	dir := duplicateRepo(t, 2)
	refs := "issues/096-b.md `096-b.md` [b](../issues/096-b.md)\n096 #096 issues/096-a.md 1096-b.md 096-b.md.bak\n"
	for _, path := range []string{"issues/050-c.md", "docs/References.md"} {
		writeDuplicateFile(t, dir, path, "# 050 — References\n\n**Status**: Open\n\n"+refs)
	}
	repoRunGit(t, dir, "add", ".")
	repoRunGit(t, dir, "commit", "-qm", "references")
	var out bytes.Buffer
	result, _, err := runIssuesMv(&out, "issues/096-b.md", "", issuesRunOptions{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if result.Number != "097" || !result.Committed {
		t.Fatalf("unexpected result: %+v", result)
	}
	for _, path := range []string{"issues/050-c.md", "docs/References.md"} {
		got := repoRunGitOutput(t, dir, "show", "HEAD:"+path)
		want := "issues/097-b.md `097-b.md` [b](../issues/097-b.md)\n096 #096 issues/096-a.md 1096-b.md 096-b.md.bak"
		if !strings.Contains(got, want) {
			t.Errorf("%s committed references: %s", path, got)
		}
	}
	if got := repoRunGitOutput(t, dir, "show", "HEAD:issues/097-b.md"); !strings.Contains(got, "# 097 — b") {
		t.Error(got)
	}
}

func TestDuplicateMoveMissingHeaderExplainsRepair(t *testing.T) {
	dir := duplicateRepo(t, 2)
	writeDuplicateFile(t, dir, "issues/096-b.md", "# Missing number\n\n**Status**: Open\n")
	var out bytes.Buffer
	_, _, err := runIssuesMv(&out, "issues/096-b.md", "", issuesRunOptions{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "issues/096-b.md") || !strings.Contains(err.Error(), "add") || !strings.Contains(err.Error(), "# 096 — ") {
		t.Fatalf("missing actionable repair: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "issues/097-b.md")); !os.IsNotExist(err) {
		t.Fatal("move wrote destination before validation")
	}
}

func TestThreeDuplicateMoves(t *testing.T) {
	dir := duplicateRepo(t, 3)
	for _, name := range []string{"b", "c"} {
		var out bytes.Buffer
		if _, _, err := runIssuesMv(&out, "issues/096-"+name+".md", "", issuesRunOptions{Dir: dir}); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := runIndex(&out, indexOptions{Dir: dir, Check: true, Stderr: &out}); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicateMoveCheckAndRollback(t *testing.T) {
	for _, mode := range []string{"check", "rollback"} {
		t.Run(mode, func(t *testing.T) {
			dir := duplicateRepo(t, 2)
			writeDuplicateFile(t, dir, "docs/References.md", "issues/096-b.md")
			if mode == "rollback" {
				writeDuplicateFile(t, dir, "issues/README.md", "# Issues\n\n| # | File | Title | Status | Extra |\n|---|---|---|---|---|\n")
			}
			before := repoRunGitOutput(t, dir, "diff")
			var out bytes.Buffer
			_, drift, err := runIssuesMv(&out, "issues/096-b.md", "", issuesRunOptions{Dir: dir, Check: mode == "check"})
			if mode == "check" && (err != nil || !drift) {
				t.Fatalf("check: drift=%v err=%v", drift, err)
			}
			if mode == "rollback" && err == nil {
				t.Fatal("expected customized README error")
			}
			if after := repoRunGitOutput(t, dir, "diff"); after != before {
				t.Fatal("check/rollback changed tracked files")
			}
			refs, _ := os.ReadFile(filepath.Join(dir, "docs/References.md"))
			if string(refs) != "issues/096-b.md" {
				t.Fatal("check/rollback changed references")
			}
			if _, err := os.Stat(filepath.Join(dir, "issues/097-b.md")); !os.IsNotExist(err) {
				t.Fatal("left moved ticket")
			}
		})
	}
}
