package subagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestClaudeResumeStreamSurfacesFinalErrorLine(t *testing.T) {
	const final = `{"type":"result","subtype":"error","is_error":true,"result":"You've hit your session limit · resets 12:20am"}`
	d := ClaudeStreamDriver{ClaudeDriver{Command: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(final + "\n"), errors.New("exit status 1")
	}}}
	_, err := d.ResumeStream(context.Background(), "session", "continue", Model{}, nil)
	if err == nil || !strings.Contains(err.Error(), "session limit") {
		t.Fatalf("error = %v, want final provider limit reason", err)
	}
}

// fakeClaudeStream is realistic `claude -p --output-format stream-json
// --verbose` output, with one malformed line. The script holds the result
// line back until $FAKE_GATE exists, so a test can prove events arrive while
// the process is still running.
const fakeClaudeStream = `{"type":"system","subtype":"init","session_id":"sess-1","model":"claude-sonnet","tools":["Bash","Read"]}
{"type":"assistant", oops}
{"type":"assistant","parent_tool_use_id":null,"session_id":"sess-1","message":{"role":"assistant","content":[{"type":"text","text":"Looking at the repo."}],"usage":{"input_tokens":5,"cache_read_input_tokens":100,"cache_creation_input_tokens":20,"output_tokens":3}}}
{"type":"assistant","parent_tool_use_id":null,"session_id":"sess-1","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"ls -la\necho two","description":"List files"}}],"usage":{"input_tokens":5,"cache_read_input_tokens":120,"cache_creation_input_tokens":0,"output_tokens":9}}}
{"type":"user","session_id":"sess-1","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"go.mod"}]}}
{"type":"assistant","parent_tool_use_id":"toolu_9","session_id":"sess-1","message":{"role":"assistant","content":[{"type":"text","text":"subagent noise"}]}}
{"type":"assistant","parent_tool_use_id":null,"session_id":"sess-1","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_2","name":"Read","input":{"file_path":"/repo/go.mod"}}]}}
{"type":"assistant","parent_tool_use_id":null,"session_id":"sess-1","message":{"role":"assistant","content":[{"type":"text","text":"Done: one module."}],"usage":{"input_tokens":4,"cache_read_input_tokens":200,"cache_creation_input_tokens":10,"output_tokens":5}}}
`

const fakeClaudeResult = `{"type":"result","subtype":"success","is_error":false,"session_id":"sess-1","result":"Done: one module.","usage":{"input_tokens":14,"cache_read_input_tokens":420,"cache_creation_input_tokens":30,"output_tokens":17}}`

// installFakeClaude puts a `claude` script on PATH that records its args to
// $FAKE_ARGS, prints stream, waits for $FAKE_GATE (when set), then prints tail.
func installFakeClaude(t *testing.T, stream, tail string) (argsFile string) {
	t.Helper()
	return installFakeCLI(t, "claude", stream, tail)
}

// installFakeCLI is installFakeClaude for any provider binary name.
func installFakeCLI(t *testing.T, name, stream, tail string) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stream.jsonl"), []byte(stream), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tail.sh"), []byte(tail), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > \"$FAKE_ARGS\"\n" +
		"cat \"" + filepath.Join(dir, "stream.jsonl") + "\"\n" +
		"if test -n \"$FAKE_GATE\"\nthen\n  while ! test -e \"$FAKE_GATE\"\n  do\n    sleep 0.01\n  done\nfi\n" +
		". \"" + filepath.Join(dir, "tail.sh") + "\"\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	argsFile = filepath.Join(dir, "args")
	t.Setenv("FAKE_ARGS", argsFile)
	t.Setenv("FAKE_GATE", "")
	return argsFile
}

