package usage

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
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

// AGYQuotaResponse models Connect RPC response from RetrieveUserQuotaSummary
type AGYQuotaResponse struct {
	Response struct {
		Groups []struct {
			DisplayName string `json:"displayName"`
			Description string `json:"description"`
			Buckets     []struct {
				BucketID          string  `json:"bucketId"`
				DisplayName       string  `json:"displayName"`
				Description       string  `json:"description"`
				Window            string  `json:"window"`
				RemainingFraction float64 `json:"remainingFraction"`
				ResetTime         string  `json:"resetTime"`
			} `json:"buckets"`
		} `json:"groups"`
		Description string `json:"description"`
	} `json:"response"`
}

// findAGYPorts locates the listening LanguageServer ports of running agy processes.
// A single agy process can listen on more than one port (e.g. a TLS-only port
// alongside the plain-HTTP RPC port), so all candidates are returned and the
// caller must try each until the actual RPC call succeeds.
func findAGYPorts() []int {
	procDirs, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	var ports []int
	for _, p := range procDirs {
		if !p.IsDir() {
			continue
		}
		pid := p.Name()
		if _, err := strconv.Atoi(pid); err != nil {
			continue
		}

		cmdlinePath := filepath.Join("/proc", pid, "cmdline")
		cmdBytes, err := os.ReadFile(cmdlinePath)
		if err != nil {
			continue
		}

		cmd := string(cmdBytes)
		if strings.Contains(cmd, "agy") || strings.Contains(cmd, "antigravity") {
			// Find socket inodes for this process
			fdDir := filepath.Join("/proc", pid, "fd")
			fds, err := os.ReadDir(fdDir)
			if err != nil {
				continue
			}

			var socketInodes []string
			for _, fd := range fds {
				target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
				if err == nil && strings.HasPrefix(target, "socket:[") {
					inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
					socketInodes = append(socketInodes, inode)
				}
			}

			if len(socketInodes) == 0 {
				continue
			}

			// Look up ports in /proc/net/tcp
			tcpData, err := os.ReadFile("/proc/net/tcp")
			if err != nil {
				continue
			}

			lines := strings.Split(string(tcpData), "\n")
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) > 9 {
					inode := fields[9]
					state := fields[3]
					// State 0A = TCP_LISTEN
					if state == "0A" {
						for _, sinode := range socketInodes {
							if inode == sinode {
								local := fields[1]
								parts := strings.Split(local, ":")
								if len(parts) == 2 {
									if port64, err := strconv.ParseInt(parts[1], 16, 32); err == nil {
										port := int(port64)
										if probeAGYPort(port) {
											ports = append(ports, port)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return ports
}

// probeAGYPort verifies a port is at least accepting TCP connections. It is a
// cheap pre-filter only — a port can accept TCP and still not serve the plain
// HTTP RPC (e.g. a TLS-only listener), so callers must still confirm with a
// real RetrieveUserQuotaSummary request.
func probeAGYPort(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// QueryAGYLocalQuota calls the local LanguageServer Connect RPC to fetch live model group quotas.
func QueryAGYLocalQuota(ctx context.Context, port int, client *http.Client) (*AGYQuotaResponse, error) {
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString("{}"))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var quotaResp AGYQuotaResponse
	if err := json.NewDecoder(resp.Body).Decode(&quotaResp); err != nil {
		return nil, err
	}
	return &quotaResp, nil
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

	// 3. Inspect recent log for masked account if available
	logDir := filepath.Join(geminiDir, "log")
	if entries, err := os.ReadDir(logDir); err == nil && len(entries) > 0 {
		usage.Sources = append(usage.Sources, "~/.gemini/antigravity-cli/log/")
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

	// 5. Query live quota pools if online / client provided. A process can
	// listen on more than one port (e.g. a TLS-only port alongside the plain
	// HTTP RPC port), so try each candidate until one actually answers.
	if client != nil {
		for _, port := range findAGYPorts() {
			quotaResp, err := QueryAGYLocalQuota(ctx, port, client)
			if err != nil || quotaResp == nil {
				continue
			}
			usage.Sources = append(usage.Sources, "127.0.0.1 (LanguageServer RPC)")
			now := time.Now()
			for _, g := range quotaResp.Response.Groups {
				mg := ModelGroup{
					Name:        g.DisplayName,
					Description: g.Description,
				}
				for _, b := range g.Buckets {
					remPct := b.RemainingFraction * 100.0
					usedPct := 100.0 - remPct
					if usedPct < 0 {
						usedPct = 0
					}
					if remPct < 0 {
						remPct = 0
					}

					qw := QuotaWindow{
						Name:             b.DisplayName,
						UsedPercent:      usedPct,
						RemainingPercent: remPct,
					}
					if b.ResetTime != "" {
						if t, err := time.Parse(time.RFC3339Nano, b.ResetTime); err == nil {
							qw.ResetAt = &t
							if t.After(now) {
								qw.DurationLeft = t.Sub(now)
							}
						}
					}
					mg.Windows = append(mg.Windows, qw)
				}
				usage.ModelGroups = append(usage.ModelGroups, mg)
			}
			break
		}
	}

	return usage
}
