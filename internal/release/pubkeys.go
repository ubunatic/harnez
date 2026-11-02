package release

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Public key publishing (issue 1103). A project commits one standard minisign
// public key file per key and an index listing them; the release attaches every
// active key and refuses a release signed by a key that is not active.

type publicKeyEntry struct {
	ID         string `yaml:"id"`
	Maintainer string `yaml:"maintainer"`
	Status     string `yaml:"status"`
	Added      string `yaml:"added"`
	Retired    string `yaml:"retired"`
	Note       string `yaml:"note"`
}

type publicKeyIndex struct {
	Schema int              `yaml:"schema"`
	Keys   []publicKeyEntry `yaml:"keys"`
}

// publicKeySet is the validated key index of a project.
type publicKeySet struct {
	Active []releaseAsset // one asset per active key, in index order
	active map[string]bool
}

var keyIDPattern = regexp.MustCompile(`^[0-9A-F]{16}$`)

// minisignKeyID returns the key ID of a base64 minisign public key or
// signature line, as minisign prints it (uppercase hex, little-endian).
func minisignKeyID(line string, wantLen int) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(line))
	if err != nil {
		return "", fmt.Errorf("invalid base64: %w", err)
	}
	if len(raw) != wantLen {
		return "", fmt.Errorf("unexpected length %d, want %d", len(raw), wantLen)
	}
	return reversedHex(raw[2:10]), nil
}

func reversedHex(b []byte) string {
	r := make([]byte, len(b))
	for i := range b {
		r[len(b)-1-i] = b[i]
	}
	return strings.ToUpper(hex.EncodeToString(r))
}

// payloadLine returns the first line that is not a minisign comment.
func payloadLine(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "untrusted comment:") || strings.HasPrefix(line, "trusted comment:") {
			continue
		}
		return line
	}
	return ""
}

func publicKeyFileID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return minisignKeyID(payloadLine(data), 42)
}

func signatureKeyID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return minisignKeyID(payloadLine(data), 74)
}

// secretKeyID returns the key ID of an unencrypted (-W) minisign secret key.
// It returns "" for an encrypted key, whose ID is not readable without the
// password; the signature check after signing covers that case.
func secretKeyID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(payloadLine(data))
	if err != nil || len(raw) < 62 {
		return "", fmt.Errorf("%s: not a minisign secret key", path)
	}
	if raw[2] != 0 || raw[3] != 0 { // kdf algorithm set: encrypted
		return "", nil
	}
	return reversedHex(raw[54:62]), nil
}

// loadPublicKeys reads and validates the project's key index. It returns
// (nil, nil) when the project has no index.
func loadPublicKeys(dir string, cfg PublishConfig) (*publicKeySet, error) {
	keysDir := filepath.Join(dir, cfg.PublicKeysDir)
	indexPath := filepath.Join(keysDir, cfg.PublicKeysIndex)
	data, err := os.ReadFile(indexPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rel := filepath.Join(cfg.PublicKeysDir, cfg.PublicKeysIndex)
	var index publicKeyIndex
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&index); err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	if index.Schema != 1 {
		return nil, fmt.Errorf("%s: schema must be 1, got %d", rel, index.Schema)
	}
	if len(index.Keys) == 0 {
		return nil, fmt.Errorf("%s: no keys listed", rel)
	}
	set := &publicKeySet{active: map[string]bool{}}
	seen := map[string]bool{}
	for i, k := range index.Keys {
		where := fmt.Sprintf("%s: key %d", rel, i+1)
		if !keyIDPattern.MatchString(k.ID) {
			return nil, fmt.Errorf("%s: id %q is not 16 uppercase hex digits", where, k.ID)
		}
		if seen[k.ID] {
			return nil, fmt.Errorf("%s: duplicate id %s", where, k.ID)
		}
		seen[k.ID] = true
		if k.Maintainer == "" || strings.ContainsAny(k.Maintainer, " \t") {
			return nil, fmt.Errorf("%s (%s): maintainer must be one handle without spaces", where, k.ID)
		}
		if !validDate(k.Added) {
			return nil, fmt.Errorf("%s (%s): added must be YYYY-MM-DD", where, k.ID)
		}
		switch k.Status {
		case "active":
			if k.Retired != "" {
				return nil, fmt.Errorf("%s (%s): an active key has no retired date", where, k.ID)
			}
		case "retired":
			if k.Retired != "" && !validDate(k.Retired) {
				return nil, fmt.Errorf("%s (%s): retired must be YYYY-MM-DD", where, k.ID)
			}
		default:
			return nil, fmt.Errorf("%s (%s): status must be active or retired, got %q", where, k.ID, k.Status)
		}
		pubRel := filepath.Join(cfg.PublicKeysDir, k.ID+".pub")
		got, err := publicKeyFileID(filepath.Join(dir, pubRel))
		if err != nil {
			return nil, fmt.Errorf("%s (%s): %s: %w", where, k.ID, pubRel, err)
		}
		if got != k.ID {
			return nil, fmt.Errorf("%s: %s holds key %s, not %s", where, pubRel, got, k.ID)
		}
		if k.Status == "active" {
			set.active[k.ID] = true
			name := strings.ReplaceAll(cfg.PublicKeyAsset, "{id}", k.ID)
			set.Active = append(set.Active, releaseAsset{Name: name, Path: pubRel})
		}
	}
	if len(set.Active) == 0 {
		return nil, fmt.Errorf("%s: no active key", rel)
	}
	return set, nil
}

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// checkSigner refuses a signing key that is not active in the index.
func (s *publicKeySet) checkSigner(id string, cfg PublishConfig) error {
	if s == nil || id == "" || s.active[id] {
		return nil
	}
	return fmt.Errorf("signing key %s is not an active key in %s; add it or sign with an active key",
		id, filepath.Join(cfg.PublicKeysDir, cfg.PublicKeysIndex))
}

// releaseKeys validates the project's key index and, when the signing key's ID
// is readable (an unencrypted key), checks that it is active. A project without
// an index gets a warning and releases as before.
func releaseKeys(opt Options, keyPath string) (*publicKeySet, error) {
	cfg, err := loadPublishConfig()
	if err != nil {
		return nil, err
	}
	keys, err := loadPublicKeys(opt.Dir, cfg)
	if err != nil {
		return nil, err
	}
	if keys == nil {
		if keyPath == "" {
			return nil, nil // unsigned release: no key to publish
		}
		fmt.Fprintf(opt.Out, "  [keys]      Warning: no %s; the release will carry no public key\n",
			filepath.Join(cfg.PublicKeysDir, cfg.PublicKeysIndex))
		return nil, nil
	}
	if keyPath != "" {
		id, err := secretKeyID(keyPath)
		if err != nil {
			return nil, err
		}
		if err := keys.checkSigner(id, cfg); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

// signedBy returns the key ID that signed dist/SHA256SUMS ("" when unsigned)
// and refuses a key that is not active in the project's index.
func signedBy(dir string, keys *publicKeySet, cfg PublishConfig) (string, error) {
	sig := filepath.Join(dir, "dist", "SHA256SUMS.minisig")
	if !fileExists(sig) {
		return "", nil
	}
	id, err := signatureKeyID(sig)
	if err != nil {
		if keys == nil {
			return "", nil // no index to enforce: keep releasing as before
		}
		return "", fmt.Errorf("dist/SHA256SUMS.minisig: %w", err)
	}
	return id, keys.checkSigner(id, cfg)
}
