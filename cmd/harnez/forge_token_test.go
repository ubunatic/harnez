package main

import (
	"bytes"
	"strings"
	"testing"
)

// rotate and revoke refuse before any prompt when nothing is stored; status never prompts
// (issue 1118). No terminal is needed for any of these.
func TestForgeTokenWiringWithoutStoredToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, args := range [][]string{{"rotate"}, {"revoke"}, {"revoke", "--local-only"}} {
		cmd := newForgeTokenCmd()
		cmd.SetArgs(args)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "no token for codeberg.org is stored") {
			t.Fatalf("%v: err = %v", args, err)
		}
	}
	var out bytes.Buffer
	cmd := newForgeTokenCmd()
	cmd.SetArgs([]string{"status"})
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil || out.String() != "no token stored for codeberg.org\n" {
		t.Fatalf("status: %v %q", err, out.String())
	}
	cmd = newForgeTokenCmd()
	cmd.SetArgs([]string{"create", "--host", "github.com"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "not a Forgejo host") {
		t.Fatalf("github host: %v", err)
	}
}
