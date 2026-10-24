package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/claude"
)

func TestInitWizardSelection(t *testing.T) {
	tests := []struct {
		name                                                     string
		force, disabled, yes, all, ci, inputTTY, outputTTY, want bool
	}{
		{name: "terminals", inputTTY: true, outputTTY: true, want: true},
		{name: "redirected input", outputTTY: true},
		{name: "redirected output", inputTTY: true},
		{name: "neither terminal"},
		{name: "CI", ci: true, inputTTY: true, outputTTY: true},
		{name: "yes", yes: true, inputTTY: true, outputTTY: true},
		{name: "no wizard", disabled: true, inputTTY: true, outputTTY: true},
		{name: "workspace", all: true, inputTTY: true, outputTTY: true},
		{name: "force", force: true, want: true},
		{name: "force in CI", force: true, ci: true, want: true},
		{name: "yes wins", force: true, yes: true},
		{name: "workspace wins", force: true, all: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := initWizardEnabled(tt.force, tt.disabled, tt.yes, tt.all, tt.ci, tt.inputTTY, tt.outputTTY); got != tt.want {
				t.Fatalf("enabled=%v, want %v", got, tt.want)
			}
		})
	}
	if initStreamTTY(strings.NewReader("")) || initStreamTTY(&bytes.Buffer{}) {
		t.Fatal("injected stream treated as terminal")
	}
}

func TestInitCmdWizardInjectedInput(t *testing.T) {
	t.Setenv("CI", "true") // --wizard deliberately overrides automatic CI suppression.
	dir := t.TempDir()
	cmd := newInitCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetIn(strings.NewReader("invalid\nyes\nyes\nno\nno\nno\nyes\n"))
	cmd.SetArgs([]string{"-d", dir, "--force", "--wizard", "--docs", "golang"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cfg, err := claude.LoadRepoConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Issues.Tracker != "harnez" || !cfg.Docs.Studies || cfg.Issues.MergeDriver || len(cfg.Docs.Copies) != 0 || cfg.Make.Scaffold || !cfg.Rules.Quota {
		t.Fatalf("choices not used by init: %+v", cfg)
	}
	if strings.Count(output.String(), "Issue tracker —") != 2 {
		t.Fatalf("invalid input not retried: %s", &output)
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "Go.md")); !os.IsNotExist(err) {
		t.Fatalf("disabled optional copy exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Makefile")); !os.IsNotExist(err) {
		t.Fatalf("disabled scaffold exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".harnez", "rules", "Quota.md")); err != nil {
		t.Fatalf("quota choice not applied: %v", err)
	}
}

type initForbiddenReader struct{ t *testing.T }

func (r initForbiddenReader) Read([]byte) (int, error) {
	r.t.Fatal("noninteractive init read stdin")
	return 0, nil
}

func TestInitCmdWizardBypassPreservesConfigBytes(t *testing.T) {
	for _, flags := range [][]string{nil, {"-y"}, {"--yes", "--wizard"}, {"--no-wizard"}} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			dir := t.TempDir()
			run := func(extra []string) string {
				t.Helper()
				cmd := newInitCmd()
				var output bytes.Buffer
				cmd.SetOut(&output)
				cmd.SetErr(&output)
				cmd.SetIn(initForbiddenReader{t})
				cmd.SetArgs(append([]string{"-d", dir, "--force"}, extra...))
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(output.String(), "Issue tracker —") {
					t.Fatal("wizard ran on bypass path")
				}
				data, err := os.ReadFile(filepath.Join(dir, ".harnez", "repo.yaml"))
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			}
			before := run([]string{"-y"})
			// Preserve comments and exact serialization, rather than just equivalent values.
			before = "# preserved configuration\n" + before
			if err := os.WriteFile(filepath.Join(dir, ".harnez", "repo.yaml"), []byte(before), 0o644); err != nil {
				t.Fatal(err)
			}
			if after := run(flags); after != before {
				t.Fatalf("config bytes changed:\n%s", after)
			}
		})
	}
}
