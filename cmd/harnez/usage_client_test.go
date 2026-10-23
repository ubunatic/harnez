package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/usage"
)

func TestEnsureLocalServerStartsOnlyWithoutAListener(t *testing.T) {
	tests := []struct {
		name     string
		probe    serverProbeResult
		wantText string
		wantRuns int
	}{
		{name: "no server", wantText: "server starting", wantRuns: 1},
		{name: "starting", probe: serverProbeResult{listening: true, starting: true}, wantText: "server starting"},
		{name: "accept never answers", probe: serverProbeResult{listening: true}, wantText: "server not answering"},
		{name: "healthy", probe: serverProbeResult{ready: true, listening: true}},
		{name: "slow response after probe timeout", probe: serverProbeResult{listening: true}, wantText: "server not answering"},
		{name: "stale socket", wantText: "server starting", wantRuns: 1},
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
			var stdout, stderr bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			reportUsageServerStatus(cmd, check.status)
			if stdout.Len() != 0 {
				t.Fatalf("status leaked to stdout: %q", stdout.String())
			}
			wantStderr := ""
			if check.status != "" {
				wantStderr = fmt.Sprintf("harnez: %s\n", check.status)
			}
			if stderr.String() != wantStderr {
				t.Fatalf("stderr=%q, want %q", stderr.String(), wantStderr)
			}
		})
	}
}

func TestReadStoredSummaryBoundedUsesInjectedStoreUntilContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, err := readStoredSummaryBoundedWith(ctx, func(ctx context.Context) (usage.UsageSummary, error) {
			close(started)
			<-ctx.Done()
			return usage.UsageSummary{}, ctx.Err()
		})
		result <- err
	}()
	<-started
	cancel()
	select {
	case err := <-result:
		if err != context.Canceled {
			t.Fatalf("bounded store error = %v, want context canceled", err)
		}
	case <-time.After(time.Second * raceFactor):
		t.Fatal("bounded store reader did not return after context cancellation")
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
