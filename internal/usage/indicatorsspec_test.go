package usage

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"ubunatic.com/harnez/internal/rograph"
)

func TestEmbeddedIndicatorsSpecIsValidAndExact(t *testing.T) {
	spec, err := loadIndicators()
	if err != nil {
		t.Fatalf("embedded %s failed to load: %v", indicatorsSpecPath, err)
	}
	want := map[string]namedIndicatorSequence{
		"block-deplete-8":          {Title: "Eighth-block countdown", Kind: "countdown", Frames: []string{"█", "▉", "▊", "▋", "▌", "▍", "▎", "▏", " "}},
		"block-deplete-vertical-8": {Title: "Vertical-block countdown", Kind: "countdown", Frames: []string{"█", "▇", "▆", "▅", "▄", "▃", "▂", "▁", " "}},
		"shade-deplete-5":          {Title: "Shade countdown", Kind: "countdown", Frames: []string{"█", "▓", "▒", "░", " "}},
		"quadrant-rotate-4":        {Title: "Quadrant spinner", Kind: "spinner", Frames: []string{"▘", "▝", "▗", "▖"}},
		"half-block-rotate-4":      {Title: "Half-block spinner", Kind: "spinner", Frames: []string{"▄", "▌", "▀", "▐"}},
		"box-line-rotate-4":        {Title: "Box-line spinner", Kind: "spinner", Frames: []string{"╷", "╴", "╵", "╶"}},
		"braille-orbit-8":          {Title: "Braille orbit", Kind: "spinner", Frames: []string{"⡀", "⠄", "⠂", "⠁", "⠈", "⠐", "⠠", "⢀"}},
		"braille-classic-10":       {Title: "Classic Braille spinner", Kind: "spinner", Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}},
		"braille-snake-2x3":        {Title: "2×3 Braille timeout snake", Kind: "countdown", Frames: []string{"⠿", "⠷", "⠧", "⠇", "⠃", "⠁", "⠀"}},
		"horizontal-eighths-7":     {Title: "Horizontal eighth-block partial fill", Kind: "bar-partial", Frames: []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉"}},
		"vertical-block-scale-8":   {Title: "Vertical block scale", Kind: "sparkline", Frames: []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}},
	}
	if !reflect.DeepEqual(spec.Sequences, want) {
		t.Fatalf("embedded sequence registry mismatch\n got: %#v\nwant: %#v", spec.Sequences, want)
	}
	if got, wantName := spec.TimeoutSnake.Sequence, "braille-snake-2x3"; got != wantName {
		t.Errorf("timeout-snake sequence = %q, want %q", got, wantName)
	}
	if !reflect.DeepEqual(spec.TimeoutSnake.Frames, want[spec.TimeoutSnake.Sequence].Frames) {
		t.Errorf("timeout-snake resolved frames = %#v, want named sequence %#v", spec.TimeoutSnake.Frames, want[spec.TimeoutSnake.Sequence].Frames)
	}
	if got, wantName := spec.UsageBar.SubCharacterSequence, "horizontal-eighths-7"; got != wantName {
		t.Errorf("usage-bar sub-character sequence = %q, want %q", got, wantName)
	}
	if got, wantName := spec.LoadSparkline.Sequence, "vertical-block-scale-8"; got != wantName {
		t.Errorf("load-sparkline sequence = %q, want %q", got, wantName)
	}
	if got, want := spec.LoadCharts.CPU, "btop"; got != want {
		t.Errorf("load-charts.cpu = %q, want %q", got, want)
	}
	if got, want := spec.LoadCharts.GPU, "btop"; got != want {
		t.Errorf("load-charts.gpu = %q, want %q", got, want)
	}
	if got, want := spec.LoadCharts.RAM, "btop"; got != want {
		t.Errorf("load-charts.ram = %q, want %q", got, want)
	}
	if got, want := spec.LoadCharts.VRAM, "btop"; got != want {
		t.Errorf("load-charts.vram = %q, want %q", got, want)
	}
	for name, sequence := range spec.Sequences {
		for i, frame := range sequence.Frames {
			if width := runewidth.StringWidth(frame); width != 1 {
				t.Errorf("sequence %q frame %d = %q has display width %d, want 1", name, i, frame, width)
			}
		}
	}
}

