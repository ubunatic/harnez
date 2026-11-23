package forge

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// TokenProbeResult is the outcome of a live API check for one token. Detail never carries a token.
type TokenProbeResult struct {
	OK     bool
	Status int
	User   string
	Detail string
}

// TokenCandidate is one configured or possible token source for a forge host.
type TokenCandidate struct {
	Role           string // "read", "write", "env", "cli"
	Source         string // e.g. "stored read token", "env CODEBERG_TOKEN"
	Meta           string // optional non-secret metadata (e.g. token name)
	Present        bool
	Unreadable     bool
	EffectiveRead  bool
	EffectiveWrite bool
	Probe          *TokenProbeResult
}

// StatusReport summarizes live token checks and effective read/write selection for one forge host.
type StatusReport struct {
	Host             string
	Kind             string
	Repo             string
	Candidates       []TokenCandidate
	EffectiveRead    string
	EffectiveReadOK  bool
	EffectiveWrite   string
	EffectiveWriteOK bool
	WriteFallbackCLI []string
}

// StoredTokenFunc looks up a stored token for host. It returns the secret (empty if missing or
// unreadable), optional non-secret metadata (e.g. token name), and whether metadata is present.
type StoredTokenFunc func(host string) (secret, meta string, present bool)

// StatusOptions configures CheckStatus.
type StatusOptions struct {
	Repo        string
	ReadStored  StoredTokenFunc
	WriteStored StoredTokenFunc
	Getenv      func(string) string
	GHToken     func() (string, error)
}

// ProbeToken performs a live read-only API check using token against fg.
// It never includes token in any returned string.
func ProbeToken(client *http.Client, fg Forge, token, ownerRepo string) TokenProbeResult {
	base := strings.TrimRight(fg.APIBase, "/")
	if fg.Kind == "forgejo" && ownerRepo != "" {
		return probeIssues(client, fg, base, token, ownerRepo)
	}
	res := probeUser(client, fg, base, token)
	if !res.OK && res.Status == http.StatusForbidden && fg.Kind == "forgejo" {
		// Scoped Forgejo tokens (read:repository, read:issue) lack read:user and get 403 on /user.
		fb := probeUserRepos(client, fg, base, token)
		if fb.OK {
			fb.Detail = "read:repository OK (no read:user scope)"
			return fb
		}
		return fb
	}
	return res
}

func probeUser(client *http.Client, fg Forge, base, token string) TokenProbeResult {
	status, body, err := doProbeGET(client, fg, base+"/user", token)
	if err != nil {
		return TokenProbeResult{Detail: err.Error()}
	}
	if status == http.StatusOK {
		var u struct {
			Login string `json:"login"`
		}
		_ = json.Unmarshal(body, &u)
		detail := "HTTP 200 OK"
		if u.Login != "" {
			detail = "authenticated as " + u.Login
		}
		return TokenProbeResult{OK: true, Status: status, User: u.Login, Detail: detail}
	}
	return probeHTTPFailure(status, "")
}

func probeUserRepos(client *http.Client, fg Forge, base, token string) TokenProbeResult {
	status, _, err := doProbeGET(client, fg, base+"/user/repos?limit=1", token)
	if err != nil {
		return TokenProbeResult{Detail: err.Error()}
	}
	if status == http.StatusOK {
		return TokenProbeResult{OK: true, Status: status, Detail: "HTTP 200 OK"}
	}
	return probeHTTPFailure(status, "")
}

func probeIssues(client *http.Client, fg Forge, base, token, ownerRepo string) TokenProbeResult {
	status, _, err := doProbeGET(client, fg, base+"/repos/"+ownerRepo+"/issues?limit=1", token)
	if err != nil {
		return TokenProbeResult{Detail: err.Error()}
	}
	if status == http.StatusOK {
		return TokenProbeResult{OK: true, Status: status, Detail: "read " + ownerRepo + " issues OK"}
	}
	return probeHTTPFailure(status, ownerRepo)
}

func probeHTTPFailure(status int, ownerRepo string) TokenProbeResult {
	switch status {
	case http.StatusUnauthorized:
		return TokenProbeResult{Status: status, Detail: "HTTP 401 unauthorized (token rejected or revoked)"}
	case http.StatusForbidden:
		if ownerRepo != "" {
			return TokenProbeResult{Status: status, Detail: "HTTP 403 forbidden (missing scope for " + ownerRepo + ")"}
		}
		return TokenProbeResult{Status: status, Detail: "HTTP 403 forbidden (missing scope)"}
	case http.StatusNotFound:
		if ownerRepo != "" {
			return TokenProbeResult{Status: status, Detail: "HTTP 404 not found (" + ownerRepo + ")"}
		}
		return TokenProbeResult{Status: status, Detail: "HTTP 404 not found"}
	default:
		return TokenProbeResult{Status: status, Detail: "HTTP " + strconv.Itoa(status)}
	}
}