func readArgs(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

func TestClaudeStreamDriverEmitsEventsLive(t *testing.T) {
	argsFile := installFakeClaude(t, fakeClaudeStream, "echo '"+fakeClaudeResult+"'\n")
	gate := filepath.Join(t.TempDir(), "gate")
	t.Setenv("FAKE_GATE", gate)
	var got []string
	d := StreamDriverFor(Model{Provider: "claude", Name: "sonnet", Tier: "med"}, t.TempDir())
	sd, ok := d.(StreamingDriver)
	if !ok {
		t.Fatalf("StreamDriverFor(claude) = %T, want a StreamingDriver", d)
	}
	r, err := sd.RunStream(context.Background(), RunOptions{Prompt: "hi", Model: Model{Provider: "claude", Name: "sonnet", Tier: "med"}}, func(ev Event) {
		got = append(got, ev.Kind+":"+ev.Text)
		if ev.Kind == "message" && ev.Text == "Done: one module." {
			// The result line is still held back: this event is live.
			if err := os.WriteFile(gate, nil, 0o644); err != nil {
				t.Error(err)
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"session:sess-1",
		"message:Looking at the repo.", "tokens:",
		"activity:running ls -la", "tokens:",
		"other:", "other:",
		"activity:Read /repo/go.mod",
		"message:Done: one module.", "tokens:",
		"other:",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events:\n got %q\nwant %q", got, want)
	}
	if r.SessionID != "sess-1" || r.Response != "Done: one module." || r.ContextTokens != 214 {
		t.Fatalf("result = %+v", r)
	}
	if args := readArgs(t, argsFile); !reflect.DeepEqual(args, []string{"-p", "--dangerously-skip-permissions", "--model", "sonnet", "--effort", "medium", "--output-format", "stream-json", "--verbose", "hi"}) {
		t.Fatalf("args = %q", args)
	}
}

// TestClaudeBatchDriverUnchanged pins the non-streaming path used by
// harnez agent start/resume: DriverFor's claude driver is not a
// StreamingDriver, runs the same arguments, and returns the same result
// as the streaming driver for the same output.
func TestClaudeBatchDriverUnchanged(t *testing.T) {
	argsFile := installFakeClaude(t, fakeClaudeStream, "echo '"+fakeClaudeResult+"'\n")
	m := Model{Provider: "claude", Name: "sonnet", Tier: "low"}
	d := DriverFor(m, t.TempDir())
	if _, ok := d.(StreamingDriver); ok {
		t.Fatalf("DriverFor(claude) = %T must stay a batch driver", d)
	}
	batch, err := d.Resume(context.Background(), "sess-0", "next", m)
	if err != nil {
		t.Fatal(err)
	}
	batchArgs := readArgs(t, argsFile)
	if want := []string{"-p", "--resume", "sess-0", "--dangerously-skip-permissions", "--model", "sonnet", "--effort", "low", "--output-format", "stream-json", "--verbose", "next"}; !reflect.DeepEqual(batchArgs, want) {
		t.Fatalf("batch args = %q, want %q", batchArgs, want)
	}
	stream, err := ClaudeStreamDriver{d.(ClaudeDriver)}.ResumeStream(context.Background(), "sess-0", "next", m, func(Event) {})
	if err != nil {
		t.Fatal(err)
	}
	if args := readArgs(t, argsFile); !reflect.DeepEqual(args, batchArgs) {
		t.Fatalf("stream args = %q, batch args %q", args, batchArgs)
	}
	direct, err := parseClaude([]byte(fakeClaudeStream + fakeClaudeResult + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(batch, direct) || !reflect.DeepEqual(stream, direct) {
		t.Fatalf("results differ:\nbatch  %+v\nstream %+v\nparse  %+v", batch, stream, direct)
	}
}

func TestClaudeStreamCancelKillsProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	installFakeClaude(t, `{"type":"system","subtype":"init","session_id":"sess-c"}`+"\n",
		"sleep 30 &\necho $! > \""+pidFile+"\"\nwait\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var session string
	done := make(chan error, 1)
	go func() {
		_, err := ClaudeStreamDriver{ClaudeDriver{Dir: t.TempDir()}}.RunStream(ctx, RunOptions{Prompt: "p"}, func(ev Event) {
			if ev.Kind == "session" {
				session = ev.Text
				for i := 0; i < 200 && !fileExists(pidFile); i++ {
					time.Sleep(10 * time.Millisecond)
				}
				cancel()
			}
		})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled turn returned no error")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancelled turn did not return")
	}
	if session != "sess-c" {
		t.Fatalf("session = %q", session)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		if !processAlive(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("provider grandchild %d survived cancel", pid)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// processAlive treats a zombie (killed, not yet reaped by its new parent) as dead.
func processAlive(pid int) bool {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	_, rest, _ := strings.Cut(string(b), ") ")
	return !strings.HasPrefix(rest, "Z") && !strings.HasPrefix(rest, "X")
}

func TestPiStreamDriverEmitsSessionAndMessages(t *testing.T) {
	head := `{"type":"session","id":"pi-1"}
{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"first"}],"stopReason":"toolUse"}}
`
	tail := `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"final"}],"stopReason":"stop"}}
{"type":"agent_settled"}
`
	installFakeCLI(t, "pi", head, "printf '%s' '"+tail+"'\n")
	gate := filepath.Join(t.TempDir(), "gate")
	t.Setenv("FAKE_GATE", gate)
	d := StreamDriverFor(Model{Provider: "pi", Name: "default", Tier: "low"}, t.TempDir())
	if _, ok := d.(PiStreamDriver); !ok {
		t.Fatalf("StreamDriverFor(pi) = %T", d)
	}
	var got []string
	r, err := d.(StreamingDriver).RunStream(context.Background(), RunOptions{Prompt: "p"}, func(ev Event) {
		got = append(got, ev.Kind+":"+ev.Text)
		if ev.Kind == "message" && ev.Text == "first" {
			// The script is still blocked on the gate: this event is live.
			if err := os.WriteFile(gate, nil, 0o644); err != nil {
				t.Error(err)
			}
		}
	})
	if err != nil || r.SessionID != "pi-1" || r.Response != "final" {
		t.Fatalf("RunStream = %+v, %v", r, err)
	}
	want := []string{"session:pi-1", "message:first", "message:final", "other:"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %q, want %q", got, want)
	}
}

func TestReadLinesDeliversOversizedLineIntact(t *testing.T) {
	big := `{"type":"user","content":"` + strings.Repeat("x", 200*1024) + `"}`
	in := `{"type":"a"}` + "\n" + big + "\n" + `{"type":"b"}`
	var got []string
	all, err := readLines(strings.NewReader(in), func(l []byte) { got = append(got, string(l)) })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != `{"type":"a"}` || got[1] != big || got[2] != `{"type":"b"}` {
		t.Fatalf("got %d lines, big intact = %v", len(got), len(got) > 1 && got[1] == big)
	}
	if string(all) != in {
		t.Fatalf("collected %d bytes, want %d", len(all), len(in))
	}
}