func TestTimeoutSnakeGlyphDrainsWithoutWrapping(t *testing.T) {
	frames := mustIndicators().TimeoutSnake.Frames
	for i, want := range frames {
		fraction := 1 - float64(i)/float64(len(frames)-1)
		if got := timeoutSnakeGlyph(fraction); got != want {
			t.Errorf("timeoutSnakeGlyph(%v) = %q, want frame %d %q", fraction, got, i, want)
		}
	}
	if got, want := timeoutSnakeGlyph(-1), frames[len(frames)-1]; got != want {
		t.Errorf("timeoutSnakeGlyph(-1) = %q, want timeout frame %q", got, want)
	}
	if got, want := timeoutSnakeGlyph(2), frames[0]; got != want {
		t.Errorf("timeoutSnakeGlyph(2) = %q, want fresh frame %q", got, want)
	}
}

func TestWatchChartRenderersUseDeclaredGlyphs(t *testing.T) {
	spec := mustIndicators()
	bar := watchBarOptions()
	wantFill, wantEmpty, wantPartialCount := []rune(spec.UsageBar.Filled)[0], []rune(spec.UsageBar.Empty)[0], len(spec.UsageBar.SubCharacter)
	if spec.UsageBar.resolvedStyle() == UsageBarStyleBraille {
		wantFill, wantEmpty, wantPartialCount = []rune(spec.UsageBar.Braille.Full)[0], []rune(spec.UsageBar.Braille.Empty)[0], 1
	}
	if got := bar.Fill; got != wantFill {
		t.Fatalf("bar fill = %q, want spec %q", got, wantFill)
	}
	if got := bar.Empty; got != wantEmpty {
		t.Fatalf("bar empty = %q, want spec %q", got, wantEmpty)
	}
	if got := len(bar.SubCharacterGlyphs); got != wantPartialCount {
		t.Fatalf("bar partial glyph count = %d, want %d", got, wantPartialCount)
	}
	got := stripANSI(watchPercentSparkline([]float64{0, 0, 0, 100}, 2))
	want := spec.LoadSparkline.Frames[0] + spec.LoadSparkline.Frames[len(spec.LoadSparkline.Frames)-1]
	if got != want {
		t.Fatalf("load sparkline = %q, want spec endpoints %q", got, want)
	}
}

// TestWatchBarWrapperDefaultsToBracketsOn covers issue 159's acceptance
// criterion that default behavior (brackets on, "["/"]") is unchanged unless
// spec/indicators.yaml is edited. It asserts the resolved BarOptions
// directly and the rendered bar text.
func TestWatchBarWrapperDefaultsToBracketsOn(t *testing.T) {
	spec := mustIndicators().UsageBar
	if !spec.Wrapper.Enabled {
		t.Fatalf("embedded spec/indicators.yaml usage-bar wrapper.enabled = false, want true (default must stay on)")
	}
	opts := watchBarOptions()
	if opts.NoWrapper {
		t.Fatalf("watchBarOptions().NoWrapper = true, want false for the default spec")
	}
	if opts.Left != "[" || opts.Right != "]" {
		t.Fatalf("watchBarOptions() Left/Right = %q/%q, want \"[\"/\"]\"", opts.Left, opts.Right)
	}
	rendered := stripANSI(rograph.RenderBar(50, opts))
	if !strings.HasPrefix(rendered, "[") || !strings.HasSuffix(rendered, "]") {
		t.Fatalf("RenderBar with default wrapper = %q, want it wrapped in [ ]", rendered)
	}
}

