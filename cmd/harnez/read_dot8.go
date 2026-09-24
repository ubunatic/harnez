//go:build dot8

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/readcard"
)

type dot8State struct {
	dot8       string
	dot8Colors string
	dot8Pitch  int
}

func registerDot8Flags(cmd *cobra.Command, state *dot8State) {
	cmd.Flags().StringVar(&state.dot8, "dot8", "", "encode as 8-dot Braille for dense cards: bare flag or --dot8=native (already encoded)")
	cmd.Flags().StringVar(&state.dot8Colors, "dot8-colors", "", "Dot8 Braille colors: default or red-white (odd/even dots)")
	cmd.Flags().IntVar(&state.dot8Pitch, "dot8-pitch", 3, "Dot8 cell pitch in pixels: 3 or 4")
	if dot8Lookup := cmd.Flags().Lookup("dot8"); dot8Lookup != nil {
		dot8Lookup.NoOptDefVal = "encode"
	}
	cmd.Flags().MarkHidden("dot8")
	cmd.Flags().MarkHidden("dot8-colors")
	cmd.Flags().MarkHidden("dot8-pitch")
}

func checkDot8Hold(cmd *cobra.Command) error {
	if (cmd.Flags().Changed("dot8") || cmd.Flags().Changed("dot8-colors") || cmd.Flags().Changed("dot8-pitch")) && os.Getenv("HARNEZ_DOT8") != "1" {
		return fmt.Errorf("--dot8 is on hold: the Dot8 card format is not usable (see issue 444); set HARNEZ_DOT8=1 to override")
	}
	return nil
}

func prepareDot8Render(state *dot8State, imageMode bool, lines []string) ([]string, string, error) {
	dot8Mode := ""
	if state.dot8 != "" {
		if state.dot8 == "encode" {
			dot8Mode = "encode"
		} else if state.dot8 == "native" {
			dot8Mode = "native"
		} else {
			return nil, "", fmt.Errorf("invalid --dot8 value %q (use --dot8 or --dot8=native)", state.dot8)
		}
	}

	renderLines := lines
	if imageMode && dot8Mode == "encode" {
		renderLines = make([]string, len(lines))
		for j := range lines {
			renderLines[j] = readcard.Dot8Encode(lines[j])
		}
	}
	return renderLines, dot8Mode, nil
}

func handleDot8TextMode(state *dot8State, dot8Mode string, res *readcard.ReadResult) error {
	if dot8Mode == "native" {
		for j := range res.Lines {
			decoded, err := readcard.Dot8Decode(res.Lines[j])
			if err != nil {
				return fmt.Errorf("decode line %d: %v", res.StartLine+j, err)
			}
			res.Lines[j] = decoded
		}
	}
	return nil
}

func applyDot8RenderOptions(opts *readcard.RenderOptions, state *dot8State, dot8Mode string) {
	opts.Dot8 = dot8Mode
	opts.Dot8Colors = state.dot8Colors
	opts.Dot8Pitch = state.dot8Pitch
}
