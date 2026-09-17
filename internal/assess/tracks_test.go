package assess

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyTrack(t *testing.T) {
	tests := []struct {
		path      string
		wantTrack TrackType
		wantOK    bool
	}{
		{"cmd/harnez/main.go", TrackCode, true},
		{"internal/assess/tracks.go", TrackCode, true},
		{"internal/assess/tracks_test.go", TrackTests, true},
		{"tests/integration_test.py", TrackTests, true},
		{"test/suite.go", TrackTests, true},
		{"docs/lang/Go.md", TrackDocs, true},
		{"README.md", TrackDocs, true},
		{"CLAUDE.md", TrackDocs, true},
		{"AGENTS.md", TrackDocs, true},
		{"skills/git/SKILL.md", TrackSkills, true},
		{"docs/commands/sprint.md", TrackSkills, true},
		{"commands/test.md", TrackSkills, true},
		{"issues/376-multi-track.md", TrackIssues, true},
		{"issues/README.md", TrackDocs, true}, // issues/README.md is doc index
		{".git/config", "", false},
		{"node_modules/pkg/index.js", "", false},
		{"dist/bundle.js", "", false},
	}

	for _, tt := range tests {
		gotTrack, gotOK := ClassifyTrack(tt.path)
		if gotOK != tt.wantOK || gotTrack != tt.wantTrack {
			t.Errorf("ClassifyTrack(%q) = (%v, %v); want (%v, %v)", tt.path, gotTrack, gotOK, tt.wantTrack, tt.wantOK)
		}
	}
}

func TestExtractMultiTrackHistoryTempRepo(t *testing.T) {
	tmpDir := t.TempDir()

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, string(out))
		}
	}

	runGit("init")
	runGit("config", "user.name", "Test")
	runGit("config", "user.email", "test@example.com")

	// Commit 1: Add Code, Tests, Docs, Skills, Issues
	_ = os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "docs"), 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "skills", "audit"), 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "issues"), 0755)

	_ = os.WriteFile(filepath.Join(tmpDir, "pkg", "lib.go"), []byte("package pkg\nfunc Hello() string { return \"hello\" }\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "pkg", "lib_test.go"), []byte("package pkg\nimport \"testing\"\nfunc TestHello(t *testing.T){}\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "docs", "Arch.md"), []byte("# Architecture\nOverview doc here.\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "skills", "audit", "SKILL.md"), []byte("# Skill Audit\nInstructions.\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "issues", "001-init.md"), []byte("# 001 — Init\n**Status**: Closed\nDone.\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "issues", "002-feature.md"), []byte("# 002 — Feature\n**Status**: In Progress\nWIP.\n"), 0644)

	runGit("add", ".")
	runGit("commit", "-m", "initial multi-track commit")

	// Commit 2: Expand code
	_ = os.WriteFile(filepath.Join(tmpDir, "pkg", "lib.go"), []byte("package pkg\nfunc Hello() string { return \"hello\" }\nfunc World() string { return \"world\" }\n"), 0644)
	runGit("add", ".")
	runGit("commit", "-m", "add world")

	// Commit 3: Remove an issue ticket and reduce code LOC
	runGit("rm", "issues/002-feature.md")
	_ = os.WriteFile(filepath.Join(tmpDir, "pkg", "lib.go"), []byte("package pkg\nfunc Hello() string { return \"hello\" }\n"), 0644)
	runGit("add", "-A")
	runGit("commit", "-m", "prune issue and trim code")

	res, err := ExtractMultiTrackHistory(tmpDir)
	if err != nil {
		t.Fatalf("ExtractMultiTrackHistory failed: %v", err)
	}

	if res.TotalCommits != 3 {
		t.Errorf("TotalCommits = %d; want 3", res.TotalCommits)
	}

	codeTrack := res.Tracks[TrackCode]
	if codeTrack == nil || codeTrack.CurrentFiles != 1 {
		t.Errorf("codeTrack unexpected: %+v", codeTrack)
	}
	if codeTrack.TotalAdded <= 0 {
		t.Errorf("codeTrack.TotalAdded = %d; want > 0", codeTrack.TotalAdded)
	}
	if codeTrack.TotalRemoved <= 0 {
		t.Errorf("codeTrack.TotalRemoved = %d; want > 0", codeTrack.TotalRemoved)
	}
	if len(codeTrack.AddedPoints) != 3 || len(codeTrack.RemovedPoints) != 3 {
		t.Errorf("codeTrack points length unexpected: added=%d, removed=%d", len(codeTrack.AddedPoints), len(codeTrack.RemovedPoints))
	}
	if codeTrack.AddSparkline == "" || codeTrack.RemoveSparkline == "" {
		t.Errorf("codeTrack missing add/remove sparklines")
	}

	testTrack := res.Tracks[TrackTests]
	if testTrack == nil || testTrack.CurrentFiles != 1 {
		t.Errorf("testTrack unexpected: %+v", testTrack)
	}

	docsTrack := res.Tracks[TrackDocs]
	if docsTrack == nil || docsTrack.CurrentFiles != 1 {
		t.Errorf("docsTrack unexpected: %+v", docsTrack)
	}

	skillsTrack := res.Tracks[TrackSkills]
	if skillsTrack == nil || skillsTrack.CurrentFiles != 1 {
		t.Errorf("skillsTrack unexpected: %+v", skillsTrack)
	}

	issuesTrack := res.Tracks[TrackIssues]
	if issuesTrack == nil || issuesTrack.CurrentFiles != 1 || issuesTrack.TotalRemoved != 1 {
		t.Errorf("issuesTrack unexpected: %+v", issuesTrack)
	}

	card := RenderMultiTrackCard(res)
	if !strings.Contains(card, "Repo Evolution") {
		t.Errorf("card missing header: %s", card)
	}
	if !strings.Contains(card, "Code:") || !strings.Contains(card, "Tests:") || !strings.Contains(card, "Issues:") {
		t.Errorf("card missing tracks: %s", card)
	}

	// Test diff rendering without color
	diffTable := RenderMultiTrackHistoryTableWithDiffs(res, RenderTracksOptions{Color: false})
	if !strings.Contains(diffTable, "Repo Evolution Diffs") {
		t.Errorf("diffTable missing header: %s", diffTable)
	}
	if !strings.Contains(diffTable, "[+] Adds:") || !strings.Contains(diffTable, "[-] Rms:") || !strings.Contains(diffTable, "[=] Net:") {
		t.Errorf("diffTable missing diff sections: %s", diffTable)
	}

	// Test diff rendering with color
	diffTableColor := RenderMultiTrackHistoryTableWithDiffs(res, RenderTracksOptions{Color: true})
	if !strings.Contains(diffTableColor, "\x1b[32m[+]\x1b[0m") || !strings.Contains(diffTableColor, "\x1b[31m[-]\x1b[0m") {
		t.Errorf("diffTableColor missing ANSI color tags: %s", diffTableColor)
	}

	// Test RenderMultiTrackCard with Diff: true
	cardDiff := RenderMultiTrackCard(res, RenderMultiTrackCardOptions{Diff: true})
	if !strings.Contains(cardDiff, "Repo Evolution Diffs") {
		t.Errorf("cardDiff missing header: %s", cardDiff)
	}
}
