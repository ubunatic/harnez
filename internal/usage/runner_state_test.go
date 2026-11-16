package usage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestRunnerSnapshotDoesNotTouchCanonicalStore(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	shared := t.TempDir()
	t.Setenv("XDG_DATA_HOME", shared)
	t.Setenv("XDG_STATE_HOME", shared)
	t.Setenv("TAIA_SESSION_DIR", root)
	dir, err := ResolvedStateDir("")
	if err != nil {
		t.Fatal(err)
	}
	u := AgentUsage{AgentID: "test", Name: "runner"}
	if err := WriteAgentSnapshot(dir, "test", u); err != nil {
		t.Fatal(err)
	}
	got, err := ReadAgentSnapshot(dir, "test")
	if err != nil || got == nil || got.Usage.Name != u.Name {
		t.Fatalf("snapshot: %#v %v", got, err)
	}
	info, err := os.Stat(filepath.Join(dir, "test.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("snapshot mode: %v %v", info, err)
	}
	entries, err := os.ReadDir(shared)
	if err != nil || len(entries) != 0 {
		t.Fatalf("shared state changed: %v %v", entries, err)
	}
	if err := InitializeUsageStore(context.Background(), "", ""); err == nil {
		t.Fatal("runner initialized collector store")
	}
	if err := PersistUsageSummary(context.Background(), "", "", UsageSummary{}); err == nil {
		t.Fatal("runner persisted collector store")
	}
	if err := RunCollectorWithFetch(context.Background(), "", "", time.Second, os.Stderr, false, nil); err == nil {
		t.Fatal("runner collector started")
	}
	entries, err = os.ReadDir(shared)
	if err != nil || len(entries) != 0 {
		t.Fatalf("collector guards changed shared state: %v %v", entries, err)
	}
}

func TestRunnerLiveCacheDoesNotMigrateLegacy(t *testing.T) {
	home, shared, root := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", shared)
	t.Setenv("TAIA_SESSION_DIR", root)
	legacy := filepath.Join(home, ".claude")
	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, liveFetchCacheFilename), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	path := liveFetchCachePath(legacy)
	if path != filepath.Join(root, "state", "usage", "quota-cache-claude.json") {
		t.Fatalf("runner cache path: %s", path)
	}
	entries, err := os.ReadDir(shared)
	if err != nil || len(entries) != 0 {
		t.Fatalf("shared cache changed: %v %v", entries, err)
	}
}

func TestRunnerAGYBackoffIsIsolated(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	host := t.TempDir()
	t.Setenv("TAIA_SESSION_DIR", root)
	path := agyAuthBackoffPath(host)
	if want := filepath.Join(root, "state", "agymeter", agyAuthBackoffFilename); path != want {
		t.Fatalf("backoff path = %q, want %q", path, want)
	}
	if err := writeAGYAuthBackoff(path, agyAuthBackoff{Until: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if readAGYAuthBackoff(path) == nil {
		t.Fatal("missing runner backoff")
	}
	if entries, err := os.ReadDir(host); err != nil || len(entries) != 0 {
		t.Fatalf("host changed: %v %v", entries, err)
	}
}

func TestRunnerAuxiliaryState(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	shared := t.TempDir()
	t.Setenv("XDG_STATE_HOME", shared)
	t.Setenv("TAIA_SESSION_DIR", root)
	t.Setenv(passiveNoQuotaCaptureEnv, "1")
	recordFetchDuration("", "local", time.Second)
	capturePassiveNoQuota("test", []byte("{}"), "missing")
	dir, err := ResolvedStateDir("")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{fetchDurationsFilename, "passive-last-no-quota.json"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Errorf("%s: %v %v", name, info, err)
		}
	}
	entries, err := os.ReadDir(shared)
	if err != nil || len(entries) != 0 {
		t.Fatalf("shared state changed: %v %v", entries, err)
	}
}

func TestRunnerKillLog(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	shared := t.TempDir()
	t.Setenv("XDG_STATE_HOME", shared)
	t.Setenv("TAIA_SESSION_DIR", root)
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	registry.recordKillFailure(Collector{ID: "test", Provider: "test"}, time.Second, errors.New("signal: killed"), nil)
	registry.waitKillDiagnostics()
	settings, _, err := usageKillLogSettings()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := ResolvedStateDir("")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, filepath.Base(settings.Path)))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("kill log: %v %v", info, err)
	}
	entries, err := os.ReadDir(shared)
	if err != nil || len(entries) != 0 {
		t.Fatalf("shared state changed: %v %v", entries, err)
	}
}

func TestRunnerProcessesWriteSeparately(t *testing.T) {
	if os.Getenv("M8_CHILD") == "1" {
		dir, err := ResolvedStateDir("")
		if err != nil {
			t.Fatal(err)
		}
		if err := WriteAgentSnapshot(dir, "test", AgentUsage{AgentID: "test", Name: os.Getenv("M8_NAME")}); err != nil {
			t.Fatal(err)
		}
		return
	}
	roots := []string{t.TempDir(), t.TempDir()}
	cmds := make([]*exec.Cmd, 0, len(roots))
	for i, root := range roots {
		if err := os.Chmod(root, 0700); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(os.Args[0], "-test.run=^TestRunnerProcessesWriteSeparately$")
		cmd.Env = append(os.Environ(), "M8_CHILD=1", "M8_NAME="+strconv.Itoa(i), "TAIA_SESSION_DIR="+root)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, cmd)
	}
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	for i, root := range roots {
		data, err := os.ReadFile(filepath.Join(root, "state", "usage", "test.json"))
		if err != nil {
			t.Fatal(err)
		}
		var snap AgentSnapshot
		if err := json.Unmarshal(data, &snap); err != nil || snap.Usage.Name != strconv.Itoa(i) {
			t.Fatalf("snapshot %d: %v %v", i, snap, err)
		}
	}
}
