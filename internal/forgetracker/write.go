package forgetracker

import (
	"net/http"

	"ubunatic.com/harnez/internal/forge"
)

// WriteAPI selects the Forgejo API only when the separate write token is stored.
func WriteAPI(t Target, client *http.Client, lookup func(string) string) (forge.Writer, bool) {
	if t.Forge.Kind != "forgejo" {
		return forge.Writer{}, false
	}
	token := lookup(t.Forge.Host)
	if token == "" {
		return forge.Writer{}, false
	}
	return forge.Writer{Client: client, Token: token}, true
}
