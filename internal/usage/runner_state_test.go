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

func TestRunnerPeakAndRemoteWritersFailClosed(t *testing.T) {
	shared := t.TempDir()
	t.Setenv("XDG_STATE_HOME", shared)
	t.Setenv("TAIA_SESSION_DIR", "relative-runner")
	if _, err := powerPeaksPath(); err == nil {
		t.Fatal("invalid peak path accepted")
	}
	if err := WriteCachedRemoteLoad("host", &LoadSnapshot{}); err == nil {
		t.Fatal("invalid remote cache path accepted")
	}
	if entries, err := os.ReadDir(shared); err != nil || len(entries) != 0 {
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
		if _, err := os.Stdout.Write([]byte{1}); err != nil {
			t.Fatal(err)
		}
		var signal [1]byte
		if _, err := os.Stdin.Read(signal[:]); err != nil {
			t.Fatal(err)
		}
		dir, err := ResolvedStateDir("")
		if err != nil {
			t.Fatal(err)
		}
		if err := WriteAgentSnapshot(dir, "test", AgentUsage{AgentID: "test", Name: os.Getenv("M8_NAME")}); err != nil {
			t.Fatal(err)
		}
		if err := WriteCachedRemoteLoad("host", &LoadSnapshot{CPU: CPULoad{NumCPU: 42}}); err != nil {
			t.Fatal(err)
		}
		path, err := powerPeaksPath()
		if err != nil {
			t.Fatal(err)
		}
		peaks := newPowerPeakStore(path, "boot", time.Now)
		peaks.Observe("cpu", 42)
		if err := peaks.Flush(); err != nil {
			t.Fatal(err)
		}
		return
	}
	roots := []string{t.TempDir(), t.TempDir()}
	cmds := make([]*exec.Cmd, 0, len(roots))
	pipes := make([]*os.File, 0, len(roots))
	readyPipes := make([]*os.File, 0, len(roots))
	for i, root := range roots {
		if err := os.Chmod(root, 0700); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(os.Args[0], "-test.run=^TestRunnerProcessesWriteSeparately$")
		cmd.Env = append(os.Environ(), "M8_CHILD=1", "M8_NAME="+strconv.Itoa(i), "TAIA_SESSION_DIR="+root)
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		readyReader, readyWriter, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stdin = reader
		cmd.Stdout = readyWriter
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		reader.Close()
		readyWriter.Close()
		pipes = append(pipes, writer)
		readyPipes = append(readyPipes, readyReader)
		cmds = append(cmds, cmd)
	}
	for _, pipe := range readyPipes {
		var ready [1]byte
		if _, err := pipe.Read(ready[:]); err != nil {
			t.Fatal(err)
		}
	}
	for _, pipe := range pipes {
		_, _ = pipe.Write([]byte{1})
		_ = pipe.Close()
	}
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	for _, pipe := range readyPipes {
		_ = pipe.Close()
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
		if f := loadPowerPeakFileForTest(t, filepath.Join(root, "state", "usage", powerPeaksFilename)); f.Peaks["cpu"] != 42 {
			t.Fatalf("peak: %+v", f)
		}
		t.Setenv("TAIA_SESSION_DIR", root)
		if got := ReadCachedRemoteLoad("host"); got == nil || got.CPU.NumCPU != 42 {
			t.Fatalf("remote cache: %+v", got)
		}
	}
}

func TestRunnerInvalidRootsThroughWriters(t *testing.T) {
	for _, kind := range []string{"relative", "missing", "permissive", "symlink", "descendant"} {
		t.Run(kind, func(t *testing.T) {
			home, shared, root := t.TempDir(), t.TempDir(), t.TempDir()
			switch kind {
			case "relative":
				root = "invalid-runner"
			case "missing":
				root = filepath.Join(root, "missing")
			case "permissive":
				if err := os.Chmod(root, 0755); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				link := filepath.Join(t.TempDir(), "link")
				if err := os.Symlink(root, link); err != nil {
					t.Fatal(err)
				}
				root = link
			case "descendant":
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "state")); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("HOME", home)
			t.Setenv("XDG_STATE_HOME", shared)
			t.Setenv("XDG_CACHE_HOME", shared)
			t.Setenv("XDG_DATA_HOME", shared)
			t.Setenv("TAIA_SESSION_DIR", root)
			t.Setenv(passiveNoQuotaCaptureEnv, "1")
			recordFetchDuration(home, "local", time.Second)
			capturePassiveNoQuota("test", []byte("{}"), "test")
			summary := CollectRegisteredCatchup(context.Background(), home, nil, time.Hour, nil, nil)
			if len(summary.Agents) != 0 {
				t.Fatalf("unexpected agents: %v", summary.Agents)
			}
			if err := writeAGYAuthBackoff(agyAuthBackoffPath(home), agyAuthBackoff{Until: time.Now()}); err == nil {
				t.Fatal("backoff write succeeded")
			}
			for _, dir := range []string{home, shared} {
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != 0 {
					t.Fatalf("shared directory changed: %v %v", entries, err)
				}
			}
		})
	}
}

func TestRunnerExplicitStateDirOverride(t *testing.T) {
	t.Setenv("TAIA_SESSION_DIR", "invalid-runner")
	dir := t.TempDir()
	if err := WriteAgentSnapshot(dir, "test", AgentUsage{AgentID: "test", Name: "override"}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadAgentSnapshot(dir, "test")
	if err != nil || got == nil || got.Usage.Name != "override" {
		t.Fatalf("override: %v %v", got, err)
	}
	if _, err := ResolvedStateDir(t.TempDir()); err == nil {
		t.Fatal("home argument bypassed invalid runner")
	}
}