// TestWatchBarWrapperDisabledBySpec covers a brackets-disabled spec value:
// parsing a usage-bar spec with wrapper.enabled: false must resolve to
// rograph.BarOptions.NoWrapper and drop any leftover Left/Right glyphs, so a
// spec author cannot accidentally leak brackets back in by leaving
// left/right set alongside enabled: false.
func TestWatchBarWrapperDisabledBySpec(t *testing.T) {
	data := []byte(`
sequences:
  countdown: {title: countdown, kind: countdown, frames: ["⠿", "⠷", "⠀"]}
  partial: {title: partial, kind: bar-partial, frames: ["▏", "▌", "▉"]}
  spark: {title: spark, kind: sparkline, frames: ["▁", "▄", "█"]}
timeout-snake:
  title: "Time gauge braille snake"
  sequence: countdown
usage-bar:
  filled: "█"
  empty: "░"
  sub-character-sequence: partial
  wrapper:
    enabled: false
    left: "["
    right: "]"
load-sparkline:
  sequence: spark
`)
	spec, err := parseIndicatorsYAML(data)
	if err != nil {
		t.Fatalf("parseIndicatorsYAML: %v", err)
	}
	opts := barOptionsFromSpec(spec.UsageBar)
	if !opts.NoWrapper {
		t.Fatalf("barOptionsFromSpec().NoWrapper = false, want true for wrapper.enabled: false")
	}
	if opts.Left != "" || opts.Right != "" {
		t.Fatalf("barOptionsFromSpec() Left/Right = %q/%q, want empty when wrapper is disabled", opts.Left, opts.Right)
	}
	rendered := stripANSI(rograph.RenderBar(50, opts))
	if strings.Contains(rendered, "[") || strings.Contains(rendered, "]") {
		t.Fatalf("RenderBar with disabled wrapper = %q, want no brackets", rendered)
	}
}

const validIndicatorsFixture = `
sequences:
  countdown: {title: countdown, kind: countdown, frames: ["█", " "]}
  spinner: {title: spinner, kind: spinner, frames: ["▘", "▝"]}
  partial: {title: partial, kind: bar-partial, frames: ["▏", "▉"]}
  spark: {title: spark, kind: sparkline, frames: ["▁", "█"]}
timeout-snake: {title: timer, sequence: countdown}
usage-bar:
  filled: "█"
  empty: "░"
  sub-character-sequence: partial
  wrapper: {enabled: true, left: "[", right: "]"}
load-sparkline: {sequence: spark}
load-charts:
  cpu: sparkline
  gpu: sparkline
  ram: bar
  vram: bar
`

