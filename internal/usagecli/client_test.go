package usagecli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/usage"
)

func TestEnsureLocalServerStartsOnlyWithoutAListener(t *testing.T) {
	tests := []struct {
		name     string
		probe    ServerProbeResult
		wantText string
		wantRuns int
	}{
		{name: "no server", wantText: "server starting", wantRuns: 1},
		{name: "starting", probe: ServerProbeResult{listening: true, starting: true}, wantText: "server starting"},
		{name: "accept never answers", probe: ServerProbeResult{listening: true}, wantText: "server not answering"},
		{name: "healthy", probe: ServerProbeResult{ready: true, listening: true}},
		{name: "outdated server", probe: ServerProbeResult{ready: true, listening: true, outdated: true, serverVersion: "0.5.7+5148"}, wantText: fmt.Sprintf("server updating (0.5.7+5148 -> %s)", harnez.BuildVersion()), wantRuns: 1},
		{name: "slow response after probe timeout", probe: ServerProbeResult{listening: true}, wantText: "server not answering"},
		{name: "stale socket", wantText: "server starting", wantRuns: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probes, starts := 0, 0
			starter := ServerStarter{
				probe: func(context.Context, server.Config) ServerProbeResult { probes++; return tt.probe },
				start: func() error { starts++; return nil },
			}
			check := starter.Ensure(context.Background(), server.Config{ProbeTimeout: time.Second})
			if check.status != tt.wantText {
				t.Fatalf("status=%q, want %q", check.status, tt.wantText)
			}
			if starts != tt.wantRuns || probes != 1 {
				t.Fatalf("starts=%d probes=%d, want starts=%d probes=1", starts, probes, tt.wantRuns)
			}
			if got, want := ServerStatusMessage(check.status), func() string {
				if check.status == "" {
					return ""
				}
				return fmt.Sprintf("harnez: %s\n", check.status)
			}(); got != want {
				t.Fatalf("status message=%q, want %q", got, want)
			}
		})
	}
}

func TestReadStoredSummaryBoundedUsesInjectedStoreUntilContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, err := ReadStoredSummaryBoundedWith(ctx, func(ctx context.Context) (usage.UsageSummary, error) {
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
	if _, err := LoadLocalUsage(context.Background(), true); err == nil {
		t.Fatal("missing offline store unexpectedly succeeded")
	}
	if _, err := os.Stat(home + "/data/harnez/telemetry.sqlite"); !os.IsNotExist(err) {
		t.Fatalf("reader created store: %v", err)
	}
}

func TestEnsureLocalServerOutdatedStartErrorReportsVersionMismatch(t *testing.T) {
	starter := ServerStarter{
		probe: func(context.Context, server.Config) ServerProbeResult {
			return ServerProbeResult{ready: true, listening: true, outdated: true, serverVersion: "0.5.7+5148"}
		},
		start: func() error { return errors.New("boom") },
	}
	check := starter.Ensure(context.Background(), server.Config{ProbeTimeout: time.Second})
	if !strings.Contains(check.status, "server outdated (0.5.7+5148 vs ") || !strings.Contains(check.status, "could not restart: boom") {
		t.Fatalf("status = %q", check.status)
	}
}

func TestProbeServerHTTPDetectsOutdatedServer(t *testing.T) {
	tests := []struct {
		name        string
		srvVersion  string
		srvIdentity string
		cliVersion  string
		cliIdentity string
		wantOutdate bool
	}{
		{name: "older semver", srvVersion: "0.5.7+5148", srvIdentity: "2026-11-12T17:35:00Z", cliVersion: "0.5.8+5234", cliIdentity: "2026-11-16T08:35:00Z", wantOutdate: true},
		{name: "same semver older binary", srvVersion: "0.5.8+5200", srvIdentity: "2026-11-12T17:35:00Z", cliVersion: "0.5.8+5234", cliIdentity: "2026-11-16T08:35:00Z", wantOutdate: true},
		{name: "equal build", srvVersion: "0.5.8+5234", srvIdentity: "2026-11-16T08:35:00Z", cliVersion: "0.5.8+5234", cliIdentity: "2026-11-16T08:35:00Z", wantOutdate: false},
		{name: "newer server", srvVersion: "0.6.0+6000", srvIdentity: "2026-11-17T08:35:00Z", cliVersion: "0.5.8+5234", cliIdentity: "2026-11-16T08:35:00Z", wantOutdate: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"health":"ok","server":"harnez","version":%q,"build_identity":%q}`, tt.srvVersion, tt.srvIdentity)
			}))
			defer srv.Close()
			got := probeServerHTTP(context.Background(), srv.Client(), srv.URL, tt.cliVersion, tt.cliIdentity)
			if !got.ready || !got.listening || got.outdated != tt.wantOutdate || got.serverVersion != tt.srvVersion {
				t.Fatalf("got = %+v, want ready=true outdated=%v version=%q", got, tt.wantOutdate, tt.srvVersion)
			}
		})
	}
}
