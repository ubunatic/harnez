package assess

import (
	"cmp"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"ubunatic.com/harnez/internal/lint"
)

// HotspotFile is one Go file ranked by size.
type HotspotFile struct {
	Path  string `json:"path"`
	Lines int    `json:"lines"`
	Test  bool   `json:"test"`
}

// HotspotFunc is one Go function ranked by length.
type HotspotFunc struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	Line   int    `json:"line"`
	Lines  int    `json:"lines"`
	Params int    `json:"params"`
}

// HotspotPackage is one Go package directory ranked by import fan-out.
type HotspotPackage struct {
	Dir       string `json:"dir"`
	Files     int    `json:"files"`
	Lines     int    `json:"lines"`
	Imports   int    `json:"imports"`
	TestFiles int    `json:"test_files"`
}

// HotspotReport lists the largest refactoring targets of a Go repository.
type HotspotReport struct {
	Top        int              `json:"top"`
	Files      []HotspotFile    `json:"files"`
	Funcs      []HotspotFunc    `json:"funcs"`
	Packages   []HotspotPackage `json:"packages"`
	Untested   []HotspotPackage `json:"untested"`
	ParseFails []string         `json:"parse_fails,omitempty"`
}

// AssessHotspots measures the git-tracked, non-generated Go files below dir
// and keeps the top entries per table (spec/lint.yaml hotspots.top).
func AssessHotspots(dir string) (*HotspotReport, error) {
	spec, err := lint.LoadSpec()
	if err != nil {
		return nil, err
	}
	files, err := lint.TrackedFiles(dir, nil)
	if err != nil {
		return nil, err
	}
	rep := &HotspotReport{Top: spec.Hotspots.Top}
	pkgs := map[string]*HotspotPackage{}
	pkgImports := map[string]map[string]bool{}
	for _, name := range files {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		m, err := lint.MeasureGo(name, content)
		if err != nil {
			rep.ParseFails = append(rep.ParseFails, name)
			continue
		}
		if m.Generated {
			continue
		}
		rep.Files = append(rep.Files, HotspotFile{Path: name, Lines: m.Lines, Test: m.Test})
		for _, fn := range m.Funcs {
			rep.Funcs = append(rep.Funcs, HotspotFunc{Path: name, Name: fn.Name, Line: fn.Line, Lines: fn.Lines, Params: fn.Params})
		}
		pkgDir := path.Dir(name)
		p := pkgs[pkgDir]
		if p == nil {
			p = &HotspotPackage{Dir: pkgDir}
			pkgs[pkgDir] = p
			pkgImports[pkgDir] = map[string]bool{}
		}
		if m.Test {
			p.TestFiles++
			continue
		}
		p.Files++
		p.Lines += m.Lines
		for _, imp := range m.Imports {
			pkgImports[pkgDir][imp] = true
		}
	}
	for dir, p := range pkgs {
		p.Imports = len(pkgImports[dir])
		if p.Files == 0 {
			continue // test-only directory
		}
		rep.Packages = append(rep.Packages, *p)
		if p.TestFiles == 0 {
			rep.Untested = append(rep.Untested, *p)
		}
	}
	slices.SortFunc(rep.Files, func(a, b HotspotFile) int { return cmp.Or(b.Lines-a.Lines, strings.Compare(a.Path, b.Path)) })
	slices.SortFunc(rep.Funcs, func(a, b HotspotFunc) int {
		return cmp.Or(b.Lines-a.Lines, strings.Compare(a.Path, b.Path), a.Line-b.Line)
	})
	slices.SortFunc(rep.Packages, func(a, b HotspotPackage) int { return cmp.Or(b.Imports-a.Imports, strings.Compare(a.Dir, b.Dir)) })
	slices.SortFunc(rep.Untested, func(a, b HotspotPackage) int { return cmp.Or(b.Lines-a.Lines, strings.Compare(a.Dir, b.Dir)) })
	rep.Files = rep.Files[:min(rep.Top, len(rep.Files))]
	rep.Funcs = rep.Funcs[:min(rep.Top, len(rep.Funcs))]
	rep.Packages = rep.Packages[:min(rep.Top, len(rep.Packages))]
	return rep, nil
}

// RenderHotspotsText renders the hotspot tables; untested packages are listed in full.
func RenderHotspotsText(r *HotspotReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Largest Go files (top %d)\n", r.Top)
	for _, f := range r.Files {
		fmt.Fprintf(&b, "  %6d  %s\n", f.Lines, f.Path)
	}
	fmt.Fprintf(&b, "\nLongest functions (top %d)\n", r.Top)
	for _, f := range r.Funcs {
		fmt.Fprintf(&b, "  %6d  %s:%d %s (%d params)\n", f.Lines, f.Path, f.Line, f.Name, f.Params)
	}
	fmt.Fprintf(&b, "\nImport fan-out by package (top %d)\n", r.Top)
	for _, p := range r.Packages {
		fmt.Fprintf(&b, "  %6d  %s (%d files, %d lines)\n", p.Imports, p.Dir, p.Files, p.Lines)
	}
	fmt.Fprintf(&b, "\nPackages without tests (%d)\n", len(r.Untested))
	for _, p := range r.Untested {
		fmt.Fprintf(&b, "  %6d  %s\n", p.Lines, p.Dir)
	}
	if len(r.ParseFails) > 0 {
		fmt.Fprintf(&b, "\nUnparsable files (%d): %s\n", len(r.ParseFails), strings.Join(r.ParseFails, ", "))
	}
	return b.String()
}
