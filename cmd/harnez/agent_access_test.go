package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/agent"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/modelcatalog"
)

func TestCodexObservedModelAccess(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	model := subagent.Model{Provider: "codex", Name: "gpt-6-luna", Tier: "low"}
	cases := []struct{ name, message, want string }{
		{"study", "The 'no-such-model-x' model is not supported when using Codex with a ChatGPT account.", "unsupported"},
		{"code", "HTTP 400 model_not_found", "unsupported"},
		{"other", "HTTP 400 invalid request", "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := filepath.Join(root, tc.name)
			t.Setenv("XDG_STATE_HOME", state)
			script := "#!/bin/sh\nprintf '%s\\n' " + fmt.Sprintf("'%s'", strings.ReplaceAll(fmt.Sprintf(`{"type":"turn.failed","error":{"message":%q}}`, tc.message), "'", "'\\''")) + "\nexit 1\n"
			if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			_, err := (subagent.CodexDriver{}).Run(context.Background(), subagent.RunOptions{Model: model, Prompt: "test"})
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("provider error: %v", err)
			}
			recordCodexAccess(&bytes.Buffer{}, "codex", model.Name, err)
			got, _, err := modelcatalog.LatestModelAccess("codex", model.Name)
			if err != nil || got != tc.want {
				t.Fatalf("access = %q, %v; want %q", got, err, tc.want)
			}
			if tc.want == "unsupported" {
				row := agent.ModelRows([]subagent.ModelEntry{{Model: model}}, map[string]usage.ProviderQuotaAvailability{}, nil, time.Now())[0]
				if row.Startable || row.StartableReason != "unsupported for this account" {
					t.Fatalf("row: %+v", row)
				}
				recordCodexAccess(&bytes.Buffer{}, "codex", model.Name, nil)
				got, _, err = modelcatalog.LatestModelAccess("codex", model.Name)
				if err != nil || got != "accessible" {
					t.Fatalf("success access = %q, %v", got, err)
				}
			}
		})
	}
}
