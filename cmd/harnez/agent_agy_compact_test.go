package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func TestAgentAgyOnDemandCompact(t *testing.T) {
	for _, command := range [][]string{{"compact"}, {"-p", "/compact"}} {
		for _, failed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/failed=%t", strings.Join(command, " "), failed), func(t *testing.T) {
				storeDir, workDir := t.TempDir(), t.TempDir()
				saveSessions(t, storeDir, &subagent.Session{
					ID: "registry-id", ProviderSessionID: "agy-conversation", Name: "worker",
					Provider: "agy", Model: "gemini-3.8-flash", Tier: "low",
					WorkingDir: workDir, ContextTokens: -1, HarnessType: "batch",
				})
				var calls int
				driver := subagent.AgyDriver{Dir: workDir, Command: func(_ context.Context, exe string, args ...string) ([]byte, error) {
					calls++
					want := []string{"--conversation", "agy-conversation", "--add-dir", workDir, "--output-format", "json", "-p", "/compact"}
					if exe != "agy" || !reflect.DeepEqual(args, want) {
						t.Fatalf("command = %s %q, want agy %q", exe, args, want)
					}
					if failed {
						return []byte(`{"status":"ERROR","error":"summary failed"}`), nil
					}
					return []byte(`{"conversation_id":"agy-conversation","status":"SUCCESS","response":""}`), nil
				}}
				args := append(append([]string{}, command...), "--name", "worker")
				_, err := runWithStore(t, driver, storeDir, args...)
				if failed {
					if err == nil || !strings.Contains(err.Error(), "summary failed") {
						t.Fatalf("error = %v, want summary failed", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				if calls != 1 {
					t.Fatalf("agy calls = %d, want 1", calls)
				}
			})
		}
	}
}
