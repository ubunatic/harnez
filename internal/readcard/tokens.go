package readcard

import (
	"math"
)

// TokenStats holds token cost metrics for both raw text and visual rendering.
type TokenStats struct {
	TextBytes     int     `json:"text_bytes"`
	TextWords     int     `json:"text_words"`
	TextTokens    int     `json:"text_tokens"`
	ImageWidth    int     `json:"image_width,omitempty"`
	ImageHeight   int     `json:"image_height,omitempty"`
	TotalPages    int     `json:"total_pages,omitempty"`
	ClaudeTokens  int     `json:"claude_vision_tokens,omitempty"`
	ClaudeRatio   float64 `json:"claude_compression_ratio,omitempty"`
	OpenAITokens  int     `json:"openai_vision_tokens,omitempty"`
	OpenAIRatio   float64 `json:"openai_compression_ratio,omitempty"`
	GeminiTokens  int     `json:"gemini_vision_tokens,omitempty"`
	GeminiRatio   float64 `json:"gemini_compression_ratio,omitempty"`
}

// ComputeTextTokens estimates token count from raw text bytes and words.
func ComputeTextTokens(text string) TokenStats {
	bytesLen := len(text)
	words := len(splitWords(text))
	// Standard token estimate: ~3.75 chars per token or ~1.33 tokens per word
	tokens := int(math.Ceil(float64(bytesLen) / 3.75))
	if tokens == 0 && bytesLen > 0 {
		tokens = 1
	}

	return TokenStats{
		TextBytes:  bytesLen,
		TextWords:  words,
		TextTokens: tokens,
	}
}

// ComputeImageTokens calculates ViT token costs for an image of width x height.
func ComputeImageTokens(textTokens int, textBytes int, width, height, pages int) TokenStats {
	if pages <= 0 {
		pages = 1
	}

	// Claude ViT: ~ Area / 750 tokens per page
	singleClaude := int(math.Ceil(float64(width*height) / 750.0))
	claudeTokens := singleClaude * pages

	// OpenAI / Codex ViT:
	// Scale so max side <= 2048, min side <= 768
	scaledW := float64(width)
	scaledH := float64(height)
	if scaledW > 2048 || scaledH > 2048 {
		scale := math.Min(2048.0/scaledW, 2048.0/scaledH)
		scaledW *= scale
		scaledH *= scale
	}
	if math.Min(scaledW, scaledH) > 768 {
		shortScale := 768.0 / math.Min(scaledW, scaledH)
		scaledW *= shortScale
		scaledH *= shortScale
	}
	tilesX := math.Ceil(scaledW / 512.0)
	tilesY := math.Ceil(scaledH / 512.0)
	singleOpenAI := int(85 + (170 * tilesX * tilesY))
	openAITokens := singleOpenAI * pages

	// Gemini ViT: ~258 tokens per 384x384 / tile grid
	geminiTilesX := math.Ceil(float64(width) / 384.0)
	geminiTilesY := math.Ceil(float64(height) / 384.0)
	singleGemini := int(258 * (geminiTilesX * geminiTilesY))
	if singleGemini < 258 {
		singleGemini = 258
	}
	geminiTokens := singleGemini * pages

	claudeRatio := 0.0
	openAIRatio := 0.0
	geminiRatio := 0.0
	if textTokens > 0 {
		if claudeTokens > 0 {
			claudeRatio = round2(float64(textTokens) / float64(claudeTokens))
		}
		if openAITokens > 0 {
			openAIRatio = round2(float64(textTokens) / float64(openAITokens))
		}
		if geminiTokens > 0 {
			geminiRatio = round2(float64(textTokens) / float64(geminiTokens))
		}
	}

	return TokenStats{
		TextBytes:    textBytes,
		TextTokens:   textTokens,
		ImageWidth:   width,
		ImageHeight:  height,
		TotalPages:   pages,
		ClaudeTokens: claudeTokens,
		ClaudeRatio:  claudeRatio,
		OpenAITokens: openAITokens,
		OpenAIRatio:  openAIRatio,
		GeminiTokens: geminiTokens,
		GeminiRatio:  geminiRatio,
	}
}

func round2(val float64) float64 {
	return math.Round(val*100) / 100
}

func splitWords(s string) []string {
	var words []string
	inWord := false
	start := 0
	for i, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if inWord {
				words = append(words, s[start:i])
				inWord = false
			}
		} else {
			if !inWord {
				start = i
				inWord = true
			}
		}
	}
	if inWord {
		words = append(words, s[start:])
	}
	return words
}
