package transcript

import (
	"os"
	"reflect"
	"testing"
)

func TestCommandsFixtures(t *testing.T) {
	for _, tt := range []struct {
		name string
		want []string
	}{
		{"claude", []string{"printf hello", "git status\ngit diff"}},
		{"codex", []string{"pwd", "git status", "printf hi", "'bash' '-lc' 'echo hi'"}},
		{"codex-exec", []string{"git status --short\nrg --files cmd/example", "printf 'hello'", "first line\nsecond line", `echo "quoted"`}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open("testdata/" + tt.name + "-commands.jsonl")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			var got []string
			if err := Commands(f, func(s string) error { got = append(got, s); return nil }); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}
