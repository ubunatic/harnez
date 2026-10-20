package claude

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"ubunatic.com/harnez/internal/markdown"
)

func TestInitLocalSectionsGating(t *testing.T) {
	dir := t.TempDir()
	agentsPath := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# Project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{AgentsMD: AgentsMD{Local: AgentsMDTarget{
		Target:   "./AGENTS.md",
		Sections: []MDSection{{Name: "Rules", Content: "- be kind"}},
	}}}

	ir := &initReport{}
	if err := initLocalSections(ir, cfg, nil, "", agentsPath, false); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(agentsPath); string(data) != "# Project\n" || ir.changes != 0 {
		t.Fatalf("unsafe managed blocks must leave AGENTS.md untouched: %q, changes %d", data, ir.changes)
	}

	if err := initLocalSections(ir, cfg, nil, "solo", agentsPath, true); err == nil || !strings.Contains(err.Error(), `unknown repo mode "solo"`) {
		t.Fatalf("unknown repo mode error = %v", err)
	}
}

func TestInitProjectSummaryDisabledIsNoop(t *testing.T) {
	dir := t.TempDir()
	ir := &initReport{}
	if err := initProjectSummary(ir, dir, false, filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if ir.changes != 0 {
		t.Fatalf("changes = %d, want 0", ir.changes)
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("disabled summary must not create AGENTS.md (stat err %v)", err)
	}
}

func TestInitLocalSectionsPreservesUserTextAndStickyBlocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := os.WriteFile(path, []byte("# User-owned text\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Obsolete", "Project Summary", "Repo Setup"} {
		if _, _, err := markdown.Apply(path, name, "keep "+name); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &Config{AgentsMD: AgentsMD{
		Local: AgentsMDTarget{Target: "./AGENTS.md", Sections: []MDSection{{Name: "Rules", Content: "rule content"}}},
	}}
	r := &initReport{}
	if err := initLocalSections(r, cfg, nil, "", path, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# User-owned text", "rule content", "keep Project Summary", "keep Repo Setup"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %q from %q", want, data)
		}
	}
	if strings.Contains(string(data), "Obsolete") || r.changes != 2 {
		t.Fatalf("prune and section write: changes=%d, content=%q", r.changes, data)
	}
	again := &initReport{}
	if err := initLocalSections(again, cfg, nil, "", path, true); err != nil || again.changes != 0 {
		t.Fatalf("repeat: changes=%d err=%v", again.changes, err)
	}
}

func TestInitCopyDocsTemplateAndManagedTail(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{FS: fstest.MapFS{
		"doc.md":             {Data: []byte("bundled doc\n")},
		"templates/Makefile": {Data: []byte("# template\n")},
	}, AgentsMD: AgentsMD{Languages: map[string]Language{
		"doc": {Source: "doc.md", Local: "./Doc.md", Template: "templates/Makefile", Targets: "missing-targets.md"},
	}}}
	r := &initReport{}
	// A newly scaffolded template already has its targets; do not read Targets.
	if err := initCopyDocs(r, true, cfg, dir, []string{"doc"}, nil); err != nil || r.changes != 2 {
		t.Fatalf("copy and template: changes=%d err=%v", r.changes, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "Makefile"))
	if err != nil || string(data) != "# template\n" {
		t.Fatalf("template=%q err=%v", data, err)
	}
	path := filepath.Join(dir, "Doc.md")
	data, err = os.ReadFile(path)
	if err != nil || string(data) != "bundled doc\n\n<!-- harnez:stop -->\n" {
		t.Fatalf("doc=%q err=%v", data, err)
	}
	const local = "\n## Local\nkeep my notes\n"
	if err := os.WriteFile(path, append(data, []byte(local)...), 0644); err != nil {
		t.Fatal(err)
	}
	lang := cfg.AgentsMD.Languages["doc"]
	lang.Targets = ""
	cfg.AgentsMD.Languages["doc"] = lang
	again := &initReport{}
	if err := initCopyDocs(again, true, cfg, dir, []string{"doc"}, nil); err != nil || again.changes != 0 {
		t.Fatalf("repeat: changes=%d err=%v", again.changes, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(data)+local {
		t.Fatalf("local tail changed: %q, %v", got, err)
	}
}

func TestInitContainerTestTarget(t *testing.T) {
	dir := t.TempDir()
	makefile := filepath.Join(dir, "Makefile")
	if err := os.WriteFile(makefile, []byte(".PHONY: ⚙️ 🤖\n\n_prim := \\033[36m\n_rst := \\033[0m\n\nhelp: 🤖  # help\n\t@true\n\ncontainer-test: ⚙️  # project check\n\t@echo project\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{FS: fstest.MapFS{
		"docs/templates/ContainerTest.mk": {Data: []byte("container-test: ⚙️  # run container checks\n\t@env -u HTTP_PROXY -u http_proxy -u HTTPS_PROXY -u https_proxy -u ALL_PROXY -u all_proxy -u NO_PROXY -u no_proxy $(CONTAINER_ENGINE) run $(CONTAINER_TEST_CMD)\n\t@echo CONTAINER_SOURCE_MOUNT ALL_PROXY all_proxy NO_PROXY no_proxy\n")},
	}}
	first := &initReport{}
	if err := initContainerTestTarget(first, true, cfg, dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(makefile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "container-test: ⚙️  # project check") || strings.Contains(string(data), "env -u") {
		t.Fatalf("custom target was overwritten: %s", data)
	}
	if err := os.WriteFile(makefile, []byte(".PHONY: ⚙️ 🤖\n\n_prim := \\033[36m\n_rst := \\033[0m\n\nhelp: 🤖  # help\n\t@true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	second := &initReport{}
	if err := initContainerTestTarget(second, true, cfg, dir); err != nil {
		t.Fatal(err)
	}
	want := []string{"container-test: ⚙️", "CONTAINER_SOURCE_MOUNT", "CONTAINER_TEST_CMD", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy"}
	for _, token := range want {
		if !strings.Contains(string(mustRead(t, makefile)), token) {
			t.Errorf("generated Makefile missing %q", token)
		}
	}
	third := &initReport{}
	if err := initContainerTestTarget(third, true, cfg, dir); err != nil || third.changes != 0 {
		t.Fatalf("repeat init changes=%d err=%v", third.changes, err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestContainerTestTargetScrubsProxyAndPropagatesFailure(t *testing.T) {
	if os.Getenv("HARNEZ_SANDBOX") == "quota1" {
		t.Skip("container engine cannot run inside the quota-1 sandbox")
	}
	dir := filepath.Join(t.TempDir(), "project with spaces")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	makefile := filepath.Join(dir, "Makefile")
	if err := os.WriteFile(makefile, []byte(".PHONY: ⚙️ 🤖\n\n_prim := \\033[36m\n_rst := \\033[0m\n\nhelp: 🤖  # help\n\t@true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{FS: fstest.MapFS{"docs/templates/ContainerTest.mk": {Data: mustRead(t, filepath.Join("..", "..", "docs", "templates", "ContainerTest.mk"))}}}
	if err := initContainerTestTarget(&initReport{}, true, cfg, dir); err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(t.TempDir(), "fake engine")
	capture := filepath.Join(t.TempDir(), "engine args")
	script := "#!/bin/sh\nprintf 'arg=%s\\n' \"$@\" >> \"$CAPTURE\"\nprintf 'proxy=%s\\n' \"${HTTP_PROXY-unset}:${HTTPS_PROXY-unset}:${ALL_PROXY-unset}:${NO_PROXY-unset}:${http_proxy-unset}:${https_proxy-unset}:${all_proxy-unset}:${no_proxy-unset}\" >> \"$CAPTURE\"\nif test \"$1\" = run && test \"${FAIL_RUN:-0}\" = 1; then exit 23; fi\n"
	if err := os.WriteFile(engine, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "container output")
	cleanEnv := func() []string {
		var env []string
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "HARNEZ_QUOTA_1=") && !strings.HasPrefix(entry, "HARNEZ_SANDBOX=") {
				env = append(env, entry)
			}
		}
		return env
	}
	run := func(fail bool) ([]byte, error) {
		cmd := exec.Command("make", "container-test", "CONTAINER_ENGINE="+engine, "CONTAINER_OUTPUT="+output, "CONTAINER_SOURCE_MODE=mount", "CONTAINER_TEST_CMD=make check")
		cmd.Dir = dir
		cmd.Env = append(cleanEnv(), "HARNEZ_QUOTA_1=0", "CAPTURE="+capture, "HTTP_PROXY=http://proxy.invalid", "HTTPS_PROXY=http://proxy.invalid", "ALL_PROXY=http://proxy.invalid", "NO_PROXY=example.invalid", "http_proxy=http://proxy.invalid", "https_proxy=http://proxy.invalid", "all_proxy=http://proxy.invalid", "no_proxy=example.invalid")
		if fail {
			cmd.Env = append(cmd.Env, "FAIL_RUN=1")
		}
		return cmd.CombinedOutput()
	}
	blocked := exec.Command("make", "container-test", "CONTAINER_ENGINE="+engine, "CONTAINER_OUTPUT="+output)
	blocked.Dir = dir
	blocked.Env = append(cleanEnv(), "CAPTURE="+capture, "HARNEZ_QUOTA_1=1")
	if output, err := blocked.CombinedOutput(); err == nil || !strings.Contains(string(output), "container tests need the host; ask the host") {
		t.Fatalf("quota diagnostic err=%v output=%q", err, output)
	}
	marked := exec.Command("make", "container-test", "CONTAINER_ENGINE="+engine, "CONTAINER_OUTPUT="+output)
	marked.Dir = dir
	marked.Env = append(cleanEnv(), "CAPTURE="+capture, "HARNEZ_SANDBOX=quota1")
	if output, err := marked.CombinedOutput(); err == nil || !strings.Contains(string(output), "container tests need the host; ask the host") {
		t.Fatalf("sandbox marker diagnostic err=%v output=%q", err, output)
	}
	if output, err := run(false); err != nil {
		t.Fatalf("make container-test: %v\n%s", err, output)
	}
	data := string(mustRead(t, capture))
	for _, want := range []string{"arg=build", "arg=run", "arg=--file", "arg=Containerfile", "arg=--volume", output + ":/out:rw", dir + ":/src:ro", "proxy=unset:unset:unset:unset:unset:unset:unset:unset"} {
		if !strings.Contains(data, want) {
			t.Errorf("fake engine capture missing %q: %s", want, data)
		}
	}
	if _, err := run(true); err == nil {
		t.Fatal("container engine failure was swallowed")
	}
}
