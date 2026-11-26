package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/decide"
	"ubunatic.com/harnez/internal/hookdispatch"
	"ubunatic.com/harnez/internal/telemetry"
)

// The registry must bind exactly the handlers the manifest names, per event, in
// a fixed order (issue 1304 plan table).
func TestHookRegistryMatchesManifestNames(t *testing.T) {
	reg, err := claudeHookRegistry(hookDeps{})
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Validate(); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"PreToolUse":         "exec,read,pre-edit",
		"PostToolUse":        "skill",
		"PostToolUseFailure": "skill",
		"UserPromptSubmit":   "skill",
		"SessionStart":       "claude-instructions",
		"PreCompact":         "codex-telemetry,compact-reset",
		"PostCompact":        "codex-telemetry",
		"Stop":               "sound",
	}
	got := map[string]string{}
	for _, event := range reg.Events() {
		var names []string
		for _, h := range reg[event] {
			names = append(names, h.Name)
		}
		got[event] = strings.Join(names, ",")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registry = %v\nwant %v", got, want)
	}
}

func TestHookRegistryToolRoutingMatchesOldMatchers(t *testing.T) {
	reg, err := claudeHookRegistry(hookDeps{})
	if err != nil {
		t.Fatal(err)
	}
	wantTools := map[string][]string{
		"exec":     {"Bash"},
		"read":     {"View", "Read", "ReadMultipleFiles"},
		"pre-edit": {"Edit", "Write"},
		"skill":    {"Skill"},
	}
	for _, event := range reg.Events() {
		for _, h := range reg[event] {
			if want, ok := wantTools[h.Name]; ok && event != "UserPromptSubmit" && !reflect.DeepEqual(h.Tools, want) {
				t.Errorf("%s/%s tools = %v, want %v", event, h.Name, h.Tools, want)
			}
		}
	}
}

// Every manifest event is a hidden `harnez hook <Event>` command.
func TestHookEventCommandsRegisteredForEveryManifestEvent(t *testing.T) {
	root := newHookCmd()
	for _, event := range manifestEvents(t, "claude") {
		c, _, err := root.Find([]string{event})
		if err != nil || c.Name() != event || !c.Hidden {
			t.Errorf("hook %s: command %v, err %v", event, c, err)
		}
	}
}

// blockingBackend answers "rule violated" for every request, so pre-edit denies.
func blockingBackend() decide.Backend {
	noul := 0.95
	return &testMockBackend{fn: func(context.Context, *decide.Request) (*decide.Response, error) {
		return &decide.Response{
			Model: "jev-test",
			Answers: map[string]decide.Answer{
				"rule_violation": {Type: decide.TypeNoul, Noul: &noul},
				"violated_rule":  {Type: decide.TypeChoice, Choice: "Tools.md"},
			},
		}, nil
	}}
}

// sideEffects records what a handler did besides writing its reply.
type sideEffects struct {
	skills    []telemetry.ToolCall
	telemetry []string
}

// goldenEnv is what one side of a golden run may touch.
type goldenEnv struct {
	stateDir string
	pre      preEditOptions
	fx       *sideEffects
}

func (e goldenEnv) deps() hookDeps {
	return hookDeps{
		PreEdit:     e.pre,
		SkillInsert: e.insertSkill,
		Telemetry:   e.sendTelemetry,
		PlaySound:   func(context.Context) error { return nil },
		StateDir:    e.stateDir,
	}
}

func (e goldenEnv) insertSkill(_ string, call telemetry.ToolCall) error {
	e.fx.skills = append(e.fx.skills, call)
	return nil
}

func (e goldenEnv) sendTelemetry(kind string, _ any) {
	e.fx.telemetry = append(e.fx.telemetry, kind)
}

