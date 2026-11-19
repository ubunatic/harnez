package exec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

const execSchemaPath = "spec/schemas/exec.schema.json"

// execSpecFS serves the real schema plus a spec document built from body.
func execSpecFS(t *testing.T, body string) fstest.MapFS {
	t.Helper()
	schema, err := os.ReadFile(filepath.Join("..", "..", execSchemaPath))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	return fstest.MapFS{
		"spec/exec.yaml": {Data: []byte(body)},
		execSchemaPath:   {Data: schema},
	}
}

const validExecSpec = `truncate:
  enabled: true
  max_output_bytes: 51200
  max_output_lines: 2000
  spill_dir: ""
  spill_prefix: "harnez-exec-"
`

func TestLoadExecSpec_EmbeddedValues(t *testing.T) {
	s, err := LoadExecSpec()
	if err != nil {
		t.Fatalf("LoadExecSpec: %v", err)
	}
	want := TruncateSpec{Enabled: true, MaxOutputBytes: 51200, MaxOutputLines: 2000, SpillDir: "", SpillPrefix: "harnez-exec-"}
	if s.Truncate != want {
		t.Fatalf("Truncate = %+v, want %+v", s.Truncate, want)
	}
}

func TestLoadExecSpecFrom_ValidDocument(t *testing.T) {
	s, err := loadExecSpecFrom(execSpecFS(t, validExecSpec), "spec/exec.yaml", execSchemaPath)
	if err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}
	if s.Truncate.MaxOutputBytes != 51200 || s.Truncate.MaxOutputLines != 2000 {
		t.Fatalf("limits = %d/%d, want 51200/2000", s.Truncate.MaxOutputBytes, s.Truncate.MaxOutputLines)
	}
}

func TestLoadExecSpecFrom_RejectsInvalidDocuments(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"negative bytes", strings.Replace(validExecSpec, "51200", "-1", 1), "max_output_bytes"},
		{"negative lines", strings.Replace(validExecSpec, "2000", "-5", 1), "max_output_lines"},
		{"bytes as string", strings.Replace(validExecSpec, "51200", `"50k"`, 1), "max_output_bytes"},
		{"lines as bool", strings.Replace(validExecSpec, "2000", "true", 1), "max_output_lines"},
		{"enabled as string", strings.Replace(validExecSpec, "enabled: true", `enabled: "yes"`, 1), "enabled"},
		{"missing truncate", "{}\n", "truncate"},
		{"missing key", strings.Replace(validExecSpec, "  spill_dir: \"\"\n", "", 1), "spill_dir"},
		{"unknown key", validExecSpec + "  extra: 1\n", "extra"},
		{"unsafe prefix", strings.Replace(validExecSpec, `"harnez-exec-"`, `"../x"`, 1), "spill_prefix"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadExecSpecFrom(execSpecFS(t, tc.body), "spec/exec.yaml", execSchemaPath)
			if err == nil {
				t.Fatalf("spec accepted, want rejection mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want mention of %q", err, tc.want)
			}
		})
	}
}
