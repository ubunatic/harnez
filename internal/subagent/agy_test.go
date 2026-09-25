package subagent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/claude"
)

func boolPtr(b bool) *bool { return &b }

func TestAgyRunArgOrderAndAddDir(t *testing.T) {
	var args []string
	d := AgyDriver{Dir: "/work/dir", Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
		args = gotArgs
		return []byte(`{"status":"SUCCESS","response":"PONG\n"}`), nil
	}}
	if _, err := d.Run(context.Background(), RunOptions{Prompt: "ping", Model: Model{Name: "gemini-3.7-flash", Tier: "low"}}); err != nil {
		t.Fatal(err)
	}
	want := []string{"--add-dir", "/work/dir", "--model", "gemini-3.7-flash", "--effort", "low", "--output-format", "json", "-p", "ping"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
	if args[len(args)-2] != "-p" {
		t.Fatalf("-p must be the last flag, directly followed by the prompt: %#v", args)
	}
}

func TestAgyRunInstallsBashShimAndSetsLaunchEnvironment(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("create fake agy bin directory: %v", err)
	}
	capturePath := filepath.Join(home, "agy-env.txt")
	agyPath := filepath.Join(binDir, "agy")
	agyScript := `#!/bin/sh
	pwd > "$AGY_ENV_CAPTURE"
	printf '%s\n' "$PATH" "$ANTIGRAVITY_AGENT" "$HARNEZ_SESSION_ID" "$HARNEZ_AGY_METER_SESSION_ID" "$HTTPS_PROXY" "$SSL_CERT_FILE" >> "$AGY_ENV_CAPTURE"
printf '%s\n' '{"status":"SUCCESS","response":"ok"}'
`
	if err := os.WriteFile(agyPath, []byte(agyScript), 0755); err != nil {
		t.Fatalf("write fake agy executable: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+"/usr/bin:/bin")
	t.Setenv("ANTIGRAVITY_AGENT", "0")
	t.Setenv("AGY_ENV_CAPTURE", capturePath)

	workDir := t.TempDir()
	d := AgyDriver{Dir: workDir, SessionID: "agent-session-uuid"}
	if _, err := d.Run(context.Background(), RunOptions{Prompt: "ping"}); err != nil {
		t.Fatalf("AgyDriver.Run: %v", err)
	}

	captured, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatalf("read captured agy environment: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(captured)), "\n")
	if len(lines) != 7 {
		t.Fatalf("captured environment = %q, want cwd, PATH, agent marker, session IDs, proxy and CA bundle", captured)
	}
	if lines[0] != workDir {
		t.Errorf("agy cwd = %q, want %q", lines[0], workDir)
	}
	shimDir := filepath.Join(home, ".harnez", "shims")
	pathEntries := filepath.SplitList(lines[1])
	if len(pathEntries) == 0 || pathEntries[0] != shimDir {
		t.Errorf("agy PATH = %q, want shim directory %q first", lines[1], shimDir)
	}
	if len(pathEntries) < 2 || pathEntries[1] != binDir {
		t.Errorf("agy PATH = %#v, want inherited PATH after shim prefix", pathEntries)
	}
	if lines[2] != "1" {
		t.Errorf("ANTIGRAVITY_AGENT = %q, want 1", lines[2])
	}
	if lines[3] != "agent-session-uuid" || lines[4] != "agent-session-uuid" {
		t.Errorf("meter session IDs = %q, %q", lines[3], lines[4])
	}
	if !strings.HasPrefix(lines[5], "http://127.0.0.1:") {
		t.Errorf("HTTPS_PROXY = %q", lines[5])
	}
	if !strings.HasSuffix(lines[6], ".harnez/agymeter/roots.pem") {
		t.Errorf("SSL_CERT_FILE = %q", lines[6])
	}

	shimPath := filepath.Join(shimDir, "bash")
	shim, err := os.ReadFile(shimPath)
	if err != nil {
		t.Fatalf("read installed bash shim: %v", err)
	}
	if string(shim) != claude.BashShimContent {
		t.Errorf("bash shim content = %q, want managed shim", shim)
	}
	info, err := os.Stat(shimPath)
	if err != nil {
		t.Fatalf("stat installed bash shim: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("bash shim mode = %o, want 755", info.Mode().Perm())
	}
}

func TestAgyLaunchEnvironmentDoesNotDuplicateShimPrefix(t *testing.T) {
	home := t.TempDir()
	shimDir := filepath.Join(home, ".harnez", "shims")
	original := []string{
		"PATH=" + shimDir + string(os.PathListSeparator) + "/usr/bin:/bin",
		"ANTIGRAVITY_AGENT=0",
		"AGY_TEST_KEEP=preserved",
	}
	got := AgyLaunchEnv(original, home)
	wantPath := shimDir + string(os.PathListSeparator) + "/usr/bin:/bin"
	if path := environmentValue(got, "PATH"); path != wantPath {
		t.Errorf("PATH = %q, want %q", path, wantPath)
	}
	if flag := environmentValue(got, "ANTIGRAVITY_AGENT"); flag != "1" {
		t.Errorf("ANTIGRAVITY_AGENT = %q, want 1", flag)
	}
	if kept := environmentValue(got, "AGY_TEST_KEEP"); kept != "preserved" {
		t.Errorf("AGY_TEST_KEEP = %q, want preserved", kept)
	}
	if original[0] != "PATH="+wantPath {
		t.Errorf("agyLaunchEnv modified input environment: %#v", original)
	}
}

func TestAgyLaunchEnvSetsMarkerAndPrependsShimToOriginalPath(t *testing.T) {
	home := t.TempDir()
	shimDir := filepath.Join(home, ".harnez", "shims")
	got := AgyLaunchEnv([]string{"PATH=/first:/second", "ANTIGRAVITY_AGENT=0"}, home)
	wantPath := shimDir + string(os.PathListSeparator) + "/first:/second"
	if path := environmentValue(got, "PATH"); path != wantPath {
		t.Errorf("PATH = %q, want %q", path, wantPath)
	}
	if marker := environmentValue(got, "ANTIGRAVITY_AGENT"); marker != "1" {
		t.Errorf("ANTIGRAVITY_AGENT = %q, want 1", marker)
	}
}

func TestAgyEffortMapping(t *testing.T) {
	for _, tc := range []struct{ tier, want string }{{"low", "low"}, {"med", "medium"}, {"high", "high"}} {
		var args []string
		d := AgyDriver{Dir: "/d", Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
			args = gotArgs
			return []byte(`{"status":"SUCCESS","response":"ok"}`), nil
		}}
		if _, err := d.Run(context.Background(), RunOptions{Prompt: "p", Model: Model{Name: "m", Tier: tc.tier}}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(args[4:6], []string{"--effort", tc.want}) {
			t.Fatalf("tier %q: args = %#v, want effort %q", tc.tier, args, tc.want)
		}
	}
}

func TestAgyResumeArgs(t *testing.T) {
	var args []string
	d := AgyDriver{Dir: "/work/dir", Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
		args = gotArgs
		return []byte(`{"conversation_id":"abc","status":"SUCCESS","response":"done"}`), nil
	}}
	r, err := d.Resume(context.Background(), "abc", "continue", Model{Name: "claude-sonnet-4-6", Tier: "high"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--conversation", "abc", "--add-dir", "/work/dir", "--model", "claude-sonnet-4-6", "--effort", "high", "--output-format", "json", "-p", "continue"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
	if r.SessionID != "abc" || r.Response != "done" {
		t.Fatalf("unexpected result %#v", r)
	}
}

func TestAgyParseJSONWithTokens(t *testing.T) {
	d := AgyDriver{Dir: "/d", Command: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"conversation_id":"c1","status":"SUCCESS","response":"PONG\n","duration_seconds":1.7,"num_turns":1,"usage":{"input_tokens":11810,"output_tokens":23,"thinking_tokens":21,"cache_read_tokens":0,"total_tokens":11833}}`), nil
	}}
	r, err := d.Run(context.Background(), RunOptions{Prompt: "ping", Model: Model{Name: "m", Tier: "low"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.SessionID != "c1" || r.Response != "PONG\n" {
		t.Fatalf("unexpected result %#v", r)
	}
	if r.InputTokens != 11810 {
		t.Fatalf("InputTokens = %d, want 11810", r.InputTokens)
	}
	if r.OutputTokens != 23+21 {
		t.Fatalf("OutputTokens = %d, want %d (output+thinking)", r.OutputTokens, 23+21)
	}
	if r.CachedTokens != 0 {
		t.Fatalf("CachedTokens = %d, want 0", r.CachedTokens)
	}
	if r.TokensTurn != r.InputTokens+r.OutputTokens+r.CachedTokens {
		t.Fatalf("TokensTurn = %d, want sum %d", r.TokensTurn, r.InputTokens+r.OutputTokens+r.CachedTokens)
	}
}

func TestAgyNonSuccessStatusIsError(t *testing.T) {
	d := AgyDriver{Dir: "/d", Command: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"status":"ERROR","response":"tool call failed"}`), nil
	}}
	_, err := d.Run(context.Background(), RunOptions{Prompt: "p", Model: Model{Name: "m"}})
	if err == nil || !strings.Contains(err.Error(), "tool call failed") {
		t.Fatalf("error = %v, want status/response included", err)
	}
}

func TestAgyMalformedJSONIsError(t *testing.T) {
	d := AgyDriver{Dir: "/d", Command: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`not json`), nil
	}}
	_, err := d.Run(context.Background(), RunOptions{Prompt: "p", Model: Model{Name: "m"}})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestAgyCompactUsesResumeWithSlashCompact(t *testing.T) {
	var args []string
	d := AgyDriver{Dir: "/d", Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
		args = gotArgs
		return []byte(`{"status":"SUCCESS","response":"ok"}`), nil
	}}
	if _, err := d.Compact(context.Background(), "abc"); err != nil {
		t.Fatal(err)
	}
	if args[len(args)-1] != "/compact" {
		t.Fatalf("args = %#v, want last element /compact", args)
	}
	for _, a := range args {
		if a == "--model" {
			t.Fatalf("args = %#v, --model must be omitted for an empty Model", args)
		}
	}
}

