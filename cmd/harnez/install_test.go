package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestInstallCommand(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, tc := range []struct {
		args    []string
		wantErr string
	}{
		{args: []string{"neus", "--dry-run"}},
		{args: []string{"other"}, wantErr: "supported: neus"},
		{wantErr: "accepts 1 arg"},
	} {
		cmd := newInstallCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append([]string{}, tc.args...))
		err := cmd.Execute()
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v", err)
			}
		} else if err != nil || out.String() != "go install ubunatic.com/neus/cmd/neus@latest\nneus install\n" {
			t.Fatalf("error = %v, output = %q", err, out.String())
		}
	}
}
