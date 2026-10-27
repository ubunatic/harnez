package claude

import (
	"strings"
	"testing"
	"testing/fstest"

	"ubunatic.com/harnez/internal/pickdocs"
)

func TestRenderSkillPicksBlock(t *testing.T) {
	fsys := fstest.MapFS{"spec/agent.yaml": {Data: []byte("model_traits: {smart: Difficult work}\nroles: {developer: {default_model: smart, models: {smart: [sol:med]}}}\n")}}
	body := pickdocs.Begin + "\nstale\n" + pickdocs.End
	got, err := renderSkillBody("test", body, fsys)
	if err != nil || strings.Contains(got, "stale") || !strings.Contains(got, "| developer | smart | Difficult work | sol:med | yes |") {
		t.Fatalf("output=%q err=%v", got, err)
	}
}
