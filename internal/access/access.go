// Package access reports the directories harnez needs in each mode and whether
// the current process can reach them. It probes with access(2) and writes
// nothing, so the caller's sandbox (Landlock, bwrap, taia) decides the result.
package access

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/xdgpath"
)

// Mode names used in the manifest and in Detect results.
const (
	ModeHost     = "host"
	ModeConfined = "confined"
)

// Entry is one directory the caller must make readable or writable.
type Entry struct {
	ID      string   `yaml:"id" json:"id"`
	Root    string   `yaml:"root" json:"root"`
	Path    string   `yaml:"path" json:"path"`
	Access  string   `yaml:"access" json:"access"`
	Modes   []string `yaml:"modes" json:"modes"`
	Purpose string   `yaml:"purpose" json:"purpose"`
}

// Spec is the embedded spec/access.yaml manifest.
type Spec struct {
	Version   int     `yaml:"version" json:"version"`
	ModeProbe string  `yaml:"mode_probe" json:"mode_probe"`
	Entries   []Entry `yaml:"entries" json:"entries"`
}

// Row is one checked entry for the current mode.
type Row struct {
	ID      string
	Access  string
	Status  string
	Path    string
	Purpose string
}

var loadOnce = sync.OnceValues(func() (Spec, error) {
	data, err := harnez.DefaultFS.ReadFile("spec/access.yaml")
	if err != nil {
		return Spec{}, err
	}
	return decode(data)
})

// Load returns the decoded and validated embedded access manifest.
func Load() (Spec, error) {
	return loadOnce()
}

func decode(data []byte) (Spec, error) {
	var s Spec
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&s); err != nil {
		return s, err
	}
	return s, s.Validate()
}

// Validate checks the schema and the cross-field rules: unique ids and a mode
// probe that names an existing entry.
func (s Spec) Validate() error {
	data, err := harnez.DefaultFS.ReadFile("spec/schemas/access.schema.json")
	if err != nil {
		return err
	}
	var schema jsonschema.Schema
	if err = json.Unmarshal(data, &schema); err != nil {
		return err
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var doc any
	if err = json.Unmarshal(encoded, &doc); err != nil {
		return err
	}
	if err = resolved.Validate(doc); err != nil {
		return err
	}
	seen := make(map[string]bool, len(s.Entries))
	for _, e := range s.Entries {
		if seen[e.ID] {
			return fmt.Errorf("duplicate access entry id: %s", e.ID)
		}
		seen[e.ID] = true
	}
	if _, ok := s.entry(s.ModeProbe); !ok {
		return fmt.Errorf("mode_probe %q names no access entry", s.ModeProbe)
	}
	return nil
}

func (s Spec) entry(id string) (Entry, bool) {
	for _, e := range s.Entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// base returns the directory an entry's root refers to, and false when the root
// is unset in this environment. repo is the caller's repository root.
func base(root, repo string) (string, bool) {
	switch root {
	case "home":
		h, err := os.UserHomeDir()
		return h, err == nil && h != ""
	case "repo":
		return repo, repo != ""
	case "tmp":
		return os.TempDir(), true
	case "xdg_data_home":
		return xdgpath.DataHome(), true
	case "xdg_state_home":
		return xdgpath.StateHome(), true
	case "xdg_cache_home":
		return xdgpath.CacheHome(), true
	case "xdg_runtime_dir":
		v := os.Getenv("XDG_RUNTIME_DIR")
		return v, v != "" && filepath.IsAbs(v)
	}
	return "", false
}

// Detect returns the mode for this process and a one-line reason. The mode is
// host when the probe entry is writable, otherwise confined.
func (s Spec) Detect(repo string) (mode, reason string) {
	probe, _ := s.entry(s.ModeProbe)
	dir, ok := base(probe.Root, repo)
	if !ok {
		return ModeConfined, fmt.Sprintf("%s root unset", probe.Root)
	}
	p := filepath.Join(dir, probe.Path)
	if allowed(nearest(p), unix.W_OK) {
		return ModeHost, p + " writable"
	}
	return ModeConfined, p + " not writable"
}

// Rows checks every entry listed for mode against the current process.
func (s Spec) Rows(mode, repo string) []Row {
	var rows []Row
	for _, e := range s.Entries {
		if !contains(e.Modes, mode) {
			continue
		}
		row := Row{ID: e.ID, Access: e.Access, Purpose: e.Purpose, Path: "-", Status: "unset"}
		if dir, ok := base(e.Root, repo); ok {
			row.Path = filepath.Join(dir, e.Path)
			row.Status = statusOf(row.Path, e.Access)
		}
		rows = append(rows, row)
	}
	return rows
}

// Render prints the mode and the rows as tab-separated lines, with a header.
func Render(w io.Writer, mode, reason string, rows []Row) error {
	if _, err := fmt.Fprintf(w, "mode\t%s\t%s\n", mode, reason); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "id\taccess\tstatus\tpath\tpurpose"); err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Access, r.Status, r.Path, r.Purpose); err != nil {
			return err
		}
	}
	return nil
}

// statusOf reports whether the process can use p for the given access. An
// existing path is "ok" or "denied". A missing path is "absent" for read, and
// "create" for readwrite when its nearest existing ancestor is writable.
func statusOf(p, access string) string {
	_, err := os.Stat(p)
	switch {
	case err == nil:
		want := uint32(unix.R_OK)
		if access == "readwrite" {
			want |= unix.W_OK
		}
		if allowed(p, want) {
			return "ok"
		}
		return "denied"
	case errors.Is(err, fs.ErrNotExist):
		if access == "read" {
			return "absent"
		}
		if allowed(nearest(p), unix.W_OK) {
			return "create"
		}
		return "denied"
	default:
		return "denied"
	}
}

// nearest returns p when it exists, otherwise its nearest existing ancestor.
func nearest(p string) string {
	for {
		if _, err := os.Lstat(p); err == nil {
			return p
		}
		parent := filepath.Dir(p)
		if parent == p {
			return p
		}
		p = parent
	}
}

func allowed(p string, want uint32) bool {
	return unix.Access(p, want) == nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
