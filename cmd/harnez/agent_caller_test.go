package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func codexCallerFixture(t *testing.T, source string) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("CODEX_THREAD_ID", "host-thread")
	t.Setenv(agentRoleEnv, "advisor")
	t.Setenv(agentSessionEnv, "stale-worker")
	t.Setenv(agentLauncherEnv, "")
	dir := filepath.Join(os.Getenv("CODEX_HOME"), "sessions", "2026", "10", "26")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-example-host-thread.jsonl")
	data := fmt.Sprintf(`{"type":"session_meta","payload":{"id":"host-thread","session_id":"host-thread","cwd":"/work","source":%q}}`+"\n", source)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInteractiveCodexIgnoresInheritedWorkerIdentity(t *testing.T) {
	for _, source := range []string{"vscode", "cli"} {
		t.Run(source, func(t *testing.T) {
			codexCallerFixture(t, source)
			if got := currentHostSession(); got != "host-thread" {
				t.Fatalf("host identity = %q", got)
			}
			for _, verb := range []string{"start", "resume", "selftest", "stop", "compact"} {
				if err := guardLeafRole(verb); err != nil {
					t.Fatalf("host %s: %v", verb, err)
				}
			}
			if _, err := startRole("reviewer", "", nil); err != nil {
				t.Fatal(err)
			}
			if _, err := resumeRole("", &subagent.Session{Role: "reviewer"}); err != nil {
				t.Fatal(err)
			}
			if !canManageTarget("host-thread", &subagent.Session{ParentSessionID: "other"}) {
				t.Fatal("interactive host cannot manage target")
			}
			var out bytes.Buffer
			cmd := newAgentCmd()
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"selftest", "--step", "hello", "--state-dir", t.TempDir()})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("selftest hello: %v", err)
			}
		})
	}
}

func TestCodexWorkersAndUnknownMetadataStayRestricted(t *testing.T) {
	for _, variant := range []string{"exec", "unknown", "missing", "malformed", "wrong-id", "launcher", "bad-launcher", "legacy-worker"} {
		t.Run(variant, func(t *testing.T) {
			path := codexCallerFixture(t, "vscode")
			switch variant {
			case "exec", "unknown":
				codexCallerFixture(t, variant)
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "malformed", "wrong-id":
				data := "invalid json"
				if variant == "wrong-id" {
					data = `{"type":"session_meta","payload":{"id":"other","cwd":"/work","source":"vscode"}}`
				}
				if err := os.WriteFile(path, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			case "launcher":
				t.Setenv(agentLauncherEnv, strconv.Itoa(os.Getppid()))
			case "bad-launcher":
				t.Setenv(agentLauncherEnv, "invalid")
			case "legacy-worker":
				store, err := subagent.NewSessionStore(subagent.DefaultStoreDir())
				if err != nil {
					t.Fatal(err)
				}
				if err := store.Save(&subagent.Session{ID: "host-thread", Name: "stale-worker", Role: "advisor"}); err != nil {
					t.Fatal(err)
				}
			}
			for _, verb := range []string{"start", "resume", "selftest"} {
				err := guardLeafRole(verb)
				if err == nil || !strings.Contains(err.Error(), "is a leaf worker") || !strings.Contains(err.Error(), "role source: HARNEZ_AGENT_ROLE") {
					t.Fatalf("%s: expected contextual refusal, got %v", verb, err)
				}
			}
			if _, err := startRole("reviewer", "", nil); err == nil {
				t.Fatal("worker start allowed")
			}
			if _, err := resumeRole("", &subagent.Session{Role: "reviewer"}); err == nil {
				t.Fatal("worker resume allowed")
			}
		})
	}
}

func TestLauncherProvenanceExportedAndRestored(t *testing.T) {
	t.Setenv(agentLauncherEnv, "previous")
	restore := setAgentEnv("advisor", "worker", "codex:luna:low")
	if got := os.Getenv(agentLauncherEnv); got != strconv.Itoa(os.Getpid()) {
		t.Fatalf("launcher = %q", got)
	}
	restore()
	if got := os.Getenv(agentLauncherEnv); got != "previous" {
		t.Fatalf("restored launcher = %q", got)
	}
}
