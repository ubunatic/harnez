package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"ubunatic.com/harnez/internal/quota1"
	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagestore"
)

func testRatingPtr(rating int) *int { return &rating }

func TestAgentCommandSurface(t *testing.T) {
	c := newAgentCmd()
	if len(c.Commands()) == 0 {
		t.Fatal("agent command has no children")
	}
	for _, name := range []string{"start", "models", "resume", "list", "status", "wait", "compact", "stop", "delete", "enable", "disable"} {
		found := false
		for _, child := range c.Commands() {
			if child.Name() == name {
				found = true
			}
		}
		if !found {
			t.Errorf("missing agent subcommand %q", name)
		}
	}
	for _, child := range c.Commands() {
		if child.Name() == "chat" {
			t.Fatal("obsolete agent chat command remains")
		}
	}
	for _, name := range []string{"start", "resume"} {
		child, _, err := c.Find([]string{name})
		if err != nil {
			t.Fatal(err)
		}
		if flag := child.Flags().Lookup("interactive"); flag == nil || flag.Shorthand != "i" {
			t.Errorf("%s interactive flag missing or has wrong shorthand: %#v", name, flag)
		}
	}
}

func TestManagedAgentExamplesParseAgainstCobra(t *testing.T) {
	config, err := os.ReadFile("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	sectionStart := strings.Index(string(config), "## Harnez Agent")
	if sectionStart < 0 {
		t.Fatal("managed Harnez Agent section not found")
	}
	sectionEnd := strings.Index(string(config[sectionStart:]), "## Code and Documentation Search")
	if sectionEnd < 0 {
		t.Fatal("end of managed Harnez Agent section not found")
	}
	section := string(config[sectionStart : sectionStart+sectionEnd])
	for _, phrase := range []string{
		"A requested model such as `terra:low` or `luna` is a Harnez agent model",
		"dispatch it with `harnez agent start --model <name>`, regardless of `subagent_mode`",
		"When a `/goal` without an exit clause is set (e.g. typed by the user), say so in the first reply",
		"/goal ... or stop and report when blocked on a user decision or denied permission",
		"once blocked on the user, suggest `/goal clear` instead of repeating the wait message",
	} {
		if !strings.Contains(section, phrase) {
			t.Errorf("managed Harnez Agent section is missing goal rule %q", phrase)
		}
	}
	examples := regexp.MustCompile("`(harnez agent [^`]+)`").FindAllStringSubmatch(section, -1)
	if len(examples) == 0 {
		t.Fatal("no harnez agent examples found in managed section")
	}
	for _, example := range examples {
		fields := strings.Fields(example[1])
		cmd, remaining, err := newAgentCmd().Find(fields[2:])
		if err != nil {
			t.Errorf("%q: resolve Cobra command: %v", example[1], err)
			continue
		}
		if err := cmd.ParseFlags(remaining); err != nil && !errors.Is(err, pflag.ErrHelp) {
			t.Errorf("%q: parse Cobra flags: %v", example[1], err)
			continue
		}
	}
}

func TestAgentWaitTimeoutAndCompletion(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "wait-session", Name: "worker", Status: "running", HarnessType: "harnez", ProcessPID: os.Getpid()}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--store-dir", storeDir, "wait", "--timeout", "20ms", "--json", "wait-session"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"status":"running"`) {
		t.Fatalf("timeout output = %s", out.String())
	}
	sess.Status = "completed"
	sess.Response = "finished"
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	cmd = newAgentCmd()
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--store-dir", storeDir, "wait", "--timeout", "1s", "--json", "wait-session"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"response":"finished"`) {
		t.Fatalf("completion output = %s", out.String())
	}
}

func TestAgentWaitConcurrentCompletions(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"wait-one", "wait-two"} {
		if err := store.Save(&subagent.Session{ID: id, Name: id, Status: "running"}); err != nil {
			t.Fatal(err)
		}
	}
	results := make(chan *subagent.Session, 2)
	errs := make(chan error, 2)
	for _, id := range []string{"wait-one", "wait-two"} {
		go func(id string) {
			sess, err := waitForAgent(context.Background(), store, id, time.Second)
			results <- sess
			errs <- err
		}(id)
	}
	for _, id := range []string{"wait-two", "wait-one"} {
		sess, err := store.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		sess.Status = "completed"
		sess.Response = id
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	got := map[string]bool{}
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
		sess := <-results
		if sess == nil || sess.Status != "completed" || sess.Response != sess.ID {
			t.Fatalf("wait result = %#v", sess)
		}
		got[sess.ID] = true
	}
	if len(got) != 2 {
		t.Fatalf("completed sessions = %#v", got)
	}
}

func TestAgentWaitCancellation(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "cancel-wait", Name: "cancel-wait", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := waitForAgent(ctx, store, "cancel-wait", 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait error = %v, want context.Canceled", err)
	}
}

func TestAgentWaitFailedWorkerIsNotOrphaned(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worker := exec.Command(os.Args[0], "-test.run=^TestWaitWorkerCrashHelper$")
	worker.Env = append(os.Environ(), "HARNEZ_WAIT_CRASH_HELPER=1")
	if err := worker.Start(); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "crashed-worker", Name: "crashed-worker", Status: "running", ProcessPID: worker.Process.Pid}); err != nil {
		_ = worker.Process.Kill()
		t.Fatal(err)
	}
	if err := worker.Wait(); err != nil {
		t.Fatalf("crash helper: %v", err)
	}
	sess, err := waitForAgent(context.Background(), store, "crashed-worker", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != "failed" || sess.ProcessPID != 0 {
		t.Fatalf("crashed worker session = %#v", sess)
	}
}

func TestWaitWorkerCrashHelper(t *testing.T) {
	if os.Getenv("HARNEZ_WAIT_CRASH_HELPER") == "1" {
		os.Exit(0)
	}
}

