package distill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"ubunatic.com/harnez/internal/readcard"
)

// ReadOutputRequest contains text already returned by an authorized native tool.
// Path is a label only; the adapter never opens it or runs source commands.
type ReadOutputRequest struct {
	Text      string            `json:"text"`
	Path      string            `json:"path"`
	StartLine int               `json:"start_line"`
	Provider  readcard.Provider `json:"provider"`
	Vision    bool              `json:"vision"`
	MaxTokens int               `json:"max_tokens"`
}

// ReadOutput is an adapter-neutral payload. Callers must attach Images, not just
// stringify their paths, and preserve native error/truncation metadata themselves.
type ReadOutput struct {
	Text      string   `json:"text,omitempty"`
	Images    []string `json:"images,omitempty"`
	Truncated bool     `json:"truncated"`
	NextLine  int      `json:"next_line,omitempty"`
}

// DistillReadOutput bounds native read output and optionally returns PNG assets.
func DistillReadOutput(req ReadOutputRequest) (ReadOutput, error) {
	if req.MaxTokens <= 0 {
		req.MaxTokens = 2000
	}
	if req.MaxTokens > 16000 {
		return ReadOutput{}, fmt.Errorf("max_tokens cannot exceed 16000")
	}
	if req.StartLine < 1 {
		req.StartLine = 1
	}
	text := req.Text
	budget := req.MaxTokens * 3 // Below the estimator's 3.75 bytes/token; reserve labels.
	result := ReadOutput{}
	if len(text) > budget {
		cut := budget
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
		if newline := strings.LastIndexByte(text[:cut], '\n'); newline > 0 {
			cut = newline + 1
		}
		text = text[:cut]
		result.Truncated = true
		result.NextLine = req.StartLine + strings.Count(text, "\n")
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if req.Vision {
		opts := readcard.RenderOptions{Title: filepath.Base(req.Path), StartLine: req.StartLine, ShowLineNumbers: true, MeasureOnly: true}
		measured, err := readcard.RenderFileToCards(lines, req.Path, opts)
		if err != nil {
			return result, err
		}
		if readcard.PreferImage(req.Provider, len(lines), measured.TokenStats) {
			dir, err := os.MkdirTemp("", "harnez-read-output-")
			if err != nil {
				return result, err
			}
			opts.MeasureOnly = false
			opts.OutputPath = filepath.Join(dir, "read.png")
			rendered, err := readcard.RenderFileToCards(lines, req.Path, opts)
			if err != nil {
				os.RemoveAll(dir)
				return result, err
			}
			result.Images = rendered.Files
		} else {
			result.Text = text
		}
	} else {
		result.Text = text
	}
	if result.Truncated {
		quoted := "'" + strings.ReplaceAll(req.Path, "'", "'\"'\"'") + "'"
		result.Text += fmt.Sprintf("\n[harnez read: output bounded; continue around line %d with harnez read -n -L %d: -- %s]", result.NextLine, result.NextLine, quoted)
	}
	return result, nil
}
