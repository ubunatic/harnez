package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/readcard"
)

func newReadCmd() *cobra.Command {
	var imageMode, autoMode, textMode, rawMode, number, jsonOutput, showTokens bool
	var outputPath, fontName, theme, wrapMode, lineRange, lineNumbers, compression string
	var columns, fontSize, maxDim, head, tail int
	cmd := &cobra.Command{
		Use:   "read [flags] [files...]",
		Short: "Read bounded text or dense visual PNG cards with provider-adaptive routing",
		Long: `Read files or stdin as text, or PNG context cards with -I.
--auto compares actual page geometry against the active provider's estimated vision
cost. Unknown/local providers and micro-snippets (<=5 lines, <100 tokens) use text.
Set HARNEZ_AGENT_HARNESS to claude, codex, or gemini to select a profile.
Explicit -I forces images; --text, --raw, and -n force text even with --auto.
Images pack up to three columns, pruning unused columns and cropping to content.

--line-numbers=all|off|N controls the gutter cadence and preserves source anchors.
--compress=ws|ast safely compacts Go and JSON. Shell uses conservative lexical
compaction in both modes; complex expansions and heredocs remain verbatim.
Compression requires complete valid Go/JSON input; source files are never changed.

Examples:
  harnez read -n -L 10:50 internal/lint/lint.go
  harnez read -I --line-numbers=10 --compress=ws internal/lint/lint.go
  harnez read --auto --head=100 internal/lint/lint.go
  harnez read -I --columns=2 --json source.go`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := readcard.ParseLineNumbers(lineNumbers); err != nil {
				return err
			}
			if compression != "off" && compression != "ws" && compression != "ast" {
				return fmt.Errorf("invalid compression %q", compression)
			}
			if imageMode && (textMode || rawMode) {
				return fmt.Errorf("--image cannot be combined with --text or --raw")
			}
			explicitText := textMode || rawMode || cmd.Flags().Changed("number")
			adaptive := autoMode && !imageMode && !explicitText && !cmd.Flags().Changed("image")
			textOpts := readcard.TextOptions{ShowLineNumbers: number, LineRange: lineRange, Head: head, Tail: tail}
			var results []any
			paths := args
			if len(paths) == 0 {
				paths = []string{"-"}
			}
			if len(paths) > 1 && outputPath != "" {
				info, err := os.Stat(outputPath)
				if err != nil || !info.IsDir() {
					return fmt.Errorf("--out must be an existing directory for multiple inputs")
				}
			}
			for i, file := range paths {
				var res *readcard.ReadResult
				var err error
				if file == "-" {
					res, err = readcard.ReadSource(cmd.InOrStdin(), "stdin", textOpts)
				} else {
					res, err = readcard.ReadFile(file, textOpts)
				}
				if err != nil {
					return err
				}
				originalStats := res.TokenStats
				if compression != "off" {
					compact, err := readcard.Compress(res.Lines, file, compression, res.StartLine)
					if err != nil {
						return err
					}
					res.Lines, res.SourceLines = compact.Lines, compact.SourceLines
					res.TokenStats = readcard.ComputeTextTokens(strings.Join(res.Lines, "\n"))
				}
				renderOpts := readcard.RenderOptions{Columns: columns, FontName: fontName, FontSize: fontSize, Theme: theme, Wrap: wrapMode, MaxDimension: maxDim, ShowLineNumbers: true, LineNumbers: lineNumbers, SourceLines: res.SourceLines, OutputPath: outputPath, Title: res.SourceFile, StartLine: res.StartLine, SourceTokens: res.TokenStats.TextTokens}
				render := imageMode
				var measured *readcard.RenderResult
				if adaptive && !(len(res.Lines) <= readcard.MicroSnippetLineThreshold && res.TokenStats.TextTokens < readcard.MicroSnippetTokenThreshold) {
					renderOpts.MeasureOnly = true
					measured, err = readcard.RenderFileToCards(res.Lines, file, renderOpts)
					if err != nil {
						return err
					}
					render = readcard.PreferImage(readcard.DetectProvider(os.Getenv), len(res.Lines), measured.TokenStats)
					renderOpts.MeasureOnly = false
				}
				if render {
					rendered, err := readcard.RenderFileToCards(res.Lines, file, renderOpts)
					if err != nil {
						return err
					}
					if compression != "off" {
						rendered.OriginalTokenStats = &originalStats
					}
					results = append(results, rendered)
					if !jsonOutput {
						if err := outputRenderResult(cmd, rendered, false, showTokens); err != nil {
							return err
						}
					}
				} else {
					results = append(results, res)
					if !jsonOutput {
						if len(paths) > 1 {
							fmt.Fprintf(cmd.OutOrStdout(), "=== %s (%d lines) ===\n", res.SourceFile, len(res.Lines))
						}
						mode := "off"
						if number || adaptive {
							mode = "all"
						}
						if cmd.Flags().Changed("line-numbers") {
							mode = lineNumbers
						}
						if rawMode {
							mode = "off"
						}
						formatted, err := readcard.FormatTextCadence(res, mode)
						if err != nil {
							return err
						}
						fmt.Fprintln(cmd.OutOrStdout(), formatted)
						if showTokens {
							printTextTokens(cmd, res.TokenStats)
						}
						if i < len(paths)-1 {
							fmt.Fprintln(cmd.OutOrStdout())
						}
					}
				}
			}
			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if len(results) == 1 {
					return enc.Encode(results[0])
				}
				return enc.Encode(results)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&imageMode, "image", "I", false, "force styled PNG output")
	cmd.Flags().BoolVar(&autoMode, "auto", false, "choose images or text by provider token estimates")
	cmd.Flags().BoolVar(&textMode, "text", false, "force text output")
	cmd.Flags().BoolVar(&rawMode, "raw", false, "force text without line numbers")
	cmd.Flags().StringVarP(&outputPath, "out", "o", "", "output PNG file or existing directory")
	cmd.Flags().IntVarP(&columns, "columns", "c", 3, "maximum columns (1-4; unused columns pruned)")
	cmd.Flags().StringVar(&fontName, "font", "pixel", "font: pixel, retro, 5x8, 3x5, micro, 6x12, standard, 8x16, 7x13")
	cmd.Flags().IntVar(&fontSize, "font-size", 11, "font size in pixels")
	cmd.Flags().StringVar(&theme, "theme", "dark", "color theme: dark, light")
	cmd.Flags().StringVar(&wrapMode, "wrap", "soft", "line wrapping: soft, truncate")
	cmd.Flags().IntVar(&maxDim, "max-dim", 1568, "maximum image dimension in pixels")
	cmd.Flags().BoolVarP(&number, "number", "n", false, "force text output with line numbers (unless -I)")
	cmd.Flags().StringVar(&lineNumbers, "line-numbers", "all", "gutter: all, off, none, or positive cadence N")
	cmd.Flags().StringVar(&compression, "compress", "off", "safe source compression: off, ws, ast")
	cmd.Flags().StringVarP(&lineRange, "lines", "L", "", "source line range, e.g. 10:50")
	cmd.Flags().IntVar(&head, "head", 0, "read only the first N lines")
	cmd.Flags().IntVar(&tail, "tail", 0, "read only the last N lines")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "structured JSON metadata")
	cmd.Flags().BoolVar(&showTokens, "tokens", false, "show token estimates")
	cmd.Flags().BoolVar(&showTokens, "stats", false, "alias for --tokens")
	return cmd
}

func outputRenderResult(cmd *cobra.Command, res *readcard.RenderResult, jsonFmt, showTokens bool) error {
	if jsonFmt {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	for _, f := range res.Files {
		fmt.Fprintf(cmd.OutOrStdout(), "See @%s (%dx%d px, %d col, %d lines)\n", f, res.Width, res.Height, res.Columns, res.TotalLines)
	}

	if !showTokens {
		return nil
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