func TestAgentDetachedSpawnAndWait(t *testing.T) {
	installUnclassifiedNeus(t)
	storeDir := t.TempDir()
	helperBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(t.TempDir(), "worker-launcher")
	script := "#!/bin/sh\nstore=''\nid=''\nwhile test $# -gt 0; do\n case \"$1\" in\n --store-dir) store=$2; shift 2 ;;\n --worker-session) id=$2; shift 2 ;;\n *) shift ;;\n esac\ndone\nHARNEZ_TEST_WORKER_STORE=$store HARNEZ_TEST_WORKER_ID=$id HARNEZ_TEST_BINARY='" + helperBinary + "' exec '" + helperBinary + "' -test.run=TestDetachedLaunchHelper\n"
	if err := os.WriteFile(launcher, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	old := agentExecutable
	agentExecutable = func() (string, error) { return launcher, nil }
	defer func() { agentExecutable = old }()
	cmd := newAgentCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err = launchDetachedWithPreflight(cmd, startRequest{Name: "async-worker", Prompt: "task", StoredPrompt: "task", ModelSpec: "codex:luna:low", Dir: t.TempDir(), JSON: true}, storeDir, "", func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	var created subagent.Session
	if err := json.Unmarshal(out.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != "running" || created.ProcessPID == 0 || created.StdoutLog == "" || created.StderrLog == "" {
		t.Fatalf("created session = %#v", created)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	finished, err := waitForAgent(context.Background(), store, created.ID, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != "completed" || finished.Response != "helper done" {
		t.Fatalf("finished session = %#v", finished)
	}
	if processExists(created.ProcessPID) {
		t.Fatalf("worker process %d remains after wait returned", created.ProcessPID)
	}
}

func TestForegroundStartAndResumeDetachIntoWaitableWorkers(t *testing.T) {
	t.Setenv(foregroundDetachTestOverrideEnv, "1")
	t.Setenv(execTimeoutEffectiveEnv, "100ms")
	t.Setenv(execTimeoutExplicitEnv, "0")
	t.Setenv(execTimeoutShortEnv, "")
	t.Setenv(execTimeoutEnv, "")
	t.Setenv(agentRoleEnv, "")
	t.Setenv(agentSessionEnv, "")
	oldGrace := foregroundDetachGrace
	foregroundDetachGrace = time.Millisecond
	defer func() { foregroundDetachGrace = oldGrace }()

	helperBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(t.TempDir(), "foreground-worker-launcher")
	script := "#!/bin/sh\nstore=''\nid=''\nwhile test $# -gt 0; do\n case \"$1\" in\n --store-dir) store=$2; shift 2 ;;\n --worker-session) id=$2; shift 2 ;;\n *) shift ;;\n esac\ndone\nHARNEZ_TEST_FOREGROUND_STORE=$store HARNEZ_TEST_FOREGROUND_ID=$id exec '" + helperBinary + "' -test.run=^TestForegroundWorkerHelper$\n"
	if err := os.WriteFile(launcher, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	oldExecutable := agentExecutable
	agentExecutable = func() (string, error) { return launcher, nil }
	defer func() { agentExecutable = oldExecutable }()
	oldDriver := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return &scriptDriver{} }
	defer func() { agentDriver = oldDriver }()

	for _, verb := range []string{"start", "resume"} {
		t.Run(verb, func(t *testing.T) {
			storeDir := t.TempDir()
			store, err := subagent.NewSessionStore(storeDir)
			if err != nil {
				t.Fatal(err)
			}
			name := "foreground-" + verb
			deps := agentDeps{
				store:    func() (*subagent.FileSessionStore, error) { return store, nil },
				storeDir: storeDir,
				parent:   func() string { return "" },
				find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
					return s.Find(id)
				},
			}
			cmd := newAgentCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			if verb == "start" {
				err = runStart(cmd, deps, startRequest{Name: name, Prompt: "task", StoredPrompt: "task", ModelSpec: "codex:luna:low", Dir: t.TempDir(), StreamMode: streamFull})
			} else {
				if err := store.Save(&subagent.Session{ID: "resume-id", Name: name, Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: t.TempDir(), Status: "completed", ContextTokens: 10}); err != nil {
					t.Fatal(err)
				}
				err = runResume(cmd, deps, resumeRequest{Name: name, Prompt: "continue", StreamMode: streamFull})
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, directive := range []string{"status=running", "cleanly detached", "Do NOT poll", "Do NOT schedule", "harnez agent wait " + name, "automatically notify"} {
				if !strings.Contains(out.String(), directive) {
					t.Errorf("detach output %q missing %q", out.String(), directive)
				}
			}
			sess, err := store.Find(name)
			if err != nil {
				t.Fatal(err)
			}
			if sess.Status != "running" || sess.ProcessPID <= 0 || sess.StdoutLog == "" || sess.StderrLog == "" {
				t.Fatalf("detached session = %#v", sess)
			}
			pgid, err := syscall.Getpgid(sess.ProcessPID)
			if err != nil || pgid != sess.ProcessPID {
				t.Fatalf("worker process group = %d, err=%v; want independent group %d", pgid, err, sess.ProcessPID)
			}

			waitCmd := newAgentCmd()
			var waitOut bytes.Buffer
			waitCmd.SetOut(&waitOut)
			waitCmd.SetArgs([]string{"--store-dir", storeDir, "wait", name})
			if err := waitCmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(waitOut.String(), "finished "+name) {
				t.Fatalf("wait output = %q", waitOut.String())
			}
		})
	}
}

func TestForegroundDetachDisabledInGoTestMode(t *testing.T) {
	t.Setenv(foregroundDetachTestOverrideEnv, "")
	t.Setenv(execTimeoutEffectiveEnv, "60s")
	t.Setenv(execTimeoutExplicitEnv, "0")
	t.Setenv(execTimeoutShortEnv, "")
	t.Setenv(execTimeoutEnv, "")
	if got := foregroundDetachTimeout(newAgentCmd()); got != 0 {
		t.Fatalf("foreground detach timeout = %s, want disabled in test mode", got)
	}
}

func TestForegroundWorkerEnvironmentOmitsLeafRole(t *testing.T) {
	t.Setenv(agentRoleEnv, "developer")
	for _, entry := range foregroundWorkerEnv() {
		if strings.HasPrefix(entry, agentRoleEnv+"=") {
			t.Fatalf("worker environment contains leaf role: %q", entry)
		}
	}
}

func TestWriteDetachGuidanceJSON(t *testing.T) {
	sess := &subagent.Session{ID: "session-id", Name: "calm-otter", Status: "running"}
	var out bytes.Buffer
	writeDetachGuidance(&out, sess, true)
	var got struct {
		Session      subagent.Session `json:"session"`
		Status       string           `json:"status"`
		Detached     bool             `json:"detached"`
		Message      string           `json:"message"`
		Instructions []string         `json:"instructions"`
		WaitCommand  string           `json:"wait_command"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("detach guidance is not JSON: %v (%s)", err, out.String())
	}
	if got.Session.ID != sess.ID || got.Session.Name != sess.Name || got.Status != "running" || !got.Detached {
		t.Fatalf("detach JSON = %#v", got)
	}
	if got.WaitCommand != "harnez agent wait calm-otter" || !strings.Contains(strings.Join(got.Instructions, " "), "Do NOT poll") || !strings.Contains(got.Message, "detached") {
		t.Fatalf("detach instructions = %#v", got)
	}
}

func TestAgentWorkerSessionCLIFlags(t *testing.T) {
	t.Setenv(agentRoleEnv, "")
	t.Setenv(agentSessionEnv, "")
	driver := &recordingAgentDriver{}
	oldDriver := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = oldDriver }()

	for _, tc := range []struct {
		name, verb, id, sessionName string
	}{
		{name: "start", verb: "start", id: "worker-start", sessionName: "start-worker"},
		{name: "resume", verb: "resume", id: "worker-resume", sessionName: "resume-worker"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storeDir := t.TempDir()
			store, err := subagent.NewSessionStore(storeDir)
			if err != nil {
				t.Fatal(err)
			}
			initial := &subagent.Session{ID: tc.id, Name: tc.sessionName, Provider: "claude", Model: "haiku", Tier: "low", ProviderSessionID: "provider-session", WorkingDir: t.TempDir(), Role: "developer", HarnessType: "harnez", Status: "running", ProcessPID: os.Getpid(), ContextTokens: 10}
			if tc.verb == "start" {
				initial.StartPrompt = "stored prompt"
			} else {
				initial.Status = "completed"
			}
			if err := store.Save(initial); err != nil {
				t.Fatal(err)
			}
			cmd := newAgentCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(new(bytes.Buffer))
			args := []string{"--store-dir", storeDir, tc.verb, "--worker-session", tc.id, "--json", "--name", tc.sessionName}
			if tc.verb == "start" {
				args = append(args, "--model", "claude:haiku:low", "--dir", initial.WorkingDir)
			}
			args = append(args, "--", "worker prompt")
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("execute %s --worker-session: %v", tc.verb, err)
			}
			var result agentOutput
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("%s worker output is not JSON: %v (%s)", tc.verb, err, out.String())
			}
			finished, err := store.Get(tc.id)
			if err != nil {
				t.Fatal(err)
			}
			if finished.Status != "completed" || finished.ProcessPID != 0 || result.Session == nil {
				t.Fatalf("%s worker result=%#v stored=%#v", tc.verb, result, finished)
			}
		})
	}
}

func TestForegroundWorkerHelper(t *testing.T) {
	storeDir, sessionID := os.Getenv("HARNEZ_TEST_FOREGROUND_STORE"), os.Getenv("HARNEZ_TEST_FOREGROUND_ID")
	if storeDir == "" || sessionID == "" {
		t.Skip("foreground worker helper")
	}
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	var sess *subagent.Session
	for i := 0; i < 100; i++ {
		sess, err = store.Get(sessionID)
		if err != nil {
			t.Fatal(err)
		}
		if sess.ProcessPID == os.Getpid() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if sess.ProcessPID != os.Getpid() {
		t.Fatal("parent did not register foreground worker PID")
	}
	time.Sleep(300 * time.Millisecond)
	sess.Status = "completed"
	sess.ProcessPID = 0
	sess.Response = "finished " + sess.Name
	sess.Messages = []string{sess.Response}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(agentOutput{Session: sess, Response: sess.Response, Messages: sess.Messages}); err != nil {
		t.Fatal(err)
	}
}

func TestDetachedLaunchHelper(t *testing.T) {
	storeDir, id := os.Getenv("HARNEZ_TEST_WORKER_STORE"), os.Getenv("HARNEZ_TEST_WORKER_ID")
	if storeDir == "" || id == "" {
		t.Skip("subprocess helper")
	}
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		sess, err := store.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if sess.ProcessPID == os.Getpid() {
			sess.Status = "completed"
			sess.ProcessPID = 0
			sess.Response = "helper done"
			if err := store.Save(sess); err != nil {
				t.Fatal(err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("parent did not register worker PID")
}

func TestMCPCommandRegistered(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"mcp"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name() != "mcp" {
		t.Fatalf("command = %q", cmd.Name())
	}
}

func TestAgentCompletionDescription(t *testing.T) {
	long := strings.Repeat("prompt ", 30)
	tests := []struct {
		name, prompt, want string
	}{
		{name: "missing", want: "agent prompt unavailable"},
		{name: "short", prompt: "fix the build", want: "fix the build"},
		{name: "multiline", prompt: "first line\nsecond\tline", want: "first line second line"},
		{name: "sensitive", prompt: "email me at user@example.com", want: "email me at [redacted-email]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sessionPromptDescription(tt.prompt); got != tt.want {
				t.Fatalf("description = %q, want %q", got, tt.want)
			}
		})
	}
	if got := sessionPromptDescription(long); len([]rune(got)) != 100 || !strings.HasSuffix(got, "...") {
		t.Fatalf("long description = %q, want 100 runes with ellipsis", got)
	}
}

func TestAgentReferenceCommandCompletionCoverage(t *testing.T) {
	root := newAgentCmd()
	for _, path := range [][]string{{"resume"}, {"status"}, {"compact"}, {"stop"}, {"delete"}} {
		cmd, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("find %v: %v", path, err)
		}
		if cmd.ValidArgsFunction == nil {
			t.Errorf("%s has no session completion", strings.Join(path, " "))
		}
	}
}

func TestAgentNameCompletion(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "one", Name: "calm-otter", StartPrompt: "Investigate the release process"}); err != nil {
		t.Fatal(err)
	}
	completer := agentSessionCompletion(storeDir, func() string { return "" })
	completions, directive := completer(newAgentCmd(), nil, "cal")
	if directive != cobra.ShellCompDirectiveNoFileComp || len(completions) != 1 || completions[0] != "calm-otter\tInvestigate the release process" {
		t.Fatalf("completion = %#v, directive = %v", completions, directive)
	}
}

func TestAgentModelsListsKnownSpecs(t *testing.T) {
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"models"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "codex:luna:low") || !strings.Contains(out.String(), "agy:flash37:low") {
		t.Fatalf("known models output = %q", out.String())
	}
}

func TestAgentModelsTableShowsRolesAndUse(t *testing.T) {
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"models"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(out.String(), "\n")
	if !strings.HasPrefix(lines[0], "SPEC") || !strings.Contains(lines[0], "COST") || !strings.Contains(lines[0], "EFF") || !strings.Contains(lines[0], "SKILLS") || !strings.Contains(lines[0], "ROLES") || !strings.Contains(lines[0], "USE") {
		t.Fatalf("header = %q", lines[0])
	}
	var low, med, high, opus, haiku string
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "claude:haiku:low"):
			haiku = line
		case strings.HasPrefix(line, "codex:luna:low"):
			low = line
		case strings.HasPrefix(line, "codex:luna:med"):
			med = line
		case strings.HasPrefix(line, "codex:luna:high"):
			high = line
		case strings.HasPrefix(line, "agy:opus:low"):
			opus = line
		}
	}
	if !strings.Contains(low, "developer") || !strings.Contains(low, "clear bounded tickets") || !strings.Contains(low, " 1 ") || !strings.Contains(low, "Go~ TUI- SQL?") {
		t.Fatalf("luna:low row = %q", low)
	}
	if !strings.Contains(med, "interface or design changes") {
		t.Fatalf("luna:med row must use use_med: %q", med)
	}
	if !strings.Contains(med, " 4 ") || !strings.Contains(high, " 6 ") {
		t.Fatalf("tier costs not shown: med=%q high=%q", med, high)
	}
	if !strings.Contains(opus, " no ") {
		t.Fatalf("agy:opus row must report no effort support: %q", opus)
	}
	if !strings.Contains(haiku, " yes ") {
		t.Fatalf("claude rows must report effort support: %q", haiku)
	}
	if !strings.Contains(out.String(), "\nCOST is estimated by effort tier; codex:luna:low = 1") {
		t.Fatalf("legend line missing: %q", out.String())
	}
}

func TestAgentModelsNamesPrintsBareSpecs(t *testing.T) {
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"models", "--names"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(out.String())
	if strings.Join(got, "\n") != strings.Join(subagent.KnownModelSpecs(), "\n") {
		t.Fatalf("--names output = %q", out.String())
	}
}

func TestAgentStartRejectsUnknownModelWithoutCreatingSession(t *testing.T) {
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver {
		t.Fatal("provider driver must not be called")
		return nil
	}
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "codex:missing:low", "x", "--store-dir", storeDir})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "model is now --model") {
		t.Fatalf("start error = %v, want model flag guidance", err)
	}
	store, storeErr := subagent.NewSessionStore(storeDir)
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	sessions, listErr := store.List("", true)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(sessions) != 0 {
		t.Fatalf("sessions after rejected start = %#v, want none", sessions)
	}
}

func TestAgentOldModelSpecGuard(t *testing.T) {
	tests := []struct {
		name, first string
		reject      bool
	}{
		{"known", "codex:luna:low", false},
		{"mistyped known provider", "codex:missing:low", true},
		{"unknown provider remains prompt", "note:fix", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := agentDriver
			d := &recordingAgentDriver{}
			agentDriver = func(subagent.Model, string) subagent.Driver { return d }
			defer func() { agentDriver = old }()
			cmd := newAgentCmd()
			var errOut bytes.Buffer
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(&errOut)
			args := []string{"start", tt.first, "prompt", "--store-dir", t.TempDir()}
			if tt.first == "codex:luna:low" {
				args = []string{"start", "--model", tt.first, "prompt", "--store-dir", t.TempDir()}
			}
			cmd.SetArgs(args)
			err := cmd.Execute()
			if tt.reject {
				if err == nil || !strings.Contains(err.Error(), "model is now --model") {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAgentResumeRequiresName(t *testing.T) {
	storeDir := t.TempDir()
	var errOut bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"resume", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "multiple resumable agents") && !strings.Contains(err.Error(), "no resumable agent") {
		t.Fatalf("error = %v", err)
	}
}

func TestAgentStartFileOnlyAndNameCollision(t *testing.T) {
	old := agentDriver
	d := &recordingAgentDriver{}
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	file := filepath.Join(t.TempDir(), "prompt.md")
	if err := os.WriteFile(file, []byte("full file prompt"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "--name", "named", "-f", file, "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Find("named")
	if err != nil || !strings.Contains(sess.StartPrompt, file+" (16 bytes)") || strings.Contains(sess.StartPrompt, "full file prompt") {
		t.Fatalf("stored prompt = %#v, err=%v", sess, err)
	}
	cmd = newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "--name", "named", "again", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "recorded") || !strings.Contains(err.Error(), sess.WorkingDir) {
		t.Fatalf("collision error = %v", err)
	}
}

func TestAgentResumeModelConflict(t *testing.T) {
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver {
		t.Fatal("provider driver must not be called")
		return nil
	}
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", Tier: "low", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "--name", "worker", "--model", "codex:luna:latest", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("conflict error = %v", err)
	}
}

type recordingInteractiveRunner struct {
	storeDir string
	opts     subagent.InteractiveOptions
	active   bool
	attachID string
	err      error
}

func (r *recordingInteractiveRunner) Chat(_ context.Context, opts subagent.InteractiveOptions) error {
	r.opts = opts
	store, err := subagent.NewSessionStore(r.storeDir)
	if err != nil {
		return err
	}
	sess, err := store.Find(opts.Name)
	r.active = err == nil && sess.Status == "active" && sess.HarnessType == "interactive"
	if err != nil {
		return err
	}
	return r.err
}

func TestAgentInteractiveFailureTransition(t *testing.T) {
	old := agentInteractiveRunner
	storeDir := t.TempDir()
	agentInteractiveRunner = &recordingInteractiveRunner{storeDir: storeDir, err: errors.New("provider exited")}
	defer func() { agentInteractiveRunner = old }()
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "-i", "--model", "claude:haiku", "--name", "bold-fox", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "provider exited") {
		t.Fatalf("chat error = %v", err)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Find("bold-fox")
	if err != nil || sess.Status != "failed" {
		t.Fatalf("failed session = %#v, %v", sess, err)
	}
}

func TestAgentInteractiveActiveControlAndDeletion(t *testing.T) {
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver {
		t.Fatal("provider driver must not be called")
		return nil
	}
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	socket := filepath.Join(storeDir, "active.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	actions := make(chan struct {
		Action string `json:"action"`
		Prompt string `json:"prompt"`
	}, 3)
	go func() {
		for range 3 {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			var request struct {
				Action string `json:"action"`
				Prompt string `json:"prompt"`
			}
			_ = json.NewDecoder(conn).Decode(&request)
			actions <- request
			_ = json.NewEncoder(conn).Encode(struct{}{})
			_ = conn.Close()
		}
	}()
	// The socket is mocked; use an exited PID instead of a literal that could
	// belong to an unrelated real process during stop's ownership checks.
	exited := exec.Command("true")
	if err := exited.Run(); err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "active-id", ProviderSessionID: "provider-id", Name: "active", Provider: "claude", Model: "haiku", HarnessType: "interactive", Status: "active", ControlSocket: socket, ProcessPID: exited.Process.Pid, ContextTokens: 100, Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1, Rating: testRatingPtr(4)}}}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"resume", "--name", "active", "orchestrator prompt"}, {"compact", "--name", "active"}, {"stop", "--name", "active"}} {
		cmd := newAgentCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs(append(args, "--store-dir", storeDir))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	for _, want := range []struct{ action, prompt string }{{"prompt", "orchestrator prompt"}, {"compact", ""}, {"stop", ""}} {
		got := <-actions
		if got.Action != want.action || got.Prompt != want.prompt {
			t.Errorf("control = %#v, want %#v", got, want)
		}
	}
	sess, err = store.Find("active")
	if err != nil || sess.Status != "stopped" {
		t.Fatalf("stopped session = %#v, %v", sess, err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "--name", "active", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Find("active"); err == nil {
		t.Fatal("completed/stopped interactive registry metadata was not deleted")
	}
	completed := &subagent.Session{ID: "completed-id", Name: "completed", Provider: "agy", Model: "gemini-3.7-flash", HarnessType: "interactive", Status: "completed", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1, Rating: testRatingPtr(4)}}}
	if err := store.Save(completed); err != nil {
		t.Fatal(err)
	}
	cmd = newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "--name", "completed", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(completed.ID); err == nil {
		t.Fatal("completed interactive registry metadata was not deleted")
	}
}

func TestAgentInteractiveMissingProviderIDLimitsPostExitOnly(t *testing.T) {
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "codex-id", Name: "codex-chat", Provider: "codex", Model: "gpt-5.6-luna", HarnessType: "interactive", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "-i", "--name", "codex-chat", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "did not expose a provider session ID") {
		t.Fatalf("attach error = %v", err)
	}
}

func (r *recordingInteractiveRunner) Attach(_ context.Context, opts subagent.InteractiveOptions, providerID string) error {
	r.opts = opts
	r.attachID = providerID
	return nil
}

func TestAgentInteractiveRegistersBeforeLaunchAndTransitions(t *testing.T) {
	old := agentInteractiveRunner
	storeDir := t.TempDir()
	runner := &recordingInteractiveRunner{storeDir: storeDir}
	agentInteractiveRunner = runner
	defer func() { agentInteractiveRunner = old }()

	dir := t.TempDir()
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetIn(bytes.NewBufferString("input"))
	cmd.SetArgs([]string{"start", "-i", "--model", "claude:haiku", "--name", "calm-otter", "-d", dir, "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !runner.active {
		t.Fatal("session was not registered as active interactive before launch")
	}
	if runner.opts.Stdin == nil || runner.opts.Stdout == nil || runner.opts.Stderr == nil {
		t.Fatal("interactive terminal streams were not inherited")
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Find("calm-otter")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != "completed" || sess.ProviderSessionID != sess.ID || sess.CallerPID != os.Getpid() || sess.WorkingDir != dir {
		t.Fatalf("saved interactive session = %#v", sess)
	}

	cmd = newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "-i", "--name", "calm-otter", "--store-dir", storeDir, "--", "continue interactively"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.attachID != sess.ID {
		t.Fatalf("attach provider ID = %q, want %q", runner.attachID, sess.ID)
	}
	if runner.opts.Prompt != "continue interactively" {
		t.Fatalf("resume opening prompt = %q", runner.opts.Prompt)
	}
}

func TestAgentInteractiveResumeDefaultsToLatestInDirectory(t *testing.T) {
	old := agentInteractiveRunner
	storeDir := t.TempDir()
	runner := &recordingInteractiveRunner{storeDir: storeDir}
	agentInteractiveRunner = runner
	defer func() { agentInteractiveRunner = old }()
	dir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	for _, sess := range []*subagent.Session{
		{ID: "old", Name: "old-session", ProviderSessionID: "old-provider", Provider: "agy", Model: "gemini-3.7-flash", WorkingDir: dir, HarnessType: "interactive", Status: "completed", LastActiveAt: time.Now().Add(-time.Minute)},
		{ID: "new", Name: "new-session", ProviderSessionID: "new-provider", Provider: "agy", Model: "gemini-3.7-flash", WorkingDir: dir, HarnessType: "interactive", Status: "active", ProcessPID: 99999999, LastActiveAt: time.Now()},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "-i", "-d", dir, "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.attachID != "new-provider" {
		t.Fatalf("attached provider ID = %q, want new-provider", runner.attachID)
	}
}

func TestInteractiveProviderIDIsPersistedDuringRun(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "session", Name: "session", Provider: "codex", Model: "gpt-5.6-luna", HarnessType: "interactive", Status: "active"}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	opts := interactiveOptions(newAgentCmd(), store, sess, storeDir, "")
	if err := opts.ProviderIDFound("codex-thread"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Get(sess.ID)
	if err != nil || loaded.ProviderSessionID != "codex-thread" {
		t.Fatalf("stored provider session ID = %#v, %v", loaded, err)
	}
}

func TestAgentInteractivePromptAndFlagConflicts(t *testing.T) {
	old := agentInteractiveRunner
	storeDir := t.TempDir()
	runner := &recordingInteractiveRunner{storeDir: storeDir}
	agentInteractiveRunner = runner
	defer func() { agentInteractiveRunner = old }()

	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "-i", "--model", "claude:haiku", "--store-dir", storeDir, "--", "opening line"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.opts.Prompt != "opening line" {
		t.Fatalf("opening prompt = %q", runner.opts.Prompt)
	}

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"detach", []string{"start", "-i", "--detach"}, "--detach/--async"},
		{"async", []string{"start", "-i", "--async"}, "--detach/--async"},
		{"timeout", []string{"start", "-i", "--timeout", "0"}, "--timeout"},
		{"json", []string{"start", "-i", "--json"}, "--json"},
		{"stream", []string{"start", "-i", "--stream", "stats"}, "--stream"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newAgentCmd()
			c.SetOut(new(bytes.Buffer))
			c.SetErr(new(bytes.Buffer))
			c.SetArgs(append(tc.args, "--store-dir", t.TempDir()))
			if err := c.Execute(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("conflict error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestAgentResumeInteractiveRejectsRunningBackgroundSession(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "busy-id", ProviderSessionID: "provider-id", Name: "busy-worker", Provider: "claude", Model: "haiku", WorkingDir: t.TempDir(), HarnessType: "harnez", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "-i", "--name", "busy-worker", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "harnez agent wait busy-worker") {
		t.Fatalf("busy-session error = %v", err)
	}
}

func TestAgentInteractiveGeneratesUniqueNameAndRejectsDuplicate(t *testing.T) {
	old := agentInteractiveRunner
	storeDir := t.TempDir()
	agentInteractiveRunner = &recordingInteractiveRunner{storeDir: storeDir}
	defer func() { agentInteractiveRunner = old }()

	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "-i", "--model", "claude:haiku", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sessions, err := store.List("", true)
	if err != nil || len(sessions) != 1 || sessions[0].Name == "" {
		t.Fatalf("generated session registration = %#v, %v", sessions, err)
	}
	cmd = newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "-i", "--model", "claude:haiku", "--name", sessions[0].Name, "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected duplicate name error")
	}
}

func TestAgentRepoStatusAndPolicyCommands(t *testing.T) {
	dir := t.TempDir()
	store := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.local.md"), []byte("local notes\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"enable", "-d", dir, "--store-dir", store})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, ".harnez", "rules", "Local.md"))
	if err != nil || !bytes.Contains(content, []byte("subagent_mode: harnez")) {
		t.Fatalf("enable did not update local overlay: %v\n%s", err, content)
	}
	legacy, err := os.ReadFile(filepath.Join(dir, "AGENTS.local.md"))
	if err != nil || string(legacy) != "local notes\n" {
		t.Fatalf("enable did not preserve legacy owner prose: %v\n%s", err, legacy)
	}
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "-d", dir, "--store-dir", store})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Subagent Policy State: Enabled (./.harnez/rules/Local.md)")) || !bytes.Contains(out.Bytes(), []byte("Repository Agent Sessions:")) {
		t.Fatalf("status output = %s", out.String())
	}
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"disable", "-d", dir, "--store-dir", store})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(filepath.Join(dir, ".harnez", "rules", "Local.md"))
	if err != nil || !bytes.Contains(content, []byte("subagent_mode: native")) {
		t.Fatalf("disable did not update local overlay: %v\n%s", err, content)
	}
}

func TestAgentRepoStatusUnsetJSONAndSingleSession(t *testing.T) {
	dir, storeDir := t.TempDir(), t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "inside", Name: "demo", Provider: "codex", Model: "model", Status: "completed", WorkingDir: dir, LastActiveAt: time.Now()}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "-d", dir, "--store-dir", storeDir, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Policy struct {
			Mode string `json:"mode"`
		} `json:"policy"`
		Sessions []subagent.Session `json:"sessions"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Policy.Mode != "unset" || len(got.Sessions) != 1 || got.Sessions[0].ID != "inside" {
		t.Fatalf("repo status = %#v", got)
	}
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "--name", "inside", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil || !bytes.Contains(out.Bytes(), []byte("ID: inside\nName: demo\nStatus: completed")) {
		t.Fatalf("single session status: err=%v output=%s", err, out.String())
	}
}

type recordingAgentDriver struct {
	dir     string
	prompt  string
	stopped []string
	deleted []string
	runHook func()
}

type resumeOutcomeDriver struct {
	recordingAgentDriver
	err      error
	resumes  int
	terminal bool
}

func (d *resumeOutcomeDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	d.resumes++
	if d.err != nil {
		return nil, d.err
	}
	return &subagent.TurnResult{Response: "resumed"}, nil
}

func (d *resumeOutcomeDriver) CheckResumable(string) (bool, string) {
	if d.terminal {
		return false, "session record is terminal"
	}
	return true, ""
}

func saveResumeSession(t *testing.T, dir, id, name, provider string) *subagent.FileSessionStore {
	t.Helper()
	store, err := subagent.NewSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: id, Name: name, Provider: provider, Model: "model", Tier: "low", Status: "completed", ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestAppendExternalAgentSessionsDeduplicatesManagedProviderIdentity(t *testing.T) {
	managed := &subagent.Session{ID: "managed", ProviderSessionID: "provider-id", Provider: "codex"}
	listed := []*subagent.Session{managed}
	external := []*subagent.Session{
		{ID: "provider-id", ProviderSessionID: "provider-id", Provider: "codex", HarnessType: "external"},
		{ID: "other-id", ProviderSessionID: "other-id", Provider: "codex", HarnessType: "external"},
	}
	got := appendExternalAgentSessions(listed, []*subagent.Session{managed}, external, nil)
	if len(got) != 2 || got[1].ID != "other-id" {
		t.Fatalf("merged sessions = %#v, want managed plus nonduplicate external", got)
	}
}

func TestAppendExternalAgentSessionsDoesNotRediscoverDeletedProviderSession(t *testing.T) {
	deleted := &subagent.Session{ID: "archived", ProviderSessionID: "provider-id", Provider: "codex"}
	external := []*subagent.Session{{ID: "provider-id", ProviderSessionID: "provider-id", Provider: "codex", HarnessType: "external"}}
	got := appendExternalAgentSessions(nil, nil, external, []*subagent.Session{deleted})
	if len(got) != 0 {
		t.Fatalf("rediscovered archived session: %#v", got)
	}
}

func TestAgentListIncludesExternalSessionsOnlyInGlobalViews(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(agentSessionEnv, "host-a")
	t.Setenv("AGY_CONVERSATION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("ANTIGRAVITY_CONVERSATION_ID", "")
	t.Setenv("ANTIGRAVITY_SESSION_ID", "")

	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "managed-id", Name: "managed", Provider: "codex", ProviderSessionID: "managed-provider-id", ParentSessionID: "host-a", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	rollout := filepath.Join(home, ".codex/sessions/2026/10/02/rollout-external.jsonl")
	if err := os.MkdirAll(filepath.Dir(rollout), 0o700); err != nil {
		t.Fatal(err)
	}
	metadata := `{"type":"session_meta","payload":{"id":"external-session-id","session_id":"external-session-id","cwd":"/work/external-project","model_provider":"openai","source":"cli","timestamp":"2026-10-02T10:00:00Z"}}` + "\n"
	if err := os.WriteFile(rollout, []byte(metadata), 0o600); err != nil {
		t.Fatal(err)
	}

	var scoped bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&scoped)
	cmd.SetArgs([]string{"list", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(scoped.String(), "managed") || strings.Contains(scoped.String(), "external-session-id") {
		t.Fatalf("host-scoped list = %q, want only its managed agents", scoped.String())
	}

	var global bytes.Buffer
	cmd = newAgentCmd()
	cmd.SetOut(&global)
	cmd.SetArgs([]string{"list", "--all-sessions", "--dead", "--json", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var sessions []subagent.Session
	if err := json.Unmarshal(global.Bytes(), &sessions); err != nil {
		t.Fatalf("decode global list: %v; output=%s", err, global.String())
	}
	if len(sessions) != 2 {
		t.Fatalf("global list contains %d entries, want managed and external: %s", len(sessions), global.String())
	}
	var table bytes.Buffer
	cmd = newAgentCmd()
	cmd.SetOut(&table)
	cmd.SetArgs([]string{"list", "--all-sessions", "--dead", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(table.String(), "SOURCE\tWORKING_DIR") || !strings.Contains(table.String(), "cli\t/work/external-project") {
		t.Fatalf("global table does not expose external project metadata: %q", table.String())
	}
	for _, session := range sessions {
		if session.ID == "external-session-id" {
			if session.Provider != "codex" || session.Status != "available" || session.WorkingDir != "/work/external-project" || session.Source != "cli" {
				t.Errorf("external metadata = %#v", session)
			}
			return
		}
	}
	t.Fatalf("external session missing from global list: %s", global.String())
}

func TestAgentListDefaultsToActiveAndHasDeadArchivedFilters(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, sess := range []*subagent.Session{
		{ID: "live", Name: "live", Provider: "fake", Status: "running"},
		{ID: "done", Name: "done", Provider: "fake", Status: "completed"},
		{ID: "old", Name: "old", Provider: "fake", Status: "completed"},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Delete("old"); err != nil {
		t.Fatal(err)
	}
	list := func(args ...string) string {
		t.Helper()
		var out bytes.Buffer
		cmd := newAgentCmd()
		cmd.SetOut(&out)
		cmd.SetArgs(append(append([]string{"list", "--all-sessions"}, args...), "--store-dir", storeDir))
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	if got := list(); !strings.Contains(got, "live") || strings.Contains(got, "done") || strings.Contains(got, "old") {
		t.Fatalf("default list = %q, want active only", got)
	}
	if got := list("--dead"); !strings.Contains(got, "live") || !strings.Contains(got, "done") || strings.Contains(got, "old") {
		t.Fatalf("--dead list = %q, want active and inactive non-archived", got)
	}
	if got := list("--archived"); !strings.Contains(got, "live") || strings.Contains(got, "done") || !strings.Contains(got, "old") {
		t.Fatalf("--archived list = %q, want active and archived", got)
	}
	if got := list("--dead", "--archived"); !strings.Contains(got, "live") || !strings.Contains(got, "done") || !strings.Contains(got, "old") {
		t.Fatalf("--dead --archived list = %q, want all statuses and archive", got)
	}
}

func TestAgentListGlobalAndChildrenViews(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(agentSessionEnv, "")
	t.Setenv("AGY_CONVERSATION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("ANTIGRAVITY_CONVERSATION_ID", "")
	t.Setenv("ANTIGRAVITY_SESSION_ID", "")

	rollout := filepath.Join(home, ".codex/sessions/2026/10/02/rollout-external.jsonl")
	if err := os.MkdirAll(filepath.Dir(rollout), 0o700); err != nil {
		t.Fatal(err)
	}
	metadata := `{"type":"session_meta","payload":{"id":"external-session-id","session_id":"external-session-id","cwd":"/work/external-project","model_provider":"openai","source":"cli","timestamp":"2026-10-02T10:00:00Z"}}` + "\n"
	if err := os.WriteFile(rollout, []byte(metadata), 0o600); err != nil {
		t.Fatal(err)
	}
	storeDir := t.TempDir()

	for _, args := range [][]string{{"list", "--dead"}, {"list", "--children", "--dead"}} {
		var out bytes.Buffer
		cmd := newAgentCmd()
		cmd.SetOut(&out)
		cmd.SetArgs(append(args, "--store-dir", storeDir))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute %v: %v", args, err)
		}
		wantExternal := len(args) == 2
		if gotExternal := strings.Contains(out.String(), "external-session-id"); gotExternal != wantExternal {
			t.Errorf("list %v includes external session = %v, want %v: %q", args[1:], gotExternal, wantExternal, out.String())
		}
	}

	cmd := newAgentCmd()
	cmd.SetArgs([]string{"list", "--all-sessions", "--children", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("conflicting list flags error = %v, want explicit incompatibility", err)
	}
}

func TestAgentListThenResumeByDisplayedIDAndName(t *testing.T) {
	for _, identifier := range []string{"sid", "worker"} {
		t.Run(identifier, func(t *testing.T) {
			old := agentDriver
			d := &resumeOutcomeDriver{}
			agentDriver = func(subagent.Model, string) subagent.Driver { return d }
			defer func() { agentDriver = old }()
			storeDir := t.TempDir()
			saveResumeSession(t, storeDir, "sid", "worker", "fake")
			var listed bytes.Buffer
			cmd := newAgentCmd()
			cmd.SetOut(&listed)
			cmd.SetArgs([]string{"list", "--all-sessions", "--dead", "--store-dir", storeDir})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(listed.String(), "sid\tworker") {
				t.Fatalf("list = %q", listed.String())
			}
			cmd = newAgentCmd()
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs([]string{"resume", "--name", identifier, "hi", "--store-dir", storeDir})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAgentResumeFailureRecordedAndCleared(t *testing.T) {
	old := agentDriver
	d := &resumeOutcomeDriver{err: errors.New("provider failed\nwith detail")}
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store := saveResumeSession(t, storeDir, "sid", "worker", "fake")
	for want := 1; want <= 2; want++ {
		cmd := newAgentCmd()
		cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
		if err := cmd.Execute(); err == nil {
			t.Fatal("resume unexpectedly succeeded")
		}
		sess, _ := store.Get("sid")
		if sess.ResumeFailures != want || sess.LastError != "provider failed" || sess.Status != "failed" || sess.ProcessPID != 0 {
			t.Fatalf("session = %#v", sess)
		}
		if len([]rune(sess.LastError)) > 300 {
			t.Fatal("last error too long")
		}
	}
	d.err = nil
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	sess, _ := store.Get("sid")
	if sess.LastError != "" || sess.ResumeFailures != 0 {
		t.Fatalf("success did not clear failure: %#v", sess)
	}
}

func TestAgentListResumeColumnStates(t *testing.T) {
	old := agentDriver
	agentDriver = func(m subagent.Model, _ string) subagent.Driver {
		d := &resumeOutcomeDriver{}
		if m.Provider == "terminal" {
			d.terminal = true
		}
		return d
	}
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	for _, sess := range []*subagent.Session{
		{ID: "ok", Name: "okay", Provider: "fake", Model: "model", Status: "completed"},
		{ID: "bad", Name: "failed", Provider: "fake", Model: "model", LastError: "provider failed", Status: "completed"},
		{ID: "term", Name: "terminal", Provider: "terminal", Model: "model", Status: "completed"},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list", "--all-sessions", "--dead", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"okay\tfake\tcompleted\tok", "failed\tfake\tcompleted\tfailed", "terminal\tterminal\tcompleted\tterminal"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("list = %q, missing %q", out.String(), want)
		}
	}
}

func TestAgentTerminalResumeRefusal(t *testing.T) {
	old := agentDriver
	d := &resumeOutcomeDriver{terminal: true}
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store := saveResumeSession(t, storeDir, "sid", "worker", "fake")
	cmd := newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot be resumed") || !strings.Contains(err.Error(), "harnez agent start --name") {
		t.Fatalf("error = %v", err)
	}
	sess, _ := store.Get("sid")
	if d.resumes != 0 || sess.ResumeFailures != 0 {
		t.Fatalf("terminal attempt changed state: resumes=%d session=%#v", d.resumes, sess)
	}
}

func TestCodexResumeQuotaRecoveryQuarantinesAndExcludesSession(t *testing.T) {
	t.Setenv(agentRoleEnv, "")
	t.Setenv(agentSessionEnv, "")
	old := agentDriver
	d := &resumeOutcomeDriver{err: errors.New("usage limit reached")}
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store := saveResumeSession(t, storeDir, "sid", "worker", "codex")
	availability := usage.ProviderQuotaAvailability{State: "exhausted"}
	deps := agentDeps{
		store:  func() (*subagent.FileSessionStore, error) { return store, nil },
		parent: func() string { return "" },
		find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
			return s.Find(id)
		},
		availability: func(string, string) usage.ProviderQuotaAvailability { return availability },
	}
	cmd := newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
	if err := runResume(cmd, deps, resumeRequest{Name: "worker", Prompt: "hi", StreamMode: streamFull}); err == nil {
		t.Fatal("expected exhausted quota failure")
	}
	availability = usage.ProviderQuotaAvailability{State: "available"}
	cmd = newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
	if err := runResume(cmd, deps, resumeRequest{Name: "worker", Prompt: "hi", StreamMode: streamFull}); err == nil {
		t.Fatal("expected quarantine failure")
	}
	sess, _ := store.Get("sid")
	if sess.CodexQuarantine == nil || sess.CodexQuarantine.Attempts != 1 {
		t.Fatalf("quarantine = %#v", sess.CodexQuarantine)
	}
	if got := attributable([]*subagent.Session{sess}, ".", ""); len(got) != 0 {
		t.Fatalf("quarantined session selected: %#v", got)
	}
	cmd = newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
	err := runResume(cmd, deps, resumeRequest{Name: "worker", Prompt: "hi", StreamMode: streamFull})
	if err == nil || !strings.Contains(err.Error(), "harnez agent start") || d.resumes != 2 {
		t.Fatalf("repeat resume err=%v resumes=%d", err, d.resumes)
	}
}

func TestCodexTransientRateLimitDoesNotQuarantine(t *testing.T) {
	t.Setenv(agentRoleEnv, "")
	t.Setenv(agentSessionEnv, "")
	old := agentDriver
	d := &resumeOutcomeDriver{err: errors.New("429 rate limit exceeded")}
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store := saveResumeSession(t, storeDir, "sid", "worker", "codex")
	deps := agentDeps{
		store:  func() (*subagent.FileSessionStore, error) { return store, nil },
		parent: func() string { return "" },
		find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
			return s.Find(id)
		},
		availability: func(string, string) usage.ProviderQuotaAvailability {
			return usage.ProviderQuotaAvailability{State: "available"}
		},
	}
	for range 2 {
		cmd := newAgentCmd()
		if err := runResume(cmd, deps, resumeRequest{Name: "worker", Prompt: "hi", StreamMode: streamFull}); err == nil {
			t.Fatal("expected rate-limit failure")
		}
	}
	sess, _ := store.Get("sid")
	if sess.CodexQuarantine != nil || sess.CodexQuotaResumePending || sess.ResumeFailures != 2 || d.resumes != 2 {
		t.Fatalf("transient errors changed quarantine state: session=%#v resumes=%d", sess, d.resumes)
	}
}

func TestAgentStatusResumeDiagnostics(t *testing.T) {
	storeDir := t.TempDir()
	store := saveResumeSession(t, storeDir, "sid", "worker", "fake")
	sess, _ := store.Get("sid")
	sess.LastError = "provider failed"
	sess.ResumeFailures = 2
	_ = store.Save(sess)
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "--name", "worker", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Last error: provider failed") || !strings.Contains(out.String(), "Resume failures: 2") {
		t.Fatalf("status = %q", out.String())
	}
	sess.LastError = ""
	_ = store.Save(sess)
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "--name", "worker", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "Last error:") || strings.Contains(out.String(), "Resume failures:") {
		t.Fatalf("clean status = %q", out.String())
	}
}

func TestAgentResumeMissingIdentifier(t *testing.T) {
	cmd := newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "nope", "hi", "--store-dir", t.TempDir()})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestAttributableOrderingAndFilters(t *testing.T) {
	old := agentDriver
	agentDriver = func(m subagent.Model, _ string) subagent.Driver {
		d := &resumeOutcomeDriver{}
		d.terminal = m.Provider == "terminal"
		return d
	}
	defer func() { agentDriver = old }()
	now := time.Now()
	sessions := []*subagent.Session{
		{ID: "old", Name: "old", Provider: "fake", WorkingDir: ".", Status: "completed", LastActiveAt: now.Add(-time.Hour)},
		{ID: "new", Name: "new", Provider: "fake", WorkingDir: ".", Status: "completed", LastActiveAt: now},
		{ID: "other-dir", Name: "other-dir", Provider: "fake", WorkingDir: "/else", Status: "completed", LastActiveAt: now.Add(time.Hour)},
		{ID: "other-caller", Name: "other-caller", Provider: "fake", WorkingDir: ".", ParentSessionID: "else", Status: "completed", LastActiveAt: now.Add(time.Hour)},
		{ID: "terminal", Name: "terminal", Provider: "terminal", WorkingDir: ".", Status: "completed", LastActiveAt: now.Add(time.Hour)},
	}
	for _, sess := range sessions[:2] {
		sess.ParentSessionID = "caller"
	}
	got := attributable(sessions, ".", "caller")
	if len(got) != 2 || got[0].Name != "new" || got[1].Name != "old" {
		t.Fatalf("attributable = %#v", got)
	}
}

func TestAgentResumeAttributionAndContinue(t *testing.T) {
	old := agentDriver
	d := &resumeOutcomeDriver{}
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	now := time.Now()
	for _, sess := range []*subagent.Session{{ID: "one", Name: "one", Provider: "fake", Model: "model", WorkingDir: ".", Status: "completed", ContextTokens: 100, LastActiveAt: now.Add(-time.Hour)}, {ID: "two", Name: "two", Provider: "fake", Model: "model", WorkingDir: ".", Status: "completed", ContextTokens: 100, LastActiveAt: now}} {
		_ = store.Save(sess)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "--continue", "hi", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if d.resumes != 1 {
		t.Fatalf("resumes=%d", d.resumes)
	}
	cmd = newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "one", "--continue", "hi", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("error=%v", err)
	}
}

func (d *recordingAgentDriver) Run(_ context.Context, opts subagent.RunOptions) (*subagent.TurnResult, error) {
	if d.runHook != nil {
		d.runHook()
	}
	d.dir = opts.Dir
	d.prompt = opts.Prompt
	return &subagent.TurnResult{SessionID: "recorded", Response: "ok"}, nil
}
func (d *recordingAgentDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{}, nil
}
func (d *recordingAgentDriver) Compact(context.Context, string) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{Response: "Context compacted.", InputTokens: 1000, ContextTokens: 1000}, nil
}
func (d *recordingAgentDriver) Stop(_ context.Context, id string) error {
	d.stopped = append(d.stopped, id)
	return nil
}
func (d *recordingAgentDriver) Delete(_ context.Context, id string) error {
	d.deleted = append(d.deleted, id)
	return nil
}

func TestAgentStopAndDeleteAllManageableSessions(t *testing.T) {
	storeDir := t.TempDir()
	oldSessionID := os.Getenv("HARNEZ_SESSION_ID")
	if err := os.Setenv("HARNEZ_SESSION_ID", "caller"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HARNEZ_SESSION_ID", oldSessionID) }()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, sess := range []*subagent.Session{
		{ID: "one", Name: "one", Provider: "codex", Model: "model", Status: "completed", HarnessType: "batch", ParentSessionID: "caller", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1, Rating: testRatingPtr(4)}}},
		{ID: "two", Name: "two", Provider: "codex", Model: "model", Status: "completed", HarnessType: "batch", ParentSessionID: "foreign"},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	old := agentDriver
	recorder := &recordingAgentDriver{}
	agentDriver = func(subagent.Model, string) subagent.Driver { return recorder }
	defer func() { agentDriver = old }()

	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"stop", "--all", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(recorder.stopped) != 1 || recorder.stopped[0] != "one" {
		t.Fatalf("stopped sessions = %#v", recorder.stopped)
	}

	cmd = newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "--all", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(recorder.deleted) != 1 || recorder.deleted[0] != "one" {
		t.Fatalf("deleted sessions = %#v", recorder.deleted)
	}
	if _, err := store.Get("one"); err == nil {
		t.Fatal("manageable session was not deleted")
	}
	if _, err := store.Get("two"); err != nil {
		t.Fatalf("foreign session was deleted: %v", err)
	}
}

func TestAgentDeleteAllSkipsUnratedSessionWithWarning(t *testing.T) {
	storeDir := t.TempDir()
	oldSessionID := os.Getenv("HARNEZ_SESSION_ID")
	if err := os.Setenv("HARNEZ_SESSION_ID", "caller"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HARNEZ_SESSION_ID", oldSessionID) }()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "unrated", Name: "needs-rating", Provider: "agy", Model: "gemini", Status: "completed", HarnessType: "interactive", ParentSessionID: "caller", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1}}}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"delete", "--all", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete --all error = %v", err)
	}
	if got := stderr.String(); !strings.Contains(got, "warning: unrated latest turns:") || !strings.Contains(got, "needs-rating: harnez agent rate --name needs-rating <1-5> \"<reason>\"") {
		t.Fatalf("warning = %q", got)
	}
	if _, err := store.Get(sess.ID); err != nil {
		t.Fatalf("unrated session was deleted: %v", err)
	}
}

func TestAgentDeleteAllRepeatSafeAndRetryFailed(t *testing.T) {
	storeDir := t.TempDir()
	oldSessionID := os.Getenv("HARNEZ_SESSION_ID")
	if err := os.Setenv("HARNEZ_SESSION_ID", "caller"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HARNEZ_SESSION_ID", oldSessionID) }()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	failed := &subagent.Session{ID: "failed", Name: "failed-session", Provider: "agy", Model: "model", Status: "completed", HarnessType: "batch", ParentSessionID: "caller"}
	if err := store.Save(failed); err != nil {
		t.Fatal(err)
	}
	old := agentDriver
	shouldFail := true
	var deleted []string
	agentDriver = func(subagent.Model, string) subagent.Driver {
		return mockAgentDeleteDriver{delete: func(id string) error {
			deleted = append(deleted, id)
			if id == "failed" && shouldFail {
				return errors.New("provider delete failed")
			}
			return nil
		}}
	}
	defer func() { agentDriver = old }()
	run := func(args ...string) (string, string, error) {
		var out, stderr bytes.Buffer
		cmd := newAgentCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		cmd.SetArgs(append(args, "--store-dir", storeDir))
		err := cmd.Execute()
		return out.String(), stderr.String(), err
	}
	_, _, err = run("delete", "--all", "--force")
	if err == nil {
		t.Fatal("first delete-all should expose the provider failure")
	}
	got, err := store.Get("failed")
	if err != nil || got.Status != "delete-failed" || !strings.Contains(got.LastError, "provider delete failed") {
		t.Fatalf("failed record = %#v, %v", got, err)
	}
	listOut, _, err := run("list")
	if err != nil || strings.Contains(listOut, "failed-session") {
		t.Fatalf("default list = %q, err %v", listOut, err)
	}
	listOut, _, err = run("list", "--failed")
	if err != nil || !strings.Contains(listOut, "failed-session") || !strings.Contains(listOut, "delete-failed") {
		t.Fatalf("failed list = %q, err %v", listOut, err)
	}
	_, _, err = run("delete", "--all", "--force")
	if err != nil {
		t.Fatalf("second delete-all: %v", err)
	}
	shouldFail = false
	_, _, err = run("delete", "--retry-failed")
	if err != nil {
		t.Fatalf("retry-failed: %v", err)
	}
	if _, err := store.Get("failed"); err == nil {
		t.Fatal("successful retry retained failed record")
	}
	if len(deleted) != 2 {
		t.Fatalf("provider deletes = %#v", deleted)
	}
}

func TestAgentDeleteAllRetriesCodexDeleteFailedRecordWithMissingRollout(t *testing.T) {
	storeDir := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("HARNEZ_SESSION_ID", "caller")
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "missing-rollout", Name: "never-started", Provider: "codex", Model: "model", Status: "delete-failed", LastError: "unknown session", HarnessType: "batch", ParentSessionID: "caller"}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "--all", "--force", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete --all: %v", err)
	}
	if _, err := store.Get(sess.ID); err == nil {
		t.Fatal("missing-rollout failed record was not cleared")
	}
}

func TestAgentDeleteAllForceSuppressesWarningAndDiscoversNewSession(t *testing.T) {
	storeDir := t.TempDir()
	oldSessionID := os.Getenv("HARNEZ_SESSION_ID")
	if err := os.Setenv("HARNEZ_SESSION_ID", "caller"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HARNEZ_SESSION_ID", oldSessionID) }()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	first := &subagent.Session{ID: "first", Name: "first-session", Provider: "claude", Model: "model", Status: "completed", HarnessType: "interactive", ParentSessionID: "caller", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1}}}
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	run := func(force bool) (string, string, error) {
		var out, stderr bytes.Buffer
		cmd := newAgentCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		args := []string{"delete", "--all"}
		if force {
			args = append(args, "--force")
		}
		cmd.SetArgs(append(args, "--store-dir", storeDir))
		err := cmd.Execute()
		return out.String(), stderr.String(), err
	}
	_, stderr, err := run(false)
	if err != nil || !strings.Contains(stderr, "unrated latest turns") {
		t.Fatalf("without force stderr=%q err=%v", stderr, err)
	}
	_, stderr, err = run(true)
	if err != nil || strings.Contains(stderr, "unrated latest turns") {
		t.Fatalf("with force stderr=%q err=%v", stderr, err)
	}
	if err := store.Save(&subagent.Session{ID: "new", Name: "new-session", Provider: "claude", Model: "model", Status: "completed", HarnessType: "interactive", ParentSessionID: "caller", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1, Rating: testRatingPtr(4)}}}); err != nil {
		t.Fatal(err)
	}
	out, _, err := run(true)
	if err != nil || !strings.Contains(out, "new-session") {
		t.Fatalf("new session output=%q err=%v", out, err)
	}
}

type mockAgentDeleteDriver struct{ delete func(string) error }

func (d mockAgentDeleteDriver) Run(context.Context, subagent.RunOptions) (*subagent.TurnResult, error) {
	return nil, nil
}
func (d mockAgentDeleteDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	return nil, nil
}
func (d mockAgentDeleteDriver) Compact(context.Context, string) (*subagent.TurnResult, error) {
	return nil, nil
}
func (d mockAgentDeleteDriver) Stop(context.Context, string) error        { return nil }
func (d mockAgentDeleteDriver) Delete(_ context.Context, id string) error { return d.delete(id) }

func TestAgentStopAndDeleteAllWithNoSessions(t *testing.T) {
	storeDir := t.TempDir()
	for _, args := range [][]string{{"stop", "--all"}, {"delete", "--all"}} {
		cmd := newAgentCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs(append(args, "--store-dir", storeDir))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

func TestAgentDeleteAllCompleted(t *testing.T) {
	storeDir := t.TempDir()
	oldSessionID := os.Getenv("HARNEZ_SESSION_ID")
	if err := os.Setenv("HARNEZ_SESSION_ID", "caller"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HARNEZ_SESSION_ID", oldSessionID) }()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, sess := range []*subagent.Session{
		{ID: "completed1", Name: "done1", Provider: "codex", Model: "model", Status: "completed", HarnessType: "batch", ParentSessionID: "caller", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1, Rating: testRatingPtr(4)}}},
		{ID: "completed2", Name: "done2", Provider: "codex", Model: "model", Status: "completed", HarnessType: "batch", ParentSessionID: "caller", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1, Rating: testRatingPtr(4)}}},
		{ID: "running", Name: "active", Provider: "codex", Model: "model", Status: "active", HarnessType: "batch", ParentSessionID: "caller"},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	old := agentDriver
	recorder := &recordingAgentDriver{}
	agentDriver = func(subagent.Model, string) subagent.Driver { return recorder }
	defer func() { agentDriver = old }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"delete", "--all-completed", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !strings.Contains(output, "done1") || !strings.Contains(output, "done2") || strings.Contains(output, "active") {
		t.Fatalf("output = %q, want done1 and done2, not active", output)
	}
	if len(recorder.deleted) != 2 {
		t.Fatalf("deleted sessions = %#v, want 2", recorder.deleted)
	}
	if _, err := store.Get("completed1"); err == nil {
		t.Fatal("completed1 was not deleted")
	}
	if _, err := store.Get("completed2"); err == nil {
		t.Fatal("completed2 was not deleted")
	}
	if _, err := store.Get("running"); err != nil {
		t.Fatalf("active session was deleted: %v", err)
	}
}

func TestAgentDeleteAllCompletedRejectsFlagConflicts(t *testing.T) {
	storeDir := t.TempDir()
	for _, args := range [][]string{
		{"delete", "--all-completed", "--all"},
		{"delete", "--all-completed", "--name", "session"},
		{"delete", "--all-completed", "somename"},
	} {
		cmd := newAgentCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs(append(args, "--store-dir", storeDir))
		if err := cmd.Execute(); err == nil {
			t.Fatalf("%v: err = %v, want error", args, err)
		}
	}
}

func TestAgentStartStoresCanonicalWorkingDir(t *testing.T) {
	old := agentDriver
	recorder := &recordingAgentDriver{}
	agentDriver = func(subagent.Model, string) subagent.Driver { return recorder }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "prompt", "-d", ".", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(".")
	if err != nil || recorder.dir != want {
		t.Fatalf("driver working directory = %q, want %q", recorder.dir, want)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Get("recorded")
	if err != nil || sess.WorkingDir != want {
		t.Fatalf("saved working directory = %q, err=%v", sess.WorkingDir, err)
	}
}

func TestAgentHaikuAliases(t *testing.T) {
	for _, spec := range []string{"claude:haiku", "claude:haiku:latest"} {
		m, err := subagent.ResolveModel(spec)
		if err != nil || m.Provider != "claude" || m.Name != "haiku" || m.Tier != "low" {
			t.Fatalf("%s resolved to %#v (%v)", spec, m, err)
		}
	}
}

func TestAgentResumePrintsReplyNotStructDump(t *testing.T) {
	// The reattach notice is asserted below; an inherited HTO=0 would suppress it.
	t.Setenv(execTimeoutShortEnv, "")
	t.Setenv(execTimeoutEnv, "")
	t.Setenv(execTimeoutExplicitEnv, "")
	// The agent command runs sessionTipHook; isolate HOME and the session
	// env so the caller's own session tips cannot leak into stderr.
	t.Setenv("HOME", t.TempDir())
	for _, name := range resolve.SessionEnvVars {
		t.Setenv(name, "")
	}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return &replyDriver{} }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", Status: "completed", ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"resume", "--name", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); strings.Contains(got, "{0x") || !strings.Contains(got, "[agent messages]\n[msg 1]\nthe reply text") {
		t.Fatalf("stdout = %q, want plain reply without struct dump", got)
	}
	if e := errOut.String(); !strings.HasPrefix(e, "[session timeline]\n") || !strings.Contains(e, "host background job") || !strings.Contains(e, "Do NOT poll") || !strings.Contains(e, " done] ") {
		t.Fatalf("stderr = %q, want timeline with wait notice", e)
	}
}

type replyDriver struct{ recordingAgentDriver }

func (*replyDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{Response: "the reply text"}, nil
}

type ackDriver struct{ recordingAgentDriver }

func (*ackDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{Response: "real reply", Messages: []string{"Context compacted.", "real reply"}, TokensTurn: 900000, CachedTokens: 890000}, nil
}

func TestAgentResumeCompactsOnceAndSeparatesAck(t *testing.T) {
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return &ackDriver{} }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "claude", Model: "luna", Status: "completed", TokensCumulative: 7000000, TokensSinceCompact: 250000, ContextTokens: 250000}); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"resume", "--name", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "[agent messages]\n[msg 1]\nreal reply\n" {
		t.Fatalf("stdout = %q", got)
	}
	if e := errOut.String(); !strings.Contains(e, " compact] verified /compact") || !strings.Contains(e, "agent acknowledged: Context compacted.") {
		t.Fatalf("stderr = %q", e)
	}
	sess, err := store.Get("sid")
	if err != nil || sess.TokensSinceCompact != 10000 || sess.TokensCumulative != 7900000 {
		t.Fatalf("since=%d cumulative=%d err=%v", sess.TokensSinceCompact, sess.TokensCumulative, err)
	}
	if subagent.ShouldCompact(sess.TokensSinceCompact) {
		t.Fatal("next resume must not compact again")
	}
}

type codexAutoCompactDriver struct {
	recordingAgentDriver
	compactCalls int
	resumeCalls  int
	result       subagent.TurnResult
}

func (d *codexAutoCompactDriver) Compact(context.Context, string) (*subagent.TurnResult, error) {
	d.compactCalls++
	return nil, errors.New("manual compact must not be called")
}

func (d *codexAutoCompactDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	d.resumeCalls++
	r := d.result
	r.Response = "resumed"
	r.TokensTurn = 5
	return &r, nil
}

func TestCodexOverThresholdResumeUsesProviderAutoCompact(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".harnez"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".harnez", "config.yaml"), []byte("agent:\n  compact_threshold_tokens: 100000\n"), 0600); err != nil {
		t.Fatal(err)
	}
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", Status: "completed", ContextTokens: 250000}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		result      subagent.TurnResult
		wantErr     bool
		wantContext string
	}{
		{name: "compaction verified", result: subagent.TurnResult{ContextTokens: 50000, CompactionObserved: true}},
		{name: "missing compaction record", result: subagent.TurnResult{ContextTokens: 50000}, wantErr: true, wantContext: "context 50000"},
		{name: "still over limit", result: subagent.TurnResult{ContextTokens: 120000, CompactionObserved: true}, wantErr: true, wantContext: "context 120000"},
		{name: "unreadable rollout", result: subagent.TurnResult{ContextTokens: -1, CompactionObserved: true}, wantErr: true, wantContext: "context unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", home)
			dir := t.TempDir()
			st, err := subagent.NewSessionStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", Status: "completed", ContextTokens: 250000}); err != nil {
				t.Fatal(err)
			}
			driver := &codexAutoCompactDriver{result: tc.result}
			old := agentDriver
			agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
			defer func() { agentDriver = old }()
			_, err = runWithStore(t, driver, dir, "resume", "--name", "worker", "prompt")
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				if !strings.Contains(err.Error(), "Codex did not auto-compact session sid") || !strings.Contains(err.Error(), tc.wantContext) || !strings.Contains(err.Error(), "start a fresh session with harnez agent start") {
					t.Fatalf("error = %v", err)
				}
				stopped, getErr := st.Get("sid")
				if getErr != nil || stopped.ResumeBlockedReason == "" {
					t.Fatalf("resume block=%q err=%v", stopped.ResumeBlockedReason, getErr)
				}
				if _, nextErr := runWithStore(t, driver, dir, "resume", "--name", "worker", "again"); nextErr == nil || !strings.Contains(nextErr.Error(), "cannot be resumed") {
					t.Fatalf("second resume error = %v", nextErr)
				}
			}
			if driver.compactCalls != 0 || driver.resumeCalls != 1 {
				t.Fatalf("compact calls=%d resume calls=%d", driver.compactCalls, driver.resumeCalls)
			}
		})
	}
}

