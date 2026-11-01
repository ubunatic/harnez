package release

import "testing"

// issue 1074: a remote whose push URLs fan out to Codeberg and GitHub (a local
// `both`) must not be chosen as the forge in place of the single-URL remote.
func TestDetectForgeSkipsFanOutRemote(t *testing.T) {
	t.Run("single-url codeberg beats fan-out both", func(t *testing.T) {
		dir := t.TempDir()
		runGitCmd(t, dir, "init")
		runGitCmd(t, dir, "remote", "add", "both", "https://codeberg.org/o/r.git")
		runGitCmd(t, dir, "remote", "set-url", "--push", "both", "https://codeberg.org/o/r.git")
		runGitCmd(t, dir, "remote", "set-url", "--add", "--push", "both", "https://github.com/o/r.git")
		runGitCmd(t, dir, "remote", "add", "codeberg", "https://codeberg.org/o/r.git")
		info, err := DetectForgeInfo(dir)
		if err != nil {
			t.Fatal(err)
		}
		if info.RemoteName != "codeberg" {
			t.Errorf("forge remote = %s, want codeberg", info.RemoteName)
		}
	})
	t.Run("single-url github beats fan-out both", func(t *testing.T) {
		dir := t.TempDir()
		runGitCmd(t, dir, "init")
		runGitCmd(t, dir, "remote", "add", "both", "https://codeberg.org/o/r.git")
		runGitCmd(t, dir, "remote", "set-url", "--push", "both", "https://codeberg.org/o/r.git")
		runGitCmd(t, dir, "remote", "set-url", "--add", "--push", "both", "https://github.com/o/r.git")
		runGitCmd(t, dir, "remote", "add", "github", "https://github.com/o/r.git")
		info, err := DetectForgeInfo(dir)
		if err != nil {
			t.Fatal(err)
		}
		if info.RemoteName != "github" {
			t.Errorf("forge remote = %s, want github", info.RemoteName)
		}
	})
	t.Run("fan-out origin beats fan-out both", func(t *testing.T) {
		dir := t.TempDir()
		runGitCmd(t, dir, "init")
		runGitCmd(t, dir, "remote", "add", "origin", "https://codeberg.org/o/r.git")
		runGitCmd(t, dir, "remote", "set-url", "--push", "origin", "https://codeberg.org/o/r.git")
		runGitCmd(t, dir, "remote", "set-url", "--add", "--push", "origin", "https://github.com/o/r.git")
		runGitCmd(t, dir, "remote", "add", "both", "https://codeberg.org/o/r.git")
		runGitCmd(t, dir, "remote", "set-url", "--push", "both", "https://codeberg.org/o/r.git")
		runGitCmd(t, dir, "remote", "set-url", "--add", "--push", "both", "https://github.com/o/r.git")
		info, err := DetectForgeInfo(dir)
		if err != nil {
			t.Fatal(err)
		}
		if info.RemoteName != "origin" {
			t.Errorf("forge remote = %s, want origin", info.RemoteName)
		}
	})
	t.Run("fan-out alone is still usable", func(t *testing.T) {
		dir := t.TempDir()
		runGitCmd(t, dir, "init")
		runGitCmd(t, dir, "remote", "add", "both", "https://codeberg.org/o/r.git")
		runGitCmd(t, dir, "remote", "set-url", "--push", "both", "https://codeberg.org/o/r.git")
		runGitCmd(t, dir, "remote", "set-url", "--add", "--push", "both", "https://github.com/o/r.git")
		info, err := DetectForgeInfo(dir)
		if err != nil {
			t.Fatal(err)
		}
		if info.RemoteName != "both" {
			t.Errorf("forge remote = %s, want both", info.RemoteName)
		}
	})
}
