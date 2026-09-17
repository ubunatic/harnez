package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/readcard"
)

func newReadCmd() *cobra.Command {
	var (
		imageMode       bool
		outputPath      string
		columns         int
		fontSize        int
		theme           string
		maxDim          int
		showLineNumbers bool
		lineRange       string
		head            int
		tail            int
		jsonOutput      bool
		showTokens      bool
	)

	cmd := &cobra.Command{
		Use:   "read [flags] [files...]",
		Short: "Read files as token-bounded text or styled visual PNG context cards (-I/--image)",
		Long: `read inspects files with line-range bounding, or renders dense, syntax-highlighted
visual PNG cards when -I/--image is passed for multimodal agent context injection.

Visual image mode (-I/--image) renders monospace cards bounded within 1568px to
prevent ViT downscaling cliffs, packing tall files into multiple columns and
reporting token compression across Claude, OpenAI, and Gemini.

Examples:
  # Read a file with line numbers:
  harnez read -n internal/lint/lint.go

  # Read a specific line range:
  harnez read --lines 10:50 internal/lint/lint.go

  # Render a source file into a visual PNG card:
  harnez read -I internal/lint/lint.go

  # Render with 2 columns, custom font size and output path:
  harnez read -I --columns=2 --font-size=11 -o /tmp/lint.png internal/lint/lint.go

  # Render multiple files with JSON metadata:
  harnez read -I --json cmd/harnez/main.go cmd/harnez/apply.go`,
		RunE: func(cmd *cobra.Command, args []string) error {
			textOpts := readcard.TextOptions{
				ShowLineNumbers: showLineNumbers,
				LineRange:       lineRange,
				Head:            head,
				Tail:            tail,
				ShowStats:       showTokens,
			}

			// If no args provided, read from Stdin
			if len(args) == 0 {
				res, err := readcard.ReadSource(os.Stdin, "stdin", textOpts)
				if err != nil {
					return err
				}
				if imageMode {
					renderOpts := readcard.RenderOptions{
						Columns:         columns,
						FontSize:        fontSize,
						Theme:           theme,
						MaxDimension:    maxDim,
						ShowLineNumbers: true,
						OutputPath:      outputPath,
						Title:           "stdin",
						StartLine:       res.StartLine,
					}
					renderRes, err := readcard.RenderFileToCards(res.Lines, "stdin.txt", renderOpts)
					if err != nil {
						return err
					}
					return outputRenderResult(cmd, renderRes, jsonOutput)
				}

				if jsonOutput {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(res)
				}
				fmt.Fprintln(cmd.OutOrStdout(), readcard.FormatText(res, showLineNumbers))
				if showTokens {
					printTextTokens(cmd, res.TokenStats)
				}
				return nil
			}

			// Process file arguments
			var allRenderResults []*readcard.RenderResult
			var allTextResults []*readcard.ReadResult

			for _, file := range args {
				res, err := readcard.ReadFile(file, textOpts)
				if err != nil {
					return fmt.Errorf("read file %s: %w", file, err)
				}

				if imageMode {
					renderOpts := readcard.RenderOptions{
						Columns:         columns,
						FontSize:        fontSize,
						Theme:           theme,
						MaxDimension:    maxDim,
						ShowLineNumbers: true,
						OutputPath:      outputPath,
						Title:           file,
						StartLine:       res.StartLine,
					}
					renderRes, err := readcard.RenderFileToCards(res.Lines, file, renderOpts)
					if err != nil {
						return fmt.Errorf("render image for %s: %w", file, err)
					}
					allRenderResults = append(allRenderResults, renderRes)
				} else {
					allTextResults = append(allTextResults, res)
				}
			}

			if imageMode {
				if jsonOutput {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					if len(allRenderResults) == 1 {
						return enc.Encode(allRenderResults[0])
					}
					return enc.Encode(allRenderResults)
				}

				for _, r := range allRenderResults {
					if err := outputRenderResult(cmd, r, false); err != nil {
						return err
					}
				}
				return nil
			}

			// Text mode output
			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if len(allTextResults) == 1 {
					return enc.Encode(allTextResults[0])
				}
				return enc.Encode(allTextResults)
			}

			for i, res := range allTextResults {
				if len(allTextResults) > 1 {
					fmt.Fprintf(cmd.OutOrStdout(), "=== %s (%d lines) ===\n", res.SourceFile, len(res.Lines))
				}
				fmt.Fprintln(cmd.OutOrStdout(), readcard.FormatText(res, showLineNumbers))
				if showTokens {
					printTextTokens(cmd, res.TokenStats)
				}
				if i < len(allTextResults)-1 {
					fmt.Fprintln(cmd.OutOrStdout())
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&imageMode, "image", "I", false, "render file(s) as styled visual PNG cards")
	cmd.Flags().StringVarP(&outputPath, "out", "o", "", "custom output PNG file or directory")
	cmd.Flags().IntVarP(&columns, "columns", "c", 0, "number of columns (1-4, default: auto)")
	cmd.Flags().IntVar(&fontSize, "font-size", 11, "font size in pixels (default: 11)")
	cmd.Flags().StringVar(&theme, "theme", "dark", "color theme: dark, light")
	cmd.Flags().IntVar(&maxDim, "max-dim", 1568, "maximum image dimension in pixels (default: 1568)")
	cmd.Flags().BoolVarP(&showLineNumbers, "number", "n", false, "display line numbers in text output")
	cmd.Flags().StringVarP(&lineRange, "lines", "L", "", "line range to read/render (e.g. 10:50, 100:, :30)")
	cmd.Flags().IntVar(&head, "head", 0, "read only the first N lines")
	cmd.Flags().IntVar(&tail, "tail", 0, "read only the last N lines")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON metadata")
	cmd.Flags().BoolVar(&showTokens, "tokens", false, "print token cost estimate and metrics")
	cmd.Flags().BoolVar(&showTokens, "stats", false, "alias for --tokens")

	return cmd
}

func outputRenderResult(cmd *cobra.Command, res *readcard.RenderResult, jsonFmt bool) error {
	if jsonFmt {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	for _, f := range res.Files {
		fmt.Fprintf(cmd.OutOrStdout(), "🖼️ Rendered: %s (%dx%d px, %d col, %d lines)\n", f, res.Width, res.Height, res.Columns, res.TotalLines)
	}

	stats := res.TokenStats
	fmt.Fprintf(cmd.OutOrStdout(), "Token Breakdown:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  • Raw Text Tokens : ~%d tokens (%d bytes)\n", stats.TextTokens, stats.TextBytes)
	fmt.Fprintf(cmd.OutOrStdout(), "  • Claude ViT      : ~%d tokens", stats.ClaudeTokens)
	if stats.ClaudeRatio > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), " (Compression: %.2fx)", stats.ClaudeRatio)
	}
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(), "  • OpenAI ViT      : ~%d tokens", stats.OpenAITokens)
	if stats.OpenAIRatio > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), " (Compression: %.2fx)", stats.OpenAIRatio)
	}
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(), "  • Gemini ViT      : ~%d tokens", stats.GeminiTokens)
	if stats.GeminiRatio > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), " (Compression: %.2fx)", stats.GeminiRatio)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	return nil
}

func printTextTokens(cmd *cobra.Command, stats readcard.TokenStats) {
	fmt.Fprintf(cmd.OutOrStdout(), "\n[Tokens: ~%d tokens | %d bytes | %d words]\n", stats.TextTokens, stats.TextBytes, stats.TextWords)
}
