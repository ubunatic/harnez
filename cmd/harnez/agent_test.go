package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"ubunatic.com/harnez/internal/subagent"
)

func testRatingPtr(rating int) *int { return &rating }

func TestAgentCommandSurface(t *testing.T) {
	c := newAgentCmd()
	if len(c.Commands()) == 0 {
		t.Fatal("agent command has no children")
	}
	for _, name := range []string{"start", "models", "resume", "list", "status", "wait", "compact", "stop", "delete", "enable", "disable", "send"} {
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
		"A requested model such as `sol:low` or `luna` is a Harnez agent model",
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
		if err != nil || m.Provider != "claude" || m.Name != "claude-haiku-5-5" || m.Tier != "low" {
			t.Fatalf("%s resolved to %#v (%v)", spec, m, err)
		}
	}
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
	err = runStart(t.Context(), cmd, agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }}, startRequest{Prompt: "direct", StoredPrompt: "direct", ModelSpec: "codex:luna", Dir: ".", StreamMode: streamFull})
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
