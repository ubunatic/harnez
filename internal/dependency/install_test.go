package dependency

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func TestInstall(t *testing.T) {
	for _, tc := range []struct {
		name               string
		dry                bool
		failGo, failNeus   bool
		wantErr, wantCalls string
	}{
		{name: "success", wantCalls: "go install ubunatic.com/neus/cmd/neus@latest\nneus install\n"},
		{name: "dry", dry: true, wantCalls: ""},
		{name: "go failure", failGo: true, wantErr: "go install", wantCalls: "go install ubunatic.com/neus/cmd/neus@latest\n"},
		{name: "bootstrap failure", failNeus: true, wantErr: "neus install", wantCalls: "go install ubunatic.com/neus/cmd/neus@latest\nneus install\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			log := filepath.Join(dir, "calls")
			t.Setenv("CALLS", log)
			t.Setenv("PATH", dir)
			for _, name := range []string{"go", "neus"} {
				script := "#!/bin/sh\nprintf '%s' '" + name + "' >> \"$CALLS\"\nprintf ' %s' \"$@\" >> \"$CALLS\"\nprintf '\\n' >> \"$CALLS\"\n"
				if name == "go" && tc.failGo || name == "neus" && tc.failNeus {
					script += "exit 7\n"
				}
				if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0755); err != nil {
					t.Fatal(err)
				}
			}
			var out bytes.Buffer
			err := Install(context.Background(), "neus", tc.dry, nil, &out, &out)
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("error = %v", err)
			}
			data, readErr := os.ReadFile(log)
			if readErr != nil && !os.IsNotExist(readErr) {
				t.Fatal(readErr)
			}
			if string(data) != tc.wantCalls {
				t.Fatalf("calls = %q, want %q", data, tc.wantCalls)
			}
			wantOutput := "go install ubunatic.com/neus/cmd/neus@latest\nneus install\n"
			if tc.dry && out.String() != wantOutput {
				t.Fatalf("dry run output = %q, want %q", out.String(), wantOutput)
			}
			if tc.wantErr != "" && !strings.Contains(out.String(), tc.wantCalls) {
				t.Fatalf("failure output = %q, want attempted command %q", out.String(), tc.wantCalls)
			}
		})
	}
}

func TestUnknownDependencyAndDryRunWithoutTools(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var out bytes.Buffer
	err := Install(context.Background(), "unknown", false, nil, &out, &out)
	if err == nil || !strings.Contains(err.Error(), "supported: neus") || out.Len() != 0 {
		t.Fatalf("error = %v, output = %q", err, out.String())
	}
	if err := Install(context.Background(), "neus", true, nil, &out, &out); err != nil {
		t.Fatal(err)
	}
	want := "go install ubunatic.com/neus/cmd/neus@latest\nneus install\n"
	if out.String() != want {
		t.Fatalf("dry run output = %q, want %q", out.String(), want)
	}
}

func TestDependencySpecSchema(t *testing.T) {
	data, err := harnez.DefaultFS.ReadFile("spec/dependencies.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	data, err = harnez.DefaultFS.ReadFile("spec/schemas/dependencies.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(doc); err != nil {
		t.Fatal(err)
	}
}