type streamDriver struct{ recordingAgentDriver }

func (*streamDriver) emit(fn subagent.EventFunc) *subagent.TurnResult {
	fn(subagent.Event{Kind: "session", Text: "thread-1", Bytes: 40})
	fn(subagent.Event{Kind: "message", Text: "on it", Bytes: 80})
	fn(subagent.Event{Kind: "activity", Text: "running sleep", Bytes: 80})
	time.Sleep(60 * time.Millisecond)
	fn(subagent.Event{Kind: "message", Text: "all done", Bytes: 80})
	return &subagent.TurnResult{SessionID: "thread-1", Response: "all done", Messages: []string{"on it", "all done"}, TokensTurn: 500, ContextTokens: 100000, CompactionObserved: true}
}
func (d *streamDriver) RunStream(_ context.Context, _ subagent.RunOptions, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	return d.emit(fn), nil
}
func (d *streamDriver) ResumeStream(_ context.Context, _, _ string, _ subagent.Model, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	return d.emit(fn), nil
}

func TestAgentStartStreamsLabeledBlocks(t *testing.T) {
	old, oldSched, oldRepeat := agentDriver, heartbeatSchedule, heartbeatRepeat
	agentDriver = func(subagent.Model, string) subagent.Driver { return &streamDriver{} }
	heartbeatSchedule, heartbeatRepeat = []time.Duration{20 * time.Millisecond}, 20*time.Millisecond
	defer func() { agentDriver, heartbeatSchedule, heartbeatRepeat = old, oldSched, oldRepeat }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "prompt", "--name", "w", "--store-dir", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	order := []string{"[session info: id=thread-1 agent=codex:gpt-6-luna action=start resolved=new]", "name=w", "reconnect: harnez agent resume thread-1", "[wait:", "[message: 0s]\non it", "[heartbeat: 0s, ~", "last: running sleep]", "[message: 0s]\nall done", "[done: 2 messages, last message is the reply"}
	pos := 0
	for _, want := range order {
		i := strings.Index(got[pos:], want)
		if i < 0 {
			t.Fatalf("missing %q (in order) in:\n%s", want, got)
		}
		pos += i + len(want)
	}
	if strings.Contains(got, "[agent messages]") || strings.Contains(got, "[msg ") {
		t.Fatalf("old labels present:\n%s", got)
	}
}