func TestParseIndicatorsYAMLRejectsInvalidRegistryAndReferences(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{"missing registry", strings.Replace(validIndicatorsFixture, "sequences:", "sequence-library:", 1), "field sequence-library not found"},
		{"empty name", strings.Replace(validIndicatorsFixture, "  countdown:", "  \"\":", 1), "sequences: empty name"},
		{"duplicate name", strings.Replace(validIndicatorsFixture, "  spinner:", "  countdown:", 1), "already defined"},
		{"missing title", strings.Replace(validIndicatorsFixture, "title: countdown", "title: \"\"", 1), "sequence \"countdown\": missing title"},
		{"unknown kind", strings.Replace(validIndicatorsFixture, "kind: countdown", "kind: pulse", 1), "unknown kind \"pulse\""},
		{"empty sequence", strings.Replace(validIndicatorsFixture, "frames: [\"█\", \" \"]", "frames: []", 1), "sequence \"countdown\": need at least one frame"},
		{"wide frame", strings.Replace(validIndicatorsFixture, "frames: [\"█\", \" \"]", "frames: [\"界\", \" \"]", 1), "display width 2, want one terminal cell"},
		{"zero-width frame", strings.Replace(validIndicatorsFixture, "frames: [\"█\", \" \"]", "frames: [\"\\u0301\", \" \"]", 1), "display width 0, want one terminal cell"},
		{"missing reference", strings.Replace(validIndicatorsFixture, "sequence: countdown", "sequence: \"\"", 1), "timeout-snake: missing sequence reference"},
		{"unknown reference", strings.Replace(validIndicatorsFixture, "sequence: countdown", "sequence: absent", 1), "timeout-snake: unknown sequence \"absent\""},
		{"semantic mismatch", strings.Replace(validIndicatorsFixture, "sequence: countdown", "sequence: spinner", 1), "has kind \"spinner\", want \"countdown\""},
		{"bar semantic mismatch", strings.Replace(validIndicatorsFixture, "sub-character-sequence: partial", "sub-character-sequence: spinner", 1), "has kind \"spinner\", want \"bar-partial\""},
		{"sparkline semantic mismatch", strings.Replace(validIndicatorsFixture, "load-sparkline: {sequence: spark}", "load-sparkline: {sequence: spinner}", 1), "has kind \"spinner\", want \"sparkline\""},
		{"inline frames rejected", strings.Replace(validIndicatorsFixture, "timeout-snake: {title: timer, sequence: countdown}", "timeout-snake: {title: timer, sequence: countdown, frames: [\"█\", \" \"]}", 1), "field frames not found"},
		{"multi-rune cell", strings.Replace(validIndicatorsFixture, "frames: [\"▏\", \"▉\"]", "frames: [\"e\\u0301\", \"▉\"]", 1), "sequence \"partial\" frame 0: want one rune"},
		{"invalid load-chart cpu mode", strings.Replace(validIndicatorsFixture, "cpu: sparkline", "cpu: circular", 1), "indicators spec: load-charts: cpu: unknown mode \"circular\""},
		{"invalid chart background", validIndicatorsFixture + "chart-background: other-bg\n", "indicators spec: chart-background: unknown color \"other-bg\""},
		{"invalid chart presentation", validIndicatorsFixture + "load-chart-presentation: rainbow\n", "indicators spec: load-chart-presentation: unknown mode \"rainbow\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseIndicatorsYAML([]byte(tt.data))
			if err == nil {
				t.Fatal("parseIndicatorsYAML succeeded, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseIndicatorsYAML error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestLoadChartPresentationHeatCouplesChartAndPercentage(t *testing.T) {
	spec, err := parseIndicatorsYAML([]byte(validIndicatorsFixture + "chart-background: panel-bg\nload-chart-presentation: heat\n"))
	if err != nil {
		t.Fatalf("parse heat indicators spec: %v", err)
	}
	if got, want := spec.chartPresentation(), LoadChartHeat; got != want {
		t.Fatalf("chart presentation = %q, want %q", got, want)
	}
	if got, want := sparklineOptionsFromSpec(spec, 1, LoadChartBraille).BackgroundANSI, colorSGR("panel-bg"); got != want {
		t.Fatalf("sparkline background = %q, want shared %q", got, want)
	}
	if got, want := watchLoadPercentWithPresentation(spec.chartPresentation(), 80), "\x1b[31m80%\x1b[0m"; got != want {
		t.Fatalf("heat percentage = %q, want %q", got, want)
	}
	chart := rograph.RenderPercentSparkline([]float64{5, 80}, sparklineOptionsFromSpec(spec, 1, LoadChartBraille))
	if want := "\x1b[40;31m⣸\x1b[0m"; chart != want {
		t.Fatalf("heat Braille = %q, want %q", chart, want)
	}
	if got := stripANSI(chart); runewidth.StringWidth(got) != 1 {
		t.Fatalf("heat chart visible width = %d, want 1", runewidth.StringWidth(got))
	}
}

func TestUsageBarPresentationHeatCouplesBarAndPercentage(t *testing.T) {
	spec, err := parseIndicatorsYAML([]byte(validIndicatorsFixture + "chart-background: panel-bg\nload-chart-presentation: monochrome\nusage-bar-presentation: heat\n"))
	if err != nil {
		t.Fatalf("parse heat usage-bar spec: %v", err)
	}
	if got, want := spec.usageBarPresentation(), UsageBarHeat; got != want {
		t.Fatalf("usage bar presentation = %q, want %q", got, want)
	}
	if got, want := watchUsagePercentWithPresentation(spec.usageBarPresentation(), 62.5), "\x1b[33m62%\x1b[0m"; got != want {
		t.Fatalf("heat usage percentage = %q, want %q", got, want)
	}

	opts := usageBarOptionsWithPresentation(spec.usageBarPresentation(), 62.5)
	opts.Width = 4
	bar := rograph.RenderBar(62.5, opts)
	if want := "[\x1b[40;33m⣿⣿⡇ \x1b[0m]"; bar != want {
		t.Fatalf("heat usage bar = %q, want %q", bar, want)
	}
	if strings.Contains(bar, "░") {
		t.Fatalf("heat usage bar = %q, want a flat background without empty stipple", bar)
	}
	if got := runewidth.StringWidth(stripANSI(bar)); got != 6 { // [ + 4 cells + ]
		t.Fatalf("heat usage bar visible width = %d, want 6", got)
	}

	monoOpts := usageBarOptionsWithPresentation(UsageBarMonochrome, 62.5)
	monoOpts.Width = 4
	monochrome := rograph.RenderBar(62.5, monoOpts)
	if want := "[\x1b[40m⣿⣿⡇ \x1b[0m]"; monochrome != want {
		t.Fatalf("monochrome usage bar = %q, want %q", monochrome, want)
	}
	if got, want := watchUsagePercentWithPresentation(UsageBarMonochrome, 62.5), "62%"; got != want {
		t.Fatalf("monochrome usage percentage = %q, want %q", got, want)
	}
}

func TestUsageBarPresentationDefaultsAndRejectsUnknownValues(t *testing.T) {
	spec, err := parseIndicatorsYAML([]byte(validIndicatorsFixture + "chart-background: panel-bg\nload-chart-presentation: monochrome\n"))
	if err != nil {
		t.Fatalf("parse default usage-bar presentation: %v", err)
	}
	if got, want := spec.usageBarPresentation(), UsageBarMonochrome; got != want {
		t.Fatalf("default usage bar presentation = %q, want %q", got, want)
	}

	_, err = parseIndicatorsYAML([]byte(validIndicatorsFixture + "chart-background: panel-bg\nload-chart-presentation: monochrome\nusage-bar-presentation: heat-256\n"))
	if err == nil || !strings.Contains(err.Error(), `usage-bar-presentation: unknown mode "heat-256"`) {
		t.Fatalf("invalid usage-bar presentation error = %v, want unknown-mode error", err)
	}
}

// validBrailleIndicatorsFixture opts a usage-bar into the Braille style
// (issue 220): the same registry as validIndicatorsFixture, with style and
// its three required glyphs declared. Width-four percentage examples below
// mirror the ticket's exact acceptance criterion: three full cells (25pp
// each) plus one half cell (12.5pp) is 87.5%, rendering "⣿⣿⣿⡇".
const validBrailleIndicatorsFixture = `
sequences:
  countdown: {title: countdown, kind: countdown, frames: ["█", " "]}
  spinner: {title: spinner, kind: spinner, frames: ["▘", "▝"]}
  partial: {title: partial, kind: bar-partial, frames: ["▏", "▉"]}
  spark: {title: spark, kind: sparkline, frames: ["▁", "█"]}
timeout-snake: {title: timer, sequence: countdown}
usage-bar:
  filled: "█"
  empty: "░"
  sub-character-sequence: partial
  style: braille
  braille: {full: "⣿", half: "⡇", empty: "⠀"}
  wrapper: {enabled: true, left: "[", right: "]"}
load-sparkline: {sequence: spark}
load-charts:
  cpu: sparkline
  gpu: sparkline
  ram: bar
  vram: bar
`

func TestUsageBarStyleBrailleParsesAndRendersQuantizedCells(t *testing.T) {
	spec, err := parseIndicatorsYAML([]byte(validBrailleIndicatorsFixture))
	if err != nil {
		t.Fatalf("parseIndicatorsYAML: %v", err)
	}
	if got, want := spec.UsageBar.resolvedStyle(), UsageBarStyleBraille; got != want {
		t.Fatalf("resolvedStyle() = %q, want %q", got, want)
	}

	opts := barOptionsFromSpec(spec.UsageBar)
	if got, want := opts.Fill, '⣿'; got != want {
		t.Fatalf("Braille bar Fill = %q, want %q", got, want)
	}
	if got, want := opts.Empty, '⠀'; got != want {
		t.Fatalf("Braille bar Empty = %q, want %q", got, want)
	}
	if got, want := opts.SubCharacterGlyphs, []rune{'⡇'}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Braille bar SubCharacterGlyphs = %q, want %q", got, want)
	}

	// ANSI is off here so the rendered empty-cell glyph is the spec's own
	// Braille blank rather than the ANSI-wrap's flat-space substitution
	// (see internal/rograph/options.go's RenderBar ANSI handling).
	opts.ANSI = false
	opts.Width = 4
	tests := []struct {
		name string
		pct  float64
		want string
	}{
		{"0% is fully empty", 0, "[⠀⠀⠀⠀]"},
		{"half-boundary at 12.5%", 12.5, "[⡇⠀⠀⠀]"},
		{"three full cells plus a half cell at 87.5%", 87.5, "[⣿⣿⣿⡇]"},
		{"100% is fully filled", 100, "[⣿⣿⣿⣿]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rograph.RenderBar(tt.pct, opts)
			if got != tt.want {
				t.Errorf("RenderBar(%v, Braille width 4) = %q, want %q", tt.pct, got, tt.want)
			}
		})
	}
}

func TestUsageBarStyleDefaultsToBlockWhenUnset(t *testing.T) {
	spec, err := parseIndicatorsYAML([]byte(validIndicatorsFixture))
	if err != nil {
		t.Fatalf("parseIndicatorsYAML: %v", err)
	}
	if got, want := spec.UsageBar.resolvedStyle(), UsageBarStyleBlock; got != want {
		t.Fatalf("resolvedStyle() with no style field = %q, want %q", got, want)
	}
}

func TestUsageBarStyleBrailleRejectsInvalidSpecs(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{"unknown style", strings.Replace(validBrailleIndicatorsFixture, "style: braille", "style: dotted", 1), `usage-bar: style: unknown style "dotted"`},
		{"missing full glyph", strings.Replace(validBrailleIndicatorsFixture, `braille: {full: "⣿", half: "⡇", empty: "⠀"}`, `braille: {full: "", half: "⡇", empty: "⠀"}`, 1), "usage-bar braille full: want one rune"},
		{"missing half glyph", strings.Replace(validBrailleIndicatorsFixture, `braille: {full: "⣿", half: "⡇", empty: "⠀"}`, `braille: {full: "⣿", half: "", empty: "⠀"}`, 1), "usage-bar braille half: want one rune"},
		{"missing empty glyph", strings.Replace(validBrailleIndicatorsFixture, `braille: {full: "⣿", half: "⡇", empty: "⠀"}`, `braille: {full: "⣿", half: "⡇", empty: ""}`, 1), "usage-bar braille empty: want one rune"},
		{"multi-rune full glyph", strings.Replace(validBrailleIndicatorsFixture, `full: "⣿"`, `full: "⣿⣿"`, 1), "usage-bar braille full: want one rune"},
		{"duplicate full and half glyphs", strings.Replace(validBrailleIndicatorsFixture, `half: "⡇"`, `half: "⣿"`, 1), "usage-bar braille: full, half, and empty must all differ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseIndicatorsYAML([]byte(tt.data))
			if err == nil {
				t.Fatal("parseIndicatorsYAML succeeded, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseIndicatorsYAML error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestWatchBarsUseTheSharedChartBackground(t *testing.T) {
	if got, want := watchBarOptions().BackgroundANSI, chartBackgroundANSI(); got != want {
		t.Fatalf("bar background = %q, want shared chart background %q", got, want)
	}
}

func TestHeatForegroundBands(t *testing.T) {
	for _, tt := range []struct {
		value float64
		want  string
	}{
		{0, "34"}, {25, "34"}, {25.01, "32"}, {50, "32"},
		{50.01, "33"}, {75, "33"}, {75.01, "31"}, {100, "31"},
	} {
		if got := heatForegroundANSI(tt.value); got != tt.want {
			t.Errorf("heatForegroundANSI(%v) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestLoadChartModesAndAliases(t *testing.T) {
	spec := loadChartsSpec{
		CPU:  "timeseries",
		GPU:  "btop",
		RAM:  "sparkline",
		VRAM: "gauge",
	}
	if got := spec.CPUMode(); got != LoadChartSparkline {
		t.Errorf("CPUMode(timeseries) = %q, want %q", got, LoadChartSparkline)
	}
	if got := spec.GPUMode(); got != LoadChartBraille {
		t.Errorf("GPUMode(btop) = %q, want %q", got, LoadChartBraille)
	}
	if got := spec.RAMMode(); got != LoadChartSparkline {
		t.Errorf("RAMMode(sparkline) = %q, want %q", got, LoadChartSparkline)
	}
	if got := spec.VRAMMode(); got != LoadChartBar {
		t.Errorf("VRAMMode(gauge) = %q, want %q", got, LoadChartBar)
	}

	// Empty defaults:
	var emptySpec loadChartsSpec
	if got := emptySpec.CPUMode(); got != LoadChartSparkline {
		t.Errorf("empty CPUMode = %q, want default %q", got, LoadChartSparkline)
	}
	if got := emptySpec.GPUMode(); got != LoadChartSparkline {
		t.Errorf("empty GPUMode = %q, want default %q", got, LoadChartSparkline)
	}
	if got := emptySpec.RAMMode(); got != LoadChartSparkline {
		t.Errorf("empty RAMMode = %q, want default %q", got, LoadChartSparkline)
	}
	if got := emptySpec.VRAMMode(); got != LoadChartSparkline {
		t.Errorf("empty VRAMMode = %q, want default %q", got, LoadChartSparkline)
	}
}

func TestResolvedCountdownSupportsVariableLengthsAndEndpoints(t *testing.T) {
	tests := []struct {
		name   string
		frames string
		want   []string
	}{
		{"two frames ending literal space", `["x", " "]`, []string{"x", " "}},
		{"four frames with accented rune and braille blank", `["A", "é", "⠀", "·"]`, []string{"A", "é", "⠀", "·"}},
		{"nine frames with nonstandard endpoints", `["9", "8", "7", "6", "5", "4", "3", "2", "1"]`, []string{"9", "8", "7", "6", "5", "4", "3", "2", "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := strings.Replace(validIndicatorsFixture, `["█", " "]`, tt.frames, 1)
			spec, err := parseIndicatorsYAML([]byte(data))
			if err != nil {
				t.Fatalf("parseIndicatorsYAML: %v", err)
			}
			if !reflect.DeepEqual(spec.TimeoutSnake.Frames, tt.want) {
				t.Fatalf("resolved frames = %#v, want %#v", spec.TimeoutSnake.Frames, tt.want)
			}
			for i, want := range tt.want {
				fraction := 1 - float64(i)/float64(len(tt.want)-1)
				if got := finiteSequenceGlyph(spec.TimeoutSnake.Frames, fraction); got != want {
					t.Errorf("finiteSequenceGlyph(%v) = %q, want frame %d %q", fraction, got, i, want)
				}
			}
			if got := finiteSequenceGlyph(spec.TimeoutSnake.Frames, -100); got != tt.want[len(tt.want)-1] {
				t.Errorf("overdue glyph = %q, want terminal endpoint %q", got, tt.want[len(tt.want)-1])
			}
			if got := finiteSequenceGlyph(spec.TimeoutSnake.Frames, 100); got != tt.want[0] {
				t.Errorf("fresh glyph = %q, want initial endpoint %q", got, tt.want[0])
			}
		})
	}
}

func TestResolvedFramesAreCopiedFromRegistry(t *testing.T) {
	spec, err := parseIndicatorsYAML([]byte(validIndicatorsFixture))
	if err != nil {
		t.Fatalf("parseIndicatorsYAML: %v", err)
	}
	spec.TimeoutSnake.Frames[0] = "x"
	if got := spec.Sequences[spec.TimeoutSnake.Sequence].Frames[0]; got != "█" {
		t.Fatalf("mutating resolved frames changed registry frame to %q", got)
	}
}

func TestSelectedIndicatorOutputsHaveStableANSIVisibleGeometry(t *testing.T) {
	spec := mustIndicators()
	for _, frame := range []string{
		spec.Sequences["block-deplete-8"].Frames[8],
		spec.Sequences["braille-snake-2x3"].Frames[6],
	} {
		for _, background := range []string{"", "100"} {
			styled := styleTimeGaugeGlyphWithSGR(frame, "38;5;229", background)
			if width := runewidth.StringWidth(stripANSI(styled)); width != 1 {
				t.Errorf("styled gauge endpoint %q with background %q has width %d, want 1", frame, background, width)
			}
		}
	}

	bar := stripANSI(rograph.RenderBar(50, watchBarOptions()))
	if width := runewidth.StringWidth(bar); width != rograph.MaxWidth+2 {
		t.Errorf("selected ANSI bar %q has width %d, want %d", bar, width, rograph.MaxWidth+2)
	}
	spark := stripANSI(watchPercentSparkline([]float64{0, 25, 50, 75, 0, 25, 50, 100}, 4))
	if width := runewidth.StringWidth(spark); width != 4 {
		t.Errorf("selected ANSI sparkline %q has width %d, want 4", spark, width)
	}
	braille := []rune(stripANSI(watchPercentSparkline([]float64{0, 100}, 1, LoadChartBraille)))
	if len(braille) != 1 || braille[0] < 0x2800 || braille[0] > 0x28ff {
		t.Errorf("selected Braille sparkline = %q, want one Braille cell", string(braille))
	}
}
