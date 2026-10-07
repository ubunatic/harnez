package channel

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestUnknownSessionID(t *testing.T) {
	cfg := testConfig(t)
	for _, id := range []string{"unknown-session", "../../other-session"} {
		if _, err := Send(context.Background(), cfg, id, "hello"); !errors.Is(err, ErrUnknownSession) {
			t.Fatalf("unknown session %q: %v", id, err)
		}
	}
}

func TestStaleDeadPIDAndPort(t *testing.T) {
	for _, cause := range []string{"dead pid", "dead port"} {
		t.Run(cause, func(t *testing.T) {
			if cause == "dead pid" && runtime.GOOS == "windows" {
				t.Skip("Windows uses authenticated health checks rather than signal-zero PID checks")
			}
			cfg := testConfig(t)
			listener, err := net.Listen("tcp", cfg.ListenAddress)
			if err != nil {
				t.Fatal(err)
			}
			address := listener.Addr().String()
			_ = listener.Close()
			pid := os.Getpid()
			if cause == "dead pid" {
				pid = 1 << 30 // Above the Unix PID limit, so this PID cannot be live.
			}
			reg := registration{Endpoint: Endpoint{SessionID: "stale", PID: pid, URL: "http://" + address}, Token: rand.Text()}
			if err := register(context.Background(), cfg, reg); err != nil {
				t.Fatal(err)
			}
			_, err = Send(context.Background(), cfg, reg.SessionID, "hello")
			if !errors.Is(err, ErrStaleSession) || (cause == "dead pid" && !strings.Contains(err.Error(), "dead pid")) {
				t.Fatalf("stale send = %v", err)
			}
			endpoints, err := List(context.Background(), cfg)
			if err != nil || len(endpoints) != 0 {
				t.Fatalf("stale endpoint was advertised: %#v, %v", endpoints, err)
			}
		})
	}
}

func TestStaleEndpointIdentity(t *testing.T) {
	cfg := testConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Endpoint{SessionID: "different-session", PID: os.Getpid()})
	}))
	defer server.Close()
	reg := registration{Endpoint: Endpoint{SessionID: "stale", PID: os.Getpid(), URL: server.URL}, Token: rand.Text()}
	if err := register(context.Background(), cfg, reg); err != nil {
		t.Fatal(err)
	}
	if _, err := Send(context.Background(), cfg, reg.SessionID, "must not go to the wrong session"); !errors.Is(err, ErrStaleSession) {
		t.Fatalf("wrong endpoint identity not rejected: %v", err)
	}
}

func TestRestartReplacesStaleEntryAndRejectsLiveDuplicate(t *testing.T) {
	cfg := testConfig(t)
	listener, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		t.Fatal(err)
	}
	stale := registration{Endpoint: Endpoint{SessionID: "restart", PID: os.Getpid(), URL: "http://" + listener.Addr().String()}, Token: rand.Text()}
	_ = listener.Close()
	if err := register(context.Background(), cfg, stale); err != nil {
		t.Fatal(err)
	}
	f := startFixture(t, cfg, stale.SessionID)
	active, err := lookup(context.Background(), cfg, stale.SessionID)
	if err != nil || active.Token == stale.Token {
		t.Fatalf("restart did not replace stale endpoint: %v", err)
	}
	// A late cleanup by the former process must preserve the new registration.
	if err := unregister(cfg, stale); err != nil {
		t.Fatal(err)
	}
	if err := register(context.Background(), cfg, stale); err == nil {
		t.Fatal("duplicate registrar replaced a live endpoint")
	}
	if _, err := Send(context.Background(), cfg, stale.SessionID, "still live"); err != nil {
		t.Fatal(err)
	}
	_ = f.read(t)
}

func TestRegistryRejectsNonLoopbackEndpoint(t *testing.T) {
	cfg := testConfig(t)
	reg := registration{Endpoint: Endpoint{SessionID: "external", PID: os.Getpid(), URL: "http://192.0.2.1:80"}, Token: rand.Text()}
	if err := register(context.Background(), cfg, reg); err != nil {
		t.Fatal(err)
	}
	if _, err := Send(context.Background(), cfg, reg.SessionID, "hello"); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("sender accepted external registry URL: %v", err)
	}
}