func TestAgentResumeStreamsCompactionAckLabel(t *testing.T) {
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return &streamDriver{} }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "claude", Model: "luna", Status: "completed", TokensSinceCompact: 250000, ContextTokens: 250000}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "--name", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "[session info: id=sid agent=claude:luna action=resume resolved=name]\n[wait: ") || !strings.Contains(got, "[compact: verified /compact at 250.0k context tokens") || !strings.Contains(got, "[compaction ack: 0s]\non it") || !strings.Contains(got, "[message: 0s]\nall done") {
		t.Fatalf("stdout:\n%s", got)
	}
}

func TestHeartbeatSchedule(t *testing.T) {
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 6 * time.Minute, 10 * time.Minute, 15 * time.Minute, 20 * time.Minute}
	for i, w := range want {
		if got := heartbeatAt(i); got != w {
			t.Fatalf("heartbeatAt(%d) = %s, want %s", i, got, w)
		}
	}
}

func TestAgentStartStatsModeShowsOnlyHeartbeatsAndReply(t *testing.T) {
	old, oldSched, oldRepeat := agentDriver, heartbeatSchedule, heartbeatRepeat
	agentDriver = func(subagent.Model, string) subagent.Driver { return &streamDriver{} }
	heartbeatSchedule, heartbeatRepeat = []time.Duration{20 * time.Millisecond}, 20*time.Millisecond
	defer func() { agentDriver, heartbeatSchedule, heartbeatRepeat = old, oldSched, oldRepeat }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "prompt", "--stream", "stats", "--store-dir", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "on it") || strings.Contains(got, "[message:") {
		t.Fatalf("intermediate message leaked in stats mode:\n%s", got)
	}
	for _, want := range []string{"[heartbeat: ", "1 messages, 1 commands", "[reply: 0s]\nall done\n", "[done: 2 messages (only the reply is shown)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestAgentRejectsUnknownStreamMode(t *testing.T) {
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "prompt", "--stream", "loud", "--store-dir", t.TempDir()})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid --stream") {
		t.Fatalf("err = %v", err)
	}
}

