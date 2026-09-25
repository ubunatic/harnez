package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
)

func TestAssemblePrompt(t *testing.T) {
	dir := t.TempDir()
	one := dir + "/one"
	two := dir + "/two"
	if err := os.WriteFile(one, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(two, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name               string
		files, words, tail []string
		stdin, want, err   string
	}{
		{"order", []string{one, two}, []string{"word", "two"}, []string{"tail", "--weird"}, "", "first\n\nsecond\n\nword two\n\ntail --weird", ""},
		{"stdin", []string{"-"}, nil, nil, "from stdin", "from stdin", ""},
		{"missing", []string{dir + "/missing"}, nil, nil, "", "", "read prompt file"},
		{"empty", nil, nil, nil, "", "", "no prompt given"},
		{"dash tail", nil, nil, []string{"-p", "--literal"}, "", "-p --literal", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assemblePrompt(tt.files, tt.words, tt.tail, strings.NewReader(tt.stdin))
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q, err %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestAgentSessionVerbsRejectPositionalSessions(t *testing.T) {
	for _, args := range [][]string{
		{"stop", "worker"}, {"delete", "worker"}, {"status", "worker"},
		{"compact", "worker"}, {"chat", "attach", "worker"},
	} {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			cmd := newAgentCmd()
			cmd.SetArgs(args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("positional session unexpectedly accepted")
			}
		})
	}
}

func TestAgentRootVerbAndPromptDispatch(t *testing.T) {
	old := agentDriver
	driver := &recordingAgentDriver{}
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = old }()

	tests := []struct {
		name, wantError, wantPrompt string
		args                        []string
	}{
		{name: "unknown verb", args: []string{"foo"}, wantError: `unknown command "foo" for "harnez agent"`},
		{name: "typo suggestion", args: []string{"resum"}, wantError: "resume"},
		{name: "quoted multi-word prompt", args: []string{"update the changelog"}, wantPrompt: "update the changelog"},
		{name: "dash prompt", args: []string{"--", "foo"}, wantPrompt: "foo"},
		{name: "prompt flag", args: []string{"-p", "foo"}, wantPrompt: "foo"},
		{name: "prompt consumes flag", args: []string{"-p", "--model"}, wantError: `-p needs prompt text but got flag "--model"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newAgentCmd()
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs(append([]string{"--store-dir", t.TempDir()}, tc.args...))
			err := cmd.Execute()
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want substring %q", err, tc.wantError)
				}
				return
			}
			if err != nil || driver.prompt != tc.wantPrompt {
				t.Fatalf("prompt = %q, error = %v; want %q", driver.prompt, err, tc.wantPrompt)
			}
		})
	}
}

func TestAgentErrorsNameTheFixWithoutUsageDump(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"stop", "foo"}, "session is now --name <session>"},
		{[]string{"delete", "foo"}, "session is now --name <session>"},
		{[]string{"status", "foo"}, "session is now --name <session>"},
		{[]string{"compact", "foo"}, "session is now --name <session>"},
		{[]string{"chat", "codex:luna"}, "model is now --model <spec>"},
		{[]string{"start"}, "no prompt given"},
	} {
		var out, errOut bytes.Buffer
		cmd := newAgentCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs(append(tc.args, "--store-dir", t.TempDir()))
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%v: err = %v, want %q", tc.args, err, tc.want)
		}
		if strings.Contains(out.String()+errOut.String(), "Usage:") {
			t.Fatalf("%v: usage dump in output:\n%s%s", tc.args, out.String(), errOut.String())
		}
	}
}

func TestAgentDeleteAllCompletedContinuesOnFailure(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	sessions := []*subagent.Session{
		{ID: "s1", Name: "success", Provider: "codex", Model: "luna", Status: "completed", CreatedAt: now, LastActiveAt: now, WorkingDir: "."},
		{ID: "s2", Name: "fail", Provider: "codex", Model: "luna", Status: "completed", CreatedAt: now, LastActiveAt: now, WorkingDir: ".", ProviderSessionID: "550e8400-e29b-41d4-a716-446655440000"},
		{ID: "s3", Name: "also-success", Provider: "codex", Model: "luna", Status: "completed", CreatedAt: now, LastActiveAt: now, WorkingDir: "."},
	}
	for _, sess := range sessions {
		if err := store.Create(sess); err != nil {
			t.Fatal(err)
		}
	}

	deleteCount := 0
	originalDriver := agentDriver
	agentDriver = func(m subagent.Model, dir string) subagent.Driver {
		return mockDriver{deleteFunc: func(ctx context.Context, id string) error {
			deleteCount++
			if id == "550e8400-e29b-41d4-a716-446655440000" {
				return fmt.Errorf("codex delete: exit status 1: --force requires a session UUID")
			}
			return nil
		}}
	}
	defer func() { agentDriver = originalDriver }()

	var out, errOut bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"delete", "--all-completed", "--store-dir", storeDir})
	err = cmd.Execute()

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "some sessions failed to delete") {
		t.Fatalf("error = %v, want 'some sessions failed to delete'", err)
	}

	// Check that 3 delete attempts were made (success, fail, also-success)
	if deleteCount != 3 {
		t.Fatalf("delete count = %d, want 3", deleteCount)
	}

	// Check that successful deletions are printed
	outStr := out.String()
	if !strings.Contains(outStr, "success") || !strings.Contains(outStr, "also-success") {
		t.Fatalf("stdout missing successful deletions:\n%s", outStr)
	}

	// Check that failure is printed to stderr
	errStr := errOut.String()
	if !strings.Contains(errStr, "fail") || !strings.Contains(errStr, "delete failed") {
		t.Fatalf("stderr missing failure message:\n%s", errStr)
	}
}

type mockDriver struct {
	deleteFunc func(context.Context, string) error
}

func (m mockDriver) Run(ctx context.Context, o subagent.RunOptions) (*subagent.TurnResult, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m mockDriver) Resume(ctx context.Context, id, prompt string, model subagent.Model) (*subagent.TurnResult, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m mockDriver) Compact(ctx context.Context, id string) (*subagent.TurnResult, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m mockDriver) Stop(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented")
}
func (m mockDriver) Delete(ctx context.Context, id string) error {
	return m.deleteFunc(ctx, id)
}
