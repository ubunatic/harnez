package usage

import (
	"fmt"
	"io/fs"
	"math"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/rograph"
)

const indicatorsSpecPath = "spec/indicators.yaml"

// indicatorsSpec mirrors spec/schemas/indicators.schema.json. Sequence
// references are resolved by parseIndicatorsYAML, before any renderer sees
// the options, so internal/rograph stays independent of application specs.
type indicatorsSpec struct {
	Sequences             map[string]namedIndicatorSequence `yaml:"sequences"`
	TimeoutSnake          indicatorReference                `yaml:"timeout-snake"`
	UsageBar              usageBarSpec                      `yaml:"usage-bar"`
	LoadSparkline         glyphSequenceReference            `yaml:"load-sparkline"`
	LoadCharts            loadChartsSpec                    `yaml:"load-charts"`
	ChartBackground       string                            `yaml:"chart-background"`
	LoadChartPresentation string                            `yaml:"load-chart-presentation"`
	UsageBarPresentation  string                            `yaml:"usage-bar-presentation"`
}

// LoadChartPresentation selects whether load histories and their current
// percentage labels are plain monochrome or use the shared heat palette.
type LoadChartPresentation string

const (
	LoadChartMonochrome LoadChartPresentation = "monochrome"
	LoadChartHeat       LoadChartPresentation = "heat"
)

// UsageBarPresentation selects whether quota bars and their current
// percentage labels stay monochrome or use the shared heat palette.
// It deliberately has its own setting: load history presentation and quota
// presentation can be chosen independently.
type UsageBarPresentation string

const (
	UsageBarMonochrome UsageBarPresentation = "monochrome"
	UsageBarHeat       UsageBarPresentation = "heat"
)

// LoadChartMode defines the visual presentation mode for load indicators.
type LoadChartMode string

const (
	LoadChartSparkline LoadChartMode = "sparkline"
	LoadChartBraille   LoadChartMode = "braille"
	LoadChartBar       LoadChartMode = "bar"
)

type loadChartsSpec struct {
	CPU  string `yaml:"cpu"`
	GPU  string `yaml:"gpu"`
	RAM  string `yaml:"ram"`
	VRAM string `yaml:"vram"`
}

func (s loadChartsSpec) CPUMode() LoadChartMode {
	return parseLoadChartMode(s.CPU, LoadChartSparkline)
}

func (s loadChartsSpec) GPUMode() LoadChartMode {
	return parseLoadChartMode(s.GPU, LoadChartSparkline)
}

func (s loadChartsSpec) RAMMode() LoadChartMode {
	return parseLoadChartMode(s.RAM, LoadChartSparkline)
}

func (s loadChartsSpec) VRAMMode() LoadChartMode {
	return parseLoadChartMode(s.VRAM, LoadChartSparkline)
}

func parseLoadChartMode(val string, defaultMode LoadChartMode) LoadChartMode {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "sparkline", "timeseries":
		return LoadChartSparkline
	case "braille", "btop":
		return LoadChartBraille
	case "bar", "gauge":
		return LoadChartBar
	case "":
		return defaultMode
	default:
		return defaultMode
	}
}

func validateLoadChartMode(field, val string) error {
	if val == "" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "sparkline", "timeseries", "braille", "btop", "bar", "gauge":
		return nil
	default:
		return fmt.Errorf("indicators spec: load-charts: %s: unknown mode %q", field, val)
	}
}

func (s indicatorsSpec) chartBackgroundName() string {
	if strings.TrimSpace(s.ChartBackground) == "" {
		return "panel-bg"
	}
	return s.ChartBackground
}

func (s indicatorsSpec) chartPresentation() LoadChartPresentation {
	if strings.EqualFold(strings.TrimSpace(s.LoadChartPresentation), string(LoadChartHeat)) {
		return LoadChartHeat
	}
	return LoadChartMonochrome
}

func (s indicatorsSpec) usageBarPresentation() UsageBarPresentation {
	if strings.EqualFold(strings.TrimSpace(s.UsageBarPresentation), string(UsageBarHeat)) {
		return UsageBarHeat
	}
	return UsageBarMonochrome
}

type usageBarSpec struct {
	Filled               string         `yaml:"filled"`
	Empty                string         `yaml:"empty"`
	SubCharacterSequence string         `yaml:"sub-character-sequence"`
	SubCharacter         []string       `yaml:"-"`
	Wrapper              barWrapperSpec `yaml:"wrapper"`
}

