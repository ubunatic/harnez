package agentpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
)

func TestConfigurePreservesLocalSectionsAndResolvesLocalFirst(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, "AGENTS.local.md")
	if err := os.WriteFile(local, []byte("# notes\n\n<!-- harnez:begin Concise Mode -->\nkeep\n<!-- harnez:end Concise Mode -->\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Configure(dir, "native", false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Configure(dir, "native", false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "keep") || strings.Count(content, "Subagent Policy") != 2 {
		t.Fatalf("managed update did not preserve/add sections:\n%s", content)
	}
	if _, _, err := Configure(dir, "harnez", true); err != nil {
		t.Fatal(err)
	}
	state, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if state.Mode != "harnez" || state.Source != "./AGENTS.md" || state.Conflict {
		t.Fatalf("persist resolution = %#v", state)
	}
	data, err = os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Subagent Policy") || !strings.Contains(string(data), "keep") {
		t.Fatalf("persist should remove only conflicting local policy:\n%s", data)
	}
	if _, changed, err := Configure(dir, "harnez", true); err != nil || changed {
		t.Fatalf("persist should be idempotent: changed=%v err=%v", changed, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "info", "exclude")); !os.IsNotExist(err) {
		t.Fatalf("unexpected git exclude in non-git directory: %v", err)
	}
}

func TestConfigureEnsuresGitExclude(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "info"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Configure(dir, "native", false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	if err != nil || !strings.Contains(string(data), "AGENTS.local.md\n") {
		t.Fatalf("git exclude = %q (%v)", data, err)
	}
}

func TestPersistMigratesSameModeToMainAndPreservesLocalProse(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, "AGENTS.local.md")
	if err := os.WriteFile(local, []byte("local developer note\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Configure(dir, "harnez", false); err != nil {
		t.Fatal(err)
	}
	path, changed, err := Configure(dir, "harnez", true)
	if err != nil || !changed || path != filepath.Join(dir, "AGENTS.md") {
		t.Fatalf("same-mode persist: path=%q changed=%v err=%v", path, changed, err)
	}
	state, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if state.Mode != "harnez" || state.Source != "./AGENTS.md" || state.Conflict {
		t.Fatalf("effective persisted state = %#v", state)
	}
	data, err := os.ReadFile(local)
	if err != nil || string(data) != "local developer note\n" {
		t.Fatalf("local prose was not preserved: %q (%v)", data, err)
	}
	if _, changed, err := Configure(dir, "harnez", true); err != nil || changed {
		t.Fatalf("same-mode persist should be idempotent: changed=%v err=%v", changed, err)
	}
}

func TestRepoSessionsFiltersAndSortsDeterministically(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	sessions := []*subagent.Session{
		{ID: "outside", WorkingDir: filepath.Dir(dir), LastActiveAt: now.Add(time.Hour)},
		{ID: "b", WorkingDir: filepath.Join(dir, "child"), LastActiveAt: now},
		{ID: "a", WorkingDir: dir, LastActiveAt: now},
	}
	got := RepoSessions(dir, sessions)
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("filtered sessions = %#v", got)
	}
}
