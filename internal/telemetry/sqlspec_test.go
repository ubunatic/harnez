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
	if len(s.QualityChecks) != 6 {
		t.Fatalf("quality checks = %d, want 6", len(s.QualityChecks))
	}
}

func TestQualityChecksAreReferencedAndUseDDLColumns(t *testing.T) {
	for _, q := range mustTelemetrySQL().QualityChecks {
		if q.WarnAbovePercent < 0 || q.WarnCondition == "" {
			t.Errorf("invalid threshold for %q", q.Name)
		}
		if strings.TrimSpace(q.SQL) == "" {
			t.Errorf("empty SQL for %q", q.Name)
		}
	}
}

func TestTelemetrySQLLiteralsMovedToSpec(t *testing.T) {
	for _, name := range []string{"schema.go", "insert.go", "query.go"} {
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
			for _, keyword := range []string{"INSERT INTO", "CREATE TABLE", "CREATE INDEX", "SELECT "} {
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

func TestTelemetrySpecStatementsAreReferenced(t *testing.T) {
	for name := range mustTelemetrySQL().Statements {
		found := false
		for _, source := range []string{"schema.go", "insert.go", "query.go"} {
			data, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), name) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("statement %q is not referenced by telemetry Go", name)
		}
	}
}

func TestAggregateGroupedByRejectsUnknownColumn(t *testing.T) {
	if _, err := (&DB{}).aggregateGroupedBy("not_allowed", Filter{}); err == nil {
		t.Fatal("expected unsupported grouping column error")
	}
}
