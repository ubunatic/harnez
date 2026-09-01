package usage

import (
	"strings"
	"testing"
)

func TestEmbeddedIndicatorsSpecIsValidAndExact(t *testing.T) {
	spec, err := loadIndicators()
	if err != nil {
		t.Fatalf("embedded %s failed to load: %v", indicatorsSpecPath, err)
	}
	want := []string{"⣿", "⡿", "⠿", "⠻", "⠹", "⠸", "⠰", "⠠", "⠀"}
	if got := spec.TimeoutSnake.Frames; strings.Join(got, "") != strings.Join(want, "") {
		t.Fatalf("timeout-snake frames = %q, want %q", got, want)
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
	if got, want := timeoutSnakeGlyph(-1), "⠀"; got != want {
		t.Errorf("timeoutSnakeGlyph(-1) = %q, want timeout frame %q", got, want)
	}
	if got, want := timeoutSnakeGlyph(2), "⣿"; got != want {
		t.Errorf("timeoutSnakeGlyph(2) = %q, want fresh frame %q", got, want)
	}
}

func TestParseIndicatorsYAMLRejectsInvalidFramesAndEndpoints(t *testing.T) {
	cases := []string{
		"timeout-snake: {title: snake, frames: [\"⣿\", \"x\", \"⠀\"]}",
		"timeout-snake: {title: snake, frames: [\"⠋⠙\", \"⠀\"]}",
		"timeout-snake: {title: snake, frames: [\"⠋\", \"⠀\"]}",
		"timeout-snake: {title: snake, frames: [\"⣿\", \"⠋\"]}",
	}
	for _, data := range cases {
		if _, err := parseIndicatorsYAML([]byte(data)); err == nil {
			t.Errorf("parseIndicatorsYAML(%q) succeeded, want validation error", data)
		}
	}
}
