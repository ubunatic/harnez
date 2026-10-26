package claude

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInvokeOnlySkillSources(t *testing.T) {
	cfg := loadTestConfig(t)
	for _, name := range []string{"epic", "reverse-sprint", "lean-sprint-v1"} {
		t.Run(name, func(t *testing.T) {
			for _, skill := range cfg.Skills {
				if skill.Name != name {
					continue
				}
				content, err := genSkillContent(skill, cfg.FS)
				if err != nil {
					t.Fatal(err)
				}
				_, header, _, present, err := parseSkillFrontmatter(content)
				if err != nil || !present {
					t.Fatalf("missing frontmatter: %v", err)
				}
				var fields map[string]any
				if err := yaml.Unmarshal([]byte(header), &fields); err != nil {
					t.Fatal(err)
				}
				if fields["disable-model-invocation"] != true || fields["name"] != name {
					t.Fatalf("generated frontmatter = %v", fields)
				}
				return
			}
			t.Fatal("skill missing from embedded config")
		})
	}
}

func TestGenSkillContentMergesSourceFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "partial source keys and body delimiter",
			body: "---\ndisable-model-invocation: true\n---\nbody\n---\nmore body",
			want: []string{"name: \"refresh-me\"", "description: \"generated description\"", "disable-model-invocation: true", "body\n---\nmore body"},
		},
		{
			name: "source name and description win",
			body: "---\nname: source-name\ndescription: source description\n---\nbody",
			want: []string{"name: \"source-name\"", "description: \"source description\"", "body"},
		},
		{
			name: "CRLF frontmatter",
			body: "---\r\ndisable-model-invocation: true\r\n---\r\nbody\r\n---\r\nmore",
			want: []string{"name: \"refresh-me\"", "disable-model-invocation: true", "body\r\n---\r\nmore"},
		},
		{
			name: "delimiter inside body is not frontmatter",
			body: "body\n---\ndisable-model-invocation: true",
			want: []string{"name: \"refresh-me\"", "body\n---\ndisable-model-invocation: true"},
		},
		{
			name: "source without frontmatter unchanged",
			body: "body without frontmatter",
			want: []string{"name: \"refresh-me\"", "description: \"generated description\"", "body without frontmatter"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := genSkillContent(Command{Name: "refresh-me", Description: "generated description", Content: tt.body}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tt.name == "source without frontmatter unchanged" {
				want := "---\nname: \"refresh-me\"\ndescription: \"generated description\"\n---\nbody without frontmatter\n"
				if content != want {
					t.Errorf("frontmatter-free generated content changed:\n%s", content)
				}
			}
			if !strings.HasPrefix(content, "---\n") {
				t.Fatalf("generated content does not start with frontmatter:\n%s", content)
			}
			closing := strings.Index(content[4:], "\n---\n")
			if closing < 0 {
				t.Fatalf("generated content has no closing frontmatter delimiter:\n%s", content)
			}
			header := content[:4+closing]
			if strings.Count(header, "name:") > 1 || strings.Count(header, "description:") > 1 {
				t.Errorf("generated frontmatter contains duplicate generated keys:\n%s", content)
			}
			for _, want := range tt.want {
				if !strings.Contains(content, want) {
					t.Errorf("generated content missing %q:\n%s", want, content)
				}
			}
		})
	}
}
