package usage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
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

// CodexWhamUsageResponse models GET https://chatgpt.com/backend-api/wham/usage
type CodexWhamUsageResponse struct {
	PlanType  string `json:"plan_type"`
	RateLimit *struct {
		Allowed       bool `json:"allowed"`
		LimitReached  bool `json:"limit_reached"`
		PrimaryWindow *struct {
			UsedPercent        float64 `json:"used_percent"`
			LimitWindowSeconds int64   `json:"limit_window_seconds"`
			ResetAfterSeconds  int64   `json:"reset_after_seconds"`
			ResetAt            int64   `json:"reset_at"`
		} `json:"primary_window"`
		SecondaryWindow *struct {
			UsedPercent        float64 `json:"used_percent"`
			LimitWindowSeconds int64   `json:"limit_window_seconds"`
			ResetAfterSeconds  int64   `json:"reset_after_seconds"`
			ResetAt            int64   `json:"reset_at"`
		} `json:"secondary_window"`
	} `json:"rate_limit"`
	Credits *struct {
		HasCredits          bool   `json:"has_credits"`
		Unlimited           bool   `json:"unlimited"`
		OverageLimitReached bool   `json:"overage_limit_reached"`
		Balance             string `json:"balance"`
	} `json:"credits"`
}

// CollectCodex inspects ~/.codex for auth status, plan tier, active model config, and queries live rate limits when online.
func CollectCodex(ctx context.Context, codexDir string, client *http.Client) AgentUsage {
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

	// 1. Read config.toml for model & reasoning configuration
	configPath := filepath.Join(codexDir, "config.toml")
	if data, err := os.ReadFile(configPath); err == nil {
		usage.Sources = append(usage.Sources, "~/.codex/config.toml")
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
	usage.Sources = append(usage.Sources, "~/.codex/auth.json")

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

	accessToken := auth.Tokens["access_token"]
	idToken := auth.Tokens["id_token"]
	if idToken == "" {
		idToken = accessToken
	}

	if idToken == "" && accessToken == "" {
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

	// 3. Query live quota endpoint if client and access_token are provided
	if client != nil && accessToken != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://chatgpt.com/backend-api/wham/usage", nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+accessToken)
			if accountID, ok := auth.Tokens["account_id"]; ok && accountID != "" {
				req.Header.Set("ChatGPT-Account-ID", accountID)
			}
			req.Header.Set("User-Agent", "codex")
			req.Header.Set("Accept", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				usage.QuotaFetchError = err.Error()
			} else {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var whamUsage CodexWhamUsageResponse
					if err := json.NewDecoder(resp.Body).Decode(&whamUsage); err != nil {
						usage.QuotaFetchError = fmt.Sprintf("decode error: %v", err)
					} else {
						usage.Sources = append(usage.Sources, "chatgpt.com/backend-api/wham/usage")
						now := time.Now()
						if whamUsage.RateLimit != nil && whamUsage.RateLimit.PrimaryWindow != nil {
							pw := whamUsage.RateLimit.PrimaryWindow
							usedPct := pw.UsedPercent
							remPct := 100.0 - usedPct
							if remPct < 0 {
								remPct = 0
							}

							windowName := "Weekly"
							if pw.LimitWindowSeconds > 0 && pw.LimitWindowSeconds < 86400 {
								windowName = fmt.Sprintf("%d-Hour", pw.LimitWindowSeconds/3600)
							}

							qw := QuotaWindow{
								Name:             windowName,
								UsedPercent:      usedPct,
								RemainingPercent: remPct,
							}
							if pw.ResetAt > 0 {
								t := time.Unix(pw.ResetAt, 0)
								qw.ResetAt = &t
								if t.After(now) {
									qw.DurationLeft = t.Sub(now)
								}
							} else if pw.ResetAfterSeconds > 0 {
								t := now.Add(time.Duration(pw.ResetAfterSeconds) * time.Second)
								qw.ResetAt = &t
								qw.DurationLeft = time.Duration(pw.ResetAfterSeconds) * time.Second
							}
							usage.Weekly = &qw
						}

						if whamUsage.PlanType != "" {
							usage.PlanTier = strings.ToUpper(whamUsage.PlanType[:1]) + whamUsage.PlanType[1:]
						}

						if whamUsage.Credits != nil && whamUsage.Credits.Balance != "" && whamUsage.Credits.Balance != "0" {
							usage.Details["credits_balance"] = whamUsage.Credits.Balance
						}
					}
				} else {
					usage.QuotaFetchError = fmt.Sprintf("HTTP %d", resp.StatusCode)
				}
			}
		}
	}

	return usage
}
