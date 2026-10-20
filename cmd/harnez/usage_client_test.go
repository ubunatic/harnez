package main

import (
	"context"
	"os"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/server"
)

func TestEnsureLocalServerStartsOnceAndWaitsForReady(t *testing.T) {
	probes, starts, waits := 0, 0, 0
	starter := serverStarter{
		probe: func(context.Context, server.Config) bool { probes++; return probes >= 3 },
		start: func() error { starts++; return nil },
		wait:  func(context.Context, time.Duration) error { waits++; return nil },
	}
	cfg := server.Config{StartupTimeout: time.Second, StartupPoll: time.Millisecond}
	if err := starter.ensure(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if starts != 1 || probes != 3 || waits != 2 {
		t.Fatalf("starts=%d probes=%d waits=%d", starts, probes, waits)
	}
}

func TestEnsureLocalServerChecksAndKeepsHealthyServer(t *testing.T) {
	starts := 0
	starter := serverStarter{probe: func(context.Context, server.Config) bool { return true }, start: func() error { starts++; return nil }}
	if err := starter.ensure(context.Background(), server.Config{}); err != nil {
		t.Fatal(err)
	}
	if starts != 1 {
		t.Fatalf("start called %d times, want one version check", starts)
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
