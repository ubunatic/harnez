package server

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseRemoteBuildVersion(t *testing.T) {
	tests := []struct {
		input   string
		version string
		count   string
		valid   bool
	}{
		{"harnez version 0.2.1+12", "0.2.1", "12", true},
		{"harnez version v0.2.1", "0.2.1", "", true},
		{"harnez version not-a-version", "", "", false},
	}
	for _, test := range tests {
		got, ok := parseRemoteBuildVersion(test.input)
		if ok != test.valid {
			t.Errorf("parseRemoteBuildVersion(%q) valid = %v, want %v", test.input, ok, test.valid)
			continue
		}
		if ok {
			want, _ := parseRemoteBuildVersion(test.version + func() string {
				if test.count == "" {
					return ""
				}
				return "+" + test.count
			}())
			if got != want {
				t.Errorf("parseRemoteBuildVersion(%q) = %+v, want %+v", test.input, got, want)
			}
		}
	}
}

func TestLoadConfigRemoteUpgradeSettings(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RemoteUpgrade.InstallPath != "~/go/bin/harnez" || cfg.RemoteUpgrade.ConnectTimeout <= 0 || cfg.RemoteUpgrade.CommandTimeout <= 0 || cfg.RemoteUpgrade.TransferTimeout <= 0 {
		t.Fatalf("remote upgrade config = %+v", cfg.RemoteUpgrade)
	}
}

func TestStartRemoteBinaryUpgradeRunsInBackgroundAndStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	done := startRemoteBinaryUpgrade(ctx, "host", RemoteUpgradeConfig{CommandTimeout: time.Minute}, "1.2.3+4", "linux", "amd64", "unused", nil, runner)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background runner did not start")
	}
	select {
	case <-done:
		t.Fatal("background runner unexpectedly finished")
	default:
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("background runner did not stop after cancellation")
	}
}

func TestRemoteUpgradeDecisions(t *testing.T) {
	tests := []struct {
		name        string
		local       string
		remote      string
		platform    string
		probeErr    bool
		wantCopy    bool
		wantMessage string
	}{
		{"older", "0.2.1+20", "0.2.0", "Linux x86_64", false, true, "installed"},
		{"legacy equal release", "0.2.1+20", "0.2.1", "Linux x86_64", false, true, "installed"},
		{"legacy newer release", "0.2.1+20", "0.2.2", "Linux x86_64", false, false, "newer"},
		{"equal build", "0.2.1+20", "0.2.1+20", "Linux x86_64", false, false, "already runs"},
		{"newer build", "0.2.1+20", "0.2.1+21", "Linux x86_64", false, false, "newer"},
		{"unknown version", "0.2.1+20", "unknown", "Linux x86_64", false, false, "unknown version"},
		{"platform mismatch", "0.2.1+20", "0.2.0", "Linux aarch64", false, false, "differs"},
		{"ssh failure", "0.2.1+20", "0.2.0", "", true, false, "probe failed"},
		{"dev local build", "0.2.1", "0.2.0", "", false, false, "no comparable"},
		{"no host", "0.2.1+20", "0.2.0", "", false, false, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binary := t.TempDir() + "/harnez"
			if err := os.WriteFile(binary, []byte("binary"), 0755); err != nil {
				t.Fatal(err)
			}
			var calls []string
			var logs bytes.Buffer
			runner := func(_ context.Context, name string, args ...string) ([]byte, error) {
				calls = append(calls, name+" "+strings.Join(args, " "))
				if name == "scp" {
					return nil, nil
				}
				last := args[len(args)-1]
				if last == "uname -sm" {
					if test.probeErr {
						return nil, errors.New("ssh failed")
					}
					return []byte(test.platform), nil
				}
				if strings.HasSuffix(last, " --version") {
					return []byte("harnez version " + test.remote), nil
				}
				return nil, nil
			}
			upgrader := remoteBinaryUpgrader{config: RemoteUpgradeConfig{InstallPath: "~/go/bin/harnez"}, run: runner, log: &logs}
			upgrader.upgrade(context.Background(), map[bool]string{true: "host"}[test.name != "no host"], test.local, "linux", "amd64", binary)
			copied := false
			for _, call := range calls {
				copied = copied || strings.HasPrefix(call, "scp ")
			}
			if copied != test.wantCopy {
				t.Errorf("copy = %v, want %v; calls=%v", copied, test.wantCopy, calls)
			}
			if test.wantMessage != "" && !strings.Contains(logs.String(), test.wantMessage) {
				t.Errorf("log %q lacks %q", logs.String(), test.wantMessage)
			}
			if test.name == "no host" && len(calls) != 0 {
				t.Errorf("no-host calls = %v", calls)
			}
		})
	}
}
