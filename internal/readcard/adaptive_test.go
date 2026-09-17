package readcard

import (
	"encoding/json"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCompressionPreservesSyntaxAndAnchors(t *testing.T) {
	goSource := "package sample\n\nvar message = `first\n\n  literal  spaces\nlast`\n\nfunc value() string {\n\t return message\n}\n"
	for _, mode := range []string{"ws", "ast"} {
		t.Run(mode, func(t *testing.T) {
			got, err := Compress(strings.Split(goSource, "\n"), "sample.go", mode, 1)
			if err != nil {
				t.Fatal(err)
			}
			text := strings.Join(got.Lines, "\n")
			if !strings.Contains(text, "`first\n\n  literal  spaces\nlast`") {
				t.Fatalf("raw literal changed: %q", text)
			}
			_, before := goTokens(goSource)
			_, after := goTokens(text)
			if before != after {
				t.Fatal("Go token sequence changed")
			}
			if !reflect.DeepEqual(got.SourceLines, []int{1, 3, 4, 5, 6, 8, 9, 10}) {
				t.Fatalf("source map = %v", got.SourceLines)
			}
			if len(text) >= len(goSource) {
				t.Fatal("no compaction")
			}
		})
	}
	jsonText := "{\n  \"x\": \" spaces  remain \",\n \"n\": 123456789012345678901234567890\n}"
	got, err := Compress(strings.Split(jsonText, "\n"), "x.json", "ast", 40)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceLines[0] != 40 || !json.Valid([]byte(got.Lines[0])) || !strings.Contains(got.Lines[0], "123456789012345678901234567890") || !strings.Contains(got.Lines[0], " spaces  remain ") {
		t.Fatalf("invalid JSON compaction: %+v", got)
	}
	for _, file := range []string{"x.py", "x.yaml"} {
		if _, err := Compress([]string{" x"}, file, "ws", 1); err == nil {
			t.Fatalf("unsupported %s must fail clearly", file)
		}
	}
	if _, err := Compress([]string{"return 1"}, "slice.go", "ws", 1); err == nil {
		t.Fatal("partial Go fragment silently compressed")
	}
}

func TestShellCompressionPreservesExecution(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash unavailable")
	}
	for _, source := range []string{
		"name='two  spaces'\n  printf   '%s\\n'    \"$name\"\n", // escaped syntax is conservative
		"name='two  spaces'\n  echo    \"$name\"   end\n",
		"cat <<'EOF'\n  body  remains\nEOF\n",
		"echo \"$(printf '  x')\"\n",
		"printf '%s' 'multiline\n  literal'\n",
		"echo one   # keep  comment\n echo two\n",
	} {
		got, err := Compress(strings.Split(source, "\n"), "script.sh", "ast", 1)
		if err != nil {
			t.Fatal(err)
		}
		before, err := exec.Command(bash, "-c", source).CombinedOutput()
		if err != nil {
			t.Fatalf("baseline: %v %s", err, before)
		}
		after, err := exec.Command(bash, "-c", strings.Join(got.Lines, "\n")).CombinedOutput()
		if err != nil || string(before) != string(after) {
			t.Fatalf("shell behavior changed: before=%q after=%q err=%v", before, after, err)
		}
	}
}

func TestCadenceOriginalLines(t *testing.T) {
	res := &ReadResult{Lines: []string{"first", "second", "third", "fourth"}, StartLine: 8, SourceLines: []int{8, 9, 10, 20}}
	got, err := FormatTextCadence(res, "10")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"8 │ first", ". │ second", "10 │ third", "20 │ fourth"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("missing %q: %s", expected, got)
		}
	}
	for _, mode := range []string{"none", "off"} {
		text, err := FormatTextCadence(res, mode)
		if err != nil || strings.Contains(text, "│") {
			t.Fatalf("off gutter: %q %v", text, err)
		}
	}
	for _, invalid := range []string{"0", "-1", "ten", "every:0"} {
		if _, err := ParseLineNumbers(invalid); err == nil {
			t.Fatalf("accepted %q", invalid)
		}
	}
}