func TestShortDur(t *testing.T) {
	for in, want := range map[time.Duration]string{0: "0s", 30 * time.Second: "30s", time.Minute: "1m", 150 * time.Second: "2m30s", time.Hour: "1h", time.Hour + 5*time.Minute: "1h5m"} {
		if got := shortDur(in); got != want {
			t.Fatalf("shortDur(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestTokenSummarySeparatesNewFromCached(t *testing.T) {
	r := &subagent.TurnResult{InputTokens: 1230000, OutputTokens: 1200, CachedTokens: 1198000, TokensTurn: 1231200}
	if got, want := tokenSummary(r), "tokens: 33.2k new (1200 out), 1.2M cached"; got != want {
		t.Fatalf("tokenSummary = %q, want %q", got, want)
	}
	for in, want := range map[int]string{999: "999", 12300: "12.3k", 1230742: "1.2M"} {
		if got := humanCount(in); got != want {
			t.Fatalf("humanCount(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestStreamingResumeKeepsStderrQuiet(t *testing.T) {
	// Isolate the caller's Harnez session state so a pending session tip
	// cannot leak into this command's deliberately quiet stderr.
	t.Setenv("HOME", t.TempDir())
	for _, name := range resolve.SessionEnvVars {
		t.Setenv(name, "")
	}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return &streamDriver{} }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", WorkingDir: t.TempDir(), Status: "completed", TokensSinceCompact: 250000, ContextTokens: 250000}); err != nil {
		t.Fatal(err)
	}
	var errOut bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"resume", "--name", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty while streaming", errOut.String())
	}
}

type unverifiedCompactDriver struct {
	recordingAgentDriver
	resumes int
}

func (d *unverifiedCompactDriver) Compact(context.Context, string) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{Response: "Context compacted.", ContextTokens: 200}, nil
}

func (d *unverifiedCompactDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	d.resumes++
	return &subagent.TurnResult{Response: "should not be sent"}, nil
}

func TestRunResumeBlocksPromptWhenCompactionIsUnverified(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".harnez"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".harnez", "config.yaml"), []byte("agent:\n  compact_threshold_tokens: 100\n"), 0600); err != nil {
		t.Fatal(err)
	}

	for _, interactive := range []bool{false, true} {
		t.Run(map[bool]string{false: "batch", true: "interactive"}[interactive], func(t *testing.T) {
			store, err := subagent.NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			provider := "claude"
			if interactive {
				provider = "codex"
			}
			sess := &subagent.Session{ID: "resume", ProviderSessionID: "provider", Name: "worker", Provider: provider, Model: "model", Status: "completed", ContextTokens: 200}
			if interactive {
				sess.HarnessType, sess.Status = "interactive", "active"
			}
			if err := store.Save(sess); err != nil {
				t.Fatal(err)
			}
			driver := &unverifiedCompactDriver{}
			old := agentDriver
			agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
			defer func() { agentDriver = old }()
			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			deps := agentDeps{
				store:  func() (*subagent.FileSessionStore, error) { return store, nil },
				parent: func() string { return "" },
				find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
					return s.Find(id)
				},
			}
			err = runResume(cmd, deps, resumeRequest{Name: "worker", Prompt: "must not be sent", StreamMode: streamStats, JSON: true})
			if interactive {
				want := `session "worker" has 200 context tokens, over the limit 100 (setting agent.compact_threshold_tokens in ~/.harnez/config.yaml); an active interactive session cannot be compacted and verified from outside; next step: resume it non-interactively, or start a fresh session with harnez agent start`
				if err == nil || err.Error() != want {
					t.Fatalf("resume error = %v, want %q", err, want)
				}
			} else if err == nil || !strings.Contains(err.Error(), "refusing to send resume prompt") {
				t.Fatalf("resume error = %v, want unverified-compaction error", err)
			}
			if !interactive && driver.resumes != 0 {
				t.Fatalf("resume prompt calls = %d, want none", driver.resumes)
			}
		})
	}
}

