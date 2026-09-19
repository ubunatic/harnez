// Command fill-missing-glyphs adds the editable question-mark matrix for glyphs
// absent from both a size-specific YAML spec and its upstream BDF.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type glyphSpec struct {
	Glyphs map[string][]string `yaml:"glyphs"`
}

var sizes = []string{"3x5", "5x8", "6x12", "7x13", "8x16"}

func main() {
	specDir := flag.String("spec-dir", "internal/readcard/spec", "directory containing charset.yaml and glyphs-*.yaml")
	upstreamDir := flag.String("upstream-dir", "internal/readcard/upstream", "directory containing upstream BDF files")
	dryRun := flag.Bool("dry-run", false, "report changes without writing files")
	flag.Parse()

	charset, err := readCharset(filepath.Join(*specDir, "charset.yaml"))
	if err != nil {
		fatal(err)
	}
	for _, size := range sizes {
		path := filepath.Join(*specDir, "glyphs-"+size+".yaml")
		changed, err := fillFile(path, bdfPath(*upstreamDir, size), charset, *dryRun)
		if err != nil {
			fatal(err)
		}
		if changed {
			fmt.Println(path)
		}
	}
}

func readCharset(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var file struct {
		Charset string `yaml:"charset"`
	}
	if err := yaml.Unmarshal(b, &file); err != nil {
		return "", fmt.Errorf("read charset: %w", err)
	}
	if file.Charset == "" {
		return "", fmt.Errorf("read charset: empty charset")
	}
	return file.Charset, nil
}

func fillFile(path, bdf, charset string, dryRun bool) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var spec glyphSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if spec.Glyphs == nil {
		spec.Glyphs = map[string][]string{}
	}
	fallback, ok := spec.Glyphs["?"]
	if !ok {
		return false, fmt.Errorf("%s: missing ? matrix", path)
	}
	upstream, err := bdfCodes(bdf)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	var missing []string
	for _, r := range []rune(charset) {
		key := string(r)
		if _, inSpec := spec.Glyphs[key]; !inSpec && !upstream[r] {
			spec.Glyphs[key] = append([]string(nil), fallback...)
			missing = append(missing, key)
		}
	}
	if len(missing) == 0 || dryRun {
		return len(missing) > 0, nil
	}
	encoded, err := yaml.Marshal(spec)
	if err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, os.WriteFile(path, append([]byte("# Only glyphs absent from the embedded upstream font, plus editable fallbacks.\n"), encoded...), 0644)
}

func bdfPath(dir, size string) string {
	names := map[string]string{"3x5": "tom-thumb.bdf", "6x12": "spleen-6x12.bdf", "7x13": "7x13.bdf", "8x16": "spleen-8x16.bdf"}
	if names[size] == "" {
		return ""
	}
	return filepath.Join(dir, names[size])
}

func bdfCodes(path string) (map[rune]bool, error) {
	codes := map[rune]bool{}
	if path == "" {
		return codes, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return codes, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "ENCODING" {
			code, err := strconv.Atoi(fields[1])
			if err == nil && code >= 0 {
				codes[rune(code)] = true
			}
		}
	}
	return codes, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "fill-missing-glyphs:", err)
	os.Exit(1)
}