func TestProviderRouting(t *testing.T) {
	stats := ComputeImageTokens(600, 2250, 300, 300, 1)
	for _, tc := range []struct {
		provider Provider
		want     bool
	}{{ProviderClaude, true}, {ProviderOpenAI, true}, {ProviderGemini, true}, {ProviderLocal, false}, {ProviderUnknown, false}} {
		if got := PreferImage(tc.provider, 60, stats); got != tc.want {
			t.Errorf("%s got %v", tc.provider, got)
		}
	}
	stats = ComputeImageTokens(400, 1500, 600, 300, 1)
	if !PreferImage(ProviderClaude, 30, stats) || PreferImage(ProviderOpenAI, 30, stats) || PreferImage(ProviderGemini, 30, stats) {
		t.Fatalf("provider differentiation lost: %+v", stats)
	}
	if PreferImage(ProviderClaude, 5, TokenStats{TextTokens: 99, ClaudeTokens: 1}) {
		t.Fatal("micro snippet must use text")
	}
	if PreferImage(ProviderClaude, 10, TokenStats{TextTokens: 999, ClaudeTokens: 1000}) {
		t.Fatal("rounded ratio must not admit an uneconomical image")
	}
	if !PreferImage(ProviderClaude, 10, TokenStats{TextTokens: 1000, ClaudeTokens: 1000}) {
		t.Fatal("equality should permit image")
	}
	for _, tc := range []struct {
		env  map[string]string
		want Provider
	}{
		{map[string]string{"HARNEZ_AGENT_HARNESS": "claude", "CODEX_CLI": "1"}, ProviderClaude},
		{map[string]string{"CODEX_CLI": "1"}, ProviderOpenAI},
		{map[string]string{"GEMINI_CLI": "true"}, ProviderGemini},
		{map[string]string{"CLAUDECODE": "1", "CODEX_CLI": "1"}, ProviderUnknown},
		{map[string]string{"HARNEZ_AGENT_HARNESS": "pi", "CLAUDECODE": "1"}, ProviderUnknown},
		{map[string]string{}, ProviderUnknown},
	} {
		if got := DetectProvider(func(key string) string { return tc.env[key] }); got != tc.want {
			t.Errorf("env %v: %s want %s", tc.env, got, tc.want)
		}
	}
}

func TestContentFirstPagesAndExactCost(t *testing.T) {
	dir := t.TempDir()
	tiny, err := RenderFileToCards([]string{"123"}, "stdin", RenderOptions{Title: "stdin", Columns: 3, ShowLineNumbers: true, OutputPath: filepath.Join(dir, "tiny.png")})
	if err != nil {
		t.Fatal(err)
	}
	if tiny.Columns != 1 || tiny.Width > 350 || tiny.Width < 250 {
		t.Fatalf("tiny card not tightly cropped: %+v", tiny)
	}
	lines := strings.Split(strings.Repeat("short\n", 480), "\n")
	res, err := RenderFileToCards(lines, "source.go", RenderOptions{ShowLineNumbers: true, OutputPath: filepath.Join(dir, "pages.png")})
	if err != nil {
		t.Fatal(err)
	}
	if res.Columns != 3 || len(res.Pages) < 2 {
		t.Fatalf("default packing/pages: %+v", res)
	}
	claude, openai, gemini := 0, 0, 0
	for i, page := range res.Pages {
		f, err := os.Open(res.Files[i])
		if err != nil {
			t.Fatal(err)
		}
		config, err := png.DecodeConfig(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if config.Width != page.Width || config.Height != page.Height {
			t.Fatal("metadata does not match actual PNG")
		}
		s := ComputeImageTokens(0, 0, page.Width, page.Height, 1)
		claude += s.ClaudeTokens
		openai += s.OpenAITokens
		gemini += s.GeminiTokens
	}
	if res.TokenStats.ClaudeTokens != claude || res.TokenStats.OpenAITokens != openai || res.TokenStats.GeminiTokens != gemini {
		t.Fatalf("incorrect per-page cost %+v", res.TokenStats)
	}
	measured, err := RenderFileToCards(lines, "source.go", RenderOptions{ShowLineNumbers: true, MeasureOnly: true, OutputPath: filepath.Join(dir, "must-not-exist.png")})
	if err != nil || !reflect.DeepEqual(res.Pages, measured.Pages) || len(measured.Files) != 0 {
		t.Fatalf("measurement diverges: %v %+v", err, measured)
	}
	if _, err := os.Stat(filepath.Join(dir, "must-not-exist.png")); !os.IsNotExist(err) {
		t.Fatal("measurement wrote a file")
	}
}
