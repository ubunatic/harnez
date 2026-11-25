package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/agent"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/modelcatalog"
)

func TestCodexObservedModelAccess(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	model := subagent.Model{Provider: "codex", Name: "gpt-6-luna", Tier: "low"}
	cases := []struct{ name, message, want string }{
		{"study", "The 'no-such-model-x' model is not supported when using Codex with a ChatGPT account.", "unsupported"},
		{"code", "HTTP 400 model_not_found: gpt-6-luna", "unsupported"},
		{"code without model", "HTTP 400 model_not_found", "unknown"},
		{"code other model", "HTTP 400 model_not_found: gpt-6-terra", "unknown"},
		{"code longer model", "HTTP 400 model_not_found: gpt-6-luna-extra", "unknown"},
		{"other", "HTTP 400 invalid request", "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := filepath.Join(root, tc.name)
			t.Setenv("XDG_STATE_HOME", state)
			script := "#!/bin/sh\nprintf '%s\\n' " + fmt.Sprintf("'%s'", strings.ReplaceAll(fmt.Sprintf(`{"type":"turn.failed","error":{"message":%q}}`, tc.message), "'", "'\\''")) + "\nexit 1\n"
			if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			_, err := (subagent.CodexDriver{}).Run(context.Background(), subagent.RunOptions{Model: model, Prompt: "test"})
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("provider error: %v", err)
			}
			recordCodexAccess(&bytes.Buffer{}, "codex", model.Name, err)
			got, _, err := modelcatalog.LatestModelAccess("codex", model.Name)
			if err != nil || got != tc.want {
				t.Fatalf("access = %q, %v; want %q", got, err, tc.want)
			}
			if tc.want == "unsupported" {
				row := agent.ModelRows([]subagent.ModelEntry{{Model: model}}, map[string]usage.ProviderQuotaAvailability{}, nil, time.Now())[0]
				if row.Startable || row.StartableReason != "unsupported for this account" {
					t.Fatalf("row: %+v", row)
				}
				recordCodexAccess(&bytes.Buffer{}, "codex", model.Name, nil)
				got, _, err = modelcatalog.LatestModelAccess("codex", model.Name)
				if err != nil || got != "accessible" {
					t.Fatalf("success access = %q, %v", got, err)
				}
			}
		})
	}
}

type accessTestCodexDriver struct{ subagent.CodexDriver }

func TestAgentModelsObservedCodexAccess(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("CODEX_HOME", filepath.Join(root, "codex"))
	t.Setenv(agentSessionEnv, "")
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	oldDriver, oldQuota := agentDriver, agentTurnQuotaReader
	agentDriver = func(m subagent.Model, dir string) subagent.Driver { return accessTestCodexDriver{} }
	agentTurnQuotaReader = func(context.Context, string, bool, time.Time, time.Time) usage.TurnQuotaReading {
		return usage.TurnQuotaReading{}
	}
	t.Cleanup(func() { agentDriver, agentTurnQuotaReader = oldDriver, oldQuota })
	writeCodex := func(output string, exit int) {
		t.Helper()
		script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '%s'\nexit %d\n", strings.ReplaceAll(output, "'", "'\\''"), exit)
		if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := newAgentCmd()
		var out, stderr bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		cmd.SetArgs(args)
		err := cmd.Execute()
		if err != nil {
			return out.String(), fmt.Errorf("%w (stderr: %s)", err, stderr.String())
		}
		return out.String(), nil
	}
	store := filepath.Join(root, "sessions")
	start := func(name string) error {
		_, err := run("start", "--role", "advisor", "--model", "codex:luna:low", "--name", name, "hello", "--store-dir", store)
		return err
	}
	message := "The 'gpt-6-luna' model is not supported when using Codex with a ChatGPT account."
	failure, err := json.Marshal(map[string]any{"type": "turn.failed", "error": map[string]string{"message": message}})
	if err != nil {
		t.Fatal(err)
	}
	writeCodex(string(failure), 1)
	if err := start("failed"); err == nil || !strings.Contains(err.Error(), message) {
		t.Fatalf("failure = %v", err)
	}
	table, err := run("models", "--store-dir", store)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range strings.Split(table, "\n") {
		if strings.Contains(line, "codex:luna:low") && strings.Contains(line, "unsupported for this account") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing unsupported table row: %s", table)
	}
	checkJSON := func(want string, startable bool) {
		t.Helper()
		output, err := run("models", "--json", "--store-dir", store)
		if err != nil {
			t.Fatal(err)
		}
		var rows []agent.ModelRow
		if err := json.Unmarshal([]byte(output), &rows); err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.Spec == "codex:luna:low" {
				if row.Entitlement.State != want || row.Startable != startable || (!startable && (row.Entitlement.Source != "model-access" || row.StartableReason != "unsupported for this account")) {
					t.Fatalf("model row = %+v", row)
				}
				return
			}
		}
		t.Fatal("missing codex:luna:low JSON row")
	}
	checkJSON("denied", false)
	writeCodex("{\"type\":\"thread.started\",\"thread_id\":\"fake-thread\"}\n{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"done\"}}\n{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}", 0)
	if err := start("success-one"); err != nil {
		t.Fatal(err)
	}
	checkJSON("verified", true)
	state := filepath.Join(store, "models", "access.yaml")
	first, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := start("success-two"); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("repeated success changed access state")
	}
}

