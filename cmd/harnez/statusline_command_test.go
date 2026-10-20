package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/usage"
)

type statuslineFlushBuffer struct {
	bytes.Buffer
	flushed bool
}

func (b *statuslineFlushBuffer) Flush() error {
	b.flushed = true
	return nil
}

func TestWriteStatuslineFlushesRenderedLine(t *testing.T) {
	var out statuslineFlushBuffer
	writeStatusline(&out, "status")
	if out.String() != "status\n" {
		t.Fatalf("statusline output = %q, want %q", out.String(), "status\n")
	}
	if !out.flushed {
		t.Fatal("statusline output was not flushed")
	}
}

func TestAppliedHarnezCommandsResolveOnRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("load embedded config: %v", err)
	}
	target := filepath.Join(home, ".claude")
	if err := claude.ApplyAll(target, cfg, nil, false, false, false); err != nil {
		t.Fatalf("apply embedded config: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(target, "settings.json"))
	if err != nil {
		t.Fatalf("read applied settings: %v", err)
	}
	var settings struct {
		StatusLine map[string]any `json:"statusLine"`
		Hooks      any            `json:"hooks"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse applied settings: %v", err)
	}

	commands := []string{}
	if command, _ := settings.StatusLine["command"].(string); strings.HasPrefix(command, "harnez ") {
		commands = append(commands, command)
	}
	commands = appendHarnezCommands(commands, settings.Hooks)
	if len(commands) == 0 {
		t.Fatal("apply output contained no harnez statusLine or hook commands")
	}

	root := newRootCmd()
	for _, command := range commands {
		tokens := strings.Fields(command)
		if len(tokens) < 2 || tokens[0] != "harnez" {
			t.Fatalf("unexpected configured command %q", command)
		}
		resolved, remaining, err := root.Find(tokens[1:])
		wantPath := "harnez " + strings.Join(tokens[1:], " ")
		if err != nil || len(remaining) != 0 || resolved.CommandPath() != wantPath {
			t.Errorf("configured command %q resolved to %q with remaining %v (error %v)", command, resolved.CommandPath(), remaining, err)
		}
	}
}

func appendHarnezCommands(commands []string, value any) []string {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if key == "command" {
				if command, ok := child.(string); ok && strings.HasPrefix(command, "harnez ") {
					commands = append(commands, command)
				}
			}
			commands = appendHarnezCommands(commands, child)
		}
	case []any:
		for _, child := range v {
			commands = appendHarnezCommands(commands, child)
		}
	}
	return commands
}

func TestStatuslineCommandAcceptsTicketPayload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := newRootCmd()
	cmd.SetArgs([]string{"statusline"})
	cmd.SetIn(strings.NewReader(`{"workspace":{"current_dir":"/tmp/project","project_dir":"/tmp","added_dirs":[],"repo":{"host":"codeberg.org","owner":"uwe","name":"harnez"}},"cwd":"/tmp/project","session_id":"session-1","session_name":"test","transcript_path":"/tmp/transcript"}`))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("harnez statusline exited with error: %v", err)
	}
}

func TestAGYStatuslinePrintsWhenStoreUnavailable(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "missing-data"))
	var out, stderr bytes.Buffer
	cmd := newStatuslineCmdWithObserver(func(context.Context, string, []byte) {})
	cmd.SetArgs([]string{"--agent", "agy"})
	cmd.SetIn(strings.NewReader(`{"context_window":{"current_usage":{"input_tokens":12000}}}`))
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("statusline with unavailable store returned error: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("statusline printed no output")
	}
	if stderr.Len() != 0 {
		t.Fatalf("statusline wrote store error to stderr: %q", stderr.String())
	}
}

func TestAGYStatuslineEmptyOrMalformedPayloadDoesNotAccessStore(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
		wantErr bool
	}{
		{name: "empty"},
		{name: "malformed", payload: "{", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dataHome := filepath.Join(root, "missing-data")
			t.Setenv("HOME", filepath.Join(root, "home"))
			t.Setenv("XDG_DATA_HOME", dataHome)
			var out bytes.Buffer
			cmd := newStatuslineCmdWithObserver(func(context.Context, string, []byte) {})
			cmd.SetArgs([]string{"--agent", "agy"})
			cmd.SetIn(strings.NewReader(tc.payload))
			cmd.SetOut(&out)
			err := cmd.Execute()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Execute() error = %v, want error %t", err, tc.wantErr)
			}
			if tc.wantErr && !strings.Contains(out.String(), "Usage:") {
				t.Fatalf("malformed payload output = %q, want the existing usage output", out.String())
			}
			if !tc.wantErr && out.Len() != 0 {
				t.Fatalf("empty payload printed %q", out.String())
			}
			if _, err := os.Stat(dataHome); !os.IsNotExist(err) {
				t.Fatalf("payload accessed or created store directory: stat error = %v", err)
			}
		})
	}
}

func TestAGYStatuslineLockedStoreStillPrintsWithinBudget(t *testing.T) {
	root := t.TempDir()
	dataHome := filepath.Join(root, "data")
	dbDir := filepath.Join(dataHome, "harnez")
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dbDir, "telemetry.sqlite")
	blocker, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := blocker.Exec(`CREATE TABLE lock_probe (x INTEGER)`); err != nil {
		blocker.Close()
		t.Fatal(err)
	}
	tx, err := blocker.Begin()
	if err != nil {
		blocker.Close()
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO lock_probe VALUES (1)`); err != nil {
		tx.Rollback()
		blocker.Close()
		t.Fatal(err)
	}
	defer tx.Rollback()
	defer blocker.Close()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_DATA_HOME", dataHome)
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	listener, err := net.Listen("unix", filepath.Join(t.TempDir(), "server.sock"))
	if err != nil {
		t.Fatal(err)
	}
	collector := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		http.Error(w, "collector busy", http.StatusServiceUnavailable)
	})}
	go func() { _ = collector.Serve(listener) }()
	defer collector.Close()
	address := listener.Addr().String()
	path, err := usage.PassiveUpdatePath()
	if err != nil {
		t.Fatal(err)
	}
	var result server.UpdateResult
	observe := func(ctx context.Context, provider string, data []byte) {
		result = usage.ObserveStatuslineTo(ctx, provider, data, address, path, cfg.UpdateTimeout)
	}
	var out, stderr bytes.Buffer
	cmd := newStatuslineCmdWithObserver(observe)
	cmd.SetArgs([]string{"--agent", "agy"})
	cmd.SetIn(strings.NewReader(`{"context_window":{"current_usage":{"input_tokens":12000}},"quota":{"gemini-weekly":{"remaining_fraction":0.75}}}`))
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	startTime := time.Now()
	err = cmd.Execute()
	elapsed := time.Since(startTime)
	if err != nil {
		t.Fatalf("statusline with locked store returned error: %v", err)
	}
	if limit := cfg.UpdateTimeout * raceFactor; elapsed > limit {
		t.Fatalf("statusline with locked store took %s, want <=%s", elapsed, limit)
	}
	select {
	case <-started:
	default:
		t.Fatal("statusline did not POST passive windows")
	}
	if result != server.UpdateDropped {
		t.Fatalf("update result = %q, want dropped", result)
	}
	if out.Len() == 0 {
		t.Fatal("statusline printed no output before locked write")
	}
	if stderr.Len() != 0 {
		t.Fatalf("statusline wrote an error to stderr: %q", stderr.String())
	}
	var count int
	if err := tx.QueryRow(`SELECT count(*) FROM lock_probe`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("locked database changed: count=%d err=%v", count, err)
	}
}

func TestAGYStatuslineUsesPayloadInsteadOfStoredQuota(t *testing.T) {
	var out bytes.Buffer
	cmd := newStatuslineCmdWithObserver(func(context.Context, string, []byte) {})
	cmd.SetArgs([]string{"--agent", "agy"})
	cmd.SetIn(strings.NewReader(`{"model":{"id":"gemini-pro"},"context_window":{"current_usage":{"input_tokens":12000}},"quota":{"gemini-weekly":{"remaining_fraction":0.75}}}`))
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("statusline exited with error: %v", err)
	}
	if !strings.Contains(out.String(), "75%") || strings.Contains(out.String(), "1%") {
		t.Fatalf("statusline output %q does not reflect payload quota only", out.String())
	}
}
