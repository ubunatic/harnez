package usage

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
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

// agyQuotaPayload is AGY's live-fetch cache payload shape (see the shared
// liveFetchCache gate in livefetchcache.go, issue 033/087). Unlike
// Claude/Codex's Session/Weekly windows, AGY reports quota as a set of named
// model groups, each with its own buckets/windows.
type agyQuotaPayload struct {
	ModelGroups []ModelGroup `json:"model_groups,omitempty"`
}

// agyUsageCmdTimeout bounds how long CollectAGY waits for `agy -p "/usage"`
// to answer. `/usage` is a local slash command — canary-verified in issue
// 104 to not itself consume model quota, even when the account's real
// model quota is exhausted — so it should return quickly, but the exec
// must never be allowed to block a harnez poll indefinitely if the agy CLI
// hangs (e.g. on an unexpected prompt or a cold start).
const agyUsageCmdTimeout = 15 * time.Second

// runAGYUsageCmdFn is runAGYUsageCmd behind a package-level variable so
// tests can stub the exec call without a real `agy` binary on PATH.
var runAGYUsageCmdFn = runAGYUsageCmd

// runAGYUsageCmd shells out to `agy -p "/usage"` and returns its raw stdout.
// This replaces the earlier live-process RPC/`/proc`-scan mechanism (issue
// 104): `/usage` is answered by a fresh, self-contained `agy` invocation,
// so it does not depend on an AGY process already happening to be running
// and listening on a port at the exact moment harnez polls.
func runAGYUsageCmd(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, agyUsageCmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "agy", "-p", "/usage")
	return cmd.Output()
}

// agyUsageFieldSplit matches runs of 2+ spaces, used as a fallback column
// splitter when `agy -p "/usage"`'s output isn't tab-delimited (e.g. it
// column-pads with spaces instead of tabs when attached to a terminal
// rather than piped, as harnez invokes it).
var agyUsageFieldSplit = regexp.MustCompile(`\s{2,}`)

// parseAGYUsageOutput parses `agy -p "/usage"`'s quota table, e.g.:
//
//	Gemini Models          Weekly Limit Remaining     1%   2026-08-31T16:27:56Z
//	Gemini Models          Five Hour Limit Remaining  91%  2026-08-30T19:33:52Z
//	Claude and GPT models  Weekly Limit Remaining     31%  2026-09-04T10:25:29Z
//
// into ModelGroups, grouping windows by their leading group-name column and
// preserving first-seen order. A line that doesn't split into the expected
// 4 columns, or whose percentage isn't a number, is skipped rather than
// failing the whole parse — this is scraping CLI text output, not a
// versioned API, so a partial reading is still more useful than none.
func parseAGYUsageOutput(out []byte) []ModelGroup {
	var order []string
	byName := make(map[string]*ModelGroup)
	now := time.Now()

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "Quota:" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			fields = agyUsageFieldSplit.Split(trimmed, -1)
		}
		if len(fields) < 4 {
			continue
		}

		groupName := strings.TrimSpace(fields[0])
		windowName := strings.TrimSpace(fields[1])
		pctStr := strings.TrimSuffix(strings.TrimSpace(fields[2]), "%")
		resetStr := strings.TrimSpace(fields[3])
		if groupName == "" || windowName == "" {
			continue
		}

		remPct, err := strconv.ParseFloat(pctStr, 64)
		if err != nil {
			continue
		}
		usedPct := 100 - remPct
		if usedPct < 0 {
			usedPct = 0
		}
		if remPct < 0 {
			remPct = 0
		}

		qw := QuotaWindow{
			Name:             windowName,
			UsedPercent:      usedPct,
			RemainingPercent: remPct,
		}
		if resetStr != "" {
			if t, err := time.Parse(time.RFC3339, resetStr); err == nil {
				qw.ResetAt = &t
				if t.After(now) {
					qw.DurationLeft = t.Sub(now)
				}
			}
		}

		g, ok := byName[groupName]
		if !ok {
			g = &ModelGroup{Name: groupName}
			byName[groupName] = g
			order = append(order, groupName)
		}
		g.Windows = append(g.Windows, qw)
	}

	groups := make([]ModelGroup, 0, len(order))
	for _, name := range order {
		groups = append(groups, *byName[name])
	}
	return groups
}

