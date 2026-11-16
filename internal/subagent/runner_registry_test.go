package subagent

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"ubunatic.com/harnez/internal/server"
)

func TestRunnerModeBlocksDirectHostSessionStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	runnerRoot := filepath.Join(t.TempDir(), "runner-sess")
	if err := os.Mkdir(runnerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", runnerRoot)

	if _, err := NewSessionStore(DefaultStoreDir()); err == nil {
		t.Fatal("expected NewSessionStore(DefaultStoreDir()) to fail closed in runner mode")
	}
	if _, err := OpenSessionStore(DefaultStoreDir()); err == nil {
		t.Fatal("expected OpenSessionStore(DefaultStoreDir()) to fail closed in runner mode")
	}
	if _, err := os.Stat(DefaultStoreDir()); !os.IsNotExist(err) {
		t.Fatalf("DefaultStoreDir should not be created in runner mode: %v", err)
	}
}

func TestFileSessionStoreCreateStampsLaunchIDAndRunnerRegistryClient(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "run"))
	t.Setenv("TAIA_SESSION_DIR", "")

	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &Session{ID: "host-created", Name: "host-created", Provider: "codex", Model: "luna", Tier: "low"}
	if err := store.Create(s); err != nil {
		t.Fatal(err)
	}
	if s.LaunchID == "" {
		t.Fatal("expected Create to auto-stamp LaunchID")
	}

	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	hostSocket, err := cfg.Socket()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", hostSocket)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var relay server.RunnerRegistryRelayPayload
		if err := json.NewDecoder(r.Body).Decode(&relay); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		if relay.SessionID == "legacy-no-id" {
			http.Error(w, "missing launch identity", http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode(&Session{
			ID:       relay.SessionID,
			LaunchID: relay.LaunchID,
			Status:   relay.Request.Status,
			Name:     "worker-r",
		})
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	hostEnd, runnerEnd, err := server.SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.RelayRunnerBroker(ctx, hostEnd, hostSocket, cfg) }()
	t.Cleanup(func() {
		cancel()
		_ = runnerEnd.Close()
		if err := <-done; err != nil {
			_ = err
		}
	})

	runnerRoot := filepath.Join(t.TempDir(), "runner-sess")
	if err := os.Mkdir(runnerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", runnerRoot)
	t.Setenv(cfg.RunnerChannel.FDEnv, strconv.Itoa(int(runnerEnd.Fd())))

	reg, err := RegisterOwnSession(context.Background(), server.RunnerRegistryRequest{
		SessionID: "runner-own-1", Name: "worker-r", Status: "running",
	})
	if err != nil || reg.ID != "runner-own-1" || reg.LaunchID == "" {
		t.Fatalf("RegisterOwnSession = (%+v, %v)", reg, err)
	}
	upd, err := UpdateOwnSession(context.Background(), server.RunnerRegistryRequest{Status: "completed"})
	if err != nil || upd.Status != "completed" {
		t.Fatalf("UpdateOwnSession = (%+v, %v)", upd, err)
	}
	qry, err := QueryOwnSession(context.Background(), "")
	if err != nil || qry.ID != "runner-own-1" {
		t.Fatalf("QueryOwnSession = (%+v, %v)", qry, err)
	}
	if _, err := QueryOwnSession(context.Background(), "other-sess"); !errors.Is(err, server.ErrRunnerForbidden) {
		t.Fatalf("cross-session QueryOwnSession err = %v, want ErrRunnerForbidden", err)
	}
}
