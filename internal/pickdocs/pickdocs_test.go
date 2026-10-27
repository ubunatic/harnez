package pickdocs

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"ubunatic.com/harnez"
)

func TestMarkdownAndManagedBlock(t *testing.T) {
	md, err := Markdown(harnez.DefaultFS)
	if err != nil {
		t.Fatal(err)
	}
	if Begin != "<!-- harnez:model-picks:begin -->" || End != "<!-- harnez:model-picks:end -->" {
		t.Fatal("wrong model picks markers")
	}
	for _, want := range []string{"shipped picks", "models.yaml", "harnez agent models --picks", "| developer | lean |", "terra:low", "| reviewer | smart |", "sol:low", "Efficient standard work."} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in %s", want, md)
		}
	}
	body := "before\n" + Begin + "\nstale\n" + End + "\nafter\n"
	got, err := Render(harnez.DefaultFS, body)
	if err != nil || !strings.Contains(got, md) || strings.Contains(got, "stale") {
		t.Fatalf("render=%q err=%v", got, err)
	}
	again, err := Render(harnez.DefaultFS, got)
	if err != nil || again != got {
		t.Fatalf("non-idempotent render: %v", err)
	}
	if _, err := Render(harnez.DefaultFS, Begin+"\nmissing end"); err == nil {
		t.Fatal("malformed block accepted")
	}
}

func TestNoModelPicksInPractice(t *testing.T) {
	re := regexp.MustCompile(`\b(luna|sol|terra|opus|haiku|argon):(low|med|high)\b`)
	check := func(s string) bool { return re.MatchString(WithoutBlocks(s)) }
	if !check("Use luna:med") || check(Begin+"\nluna:med\n"+End) {
		t.Fatal("guard or block exemption failed")
	}
	for _, dir := range []string{"../../docs/commands", "../../docs/practices"} {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if check(string(data)) {
				t.Errorf("model pick in %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestSourceBlocksCurrent(t *testing.T) {
	for _, path := range []string{"../../docs/practices/AgenticLoop.md", "../../docs/practices/AgenticLoop.lite.md", "../../docs/practices/ModelRoles.md", "../../docs/commands/lean-sprint.md", "../../docs/commands/lean-sprint-v1.md", "../../docs/commands/epic.md", "../../docs/commands/HarnezStatus.md"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), Begin) {
			t.Errorf("%s lacks block", path)
			continue
		}
		got, err := Render(harnez.DefaultFS, string(data))
		if err != nil || got != string(data) {
			t.Errorf("%s stale: %v", path, err)
		}
	}
}