type goldenCase struct {
	name, event, payload string
	// agent selects the registry: "" is Claude, "codex" is Codex.
	agent string
	// session names a session whose read-marker dir exists before the run.
	session string
	// old is the subcommand behaviour the event replaced.
	old func(ctx context.Context, in io.Reader, out io.Writer, e goldenEnv) error
	// wantOut must appear in the output; wantSkills and wantTelemetry are the
	// minimum side effects, so a comparison of two empty results cannot pass.
	wantOut        string
	wantSkills     int
	wantTelemetry  int
	wantMarkerGone bool
	// wantErr marks a payload the old subcommand rejects with an error. The
	// dispatcher must then fail with the same error text and write nothing.
	wantErr bool
}

type goldenResult struct {
	out        string
	fx         sideEffects
	markerGone bool
}

// runGoldenSide runs one side of a case under its own temp HOME and state dir.
// dispatch selects the hookdispatch path; otherwise the old behaviour runs.
func runGoldenSide(t *testing.T, tc goldenCase, budget time.Duration, dispatch bool) (goldenResult, error) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeSkillFile(t, filepath.Join(home, ".claude", "skills", "issue", "SKILL.md"))
	stateDir := t.TempDir()
	marker := ""
	if tc.session != "" {
		marker = filepath.Join(stateDir, sessionMarkerDir(tc.session))
		if err := os.MkdirAll(marker, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(marker, "read"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	enforce := true
	env := goldenEnv{
		stateDir: stateDir,
		pre:      preEditOptions{Backend: blockingBackend(), Enforce: &enforce},
		fx:       &sideEffects{},
	}
	var out bytes.Buffer
	var runErr error
	if dispatch {
		reg, err := registryFor(tc.agent, env.deps())
		if err != nil {
			t.Fatal(err)
		}
		runErr = hookdispatch.Dispatch(context.Background(), reg, tc.event, []byte(tc.payload), &out,
			hookdispatch.Options{Budget: budget, Stderr: io.Discard})
	} else {
		runErr = tc.old(context.Background(), strings.NewReader(tc.payload), &out, env)
	}
	res := goldenResult{out: out.String(), fx: *env.fx}
	if marker != "" {
		_, err := os.Stat(marker)
		res.markerGone = os.IsNotExist(err)
	}
	return res, runErr
}

// Each event's dispatcher output and side effects must equal what the old
// subcommands did for the same payload. Every case also asserts a minimum
// effect, so the comparison has teeth.
func TestHookDispatchMatchesSubcommandBehaviour(t *testing.T) {
	repo := t.TempDir()
	big := filepath.Join(repo, "big.go")
	var src strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&src, "var v%d = %d\n", i, i)
	}
	if err := os.WriteFile(big, []byte(src.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	edit := filepath.Join(repo, "x.go")
	if err := os.WriteFile(edit, []byte("package x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	codexPayload := `{"hook_event_name":"PreCompact","session_id":"s1","model":"gpt-5","token_usage":{"total_tokens":10}}`

	oldSkill := func(_ context.Context, in io.Reader, _ io.Writer, e goldenEnv) error {
		runClaudeSkillHook(in, skillHookOptions{Insert: e.insertSkill})
		return nil
	}
	cases := []goldenCase{
		{
			name:    "PreToolUse Bash rewrite",
			event:   "PreToolUse",
			payload: `{"tool_name":"Bash","tool_input":{"command":"go test ./internal/x"},"session_id":"s1","cwd":"` + repo + `"}`,
			old: func(_ context.Context, in io.Reader, out io.Writer, _ goldenEnv) error {
				return runExecHook(in, out)
			},
			wantOut: "⚙",
		},
		{
			name:    "PreToolUse Read large file",
			event:   "PreToolUse",
			payload: `{"tool_name":"Read","tool_input":{"file_path":"` + big + `"},"session_id":"s1","cwd":"` + repo + `"}`,
			old: func(_ context.Context, in io.Reader, out io.Writer, _ goldenEnv) error {
				return runClaudeReadHook(in, out, readHookOptions{})
			},
		},
		{
			name:    "PreToolUse Read plain file",
			event:   "PreToolUse",
			payload: `{"tool_name":"Read","tool_input":{"file_path":"` + edit + `"},"session_id":"s1","cwd":"` + repo + `"}`,
			old: func(_ context.Context, in io.Reader, out io.Writer, _ goldenEnv) error {
				return runClaudeReadHook(in, out, readHookOptions{})
			},
			wantOut: "{}",
		},
		{
			name:    "PreToolUse Edit denied by the backend",
			event:   "PreToolUse",
			payload: `{"hookEventName":"PreToolUse","tool_name":"Edit","tool_input":{"file_path":"` + edit + `","new_string":"package x\n"},"session_id":"s1","cwd":"` + repo + `"}`,
			old: func(ctx context.Context, in io.Reader, out io.Writer, e goldenEnv) error {
				return runPreEditHook(ctx, in, out, "", "", e.pre)
			},
			wantOut: `"permissionDecision":"deny"`,
		},
		{
			name:       "PostToolUse Skill",
			event:      "PostToolUse",
			payload:    `{"hook_event_name":"PostToolUse","tool_name":"Skill","tool_input":{"skill":"issue"},"session_id":"s1","cwd":"` + repo + `"}`,
			old:        oldSkill,
			wantSkills: 1,
		},
		{
			name:       "PostToolUseFailure Skill",
			event:      "PostToolUseFailure",
			payload:    `{"hook_event_name":"PostToolUseFailure","tool_name":"Skill","tool_input":{"skill":"lean-sprint"},"session_id":"s1","cwd":"` + repo + `"}`,
			old:        oldSkill,
			wantSkills: 1,
		},
		{
			name:       "UserPromptSubmit typed installed skill",
			event:      "UserPromptSubmit",
			payload:    `{"hook_event_name":"UserPromptSubmit","prompt":"/issue 12","session_id":"s1","cwd":"` + repo + `"}`,
			old:        oldSkill,
			wantSkills: 1,
		},
		{
			name:    "UserPromptSubmit plain prompt",
			event:   "UserPromptSubmit",
			payload: `{"hook_event_name":"UserPromptSubmit","prompt":"hello","session_id":"s1","cwd":"` + repo + `"}`,
			old:     oldSkill,
		},
		{
			name:    "SessionStart instructions",
			event:   "SessionStart",
			payload: `{"hook_event_name":"SessionStart"}`,
			old: func(_ context.Context, _ io.Reader, out io.Writer, _ goldenEnv) error {
				return runClaudeInstructionsHook(out)
			},
			wantOut: "Harnez instruction rule:",
		},
		{
			name:           "PreCompact resets markers and sends codex telemetry",
			event:          "PreCompact",
			payload:        codexPayload,
			session:        "s1",
			wantTelemetry:  1,
			wantMarkerGone: true,
			old: func(ctx context.Context, in io.Reader, _ io.Writer, e goldenEnv) error {
				payload, _ := io.ReadAll(in)
				if err := runCodexTelemetryWith(bytes.NewReader(payload), e.sendTelemetry); err != nil {
					return err
				}
				return runCompactResetHook(bytes.NewReader(payload), e.stateDir)
			},
		},
		{
			name:          "PostCompact sends codex telemetry",
			event:         "PostCompact",
			payload:       codexPayload,
			wantTelemetry: 1,
			old: func(_ context.Context, in io.Reader, _ io.Writer, e goldenEnv) error {
				return runCodexTelemetryWith(in, e.sendTelemetry)
			},
		},
	}

	runGoldenCases(t, cases)
}

// registryFor returns the registry for a golden case's agent.
func registryFor(agent string, deps hookDeps) (hookdispatch.Registry, error) {
	if agent == "codex" {
		return codexHookRegistry(deps)
	}
	return claudeHookRegistry(deps)
}

// manifestEvents returns the event names of an embedded agent manifest.
func manifestEvents(t *testing.T, agent string) []string {
	t.Helper()
	m, err := hookdispatch.LoadManifest(agent)
	if err != nil {
		t.Fatal(err)
	}
	return m.EventNames()
}

// runGoldenCases runs each case on both sides and checks they agree.
func runGoldenCases(t *testing.T, cases []goldenCase) {
	t.Helper()
	budget, err := hookdispatch.LoadBudget()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, wantErr := runGoldenSide(t, tc, budget, false)
			got, gotErr := runGoldenSide(t, tc, budget, true)

			if (wantErr != nil) != tc.wantErr {
				t.Fatalf("old subcommand error = %v, want error %v", wantErr, tc.wantErr)
			}
			if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
				t.Fatalf("dispatch error = %v, old subcommand error = %v", gotErr, wantErr)
			}
			if wantErr != nil {
				if got.out != want.out || got.out != "" {
					t.Fatalf("output on error = %q, old %q; want nothing", got.out, want.out)
				}
				return
			}
			if got.out != want.out {
				t.Fatalf("output changed\n got %q\nwant %q", got.out, want.out)
			}
			if tc.wantOut != "" && !strings.Contains(got.out, tc.wantOut) {
				t.Fatalf("output %q lacks %q", got.out, tc.wantOut)
			}
			if got.markerGone != want.markerGone || got.markerGone != tc.wantMarkerGone {
				t.Fatalf("marker removed = %v, old %v, want %v", got.markerGone, want.markerGone, tc.wantMarkerGone)
			}
			if !reflect.DeepEqual(got.fx.telemetry, want.fx.telemetry) {
				t.Fatalf("telemetry = %v, old %v", got.fx.telemetry, want.fx.telemetry)
			}
			if len(got.fx.telemetry) < tc.wantTelemetry {
				t.Fatalf("telemetry sent %d, want at least %d", len(got.fx.telemetry), tc.wantTelemetry)
			}
			if len(got.fx.skills) != len(want.fx.skills) || len(got.fx.skills) < tc.wantSkills {
				t.Fatalf("skill rows = %d, old %d, want at least %d", len(got.fx.skills), len(want.fx.skills), tc.wantSkills)
			}
			for i := range got.fx.skills {
				g, w := got.fx.skills[i], want.fx.skills[i]
				if g.ToolName != w.ToolName || g.SkillName != w.SkillName || g.CallType != w.CallType ||
					g.SessionID != w.SessionID || g.WorkingDir != w.WorkingDir || g.AgentID != w.AgentID {
					t.Fatalf("skill row %d = %+v, old %+v", i, g, w)
				}
			}
		})
	}
}

// The Stop sound goes through the injected player, once per event, and never
// writes to stdout.
func TestStopEventPlaysSoundOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	plays := 0
	reg, err := claudeHookRegistry(hookDeps{PlaySound: func(context.Context) error { plays++; return nil }})
	if err != nil {
		t.Fatal(err)
	}
	budget, err := hookdispatch.LoadBudget()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := hookdispatch.Dispatch(context.Background(), reg, "Stop", []byte(`{"hook_event_name":"Stop"}`), &out,
		hookdispatch.Options{Budget: budget, Stderr: io.Discard}); err != nil {
		t.Fatal(err)
	}
	if plays != 1 || out.Len() != 0 {
		t.Fatalf("plays = %d, output %q; want 1 play and no output", plays, out.String())
	}
}

// `harnez hook UserPromptSubmit` reads stdin once and dispatches. A plain prompt
// writes nothing and exits 0.
func TestHookEventCommandDispatches(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := newHookCmd()
	cmd.SetArgs([]string{"UserPromptSubmit"})
	cmd.SetIn(strings.NewReader(`{"hook_event_name":"UserPromptSubmit","prompt":"just text"}`))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hook UserPromptSubmit: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected output %q", out.String())
	}
}
