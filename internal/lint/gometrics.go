package lint

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// FuncMetrics measures one function or method.
type FuncMetrics struct {
	Name   string `json:"name"`
	Line   int    `json:"line"`
	Lines  int    `json:"lines"`
	Params int    `json:"params"`
}

// GoFileMetrics measures one Go source file.
type GoFileMetrics struct {
	Package   string        `json:"package"`
	Lines     int           `json:"lines"`
	Test      bool          `json:"test"`
	Generated bool          `json:"generated"`
	Imports   []string      `json:"imports"`
	Funcs     []FuncMetrics `json:"funcs"`
}

// MeasureGo parses Go source and reports its size metrics.
func MeasureGo(path string, content []byte) (GoFileMetrics, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, content, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return GoFileMetrics{}, fmt.Errorf("parse %s: %w", path, err)
	}
	m := GoFileMetrics{
		Package:   f.Name.Name,
		Lines:     bytes.Count(content, []byte("\n")),
		Test:      strings.HasSuffix(path, "_test.go"),
		Generated: ast.IsGenerated(f),
	}
	if len(content) > 0 && content[len(content)-1] != '\n' {
		m.Lines++
	}
	for _, imp := range f.Imports {
		m.Imports = append(m.Imports, strings.Trim(imp.Path.Value, `"`))
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		start, end := fset.Position(fn.Pos()).Line, fset.Position(fn.End()).Line
		m.Funcs = append(m.Funcs, FuncMetrics{
			Name:   funcName(fn),
			Line:   start,
			Lines:  end - start + 1,
			Params: countFields(fn.Type.Params),
		})
	}
	return m, nil
}

func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	t := fn.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if idx, ok := t.(*ast.IndexExpr); ok {
		t = idx.X
	} else if idx, ok := t.(*ast.IndexListExpr); ok {
		t = idx.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

func countFields(fl *ast.FieldList) int {
	if fl == nil {
		return 0
	}
	n := 0
	for _, f := range fl.List {
		n += max(1, len(f.Names))
	}
	return n
}

// GoQualityRule reports Go files and functions that exceed the size limits in
// spec/lint.yaml as warnings: go-file-length, go-func-length, go-params and
// go-import-fanout. Generated files are skipped.
type GoQualityRule struct {
	limits GoLimits
}

// NewGoQualityRule builds the rule from explicit limits.
func NewGoQualityRule(limits GoLimits) *GoQualityRule {
	return &GoQualityRule{limits: limits}
}

func (r *GoQualityRule) ID() string { return "go-quality" }

func (r *GoQualityRule) Description() string {
	return "Warn on Go files, functions, parameter lists and imports above spec/lint.yaml limits"
}

func (r *GoQualityRule) Languages() []Language { return []Language{LangGo} }

func (r *GoQualityRule) Check(file string, content []byte) []Finding {
	m, err := MeasureGo(file, content)
	if err != nil {
		return []Finding{{File: file, Line: 1, Col: 1, RuleID: "go-parse", Severity: SeverityError, Message: err.Error()}}
	}
	if m.Generated {
		return nil
	}
	var findings []Finding
	warn := func(line int, rule, msg string) {
		findings = append(findings, Finding{File: file, Line: line, Col: 1, RuleID: rule, Severity: SeverityWarning, Message: msg})
	}
	fileLimit := r.limits.FileLines
	if m.Test {
		fileLimit = r.limits.TestFileLines
	}
	if m.Lines > fileLimit {
		warn(1, "go-file-length", fmt.Sprintf("file has %d lines (limit %d); split it by concern", m.Lines, fileLimit))
	}
	if len(m.Imports) > r.limits.Imports {
		warn(1, "go-import-fanout", fmt.Sprintf("file imports %d packages (limit %d)", len(m.Imports), r.limits.Imports))
	}
	for _, fn := range m.Funcs {
		if fn.Lines > r.limits.FuncLines {
			warn(fn.Line, "go-func-length", fmt.Sprintf("%s has %d lines (limit %d)", fn.Name, fn.Lines, r.limits.FuncLines))
		}
		if fn.Params > r.limits.Params {
			warn(fn.Line, "go-params", fmt.Sprintf("%s takes %d parameters (limit %d)", fn.Name, fn.Params, r.limits.Params))
		}
	}
	return findings
}
