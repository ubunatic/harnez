package find

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ScanRepo walks root and extracts code/doc chunks.
func ScanRepo(root string) ([]Chunk, error) {
	var chunks []Chunk

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)

		// Ignore standard directories and test fixtures
		if info.IsDir() {
			base := info.Name()
			if base == "vendor" || base == ".git" || base == ".harnez" || base == "testdata" || base == "fixtures" || strings.HasPrefix(base, ".") {
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Ignore hidden files and test files
		base := info.Name()
		if strings.HasPrefix(base, ".") {
			return nil
		}

		if strings.HasSuffix(base, ".go") {
			if strings.HasSuffix(base, "_test.go") {
				return nil
			}
			goChunks, err := ScanGoFile(root, rel, path)
			if err == nil {
				chunks = append(chunks, goChunks...)
			}
		} else if strings.HasSuffix(base, ".md") {
			mdChunks, err := ScanMarkdownFile(root, rel, path)
			if err == nil {
				chunks = append(chunks, mdChunks...)
			}
		}

		return nil
	})

	return chunks, err
}

// ScanGoFile scans a single Go file using go/parser.
func ScanGoFile(root, relPath, fullPath string) ([]Chunk, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, fullPath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var chunks []Chunk
	pkgName := node.Name.Name

	// Extract imports
	var imports []string
	for _, imp := range node.Imports {
		if imp.Path != nil {
			unquoted, err := strconv.Unquote(imp.Path.Value)
			if err == nil {
				imports = append(imports, unquoted)
			}
		}
	}

	// Package doc
	if node.Doc != nil {
		pStart := fset.Position(node.Doc.Pos())
		pEnd := fset.Position(node.Doc.End())
		docText := strings.TrimSpace(node.Doc.Text())
		if docText != "" {
			chunks = append(chunks, Chunk{
				FilePath:   relPath,
				Package:    pkgName,
				Kind:       KindPackageDoc,
				Identifier: pkgName,
				LineStart:  pStart.Line,
				LineEnd:    pEnd.Line,
				Summary:    docText,
				Imports:    imports,
			})
		}
	}

	// Iterate declarations
	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.TYPE {
				for _, spec := range d.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					pos := fset.Position(d.Pos())
					endPos := fset.Position(d.End())

					docText := ""
					if d.Doc != nil {
						docText = strings.TrimSpace(d.Doc.Text())
					} else if typeSpec.Doc != nil {
						docText = strings.TrimSpace(typeSpec.Doc.Text())
					}

					var buf bytes.Buffer
					// Render type spec without heavy details
					_ = printer.Fprint(&buf, fset, typeSpec)
					typeSig := buf.String()

					summary := typeSig
					if docText != "" {
						summary = docText + "\n" + typeSig
					}

					chunks = append(chunks, Chunk{
						FilePath:   relPath,
						Package:    pkgName,
						Kind:       KindType,
						Identifier: typeSpec.Name.Name,
						LineStart:  pos.Line,
						LineEnd:    endPos.Line,
						Summary:    summary,
						Imports:    imports,
					})
				}
			}
		case *ast.FuncDecl:
			pos := fset.Position(d.Pos())
			endPos := fset.Position(d.End())

			docText := ""
			if d.Doc != nil {
				docText = strings.TrimSpace(d.Doc.Text())
			}

			// Format signature without function body
			sig := formatFuncSignature(fset, d)

			summary := sig
			if docText != "" {
				summary = docText + "\n" + sig
			}

			ident := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				recvType := formatExpr(fset, d.Recv.List[0].Type)
				ident = fmt.Sprintf("(%s).%s", recvType, d.Name.Name)
			}

			chunks = append(chunks, Chunk{
				FilePath:   relPath,
				Package:    pkgName,
				Kind:       KindFunc,
				Identifier: ident,
				LineStart:  pos.Line,
				LineEnd:    endPos.Line,
				Summary:    summary,
				Imports:    imports,
			})
		}
	}

	return chunks, nil
}

func formatFuncSignature(fset *token.FileSet, d *ast.FuncDecl) string {
	// Temporarily clear body to print signature only
	body := d.Body
	d.Body = nil
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, fset, d)
	d.Body = body
	return strings.TrimSpace(buf.String())
}

func formatExpr(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, fset, expr)
	return strings.TrimSpace(buf.String())
}

// ScanMarkdownFile scans a Markdown file and slices sections by H1/H2/H3 headers.
func ScanMarkdownFile(root, relPath, fullPath string) ([]Chunk, error) {
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var chunks []Chunk
	scanner := bufio.NewScanner(file)

	type section struct {
		title     string
		lineStart int
		lines     []string
	}

	var current *section
	lineNum := 0

	finishSection := func() {
		if current == nil {
			return
		}
		body := strings.TrimSpace(strings.Join(current.lines, "\n"))
		if body != "" {
			chunks = append(chunks, Chunk{
				FilePath:   relPath,
				Kind:       KindDocSection,
				Identifier: current.title,
				LineStart:  current.lineStart,
				LineEnd:    lineNum,
				Summary:    body,
			})
		}
		current = nil
	}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			finishSection()
			headingTitle := strings.TrimLeft(trimmed, "# ")
			headingTitle = strings.TrimSpace(headingTitle)
			current = &section{
				title:     headingTitle,
				lineStart: lineNum,
				lines:     []string{line},
			}
		} else if current != nil {
			current.lines = append(current.lines, line)
		}
	}
	finishSection()

	return chunks, scanner.Err()
}
