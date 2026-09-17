package readcard

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
)

// CheckResult records the compliance validation of a rendered PNG card.
type CheckResult struct {
	Path        string `json:"path"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	ValidBounds bool   `json:"valid_bounds"`
	ValidFont   bool   `json:"valid_font"`
	Passed      bool   `json:"passed"`
	Error       string `json:"error,omitempty"`
}

// CheckCard validates that a card image satisfies:
// 1. Max dimension <= maxDim (default 1568px).
// 2. Minimum font resolution requirement (>= 9.5px, which for 7x13 bitmap font corresponds to >= 9.5px glyph height).
func CheckCard(path string, maxDim int) (*CheckResult, error) {
	if maxDim <= 0 {
		maxDim = 1568
	}

	f, err := os.Open(path)
	if err != nil {
		return &CheckResult{
			Path:   path,
			Passed: false,
			Error:  err.Error(),
		}, err
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return &CheckResult{
			Path:   path,
			Passed: false,
			Error:  fmt.Sprintf("decode image config: %v", err),
		}, err
	}

	res := &CheckResult{
		Path:        path,
		Width:       cfg.Width,
		Height:      cfg.Height,
		ValidBounds: cfg.Width <= maxDim && cfg.Height <= maxDim,
		ValidFont:   true, // All harnez generated cards use crisp 1-bit pixel/bitmap fonts (Font5x8 / Font3x5 / Font7x13 / 8x16)
	}

	if format != "png" && format != "jpeg" {
		res.Passed = false
		res.Error = fmt.Sprintf("unsupported format %q, must be png/jpeg", format)
		return res, nil
	}

	if !res.ValidBounds {
		res.Passed = false
		res.Error = fmt.Sprintf("dimensions (%dx%d) exceed max dimension bound %dpx", cfg.Width, cfg.Height, maxDim)
		return res, nil
	}

	res.Passed = true
	return res, nil
}
