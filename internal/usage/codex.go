package usage

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

// CodexRateWindow models one rate-limit window (primary or secondary) in the
// wham/usage response. Codex reports a short session window (typically
// 5-hour) as primary and a rolling weekly window as secondary, but the
// mapping is done by LimitWindowSeconds rather than assumed from position.
type CodexRateWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	LimitWindowSeconds int64   `json:"limit_window_seconds"`
	ResetAfterSeconds  int64   `json:"reset_after_seconds"`
	ResetAt            int64   `json:"reset_at"`
}

// CodexWhamUsageResponse models GET https://chatgpt.com/backend-api/wham/usage
type CodexWhamUsageResponse struct {
	PlanType  string `json:"plan_type"`
	RateLimit *struct {
		Allowed         bool             `json:"allowed"`
		LimitReached    bool             `json:"limit_reached"`
		PrimaryWindow   *CodexRateWindow `json:"primary_window"`
		SecondaryWindow *CodexRateWindow `json:"secondary_window"`
	} `json:"rate_limit"`
	Credits *struct {
		HasCredits          bool   `json:"has_credits"`
		Unlimited           bool   `json:"unlimited"`
		OverageLimitReached bool   `json:"overage_limit_reached"`
		Balance             string `json:"balance"`
	} `json:"credits"`
}

// codexQuotaPayload is Codex's live-fetch cache payload shape (see the
// shared liveFetchCache gate in livefetchcache.go, issue 033/087).
type codexQuotaPayload struct {
	Session *QuotaWindow `json:"session,omitempty"`
	Weekly  *QuotaWindow `json:"weekly,omitempty"`
}

type codexTokenUsage struct {
	Input, Cached, CacheWrite, Output, Total int64
}

func collectCodexTokens(codexDir string) (*TokenBreakdown, int) {
	root := filepath.Join(codexDir, "sessions")
	bySession := map[string]codexTokenUsage{}
	rollouts := 0
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.HasPrefix(info.Name(), "rollout-") || !strings.HasSuffix(info.Name(), ".jsonl") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		sessionID := path
		var latest codexTokenUsage
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var record struct {
				Type    string `json:"type"`
				Payload struct {
					SessionID string `json:"session_id"`
					Type      string `json:"type"`
					Info      struct {
						Total struct {
							Input      int64 `json:"input_tokens"`
							Cached     int64 `json:"cached_input_tokens"`
							CacheWrite int64 `json:"cache_write_input_tokens"`
							Output     int64 `json:"output_tokens"`
							Total      int64 `json:"total_tokens"`
						} `json:"total_token_usage"`
					} `json:"info"`
				} `json:"payload"`
			}
			if json.Unmarshal(scanner.Bytes(), &record) != nil {
				continue
			}
			if record.Type == "session_meta" && record.Payload.SessionID != "" {
				sessionID = record.Payload.SessionID
			}
			if record.Type == "event_msg" && record.Payload.Type == "token_count" {
				t := record.Payload.Info.Total
				if t.Total > 0 {
					latest = codexTokenUsage{t.Input, t.Cached, t.CacheWrite, t.Output, t.Total}
				}
			}
		}
		if latest.Total > 0 {
			bySession[sessionID] = latest
			rollouts++
		}
		return nil
	})
	if len(bySession) == 0 {
		return nil, rollouts
	}
	total := &TokenBreakdown{}
	for _, t := range bySession {
		total.InputTokens += t.Input
		total.CacheReadTokens += t.Cached
		total.CacheWriteTokens += t.CacheWrite
		total.OutputTokens += t.Output
		total.TotalTokens += t.Total
	}
	return total, rollouts
}

// buildCodexQuotaWindow converts one wham rate-limit window into a QuotaWindow,
// naming it "Weekly" for windows of a day or longer and "N-Hour" otherwise.
func buildCodexQuotaWindow(pw *CodexRateWindow, now time.Time) QuotaWindow {
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
	return qw
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
	if tokens, rollouts := collectCodexTokens(codexDir); tokens != nil {
		usage.Tokens = tokens
		usage.Sources = append(usage.Sources, fmt.Sprintf("~/.codex/sessions (%s rollouts)", strconv.Itoa(rollouts)))
	}

	// 3. Query live quota endpoint if client and access_token are provided,
	// but check the shared on-disk cache first so a warm reading from a
	// sibling `harnez` process (or this process's own last tick)
	// short-circuits the network call entirely (issue 033, generalized to
	// Codex in issue 087).
	if client != nil && accessToken != "" {
		cachePath := liveFetchCachePath(codexDir)
		defer lockLiveFetchInProcess(cachePath)()
		cache := readLiveFetchCache[codexQuotaPayload](cachePath)

		if cache != nil && time.Since(cache.FetchedAt) < MinWatchInterval && !codexQuotaCacheExpired(cache.Payload, time.Now()) {
			usage.Session = cache.Payload.Session
			usage.Weekly = cache.Payload.Weekly
			usage.Sources = append(usage.Sources, "~/.codex/harnez-quota-cache.json")
			return usage
		}

		// Cache is stale or missing: fetch live. Take the advisory lock
		// first (bounded retry, never blocks indefinitely) so only one
		// process at a time writes the refreshed reading to disk; if the
		// lock can't be acquired quickly, still perform the fetch and use
		// its result in-memory for this call, just skip persisting it (a
		// sibling process is presumably writing its own fresh reading right
		// now anyway).
		lockFile, locked := lockLiveFetchCache(cachePath)
		if locked {
			defer unlockLiveFetchCache(lockFile)
		}

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
						if whamUsage.RateLimit != nil {
							for _, pw := range []*CodexRateWindow{whamUsage.RateLimit.PrimaryWindow, whamUsage.RateLimit.SecondaryWindow} {
								if pw == nil {
									continue
								}
								qw := buildCodexQuotaWindow(pw, now)
								if qw.Name == "Weekly" {
									usage.Weekly = &qw
								} else if usage.Session == nil {
									usage.Session = &qw
								}
							}
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
		} else {
			usage.QuotaFetchError = fmt.Sprintf("request build error: %v", err)
		}

		if usage.QuotaFetchError == "" {
			// Live fetch succeeded: persist it for sibling processes/next
			// tick, but only if we actually hold the lock.
			if locked {
				_ = writeLiveFetchCache(cachePath, liveFetchCache[codexQuotaPayload]{
					FetchedAt: time.Now(),
					Payload:   codexQuotaPayload{Session: usage.Session, Weekly: usage.Weekly},
				})
			}
		} else if cache != nil {
			// Live fetch failed: fall back to the disk cache regardless of
			// its age, labeled stale (issue 032's " (stale)" convention),
			// mirroring Claude's behavior.
			if cache.Payload.Session != nil {
				usage.Session = staleQuotaWindow(cache.Payload.Session)
			}
			if cache.Payload.Weekly != nil {
				usage.Weekly = staleQuotaWindow(cache.Payload.Weekly)
			}
			usage.Sources = append(usage.Sources, "~/.codex/harnez-quota-cache.json (stale)")
		}
	}

	return usage
}

func codexQuotaCacheExpired(payload codexQuotaPayload, now time.Time) bool {
	windows := 0
	expired := 0
	for _, w := range []*QuotaWindow{payload.Session, payload.Weekly} {
		if w == nil {
			continue
		}
		windows++
		if w.ExpiredAt(now) {
			expired++
		}
	}
	return windows > 0 && windows == expired
}