func doProbeGET(client *http.Client, fg Forge, rawURL, token string) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("request build failed")
	}
	req.Header.Set("Accept", "application/json")
	if fg.Kind == "github" {
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("Authorization", "Bearer "+token)
	} else {
		req.Header.Set("Authorization", "token "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		if isTimeout(err) {
			return 0, nil, fmt.Errorf("request to %s timed out", fg.Host)
		}
		return 0, nil, fmt.Errorf("request to %s failed", fg.Host)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return resp.StatusCode, body, nil
}

// CheckStatus inspects all token sources for fg, probes each present token against the live API,
// and reports which token is effective for reads and writes.
func CheckStatus(client *http.Client, fg Forge, opt StatusOptions) StatusReport {
	r := StatusReport{
		Host:             fg.Host,
		Kind:             fg.Kind,
		Repo:             opt.Repo,
		EffectiveRead:    "none",
		EffectiveWrite:   "none",
		WriteFallbackCLI: fg.IssueCLI,
	}
	cache := map[string]TokenProbeResult{}
	probe := func(secret string) *TokenProbeResult {
		if res, ok := cache[secret]; ok {
			cp := res
			return &cp
		}
		res := ProbeToken(client, fg, secret, opt.Repo)
		cache[secret] = res
		cp := res
		return &cp
	}

	if fg.Kind == "forgejo" {
		// 1. Stored read token.
		readCand := TokenCandidate{Role: "read", Source: "stored read token"}
		if opt.ReadStored != nil {
			sec, meta, present := opt.ReadStored(fg.Host)
			readCand.Present = present
			readCand.Meta = meta
			if present && strings.TrimSpace(sec) == "" {
				readCand.Unreadable = true
			} else if strings.TrimSpace(sec) != "" {
				readCand.Present = true
				readCand.Probe = probe(strings.TrimSpace(sec))
				readCand.EffectiveRead = true
				r.EffectiveRead = readCand.Source
				r.EffectiveReadOK = readCand.Probe.OK
			}
		}
		r.Candidates = append(r.Candidates, readCand)

		// 2. Stored write token.
		writeCand := TokenCandidate{Role: "write", Source: "stored write token"}
		if opt.WriteStored != nil {
			sec, meta, present := opt.WriteStored(fg.Host)
			writeCand.Present = present
			writeCand.Meta = meta
			if present && strings.TrimSpace(sec) == "" {
				writeCand.Unreadable = true
			} else if strings.TrimSpace(sec) != "" {
				writeCand.Present = true
				writeCand.Probe = probe(strings.TrimSpace(sec))
				writeCand.EffectiveWrite = true
				r.EffectiveWrite = writeCand.Source
				r.EffectiveWriteOK = writeCand.Probe.OK
			}
		}
		r.Candidates = append(r.Candidates, writeCand)
	}

	// 3. Environment variables in spec order.
	for _, name := range fg.TokenEnv {
		cand := TokenCandidate{Role: "env", Source: "env " + name}
		var sec string
		if opt.Getenv != nil {
			sec = strings.TrimSpace(opt.Getenv(name))
		}
		if sec != "" {
			cand.Present = true
			cand.Probe = probe(sec)
			if r.EffectiveRead == "none" {
				cand.EffectiveRead = true
				r.EffectiveRead = cand.Source
				r.EffectiveReadOK = cand.Probe.OK
			}
			if r.EffectiveWrite == "none" {
				cand.EffectiveWrite = true
				r.EffectiveWrite = cand.Source
				r.EffectiveWriteOK = cand.Probe.OK
			}
		}
		r.Candidates = append(r.Candidates, cand)
	}

	// 4. GitHub CLI fallback.
	if fg.Kind == "github" {
		cand := TokenCandidate{Role: "cli", Source: "gh auth token"}
		if opt.GHToken != nil {
			if tok, err := opt.GHToken(); err == nil && strings.TrimSpace(tok) != "" {
				cand.Present = true
				cand.Probe = probe(strings.TrimSpace(tok))
				if r.EffectiveRead == "none" {
					cand.EffectiveRead = true
					r.EffectiveRead = cand.Source
					r.EffectiveReadOK = cand.Probe.OK
				}
				if r.EffectiveWrite == "none" {
					cand.EffectiveWrite = true
					r.EffectiveWrite = cand.Source
					r.EffectiveWriteOK = cand.Probe.OK
				}
			}
		}
		r.Candidates = append(r.Candidates, cand)
	}

	return r
}

// FormatStatus writes a human-readable status report without ever printing any token secret.
func FormatStatus(w io.Writer, r StatusReport) {
	header := fmt.Sprintf("forge: %s (%s)", r.Host, r.Kind)
	if r.Repo != "" {
		header += "  repo: " + r.Repo
	}
	fmt.Fprintln(w, header)
	fmt.Fprintln(w, "tokens:")
	for _, c := range r.Candidates {
		label := c.Source
		if c.Meta != "" {
			label += " (" + c.Meta + ")"
		}
		var tags []string
		if c.EffectiveRead {
			tags = append(tags, "effective:read")
		}
		if c.EffectiveWrite {
			tags = append(tags, "effective:write")
		}
		tagStr := ""
		if len(tags) > 0 {
			tagStr = " [" + strings.Join(tags, ", ") + "]"
		}
		switch {
		case c.Unreadable:
			fmt.Fprintf(w, "  %-28s MISSING (metadata present, secret unreadable)%s\n", label, tagStr)
		case !c.Present:
			fmt.Fprintf(w, "  %-28s not set\n", label)
		case c.Probe == nil:
			fmt.Fprintf(w, "  %-28s present (not probed)%s\n", label, tagStr)
		case c.Probe.OK:
			fmt.Fprintf(w, "  %-28s OK (%s)%s\n", label, c.Probe.Detail, tagStr)
		default:
			fmt.Fprintf(w, "  %-28s FAIL (%s)%s\n", label, c.Probe.Detail, tagStr)
		}
	}
	fmt.Fprintf(w, "effective read:  %s\n", formatEffective(r.EffectiveRead, r.EffectiveReadOK, nil))
	fmt.Fprintf(w, "effective write: %s\n", formatEffective(r.EffectiveWrite, r.EffectiveWriteOK, r.WriteFallbackCLI))
}

func formatEffective(source string, ok bool, fallbackCLI []string) string {
	if source == "none" {
		if len(fallbackCLI) > 0 {
			return "none (falls back to " + strings.Join(fallbackCLI, "/") + ")"
		}
		return "none"
	}
	if ok {
		return source + " (OK)"
	}
	return source + " (FAIL)"
}
