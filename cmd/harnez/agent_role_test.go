package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func TestAgentRoleStoredEnvAndPreamble(t *testing.T) {
	t.Setenv(agentRoleEnv, "")
	t.Setenv(agentSessionEnv, "")
	storeDir := t.TempDir()
	d := &scriptDriver{steps: []step{{0, subagent.Event{Kind: "session", Text: "thread-1"}}, {0, msg("CONFIRM: ok")}, {0, msg("done")}}}
	if _, err := runWithStore(t, d, storeDir, "start", "--name", "boss", "--role", "orchestrator", "run the sprint"); err != nil {
		t.Fatal(err)
	}
	if d.envRole != "orchestrator" || d.envSession != "boss" {
		t.Fatalf("child env role=%q session=%q", d.envRole, d.envSession)
	}
	if !strings.Contains(d.prompt, "Harnez role: orchestrator. You coordinate; you never write code") {
		t.Fatalf("preamble lacks orchestrator rules:\n%s", d.prompt)
	}
	if os.Getenv(agentRoleEnv) != "" || os.Getenv(agentSessionEnv) != "" {
		t.Fatal("agent environment leaked out of the command")
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Get("thread-1")
	if err != nil || sess.Role != "orchestrator" {
		t.Fatalf("stored role = %q, err=%v", sess.Role, err)
	}
	// resume keeps the stored role and rejects a conflicting one
	if _, err := runWithStore(t, d, storeDir, "resume", "--name", "boss", "go on"); err != nil {
		t.Fatal(err)
	}
	if d.envRole != "orchestrator" || !strings.Contains(d.prompt, "Harnez role: orchestrator.") {
		t.Fatalf("resume env role=%q prompt:\n%s", d.envRole, d.prompt)
	}
	if _, err := runWithStore(t, d, storeDir, "resume", "--name", "boss", "--role", "developer", "x"); err == nil || !strings.Contains(err.Error(), "conflicts with session") {
		t.Fatalf("conflicting --role err = %v", err)
	}
}

func TestAgentStartDefaultsToDeveloperRole(t *testing.T) {
	t.Setenv(agentRoleEnv, "")
	installUnclassifiedNeus(t)
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
	if _, err := runWithStore(t, d, t.TempDir(), "start", "task"); err != nil {
		t.Fatal(err)
	}
	if d.envRole != "developer" || !strings.Contains(d.prompt, "You are a leaf worker") || strings.Contains(d.prompt, "Never run `harnez agent`") {
		t.Fatalf("env=%q prompt:\n%s", d.envRole, d.prompt)
	}
	if _, err := runWithStore(t, d, t.TempDir(), "start", "--role", "wizard", "task"); err == nil || !strings.Contains(err.Error(), "unknown agent role") || !strings.Contains(err.Error(), "explorer") || !strings.Contains(err.Error(), "Research the code") {
		t.Fatalf("unknown role err = %v; want full role catalog", err)
	}
}

func installUnclassifiedNeus(t *testing.T) {
	t.Helper()
	binDir := t.TempDir()
	neus := filepath.Join(binDir, "neus")
	if err := os.WriteFile(neus, []byte("#!/bin/sh\nexit 4\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
}

func TestStartRoleInfersFromPromptAndLogsNotice(t *testing.T) {
	t.Setenv(agentRoleEnv, "")
	binDir := t.TempDir()
	neus := filepath.Join(binDir, "neus")
	if err := os.WriteFile(neus, []byte("#!/bin/sh\nprintf '%s\\n' '{\"class\":\"reviewer\"}'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	var stderr bytes.Buffer
	role, err := startRole("", "Audit the patch. Then summarize findings.", &stderr)
	if err != nil || role != "reviewer" {
		t.Fatalf("startRole() = %q, %v; want reviewer", role, err)
	}
	if got, want := stderr.String(), "harnez: inferred role \"reviewer\" from prompt\n"; got != want {
		t.Fatalf("notice = %q, want %q", got, want)
	}
	stderr.Reset()
	role, err = startRole("reviewer", "do the task", &stderr)
	if err != nil || role != "reviewer" || stderr.Len() != 0 {
		t.Fatalf("explicit canonical role = %q, %v, notice=%q", role, err, stderr.String())
	}
	if err := os.WriteFile(neus, []byte("#!/bin/sh\nexit 4\n"), 0755); err != nil {
		t.Fatal(err)
	}
	role, err = startRole("", "Unclear request.", &stderr)
	if err != nil || role != "developer" {
		t.Fatalf("ambiguous prompt fallback = %q, %v; want developer", role, err)
	}
	if _, err := startRole("wizard", "", &stderr); err == nil || !strings.Contains(err.Error(), "known roles:") {
		t.Fatalf("unknown requested role error = %v; want role catalog", err)
	}
}

func TestAgentStartResolvesRoleAliasAndHelpListsRoles(t *testing.T) {
	t.Setenv(agentRoleEnv, "")
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
	storeDir := t.TempDir()
	if _, err := runWithStore(t, d, storeDir, "start", "--role", "explorer", "task"); err != nil {
		t.Fatal(err)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Get("thread-1")
	if err != nil || sess.Role != "advisor" || d.envRole != "advisor" {
		t.Fatalf("alias session role=%q env=%q err=%v; want advisor", sess.Role, d.envRole, err)
	}

	for _, tc := range []struct {
		args []string
		name string
		role string
		text string
	}{
		{[]string{"start", "explorer", "audit SQLite queries"}, "", "advisor", "audit SQLite queries"},
		{[]string{"start", "coder", "my-worker", "implement ticket 041"}, "my-worker", "developer", "implement ticket 041"},
	} {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
			dir := t.TempDir()
			if _, err := runWithStore(t, d, dir, tc.args...); err != nil {
				t.Fatal(err)
			}
			store, err := subagent.NewSessionStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			var started *subagent.Session
			if tc.name != "" {
				var sessions []*subagent.Session
				sessions, err = store.List("", true)
				for _, sess := range sessions {
					if sess.Name == tc.name {
						started = sess
						break
					}
				}
			} else {
				started, err = store.Get("thread-1")
			}
			if err != nil || started == nil || started.Role != tc.role || d.envRole != tc.role || !strings.Contains(d.prompt, tc.text) {
				t.Fatalf("session=%#v envRole=%q prompt=%q err=%v", started, d.envRole, d.prompt, err)
			}
		})
	}

	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"start", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"explorer", "advisor", "developer"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("start help missing %q:\n%s", want, out.String())
		}
	}
}

func TestLeafRolesCannotStartOrManageAgents(t *testing.T) {
	storeDir := t.TempDir()
	saveSessions(t, storeDir, &subagent.Session{ID: "id-a", Name: "a"})
	for _, role := range []string{"developer", "reviewer", "advisor"} {
		t.Setenv(agentRoleEnv, role)
		for _, args := range [][]string{
			{"start", "task"}, {"resume", "--name", "a", "x"}, {"-p", "hello"}, {"--name", "a", "--", "hello"}, {"-p", "/stop", "--name", "a"},
			{"compact", "--name", "a"}, {"stop", "--name", "a"}, {"delete", "--name", "a"}, {"enable"},
		} {
			d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
			_, err := runWithStore(t, d, storeDir, args...)
			if err == nil || !strings.Contains(err.Error(), "is a leaf worker") {
				t.Fatalf("%s %v: err = %v, want leaf-worker refusal", role, args, err)
			}
			if len(d.resumed) != 0 || d.prompt != "" {
				t.Fatalf("%s %v reached the provider", role, args)
			}
		}
		for _, args := range [][]string{{"list"}, {"status"}, {"status", "--name", "a"}, {"models"}} {
			if _, err := runWithStore(t, &recordingAgentDriver{}, storeDir, args...); err != nil {
				t.Fatalf("%s %v must stay available: %v", role, args, err)
			}
		}
	}
}

func TestOrchestratorMayStartHelpersButNotOrchestrators(t *testing.T) {
	t.Setenv(agentRoleEnv, "orchestrator")
	t.Setenv(agentSessionEnv, "boss")
	storeDir := t.TempDir()
	for _, role := range []string{"", "developer", "reviewer", "advisor"} {
		d := &scriptDriver{sid: "thread-" + role, steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
		args := []string{"start", "task"}
		if role != "" {
			args = append(args, "--role", role)
		}
		if _, err := runWithStore(t, d, storeDir, args...); err != nil {
			t.Fatalf("role %q: %v", role, err)
		}
		if d.envSession == "boss" {
			t.Fatal("helper must get its own session name as parent id, not the orchestrator's")
		}
	}
	d := &scriptDriver{steps: []step{{0, msg("done")}}}
	_, err := runWithStore(t, d, storeDir, "start", "--role", "orchestrator", "task")
	if err == nil || !strings.Contains(err.Error(), "may only start developer, reviewer, advisor sessions") || d.prompt != "" {
		t.Fatalf("nested orchestrator err = %v", err)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sessions, _ := store.List("", true)
	for _, s := range sessions {
		if s.ParentSessionID != "boss" {
			t.Fatalf("session %s parent = %q, want boss", s.Name, s.ParentSessionID)
		}
	}
}
