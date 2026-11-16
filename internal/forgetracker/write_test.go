package forgetracker

import (
	"net/http"
	"testing"

	"ubunatic.com/harnez/internal/forge"
)

func TestWriteSelection(t *testing.T) {
	client := &http.Client{}
	lookup := func(host string) string {
		if host == "codeberg.org" {
			return "stored-write"
		}
		return ""
	}
	target := Target{Forge: forge.Forge{Kind: "forgejo", Host: "codeberg.org"}}
	w, ok := WriteAPI(target, client, lookup)
	if !ok || w.Token != "stored-write" || w.Client != client {
		t.Fatalf("writer=%+v selected=%v", w, ok)
	}
	target.Forge.Host = "missing"
	if _, ok := WriteAPI(target, client, lookup); ok {
		t.Fatal("selected without token")
	}
	target.Forge.Kind = "github"
	target.Forge.Host = "codeberg.org"
	if _, ok := WriteAPI(target, client, lookup); ok {
		t.Fatal("selected github")
	}
}
