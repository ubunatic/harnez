package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/dev/shellsetup"
)

// withCompletionRenderer installs r for the test and restores the previous renderer.
func withCompletionRenderer(t *testing.T, r CompletionRenderer) {
	t.Helper()
	prev := completionRenderer
	t.Cleanup(func() { SetCompletionRenderer(prev) })
	SetCompletionRenderer(r)
}

func TestShellSetupHasNoCompletionWithoutRenderer(t *testing.T) {
	home := t.TempDir()
	withCompletionRenderer(t, nil)
	if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	checks := shellSetupChecks(home)
	for _, c := range checks {
		if strings.Contains(c.Label, "[completion") {
			t.Errorf("completion row without a renderer: %+v", c)
		}
	}
}

func TestShellSetupWiresCompletionWhenRendererSet(t *testing.T) {
	home := t.TempDir()
	withCompletionRenderer(t, func(shell string) ([]byte, error) {
		return []byte("# completion for " + shell + "\n"), nil
	})
	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	inst, err := shellSetup(home)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inst.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if !hasChange(res, shellsetup.FeatureCompletion, filepath.Join(home, ".harnez", "completions", "harnez.zsh")) {
		t.Errorf("zsh completion script not written: %+v", res.Changes)
	}
}

func hasChange(res shellsetup.Result, feature shellsetup.Feature, path string) bool {
	for _, c := range res.Changes {
		if c.Feature == feature && c.Path == path {
			return true
		}
	}
	return false
}