// barWrapperSpec (issue 159) controls the `[`/`]` brackets drawn around a
// usage bar. Enabled false maps to rograph.BarOptions.NoWrapper; Left/Right
// map to BarOptions.Left/Right and are only consulted when Enabled is true.
type barWrapperSpec struct {
	Enabled bool   `yaml:"enabled"`
	Left    string `yaml:"left"`
	Right   string `yaml:"right"`
}

type glyphSequenceReference struct {
	Sequence string   `yaml:"sequence"`
	Frames   []string `yaml:"-"`
}

type indicatorReference struct {
	Title    string   `yaml:"title"`
	Sequence string   `yaml:"sequence"`
	Frames   []string `yaml:"-"`
}

type namedIndicatorSequence struct {
	Title  string   `yaml:"title"`
	Kind   string   `yaml:"kind"`
	Frames []string `yaml:"frames"`
}

func parseIndicatorsYAML(data []byte) (indicatorsSpec, error) {
	var spec indicatorsSpec
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&spec); err != nil {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: parse: %w", err)
	}
	if len(spec.Sequences) == 0 {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: sequences: need at least one named sequence")
	}
	for name, sequence := range spec.Sequences {
		if strings.TrimSpace(name) == "" {
			return indicatorsSpec{}, fmt.Errorf("indicators spec: sequences: empty name")
		}
		if strings.TrimSpace(sequence.Title) == "" {
			return indicatorsSpec{}, fmt.Errorf("indicators spec: sequence %q: missing title", name)
		}
		switch sequence.Kind {
		case "countdown", "spinner", "bar-partial", "sparkline":
		default:
			return indicatorsSpec{}, fmt.Errorf("indicators spec: sequence %q: unknown kind %q", name, sequence.Kind)
		}
		if len(sequence.Frames) == 0 {
			return indicatorsSpec{}, fmt.Errorf("indicators spec: sequence %q: need at least one frame", name)
		}
		for i, frame := range sequence.Frames {
			if err := validateOneRune(fmt.Sprintf("sequence %q frame %d", name, i), frame); err != nil {
				return indicatorsSpec{}, err
			}
		}
	}

	if strings.TrimSpace(spec.TimeoutSnake.Title) == "" {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: timeout-snake: missing title")
	}
	timeoutFrames, err := resolveIndicatorSequence(spec.Sequences, "timeout-snake", spec.TimeoutSnake.Sequence, "countdown")
	if err != nil {
		return indicatorsSpec{}, err
	}
	if len(timeoutFrames) < 2 {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: timeout-snake: need at least two frames")
	}
	spec.TimeoutSnake.Frames = timeoutFrames

	if err := validateOneRune("usage-bar filled", spec.UsageBar.Filled); err != nil {
		return indicatorsSpec{}, err
	}
	if err := validateOneRune("usage-bar empty", spec.UsageBar.Empty); err != nil {
		return indicatorsSpec{}, err
	}
	if spec.UsageBar.Filled == spec.UsageBar.Empty {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: usage-bar: filled and empty must differ")
	}
	subCharacterFrames, err := resolveIndicatorSequence(spec.Sequences, "usage-bar sub-character", spec.UsageBar.SubCharacterSequence, "bar-partial")
	if err != nil {
		return indicatorsSpec{}, err
	}
	if len(subCharacterFrames) == 0 {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: usage-bar: need sub-character frames")
	}
	for i, frame := range subCharacterFrames {
		if err := validateOneRune(fmt.Sprintf("usage-bar sub-character frame %d", i), frame); err != nil {
			return indicatorsSpec{}, err
		}
	}
	spec.UsageBar.SubCharacter = subCharacterFrames
	if spec.UsageBar.Wrapper.Enabled && spec.UsageBar.Wrapper.Left == "" && spec.UsageBar.Wrapper.Right != "" {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: usage-bar wrapper: left is empty but right is set")
	}
	if spec.UsageBar.Wrapper.Enabled && spec.UsageBar.Wrapper.Right == "" && spec.UsageBar.Wrapper.Left != "" {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: usage-bar wrapper: right is empty but left is set")
	}
	sparklineFrames, err := resolveIndicatorSequence(spec.Sequences, "load-sparkline", spec.LoadSparkline.Sequence, "sparkline")
	if err != nil {
		return indicatorsSpec{}, err
	}
	if len(sparklineFrames) < 2 {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: load-sparkline: need at least two frames")
	}
	for i, frame := range sparklineFrames {
		if err := validateOneRune(fmt.Sprintf("load-sparkline frame %d", i), frame); err != nil {
			return indicatorsSpec{}, err
		}
	}
	spec.LoadSparkline.Frames = sparklineFrames

	if err := validateLoadChartMode("cpu", spec.LoadCharts.CPU); err != nil {
		return indicatorsSpec{}, err
	}
	if err := validateLoadChartMode("gpu", spec.LoadCharts.GPU); err != nil {
		return indicatorsSpec{}, err
	}
	if err := validateLoadChartMode("ram", spec.LoadCharts.RAM); err != nil {
		return indicatorsSpec{}, err
	}
	if err := validateLoadChartMode("vram", spec.LoadCharts.VRAM); err != nil {
		return indicatorsSpec{}, err
	}
	if strings.TrimSpace(spec.ChartBackground) != "" && spec.ChartBackground != "panel-bg" {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: chart-background: unknown color %q", spec.ChartBackground)
	}
	if presentation := strings.ToLower(strings.TrimSpace(spec.LoadChartPresentation)); presentation != "" && presentation != string(LoadChartMonochrome) && presentation != string(LoadChartHeat) {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: load-chart-presentation: unknown mode %q", spec.LoadChartPresentation)
	}
	if presentation := strings.ToLower(strings.TrimSpace(spec.UsageBarPresentation)); presentation != "" && presentation != string(UsageBarMonochrome) && presentation != string(UsageBarHeat) {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: usage-bar-presentation: unknown mode %q", spec.UsageBarPresentation)
	}

	return spec, nil
}

