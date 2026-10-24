package main

import (
	"io"
	"strings"
	"testing"
)

func TestServerNeusIndexRequiresRegisteredRoots(t *testing.T) {
	cmd := newServerCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--neus-index"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--neus-repo") {
		t.Fatalf("error = %v", err)
	}
	if flag := newServerCmd().Flags().Lookup("neus-index"); flag == nil || flag.DefValue != "false" {
		t.Fatalf("opt-in flag = %v", flag)
	}
}
