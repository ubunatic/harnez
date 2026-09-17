package distill

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"ubunatic.com/harnez/internal/readcard"
)

func TestDistillReadOutputBoundedAndImages(t *testing.T) {
	source := strings.Repeat("line with useful source text\n", 200)
	bounded, err := DistillReadOutput(ReadOutputRequest{Text: source, Path: "never-open-this.txt", MaxTokens: 100, StartLine: 10})
	if err != nil {
		t.Fatal(err)
	}
	if !bounded.Truncated || bounded.NextLine <= 10 || len(bounded.Images) != 0 || !strings.Contains(bounded.Text, "output bounded") {
		t.Fatalf("unbounded or unclear text: %+v", bounded)
	}
	if readcard.ComputeTextTokens(bounded.Text).TextTokens > 100+60 {
		t.Fatal("text budget exceeded beyond continuation label")
	}
	for _, provider := range []readcard.Provider{readcard.ProviderClaude, readcard.ProviderOpenAI, readcard.ProviderGemini, readcard.ProviderUnknown} {
		result, err := DistillReadOutput(ReadOutputRequest{Text: source, Path: "source.go", Vision: true, Provider: provider})
		if err != nil {
			t.Fatal(err)
		}
		if provider == readcard.ProviderUnknown && len(result.Images) != 0 {
			t.Fatal("unknown provider got images")
		}
		if provider == readcard.ProviderClaude && len(result.Images) == 0 {
			t.Fatal("dense Claude source should render")
		}
		for _, path := range result.Images {
			if _, err := os.Stat(path); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(filepath.Dir(path)) })
		}
	}
	unicodeResult, err := DistillReadOutput(ReadOutputRequest{Text: strings.Repeat("世", 100), MaxTokens: 7})
	if err != nil || !utf8.ValidString(unicodeResult.Text) {
		t.Fatal("truncation split UTF-8")
	}
	data, _ := json.Marshal(unicodeResult)
	if !json.Valid(data) {
		t.Fatal("invalid JSON output")
	}
}
