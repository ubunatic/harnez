package usage

import (
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/rograph"
)

// colorsSpecPath is the embedded location of the named ANSI color registry
// (issue 136). rograph's ANSI background default -- used by
// RenderBar/RenderSparkline whenever a caller leaves BackgroundANSI empty --
// is sourced from here rather than a hardcoded Go string literal.
const colorsSpecPath = "spec/colors.yaml"

// watchColor is one entry in spec/colors.yaml. Mirrors
// spec/schemas/colors.schema.json field-for-field.
type watchColor struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description,omitempty"`
	SGR         string `yaml:"sgr"`
}

// watchColorsSpec is the top-level shape of spec/colors.yaml.
type watchColorsSpec struct {
	Colors map[string]watchColor `yaml:"colors"`
}

// parseWatchColorsYAML parses and validates spec/colors.yaml content. It
// returns a clear error (never panics) for malformed YAML or a spec missing
// required fields -- the same "fail clearly, not panic" contract issue 132
// established for spec/actions.yaml.
func parseWatchColorsYAML(data []byte) (watchColorsSpec, error) {
	var spec watchColorsSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return watchColorsSpec{}, fmt.Errorf("colors spec: parse: %w", err)
	}
	if len(spec.Colors) == 0 {
		return watchColorsSpec{}, fmt.Errorf("colors spec: no colors defined")
	}
	for name, c := range spec.Colors {
		if strings.TrimSpace(c.Title) == "" {
			return watchColorsSpec{}, fmt.Errorf("colors spec: color %q: missing title", name)
		}
		if strings.TrimSpace(c.SGR) == "" {
			return watchColorsSpec{}, fmt.Errorf("colors spec: color %q: missing sgr", name)
		}
	}
	return spec, nil
}

// loadWatchColors reads and validates the embedded spec/colors.yaml.
func loadWatchColors() (watchColorsSpec, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, colorsSpecPath)
	if err != nil {
		return watchColorsSpec{}, fmt.Errorf("colors spec: read %s: %w", colorsSpecPath, err)
	}
	return parseWatchColorsYAML(data)
}

var watchColorsOnce = sync.OnceValues(loadWatchColors)

// mustWatchColors returns the parsed embedded colors spec. It is expected to
// always succeed -- spec/colors.yaml is compiled into the binary and covered
// by TestEmbeddedColorsSpecIsValid -- so a failure here means the binary
// itself was built with a broken spec, not a runtime/user condition.
func mustWatchColors() watchColorsSpec {
	spec, err := watchColorsOnce()
	if err != nil {
		panic(fmt.Sprintf("harnez usage: embedded %s is invalid: %v", colorsSpecPath, err))
	}
	return spec
}

// panelBackgroundSGR returns the named color's SGR code from the embedded
// spec/colors.yaml, panicking if name is undefined (same "broken build, not
// a runtime condition" reasoning as mustWatchColors).
func panelBackgroundSGR(name string) string {
	spec := mustWatchColors()
	c, ok := spec.Colors[name]
	if !ok {
		panic(fmt.Sprintf("harnez usage: embedded %s missing color %q", colorsSpecPath, name))
	}
	return c.SGR
}

// init wires rograph's ANSI background default to the "panel-bg" color
// spec/colors.yaml defines. internal/rograph is a separate, dependency-free
// package (no imports beyond stdlib, per its package doc) and must not gain
// spec-loading machinery of its own, so the spec lookup happens here in
// internal/usage -- which already imports rograph -- and the resolved value
// is pushed into rograph as a plain string default. This avoids both an
// import cycle (rograph would need to import usage to self-resolve) and a
// layering violation (rograph would need to know about spec/ and yaml
// parsing).
func init() {
	rograph.DefaultBackgroundANSI = panelBackgroundSGR("panel-bg")
}
