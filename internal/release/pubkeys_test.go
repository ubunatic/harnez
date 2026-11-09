package release

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// keyBytes returns the 8 key-ID bytes for a display ID (little-endian).
func keyBytes(t *testing.T, id string) []byte {
	t.Helper()
	b := make([]byte, 8)
	for i := 0; i < 8; i++ {
		var v byte
		for _, c := range id[14-2*i : 16-2*i] {
			v <<= 4
			if c >= 'A' {
				v |= byte(c-'A') + 10
			} else {
				v |= byte(c - '0')
			}
		}
		b[i] = v
	}
	return b
}

func fakeMinisign(t *testing.T, id string, size int) string {
	raw := make([]byte, size)
	copy(raw, "Ed")
	copy(raw[2:], keyBytes(t, id))
	return base64.StdEncoding.EncodeToString(raw)
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writePub(t *testing.T, dir, file, id string) {
	writeFile(t, filepath.Join(dir, "keys", "minisign", file),
		"untrusted comment: minisign public key "+id+"\n"+fakeMinisign(t, id, 42)+"\n")
}

const (
	activeID  = "6C9CA9E043C23FEB"
	retiredID = "0123456789ABCDEF"
)

const validIndex = `schema: 1
keys:
  - id: 6C9CA9E043C23FEB
    maintainer: flashcoder
    status: active
    added: 2026-11-02
  - id: 0123456789ABCDEF
    maintainer: other
    status: retired
    added: 2026-01-01
    retired: 2026-10-01
    note: rotated
`

func keysProject(t *testing.T, index string) string {
	dir := t.TempDir()
	writePub(t, dir, activeID+".pub", activeID)
	writePub(t, dir, retiredID+".pub", retiredID)
	writeFile(t, filepath.Join(dir, "keys", "minisign", "keys.yaml"), index)
	return dir
}

func mustCfg(t *testing.T) PublishConfig {
	t.Helper()
	cfg, err := loadPublishConfig()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestMinisignKeyIDMatchesMinisignDisplay(t *testing.T) {
	got, err := minisignKeyID(fakeMinisign(t, activeID, 42), 42)
	if err != nil || got != activeID {
		t.Fatalf("got %q, %v; want %s", got, err, activeID)
	}
}

func TestLoadPublicKeysAttachesActiveOnly(t *testing.T) {
	keys, err := loadPublicKeys(keysProject(t, validIndex), mustCfg(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys.Active) != 1 || keys.Active[0].Name != "minisign-"+activeID+".pub" ||
		keys.Active[0].Path != filepath.Join("keys", "minisign", activeID+".pub") {
		t.Fatalf("active assets = %+v", keys.Active)
	}
	if err := keys.checkSigner(retiredID, mustCfg(t)); err == nil {
		t.Fatal("retired signer accepted")
	}
	if err := keys.checkSigner(activeID, mustCfg(t)); err != nil {
		t.Fatal(err)
	}
}

func TestLoadPublicKeysWithoutIndex(t *testing.T) {
	keys, err := loadPublicKeys(t.TempDir(), mustCfg(t))
	if keys != nil || err != nil {
		t.Fatalf("got %v, %v; want nil, nil", keys, err)
	}
}

func TestLoadPublicKeysRejects(t *testing.T) {
	cases := map[string]struct {
		index string
		setup func(t *testing.T, dir string)
		want  string
	}{
		"schema":      {index: strings.Replace(validIndex, "schema: 1", "schema: 2", 1), want: "schema must be 1"},
		"duplicate":   {index: validIndex + "  - id: 6C9CA9E043C23FEB\n    maintainer: x\n    status: active\n    added: 2026-11-02\n", want: "duplicate id"},
		"status":      {index: strings.Replace(validIndex, "status: active", "status: revoked", 1), want: "status must be"},
		"maintainer":  {index: strings.Replace(validIndex, "flashcoder", "flash coder", 1), want: "one handle"},
		"date":        {index: strings.Replace(validIndex, "added: 2026-11-02", "added: 11/02/2026", 1), want: "YYYY-MM-DD"},
		"unknown key": {index: validIndex + "extra: 1\n", want: "extra"},
		"missing pub": {index: validIndex, setup: func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, "keys", "minisign", retiredID+".pub")); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}, want: retiredID + ".pub"},
		"id mismatch": {index: validIndex, setup: func(t *testing.T, dir string) {
			writePub(t, dir, retiredID+".pub", activeID)
		}, want: "holds key " + activeID},
		"no active": {index: strings.Replace(validIndex, "status: active", "status: retired", 1), want: "no active key"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := keysProject(t, tc.index)
			if tc.setup != nil {
				tc.setup(t, dir)
			}
			_, err := loadPublicKeys(dir, mustCfg(t))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSecretKeyIDUnencryptedAndEncrypted(t *testing.T) {
	raw := make([]byte, 158)
	copy(raw, "Ed")
	copy(raw[54:], keyBytes(t, activeID))
	path := filepath.Join(t.TempDir(), "k.key")
	writeFile(t, path, "untrusted comment: minisign secret key\n"+base64.StdEncoding.EncodeToString(raw)+"\n")
	if id, err := secretKeyID(path); err != nil || id != activeID {
		t.Fatalf("unencrypted: %q, %v", id, err)
	}
	copy(raw[2:], "Sc") // scrypt KDF: encrypted, ID unreadable
	writeFile(t, path, "untrusted comment: minisign secret key\n"+base64.StdEncoding.EncodeToString(raw)+"\n")
	if id, err := secretKeyID(path); err != nil || id != "" {
		t.Fatalf("encrypted: %q, %v", id, err)
	}
}

func keySignedDist(t *testing.T, dir, signer string) {
	dist := filepath.Join(dir, "dist")
	writeFile(t, filepath.Join(dist, "loom-1.0.0-x86_64-linux.tar.gz"), "artifact")
	writeFile(t, filepath.Join(dist, "SHA256SUMS"), "deadbeef  loom-1.0.0-x86_64-linux.tar.gz\n")
	writeFile(t, filepath.Join(dist, "SHA256SUMS.minisig"),
		"untrusted comment: signature\n"+fakeMinisign(t, signer, 74)+"\ntrusted comment: loom\nAAAA\n")
}

func TestPublishAttachesActiveKeys(t *testing.T) {
	dir := keysProject(t, validIndex)
	keySignedDist(t, dir, activeID)
	var out bytes.Buffer
	if err := runPublishStep(Options{Dir: dir, DryRun: true, Out: &out}, "loom", "v1.0.0", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "minisign-"+activeID+".pub") || strings.Contains(out.String(), retiredID) {
		t.Fatalf("want only the active key attached, output: %s", out.String())
	}
}

func TestPublishRefusesRetiredSigner(t *testing.T) {
	dir := keysProject(t, validIndex)
	keySignedDist(t, dir, retiredID)
	err := runPublishStep(Options{Dir: dir, DryRun: true, Out: &bytes.Buffer{}}, "loom", "v1.0.0", nil)
	if err == nil || !strings.Contains(err.Error(), "not an active key") {
		t.Fatalf("err = %v", err)
	}
}

func TestReleaseKeysWarnsWithoutIndex(t *testing.T) {
	var out bytes.Buffer
	if _, err := releaseKeys(Options{Dir: t.TempDir(), Out: &out}, "/unused.key"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Warning: no keys/minisign/keys.yaml") {
		t.Fatalf("output: %s", out.String())
	}
}

func TestPublishBodyNamesSigner(t *testing.T) {
	var created []string
	run := func(args ...string) ([]byte, error) {
		if len(args) > 3 && args[3] == "view" {
			return []byte("not found"), os.ErrNotExist
		}
		if len(args) > 1 && args[1] == "create" && args[0] == "release" {
			created = args
		}
		return nil, nil
	}
	cfg := mustCfg(t)
	body := "loom v1.0.0\n\nSHA256SUMS signed by minisign key " + activeID + "."
	if err := publishForgejoAssetsWithConfig(t.TempDir(), "v1.0.0", "loom v1.0.0", body, nil, &bytes.Buffer{}, run, cfg, nil); err != nil {
		t.Fatal(err)
	}
	if len(created) == 0 || created[len(created)-1] != body || created[len(created)-2] != "-b" {
		t.Fatalf("create args = %q, want -b %q", created, body)
	}
}

func TestPublishSkipsKeysWhenUnsigned(t *testing.T) {
	dir := keysProject(t, validIndex)
	keySignedDist(t, dir, activeID)
	if err := os.Remove(filepath.Join(dir, "dist", "SHA256SUMS.minisig")); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runPublishStep(Options{Dir: dir, DryRun: true, Out: &out}, "loom", "v1.0.0", nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "minisign-") {
		t.Fatalf("unsigned release must not attach keys, output: %s", out.String())
	}
}
