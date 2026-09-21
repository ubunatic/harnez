package telemetry

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestEmbeddedTelemetrySpecIsValid(t *testing.T) {
	s := mustTelemetrySQL()
	if strings.TrimSpace(s.Schema) == "" || len(s.Statements) < 6 {
		t.Fatal("embedded telemetry spec is incomplete")
	}
	for name, statement := range s.Statements {
		if strings.TrimSpace(statement) == "" {
			t.Fatalf("statement %q is empty", name)
		}
	}
}

func TestTelemetrySQLLiteralsMovedToSpec(t *testing.T) {
	for _, name := range []string{"schema.go", "insert.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			data, err = os.ReadFile("internal/telemetry/" + name)
		}
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, data, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value := strings.ToUpper(literal.Value)
			for _, keyword := range []string{"INSERT INTO", "CREATE TABLE", "CREATE INDEX"} {
				if strings.Contains(value, keyword) {
					t.Errorf("%s still contains moved SQL literal %q", name, keyword)
				}
			}
			return true
		})
		if t.Failed() {
			return
		}
	}
}
