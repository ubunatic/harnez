package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/readcard"
)

func newDocsCardsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cards",
		Short: "Build and validate visual PNG documentation cheatsheet cards",
		Long: `cards compiles repository markdown documentation into token-bounded visual PNG
cheatsheet cards (using internal/readcard layout engine) and validates existing card
images against ViT bounds and font resolution rules.`,
	}

	cmd.AddCommand(newDocsCardsBuildCmd())
	cmd.AddCommand(newDocsCardsCheckCmd())
	return cmd
}

func newDocsCardsBuildCmd() *cobra.Command {
	var (
		outDir     string
		bundleName string
		fontSize   int
		theme      string
		maxDim     int
		configPath string
		dir        string
	)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Compile markdown documentation into bounded visual PNG cards",
		Long: `build compiles markdown docs into bounded visual PNG cards.

Supports built-in bundles (e.g. --bundle=dev-3in1 bundling Bash.md, Make.md, Git.md),
individual doc compilation, or building all standard doc cards into docs/vision/ or .harnez/cards/.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if fontSize <= 0 {
				fontSize = 11
			}
			if maxDim <= 0 {
				maxDim = 1568
			}
			if theme == "" {
				theme = "dark"
			}

			if outDir == "" {
				outDir = filepath.Join(dir, "docs", "vision")
			}

			if err := os.MkdirAll(outDir, 0755); err != nil {
				return fmt.Errorf("create output directory: %w", err)
			}

			// Handle specific bundle
			if bundleName != "" {
				return buildBundle(cmd, cfg, dir, bundleName, outDir, fontSize, theme, maxDim)
			}

			// Default behavior: build all standard doc cards + built-in bundles
			var builtCards []string

			// Build dev-3in1 bundle
			dev3in1Out := filepath.Join(outDir, "dev_3in1.png")
			if err := buildBundle(cmd, cfg, dir, "dev-3in1", dev3in1Out, fontSize, theme, maxDim); err != nil {
				// if individual files don't exist locally, still proceed with available docs
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: bundle dev-3in1: %v\n", err)
			} else {
				builtCards = append(builtCards, dev3in1Out)
			}

			// Build individual language cards
			for name, lang := range cfg.AgentsMD.Languages {
				docPath := filepath.Join(dir, lang.Source)
				content, err := os.ReadFile(docPath)
				if err != nil {
					// Fallback to reading from embedded filesystem if local file absent
					if cfg.FS != nil {
						content, err = fs.ReadFile(cfg.FS, lang.Source)
					}
					if err != nil {
						continue
					}
				}

				lines := strings.Split(string(content), "\n")
				cardOut := filepath.Join(outDir, fmt.Sprintf("%s.png", name))

				renderRes, err := readcard.RenderFileToCards(lines, filepath.Base(lang.Source), readcard.RenderOptions{
					Columns:         2,
					FontSize:        fontSize,
					Theme:           theme,
					MaxDimension:    maxDim,
					ShowLineNumbers: false,
					OutputPath:      cardOut,
					Title:           lang.Name,
				})
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: render %s card: %v\n", name, err)
					continue
				}

				for _, f := range renderRes.Files {
					builtCards = append(builtCards, f)
					fmt.Fprintf(cmd.OutOrStdout(), "🖼️ Built card: %s (%dx%d px)\n", f, renderRes.Width, renderRes.Height)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully built %d visual documentation cards in %s\n", len(builtCards), outDir)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outDir, "out", "o", "", "output directory or file path for generated card PNGs (default: ./docs/vision/)")
	cmd.Flags().StringVarP(&bundleName, "bundle", "b", "", "named card bundle to build (e.g. dev-3in1)")
	cmd.Flags().IntVar(&fontSize, "font-size", 11, "font size in pixels (default: 11)")
	cmd.Flags().StringVar(&theme, "theme", "dark", "color theme (dark or light)")
	cmd.Flags().IntVar(&maxDim, "max-dim", 1568, "maximum dimension bounds (default: 1568)")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "project root directory (default: current directory)")

	return cmd
}

func buildBundle(cmd *cobra.Command, cfg *claude.Config, dir, bundleName, targetOut string, fontSize int, theme string, maxDim int) error {
	var sections []readcard.CardSection
	var title, badge string

	normBundle := strings.ToLower(strings.TrimSpace(bundleName))
	switch normBundle {
	case "dev-3in1", "dev_3in1", "dev":
		title = "Harnez Core Developer Cheatsheet (3-in-1)"
		badge = "Bash + Make + Git | 3-Column Micro-Grid"
		docDefs := []struct {
			name     string
			fallback string
		}{
			{"bash", "docs/lang/Bash.md"},
			{"make", "docs/lang/Make.md"},
			{"git", "docs/lang/Git.md"},
		}

		for _, d := range docDefs {
			docPath := filepath.Join(dir, d.fallback)
			content, err := os.ReadFile(docPath)
			if err != nil {
				if cfg.FS != nil {
					content, err = fs.ReadFile(cfg.FS, d.fallback)
				}
				if err != nil {
					return fmt.Errorf("read doc %s: %w", d.fallback, err)
				}
			}
			secTitle := strings.ToUpper(d.name[:1]) + d.name[1:] + " Rules"
			if lang, ok := cfg.AgentsMD.Languages[d.name]; ok && lang.Name != "" {
				secTitle = lang.Name
			}
			sections = append(sections, readcard.CardSection{
				Title:    secTitle,
				Filename: filepath.Base(d.fallback),
				Lines:    strings.Split(string(content), "\n"),
			})
		}
	default:
		return fmt.Errorf("unknown bundle %q: available bundles: dev-3in1", bundleName)
	}

	outPath := targetOut
	if fi, err := os.Stat(targetOut); err == nil && fi.IsDir() {
		outPath = filepath.Join(targetOut, fmt.Sprintf("%s.png", bundleName))
	} else if strings.HasSuffix(targetOut, "/") {
		_ = os.MkdirAll(targetOut, 0755)
		outPath = filepath.Join(targetOut, fmt.Sprintf("%s.png", bundleName))
	}

	res, err := readcard.RenderBundleCard(sections, readcard.BundleOptions{
		Title:        title,
		Badge:        badge,
		Columns:      len(sections),
		FontSize:     fontSize,
		Theme:        theme,
		MaxDimension: maxDim,
		OutputPath:   outPath,
	})
	if err != nil {
		return fmt.Errorf("render bundle %s: %w", bundleName, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🖼️ Built bundle card: %s (%dx%d px, %d columns, ~%d ViT tokens)\n", res.PrimaryPath, res.Width, res.Height, res.Columns, res.TokenStats.ClaudeTokens)
	return nil
}

func newDocsCardsCheckCmd() *cobra.Command {
	var maxDim int

	cmd := &cobra.Command{
		Use:   "check <files...>",
		Short: "Validate card image files against max bounds (<=1568px) and font ladder rules",
		Long: `check inspects PNG card files to guarantee they satisfy:
1. Longest edge <= 1568px (avoiding server-side downsampling cliffs in Claude and OpenAI).
2. Minimum font resolution (>= 9.5px font ladder rule) for 100% OCR reliability.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if maxDim <= 0 {
				maxDim = 1568
			}

			var failed int
			var checked int

			for _, path := range args {
				// Expand globs if shell didn't
				matches, err := filepath.Glob(path)
				if err != nil || len(matches) == 0 {
					matches = []string{path}
				}

				for _, m := range matches {
					checked++
					res, err := readcard.CheckCard(m, maxDim)
					if err != nil {
						failed++
						fmt.Fprintf(cmd.ErrOrStderr(), "❌ %s: %v\n", m, err)
						continue
					}
					if !res.Passed {
						failed++
						fmt.Fprintf(cmd.ErrOrStderr(), "❌ %s: %s (%dx%d px)\n", res.Path, res.Error, res.Width, res.Height)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "✅ %s: %dx%d px (bounds <= %dpx, font >= 9.5px OK)\n", res.Path, res.Width, res.Height, maxDim)
					}
				}
			}

			if failed > 0 {
				return fmt.Errorf("%d of %d card check(s) failed", failed, checked)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "All %d card image(s) passed validation checks.\n", checked)
			return nil
		},
	}

	cmd.Flags().IntVar(&maxDim, "max-dim", 1568, "maximum allowed dimension in pixels (default: 1568)")
	return cmd
}
