package usage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AGYOauthToken models ~/.gemini/antigravity-cli/antigravity-oauth-token
type AGYOauthToken struct {
	Token      any    `json:"token"`
	AuthMethod string `json:"auth_method"`
}

// AGYSettings models ~/.gemini/antigravity-cli/settings.json
type AGYSettings struct {
	Model string `json:"model"`
}

// CollectAGY inspects ~/.gemini/antigravity-cli for token, model settings, and session logs.
func CollectAGY(ctx context.Context, geminiDir string) AgentUsage {
	usage := AgentUsage{
		AgentID:      "agy",
		Name:         "Antigravity (AGY)",
		Details:      make(map[string]string),
		ExtraWindows: make(map[string]QuotaWindow),
	}

	if geminiDir == "" {
		home, _ := os.UserHomeDir()
		geminiDir = filepath.Join(home, ".gemini", "antigravity-cli")
	}

	if _, err := os.Stat(geminiDir); os.IsNotExist(err) {
		// Try fallback ~/.gemini
		home, _ := os.UserHomeDir()
		fallbackDir := filepath.Join(home, ".gemini")
		if _, err := os.Stat(fallbackDir); os.IsNotExist(err) {
			usage.Installed = false
			return usage
		}
	}
	usage.Installed = true

	// 1. Read settings.json
	settingsPath := filepath.Join(geminiDir, "settings.json")
	if data, err := os.ReadFile(settingsPath); err == nil {
		var s AGYSettings
		if err := json.Unmarshal(data, &s); err == nil && s.Model != "" {
			usage.ActiveModel = s.Model
		}
	}

	// 2. Read antigravity-oauth-token
	tokenPath := filepath.Join(geminiDir, "antigravity-oauth-token")
	if data, err := os.ReadFile(tokenPath); err == nil {
		var tok AGYOauthToken
		if err := json.Unmarshal(data, &tok); err == nil {
			if tok.Token != nil || tok.AuthMethod != "" {
				usage.Authenticated = true
				if tok.AuthMethod != "" {
					usage.PlanTier = strings.ToUpper(tok.AuthMethod[:1]) + tok.AuthMethod[1:]
				}
			}
		}
	}

	// 3. Inspect recent log for masked account if available
	logDir := filepath.Join(geminiDir, "log")
	if entries, err := os.ReadDir(logDir); err == nil && len(entries) > 0 {
		// Read the latest log file backwards / forwards looking for applyAuthResult email
		for i := len(entries) - 1; i >= 0 && usage.Account == ""; i-- {
			if !strings.HasSuffix(entries[i].Name(), ".log") {
				continue
			}
			f, err := os.Open(filepath.Join(logDir, entries[i].Name()))
			if err != nil {
				continue
			}
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				if idx := strings.Index(line, "email="); idx != -1 {
					sub := line[idx+len("email="):]
					if commaIdx := strings.Index(sub, ","); commaIdx != -1 {
						email := strings.TrimSpace(sub[:commaIdx])
						if email != "" && strings.Contains(email, "@") {
							usage.Account = MaskAccount(email)
							break
						}
					}
				}
			}
			f.Close()
		}
	}

	// 4. Count conversations/sessions
	convDir := filepath.Join(geminiDir, "conversations")
	if entries, err := os.ReadDir(convDir); err == nil {
		dbCount := 0
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".db") {
				dbCount++
			}
		}
		if dbCount > 0 {
			usage.Details["total_conversations"] = fmt.Sprintf("%d", dbCount)
		}
	}

	return usage
}
