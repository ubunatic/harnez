package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestReleaseRuntimeErrorSuppressesUsage(t *testing.T) {
	root := &cobra.Command{Use: "harnez"}
	root.AddCommand(newReleaseCmd())
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"release", "--dir", t.TempDir(), "--skip-sign", "--skip-build", "--skip-push", "--skip-publish"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected git status error outside a repository")
	}
	if strings.Contains(output.String(), "Usage:") {
		t.Fatalf("runtime error printed usage: %s", output.String())
	}
}