func TestModelAccessDirFollowsStoreOverride(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv(subagent.StoreDirEnv, "")
	cmd := &cobra.Command{Use: "start"}
	if got := modelAccessDir(cmd); got != "" {
		t.Fatalf("no override: modelAccessDir = %q, want default state (empty)", got)
	}

	envStore := filepath.Join(root, "env-store")
	t.Setenv(subagent.StoreDirEnv, envStore)
	if got := modelAccessDir(cmd); got != filepath.Join(envStore, "models") {
		t.Fatalf("HARNEZ_STORE_DIR: modelAccessDir = %q", got)
	}

	root2 := &cobra.Command{Use: "agent"}
	root2.PersistentFlags().String("store-dir", "", "")
	child := &cobra.Command{Use: "start"}
	root2.AddCommand(child)
	flagStore := filepath.Join(root, "flag-store")
	if err := root2.PersistentFlags().Set("store-dir", flagStore); err != nil {
		t.Fatal(err)
	}
	if got := modelAccessDir(child); got != filepath.Join(flagStore, "models") {
		t.Fatalf("--store-dir: modelAccessDir = %q, want the flag store beating the env store", got)
	}
}

func TestRecordCodexAccessUnderStoreOverrideWritesOnlyInStore(t *testing.T) {
	root := t.TempDir()
	store := filepath.Join(root, "store")
	state := filepath.Join(root, "state")
	for _, dir := range []string{store, state} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv(subagent.StoreDirEnv, store)
	if err := os.Chmod(state, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(state, 0700) })

	var stderr bytes.Buffer
	modelcatalog.SetAccessDir(modelAccessDir(&cobra.Command{Use: "start"}))
	t.Cleanup(func() { modelcatalog.SetAccessDir("") })
	recordCodexAccess(&stderr, "codex", "gpt-6-luna", nil)
	if stderr.Len() != 0 {
		t.Fatalf("start printed a warning: %q", stderr.String())
	}
	for _, name := range []string{"access.yaml", "access.yaml.lock"} {
		if _, err := os.Stat(filepath.Join(store, "models", name)); err != nil {
			t.Fatalf("%s not beneath the store override: %v", name, err)
		}
	}
	entries, err := os.ReadDir(state)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("default state dir was written: %d entries", len(entries))
	}
}

func TestAgentStartUnderStoreEnvWritesNothingOutsideStore(t *testing.T) {
	root := t.TempDir()
	store := filepath.Join(root, "store")
	state := filepath.Join(root, "state")
	for _, dir := range []string{store, state, filepath.Join(root, "bin")} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", root)
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("CODEX_HOME", filepath.Join(root, "codex"))
	t.Setenv(agentSessionEnv, "")
	t.Setenv(subagent.StoreDirEnv, store)
	bin := filepath.Join(root, "bin")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	oldDriver, oldQuota := agentDriver, agentTurnQuotaReader
	agentDriver = func(m subagent.Model, dir string) subagent.Driver { return accessTestCodexDriver{} }
	agentTurnQuotaReader = func(context.Context, string, bool, time.Time, time.Time) usage.TurnQuotaReading {
		return usage.TurnQuotaReading{}
	}
	t.Cleanup(func() { agentDriver, agentTurnQuotaReader = oldDriver, oldQuota })
	script := "#!/bin/sh\nprintf '%s\\n' '{\"type\":\"thread.started\",\"thread_id\":\"fake-thread\"}'\nprintf '%s\\n' '{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"done\"}}'\nprintf '%s\\n' '{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}'\n"
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(state, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(state, 0700) })

	cmd := newAgentCmd()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"start", "--role", "advisor", "--model", "codex:luna:low", "--name", "env-start", "hello"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("start: %v (stderr: %s)", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "could not record model access") {
		t.Fatalf("start printed an access warning: %s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(store, "models", "access.yaml")); err != nil {
		t.Fatalf("access record not beneath the store: %v", err)
	}
	entries, err := os.ReadDir(state)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("default state dir was written: %d entries", len(entries))
	}
}
