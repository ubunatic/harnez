//go:build !dot8

package main

import (
	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/readcard"
)

type dot8State struct{}

func registerDot8Flags(cmd *cobra.Command, state *dot8State) {}

func checkDot8Hold(cmd *cobra.Command) error { return nil }

func prepareDot8Render(state *dot8State, imageMode bool, lines []string) ([]string, string, error) {
	return lines, "", nil
}

func handleDot8TextMode(state *dot8State, dot8Mode string, res *readcard.ReadResult) error {
	return nil
}

func applyDot8RenderOptions(opts *readcard.RenderOptions, state *dot8State, dot8Mode string) {}
