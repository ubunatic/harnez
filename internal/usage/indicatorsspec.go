package usage

import (
	"fmt"
	"io/fs"
	"math"
	"strings"
	"sync"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/rograph"
)

const indicatorsSpecPath = "spec/indicators.yaml"

// indicatorsSpec mirrors spec/schemas/indicators.schema.json. The parser
// additionally fixes the semantic endpoints of the finite time gauge.
type indicatorsSpec struct {
	TimeoutSnake  indicatorSequence `yaml:"timeout-snake"`
	UsageBar      usageBarSpec      `yaml:"usage-bar"`
	LoadSparkline glyphSequence     `yaml:"load-sparkline"`
}

type usageBarSpec struct {
	Filled       string         `yaml:"filled"`
	Empty        string         `yaml:"empty"`
	SubCharacter []string       `yaml:"sub-character"`
	Wrapper      barWrapperSpec `yaml:"wrapper"`
}

// barWrapperSpec (issue 159) controls the `[`/`]` brackets drawn around a
// usage bar. Enabled false maps to rograph.BarOptions.NoWrapper; Left/Right
// map to BarOptions.Left/Right and are only consulted when Enabled is true.
type barWrapperSpec struct {
	Enabled bool   `yaml:"enabled"`
	Left    string `yaml:"left"`
	Right   string `yaml:"right"`
}

type glyphSequence struct {
	Frames []string `yaml:"frames"`
}

type indicatorSequence struct {
	Title  string   `yaml:"title"`
	Frames []string `yaml:"frames"`
}

func parseIndicatorsYAML(data []byte) (indicatorsSpec, error) {
	var spec indicatorsSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: parse: %w", err)
	}
	sequence := spec.TimeoutSnake
	if strings.TrimSpace(sequence.Title) == "" {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: timeout-snake: missing title")
	}
	if len(sequence.Frames) < 2 {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: timeout-snake: need at least two frames")
	}
	for i, frame := range sequence.Frames {
		if utf8.RuneCountInString(frame) != 1 {
			return indicatorsSpec{}, fmt.Errorf("indicators spec: timeout-snake frame %d: want one rune", i)
		}
		for _, r := range frame {
			if r < '\u2800' || r > '\u28ff' {
				return indicatorsSpec{}, fmt.Errorf("indicators spec: timeout-snake frame %d: %q is not braille", i, frame)
			}
		}
	}
	if err := validateOneRune("usage-bar filled", spec.UsageBar.Filled); err != nil {
		return indicatorsSpec{}, err
	}
	if err := validateOneRune("usage-bar empty", spec.UsageBar.Empty); err != nil {
		return indicatorsSpec{}, err
	}
	if spec.UsageBar.Filled == spec.UsageBar.Empty {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: usage-bar: filled and empty must differ")
	}
	if len(spec.UsageBar.SubCharacter) == 0 {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: usage-bar: need sub-character frames")
	}
	for i, frame := range spec.UsageBar.SubCharacter {
		if err := validateOneRune(fmt.Sprintf("usage-bar sub-character frame %d", i), frame); err != nil {
			return indicatorsSpec{}, err
		}
	}
	if spec.UsageBar.Wrapper.Enabled && spec.UsageBar.Wrapper.Left == "" && spec.UsageBar.Wrapper.Right != "" {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: usage-bar wrapper: left is empty but right is set")
	}
	if spec.UsageBar.Wrapper.Enabled && spec.UsageBar.Wrapper.Right == "" && spec.UsageBar.Wrapper.Left != "" {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: usage-bar wrapper: right is empty but left is set")
	}
	if len(spec.LoadSparkline.Frames) < 2 {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: load-sparkline: need at least two frames")
	}
	for i, frame := range spec.LoadSparkline.Frames {
		if err := validateOneRune(fmt.Sprintf("load-sparkline frame %d", i), frame); err != nil {
			return indicatorsSpec{}, err
		}
	}
	return spec, nil
}

func validateOneRune(name, value string) error {
	if utf8.RuneCountInString(value) != 1 {
		return fmt.Errorf("indicators spec: %s: want one rune", name)
	}
	return nil
}

func loadIndicators() (indicatorsSpec, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, indicatorsSpecPath)
	if err != nil {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: read %s: %w", indicatorsSpecPath, err)
	}
	return parseIndicatorsYAML(data)
}

var indicatorsOnce = sync.OnceValues(loadIndicators)

func mustIndicators() indicatorsSpec {
	spec, err := indicatorsOnce()
	if err != nil {
		panic(fmt.Sprintf("harnez usage: embedded %s is invalid: %v", indicatorsSpecPath, err))
	}
	return spec
}

// timeoutSnakeGlyph maps remaining freshness time to the finite, ordered
// sequence from spec/indicators.yaml. A fresh reading is full; timeout and
// unknown freshness are empty. No frame wraps, so this is a time gauge rather
// than a spinner.
func timeoutSnakeGlyph(fraction float64) string {
	frames := mustIndicators().TimeoutSnake.Frames
	if fraction <= 0 {
		return frames[len(frames)-1]
	}
	if fraction >= 1 {
		return frames[0]
	}
	idx := int(math.Round((1 - fraction) * float64(len(frames)-1)))
	return frames[idx]
}

func watchBarOptions() rograph.BarOptions {
	return barOptionsFromSpec(mustIndicators().UsageBar)
}

// barOptionsFromSpec converts a parsed usage-bar spec into rograph.BarOptions.
// Split out from watchBarOptions so tests can exercise the
// wrapper-enabled/disabled resolution directly against parsed YAML rather
// than only the embedded default.
func barOptionsFromSpec(spec usageBarSpec) rograph.BarOptions {
	partial := make([]rune, len(spec.SubCharacter))
	for i, glyph := range spec.SubCharacter {
		partial[i] = []rune(glyph)[0]
	}
	opts := rograph.BarOptions{
		Fill:               []rune(spec.Filled)[0],
		Empty:              []rune(spec.Empty)[0],
		SubChar:            true,
		SubCharacterGlyphs: partial,
		ANSI:               true,
		NoWrapper:          !spec.Wrapper.Enabled,
	}
	// Left/Right only apply when the wrapper is enabled -- rograph.RenderBar
	// writes whatever Left/Right hold regardless of NoWrapper, so leaving
	// them at the spec's configured glyphs while NoWrapper is true would let
	// the brackets leak back in even though the spec turned them off.
	if spec.Wrapper.Enabled {
		opts.Left = spec.Wrapper.Left
		opts.Right = spec.Wrapper.Right
	}
	return opts
}

func watchPercentSparkline(values []float64, width int) string {
	frames := mustIndicators().LoadSparkline.Frames
	glyphs := make([]rune, len(frames))
	for i, glyph := range frames {
		glyphs[i] = []rune(glyph)[0]
	}
	return rograph.RenderPercentSparkline(values, rograph.SparklineOptions{Width: width, Glyphs: glyphs, ANSI: true})
}
