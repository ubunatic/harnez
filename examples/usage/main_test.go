package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"ubunatic.com/harnez/usage"
)

func TestDemoOptionsUseIsolatedTempDirectories(t *testing.T) {
	opts, err := demoOptions()
	if err != nil {
		t.Fatalf("demoOptions: %v", err)
	}
	if opts.StateDir == usage.StateDir("") {
		t.Fatalf("demo state resolves to real usage state directory %q", opts.StateDir)
	}
	if filepath.Dir(opts.StateDir) != filepath.Dir(opts.RuntimeDir) {
		t.Fatalf("demo state/runtime dirs are not under one temp root: %q, %q", opts.StateDir, opts.RuntimeDir)
	}
	if !filepath.IsAbs(opts.StateDir) || !filepath.IsAbs(opts.RuntimeDir) {
		t.Fatalf("demo paths are not absolute: %+v", opts)
	}
	if _, err := os.Stat(opts.StateDir); err != nil {
		t.Fatalf("stat demo state directory: %v", err)
	}
}

func TestDemoSnapshotsAreExplicitlyLabelled(t *testing.T) {
	stateDir := t.TempDir()
	if err := seedDemoSnapshots(stateDir); err != nil {
		t.Fatalf("seedDemoSnapshots: %v", err)
	}
	for _, provider := range []usage.ProviderID{usage.ProviderClaude, usage.ProviderCodex} {
		snapshot, err := usage.ReadSnapshot(stateDir, provider)
		if err != nil {
			t.Fatalf("ReadSnapshot(%s): %v", provider, err)
		}
		if snapshot == nil || snapshot.Source != usage.SourceDemo || snapshot.Status != usage.StatusCached && snapshot.Status != usage.StatusStale {
			t.Errorf("snapshot source for %s = %+v, want demo", provider, snapshot)
		}
	}
	if _, err := os.Stat(filepath.Join(stateDir, "agy.json")); !os.IsNotExist(err) {
		t.Errorf("demo seeded an AGY snapshot despite it being uninstalled: %v", err)
	}
}

func TestSubscribeReturnsOnCancelledSignalContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := subscribe(ctx, emptyClient{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("subscribe error = %v, want context.Canceled", err)
	}
}

type emptyClient struct{}

func (emptyClient) ControllerInfo() usage.ControllerInfo { return usage.ControllerInfo{} }
func (emptyClient) Snapshot(context.Context, usage.ProviderID) (*usage.Snapshot, error) {
	return nil, nil
}
func (emptyClient) Subscribe(context.Context, usage.ProviderID) (<-chan usage.SnapshotEvent, error) {
	return make(chan usage.SnapshotEvent), nil
}
func (emptyClient) Refresh(context.Context, ...usage.ProviderID) ([]usage.Snapshot, error) {
	return nil, nil
}
func (emptyClient) Close() error { return nil }