func resolveIndicatorSequence(registry map[string]namedIndicatorSequence, consumer, name, wantKind string) ([]string, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("indicators spec: %s: missing sequence reference", consumer)
	}
	sequence, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("indicators spec: %s: unknown sequence %q", consumer, name)
	}
	if sequence.Kind != wantKind {
		return nil, fmt.Errorf("indicators spec: %s: sequence %q has kind %q, want %q", consumer, name, sequence.Kind, wantKind)
	}
	return append([]string(nil), sequence.Frames...), nil
}

func validateOneRune(name, value string) error {
	if utf8.RuneCountInString(value) != 1 {
		return fmt.Errorf("indicators spec: %s: want one rune", name)
	}
	if width := runewidth.StringWidth(value); width != 1 {
		return fmt.Errorf("indicators spec: %s: display width %d, want one terminal cell", name, width)
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
	return finiteSequenceGlyph(mustIndicators().TimeoutSnake.Frames, fraction)
}

// finiteSequenceGlyph selects from a resolved determinate sequence without
// assuming its length or endpoint glyphs. Callers must supply a validated,
// non-empty sequence.
func finiteSequenceGlyph(frames []string, fraction float64) string {
	if fraction <= 0 {
		return frames[len(frames)-1]
	}
	if fraction >= 1 {
		return frames[0]
	}
	idx := int(math.Round((1 - fraction) * float64(len(frames)-1)))
	return frames[idx]
}

// namedSequenceFrames returns the frames for a registered spec/indicators.yaml
// sequence by name, validating its kind matches wantKind. It panics on a
// missing name or kind mismatch -- same "broken build, not a runtime
// condition" contract as mustIndicators() itself, since callers only ever
// pass Go-side constants, never user input.
func namedSequenceFrames(name, wantKind string) []string {
	spec := mustIndicators()
	seq, ok := spec.Sequences[name]
	if !ok {
		panic(fmt.Sprintf("harnez usage: embedded %s missing sequence %q", indicatorsSpecPath, name))
	}
	if seq.Kind != wantKind {
		panic(fmt.Sprintf("harnez usage: embedded %s: sequence %q has kind %q, want %q", indicatorsSpecPath, name, seq.Kind, wantKind))
	}
	return seq.Frames
}

func watchBarOptions() rograph.BarOptions {
	opts := barOptionsFromSpec(mustIndicators().UsageBar)
	opts.BackgroundANSI = chartBackgroundANSI()
	return opts
}

// watchUsageBarOptions applies the optional quota-bar heat presentation.
// Its background and all glyph geometry still come from watchBarOptions.
func watchUsageBarOptions(value float64) rograph.BarOptions {
	return usageBarOptionsWithPresentation(mustIndicators().usageBarPresentation(), value)
}

func usageBarOptionsWithPresentation(presentation UsageBarPresentation, value float64) rograph.BarOptions {
	opts := watchBarOptions()
	if presentation == UsageBarHeat {
		opts.ForegroundANSI = heatForegroundANSI(value)
	}
	return opts
}

// watchLoadBarOptions adds the same heat foreground used by a colored history
// and its adjacent current-value label. Generic quota bars retain their own
// foreground policy while still inheriting the shared chart background.
func watchLoadBarOptions(value float64) rograph.BarOptions {
	opts := watchBarOptions()
	if mustIndicators().chartPresentation() == LoadChartHeat {
		opts.ForegroundANSI = heatForegroundANSI(value)
	}
	return opts
}

func chartBackgroundANSI() string {
	return colorSGR(mustIndicators().chartBackgroundName())
}

// heatForegroundANSI implements the btop-inspired, discrete four-band load
// ramp. Values at the endpoints stay in their lower band: 0–25 cool blue,
// >25–50 green, >50–75 yellow, and >75–100 warm red.
func heatForegroundANSI(value float64) string {
	switch {
	case math.IsNaN(value) || value <= 25:
		return colorSGR("chart-cool")
	case value <= 50:
		return colorSGR("chart-green")
	case value <= 75:
		return colorSGR("chart-yellow")
	default:
		return colorSGR("chart-warm")
	}
}

func watchLoadPercent(value float64) string {
	return watchLoadPercentWithPresentation(mustIndicators().chartPresentation(), value)
}

func watchLoadPercentWithPresentation(presentation LoadChartPresentation, value float64) string {
	label := fmt.Sprintf("%.0f%%", value)
	if presentation != LoadChartHeat {
		return label
	}
	return "\x1b[" + heatForegroundANSI(value) + "m" + label + "\x1b[0m"
}

func watchUsagePercent(value float64) string {
	return watchUsagePercentWithPresentation(mustIndicators().usageBarPresentation(), value)
}

func watchUsagePercentWithPresentation(presentation UsageBarPresentation, value float64) string {
	label := fmt.Sprintf("%.0f%%", value)
	if presentation != UsageBarHeat {
		return label
	}
	return "\x1b[" + heatForegroundANSI(value) + "m" + label + "\x1b[0m"
}

// padWatchUsagePercent adds leading display cells without including ANSI
// bytes in the width calculation, so compact quota columns stay aligned in
// both presentation modes.
func padWatchUsagePercent(value float64, width int) string {
	return padWatchUsagePercentWithPresentation(mustIndicators().usageBarPresentation(), value, width)
}

func padWatchUsagePercentWithPresentation(presentation UsageBarPresentation, value float64, width int) string {
	label := watchUsagePercentWithPresentation(presentation, value)
	return strings.Repeat(" ", max(0, width-visLen(label))) + label
}

func watchUsagePercentWithDuration(value float64, duration string) string {
	return watchUsagePercentWithDurationAndPresentation(mustIndicators().usageBarPresentation(), value, duration)
}

func watchUsagePercentWithDurationAndPresentation(presentation UsageBarPresentation, value float64, duration string) string {
	label := watchUsagePercentWithPresentation(presentation, value)
	if duration == "" {
		return label
	}
	return label + " " + duration
}

func padWatchUsagePercentWithDuration(value float64, duration string, width int) string {
	return padWatchUsagePercentWithDurationAndPresentation(mustIndicators().usageBarPresentation(), value, duration, width)
}

func padWatchUsagePercentWithDurationAndPresentation(presentation UsageBarPresentation, value float64, duration string, width int) string {
	label := watchUsagePercentWithPresentation(presentation, value)
	if duration != "" {
		label += " " + duration
	}
	return label + strings.Repeat(" ", max(0, width-visLen(label)))
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

func watchPercentSparkline(values []float64, width int, modes ...LoadChartMode) string {
	mode := LoadChartSparkline
	if len(modes) > 0 {
		mode = modes[0]
	}
	return rograph.RenderPercentSparkline(values, sparklineOptionsFromSpec(mustIndicators(), width, mode))
}

func sparklineOptionsFromSpec(spec indicatorsSpec, width int, mode LoadChartMode) rograph.SparklineOptions {
	frames := spec.LoadSparkline.Frames
	glyphs := make([]rune, len(frames))
	for i, glyph := range frames {
		glyphs[i] = []rune(glyph)[0]
	}
	opts := rograph.SparklineOptions{Width: width, Glyphs: glyphs, ANSI: true, BackgroundANSI: colorSGR(spec.chartBackgroundName())}
	if mode == LoadChartBraille {
		opts.Presentation = rograph.SparklineBraille
	}
	if spec.chartPresentation() == LoadChartHeat {
		opts.ForegroundANSI = heatForegroundANSI
	}
	return opts
}
