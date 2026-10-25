package server

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
			t.Setenv("HOME", t.TempDir())
			t.Setenv("SSH_AUTH_SOCK", "")
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
						return []byte("Bad owner or permissions\nignored detail"), errors.New("exit status 255")
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
			if test.probeErr && (!strings.Contains(logs.String(), "Bad owner or permissions") || strings.Contains(logs.String(), "ignored detail")) {
				t.Errorf("probe failure log should contain only the first output line: %q", logs.String())
			}
			if test.name == "no host" && len(calls) != 0 {
				t.Errorf("no-host calls = %v", calls)
			}
		})
	}
}

func TestRemoteUpgradeUsesSSHConfigForSSHAndSCP(t *testing.T) {
	for _, hasConfig := range []bool{false, true} {
		name := "without config"
		if hasConfig {
			name = "with config"
		}
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("SSH_AUTH_SOCK", "")
			configPath := filepath.Join(home, ".ssh", "config")
			if hasConfig {
				if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(configPath, []byte("Host example\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			binary := filepath.Join(t.TempDir(), "harnez")
			if err := os.WriteFile(binary, []byte("binary"), 0755); err != nil {
				t.Fatal(err)
			}
			var calls [][]string
			runner := func(_ context.Context, name string, args ...string) ([]byte, error) {
				calls = append(calls, append([]string{name}, args...))
				last := args[len(args)-1]
				if name == "ssh" && last == "uname -sm" {
					return []byte("Linux x86_64"), nil
				}
				if name == "ssh" && strings.HasSuffix(last, " --version") {
					return []byte("harnez version 0.2.0"), nil
				}
				return nil, nil
			}
			(remoteBinaryUpgrader{config: RemoteUpgradeConfig{InstallPath: "~/go/bin/harnez"}, run: runner}).upgrade(context.Background(), "example", "0.2.1+20", "linux", "amd64", binary)

			for _, call := range calls {
				gotConfig := false
				for i := 0; i+1 < len(call); i++ {
					gotConfig = gotConfig || (call[i] == "-F" && call[i+1] == configPath)
				}
				if (call[0] == "ssh" || call[0] == "scp") && gotConfig != hasConfig {
					t.Errorf("%s config option = %v, want %v; call=%v", call[0], gotConfig, hasConfig, call)
				}
			}
			if !hasConfig {
				for _, call := range calls {
					if call[0] == "ssh" && len(call) > 1 && call[len(call)-1] == "~/go/bin/harnez --version" {
						return // A working key without an agent still reaches the version probe.
					}
				}
				t.Errorf("version probe did not run without SSH_AUTH_SOCK: %v", calls)
			}
		})
	}
}

func TestRemoteUpgradeLogsClearSkipForMissingSSHAuthentication(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	var calls int
	var logs bytes.Buffer
	runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		calls++
		return []byte("Permission denied (publickey)."), errors.New("exit status 255")
	}
	(remoteBinaryUpgrader{config: RemoteUpgradeConfig{}, run: runner, log: &logs}).upgrade(context.Background(), "example", "0.2.1+20", "linux", "amd64", "unused")
	if calls != 1 {
		t.Fatalf("SSH calls = %d, want one failed probe", calls)
	}
	if got := logs.String(); strings.Count(got, "skipping example") != 1 || !strings.Contains(got, "no agent socket or usable key") {
		t.Fatalf("skip log = %q", got)
	}
}
