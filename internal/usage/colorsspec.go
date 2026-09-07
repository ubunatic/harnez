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
		if name != "background" && name != "panel-bg" && name != "time-gauge-bg" && strings.TrimSpace(c.SGR) == "" {
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

// colorSGR returns the named color's SGR code from the embedded
// spec/colors.yaml, panicking if name is undefined (same "broken build, not
// a runtime condition" reasoning as mustWatchColors). It is the single
// lookup used by every named-color call site in this package (issue 137) --
// callers that need a full escape sequence use ansiOpen/ansiWrap below
// rather than reconstructing "\x1b[" + code + "m" themselves.
func colorSGR(name string) string {
	spec := mustWatchColors()
	c, ok := spec.Colors[name]
	if !ok {
		panic(fmt.Sprintf("harnez usage: embedded %s missing color %q", colorsSpecPath, name))
	}
	return c.SGR
}

// ansiOpen returns the SGR "open" escape sequence ("\x1b[<code>m") for a
// named spec/colors.yaml entry. The matching "\x1b[0m" reset is left as a
// raw literal at call sites rather than a named spec entry -- it is the
// universal closer for any SGR sequence, not a color/style choice, mirroring
// how issue 136 already left RenderBar/RenderSparkline's reset as a literal.
func ansiOpen(name string) string {
	return "\x1b[" + colorSGR(name) + "m"
}

// ansiWrap wraps s in the named color's open sequence and a plain "\x1b[0m"
// reset.
func ansiWrap(name, s string) string {
	return ansiOpen(name) + s + "\x1b[0m"
}

// ansiWrapPreservingResets is ansiWrap for strings that already contain their
// own embedded "\x1b[0m" resets (e.g. a compact quota line built from
// rograph.RenderBar segments, which each close with their own reset). A
// plain ansiWrap would only dim up to the first embedded reset -- SGR "0" is
// a full attribute reset, so anything after it reverts to the terminal's
// normal styling, silently un-dimming the rest of the line. This re-asserts
// the named color immediately after every embedded reset so the whole string
// reads uniformly styled end to end.
func ansiWrapPreservingResets(name, s string) string {
	if s == "" {
		return s
	}
	open := ansiOpen(name)
	body := strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+open)
	return open + body + "\x1b[0m"
}

// staleValueANSI applies the staleness/de-emphasis convention issue 107
// introduces to a rendered quota line: dim-grey (\x1b[90m), reconciling the
// two pre-existing "dim" conventions this codebase carried (dim-grey,
// pervasive across watch.go, vs. dim-faint/\x1b[2m, previously used only by
// RenderText's "sources:" note). dim-grey was picked as the sole
// de-emphasis/staleness convention going forward because (a) it was already
// the dominant convention by a wide margin, and (b) it substitutes an
// explicit gray foreground color rather than relying on the SGR "faint"
// attribute, which a meaningful share of terminal emulators render
// identically to normal-intensity text -- dim-grey's degradation (falling
// back to a literal gray-256/ANSI color the terminal's palette maps
// somehow) is more consistent across emulators than faint's (silently doing
// nothing). RenderText's sources note was migrated onto this convention
// too, so dim-faint (still a valid named color in spec/colors.yaml) has no
// remaining call site; it is left defined rather than deleted in case a
// future feature genuinely wants the distinct "faint" semantic rather than
// "de-emphasized."
func staleValueANSI(s string) string {
	return ansiWrapPreservingResets("dim-grey", s)
}

// Named ANSI open-sequence vars for the watch TUI's most common call sites
// (issue 137's color/style audit). These resolve spec/colors.yaml once, at
// package init, so watch.go/usage.go call sites can concatenate them into
// format strings without a lookup per render.
var (
	ansiBold     = ansiOpen("bold")
	ansiDimGrey  = ansiOpen("dim-grey")
	ansiDimFaint = ansiOpen("dim-faint")
)

// init wires rograph's ANSI background default to the "background" color
// spec/colors.yaml defines (resolving to empty string / terminal-native background).
// internal/rograph is a separate, dependency-free package (no imports beyond stdlib,
// per its package doc) and must not gain spec-loading machinery of its own, so the
// spec lookup happens here in internal/usage -- which already imports rograph -- and
// the resolved value is pushed into rograph as a plain string default. This avoids
// both an import cycle (rograph would need to import usage to self-resolve) and a
// layering violation (rograph would need to know about spec/ and yaml parsing).
func init() {
	rograph.DefaultBackgroundANSI = colorSGR("background")
}
