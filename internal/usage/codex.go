package usage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CodexAuth models ~/.codex/auth.json
type CodexAuth struct {
	AuthMode    string            `json:"auth_mode"`
	OpenAIKey   *string           `json:"OPENAI_API_KEY"`
	Tokens      map[string]string `json:"tokens"`
	LastRefresh string            `json:"last_refresh"`
}

// decodeJWTPayload parses the unverified claims JSON from a standard 3-part JWT token.
func decodeJWTPayload(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, os.ErrInvalid
	}
	payload := parts[1]
	// Handle Base64 URL-safe padding
	if rem := len(payload) % 4; rem > 0 {
		payload += strings.Repeat("=", 4-rem)
	}
	data, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		data, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, err
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// CollectCodex inspects ~/.codex for auth status, plan tier, active model config, and local state.
func CollectCodex(ctx context.Context, codexDir string) AgentUsage {
	usage := AgentUsage{
		AgentID:      "codex",
		Name:         "OpenAI Codex",
		Details:      make(map[string]string),
		ExtraWindows: make(map[string]QuotaWindow),
	}

	if codexDir == "" {
		home, _ := os.UserHomeDir()
		codexDir = filepath.Join(home, ".codex")
	}

	if _, err := os.Stat(codexDir); os.IsNotExist(err) {
		usage.Installed = false
		return usage
	}
	usage.Installed = true

	// 1. Read config.toml for model & reasoning
	configPath := filepath.Join(codexDir, "config.toml")
	if data, err := os.ReadFile(configPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "model =") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					usage.ActiveModel = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				}
			}
			if strings.HasPrefix(line, "model_reasoning_effort =") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					usage.Details["reasoning_effort"] = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				}
			}
		}
	}

	// 2. Read auth.json
	authPath := filepath.Join(codexDir, "auth.json")
	data, err := os.ReadFile(authPath)
	if err != nil {
		usage.Authenticated = false
		return usage
	}

	var auth CodexAuth
	if err := json.Unmarshal(data, &auth); err != nil {
		usage.Authenticated = false
		usage.Error = "invalid auth.json format"
		return usage
	}

	if auth.AuthMode == "api_key" && auth.OpenAIKey != nil && *auth.OpenAIKey != "" {
		usage.Authenticated = true
		usage.PlanTier = "API Key"
		return usage
	}

	idToken := auth.Tokens["id_token"]
	if idToken == "" {
		idToken = auth.Tokens["access_token"]
	}

	if idToken == "" {
		usage.Authenticated = false
		return usage
	}

	usage.Authenticated = true
	claims, err := decodeJWTPayload(idToken)
	if err == nil && claims != nil {
		// Extract email if present
		if profile, ok := claims["https://api.openai.com/profile"].(map[string]any); ok {
			if email, ok := profile["email"].(string); ok && email != "" {
				usage.Account = MaskAccount(email)
			}
		} else if email, ok := claims["email"].(string); ok && email != "" {
			usage.Account = MaskAccount(email)
		}

		// Extract plan tier
		if authObj, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
			if plan, ok := authObj["chatgpt_plan_type"].(string); ok && plan != "" {
				usage.PlanTier = strings.ToUpper(plan[:1]) + plan[1:]
			}
		}

		if expFloat, ok := claims["exp"].(float64); ok {
			expTime := time.Unix(int64(expFloat), 0)
			if expTime.Before(time.Now()) {
				usage.Details["token_expired"] = "true"
			}
		}
	}

	return usage
}
