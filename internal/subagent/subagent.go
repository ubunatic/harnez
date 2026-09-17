package subagent

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/readcard"
)

// DocMode represents the documentation delivery mode for subagent execution.
type DocMode string

const (
	DocModeFull   DocMode = "full"
	DocModeLite   DocMode = "lite"
	DocModeVision DocMode = "vision"
	DocModeAuto   DocMode = "auto"
)

// ParseDocMode validates and parses a string into a DocMode.
func ParseDocMode(s string) (DocMode, error) {
	norm := strings.ToLower(strings.TrimSpace(s))
	switch norm {
	case "auto":
		return DocModeAuto, nil
	case "vision", "img", "image":
		return DocModeVision, nil
	case "lite", "compact":
		return DocModeLite, nil
	case "full", "standard", "":
		return DocModeFull, nil
	default:
		return "", fmt.Errorf("unknown doc-mode %q: expected auto, vision, lite, or full", s)
	}
}

// StageOptions configures staging of documentation and visual assets in the subagent's scratch/workspace directory.
type StageOptions struct {
	DocMode    DocMode
	CardPath   string            // Optional pre-rendered card PNG path or bundle name (e.g. dev-3in1)
	CardBundle string            // Named bundle (e.g. dev-3in1)
	Task       string            // Subagent prompt/task description
	OutFile    string            // Expected target output file
	Provider   readcard.Provider // Target provider, overriding active harness detection
	LiteRules  string            // Optional concise rule text for providers without image delivery
}

// StagedContext contains metadata about the prepared subagent environment.
type StagedContext struct {
	DocMode        DocMode
	Prompt         string
	StagedCardPath string
	StagedCardName string
}

// StageSubagentContext prepares a subagent workspace directory with appropriate doc context.
// Under DocModeVision, it copies/stages the cheatsheet card as STYLE_GUIDE.png and constructs
// the visual multimodal prompt header. Under DocModeFull or Lite, standard prompt structure is prepared.
func StageSubagentContext(workspaceDir string, opts StageOptions) (*StagedContext, error) {
	if workspaceDir == "" {
		return nil, fmt.Errorf("workspace directory cannot be empty")
	}

	mode := opts.DocMode
	if mode == "" {
		mode = DocModeFull
	}
	if mode == DocModeAuto {
		provider := opts.Provider
		if provider == "" {
			provider = readcard.DetectProvider(os.Getenv)
		}
		mode = DocModeLite
		if provider == readcard.ProviderClaude || provider == readcard.ProviderOpenAI {
			mode = DocModeVision
		}
	}
	if mode != DocModeFull && mode != DocModeLite && mode != DocModeVision {
		return nil, fmt.Errorf("invalid doc mode %q", mode)
	}

	res := &StagedContext{
		DocMode: mode,
	}

	if mode == DocModeVision {
		stagedCardName := "STYLE_GUIDE.png"
		stagedCardPath := filepath.Join(workspaceDir, stagedCardName)

		// Locate card source
		cardSrc := opts.CardPath
		if cardSrc == "" {
			candidate := filepath.Join("docs", "cards", "bundle.png")
			if _, err := os.Stat(candidate); err == nil {
				cardSrc = candidate
			}
		}
		if cardSrc == "" && opts.CardBundle != "" {
			// Check if ./docs/vision/<bundle>.png exists, or fallback
			candidate := filepath.Join("docs", "vision", opts.CardBundle+".png")
			if _, err := os.Stat(candidate); err == nil {
				cardSrc = candidate
			}
		}

		if cardSrc != "" {
			if err := copyFile(cardSrc, stagedCardPath); err != nil {
				return nil, fmt.Errorf("stage visual card: %w", err)
			}
		} else {
			// Generate a default 3-in-1 dev cheatsheet card into workspace
			devSections := []readcard.CardSection{
				{
					Title:    "Bash Rules",
					Filename: "Bash.md",
					Lines: []string{
						"# Bash Rules",
						"- No ';', break before then/else",
						"- Use if test, no [[ ]]",
						"- Use git -C / make -C, not cd",
					},
				},
				{
					Title:    "Make Rules",
					Filename: "Make.md",
					Lines: []string{
						"# Make Rules",
						"- Phony sentinel convention",
						"- Self-documenting help targets",
					},
				},
				{
					Title:    "Git Rules",
					Filename: "Git.md",
					Lines: []string{
						"# Git Rules",
						"- Conventional commits",
						"- Work on default branch",
					},
				},
			}
			_, err := readcard.RenderBundleCard(devSections, readcard.BundleOptions{
				Title:        "Harnez Core Developer Cheatsheet (3-in-1)",
				Badge:        "STYLE_GUIDE.png | 3-Column Micro-Grid",
				Columns:      3,
				FontSize:     11,
				OutputPath:   stagedCardPath,
				MaxDimension: 1568,
			})
			if err != nil {
				return nil, fmt.Errorf("generate default vision card: %w", err)
			}
		}

		res.StagedCardPath = stagedCardPath
		res.StagedCardName = stagedCardName

		res.Prompt = fmt.Sprintf(`Read the staged style guide image %s using your image tool.
Follow its coding style, syntax, and formatting conventions together with repository instructions.

Task:
%s`, stagedCardName, strings.TrimSpace(opts.Task))
		res.Prompt += "\n\n" + readGuidance
		if opts.OutFile != "" {
			res.Prompt += fmt.Sprintf("\n\nWrite the required output file (%s) to disk.", opts.OutFile)
		}
		return res, nil
	}

	// DocModeLite / DocModeFull
	res.Prompt = fmt.Sprintf(`Task:
%s`, strings.TrimSpace(opts.Task))
	if mode == DocModeLite {
		rules := opts.LiteRules
		if rules == "" {
			rules = "Use conventional commits. Follow repository language conventions. Use git -C and make -C for directory targeting. Verify changes with repository checks."
		}
		res.Prompt = "Concise development rules:\n" + rules + "\n\n" + res.Prompt
	}
	res.Prompt += "\n\n" + readGuidance
	if opts.OutFile != "" {
		res.Prompt += fmt.Sprintf("\n\nWrite the required output file (%s) to disk.", opts.OutFile)
	}

	return res, nil
}

const readGuidance = "For files over 100 lines or repeated slices, execute harnez read --auto <file> or harnez read -n -L START:END <file>. Open returned PNG paths with an image tool; a path alone is not an image attachment."

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
