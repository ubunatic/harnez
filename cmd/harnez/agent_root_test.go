package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
)

func TestAgentRootPromptStartsGeneratedSession(t *testing.T) {
	d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "root"}}, {ev: msg("CONFIRM: ok")}, {ev: msg("done")}}}
	out, err := runWithStore(t, d, t.TempDir(), "--role", "advisor", "-p", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "resolved=new") {
		t.Fatalf("output = %q", out)
	}
}

func TestAgentRootPromptModelAlias(t *testing.T) {
	d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "root"}}, {ev: msg("ok")}}}
	if _, err := runWithStore(t, d, t.TempDir(), "--role", "advisor", "--model", "luna", "-p", "hello"); err != nil {
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
				saveSessions(t, dir, &subagent.Session{ID: "named", Name: "worker", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: "."})
			}
			args := []string{"--role", "advisor", "--name", "worker", "--", "words"}
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
		out, err := runWithStore(t, d, t.TempDir(), "--role", "advisor", "-c", "-p", "hello")
		if err != nil || !strings.Contains(out, "resolved=new") {
			t.Fatalf("out=%q err=%v", out, err)
		}
	})
	t.Run("resume-most-recent", func(t *testing.T) {
		dir := t.TempDir()
		now := time.Now()
		saveSessions(t, dir, &subagent.Session{ID: "old", Name: "old", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: ".", LastActiveAt: now.Add(-time.Hour)}, &subagent.Session{ID: "new", Name: "new", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: ".", LastActiveAt: now})
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
	for _, args := range [][]string{{"--role", "advisor", "-f", file}, {"--role", "advisor", "--", "/x"}} {
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
	saveSessions(t, dir, &subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: "."})
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
			saveSessions(t, dir, &subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: "."})
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
	if _, err := runWithStore(t, d, dir, "--role", "advisor", "--", "/x"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.prompt, "/x") {
		t.Fatalf("prompt = %q", d.prompt)
	}
	dir = t.TempDir()
	saveSessions(t, dir, &subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: "."})
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
