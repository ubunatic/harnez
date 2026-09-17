// canary-visual-doc — end-to-end visual cheatsheet subagent dispatch and mechanical lint canary.
//
// Spawns an isolated agent session (in a clean scratch directory outside any
// harnez-managed project, so no local AGENTS.md or CLAUDE.md is auto-injected)
// whose ONLY context is an attached visual cheatsheet image (e.g. Bash_2col.png
// or dev_cheatsheet_3in1.png) and ZERO text rules.
//
// After dispatching the task, it mechanically lints the generated output file
// with internal/lint (the same engine backing `harnez lint`), logs ViT token
// metrics across harnesses (Claude, OpenAI, Gemini), and compares compression
// ratios against raw text baselines. See issues 393 and 394.
package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/lint"
)

type TokenStats struct {
	Width        int
	Height       int
	ClaudeTokens int
	OpenAITokens int
	GeminiTokens int

	TextBytes  int
	TextTokens int
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("canary-visual-doc", flag.ContinueOnError)

	imageFlag := fs.String("image", "", "path to cheatsheet image (png/jpg)")
	taskFlag := fs.String("task", "", "path to task markdown fixture")
	outFlag := fs.String("out", "", "expected output filename (e.g. deploy-check.sh)")
	baselineFlag := fs.String("baseline", "", "optional path to baseline markdown doc for token comparison")
	agentFlag := fs.String("agent", "claude", "agent CLI executable to run (default: claude)")
	dryRunFlag := fs.Bool("dry-run", false, "skip agent dispatch; evaluate token metrics and lint if file exists")
	keepFlag := fs.Bool("keep", false, "preserve temporary scratch directory on completion")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	imagePath := *imageFlag
	taskPath := *taskFlag
	outName := *outFlag

	// Allow positional arguments if flags were omitted:
	// canary-visual-doc <image-path> <task-file> <output-filename>
	if imagePath == "" && len(remaining) > 0 {
		imagePath = remaining[0]
	}
	if taskPath == "" && len(remaining) > 1 {
		taskPath = remaining[1]
	}
	if outName == "" && len(remaining) > 2 {
		outName = remaining[2]
	}

	if imagePath == "" || taskPath == "" || outName == "" {
		fs.Usage()
		return fmt.Errorf("usage: canary-visual-doc [flags] <image-path> <task-file> <output-filename>\n" +
			"example: canary-visual-doc -baseline docs/lang/Bash.md scratch/vision/Bash_2col.png " +
			"scripts/canary-lite-doc/fixtures/bash-deploy-check.task.md deploy-check.sh")
	}

	imageAbs, err := filepath.Abs(imagePath)
	if err != nil {
		return fmt.Errorf("resolve image path: %w", err)
	}
	if _, err := os.Stat(imageAbs); err != nil {
		return fmt.Errorf("image not found: %w", err)
	}

	taskAbs, err := filepath.Abs(taskPath)
	if err != nil {
		return fmt.Errorf("resolve task path: %w", err)
	}
	taskText, err := os.ReadFile(taskAbs)
	if err != nil {
		return fmt.Errorf("read task file: %w", err)
	}

	stats, err := computeTokenMetrics(imageAbs, *baselineFlag)
	if err != nil {
		return fmt.Errorf("compute token metrics: %w", err)
	}

	printBenchmarkHeader(imagePath, taskPath, outName, *baselineFlag, stats)

	work, err := os.MkdirTemp("", "canary-visual-doc.*")
	if err != nil {
		return fmt.Errorf("create scratch dir: %w", err)
	}
	if !*keepFlag {
		defer os.RemoveAll(work)
	} else {
		fmt.Printf("Notice: keeping scratch directory at %s\n", work)
	}

	ext := strings.ToLower(filepath.Ext(imageAbs))
	if ext == "" {
		ext = ".png"
	}
	stagedImageName := "STYLE_GUIDE" + ext
	stagedImagePath := filepath.Join(work, stagedImageName)
	if err := copyFile(imageAbs, stagedImagePath); err != nil {
		return fmt.Errorf("copy cheatsheet image into scratch dir: %w", err)
	}

	prompt := fmt.Sprintf(`You are an automated coding subagent in an isolated workspace.
Your ONLY style guide is the attached image file: %s
You have ZERO text rules or text style guides. All coding style, syntax, and formatting conventions MUST be read and followed directly from %s.

Task:
%s

Write the required output file (%s) to disk.`, stagedImageName, stagedImageName, strings.TrimSpace(string(taskText)), outName)

	outPath := filepath.Join(work, outName)

	if !*dryRunFlag {
		fmt.Printf("\n==> Dispatching isolated agent (%s) in %s ...\n", *agentFlag, work)
		var cmd *exec.Cmd
		switch *agentFlag {
		case "claude":
			cmd = exec.Command("claude", "-p", "--permission-mode", "bypassPermissions", prompt)
		default:
			cmd = exec.Command(*agentFlag, prompt)
		}
		cmd.Dir = work
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("agent dispatch (%s): %w", *agentFlag, err)
		}
	} else {
		fmt.Println("\n==> Dry-run mode enabled; skipping agent execution.")
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		if *dryRunFlag {
			fmt.Println("No output file generated (dry-run). Token analysis complete.")
			return nil
		}
		return fmt.Errorf("agent did not produce expected output file %s: %w", outPath, err)
	}

	fmt.Printf("\n=== generated output: %s (%d bytes) ===\n%s\n", outPath, len(content), string(content))

	findings := lint.DefaultLinter().LintBytes(outName, content, lint.LangAuto)
	fmt.Println("=== mechanical lint evaluation ===")
	if len(findings) == 0 {
		fmt.Printf("✅ PASS: 0 lint findings for %s (image=%s, task=%s)\n", outName, filepath.Base(imagePath), filepath.Base(taskPath))
		return nil
	}

	for _, f := range findings {
		fmt.Printf("%s:%d:%d: %s [%s]\n", outName, f.Line, f.Col, f.Message, f.RuleID)
	}
	return fmt.Errorf("❌ FAIL: %d mechanical lint finding(s) for %s", len(findings), outName)
}

