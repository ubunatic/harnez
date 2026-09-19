package bench

import (
	"context"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const claudeJSON = `{"result":"ready","is_error":false,"total_cost_usd":0.03,"usage":{"input_tokens":10,"output_tokens":48,"cache_read_input_tokens":5,"cache_creation_input_tokens":100}}`

const codexJSONL = `Reading additional input from stdin...
{"type":"thread.started","thread_id":"x"}
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"draft"}}
{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":5}}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"ready"}}
{"type":"turn.completed","usage":{"input_tokens":200,"output_tokens":7}}
`

func TestEmbeddedSpecIsValid(t *testing.T) {
	s, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Tasks) < 5 || len(s.BaseDocs) == 0 {
		t.Fatalf("spec too small: %d tasks, %d base docs", len(s.Tasks), len(s.BaseDocs))
	}
	root := repoRoot(t)
	for _, task := range s.Tasks {
		for _, cond := range []string{"full", "lite"} {
			for _, rel := range append(append([]string{}, s.BaseDocs...), task.Docs...) {
				if _, err := docPath(root, rel, cond); err != nil {
					t.Errorf("task %s (%s): %v", task.ID, cond, err)
				}
			}
		}
	}
}

func TestParseSpecRejectsBadInput(t *testing.T) {
	for name, y := range map[string]string{
		"duplicate": "tasks:\n- {id: a, prompt: p, pattern: x}\n- {id: a, prompt: p, pattern: x}\n",
		"no check":  "tasks:\n- {id: a, prompt: p}\n",
		"bad regex": "tasks:\n- {id: a, prompt: p, pattern: '(?!x)'}\n",
		"no prompt": "tasks:\n- {id: a, pattern: x}\n",
	} {
		if _, err := ParseSpec([]byte(y)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestSelectAndScore(t *testing.T) {
	s, _ := ParseSpec([]byte("tasks:\n- {id: a, prompt: p, pattern: 'if test', forbid_pattern: '\\[\\['}\n- {id: b, prompt: p, pattern: x}\n"))
	if got, err := s.Select([]string{"b"}); err != nil || len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("Select = %v, %v", got, err)
	}
	if _, err := s.Select([]string{"zzz"}); err == nil {
		t.Fatal("unknown id accepted")
	}
	a := s.Tasks[0]
	for resp, want := range map[string]bool{"use IF TEST -f x": true, "use [[ -f x ]]": false, "if test and [[": false, "": false, "nothing": false} {
		if got, _ := a.Score(resp); got != want {
			t.Errorf("Score(%q) = %v, want %v", resp, got, want)
		}
	}
}

func TestParseClaudeAndCodex(t *testing.T) {
	c, err := ParseClaude([]byte(claudeJSON))
	if err != nil || c.Text != "ready" || c.InputTokens != 115 || c.OutputTokens != 48 || c.CostUSD != 0.03 {
		t.Fatalf("claude = %+v, %v", c, err)
	}
	if _, err := ParseClaude([]byte(`{"result":"boom","is_error":true}`)); err == nil {
		t.Fatal("is_error accepted")
	}
	x, err := ParseCodex([]byte(codexJSONL))
	if err != nil || x.Text != "ready" || x.InputTokens != 300 || x.OutputTokens != 12 {
		t.Fatalf("codex = %+v, %v", x, err)
	}
	if _, err := ParseCodex([]byte(`{"type":"turn.started"}`)); err == nil {
		t.Fatal("codex output without message accepted")
	}
}

func TestInvokeBuildsAgentCommands(t *testing.T) {
	var name string
	var args []string
	fake := func(out string) CommandRunner {
		return func(_ context.Context, _, n string, a ...string) ([]byte, error) {
			name, args = n, a
			return []byte(out), nil
		}
	}
	if _, err := Invoke(context.Background(), fake(claudeJSON), AgentClaude, "", t.TempDir(), "hi"); err != nil {
		t.Fatal(err)
	}
	if name != "claude" || !contains(args, "--model", "haiku") || args[len(args)-1] != "hi" {
		t.Errorf("claude cmd = %s %v", name, args)
	}
	if _, err := Invoke(context.Background(), fake(codexJSONL), AgentCodex, "luna", t.TempDir(), "hi"); err != nil {
		t.Fatal(err)
	}
	if name != "codex" || !contains(args, "-m", "gpt-5.6-luna") || args[0] != "exec" {
		t.Errorf("codex cmd = %s %v", name, args)
	}
	if _, err := Invoke(context.Background(), fake(""), "agy", "", t.TempDir(), "hi"); err == nil {
		t.Error("unsupported agent accepted")
	}
}

func contains(args []string, flag, val string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == val {
			return true
		}
	}
	return false
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestStageWorkspaceMarkdownAndCards(t *testing.T) {
	spec := &Spec{Preamble: "READ THESE", BaseDocs: []string{"docs/practices/AgenticLoop.md"}}
	task := Task{ID: "t", Docs: []string{"docs/lang/Bash.md"}}
	root := repoRoot(t)

	md := t.TempDir()
	names, err := StageWorkspace(md, root, spec, task, Condition{Docs: "lite"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, ",") != "AgenticLoop.lite.md,Bash.lite.md" {
		t.Fatalf("lite docs = %v", names)
	}
	agents, _ := os.ReadFile(filepath.Join(md, "AGENTS.md"))
	if !strings.Contains(string(agents), "@Bash.lite.md") || !strings.HasPrefix(string(agents), "READ THESE") {
		t.Errorf("AGENTS.md = %q", agents)
	}
	if _, err := os.Stat(filepath.Join(md, "CLAUDE.md")); err != nil {
		t.Error("CLAUDE.md missing")
	}

	cards := t.TempDir()
	names, err = StageWorkspace(cards, root, spec, task, Condition{Docs: "full", Cards: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(names) < 2 || !strings.HasSuffix(names[len(names)-1], ".png") {
		t.Fatalf("card names = %v", names)
	}
	f, err := os.Open(filepath.Join(cards, names[len(names)-1]))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil || img.Bounds().Dx() < 100 {
		t.Fatalf("card invalid: %v %v", err, img)
	}
	if _, err := os.Stat(filepath.Join(cards, "Bash.md")); err == nil {
		t.Error("card condition also delivered Markdown")
	}
}

func TestStoreRunTasksAndSummaries(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "sub", "bench.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	spec := &Spec{Preamble: "p", BaseDocs: []string{"docs/practices/AgenticLoop.md"}, Tasks: []Task{
		{ID: "ok", Prompt: "x", Pattern: "ready"},
		{ID: "wrong", Prompt: "x", Pattern: "nomatch"},
		{ID: "broken", Prompt: "x", Pattern: "ready"},
	}}
	fake := func(_ context.Context, _, _ string, a ...string) ([]byte, error) {
		if a[len(a)-1] == "boom" {
			return nil, errors.New("exit 1")
		}
		return []byte(claudeJSON), nil
	}
	spec.Tasks[2].Prompt = "boom"
	var seen int
	err = RunTasks(context.Background(), store, spec, spec.Tasks, Options{
		Agent: AgentClaude, Cond: Condition{Docs: "full"}, RepoRoot: repoRoot(t), Repeat: 2, Run: fake,
	}, func(Run) { seen++ })
	if err != nil {
		t.Fatal(err)
	}
	if seen != 6 {
		t.Fatalf("progress calls = %d, want 6", seen)
	}
	sums, err := store.Summaries()
	if err != nil || len(sums) != 1 {
		t.Fatalf("summaries = %+v, %v", sums, err)
	}
	s := sums[0]
	if s.Runs != 4 || s.Passes != 2 || s.Errors != 2 || s.Model != "haiku" || s.AvgInput != 115 {
		t.Errorf("summary = %+v", s)
	}
	recent, _ := store.Recent(10)
	if len(recent) != 6 || recent[0].Task != "broken" || recent[0].Error == "" {
		t.Errorf("recent[0] = %+v", recent[0])
	}
}

func TestSetupGatesBench(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bench")
	if err := Ready(dir); !errors.Is(err, ErrNotSetUp) {
		t.Fatalf("Ready before setup = %v", err)
	}
	look := func(n string) (string, error) {
		if n == AgentClaude {
			return "/bin/claude", nil
		}
		return "", errors.New("missing")
	}
	found, err := Setup(dir, look)
	if err != nil || found[AgentClaude] == "" || found[AgentCodex] != "" {
		t.Fatalf("Setup = %v, %v", found, err)
	}
	if err := Ready(dir); err != nil {
		t.Fatalf("Ready after setup = %v", err)
	}
	if _, err := os.Stat(DBPath(dir)); err != nil {
		t.Error("bench db not created")
	}
	if _, err := Setup(dir, look); err != nil {
		t.Errorf("Setup not idempotent: %v", err)
	}
}
