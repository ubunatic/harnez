package readcard

import "strings"

const (
	// MicroSnippetLineThreshold and MicroSnippetTokenThreshold avoid image floors.
	MicroSnippetLineThreshold  = 5
	MicroSnippetTokenThreshold = 100
)

// Provider identifies an estimated vision pricing profile, not a precise model tariff.
type Provider string

const (
	ProviderUnknown Provider = "unknown"
	ProviderClaude  Provider = "claude"
	ProviderOpenAI  Provider = "openai"
	ProviderGemini  Provider = "gemini"
	ProviderLocal   Provider = "local"
)

// ParseProvider maps harness/model names to supported conservative profiles.
func ParseProvider(name string) Provider {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "claude", "anthropic":
		return ProviderClaude
	case "codex", "openai":
		return ProviderOpenAI
	case "gemini", "google", "agy", "antigravity":
		return ProviderGemini
	case "local", "ollama", "lmcoder":
		return ProviderLocal
	default:
		return ProviderUnknown
	}
}

// DetectProvider honors an explicit harness marker before common environment markers.
// Ambiguous simultaneous markers conservatively select text.
func DetectProvider(getenv func(string) string) Provider {
	if name := getenv("HARNEZ_AGENT_HARNESS"); name != "" {
		return ParseProvider(name)
	}
	found := ProviderUnknown
	for _, marker := range []struct {
		key      string
		provider Provider
	}{
		{"CLAUDE_CODE", ProviderClaude}, {"CLAUDECODE", ProviderClaude},
		{"CODEX_CLI", ProviderOpenAI}, {"CODEX_THREAD_ID", ProviderOpenAI},
		{"GEMINI_CLI", ProviderGemini},
	} {
		if value := getenv(marker.key); value != "" && value != "0" && value != "false" {
			if found != ProviderUnknown && found != marker.provider {
				return ProviderUnknown
			}
			found = marker.provider
		}
	}
	return found
}

// PreferImage compares exact integer estimates, avoiding rounded ratio boundaries.
func PreferImage(provider Provider, totalLines int, stats TokenStats) bool {
	if totalLines <= MicroSnippetLineThreshold && stats.TextTokens < MicroSnippetTokenThreshold {
		return false
	}
	cost := 0
	switch provider {
	case ProviderClaude:
		cost = stats.ClaudeTokens
	case ProviderOpenAI:
		cost = stats.OpenAITokens
	case ProviderGemini:
		cost = stats.GeminiTokens
	}
	return cost > 0 && stats.TextTokens >= cost
}