func computeTokenMetrics(imagePath, baselineDocPath string) (*TokenStats, error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, fmt.Errorf("decode image geometry: %w", err)
	}

	stats := &TokenStats{
		Width:  cfg.Width,
		Height: cfg.Height,
	}

	// Claude ViT: ~ Area / 750
	stats.ClaudeTokens = int(math.Ceil(float64(cfg.Width*cfg.Height) / 750.0))

	// OpenAI / Codex ViT: 85 base + 170 per 512x512 tile
	tilesW := int(math.Ceil(float64(cfg.Width) / 512.0))
	tilesH := int(math.Ceil(float64(cfg.Height) / 512.0))
	stats.OpenAITokens = 85 + 170*(tilesW*tilesH)

	// Gemini ViT: 258 per 384x384 tile
	geminiTilesW := int(math.Ceil(float64(cfg.Width) / 384.0))
	geminiTilesH := int(math.Ceil(float64(cfg.Height) / 384.0))
	stats.GeminiTokens = 258 * (geminiTilesW * geminiTilesH)

	if baselineDocPath != "" {
		docBytes, err := os.ReadFile(baselineDocPath)
		if err == nil {
			stats.TextBytes = len(docBytes)
			// Text token rule of thumb: ~3.75 chars per token
			stats.TextTokens = int(math.Ceil(float64(len(docBytes)) / 3.75))
		}
	}

	return stats, nil
}

func printBenchmarkHeader(imagePath, taskPath, outName, baselinePath string, stats *TokenStats) {
	fmt.Println("================================================================================")
	fmt.Println("               MULTIMODAL DOC CANARY & TOKEN BENCHMARK HARNESS                 ")
	fmt.Println("================================================================================")
	fmt.Printf("Image Cheatsheet : %s (%dx%d px)\n", imagePath, stats.Width, stats.Height)
	fmt.Printf("Task Fixture     : %s\n", taskPath)
	fmt.Printf("Expected Output  : %s\n", outName)
	if baselinePath != "" {
		fmt.Printf("Baseline Doc     : %s (%d bytes, ~%d tokens)\n", baselinePath, stats.TextBytes, stats.TextTokens)
	}
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println("ViT Multimodal Token Cost Breakdown:")
	fmt.Printf("  • OpenAI / Codex (tiles) : %5d tokens", stats.OpenAITokens)
	if stats.TextTokens > 0 {
		fmt.Printf("  (Compression: %5.2fx)", float64(stats.TextTokens)/float64(stats.OpenAITokens))
	}
	fmt.Println()

	fmt.Printf("  • Google Gemini (tiles)  : %5d tokens", stats.GeminiTokens)
	if stats.TextTokens > 0 {
		fmt.Printf("  (Compression: %5.2fx)", float64(stats.TextTokens)/float64(stats.GeminiTokens))
	}
	fmt.Println()

	fmt.Printf("  • Anthropic Claude (area): %5d tokens", stats.ClaudeTokens)
	if stats.TextTokens > 0 {
		fmt.Printf("  (Compression: %5.2fx)", float64(stats.TextTokens)/float64(stats.ClaudeTokens))
	}
	fmt.Println()
	fmt.Println("================================================================================")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
