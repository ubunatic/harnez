package usage

import (
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

const indicatorsSpecPath = "spec/indicators.yaml"

// indicatorsSpec mirrors spec/schemas/indicators.schema.json. The parser
// additionally fixes the semantic endpoints of the finite time gauge.
type indicatorsSpec struct {
	TimeoutSnake indicatorSequence `yaml:"timeout-snake"`
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
	if sequence.Frames[0] != "⣿" {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: timeout-snake: first frame must be full braille ⣿")
	}
	if sequence.Frames[len(sequence.Frames)-1] != "⠀" {
		return indicatorsSpec{}, fmt.Errorf("indicators spec: timeout-snake: last frame must be empty braille ⠀")
	}
	return spec, nil
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
	idx := int((1 - fraction) * float64(len(frames)-1))
	return frames[idx]
}
