package forgetoken

import (
	"fmt"
	"time"
)

// TokenName is <prefix>-<host>-<YYYYMMDD-HHMM>: recognizable in the forge's token list and
// unique per rotation.
func TokenName(prefix, host string, now time.Time) string {
	return prefix + "-" + host + "-" + now.Format("20060102-1504")
}

// Result reports what Create or Rotate did.
type Result struct {
	Meta       Meta
	OldName    string // Rotate only: the replaced token
	OldRevoked bool   // Rotate only: false when the old token is still live on the forge
	OldErr     error
}

// Create makes a new read-only token for host and stores it. It refuses when one is already
// stored; use Rotate then.
func Create(spec Spec, st Store, c Client, host string, l Login, now time.Time) (Result, error) {
	if _, ok, err := st.Meta(host); err != nil {
		return Result{}, err
	} else if ok {
		return Result{}, fmt.Errorf("a token for %s is already stored; use `harnez forge token rotate`", host)
	}
	m, err := createAndSave(spec, st, c, host, l, now)
	return Result{Meta: m}, err
}

// Rotate creates and stores a new token, then revokes the old one by id. A failed revoke keeps
// the new token and reports the old name so it can be deleted on the forge by hand.
func Rotate(spec Spec, st Store, c Client, host string, l Login, now time.Time) (Result, error) {
	old, ok, err := st.Meta(host)
	if err != nil {
		return Result{}, err
	}
	if !ok {
		return Result{}, fmt.Errorf("no token for %s is stored; use `harnez forge token create`", host)
	}
	m, err := createAndSave(spec, st, c, host, l, now)
	if err != nil {
		return Result{}, err
	}
	res := Result{Meta: m, OldName: old.Name}
	res.OldErr = c.Delete(l, old.ID)
	res.OldRevoked = res.OldErr == nil
	return res, nil
}

// Revoke deletes the token on the forge, then locally. With localOnly it only forgets it.
func Revoke(st Store, c Client, host string, l Login, localOnly bool) (Meta, error) {
	m, ok, err := st.Meta(host)
	if err != nil {
		return Meta{}, err
	}
	if !ok {
		return Meta{}, fmt.Errorf("no token for %s is stored", host)
	}
	if !localOnly {
		if err := c.Delete(l, m.ID); err != nil {
			return m, err
		}
	}
	return m, st.Delete(host)
}

func createAndSave(spec Spec, st Store, c Client, host string, l Login, now time.Time) (Meta, error) {
	name := TokenName(spec.NamePrefix, host, now)
	tok, err := c.Create(l, name, spec.Scopes)
	if err != nil {
		return Meta{}, err
	}
	m := Meta{Host: host, User: l.User, Name: tok.Name, ID: tok.ID, Scopes: spec.Scopes, Created: now.UTC()}
	if m.Name == "" {
		m.Name = name
	}
	saved, err := st.Save(m, tok.Secret)
	if err != nil {
		// The token exists on the forge but could not be stored; try to take it back.
		if derr := c.Delete(l, tok.ID); derr != nil {
			return Meta{}, fmt.Errorf("store token: %w; token %q is still live on %s, delete it there", err, name, host)
		}
		return Meta{}, fmt.Errorf("store token: %w (token revoked again)", err)
	}
	return saved, nil
}

// SystemStore opens the default store: secret-tool keyring when installed, files otherwise.
func SystemStore(spec Spec) (Store, error) {
	dir, err := DefaultDir(spec.TokenDir)
	if err != nil {
		return Store{}, err
	}
	return Store{Dir: dir, Keyring: SystemKeyring(spec.KeyringService)}, nil
}

// Lookup returns the stored token for host, or "" when none is stored or the spec cannot load.
// Forge read paths call it before the token env vars.
func Lookup(host string) string {
	spec, err := LoadSpec()
	if err != nil {
		return ""
	}
	st, err := SystemStore(spec)
	if err != nil {
		return ""
	}
	return st.Secret(host)
}
