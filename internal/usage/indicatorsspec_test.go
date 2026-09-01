package usage

import (
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/rograph"
)

func TestEmbeddedIndicatorsSpecIsValidAndExact(t *testing.T) {
	spec, err := loadIndicators()
	if err != nil {
		t.Fatalf("embedded %s failed to load: %v", indicatorsSpecPath, err)
	}
	for i, frame := range spec.TimeoutSnake.Frames {
		if n := len([]rune(frame)); n != 1 {
			t.Errorf("frame %d = %q has %d runes, want 1", i, frame, n)
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
timeout-snake:
  title: "Time gauge braille snake"
  frames: ["⠿", "⠷", "⠧", "⠇", "⠃", "⠁", "⠀"]
usage-bar:
  filled: "█"
  empty: "░"
  sub-character: ["▏", "▎", "▍", "▌", "▋", "▊", "▉"]
  wrapper:
    enabled: false
    left: "["
    right: "]"
load-sparkline:
  frames: ["▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"]
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

func TestParseIndicatorsYAMLRejectsInvalidFrames(t *testing.T) {
	cases := []string{
		"timeout-snake: {title: snake, frames: [\"⣿\", \"x\", \"⠀\"]}",
		"timeout-snake: {title: snake, frames: [\"⠋⠙\", \"⠀\"]}",
	}
	for _, data := range cases {
		if _, err := parseIndicatorsYAML([]byte(data)); err == nil {
			t.Errorf("parseIndicatorsYAML(%q) succeeded, want validation error", data)
		}
	}
}
