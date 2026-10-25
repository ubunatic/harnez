package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/sshutil"
)

func indexConfig(t *testing.T) NeusIndexConfig {
	t.Helper()
	c, err := LoadNeusIndexConfig()
	if err != nil {
		t.Fatal(err)
	}
	c.Enabled = true
	c.Repos = []string{t.TempDir()}
	return c
}

func fakeIndexerTools(t *testing.T, status, scan string) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	t.Setenv("CALLS", log)
	t.Setenv("PATH", dir)
	for name, body := range map[string]string{
		"systemctl": "printf 'systemctl %s\\n' \"$*\" >> \"$CALLS\"\n" + status,
		"neus":      "printf 'neus %s\\n' \"$*\" >> \"$CALLS\"\n" + scan,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return log
}

func TestNeusIndexCycleFakeBinaries(t *testing.T) {
	for _, tc := range []struct {
		name, status, scan string
		indexed, wantErr   bool
	}{
		{"active", "exit 0", "exit 0", false, false},
		{"inactive", "exit 3", "exit 0", true, false},
		{"unit absent", "exit 4", "exit 0", true, false},
		{"unknown status", "exit 1", "exit 0", false, true},
		{"index failure", "exit 3", "exit 7", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := indexConfig(t)
			cfg.Repos = append(cfg.Repos, cfg.Repos[0])
			log := fakeIndexerTools(t, tc.status, tc.scan)
			s := NeusIndexer{Config: cfg}
			err := s.cycle(context.Background())
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v", err)
			}
			data, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			want := "systemctl --user is-active --quiet neus-indexer.service\n"
			if tc.indexed {
				want += "neus index " + cfg.Repos[0] + "\n"
			}
			if string(data) != want {
				t.Fatalf("calls = %q, want %q", data, want)
			}
		})
	}
}

func TestNeusIndexerInjectedTicker(t *testing.T) {
	cfg := indexConfig(t)
	log := fakeIndexerTools(t, "exit 3", "exit 0")
	ticks := make(chan time.Time)
	stopped := make(chan struct{})
	scanned := make(chan struct{}, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NeusIndexer{Config: cfg, Ticker: func(d time.Duration) (<-chan time.Time, func()) {
		want, _ := time.ParseDuration(cfg.Interval)
		if d != want {
			t.Errorf("interval = %v", d)
		}
		return ticks, func() { close(stopped) }
	}, RunCommand: func(ctx context.Context, name string, args ...string) error {
		err := exec.CommandContext(ctx, name, args...).Run()
		if name == "neus" {
			scanned <- struct{}{}
		}
		return err
	}}
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-stopped })
	for range 2 {
		ticks <- time.Time{}
		<-scanned
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "neus index ") != 2 {
		t.Fatalf("calls = %q", data)
	}
}

func TestNeusIndexerDoesNotOverlapAndJoinsCancellation(t *testing.T) {
	cfg := indexConfig(t)
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NeusIndexer{Config: cfg, RunCommand: func(ctx context.Context, name string, args ...string) error {
		if name == "systemctl" {
			return exec.ErrNotFound
		}
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}}
	done := make(chan error, 1)
	go func() { done <- s.cycle(ctx) }()
	<-started
	if err := s.cycle(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestNeusIndexerBackgroundChildHoldingStdoutDoesNotBlock(t *testing.T) {
	cfg := indexConfig(t)
	cfg.Timeout = "100ms"
	log := fakeIndexerTools(t, "exit 3", "exit 0")
	dir := filepath.Dir(log)
	pidFile := filepath.Join(dir, "sleeper.pid")
	t.Setenv("NEUS_SLEEP_PID", pidFile)
	if err := os.WriteFile(filepath.Join(dir, "neus"), []byte("#!/bin/sh\n/bin/sleep 30 &\nprintf '%s\\n' \"$!\" > \"$NEUS_SLEEP_PID\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	watchdog, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- (&NeusIndexer{Config: cfg}).cycle(watchdog) }()
	stopSleeper := func() {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if parseErr == nil {
				if process, findErr := os.FindProcess(pid); findErr == nil {
					_ = process.Kill()
				}
			}
		}
	}
	select {
	case err := <-done:
		stopSleeper()
		if err != nil {
			t.Fatal(err)
		}
	case <-watchdog.Done():
		stopSleeper()
		<-done
		t.Fatal("neus child holding stdout blocked command completion")
	}
}

func TestNeusIndexerDisabledAndInvalidRoots(t *testing.T) {
	cfg := indexConfig(t)
	cfg.Enabled = false
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := NeusIndexer{Config: cfg, Output: io.Discard, Ticker: func(time.Duration) (<-chan time.Time, func()) {
		t.Fatal("disabled indexer started ticker")
		return nil, nil
	}}
	if err := s.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalIndexRoots([]string{""}); err == nil {
		t.Fatal("accepted empty root")
	}
	if _, err := canonicalIndexRoots([]string{filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("accepted missing root")
	}
	cfg.Enabled, cfg.Repos = true, nil
	if cfg.Validate() == nil {
		t.Fatal("enabled indexing without registered roots")
	}
}

func TestServerSpecSchemaIncludesNeusIndexing(t *testing.T) {
	cfg, err := LoadNeusIndexConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled {
		t.Fatal("indexing must default off")
	}
	data, err := harnez.DefaultFS.ReadFile("spec/server.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	data, err = harnez.DefaultFS.ReadFile("spec/schemas/server.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(doc); err != nil {
		t.Fatal(err)
	}
}

func TestServerInstallPathSchemaParity(t *testing.T) {
	data, err := harnez.DefaultFS.ReadFile("spec/schemas/server.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, installPath := range []string{"/./tmp/x", "/var/./tmp/x", "~/./x", "/a/./b"} {
		t.Run(installPath, func(t *testing.T) {
			doc := map[string]any{
				"socket_path":         "server.sock",
				"read_header_timeout": "1s", "shutdown_timeout": "1s", "http_client_timeout": "1s",
				"probe_timeout": "1s", "usage_deadline": "1s", "update_timeout": "1s", "update_max_body_bytes": 1,
				"telemetry_update_path": "/update", "telemetry_spool_path": "spool", "telemetry_spool_max_bytes": 1,
				"startup_timeout": "1s", "startup_poll_interval": "1s", "takeover_timeout": "1s",
				"systemd_unit_name": "harnez.service", "systemd_unit_source": "systemd/harnez.service",
				"systemd_restart_policy": "on-failure", "systemd_restart_delay": "1s", "telemetry_warning_rate_limit": "1s",
				"default_log_lines": 1,
				"remote_upgrade": map[string]any{
					"install_path": installPath, "connect_timeout": "1s", "command_timeout": "1s", "transfer_timeout": "1s",
				},
				"neus_indexing": map[string]any{"enabled": false, "interval": "1s", "timeout": "1s", "command": []any{"neus"}, "service_name": "neus.service", "repos": []any{}},
			}
			if err := resolved.Validate(doc); err == nil {
				t.Errorf("schema accepted invalid install_path %q", installPath)
			}
			if sshutil.ValidInstallPath(installPath) {
				t.Errorf("runtime accepted invalid install_path %q", installPath)
			}
		})
	}
}
