package forgetracker

import (
	"net/http"
	"strings"

	"ubunatic.com/harnez/internal/forge"
)

// WriteAPI selects the Forgejo API for issue writes. The stored write token wins; without
// one, the first non-empty variable in the forge's token_env (CODEBERG_TOKEN, FORGEJO_TOKEN)
// is used (issue 1247). Writer.Source names the variable, or is empty for the stored token.
func WriteAPI(t Target, client *http.Client, lookup, getenv func(string) string) (forge.Writer, bool) {
	if t.Forge.Kind != "forgejo" {
		return forge.Writer{}, false
	}
	if token := strings.TrimSpace(lookup(t.Forge.Host)); token != "" {
		return forge.Writer{Client: client, Token: token}, true
	}
	if name := WriteEnvName(t.Forge, getenv); name != "" {
		return forge.Writer{Client: client, Token: strings.TrimSpace(getenv(name)), Source: name}, true
	}
	return forge.Writer{}, false
}

// WriteEnvName returns the first token_env variable of a Forgejo forge that is set, or "".
// It never returns the value, so callers can report the source without the secret.
func WriteEnvName(f forge.Forge, getenv func(string) string) string {
	if f.Kind != "forgejo" {
		return ""
	}
	for _, name := range f.TokenEnv {
		if strings.TrimSpace(getenv(name)) != "" {
			return name
		}
	}
	return ""
}