// CollectAGY inspects ~/.gemini/antigravity-cli for token, model settings, and session logs, and queries live quota pools.
func CollectAGY(ctx context.Context, geminiDir string, client *http.Client) AgentUsage {
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
		usage.Sources = append(usage.Sources, "~/.gemini/antigravity-cli/settings.json")
		var s AGYSettings
		if err := json.Unmarshal(data, &s); err == nil && s.Model != "" {
			usage.ActiveModel = s.Model
		}
	}

	// 2. Read antigravity-oauth-token
	tokenPath := filepath.Join(geminiDir, "antigravity-oauth-token")
	if data, err := os.ReadFile(tokenPath); err == nil {
		usage.Sources = append(usage.Sources, "~/.gemini/antigravity-cli/antigravity-oauth-token")
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

	// 3. Inspect recent logs for masked account and auth method if available
	logDir := filepath.Join(geminiDir, "log")
	if entries, err := os.ReadDir(logDir); err == nil && len(entries) > 0 {
		usage.Sources = append(usage.Sources, "~/.gemini/antigravity-cli/log/")
		// Read recent log files backwards looking for applyAuthResult or OAuth email
		for i := len(entries) - 1; i >= 0 && (usage.Account == "" || usage.PlanTier == ""); i-- {
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
				if idx := strings.Index(line, "email="); idx != -1 && usage.Account == "" {
					sub := line[idx+len("email="):]
					if commaIdx := strings.Index(sub, ","); commaIdx != -1 {
						email := strings.TrimSpace(sub[:commaIdx])
						if email != "" && strings.Contains(email, "@") {
							usage.Account = MaskAccount(email)
							usage.Authenticated = true
						}
					}
				}
				if idx := strings.Index(line, "authMethod="); idx != -1 && usage.PlanTier == "" {
					sub := line[idx+len("authMethod="):]
					if commaIdx := strings.Index(sub, ","); commaIdx != -1 {
						sub = sub[:commaIdx]
					}
					sub = strings.TrimSpace(sub)
					if sub != "" {
						usage.PlanTier = strings.ToUpper(sub[:1]) + sub[1:]
						usage.Authenticated = true
					}
				}
				if idx := strings.Index(line, "authenticated successfully as "); idx != -1 && usage.Account == "" {
					sub := strings.TrimSpace(line[idx+len("authenticated successfully as "):])
					if sub != "" && strings.Contains(sub, "@") {
						usage.Account = MaskAccount(sub)
						usage.Authenticated = true
					}
				}
			}
			f.Close()
		}
	}

	// 4. Count conversations/sessions
	convDir := filepath.Join(geminiDir, "conversations")
	if entries, err := os.ReadDir(convDir); err == nil {
		usage.Sources = append(usage.Sources, "~/.gemini/antigravity-cli/conversations/")
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

	// 5. Query live quota via `agy -p "/usage"`, but check the shared
	// on-disk cache first so a warm reading from a sibling `harnez`
	// process (or this process's own last tick) short-circuits the exec
	// call entirely (issue 033, generalized to AGY in issue 087).
	if client != nil {
		cachePath := liveFetchCachePath(geminiDir)
		defer lockLiveFetchInProcess(cachePath)()
		cache := readLiveFetchCache[agyQuotaPayload](cachePath)

		if cache != nil && time.Since(cache.FetchedAt) < MinWatchInterval {
			usage.ModelGroups = cache.Payload.ModelGroups
			usage.Sources = append(usage.Sources, "~/.gemini/antigravity-cli/harnez-quota-cache.json")
		} else {
			// Cache is stale or missing: fetch live. Take the advisory lock
			// first (bounded retry, never blocks indefinitely) so only one
			// process at a time writes the refreshed reading to disk; if the
			// lock can't be acquired quickly, still perform the fetch and use
			// its result in-memory for this call, just skip persisting it (a
			// sibling process is presumably writing its own fresh reading
			// right now anyway).
			lockFile, locked := lockLiveFetchCache(cachePath)
			if locked {
				defer unlockLiveFetchCache(lockFile)
			}

			out, err := runAGYUsageCmdFn(ctx)
			var groups []ModelGroup
			if err == nil {
				groups = parseAGYUsageOutput(out)
			}
			fetched := err == nil && len(groups) > 0

			if fetched {
				usage.Authenticated = true
				usage.Sources = append(usage.Sources, `agy -p "/usage"`)
				usage.ModelGroups = groups

				// Live fetch succeeded: persist it for sibling
				// processes/next tick, but only if we actually hold the
				// lock.
				if locked {
					_ = writeLiveFetchCache(cachePath, liveFetchCache[agyQuotaPayload]{
						FetchedAt: time.Now(),
						Payload:   agyQuotaPayload{ModelGroups: groups},
					})
				}
			} else {
				// The exec call failed, timed out, or came back with no
				// parseable quota lines. Never let an empty/failed result
				// overwrite the on-disk cache or blank out real data:
				// surface it as a fetch error for diagnostics, and fall
				// back to whatever is on disk regardless of its age,
				// labeled stale (issue 032's " (stale)" convention),
				// mirroring Claude's behavior. This is the same guarantee
				// the earlier RPC-based fetch gave when no AGY process was
				// listening — a failed/empty live attempt here must never
				// be the thing that makes prior quota data disappear.
				if err != nil {
					usage.QuotaFetchError = err.Error()
				} else {
					usage.QuotaFetchError = `agy -p "/usage" returned no parseable quota lines`
				}

				if cache != nil {
					staleGroups := make([]ModelGroup, len(cache.Payload.ModelGroups))
					for i, g := range cache.Payload.ModelGroups {
						ng := g
						ng.Windows = make([]QuotaWindow, len(g.Windows))
						for j, w := range g.Windows {
							w := w
							ng.Windows[j] = *staleQuotaWindow(&w)
						}
						staleGroups[i] = ng
					}
					usage.ModelGroups = staleGroups
					usage.Sources = append(usage.Sources, "~/.gemini/antigravity-cli/harnez-quota-cache.json (stale)")
				}
			}
		}
	}

	if len(usage.ModelGroups) > 0 {
		usage.Authenticated = true
	}
	if usage.Authenticated && usage.PlanTier == "" {
		usage.PlanTier = "Consumer"
	}

	return usage
}
