package lint

import (
	"bytes"
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// specPath holds the code-quality thresholds; see docs/Spec.md.
const specPath = "spec/lint.yaml"

// GoLimits are the thresholds of the Go quality rules.
type GoLimits struct {
	FileLines     int `yaml:"file_lines"`
	TestFileLines int `yaml:"test_file_lines"`
	FuncLines     int `yaml:"func_lines"`
	Params        int `yaml:"params"`
	Imports       int `yaml:"imports"`
}

// HotspotSpec sizes the `harnez assess --hotspots` tables.
type HotspotSpec struct {
	Top int `yaml:"top"`
}

// ReportSpec shapes the text report of `harnez lint`.
type ReportSpec struct {
	TopFiles int `yaml:"top_files"`
}

// Spec is the top-level shape of spec/lint.yaml.
type Spec struct {
	Go       GoLimits    `yaml:"go"`
	Report   ReportSpec  `yaml:"report"`
	Hotspots HotspotSpec `yaml:"hotspots"`
}

// LoadSpec reads and validates the embedded lint specification.
func LoadSpec() (*Spec, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, specPath)
	if err != nil {
		return nil, fmt.Errorf("read embedded %s: %w", specPath, err)
	}
	return ParseSpec(data)
}

// ParseSpec unmarshals and validates lint specification YAML bytes.
func ParseSpec(data []byte) (*Spec, error) {
	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("%s: %w", specPath, err)
	}
	for name, v := range map[string]int{
		"go.file_lines":      s.Go.FileLines,
		"go.test_file_lines": s.Go.TestFileLines,
		"go.func_lines":      s.Go.FuncLines,
		"go.params":          s.Go.Params,
		"go.imports":         s.Go.Imports,
		"report.top_files":   s.Report.TopFiles,
		"hotspots.top":       s.Hotspots.Top,
	} {
		if v < 1 {
			return nil, fmt.Errorf("%s: %s must be a positive integer, got %d", specPath, name, v)
		}
	}
	return &s, nil
}
