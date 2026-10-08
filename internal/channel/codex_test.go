package channel

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadCodexRPCResponse(t *testing.T) {
	for _, tc := range []struct {
		name, input, wantErr string
	}{
		{"skips notification and unrelated response", "{\"jsonrpc\":\"2.0\",\"method\":\"thread/started\"}\n{\"jsonrpc\":\"2.0\",\"id\":9,\"result\":{}}\n{\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{\"ok\":true}}\n", ""},
		{"Codex response omits jsonrpc", "{\"id\":2,\"result\":{\"ok\":true}}\n", ""},
		{"rpc error", "{\"jsonrpc\":\"2.0\",\"id\":2,\"error\":{\"code\":-32603,\"message\":\"state db unavailable\"}}\n", "state db unavailable"},
		{"missing result", "{\"jsonrpc\":\"2.0\",\"id\":2}\n", "no result"},
		{"malformed json", "not-json\n", "invalid JSON-RPC"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := readCodexRPCResponse(bufio.NewReader(strings.NewReader(tc.input)), 2, 4096)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || string(response.Result) != `{"ok":true}` {
				t.Fatalf("response = %#v, %v", response, err)
			}
		})
	}
}

func TestDiscoverCodexThreadsUsesDBOnlyAndPaginates(t *testing.T) {
	executable := fakeCodexAppServer(t, `
while IFS= read -r line; do
  case "$line" in
    *notifications/initialized*) : ;;
    *method*initialize*) printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{}}' ;;
    *thread/list*)
      case "$line" in *useStateDbOnly*true*) ;; *) exit 5 ;; esac
      case "$line" in
        *cursor*next*) printf '%s\n' '{"jsonrpc":"2.0","id":3,"result":{"data":[{"id":"thread-2","name":"cati1","cwd":"/work/two"}],"nextCursor":null}}' ;;
        *) printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"data":[{"id":"thread-1","name":"first","cwd":"/work/one"}],"nextCursor":"next"}}' ;;
      esac ;;
    *) exit 4 ;;
  esac
done
`)
	threads, err := DiscoverCodexThreads(context.Background(), executable, time.Second*3, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 2 || threads[0] != (CodexThread{ID: "thread-1", Name: "first", CWD: "/work/one"}) || threads[1] != (CodexThread{ID: "thread-2", Name: "cati1", CWD: "/work/two"}) {
		t.Fatalf("threads = %#v", threads)
	}
}

func TestDiscoverCodexThreadsReportsMissingAndRPCFailures(t *testing.T) {
	if _, err := DiscoverCodexThreads(context.Background(), filepath.Join(t.TempDir(), "missing-codex"), time.Second, 4096); err == nil || !strings.Contains(err.Error(), "start codex app-server") {
		t.Fatalf("missing executable error = %v", err)
	}
	executable := fakeCodexAppServer(t, `
while IFS= read -r line; do
  case "$line" in
    *notifications/initialized*) : ;;
    *method*initialize*) printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{}}' ;;
    *thread/list*) printf '%s\n' '{"jsonrpc":"2.0","id":2,"error":{"code":-32603,"message":"state db unavailable"}}' ;;
    *) exit 4 ;;
  esac
done
`)
	if _, err := DiscoverCodexThreads(context.Background(), executable, time.Second*3, 4096); err == nil || !strings.Contains(err.Error(), "state db unavailable") {
		t.Fatalf("RPC failure error = %v", err)
	}
}

func fakeCodexAppServer(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}
