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
	if got, want := spec.LoadCharts.CPU, "sparkline"; got != want {
		t.Errorf("load-charts.cpu = %q, want %q", got, want)
	}
	if got, want := spec.LoadCharts.GPU, "sparkline"; got != want {
		t.Errorf("load-charts.gpu = %q, want %q", got, want)
	}
	if got, want := spec.LoadCharts.RAM, "bar"; got != want {
		t.Errorf("load-charts.ram = %q, want %q", got, want)
	}
	if got, want := spec.LoadCharts.VRAM, "bar"; got != want {
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
	if got, want := bar.Fill, []rune(spec.UsageBar.Filled)[0]; got != want {
		t.Fatalf("bar fill = %q, want spec %q", got, want)
	}
	if got, want := bar.Empty, []rune(spec.UsageBar.Empty)[0]; got != want {
		t.Fatalf("bar empty = %q, want spec %q", got, want)
	}
	if got, want := len(bar.SubCharacterGlyphs), len(spec.UsageBar.SubCharacter); got != want {
		t.Fatalf("bar partial glyph count = %d, want %d", got, want)
	}
	got := stripANSI(watchPercentSparkline([]float64{0, 100}, 2))
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

func TestLoadChartModesAndAliases(t *testing.T) {
	spec := loadChartsSpec{
		CPU:  "timeseries",
		GPU:  "gauge",
		RAM:  "sparkline",
		VRAM: "bar",
	}
	if got := spec.CPUMode(); got != LoadChartSparkline {
		t.Errorf("CPUMode(timeseries) = %q, want %q", got, LoadChartSparkline)
	}
	if got := spec.GPUMode(); got != LoadChartBar {
		t.Errorf("GPUMode(gauge) = %q, want %q", got, LoadChartBar)
	}
	if got := spec.RAMMode(); got != LoadChartSparkline {
		t.Errorf("RAMMode(sparkline) = %q, want %q", got, LoadChartSparkline)
	}
	if got := spec.VRAMMode(); got != LoadChartBar {
		t.Errorf("VRAMMode(bar) = %q, want %q", got, LoadChartBar)
	}

	// Empty defaults:
	var emptySpec loadChartsSpec
	if got := emptySpec.CPUMode(); got != LoadChartSparkline {
		t.Errorf("empty CPUMode = %q, want default %q", got, LoadChartSparkline)
	}
	if got := emptySpec.GPUMode(); got != LoadChartSparkline {
		t.Errorf("empty GPUMode = %q, want default %q", got, LoadChartSparkline)
	}
	if got := emptySpec.RAMMode(); got != LoadChartBar {
		t.Errorf("empty RAMMode = %q, want default %q", got, LoadChartBar)
	}
	if got := emptySpec.VRAMMode(); got != LoadChartBar {
		t.Errorf("empty VRAMMode = %q, want default %q", got, LoadChartBar)
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
	spark := stripANSI(watchPercentSparkline([]float64{0, 25, 50, 100}, 4))
	if width := runewidth.StringWidth(spark); width != 4 {
		t.Errorf("selected ANSI sparkline %q has width %d, want 4", spark, width)
	}
}
