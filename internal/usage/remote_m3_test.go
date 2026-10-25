package usage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoteLoadSSHConfig(t *testing.T) {
	for _, config := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "present"}[config], func(t *testing.T) {
			home := isolatedRemoteLoadEnv(t)
			configPath := filepath.Join(home, ".ssh", "config")
			if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
				t.Fatal(err)
			}
			if config {
				if err := os.WriteFile(configPath, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			argsPath := filepath.Join(home, "args")
			t.Setenv("SSH_TEST_ARGS", argsPath)
			t.Setenv("PATH", home+string(os.PathListSeparator)+os.Getenv("PATH"))
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SSH_TEST_ARGS\"\nprintf '  ssh diagnostic  \\n' >&2\nexit 255\n"
			if err := os.WriteFile(filepath.Join(home, "ssh"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			_, err := CollectRemoteLoadSnapshot(context.Background(), "example-host")
			if err == nil || !strings.Contains(err.Error(), "failed: ssh diagnostic") {
				t.Fatalf("error = %v, want trimmed SSH stderr", err)
			}
			data, err := os.ReadFile(argsPath)
			if err != nil {
				t.Fatal(err)
			}
			args := string(data)
			if strings.Contains(args, "-F\n"+configPath+"\n") != config {
				t.Fatalf("SSH args = %q, config present = %v", args, config)
			}
			if !config && strings.Contains(args, "-F\n") {
				t.Fatalf("unexpected config flag: %q", args)
			}
			if strings.Contains(args, "-q\n") || !strings.Contains(args, "BatchMode=yes\n") || !strings.Contains(args, "ConnectTimeout=") {
				t.Fatalf("unexpected SSH options: %q", args)
			}
		})
	}
}

func TestRemoteLoadLogsStateChanges(t *testing.T) {
	isolatedRemoteLoadEnv(t)
	_, opt, ctx := scriptedPoll(t, errors.New("  ssh unavailable \n"), errors.New("again"), nil, nil)
	var out bytes.Buffer
	opt.Out = &out
	runRemoteLoadPoller(ctx, opt)
	want := "harnez server: remote load x600 failed: ssh unavailable\nharnez server: remote load x600 recovered\n"
	if out.String() != want {
		t.Fatalf("log = %q, want %q", out.String(), want)
	}
}

func TestRemoteLoadStreamSampleKillsAndReapsAfterFirstRecord(t *testing.T) {
	home := isolatedRemoteLoadEnv(t)
	t.Setenv("PATH", home+string(os.PathListSeparator)+os.Getenv("PATH"))
	observed := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)
	stream := `{"cpu":{"NumCPU":8,"Memory":{"UsedMiB":20,"TotalMiB":40,"SwapUsedMiB":2,"SwapTotalMiB":8}},"gpus":[{"Name":"GPU0"}],"observed_at":"` + observed.Format(time.RFC3339) + `"}`
	script := "#!/bin/sh\nprintf '%s\\n' '" + stream + "'\nexec sleep 30\n"
	if err := os.WriteFile(filepath.Join(home, "ssh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	snapshot, err := collectRemoteLoadStreamSample(context.Background(), "example-host")
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("one-sample SSH took %s; it may have waited for the stream to end", elapsed)
	}
	if snapshot.ObservedAt.Unix() != observed.Unix() || snapshot.CPU.NumCPU != 8 || len(snapshot.GPUs) != 1 {
		t.Fatalf("remote sample = %+v, want timestamp %s and CPU/GPU readings", snapshot, observed)
	}
}

func TestRemoteLoadStreamMissingCommandReturnsError(t *testing.T) {
	home := isolatedRemoteLoadEnv(t)
	t.Setenv("PATH", home+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(home, "ssh"), []byte("#!/bin/sh\nprintf '%s\\n' 'harnez: unknown command load-stream' >&2\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	_, err := collectRemoteLoadStreamSample(context.Background(), "example-host")
	if err == nil || !strings.Contains(err.Error(), "unknown command load-stream") {
		t.Fatalf("error = %v, want legacy remote command diagnostic", err)
	}
}

func TestRemoteLoadCachePreservesObservationTime(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	observed := time.Now().Add(-10 * time.Minute).UTC().Truncate(time.Second)
	snapshot := &LoadSnapshot{CPU: CPULoad{NumCPU: 8}, ObservedAt: observed}
	if err := WriteCachedRemoteLoad("x600", snapshot); err != nil {
		t.Fatal(err)
	}
	if err := WriteCachedRemoteLoad("x600", snapshot); err != nil {
		t.Fatal(err)
	}
	entry := ReadCachedRemoteLoadEntry("x600")
	if !entry.UpdatedAt.Equal(observed) {
		t.Fatalf("cached observation time = %s, want %s", entry.UpdatedAt, observed)
	}
	if got := remoteLoadAgeLabel(entry.UpdatedAt, time.Now()); !strings.Contains(got, "stale") {
		t.Fatalf("age label = %q, want stale old sample", got)
	}
}
