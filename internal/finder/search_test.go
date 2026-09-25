package finder

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeExecutable(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func fakeToolPath(t *testing.T) (string, string, string) {
	t.Helper()
	base := t.TempDir()
	bin := filepath.Join(base, "bin")
	root := filepath.Join(base, "repo")
	home := filepath.Join(base, "home")
	for _, dir := range []string{bin, root, home} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("HOME", home)
	return bin, root, home
}

func TestRegistryProjectOverridesUserFinder(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	root := filepath.Join(base, "repo")
	_ = os.MkdirAll(filepath.Join(home, ".harnez"), 0755)
	_ = os.MkdirAll(root, 0755)
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".harnez", "config.yaml"), []byte("finders:\n  - name: custom\n    scope: code\n    command: [old, '{query}']\n    timeout: 1s\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("finders:\n  - name: custom\n    scope: docs\n    command: [new, '{query}']\n    timeout: 2s\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Registry(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Scope != "docs" || got[0].Command[0] != "new" {
		t.Fatalf("registry=%+v", got)
	}
}

func TestSearchMergesDeduplicatesAndUsesArgumentSafeFakeFinders(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	home := filepath.Join(base, "home")
	bin := filepath.Join(base, "bin")
	for _, p := range []string{root, home, bin} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin)
	first := writeExecutable(t, bin, "first", `/bin/sleep 0.2; printf '%s' '[{"path":"shared.go","line":4,"title":"shared","snippet":"first","score":0.01,"kind":"code"},{"path":"first.go","line":2,"title":"first","snippet":"one","kind":"code"}]'`)
	second := writeExecutable(t, bin, "second", `/bin/sleep 0.2; printf '%s\n' '{"path":"shared.go","line":4,"title":"shared","snippet":"second","score":0.99,"kind":"code"}' '{"path":"second.go","line":7,"title":"second","snippet":"two","kind":"code"}'`)
	cfg := `finders:
  - name: first
    scope: code
    command: ["` + first + `", "{query}"]
    timeout: 2s
  - name: second
    scope: code
    command: ["` + second + `", "{query}"]
    timeout: 2s
`
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	got, err := Search(context.Background(), root, "code", "quoted ; $(false)", "", 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(started) > 450*time.Millisecond {
		t.Fatal("finders did not execute concurrently")
	}
	if len(got.Results) != 3 {
		t.Fatalf("results=%+v", got.Results)
	}
	if got.Results[0].Path != "shared.go" || got.Results[0].Score < 0.03 {
		t.Fatalf("dedup/rank=%+v", got.Results)
	}
	if len(got.Finders) < 2 {
		t.Fatalf("statuses=%+v", got.Finders)
	}
}

func TestSearchViaRejectsOutOfScopeAndUnknown(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	home := filepath.Join(base, "home")
	_ = os.MkdirAll(root, 0755)
	_ = os.MkdirAll(home, 0755)
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("finders:\n  - name: docs-only\n    scope: docs\n    command: [x]\n    timeout: 1s\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, via := range []string{"missing", "docs-only"} {
		if _, err := Search(context.Background(), root, "code", "x", via, 10, nil); err == nil {
			t.Errorf("via %q accepted", via)
		}
	}
}

func TestSearchIsolatesTimeoutAndFinderFailures(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	home := filepath.Join(base, "home")
	bin := filepath.Join(base, "bin")
	for _, p := range []string{root, home, bin} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin)
	slow := "/bin/sleep"
	good := writeExecutable(t, bin, "good", `printf '%s' '[{"path":"ok.go","line":1,"title":"ok","snippet":"kept","kind":"code"}]'`)
	cfg := `finders:
  - name: slow
    scope: code
    command: ["` + slow + `", "1"]
    timeout: 30ms
  - name: good
    scope: code
    command: ["` + good + `"]
    timeout: 1s
`
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Search(context.Background(), root, "code", "query", "", 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 || got.Results[0].Path != "ok.go" {
		t.Fatalf("successful finder result lost: %+v", got)
	}
	foundTimeout := false
	for _, status := range got.Finders {
		if status.Name == "slow" && status.Status == "timeout" {
			foundTimeout = true
		}
	}
	if !foundTimeout {
		t.Fatalf("timeout status missing: %+v", got.Finders)
	}
}

func TestNeusContractIndexesOnceAndRetriesWithExactArgv(t *testing.T) {
	bin, root, _ := fakeToolPath(t)
	logPath, countPath := filepath.Join(t.TempDir(), "calls"), filepath.Join(t.TempDir(), "count")
	t.Setenv("CALL_LOG", logPath)
	t.Setenv("CALL_COUNT", countPath)
	writeExecutable(t, bin, "neus", `
if [ "$1" = index ]; then printf 'index:%s\n' "$2" >> "$CALL_LOG"; exit 0; fi
if [ "$1" != search ]; then exit 9; fi
shift
printf 'search\n' >> "$CALL_LOG"
printf '<%s>\n' "$@" >> "$CALL_LOG"
n=0
if [ -f "$CALL_COUNT" ]; then read n < "$CALL_COUNT"; fi
n=$((n+1)); printf '%s' "$n" > "$CALL_COUNT"
if [ "$n" -eq 1 ]; then exit 3; fi
	printf '%s' '[{"path":"result.go","line":8,"title":"result","snippet":"ok","score":0.25,"kind":"code"}]'
`)
	query := "agent resume; $(false)"
	got, err := Search(context.Background(), root, "code", query, "", 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 || len(got.Finders) != 1 || got.Finders[0].Status != "ok" {
		t.Fatalf("response=%+v", got)
	}
	want := "search\n<--json>\n<--root>\n<" + root + ">\n<--kind>\n<code>\n<-k>\n<3>\n<--timeout>\n<30s>\n<" + query + ">\nindex:" + root + "\nsearch\n<--json>\n<--root>\n<" + root + ">\n<--kind>\n<code>\n<-k>\n<3>\n<--timeout>\n<30s>\n<" + query + ">\n"
	gotLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotLog) != want {
		t.Fatalf("neus calls:\n%s\nwant:\n%s", gotLog, want)
	}
	count, err := os.ReadFile(countPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(count) != "2" {
		t.Fatalf("search attempts=%s, want 2", count)
	}
}

func TestNeusBackendErrorDoesNotHideRegisteredFinder(t *testing.T) {
	bin, root, _ := fakeToolPath(t)
	writeExecutable(t, bin, "neus", `if [ "$1" = search ]; then exit 4; fi; exit 0`)
	good := writeExecutable(t, bin, "good", `printf '%s' '[{"path":"kept.go","line":2,"title":"kept","snippet":"yes","kind":"code"}]'`)
	cfg := `finders:
  - name: good
    scope: code
    command: ["` + good + `"]
    timeout: 1s
`
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Search(context.Background(), root, "code", "query", "", 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 || got.Results[0].Path != "kept.go" {
		t.Fatalf("results=%+v", got.Results)
	}
	foundError := false
	for _, s := range got.Finders {
		if s.Name == "neus" && s.Status == "error" {
			foundError = true
		}
	}
	if !foundError {
		t.Fatalf("neus exit 4 status missing: %+v", got.Finders)
	}
}

func TestRGIsCodeFallbackWhenNeusIsMissing(t *testing.T) {
	bin, root, _ := fakeToolPath(t)
	logPath := filepath.Join(t.TempDir(), "rg.args")
	t.Setenv("RG_LOG", logPath)
	writeExecutable(t, bin, "rg", `printf '%s\n' "$@" > "$RG_LOG"; printf '%s\n' 'src/a.go:6:match'`)
	query := "needle with spaces"
	got, err := Search(context.Background(), root, "code", query, "", 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 || got.Results[0].Path != "src/a.go" || got.Results[0].Line != 6 {
		t.Fatalf("results=%+v", got.Results)
	}
	args, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--\n"+query+"\n"+root+"\n") {
		t.Fatalf("rg argv=%q", args)
	}
	if len(got.Finders) != 1 || got.Finders[0].Name != "rg" || got.Finders[0].Status != "ok" {
		t.Fatalf("finders=%+v", got.Finders)
	}
}

func TestDocsFuzzyFallbackSelectionDependsOnNeus(t *testing.T) {
	t.Run("neus missing", func(t *testing.T) {
		bin, root, _ := fakeToolPath(t)
		_ = bin
		if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("# Agent resume\nmatching documentation\n"), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := Search(context.Background(), root, "docs", "agent resume", "", 5, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Results) != 1 || got.Results[0].Path != "guide.md" || len(got.Finders) != 1 || got.Finders[0].Name != "fuzzy" {
			t.Fatalf("response=%+v", got)
		}
	})
	t.Run("neus present", func(t *testing.T) {
		bin, root, _ := fakeToolPath(t)
		writeExecutable(t, bin, "neus", `printf '%s' '[]'`)
		got, err := Search(context.Background(), root, "docs", "agent resume", "", 5, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Finders) != 1 || got.Finders[0].Name != "neus" {
			t.Fatalf("fuzzy must not run when neus is available: %+v", got.Finders)
		}
	})
}

func TestBuiltinDocsReportsCanceledContextAsTimeout(t *testing.T) {
	_, root, _ := fakeToolPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, status := run(ctx, root, Definition{Name: "fuzzy", Scope: "docs", Timeout: "1s", builtin: true}, "query", 5)
	if status.Status != "timeout" {
		t.Fatalf("status=%+v", status)
	}
}

func TestParseJSONLAndRelativePaths(t *testing.T) {
	root := t.TempDir()
	raw := "{\"path\":\"" + filepath.Join(root, "a.md") + "\",\"line\":3,\"kind\":\"docs\"}\n"
	got, err := parseJSON([]byte(raw), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "a.md" {
		t.Fatalf("parsed=%+v", got)
	}
	if strings.Contains(got[0].Path, root) {
		t.Fatalf("path not normalized: %+v", got)
	}
}
