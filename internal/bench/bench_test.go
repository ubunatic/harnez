package bench

import (
	"context"
	"database/sql"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez/internal/agymeter"
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

func TestReadLangSummaryScoringAndSavedSessions(t *testing.T) {
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := spec.Select([]string{"read-lang-summary"})
	if err != nil {
		t.Fatal(err)
	}
	task := tasks[0]
	good := `| Doc | Summary |
|---|---|
| Go | standard library |
| Make | help first |
| ManPages | Cobra |
| Bash | if test |
| Git | Conventional Commit |
| Markdown | PascalCase |`
	if pass, detail := task.Score(good); !pass {
		t.Fatalf("complete answer rejected: %s", detail)
	}
	if task.AllowMissing != 2 {
		t.Fatalf("allow_missing = %d, want 2", task.AllowMissing)
	}
	twoMissing := strings.NewReplacer("PascalCase", "kebab-case", "Cobra", "roff").Replace(good)
	if pass, detail := task.Score(twoMissing); !pass || !strings.Contains(detail, "PascalCase") || !strings.Contains(detail, "Cobra") {
		t.Errorf("two missing keywords result = %v, %q (want pass, both named)", pass, detail)
	}
	threeMissing := strings.NewReplacer("PascalCase", "kebab-case", "Cobra", "roff", "if test", "brackets").Replace(good)
	if pass, detail := task.Score(threeMissing); pass || !strings.Contains(detail, "if test") {
		t.Errorf("three missing keywords result = %v, %q (want fail)", pass, detail)
	}
	// Wordings from real passing-quality answers that earlier checks rejected.
	for _, variant := range []string{
		strings.Replace(good, "standard library", "stdlib first", 1),
		strings.Replace(good, "help first", "`help` the default target", 1),
		strings.Replace(good, "|---|---|", "| :--- | :--- |", 1),
	} {
		if pass, detail := task.Score(variant); !pass {
			t.Errorf("valid wording rejected: %s\n%s", detail, variant)
		}
	}
	noTable := strings.Replace(good, "|---|---|", "separator absent", 1)
	if pass, detail := task.Score(noTable); pass || !strings.Contains(detail, "pattern") {
		t.Errorf("missing table result = %v, %q", pass, detail)
	}

	sessions, err := filepath.Glob(filepath.Join(repoRoot(t), "docs/data/codex-bench-read-lang-docs-*-luna-med-pass-*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 6 {
		t.Fatalf("found %d saved sessions, want 6", len(sessions))
	}
	for _, path := range sessions {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(string(data), "version https://git-lfs.github.com/spec/v1") {
			t.Skipf("%s is a Git LFS pointer; run git lfs pull to validate saved-session answers", filepath.Base(path))
		}
		at := strings.LastIndex(string(data), "## Assistant\n")
		if at < 0 {
			t.Errorf("%s has no final assistant answer", filepath.Base(path))
			continue
		}
		if pass, detail := task.Score(string(data[at+len("## Assistant\n"):])); !pass {
			t.Errorf("%s: %s", filepath.Base(path), detail)
		}
	}
}

func TestReadPromptByMode(t *testing.T) {
	task := Task{Prompt: "default", ReadPrompts: map[string]string{"native": "native prompt"}}
	if got := TaskPrompt(task, "native"); got != "native prompt" {
		t.Fatalf("native prompt = %q", got)
	}
	if got := TaskPrompt(task, "card"); got != "default" {
		t.Fatalf("fallback prompt = %q", got)
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
	if err != nil || c.Text != "ready" || c.InputTokens != 115 || c.CachedInputTokens == nil || *c.CachedInputTokens != 105 || c.OutputTokens != 48 || c.CostUSD != 0.03 {
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

const agyJSON = `{"conversation_id":"c1","status":"SUCCESS","response":"ready\n","duration_seconds":1.5,"num_turns":3,"usage":{"input_tokens":100,"output_tokens":20,"thinking_tokens":5,"cache_read_tokens":40,"total_tokens":165}}`

func TestParseAgy(t *testing.T) {
	r, err := ParseAgy([]byte(agyJSON))
	if err != nil || r.Text != "ready\n" || r.InputTokens != 140 || r.CachedInputTokens == nil || *r.CachedInputTokens != 40 || r.OutputTokens != 25 || r.Turns != 3 {
		t.Fatalf("agy = %+v, %v", r, err)
	}
	if _, err := ParseAgy([]byte(`{"status":"ERROR","response":"boom"}`)); err == nil {
		t.Fatal("non-SUCCESS status accepted")
	}
	if _, err := ParseAgy([]byte("not json")); err == nil {
		t.Fatal("garbage accepted")
	}
}

func TestReadOrderPromptAndParse(t *testing.T) {
	task := Task{Prompt: "summarise", ReadPrompts: map[string]string{"native": "read docs"}, ReadOrders: map[string]string{"batch": "Read all files in one batch."}}
	if got := TaskPrompt(task, "native", "batch"); got != "read docs Read all files in one batch." {
		t.Fatalf("prompt = %q", got)
	}
	if got := TaskPrompt(task, "native"); got != "read docs" {
		t.Fatalf("default prompt = %q", got)
	}
	orders, err := ParseOrderList(" batch, sequential ")
	if err != nil || strings.Join(orders, ",") != "batch,sequential" {
		t.Fatalf("orders = %v, %v", orders, err)
	}
	if _, err := ParseOrderList("bad"); err == nil {
		t.Fatal("bad order accepted")
	}
	if got, ok := estimateCached([]int64{11, 25, 41}); !ok || got != 36 {
		t.Fatalf("estimated cache = %d, %v", got, ok)
	}
	if _, ok := estimateCached([]int64{11}); ok {
		t.Fatal("single call cache should be unknown")
	}
	spec, err := ParseSpec([]byte("tasks:\n- id: read\n  prompt: read\n  fixtures: [A.md]\n  pattern: x\n  read_orders: {batch: 'all at once', sequential: 'one by one'}\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, order := range []string{"batch", "sequential"} {
		selected, err := spec.SelectFor(nil, Condition{Read: "native", Order: order})
		if err != nil || len(selected) != 1 || TaskPrompt(selected[0], "native", order) != "read "+selected[0].ReadOrders[order] {
			t.Fatalf("order %s matrix = %v, %v", order, selected, err)
		}
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
	if _, err := Invoke(context.Background(), fake(claudeJSON), AgentClaude, "claude:haiku:low", t.TempDir(), "hi"); err != nil {
		t.Fatal(err)
	}
	if name != "claude" || !contains(args, "--model", "haiku") || args[len(args)-1] != "hi" {
		t.Errorf("claude cmd = %s %v", name, args)
	}
	if _, err := Invoke(context.Background(), fake(codexJSONL), AgentCodex, "codex:luna:low", t.TempDir(), "hi"); err != nil {
		t.Fatal(err)
	}
	if name != "codex" || !contains(args, "-m", "gpt-6-luna") {
		t.Errorf("codex default cmd = %s %v", name, args)
	}
	if _, err := Invoke(context.Background(), fake(codexJSONL), AgentCodex, "codex:luna:med", t.TempDir(), "hi"); err != nil {
		t.Fatal(err)
	}
	if name != "codex" || !contains(args, "-m", "gpt-6-luna") || !contains(args, "-c", "model_reasoning_effort=medium") || args[0] != "exec" {
		t.Errorf("codex cmd = %s %v", name, args)
	}
	if _, err := Invoke(context.Background(), fake(agyJSON), AgentAgy, "agy:flash37:low", t.TempDir(), "hi"); err != nil {
		t.Fatal(err)
	}
	if name != "agy" || !contains(args, "--model", "gemini-3.7-flash") || !contains(args, "--effort", "low") || args[1] != "hi" || !contains(args, "--output-format", "json") {
		t.Errorf("agy cmd = %s %v", name, args)
	}
	if _, err := Invoke(context.Background(), fake(""), "gemini", "", t.TempDir(), "hi"); err == nil {
		t.Error("unsupported agent accepted")
	}
}

func TestRunTasksUsesMeterRecordsForAgy(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "bench.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	spec := &Spec{Tasks: []Task{{ID: "smoke", Prompt: "Reply ready", Pattern: "ready"}}}
	var meterSession string
	fakeRunner := func(_ context.Context, _, name string, args ...string) ([]byte, error) {
		if name != "agy" || len(args) < 2 || args[1] != "Reply ready" {
			t.Fatalf("agy call = %s %v", name, args)
		}
		return []byte(agyJSON), nil
	}
	err = RunTasks(context.Background(), store, spec, spec.Tasks, Options{
		Agent: AgentAgy, RepoRoot: t.TempDir(), Run: fakeRunner,
		ReadMeter: func(sessionID string) ([]agymeter.Record, error) {
			meterSession = sessionID
			return []agymeter.Record{{Prompt: 30, Total: 50}, {Prompt: 8, Total: 10}}, nil
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := store.Recent(1)
	if err != nil || len(runs) != 1 {
		t.Fatalf("recent runs = %v, %v", runs, err)
	}
	r := runs[0]
	if meterSession == "" || r.SessionID != meterSession || r.InputTokens != 38 || r.TotalTokens != 60 || r.OutputTokens != 22 || r.Turns != 2 || !r.Pass {
		t.Fatalf("metered run = %+v; meter session %q", r, meterSession)
	}
}

func TestSplitAgyUsageMainAndHelpers(t *testing.T) {
	rows := []agymeter.Record{
		{Model: "main", Prompt: 100, Total: 110},
		{Model: "helper", Prompt: 12, Total: 13},
		{Model: "main", Prompt: 80, Total: 90},
		{Model: "helper", Prompt: 9, Total: 10},
	}
	in, total, callsN, calls, helpers := splitAgyUsage(rows)
	if in != 180 || total != 200 || callsN != 2 || len(calls) != 2 || calls[0] != 100 || calls[1] != 80 {
		t.Fatalf("main usage = %d/%d calls=%d series=%v", in, total, callsN, calls)
	}
	if len(helpers) != 1 || helpers[0] != (HelperUsage{Model: "helper", Calls: 2, InputTokens: 21, TotalTokens: 23}) {
		t.Fatalf("helpers = %+v", helpers)
	}
}

func TestSplitAgyUsageSingleAndHelperOnly(t *testing.T) {
	for _, rows := range [][]agymeter.Record{
		{{Model: "only", Prompt: 40, Total: 44}},
		{{Model: "helper", Prompt: 8, Total: 9}},
	} {
		in, total, turns, calls, helpers := splitAgyUsage(rows)
		if in != int(rows[0].Prompt) || total != int(rows[0].Total) || turns != 1 || len(calls) != 1 || len(helpers) != 0 {
			t.Fatalf("single-model split = %d/%d/%d %v %v", in, total, turns, calls, helpers)
		}
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
	if s.Runs != 4 || s.Passes != 2 || s.Errors != 2 || s.Model != "claude:haiku:low" || s.AvgInput != 115 {
		t.Errorf("summary = %+v", s)
	}
	recent, _ := store.Recent(10)
	if len(recent) != 6 || recent[0].Task != "broken" || recent[0].Error == "" {
		t.Errorf("recent[0] = %+v", recent[0])
	}
	if recent[2].CachedInputTokens == nil || *recent[2].CachedInputTokens != 105 || recent[2].CachedEstimated {
		t.Errorf("stored cache metadata = %+v", recent[2])
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

func TestFixtureHoldsTheAnswersToReadTasks(t *testing.T) {
	s, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := fixtureDoc("RUNBOOK.md")
	if n := strings.Count(doc, "\n"); n < 500 {
		t.Fatalf("fixture has %d lines, want a large doc", n)
	}
	// Each needle appears exactly once, so a correct answer is unambiguous.
	for _, needle := range []string{"retry limit: 17 attempts", "port: 7431\n", "Team Bramble"} {
		if c := strings.Count(doc, needle); c != 1 {
			t.Errorf("%q appears %d times, want 1", needle, c)
		}
	}
	cases := map[string][2]string{
		"read-one-fact": {"17", "3"},
		"read-two-hop":  {"tarnwick Bramble", "brindle Heron"},
	}
	for id, c := range cases {
		task, err := s.Select([]string{id})
		if err != nil {
			t.Fatal(err)
		}
		if ok, d := task[0].Score(c[0]); !ok {
			t.Errorf("%s rejects the right answer: %s", id, d)
		}
		if ok, _ := task[0].Score(c[1]); ok {
			t.Errorf("%s accepts a wrong answer %q", id, c[1])
		}
	}
}

func TestReadConditionSelectsAndStagesOnlyFixtures(t *testing.T) {
	s, _ := LoadSpec()
	for _, mode := range ReadModes {
		cond := Condition{Docs: "full", Read: mode}
		tasks, err := s.SelectFor(nil, cond)
		want := 3
		if mode == "auto" {
			want = 2
		}
		if err != nil || len(tasks) != want {
			t.Fatalf("%s: SelectFor = %d tasks, want %d, %v", mode, len(tasks), want, err)
		}
		dir := t.TempDir()
		got, err := StageWorkspace(dir, repoRoot(t), s, tasks[0], cond)
		if err != nil || len(got) != 1 || got[0] != "docs/RUNBOOK.md" {
			t.Fatalf("%s: staged %v, %v", mode, got, err)
		}
		agents, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
		if !strings.Contains(string(agents), s.ReadModes[mode][:20]) || !strings.Contains(string(agents), "docs/RUNBOOK.md") {
			t.Errorf("%s: AGENTS.md lacks the read instruction or doc list:\n%s", mode, agents)
		}
		if strings.Contains(string(agents), "AgenticLoop") {
			t.Errorf("%s: read workspace leaks project docs", mode)
		}
		if mode == "card" && !strings.Contains(string(agents), "harnez read -I") {
			t.Errorf("card mode does not force image output:\n%s", agents)
		}
		if cond.Label() != "read:"+mode {
			t.Errorf("label = %q", cond.Label())
		}
	}
	docsTasks, _ := s.SelectFor(nil, Condition{Docs: "full"})
	for _, task := range docsTasks {
		if len(task.Fixtures) > 0 {
			t.Errorf("docs condition selected fixture task %s", task.ID)
		}
	}
	if _, err := s.SelectFor([]string{"hello"}, Condition{Read: "text"}); err == nil {
		t.Error("explicit docs task under a read condition should error")
	}
	if _, err := ParseRead("bogus"); err == nil {
		t.Error("ParseRead accepted bogus")
	}
}

func TestParseReadAcceptsCard(t *testing.T) {
	if got, err := ParseRead("card"); err != nil || got != "card" {
		t.Fatalf("ParseRead(card) = %q, %v", got, err)
	}
}

func TestTurnsAreParsedAndStored(t *testing.T) {
	res, err := ParseClaude([]byte(`{"result":"x","num_turns":4,"usage":{}}`))
	if err != nil || res.Turns != 4 {
		t.Fatalf("claude turns = %d, %v", res.Turns, err)
	}
	codex := `{"type":"item.completed","item":{"type":"reasoning","text":"r"}}
{"type":"item.completed","item":{"type":"command_execution"}}
{"type":"item.completed","item":{"type":"command_execution"}}
{"type":"item.completed","item":{"type":"agent_message","text":"17"}}
{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}
`
	if res, err = ParseCodex([]byte(codex)); err != nil || res.Turns != 3 {
		t.Fatalf("codex turns = %d, %v", res.Turns, err)
	}
	st, err := OpenStore(filepath.Join(t.TempDir(), "b.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, turns := range []int{2, 4} {
		if err := st.Insert(Run{Task: "read-one-fact", Agent: "codex", Model: "m", Docs: "full", ReadMode: "auto", Turns: turns, Pass: true}); err != nil {
			t.Fatal(err)
		}
	}
	sums, err := st.Summaries()
	if err != nil || len(sums) != 1 || sums[0].Read != "auto" || sums[0].AvgTurns != 3 {
		t.Fatalf("summaries = %+v, %v", sums, err)
	}
}

func TestOpenStoreMigratesPreReadModeDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE runs (id INTEGER PRIMARY KEY AUTOINCREMENT, ts TEXT NOT NULL, task TEXT NOT NULL, agent TEXT NOT NULL, model TEXT NOT NULL, docs TEXT NOT NULL, cards INTEGER NOT NULL, pass INTEGER NOT NULL, detail TEXT NOT NULL DEFAULT '', input_tokens INTEGER NOT NULL DEFAULT 0, output_tokens INTEGER NOT NULL DEFAULT 0, cost_usd REAL NOT NULL DEFAULT 0, duration_ms INTEGER NOT NULL DEFAULT 0, response TEXT NOT NULL DEFAULT '', error TEXT NOT NULL DEFAULT '')`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // second open proves the migration is idempotent
		st, err := OpenStore(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		if err := st.Insert(Run{Task: "t", Agent: "a", Model: "m", Docs: "full", ReadMode: "text", Turns: 1}); err != nil {
			t.Fatal(err)
		}
		st.Close()
	}
}

func TestFixtureYamlParsesAndMatchesMarkdown(t *testing.T) {
	files, ok := fixtureFiles("RUNBOOK.md", true, 0)
	if !ok || len(files) != 1 || files[0].Name != "RUNBOOK.yaml" {
		t.Fatalf("yaml files = %v", files)
	}
	var doc struct {
		Services map[string]map[string]any `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(files[0].Content), &doc); err != nil {
		t.Fatalf("fixture yaml does not parse: %v", err)
	}
	if len(doc.Services) != len(fixtureNames) {
		t.Fatalf("yaml has %d services, want %d", len(doc.Services), len(fixtureNames))
	}
	if got := doc.Services[needleService]["retry_limit"]; got != "17 attempts before the message is parked" {
		t.Errorf("quillfox retry_limit = %v", got)
	}
	if got := doc.Services[needlePortSvc]["port"]; got != needlePort {
		t.Errorf("tarnwick port = %v (%T)", got, got)
	}
}

func TestFixtureMultiSplitsByLetterGroups(t *testing.T) {
	whole, _ := fixtureDoc("RUNBOOK.md")
	for _, yamlOut := range []bool{false, true} {
		files, _ := fixtureFiles("RUNBOOK.md", yamlOut, 5)
		if len(files) != 5 {
			t.Fatalf("multi 5 gave %d files: %v", len(files), files)
		}
		ext := map[bool]string{false: ".md", true: ".yaml"}[yamlOut]
		want := []string{"RUNBOOK-a-f", "RUNBOOK-g-l", "RUNBOOK-m-r", "RUNBOOK-s-x", "RUNBOOK-y-z"}
		services := 0
		for i, f := range files {
			if f.Name != want[i]+ext {
				t.Errorf("file %d = %s, want %s%s", i, f.Name, want[i], ext)
			}
			services += strings.Count(f.Content, "ledger\n") + strings.Count(f.Content, "ledger\"\n")
		}
		if services != len(fixtureNames) {
			t.Errorf("split files hold %d services, want %d", services, len(fixtureNames))
		}
	}
	files, _ := fixtureFiles("RUNBOOK.md", false, 5)
	var joined strings.Builder
	for _, f := range files {
		joined.WriteString(f.Content)
	}
	for _, needle := range []string{"retry limit: 17 attempts", "port: 7431\n", "Team Bramble"} {
		if strings.Count(joined.String(), needle) != 1 || strings.Count(whole, needle) != 1 {
			t.Errorf("needle %q not exactly once in both whole and split", needle)
		}
	}
	if one, _ := fixtureFiles("RUNBOOK.md", false, 1); len(one) != 1 {
		t.Errorf("multi 1 should stay one file, got %d", len(one))
	}
	if all, _ := fixtureFiles("RUNBOOK.md", false, 26); len(all) != 25 { // no service starts with x
		t.Errorf("multi 26 gave %d files, want 25 (no x services)", len(all))
	}
}

func TestVariantStagingAndLabel(t *testing.T) {
	s, _ := LoadSpec()
	tasks, _ := s.SelectFor(nil, Condition{Read: "text"})
	cond := Condition{Docs: "full", Read: "text", Yaml: true, Multi: 5}
	if cond.Label() != "read:text+yaml+multi5" {
		t.Errorf("label = %q", cond.Label())
	}
	dir := t.TempDir()
	got, err := StageWorkspace(dir, repoRoot(t), s, tasks[0], cond)
	if err != nil || len(got) != 5 || got[0] != "docs/RUNBOOK-a-f.yaml" {
		t.Fatalf("staged %v, %v", got, err)
	}
	agents, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	for _, name := range got {
		if !strings.Contains(string(agents), name) {
			t.Errorf("AGENTS.md does not list %s", name)
		}
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Error(err)
		}
	}
}

func TestCardFlagsReachTheAutoInstructionAndLabel(t *testing.T) {
	s, _ := LoadSpec()
	tasks, _ := s.SelectFor(nil, Condition{Read: "auto"})
	cond := Condition{Docs: "full", Read: "auto", Card: "--style=compact --frame=box"}
	if got := cond.Label(); got != "read:auto+style=compact,frame=box" {
		t.Errorf("label = %q", got)
	}
	dir := t.TempDir()
	if _, err := StageWorkspace(dir, repoRoot(t), s, tasks[0], cond); err != nil {
		t.Fatal(err)
	}
	agents, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(agents), "harnez read --auto --style=compact --frame=box <file>") || strings.Contains(string(agents), "{{card}}") {
		t.Errorf("instruction: %s", agents)
	}
	plain := t.TempDir()
	_, _ = StageWorkspace(plain, repoRoot(t), s, tasks[0], Condition{Docs: "full", Read: "auto"})
	agents, _ = os.ReadFile(filepath.Join(plain, "AGENTS.md"))
	if !strings.Contains(string(agents), "harnez read --auto <file>") {
		t.Errorf("plain instruction changed: %s", agents)
	}
}

func TestParseReadListOrderAndErrors(t *testing.T) {
	got, err := ParseReadList(" text,card ")
	if err != nil || len(got) != 2 || got[0] != "text" || got[1] != "card" {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, input := range []string{"text,,card", "unknown"} {
		if _, err := ParseReadList(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}

func TestParseModelsResolvesSpecsAndRejectsBadValues(t *testing.T) {
	got, err := ParseModels("agy:flash37:low,claude:haiku:low")
	if err != nil || len(got) != 2 || got[0].Provider != AgentAgy || got[0].Name != "gemini-3.7-flash" || got[0].Tier != "low" || got[1].Provider != AgentClaude {
		t.Fatalf("got %#v, %v", got, err)
	}
	for _, input := range []string{"agy:flash37:low,", "madeup:model:low"} {
		if _, err := ParseModels(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}