// step is one scripted event, emitted after wait.
type step struct {
	wait time.Duration
	ev   subagent.Event
}

type scriptDriver struct {
	recordingAgentDriver
	steps   []step
	prompt  string
	sid     string   // session id returned by a turn; "thread-1" when empty
	resumed []string // provider ids passed to ResumeStream
	// environment the provider process would inherit during the turn
	envRole, envSession string
}

func (d *scriptDriver) play(fn subagent.EventFunc) *subagent.TurnResult {
	var msgs []string
	for _, s := range d.steps {
		time.Sleep(s.wait)
		fn(s.ev)
		if s.ev.Kind == "message" {
			msgs = append(msgs, s.ev.Text)
		}
	}
	sid := d.sid
	if sid == "" {
		sid = "thread-1"
	}
	return &subagent.TurnResult{SessionID: sid, Response: msgs[len(msgs)-1], Messages: msgs, ContextTokens: 100}
}
func (d *scriptDriver) RunStream(_ context.Context, o subagent.RunOptions, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	d.prompt = o.Prompt
	d.envRole, d.envSession = os.Getenv(agentRoleEnv), os.Getenv(agentSessionEnv)
	return d.play(fn), nil
}
func (d *scriptDriver) ResumeStream(_ context.Context, id, p string, _ subagent.Model, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	d.prompt = p
	d.envRole, d.envSession = os.Getenv(agentRoleEnv), os.Getenv(agentSessionEnv)
	d.resumed = append(d.resumed, id)
	return d.play(fn), nil
}

func runScripted(t *testing.T, d *scriptDriver, args ...string) string {
	t.Helper()
	old, oldTimeout := agentDriver, confirmTimeout
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	confirmTimeout = 30 * time.Millisecond
	defer func() { agentDriver, confirmTimeout = old, oldTimeout }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(append(args, "--store-dir", t.TempDir()))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func msg(text string) subagent.Event  { return subagent.Event{Kind: "message", Text: text, Bytes: 10} }
func tool(text string) subagent.Event { return subagent.Event{Kind: "activity", Text: text, Bytes: 10} }

func TestConfirmationAndPlanShowInStatsMode(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: will count files")}, {0, tool("running ls")}, {0, msg("PLAN: ls then sort")}, {0, msg("interim chatter")}, {0, msg("9 files")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "orig task", "--name", "w", "--stream", "stats")
	for _, want := range []string{"[confirmation: 0s]\nwill count files\n", "[plan: 0s]\nls then sort\n", "[reply: 0s]\n9 files\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "interim chatter") || strings.Contains(got, "violation") || strings.Contains(got, "warning") {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestToolBeforeConfirmationIsAViolation(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, tool("running rg foo")}, {0, msg("CONFIRM: late")}, {0, msg("done")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "task", "--name", "w")
	want := "[violation: running rg foo before any confirmation; caller may stop the agent: harnez agent stop w]"
	if !strings.Contains(got, want) || strings.Count(got, "[violation:") != 1 {
		t.Fatalf("stdout:\n%s", got)
	}
}

func TestSilentAgentGetsWarning(t *testing.T) {
	d := &scriptDriver{steps: []step{{100 * time.Millisecond, msg("late reply")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "task", "--name", "w")
	if !strings.Contains(got, "[warning: no confirmation after 0s; caller may stop the agent: harnez agent stop w]") || strings.Count(got, "[warning:") != 1 {
		t.Fatalf("stdout:\n%s", got)
	}
}

func TestPromptGetsProtocolButStoredPromptStaysOriginal(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
	storeDir := t.TempDir()
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "orig task", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(d.prompt, "Harnez dispatch protocol:") || !strings.HasSuffix(d.prompt, "\n\norig task") || !strings.Contains(d.prompt, "CONFIRM:") {
		t.Fatalf("driver prompt = %q", d.prompt)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Get("thread-1")
	if err != nil || sess.StartPrompt != "orig task" {
		t.Fatalf("stored prompt = %q, err=%v", sess.StartPrompt, err)
	}
}

func TestPlanFirstAddsGateAndPrompt(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: will plan")}, {0, msg("PLAN: 1) read 2) count")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "count files", "--name", "w", "--plan", "yes")
	if !strings.Contains(d.prompt, "plan-first turn") || !strings.HasSuffix(d.prompt, "\n\ncount files") {
		t.Fatalf("driver prompt = %q", d.prompt)
	}
	for _, want := range []string{"[plan: 0s]\n1) read 2) count\n", `[gate: plan-first turn ended, nothing was executed; to proceed: harnez agent resume w "go ahead"]`, "[done: 2 messages, tokens:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "warning") {
		t.Fatalf("unexpected warning:\n%s", got)
	}
}

func TestPlanFirstWithoutPlanWarns(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("I just did it")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "task", "--name", "w", "--plan", "yes")
	if !strings.Contains(got, "[warning: plan-first turn ended without a PLAN: message; the last message was: I just did it]") {
		t.Fatalf("stdout:\n%s", got)
	}
}

func TestAgentPlanFlagValidationAndCompatibility(t *testing.T) {
	if _, err := runWithStore(t, &scriptDriver{steps: []step{{ev: msg("ok")}}}, t.TempDir(), "start", "--model", "luna", "--plan", "maybe", "task"); err == nil || err.Error() != `invalid --plan "maybe": want "yes" or "no"` {
		t.Fatalf("invalid plan error = %v", err)
	}
	if _, err := runWithStore(t, &scriptDriver{steps: []step{{ev: msg("ok")}}}, t.TempDir(), "start", "--model", "luna", "--plan-first", "task"); err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("old flag error = %v", err)
	}
}

func TestAgentPlanNoEqualsDefault(t *testing.T) {
	for _, args := range [][]string{{"task"}, {"--plan", "no", "task"}} {
		d := &scriptDriver{steps: []step{{ev: msg("ok")}}}
		if _, err := runWithStore(t, d, t.TempDir(), append([]string{"start", "--model", "luna"}, args...)...); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(d.prompt, "plan-first") {
			t.Fatalf("unexpected plan preamble: %q", d.prompt)
		}
	}
}

func TestAgentRootPlanYes(t *testing.T) {
	d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "root"}}, {ev: msg("CONFIRM: ok")}, {ev: msg("PLAN: wait")}}}
	out, err := runWithStore(t, d, t.TempDir(), "--plan", "yes", "-p", "task")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.prompt, "plan-first turn") || !strings.Contains(out, "[gate: plan-first turn ended") {
		t.Fatalf("plan output=%q prompt=%q", out, d.prompt)
	}
}

func TestAgentHelpUsesNewForms(t *testing.T) {
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "--name") || strings.Contains(out.String(), "start codex:") || strings.Contains(out.String(), "resume <session>") {
		t.Fatalf("root help=%q", out.String())
	}
	start, _, err := newAgentCmd().Find([]string{"start"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(start.Example, "--name") || strings.Contains(start.Example, "start codex:") || strings.Contains(start.Example, "resume <session>") {
		t.Fatalf("start example=%q", start.Example)
	}
}

func TestNormalTurnHasNoPlanFirstText(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "task", "--name", "w")
	if strings.Contains(d.prompt, "plan-first") || strings.Contains(got, "[gate:") {
		t.Fatalf("prompt=%q\nstdout:\n%s", d.prompt, got)
	}
}

type blockingResumeDriver struct {
	recordingAgentDriver
	started chan struct{}
	release chan struct{}
}

func (d *blockingResumeDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	close(d.started)
	<-d.release
	return &subagent.TurnResult{SessionID: "provider-session", Response: "done", ContextTokens: 100}, nil
}

func TestAgentResumeStatusIsRunningDuringTurn(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	driver := &blockingResumeDriver{started: make(chan struct{}), release: make(chan struct{})}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = old }()

	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "claude", Model: "haiku", Tier: "low", Status: "completed", ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(driver.release) }) }
	resumeDone := make(chan error, 1)
	go func() {
		_, err := runWithStore(t, driver, storeDir, "resume", "--name", "worker", "prompt")
		resumeDone <- err
	}()
	select {
	case <-driver.started:
	case <-time.After(2 * time.Second):
		release()
		t.Fatal("provider turn did not start")
	}
	defer release()

	var out bytes.Buffer
	status := newAgentCmd()
	status.SetOut(&out)
	status.SetErr(new(bytes.Buffer))
	status.SetArgs([]string{"status", "--name", "worker", "--json", "--store-dir", storeDir})
	if err := status.Execute(); err != nil {
		t.Fatal(err)
	}
	var got subagent.Session
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "running" {
		t.Errorf("mid-turn status = %q, want running", got.Status)
	}
	activeRole, activeSession := os.Getenv(agentRoleEnv), os.Getenv(agentSessionEnv)
	_ = os.Setenv(agentRoleEnv, "")
	_ = os.Setenv(agentSessionEnv, "")
	_, secondErr := runWithStore(t, driver, storeDir, "resume", "--name", "worker", "conflict")
	_ = os.Setenv(agentRoleEnv, activeRole)
	_ = os.Setenv(agentSessionEnv, activeSession)
	if secondErr == nil || !strings.Contains(secondErr.Error(), "already has a resume turn running") {
		t.Errorf("second resume error = %v, want active-turn refusal", secondErr)
	}
	release()
	if err := <-resumeDone; err != nil {
		t.Fatal(err)
	}
	finished, err := store.Get("sid")
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != "completed" || finished.ProcessPID != 0 {
		t.Fatalf("final session state = status %q pid %d, want completed/0", finished.Status, finished.ProcessPID)
	}
}

