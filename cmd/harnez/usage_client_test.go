package main

import (
	"context"
	"os"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/server"
)

func TestEnsureLocalServerStartsOnlyWithoutAListener(t *testing.T) {
	tests := []struct {
		name     string
		probe    serverProbeResult
		wantText string
		wantRuns int
	}{
		{name: "no listener", wantText: "server starting", wantRuns: 1},
		{name: "starting", probe: serverProbeResult{listening: true, starting: true}, wantText: "server starting"},
		{name: "hung", probe: serverProbeResult{listening: true}, wantText: "server not answering"},
		{name: "healthy", probe: serverProbeResult{ready: true, listening: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probes, starts := 0, 0
			starter := serverStarter{
				probe: func(context.Context, server.Config) serverProbeResult { probes++; return tt.probe },
				start: func() error { starts++; return nil },
			}
			check := starter.ensure(context.Background(), server.Config{ProbeTimeout: time.Second})
			if check.status != tt.wantText {
				t.Fatalf("status=%q, want %q", check.status, tt.wantText)
			}
			if starts != tt.wantRuns || probes != 1 {
				t.Fatalf("starts=%d probes=%d, want starts=%d probes=1", starts, probes, tt.wantRuns)
			}
		})
	}
}

func TestLoadLocalUsageOfflineMissingStoreDoesNotCreateIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", home+"/data")
	if _, err := loadLocalUsage(context.Background(), true); err == nil {
		t.Fatal("missing offline store unexpectedly succeeded")
	}
	if _, err := os.Stat(home + "/data/harnez/telemetry.sqlite"); !os.IsNotExist(err) {
		t.Fatalf("reader created store: %v", err)
	}
}