func TestAgyOmitsEffortWhenUnsupported(t *testing.T) {
	var args []string
	d := AgyDriver{Dir: "/d", Command: func(_ context.Context, _ string, gotArgs ...string) ([]byte, error) {
		args = gotArgs
		return []byte(`{"status":"SUCCESS","response":"ok"}`), nil
	}}
	m := Model{Name: "claude-sonnet-4-6", Tier: "low", Effort: boolPtr(false)}
	if _, err := d.Run(context.Background(), RunOptions{Prompt: "p", Model: m}); err != nil {
		t.Fatal(err)
	}
	want := []string{"--add-dir", "/d", "--model", "claude-sonnet-4-6", "--output-format", "json", "-p", "p"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestAgyExitOneParsesStdoutErrorJSON(t *testing.T) {
	d := AgyDriver{Dir: "/d", Command: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"status":"ERROR","error":"--effort is not supported for model claude-sonnet-4-6"}`), &exec.ExitError{}
	}}
	_, err := d.Run(context.Background(), RunOptions{Prompt: "p", Model: Model{Name: "claude-sonnet-4-6"}})
	if err == nil || !strings.Contains(err.Error(), "--effort is not supported") {
		t.Fatalf("error = %v, want the stdout error field surfaced", err)
	}
}

func TestAgyRunRefusesEmptyDir(t *testing.T) {
	d := AgyDriver{Command: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("command must not run with an empty Dir")
		return nil, nil
	}}
	if _, err := d.Run(context.Background(), RunOptions{Prompt: "p", Model: Model{Name: "m"}}); err == nil {
		t.Fatal("expected error for empty Dir")
	}
}

func TestAgyResumeRefusesEmptyDir(t *testing.T) {
	d := AgyDriver{Command: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("command must not run with an empty Dir")
		return nil, nil
	}}
	if _, err := d.Resume(context.Background(), "id", "p", Model{Name: "m"}); err == nil {
		t.Fatal("expected error for empty Dir")
	}
}

func TestAgyStopAndDeleteAreNoop(t *testing.T) {
	d := AgyDriver{}
	if err := d.Stop(context.Background(), "id"); err != nil {
		t.Fatalf("Stop returned %v, want nil", err)
	}
	if err := d.Delete(context.Background(), "id"); err != nil {
		t.Fatalf("Delete returned %v, want nil", err)
	}
}

var _ Driver = AgyDriver{}