func TestAgentResumeReclaimsStaleRunningSession(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	driver := &recordingAgentDriver{}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	deadProcess := exec.Command("true")
	if err := deadProcess.Start(); err != nil {
		t.Fatal(err)
	}
	deadPID := deadProcess.Process.Pid
	if err := deadProcess.Wait(); err != nil {
		t.Fatalf("wait for dead PID fixture: %v", err)
	}
	if processExists(deadPID) {
		t.Fatalf("test fixture PID %d unexpectedly exists", deadPID)
	}
	if err := store.Save(&subagent.Session{ID: "stale", Name: "worker", Provider: "claude", Model: "haiku", Tier: "low", Status: "running", ProcessPID: deadPID, ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	if _, err := runWithStore(t, driver, storeDir, "resume", "--name", "worker", "prompt"); err != nil {
		t.Fatalf("resume stale session: %v", err)
	}
	finished, err := store.Get("stale")
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != "completed" || finished.ProcessPID != 0 || finished.LastError != "" {
		t.Fatalf("reclaimed session = status %q pid %d error %q", finished.Status, finished.ProcessPID, finished.LastError)
	}
}

// runWithStore runs an agent command against a preloaded store directory.
func runWithStore(t *testing.T, d subagent.Driver, storeDir string, args ...string) (string, error) {
	t.Helper()
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(append(args, "--store-dir", storeDir))
	err := cmd.Execute()
	return out.String(), err
}

func saveSessions(t *testing.T, storeDir string, sessions ...*subagent.Session) {
	t.Helper()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sessions {
		if s.Provider == "" {
			s.Provider, s.Model = "codex", "gpt-5.6-luna"
		}
		if s.Status == "" {
			s.Status = "completed"
		}
		if s.WorkingDir == "" {
			s.WorkingDir = "."
		}
		if s.ContextTokens == 0 {
			s.ContextTokens = 100
		}
		if err := store.Save(s); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAgentResumeWithoutNameResolution(t *testing.T) {
	now := time.Now()
	older := &subagent.Session{ID: "id-old", Name: "old", LastActiveAt: now.Add(-time.Hour)}
	newer := &subagent.Session{ID: "id-new", Name: "new", LastActiveAt: now}
	other := &subagent.Session{ID: "id-else", Name: "elsewhere", WorkingDir: "/somewhere/else", LastActiveAt: now}
	for _, tc := range []struct {
		name     string
		sessions []*subagent.Session
		args     []string
		wantID   string
		header   string
		wantErr  []string
	}{
		{"unique in dir", []*subagent.Session{older, other}, []string{"resume", "go"}, "id-old", "resolved=dir]", nil},
		{"unique with -c", []*subagent.Session{older}, []string{"resume", "-c", "go"}, "id-old", "resolved=continue]", nil},
		{"-c picks most recent", []*subagent.Session{older, newer, other}, []string{"resume", "-c", "go"}, "id-new", "resolved=continue]", nil},
		{"latest in directory", []*subagent.Session{older, newer}, []string{"resume", "go"}, "id-new", "resolved=dir]", nil},
		{"fuzzy name", []*subagent.Session{{ID: "id-classify", Name: "neus-classify-dev", LastActiveAt: now}}, []string{"resume", "classify", "again"}, "id-classify", "resolved=id]", nil},
		{"id prefix", []*subagent.Session{{ID: "id-prefix-123456", Name: "prefixed", LastActiveAt: now}}, []string{"resume", "id-prefix", "again"}, "id-prefix-123456", "resolved=id]", nil},
		{"ambiguous fuzzy words remain prompt", []*subagent.Session{{ID: "id-a", Name: "neus-classify-dev", LastActiveAt: now}, {ID: "id-b", Name: "classify-api", LastActiveAt: now}}, []string{"resume", "classify", "again"}, "id-b", "resolved=dir]", nil},
		{"none", []*subagent.Session{other}, []string{"resume", "go"}, "", "", []string{"no resumable agent in .", "harnez agent start --name"}},
		{"none with -c", nil, []string{"resume", "-c", "go"}, "", "", []string{"no resumable agent in ."}},
		{"by name", []*subagent.Session{older, newer}, []string{"resume", "--name", "old", "go"}, "id-old", "resolved=name]", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storeDir := t.TempDir()
			saveSessions(t, storeDir, tc.sessions...)
			d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
			out, err := runWithStore(t, d, storeDir, tc.args...)
			if tc.wantErr != nil {
				for _, w := range tc.wantErr {
					if err == nil || !strings.Contains(err.Error(), w) {
						t.Fatalf("err = %v, want %q", err, w)
					}
				}
				if len(d.resumed) != 0 {
					t.Fatalf("provider resumed despite error: %v", d.resumed)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(d.resumed) != 1 || d.resumed[0] != tc.wantID || !strings.Contains(out, tc.header) {
				t.Fatalf("resumed=%v want %s; header %q missing in:\n%s", d.resumed, tc.wantID, tc.header, out)
			}
		})
	}
}

func TestAgentStartFailureKeepsSessionAndResumeSelector(t *testing.T) {
	old := agentDriver
	d := &failedStartStreamDriver{}
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, sess := range []*subagent.Session{
		{ID: "other-1", Name: "other-one", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: ".", Status: "completed"},
		{ID: "other-2", Name: "other-two", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: ".", Status: "completed"},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }}
	err = runStart(cmd, deps, startRequest{Name: "crashed", Prompt: "do work", StoredPrompt: "do work", ModelSpec: "codex:luna:low", Dir: t.TempDir(), StreamMode: streamFull})
	if err == nil || !strings.Contains(err.Error(), "temporary provider failure") {
		t.Fatalf("start error = %v, want provider failure", err)
	}
	startErr := err
	sessions, err := store.List("", true)
	if err != nil {
		t.Fatal(err)
	}
	var recovered *subagent.Session
	for _, sess := range sessions {
		if sess.Name == "crashed" {
			recovered = sess
		}
	}
	if recovered == nil || recovered.ProviderSessionID != "provider-thread" || recovered.Status == "completed" || !strings.Contains(recovered.LastError, "temporary provider failure") {
		t.Fatalf("failed session record = %#v", recovered)
	}
	if strings.Contains(startErr.Error(), "raw generated source") || strings.Contains(startErr.Error(), "traceback line 2") {
		t.Fatalf("failure exposed raw tool output: %v", startErr)
	}

	for _, selector := range []string{"crashed", recovered.ID[:8], "provider-thread"} {
		t.Run("resume-"+selector, func(t *testing.T) {
			resume := newAgentCmd()
			resume.SetOut(new(bytes.Buffer))
			resume.SetErr(new(bytes.Buffer))
			resume.SetArgs([]string{"--store-dir", storeDir, "resume", selector, "continue work"})
			if err := resume.Execute(); err != nil {
				t.Fatalf("resume by %q: %v", selector, err)
			}
		})
	}
	implicit := newAgentCmd()
	implicit.SetOut(new(bytes.Buffer))
	implicit.SetErr(new(bytes.Buffer))
	implicit.SetArgs([]string{"--store-dir", storeDir, "resume", "ordinary prompt"})
	if err := implicit.Execute(); err != nil {
		t.Fatalf("implicit latest-session resume: %v", err)
	}
}

func TestAgentResumeHexLikeWordsRemainPrompts(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	for _, tc := range []struct {
		prompt string
		ids    []string
	}{
		{prompt: "add", ids: []string{"add12345"}},
		{prompt: "c", ids: []string{"cafe1234"}},
		{prompt: "deface", ids: []string{"deface1234", "deface5678"}},
	} {
		t.Run(tc.prompt, func(t *testing.T) {
			storeDir := t.TempDir()
			sessions := make([]*subagent.Session, 0, len(tc.ids))
			for i, id := range tc.ids {
				sessions = append(sessions, &subagent.Session{ID: id, Name: "worker-" + id, Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: ".", Status: "completed", LastActiveAt: time.Now().Add(time.Duration(i) * time.Second)})
			}
			saveSessions(t, storeDir, sessions...)
			d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
			if _, err := runWithStore(t, d, storeDir, "resume", "--continue", tc.prompt); err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(d.prompt, tc.prompt) {
				t.Fatalf("resume prompt = %q, want original prompt suffix %q", d.prompt, tc.prompt)
			}
		})
	}
}

type failedStartStreamDriver struct{ recordingAgentDriver }

func (d *failedStartStreamDriver) Run(ctx context.Context, opts subagent.RunOptions) (*subagent.TurnResult, error) {
	return nil, errors.New("Run path used")
}

func (d *failedStartStreamDriver) RunStream(_ context.Context, _ subagent.RunOptions, emit subagent.EventFunc) (*subagent.TurnResult, error) {
	emit(subagent.Event{Kind: "session", Text: "provider-thread"})
	return nil, errors.New("temporary provider failure\nraw generated source\ntraceback line 2")
}

func (d *failedStartStreamDriver) ResumeStream(ctx context.Context, id, prompt string, model subagent.Model, emit subagent.EventFunc) (*subagent.TurnResult, error) {
	return d.Resume(ctx, id, prompt, model)
}

func (d *failedStartStreamDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{SessionID: "provider-thread", Response: "resumed", Messages: []string{"resumed"}, ContextTokens: 100}, nil
}

func TestAgentDestructiveVerbsRejectContinue(t *testing.T) {
	for _, verb := range []string{"stop", "delete", "compact", "status"} {
		for _, flag := range []string{"-c", "--continue"} {
			_, err := runWithStore(t, &recordingAgentDriver{}, t.TempDir(), verb, flag)
			if err == nil || !strings.Contains(err.Error(), "unknown") {
				t.Fatalf("%s %s: err = %v, want unknown flag", verb, flag, err)
			}
		}
	}
}

func TestAgentStartGeneratesMemorableUniqueNames(t *testing.T) {
	storeDir := t.TempDir()
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		sid := fmt.Sprintf("thread-%d", i)
		d := &scriptDriver{sid: sid, steps: []step{{0, subagent.Event{Kind: "session", Text: sid}}, {0, msg("CONFIRM: ok")}, {0, msg("done")}}}
		out, err := runWithStore(t, d, storeDir, "start", "task")
		if err != nil {
			t.Fatal(err)
		}
		var name string
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "name=") {
				name = strings.Fields(strings.TrimPrefix(line, "name="))[0]
			}
		}
		if name == "" || strings.HasPrefix(name, "agent-") || !strings.Contains(name, "-") || seen[name] {
			t.Fatalf("run %d: generated name %q (seen %v)\n%s", i, name, seen, out)
		}
		seen[name] = true
	}
}

func TestRunStartWithoutCobraFlags(t *testing.T) {
	storeDir := t.TempDir()
	driver := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "direct-start"}}, {ev: msg("CONFIRM: ready")}, {ev: msg("done")}}}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = old }()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err = runStart(cmd, agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }}, startRequest{Prompt: "direct", StoredPrompt: "direct", ModelSpec: "codex:luna", Dir: ".", StreamMode: streamFull})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "name=") {
		t.Fatalf("stream header missing: %s", out.String())
	}
	sessions, err := store.List("", true)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("saved sessions = %v, err=%v", sessions, err)
	}
}

func TestRunStartRecordsQuotaBoundariesForTurn(t *testing.T) {
	storeDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	driver := &recordingAgentDriver{}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = old }()
	var calls []bool
	var captureOnce sync.Once
	captureStarted := make(chan struct{})
	continueCapture := make(chan struct{})
	driver.runHook = func() {
		<-captureStarted
		close(continueCapture)
	}
	capture := func(_ context.Context, _ string, force bool) usage.TurnQuotaReading {
		captureOnce.Do(func() {
			close(captureStarted)
			<-continueCapture
		})
		calls = append(calls, force)
		return usage.TurnQuotaReading{CapturedAt: time.Now().UTC(), HasCache: true, CacheAgeMS: 123}
	}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err = runStart(cmd, agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }, quota: capture, dbPath: dbPath}, startRequest{Prompt: "p", StoredPrompt: "p", ModelSpec: "codex:luna", Dir: ".", StreamMode: streamFull, JSON: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []bool{false, false}) {
		t.Fatalf("quota capture force flags=%v, want [false false]", calls)
	}
	sessions, err := store.List("", true)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions=%+v err=%v", sessions, err)
	}
	usageStore, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer usageStore.Close()
	var boundaries int
	if err := usageStore.QueryRow(context.Background(), `SELECT count(*) FROM turn_quota_boundaries WHERE session_id=? AND turn=1`, sessions[0].ID).Scan(&boundaries); err != nil {
		t.Fatal(err)
	}
	if boundaries != 2 {
		t.Fatalf("stored boundary count=%d, want 2", boundaries)
	}
	tokens, err := usageStore.TurnTokenUsages(context.Background(), sessions[0].ID)
	if err != nil || len(tokens) != 2 {
		t.Fatalf("stored token rows=%+v err=%v, want delta and cumulative", tokens, err)
	}
	if _, err := os.Stat(filepath.Join(storeDir, "quota-readings.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("legacy quota spool exists after turn: stat err=%v", err)
	}
}

func TestTurnQuotaBaselineUsesBeforeCaptureCompletion(t *testing.T) {
	turnStarted := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	capturedAt := turnStarted.Add(5 * time.Second)
	before := usage.TurnQuotaReading{CapturedAt: capturedAt, HasCache: true, CacheAgeMS: 40}

	got := turnQuotaBaseline(turnStarted, before)
	want := capturedAt.Add(-40 * time.Millisecond)
	if !got.Equal(want) || !got.After(turnStarted) {
		t.Fatalf("turnQuotaBaseline = %s, want %s after turn start", got, want)
	}
}

func TestWarnQuota1Changes(t *testing.T) {
	dir := t.TempDir()
	state, _, err := quota1.ResolveStateFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	runAt := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	if err := os.MkdirAll(filepath.Dir(state), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state, []byte(runAt.Format(time.RFC3339Nano)+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 7; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file-%d.go", i))
		if err := os.WriteFile(path, []byte("package test\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, runAt.Add(time.Minute), runAt.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	cmd := &cobra.Command{}
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	warnQuota1Changes(cmd, dir, runAt)
	got := errOut.String()
	if !strings.Contains(got, "7 file(s) changed after the last quota-1 run at "+runAt.Format(time.RFC3339)+":") || !strings.Contains(got, "— code is untested, run make test-q1") {
		t.Fatalf("warning = %q", got)
	}
	if strings.Count(got, "file-") != 5 || out.Len() != 0 {
		t.Fatalf("warning should list 5 files once on stderr; stderr=%q stdout=%q", got, out.String())
	}
}

func TestAgentTurnsWarnAboutQuota1Changes(t *testing.T) {
	for _, mode := range []string{"start", "resume"} {
		t.Run(mode, func(t *testing.T) {
			repoDir := t.TempDir()
			store, err := subagent.NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			runAt := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
			state, _, err := quota1.ResolveStateFile(repoDir)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(state), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(state, []byte(runAt.Format(time.RFC3339Nano)+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(repoDir, "changed.go")
			if err := os.WriteFile(source, []byte("package changed\n"), 0644); err != nil {
				t.Fatal(err)
			}
			future := runAt.Add(time.Minute)
			if err := os.Chtimes(source, future, future); err != nil {
				t.Fatal(err)
			}
			driver := &recordingAgentDriver{}
			old := agentDriver
			agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
			defer func() { agentDriver = old }()
			cmd := &cobra.Command{}
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }}
			if mode == "start" {
				err = runStart(cmd, deps, startRequest{Prompt: "task", StoredPrompt: "task", ModelSpec: "codex:luna", Dir: repoDir, StreamMode: streamFull, JSON: true})
			} else {
				if err := store.Save(&subagent.Session{ID: "resume-id", Name: "worker", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: repoDir, Status: "completed", ContextTokens: 100}); err != nil {
					t.Fatal(err)
				}
				deps.find = func(_ *cobra.Command, s *subagent.FileSessionStore, name string) (*subagent.Session, error) {
					sessions, err := s.List("", true)
					if err != nil {
						return nil, err
					}
					for _, session := range sessions {
						if session.Name == name || session.ID == name {
							return session, nil
						}
					}
					return nil, fmt.Errorf("session %q not found", name)
				}
				err = runResume(cmd, deps, resumeRequest{Name: "worker", Prompt: "continue", StreamMode: streamFull, JSON: true})
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stderr.String(), "1 file(s) changed after the last quota-1 run at "+runAt.Format(time.RFC3339)+": changed.go — code is untested, run make test-q1") {
				t.Fatalf("stderr warning = %q", stderr.String())
			}
			if !strings.HasPrefix(stdout.String(), "{") {
				t.Fatalf("stdout should remain JSON, got %q", stdout.String())
			}
		})
	}
}

func TestRunResumeRecordsFreshQuotaPairAndAdvancesTurn(t *testing.T) {
	storeDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "resume-1", Name: "worker", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: ".", Status: "completed", Turn: 4, ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	driver := &recordingAgentDriver{}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = old }()
	var calls []bool
	capture := func(_ context.Context, _ string, force bool) usage.TurnQuotaReading {
		calls = append(calls, force)
		return usage.TurnQuotaReading{CapturedAt: time.Now().UTC(), HasCache: true, CacheAgeMS: 10}
	}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }, quota: capture, dbPath: dbPath, find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
		return s.Find(id)
	}}
	if err := runResume(cmd, deps, resumeRequest{Name: "worker", Prompt: "continue", JSON: true, StreamMode: streamFull}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []bool{false, false}) {
		t.Fatalf("quota capture force flags=%v, want [false false]", calls)
	}
	sess, err := store.Get("resume-1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Turn != 5 {
		t.Fatalf("persisted turn=%d, want 5", sess.Turn)
	}
	usageStore, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer usageStore.Close()
	var boundaries int
	if err := usageStore.QueryRow(context.Background(), `SELECT count(*) FROM turn_quota_boundaries WHERE session_id=? AND turn=5`, "resume-1").Scan(&boundaries); err != nil {
		t.Fatal(err)
	}
	if boundaries != 2 {
		t.Fatalf("stored boundary count=%d, want 2", boundaries)
	}
	if _, err := os.Stat(filepath.Join(storeDir, "quota-readings.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("legacy quota spool exists after resume: stat err=%v", err)
	}
}

func TestRunResumeWithoutCobraFlags(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "direct-resume", Name: "direct", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: ".", Status: "completed", ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	driver := &scriptDriver{steps: []step{{ev: msg("CONFIRM: resumed")}, {ev: msg("done")}}}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = old }()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }, find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
		return resolveSession(s, id, "")
	}}
	if err := runResume(cmd, deps, resumeRequest{Prompt: "continue", Name: "direct", Dir: ".", StreamMode: streamFull}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "resolved=name") {
		t.Fatalf("stream header missing: %s", out.String())
	}
	if len(driver.resumed) != 1 || driver.resumed[0] != "direct-resume" {
		t.Fatalf("resumed = %v", driver.resumed)
	}
}

func TestRunResumePassesStoredAgyModelAndTier(t *testing.T) {
	workDir := t.TempDir()
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "agy-session", Name: "worker", Provider: "agy", Model: "flash37", Tier: "med", WorkingDir: workDir, Status: "completed", ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	var args []string
	old := agentDriver
	agentDriver = func(_ subagent.Model, dir string) subagent.Driver {
		return subagent.AgyDriver{Dir: dir, Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
			args = gotArgs
			return []byte(`{"conversation_id":"agy-session","status":"SUCCESS","response":"done"}`), nil
		}}
	}
	defer func() { agentDriver = old }()
	cmd := &cobra.Command{}
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }, find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
		return s.Find(id)
	}}
	if err := runResume(cmd, deps, resumeRequest{Name: "worker", Prompt: "continue", StreamMode: streamFull}); err != nil {
		t.Fatal(err)
	}
	want := []string{"--conversation", "agy-session", "--add-dir", workDir, "--model", "flash37", "--effort", "medium", "--output-format", "json", "-p", "continue"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("agy resume args = %#v, want %#v", args, want)
	}
}

