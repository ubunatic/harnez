package forge

import (
	"os/exec"
	"strconv"
	"strings"

	"ubunatic.com/harnez/internal/forgetoken"
)

// Forge describes one supported forge host from spec/pr.yaml.
type Forge struct {
	Host     string   `yaml:"host"`
	Kind     string   `yaml:"kind"`
	APIBase  string   `yaml:"api_base"`
	TokenEnv []string `yaml:"token_env"`
}

// APIError is a failed forge API call. Transient errors (network, 429, 5xx) may be retried.
// Detail never carries a token.
type APIError struct {
	Status    int // 0 when no HTTP response was received
	Transient bool
	Detail    string
}

func (e *APIError) Error() string {
	if e.Status == 0 {
		return "forge API request failed: " + e.Detail
	}
	return "forge API request failed: HTTP " + strconv.Itoa(e.Status) + " " + e.Detail
}

// TokenSource finds an optional token for a forge: for Forgejo the token stored by
// `harnez forge token create` (issue 1118), then the environment variables named in the spec,
// then, for GitHub only, the output of `gh auth token`. Nothing is ever printed.
type TokenSource struct {
	Getenv  func(string) string
	GHToken func() (string, error)
	Stored  func(host string) string // nil: no stored tokens
}

// SystemTokenSource reads the stored token, the real environment and the gh CLI when installed.
func SystemTokenSource(getenv func(string) string) TokenSource {
	return TokenSource{Getenv: getenv, GHToken: ghAuthToken, Stored: forgetoken.Lookup}
}

// For returns the token for forge, or "" when none is configured.
func (ts TokenSource) For(fg Forge) string {
	if fg.Kind == "forgejo" && ts.Stored != nil {
		if v := strings.TrimSpace(ts.Stored(fg.Host)); v != "" {
			return v
		}
	}
	for _, name := range fg.TokenEnv {
		if v := strings.TrimSpace(ts.Getenv(name)); v != "" {
			return v
		}
	}
	if fg.Kind == "github" && ts.GHToken != nil {
		if tok, err := ts.GHToken(); err == nil {
			return strings.TrimSpace(tok)
		}
	}
	return ""
}

func ghAuthToken() (string, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		// Discard stderr on purpose: gh errors are not needed and must not be echoed.
		return "", err
	}
	return string(out), nil
}
