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
// the new token and reports the old name so it can be deleted on the forge by hand. A 2FA code
// is single-use (Forgejo rejects a replayed TOTP), so with l.OTP set, freshOTP supplies a new
// code for the revoke.
func Rotate(spec Spec, st Store, c Client, host string, l Login, now time.Time, freshOTP func() (string, error)) (Result, error) {
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
	if l.OTP != "" && freshOTP != nil {
		if l.OTP, res.OldErr = freshOTP(); res.OldErr != nil {
			return res, nil
		}
	}
	if old.ref() == "" {
		res.OldErr = fmt.Errorf("it was imported without a name")
		return res, nil
	}
	res.OldErr = c.DeleteRef(l, old.ref())
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
		if m.ref() == "" {
			return m, fmt.Errorf("the stored token has no forge name or id; delete it in the forge's settings, then run revoke --local-only")
		}
		if err := c.DeleteRef(l, m.ref()); err != nil {
			return m, err
		}
	}
	return m, st.Delete(host)
}

// ref is how the forge finds the token: its id, else its name (imported tokens have no id).
func (m Meta) ref() string {
	if m.ID > 0 {
		return fmt.Sprint(m.ID)
	}
	return m.Name
}

// Import stores a token created elsewhere (e.g. in the forge's web settings). name is its name
// on the forge, needed later by revoke. With checkRepo ("owner/repo") it first probes a read
// with the token and stores nothing when the read fails.
func Import(st Store, c Client, host, user, name, secret, checkRepo string, now time.Time) (Meta, error) {
	if secret == "" || user == "" {
		return Meta{}, fmt.Errorf("import needs a user and a token")
	}
	if _, ok, err := st.Meta(host); err != nil {
		return Meta{}, err
	} else if ok {
		return Meta{}, fmt.Errorf("a token for %s is already stored; run `harnez forge token revoke` first", host)
	}
	if checkRepo != "" {
		if err := c.CheckRead(secret, checkRepo); err != nil {
			return Meta{}, err
		}
	}
	return st.Save(Meta{Host: host, User: user, Name: name, Created: now.UTC()}, secret)
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
		// The token exists on the forge but could not be stored; try to take it back. With 2FA
		// the code was just used, so this revoke likely fails and the error names the token.
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