func TestAgentRootPromptStartsGeneratedSession(t *testing.T) {
	d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "root"}}, {ev: msg("CONFIRM: ok")}, {ev: msg("done")}}}
	out, err := runWithStore(t, d, t.TempDir(), "-p", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "resolved=new") {
		t.Fatalf("output = %q", out)
	}
}

func TestAgentRootPromptModelAlias(t *testing.T) {
	d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "root"}}, {ev: msg("ok")}}}
	if _, err := runWithStore(t, d, t.TempDir(), "--model", "luna", "-p", "hello"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(d.prompt, "\n\nhello") {
		t.Fatalf("prompt = %q", d.prompt)
	}
}

func TestAgentRootNameUpsert(t *testing.T) {
	for _, exists := range []bool{false, true} {
		t.Run(fmt.Sprintf("exists=%v", exists), func(t *testing.T) {
			d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "root"}}, {ev: msg("ok")}}}
			dir := t.TempDir()
			if exists {
				saveSessions(t, dir, &subagent.Session{ID: "named", Name: "worker", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: "."})
			}
			args := []string{"--name", "worker", "--", "words"}
			out, err := runWithStore(t, d, dir, args...)
			if err != nil {
				t.Fatal(err)
			}
			want := "resolved=name"
			if !exists {
				want = "resolved=new"
			}
			if !strings.Contains(out, want) {
				t.Fatalf("output = %q", out)
			}
		})
	}
}

func TestAgentRootContinueStartOrResumeMostRecent(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "new"}}, {ev: msg("ok")}}}
		out, err := runWithStore(t, d, t.TempDir(), "-c", "-p", "hello")
		if err != nil || !strings.Contains(out, "resolved=new") {
			t.Fatalf("out=%q err=%v", out, err)
		}
	})
	t.Run("resume-most-recent", func(t *testing.T) {
		dir := t.TempDir()
		now := time.Now()
		saveSessions(t, dir, &subagent.Session{ID: "old", Name: "old", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: ".", LastActiveAt: now.Add(-time.Hour)}, &subagent.Session{ID: "new", Name: "new", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: ".", LastActiveAt: now})
		d := &scriptDriver{steps: []step{{ev: msg("ok")}}}
		out, err := runWithStore(t, d, dir, "-c", "-p", "hello")
		if err != nil || !strings.Contains(out, "resolved=continue") || len(d.resumed) != 1 || d.resumed[0] != "new" {
			t.Fatalf("out=%q resumed=%v err=%v", out, d.resumed, err)
		}
	})
}

func TestAgentRootContinueNameConflict(t *testing.T) {
	_, err := runWithStore(t, &scriptDriver{}, t.TempDir(), "-c", "--name", "worker", "-p", "hello")
	if err == nil || err.Error() != "agent: --continue cannot be combined with --name" {
		t.Fatalf("err = %v", err)
	}
}

func TestAgentRootPromptInputs(t *testing.T) {
	file := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(file, []byte("from file"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-f", file}, {"--", "/x"}} {
		d := &scriptDriver{steps: []step{{ev: msg("ok")}}}
		if _, err := runWithStore(t, d, t.TempDir(), args...); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(d.prompt, strings.TrimPrefix(args[len(args)-1], "/")) && args[0] == "--" {
			t.Fatalf("prompt = %q", d.prompt)
		}
	}
}

func TestAgentRootNoPromptShowsHelp(t *testing.T) {
	out, err := runWithStore(t, &scriptDriver{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Manage subagent sessions") {
		t.Fatalf("help = %q", out)
	}
}

type slashDriver struct{ compacted, stopped []string }

func (d *slashDriver) Run(context.Context, subagent.RunOptions) (*subagent.TurnResult, error) {
	return nil, errors.New("provider Run called")
}
func (d *slashDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	return nil, errors.New("provider Resume called")
}
func (d *slashDriver) Compact(_ context.Context, id string) (*subagent.TurnResult, error) {
	d.compacted = append(d.compacted, id)
	return &subagent.TurnResult{}, nil
}
func (d *slashDriver) Stop(_ context.Context, id string) error {
	d.stopped = append(d.stopped, id)
	return nil
}
func (d *slashDriver) Delete(context.Context, string) error { return nil }

func TestAgentRootSlashCompactIntercepts(t *testing.T) {
	dir := t.TempDir()
	saveSessions(t, dir, &subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: "."})
	d := &slashDriver{}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	if _, err := runWithStore(t, d, dir, "-p", "/compact"); err == nil || !strings.Contains(err.Error(), "manual /compact is unsupported") {
		t.Fatalf("compact error = %v, want unsupported Codex manual compact", err)
	}
	if len(d.compacted) != 0 {
		t.Fatalf("manual Codex compact calls = %v, want none", d.compacted)
	}
}

func TestAgentRootSlashStatusAndStop(t *testing.T) {
	for _, command := range []string{"/status", "/stop"} {
		t.Run(command, func(t *testing.T) {
			dir := t.TempDir()
			saveSessions(t, dir, &subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: "."})
			d := &slashDriver{}
			old := agentDriver
			agentDriver = func(subagent.Model, string) subagent.Driver { return d }
			defer func() { agentDriver = old }()
			out, err := runWithStore(t, d, dir, "-p", command)
			if err != nil {
				t.Fatal(err)
			}
			if command == "/status" && !strings.Contains(out, "Status: completed") {
				t.Fatalf("output = %q", out)
			}
			if command == "/stop" && !reflect.DeepEqual(d.stopped, []string{"sid"}) {
				t.Fatalf("stopped = %v", d.stopped)
			}
		})
	}
}

func TestAgentRootSlashErrorsAndLiteral(t *testing.T) {
	// A known command with no session resolves like `resume`; an unknown one
	// is rejected first (see TestAgentRootUnknownSlashIsRejectedBeforeSessionResolution).
	_, err := runWithStore(t, &slashDriver{}, t.TempDir(), "-p", "/status")
	if err == nil || err.Error() != "no resumable agent in .; start one with: harnez agent start --name <name> ..." {
		t.Fatalf("err = %v", err)
	}
	dir := t.TempDir()
	d := &scriptDriver{steps: []step{{ev: msg("ok")}}}
	if _, err := runWithStore(t, d, dir, "--", "/x"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.prompt, "/x") {
		t.Fatalf("prompt = %q", d.prompt)
	}
	dir = t.TempDir()
	saveSessions(t, dir, &subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: "."})
	_, err = runWithStore(t, &slashDriver{}, dir, "-p", "/x")
	if err == nil || err.Error() != `unknown agent command "/x"; send it literally with: -- /x` {
		t.Fatalf("err = %v", err)
	}
}

func TestAgentRootUnknownSlashIsRejectedBeforeSessionResolution(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
	_, err := runWithStore(t, d, t.TempDir(), "-p", "/foo") // empty store: no session to resolve
	if err == nil || err.Error() != `unknown agent command "/foo"; send it literally with: -- /foo` {
		t.Fatalf("err = %v", err)
	}
	if len(d.resumed) != 0 {
		t.Fatalf("provider resumed: %v", d.resumed)
	}
}

func TestAgentRootSlashRejectsContinueWithName(t *testing.T) {
	storeDir := t.TempDir()
	saveSessions(t, storeDir, &subagent.Session{ID: "id-a", Name: "a"})
	_, err := runWithStore(t, &recordingAgentDriver{}, storeDir, "-p", "/compact", "-c", "--name", "a")
	if err == nil || !strings.Contains(err.Error(), "--continue cannot be combined with --name") {
		t.Fatalf("err = %v", err)
	}
}

func TestStatsModeDoesNotRepeatAnAlreadyShownFinalMessage(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: will do it")}}} // the only message is also the reply
	got := runScripted(t, d, "start", "--model", "luna", "task", "--name", "w", "--stream", "stats")
	if strings.Count(got, "will do it") != 1 || strings.Contains(got, "CONFIRM:") {
		t.Fatalf("final message repeated or raw tag shown:\n%s", got)
	}
	if !strings.Contains(got, "[reply: 0s]\n(the final message was already shown above)\n") {
		t.Fatalf("missing reply marker:\n%s", got)
	}
}

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

func TestRateLatestSessionTurnPersistsRatingAndReason(t *testing.T) {
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "session-1", Name: "worker", Turn: 2, TurnRecords: []subagent.TurnRecord{{Turn: 1}, {Turn: 2}}}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	if err := rateLatestSessionTurn(store, sess, 4, "tests green"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Get(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TurnRecords[1].Rating == nil || *loaded.TurnRecords[1].Rating != 4 || loaded.TurnRecords[1].RatingReason != "tests green" {
		t.Fatalf("latest turn rating = %+v", loaded.TurnRecords[1])
	}
	if err := rateLatestSessionTurn(store, loaded, 6, "invalid"); err == nil {
		t.Fatal("out-of-range score accepted")
	}
}

func TestRateLatestSessionTurnCreatesRecordForPreM1Session(t *testing.T) {
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "legacy", Name: "dev519", Turn: 2}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	if err := rateLatestSessionTurn(store, sess, 4, "good run"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Get(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.TurnRecords) != 1 || loaded.TurnRecords[0].Turn != 2 || loaded.TurnRecords[0].Rating == nil || *loaded.TurnRecords[0].Rating != 4 {
		t.Fatalf("legacy session turn records = %+v", loaded.TurnRecords)
	}
}

func TestAgentDeleteWarnsAndProtectsUnratedSessions(t *testing.T) {
	for _, tc := range []struct {
		name       string
		rated      bool
		force      bool
		wantDelete bool
		wantWarn   bool
	}{
		{name: "rated", rated: true, wantDelete: true},
		{name: "unrated", wantWarn: true},
		{name: "forced", force: true, wantDelete: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storeDir := t.TempDir()
			store, err := subagent.NewSessionStore(storeDir)
			if err != nil {
				t.Fatal(err)
			}
			sess := &subagent.Session{ID: "session-" + tc.name, Name: "worker", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1}}, HarnessType: "interactive", Status: "completed"}
			if tc.rated {
				rating := 4
				sess.TurnRecords[0].Rating = &rating
			}
			if err := store.Save(sess); err != nil {
				t.Fatal(err)
			}
			out, stderr := new(bytes.Buffer), new(bytes.Buffer)
			cmd := newAgentCmd()
			cmd.SetOut(out)
			cmd.SetErr(stderr)
			args := []string{"delete", "--name", "worker", "--store-dir", storeDir}
			if tc.force {
				args = append(args, "--force")
			}
			cmd.SetArgs(args)
			err = cmd.Execute()
			if tc.wantDelete && err != nil {
				t.Fatalf("delete error = %v", err)
			}
			if !tc.wantDelete && err == nil {
				t.Fatal("unrated session deletion succeeded without --force")
			}
			if got := stderr.String(); tc.wantWarn {
				if !strings.Contains(got, "warning: unrated latest turns:") || !strings.Contains(got, `worker: harnez agent rate --name worker <1-5> "<reason>"`) {
					t.Fatalf("warning = %q", got)
				}
			} else if strings.Contains(stderr.String(), "warning: unrated latest turns:") {
				t.Fatalf("rated deletion warning = %q", stderr.String())
			}
			_, findErr := store.Get(sess.ID)
			if tc.wantDelete && findErr == nil {
				t.Fatal("session remains after successful deletion")
			}
			if !tc.wantDelete && findErr != nil {
				t.Fatalf("refused session was deleted: %v", findErr)
			}
		})
	}
}

func TestReattachStepsUseHostVocabulary(t *testing.T) {
	for host, want := range map[string][]string{
		"codex":  {"exec_command", "write_stdin", "session_id", "Do NOT end your turn"},
		"claude": {"run_in_background: true", "Do NOT poll"},
		"agy":    {"background task", "Do NOT poll"},
		"":       {"host background job", "Do NOT poll"},
	} {
		got := strings.Join(reattachSteps(host, "calm-otter"), " ")
		for _, w := range append(want, "harnez agent wait calm-otter") {
			if !strings.Contains(got, w) {
				t.Errorf("host %q: %q missing %q", host, got, w)
			}
		}
	}
	if got := strings.Join(reattachSteps("codex", "x"), " "); strings.Contains(got, "Do NOT poll") {
		t.Errorf("codex steps forbid polling, its only wait mechanism: %q", got)
	}
}

func TestDetachHostPrefersCodexThread(t *testing.T) {
	t.Setenv("HARNEZ_AGENT", "claude")
	t.Setenv("CODEX_THREAD_ID", "t1")
	if got := detachHost(); got != "codex" {
		t.Fatalf("detachHost = %q, want codex", got)
	}
}

// TestAgentHelpHidesDetachAndNamesSessionBackground guards issue 630: hosts that
// read --detach in the help lost track of their workers.
func TestAgentHelpHidesDetachAndNamesSessionBackground(t *testing.T) {
	for _, sub := range []string{"start", "resume"} {
		t.Run(sub, func(t *testing.T) {
			c := newAgentCmd()
			var out bytes.Buffer
			c.SetOut(&out)
			c.SetErr(new(bytes.Buffer))
			c.SetArgs([]string{sub, "--help"})
			if err := c.Execute(); err != nil {
				t.Fatal(err)
			}
			help := out.String()
			for _, hidden := range []string{"--detach", "--async"} {
				if strings.Contains(help, hidden) {
					t.Errorf("%s help still lists %s:\n%s", sub, hidden, help)
				}
			}
			for _, want := range []string{"session background", "HTO=0", "run_in_background: true", "run_command", "WaitMsBeforeAsync", "exec_command", "write_stdin", "30 minutes", "Never check status or logs in a loop"} {
				if !strings.Contains(help, want) {
					t.Errorf("%s help missing %q", sub, want)
				}
			}
		})
	}
}

// TestHTOZeroDisablesForegroundDetach guards the one-call session-background
// form (issue 630): with HTO=0 the turn must never detach, and the wait hint
// must not ask for a separate wait job.
func TestHTOZeroDisablesForegroundDetach(t *testing.T) {
	t.Setenv(foregroundDetachTestOverrideEnv, "1")
	t.Setenv(execTimeoutEffectiveEnv, "60s")
	t.Setenv(execTimeoutExplicitEnv, "")
	t.Setenv(execTimeoutEnv, "")
	t.Setenv(execTimeoutShortEnv, "")
	if got := foregroundDetachTimeout(newAgentCmd()); got <= 0 {
		t.Fatalf("baseline detach timeout = %s, want > 0 without HTO", got)
	}
	if hint := waitHint("w"); !strings.Contains(hint, "harnez agent wait w") {
		t.Fatalf("baseline wait hint = %q, want reattach steps", hint)
	}
	t.Setenv(execTimeoutShortEnv, "0")
	if got := foregroundDetachTimeout(newAgentCmd()); got != 0 {
		t.Fatalf("detach timeout with HTO=0 = %s, want 0", got)
	}
	hint := waitHint("w")
	if strings.Contains(hint, "harnez agent wait") || !strings.Contains(hint, "does not detach") {
		t.Fatalf("wait hint with HTO=0 = %q, want no separate wait job", hint)
	}
}
