package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/channel"
	"ubunatic.com/harnez/internal/subagent"
)

type queuedResumeDriver struct {
	resumeOutcomeDriver
	prompts  []string
	fail     bool
	onResume func()
}

func (d *queuedResumeDriver) Resume(_ context.Context, id, prompt string, _ subagent.Model) (*subagent.TurnResult, error) {
	d.prompts = append(d.prompts, prompt)
	if d.onResume != nil {
		d.onResume()
	}
	if d.fail {
		return nil, errors.New("resume failed")
	}
	return &subagent.TurnResult{SessionID: id, Response: "ok", ContextTokens: 100}, nil
}

func TestQueuedMessagesResumeOnceAndFailurePreserves(t *testing.T) {
	driver := &queuedResumeDriver{fail: true}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	t.Cleanup(func() { agentDriver = old })
	dir := t.TempDir()
	store := saveResumeSession(t, dir, "sid", "worker", "fake")
	sess, _ := store.Get("sid")
	sess.Status = "running"
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	for _, msg := range []string{"FIRST-815", "SECOND-815"} {
		if _, err := store.QueueMessage("worker", "", "sender-815", msg); err != nil {
			t.Fatal(err)
		}
	}
	resume := func() error {
		cmd := newAgentCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"resume", "--name", "worker", "continue", "--store-dir", dir})
		return cmd.Execute()
	}
	if err := resume(); err == nil {
		t.Fatal("expected failure")
	}
	sess, _ = store.Get("sid")
	if len(sess.PendingMessages) != 2 {
		t.Fatal("failure lost messages")
	}
	driver.fail = false
	driver.onResume = func() {
		if _, err := store.QueueMessage("worker", "", "late-sender", "LATER-815"); err != nil {
			t.Fatal(err)
		}
	}
	if err := resume(); err != nil {
		t.Fatal(err)
	}
	driver.onResume = nil
	if err := resume(); err != nil {
		t.Fatal(err)
	}
	for _, prompt := range driver.prompts[:2] {
		if strings.Count(prompt, "FIRST-815") != 1 || strings.Count(prompt, "SECOND-815") != 1 || strings.Index(prompt, "FIRST-815") > strings.Index(prompt, "SECOND-815") || !strings.Contains(prompt, "sender-815") {
			t.Fatalf("prompt = %q", prompt)
		}
	}
	if strings.Contains(driver.prompts[2], "FIRST-815") || strings.Count(driver.prompts[2], "LATER-815") != 1 {
		t.Fatalf("next prompt = %q", driver.prompts[2])
	}
	sess, _ = store.Get("sid")
	if len(sess.PendingMessages) != 0 || sess.Turn != 2 {
		t.Fatalf("session = %#v", sess)
	}
}

func TestAgentSendRegistryFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	// Disable discovery of any real local Codex sessions.
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("HARNEZ_SESSION_ID", "sender-815")
	dir := t.TempDir()
	store, _ := subagent.NewSessionStore(dir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", HarnessType: "harnez", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	for _, message := range []string{"ONE", "TWO"} {
		var out bytes.Buffer
		cmd := newAgentCmd()
		cmd.SetOut(&out)
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"send", "--name", "worker", "--store-dir", dir, message})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if out.String() != "queued for next turn of worker\n" {
			t.Fatalf("output = %q", out.String())
		}
	}
	sess, err := store.Get("sid")
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.PendingMessages) != 2 || sess.PendingMessages[0].Text != "ONE" || sess.PendingMessages[1].Text != "TWO" || !strings.Contains(sess.PendingMessages[0].Sender, "sender-815") {
		t.Fatalf("messages = %#v", sess.PendingMessages)
	}
	for _, name := range []string{"missing", "sid"} {
		cmd := newAgentCmd()
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"send", "--name", name, "--store-dir", dir, "msg"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown") {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestLiveSendResolutionDoesNotFallBack(t *testing.T) {
	threads := []channel.CodexThread{{ID: "live", Name: "worker"}}
	target, err := resolveAgentSendTarget(nil, nil, threads, nil, "worker", "", "")
	if err != nil || target.Provider != "codex" || target.Endpoint.SessionID != "live" {
		t.Fatalf("target = %#v, %v", target, err)
	}
	threads = append(threads, channel.CodexThread{ID: "other-live", Name: "worker"})
	_, err = resolveAgentSendTarget(nil, nil, threads, nil, "worker", "", "")
	var unknown *unknownLiveSendTarget
	if err == nil || errors.As(err, &unknown) {
		t.Fatalf("live ambiguity would fall back: %v", err)
	}
}

func TestUnknownLiveTargetCapsCandidateList(t *testing.T) {
	threads := make([]channel.CodexThread, 14)
	for i := range threads {
		threads[i] = channel.CodexThread{ID: fmt.Sprintf("thread-%02d", i), Name: fmt.Sprintf("worker-%02d", i), CWD: "/projects/other"}
	}
	_, err := resolveAgentSendTarget(nil, nil, threads, nil, "missing", "", "")
	if err == nil || !strings.Contains(err.Error(), "(4 more; use harnez agent list)") {
		t.Fatalf("candidate limit notice: %v", err)
	}
	for _, omitted := range []string{"thread-10", "thread-11", "thread-12", "thread-13"} {
		if strings.Contains(err.Error(), omitted) {
			t.Fatalf("candidate list is not capped: %v", err)
		}
	}
	if !strings.Contains(err.Error(), "thread-00") || !strings.Contains(err.Error(), "thread-09") {
		t.Fatalf("candidate list missing first ten: %v", err)
	}
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	threads[13].CWD = workingDir
	_, err = resolveAgentSendTarget(nil, nil, threads, nil, "missing", "", "")
	if err == nil || !strings.Contains(strings.Split(err.Error(), "\n")[1], "thread-13") {
		t.Fatalf("current directory candidate was not prioritized: %v", err)
	}
}

func TestAgentSendQueuesOnlyResumableRecords(t *testing.T) {
	completed := &subagent.Session{ID: "done", Name: "done", Provider: "codex", Status: "completed"}
	if err := checkAgentSendResumable(completed, &resumeOutcomeDriver{}); err != nil {
		t.Fatalf("completed resumable record rejected: %v", err)
	}
	if err := checkAgentSendResumable(completed, &resumeOutcomeDriver{terminal: true}); err == nil || !strings.Contains(err.Error(), "cannot be resumed") {
		t.Fatalf("terminal provider session accepted: %v", err)
	}
	completed.ResumeBlockedReason = "provider session expired"
	if err := checkAgentSendResumable(completed, &resumeOutcomeDriver{}); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("resume-blocked session accepted: %v", err)
	}
}
