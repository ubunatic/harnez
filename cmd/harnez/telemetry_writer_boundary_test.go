package main

import (
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritableTelemetryOpenIsServerOnly(t *testing.T) {
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.HasPrefix(filepath.Clean(path), filepath.Join(root, "internal", "server")+string(filepath.Separator)) {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Open" {
				return true
			}
			if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "telemetry" {
				t.Errorf("%s: telemetry.Open is writable; route writes through internal/server", path)
			}
			if strings.HasPrefix(filepath.Clean(path), filepath.Join(root, "cmd", "harnez")+string(filepath.Separator)) {
				pkg, ok := selector.X.(*ast.Ident)
				if !ok || pkg.Name != "sql" || len(call.Args) <= 1 {
					return true
				}
				if driver, ok := call.Args[0].(*ast.BasicLit); ok && constant.StringVal(constant.MakeFromLiteral(driver.Value, token.STRING, 0)) == "sqlite" {
					readOnly := false
					ast.Inspect(call.Args[1], func(dsnNode ast.Node) bool {
						literal, ok := dsnNode.(*ast.BasicLit)
						if ok && literal.Kind == token.STRING {
							value := constant.MakeFromLiteral(literal.Value, token.STRING, 0)
							readOnly = readOnly || (value.Kind() == constant.String && strings.Contains(constant.StringVal(value), "mode=ro"))
						}
						return true
					})
					if !readOnly {
						t.Errorf("%s: sql.Open sqlite DSN is not read-only; route writes through internal/server", path)
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
