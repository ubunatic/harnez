package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func TestWarnSmallPiWindow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	models := `{"providers": {
  "lmcoder": {"models": [{"id": "coder", "contextWindow": 32768}, {"id": "default", "contextWindow": 65536}]},
  "a": {"models": [{"id": "dup", "contextWindow": 32768}, {"id": "solo", "contextWindow": 32768}]},
  "b": {"models": [{"id": "dup", "contextWindow": 32768}]}}}`
	if err := os.WriteFile(filepath.Join(dir, "models.json"), []byte(models), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := piAgentDir(); got != dir {
		t.Fatalf("piAgentDir() = %q; want PI_CODING_AGENT_DIR %q", got, dir)
	}
	for _, tc := range []struct {
		model subagent.Model
		want  string
	}{
		{subagent.Model{Provider: "lmcoder", Name: "coder"}, "lmcoder/coder has a 32768-token"},
		{subagent.Model{Provider: "pi", Name: "solo"}, "a/solo has a 32768-token"},
		{subagent.Model{Provider: "pi", Name: "dup"}, ""},
		{subagent.Model{Provider: "lmcoder", Name: "default"}, ""},
		{subagent.Model{Provider: "claude", Name: "coder"}, ""},
	} {
		var out bytes.Buffer
		warnSmallPiWindow(&out, tc.model, dir)
		if tc.want == "" {
			if out.Len() != 0 {
				t.Errorf("%s: unexpected warning %q", tc.model.Spec(), out.String())
			}
			continue
		}
		if strings.Count(out.String(), "\n") != 1 || !strings.Contains(out.String(), tc.want) || !strings.Contains(out.String(), "harnez apply") {
			t.Errorf("%s: warning = %q; want one line with %q and 'harnez apply'", tc.model.Spec(), out.String(), tc.want)
		}
	}
	if err := applyPiCompaction(dir, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	warnSmallPiWindow(&out, subagent.Model{Provider: "lmcoder", Name: "coder"}, dir)
	if out.Len() != 0 {
		t.Fatalf("warning after apply wrote the override: %q", out.String())
	}
}
