package forgetoken

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Meta describes a stored token. It holds no secret and lives at <dir>/<host>.json (0600).
type Meta struct {
	Host    string    `json:"host"`
	User    string    `json:"user"`
	Name    string    `json:"name"`
	ID      int64     `json:"id"`
	Scopes  []string  `json:"scopes"`
	Store   string    `json:"store"` // "keyring" or "file"
	Created time.Time `json:"created"`
}

// Keyring is the OS keyring, reached through secret-tool (libsecret). Run executes the tool
// with the secret on stdin and returns stdout; tests replace it.
type Keyring struct {
	Service string
	Run     func(stdin string, args ...string) (string, error)
}

// SystemKeyring returns a Keyring backed by secret-tool, or nil when it is not installed.
func SystemKeyring(service string) *Keyring {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return nil
	}
	return &Keyring{Service: service, Run: runSecretTool}
}

func runSecretTool(stdin string, args ...string) (string, error) {
	cmd := exec.Command("secret-tool", args...)
	cmd.Stdin = strings.NewReader(stdin)
	var out bytes.Buffer
	cmd.Stdout = &out
	// stderr is discarded: it never holds the secret, but it is noise for callers.
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("secret-tool %s: %w", args[0], err)
	}
	return out.String(), nil
}

func (k *Keyring) attrs(host string) []string { return []string{"service", k.Service, "host", host} }

func (k *Keyring) set(host, secret string) error {
	args := append([]string{"store", "--label", "harnez forge token " + host}, k.attrs(host)...)
	_, err := k.Run(secret, args...)
	return err
}

func (k *Keyring) get(host string) (string, error) {
	return k.Run("", append([]string{"lookup"}, k.attrs(host)...)...)
}

func (k *Keyring) clear(host string) error {
	_, err := k.Run("", append([]string{"clear"}, k.attrs(host)...)...)
	return err
}

// Store keeps tokens per forge host: the secret in the keyring when one works, else in a 0600
// file; the metadata always in a 0600 file. Dir is created 0700.
type Store struct {
	Dir     string
	Keyring *Keyring // nil: file only
}

// DefaultDir is $XDG_CONFIG_HOME/harnez/<tokenDir>, else ~/.config/harnez/<tokenDir>.
func DefaultDir(tokenDir string) (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "harnez", tokenDir), nil
}

func (s Store) metaPath(host string) string   { return filepath.Join(s.Dir, host+".json") }
func (s Store) secretPath(host string) string { return filepath.Join(s.Dir, host+".token") }

// Meta returns the metadata for host, or ok=false when no token is stored.
func (s Store) Meta(host string) (Meta, bool, error) {
	data, err := os.ReadFile(s.metaPath(host))
	if errors.Is(err, os.ErrNotExist) {
		return Meta{}, false, nil
	}
	if err != nil {
		return Meta{}, false, err
	}
	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return Meta{}, false, fmt.Errorf("parse %s: %w", s.metaPath(host), err)
	}
	return m, true, nil
}

// Save stores secret and meta. It tries the keyring first and falls back to the file; meta.Store
// records which one was used.
func (s Store) Save(m Meta, secret string) (Meta, error) {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return m, err
	}
	m.Store = "file"
	if s.Keyring != nil && s.Keyring.set(m.Host, secret) == nil {
		m.Store = "keyring"
		_ = os.Remove(s.secretPath(m.Host))
	} else if err := writePrivate(s.secretPath(m.Host), []byte(secret+"\n")); err != nil {
		return m, err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return m, err
	}
	return m, writePrivate(s.metaPath(m.Host), append(data, '\n'))
}

// Secret returns the stored token for host, or "" when none is stored or it cannot be read.
func (s Store) Secret(host string) string {
	m, ok, err := s.Meta(host)
	if err != nil || !ok {
		return ""
	}
	if m.Store == "keyring" {
		if s.Keyring == nil {
			return ""
		}
		v, err := s.Keyring.get(host)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(v)
	}
	data, err := os.ReadFile(s.secretPath(host))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Delete removes the local secret and metadata for host.
func (s Store) Delete(host string) error {
	if s.Keyring != nil {
		_ = s.Keyring.clear(host)
	}
	for _, p := range []string{s.secretPath(host), s.metaPath(host)} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// writePrivate writes data to path with mode 0600 through a temp file and rename.
func writePrivate(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // best-effort cleanup; error not actionable
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
