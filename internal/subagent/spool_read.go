package subagent

import (
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Spool entry states. A pointer is shown as verified only when the registry at
// its store confirms it; otherwise it is a hint the host must not act on.
const (
	SpoolVerified   = "verified"   // registry at Store holds the same session_id
	SpoolStale      = "stale"      // store or entry is gone; the copied metadata is shown
	SpoolUnverified = "unverified" // unknown version, or the registry disagrees
)

// maxSpoolPointerBytes bounds one pointer read. A confined agent controls these
// files, so the host never reads an unbounded file.
const maxSpoolPointerBytes = 1 << 20

// SpoolEntry is one spool pointer as the host displays it.
type SpoolEntry struct {
	ID      string
	Store   string
	State   string
	Session *Session // metadata: the registry copy when verified, else the pointer copy; nil when unreadable
}

// ReadSpoolEntries lists pointers whose session IDs are not in known, which the
// caller fills from the registry it already read. Registry entries always win.
// The spool is read-only here: nothing in it is created, changed, or removed.
func ReadSpoolEntries(known map[string]bool) ([]SpoolEntry, error) {
	dir := SpoolDir()
	dirents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []SpoolEntry
	for _, de := range dirents {
		name := de.Name()
		if !strings.HasSuffix(name, ".json") || !de.Type().IsRegular() {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		if !safeSpoolID(id) || known[id] {
			continue
		}
		out = append(out, readSpoolEntry(filepath.Join(dir, name), id))
	}
	return out, nil
}

// readSpoolEntry classifies one pointer file named <id>.json.
func readSpoolEntry(path, fileID string) SpoolEntry {
	entry := SpoolEntry{ID: fileID, State: SpoolUnverified}
	raw, err := readBounded(path)
	if err != nil {
		return entry
	}
	var p spoolPointer
	if err := json.Unmarshal(raw, &p); err != nil {
		return entry
	}
	entry.Store = p.Store
	// An unknown format is a hint only: no metadata is shown.
	if p.V != SpoolVersion {
		return entry
	}
	copied := copiedSession(p.Entry)
	entry.Session = copied
	// A pointer must name the file it sits in. Anything else is forged or broken.
	if p.SessionID != fileID {
		return entry
	}
	registry, found, ok := readRegistryEntry(p.Store, fileID)
	switch {
	case !ok:
		// The store exists but holds a different entry or cannot be read: unverified.
		entry.Session = copied
	case !found:
		entry.State = SpoolStale
		entry.Session = copied
	default:
		entry.State = SpoolVerified
		entry.Session = registry
	}
	return entry
}

// readRegistryEntry reads <store>/<id>.json without following symlinks. found is
// false when the store or entry is absent. ok is false when the entry is present
// but disagrees with the pointer's session_id, or cannot be read safely.
func readRegistryEntry(store, id string) (sess *Session, found, ok bool) {
	if !filepath.IsAbs(store) {
		return nil, false, false
	}
	info, err := os.Lstat(store)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, true
		}
		return nil, false, false
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return nil, false, false
	}
	path := filepath.Join(store, id+".json")
	finfo, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, true
		}
		return nil, false, false
	}
	if !finfo.Mode().IsRegular() {
		return nil, false, false
	}
	raw, err := readBounded(path)
	if err != nil {
		return nil, false, false
	}
	var s Session
	if err := json.Unmarshal(raw, &s); err != nil || s.ID != id {
		return nil, false, false
	}
	return &s, true, true
}

// copiedSession decodes the entry copy in a pointer. It returns nil when the
// copy is absent or malformed.
func copiedSession(raw json.RawMessage) *Session {
	if len(raw) == 0 {
		return nil
	}
	var s Session
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil
	}
	return &s
}

// readBounded reads at most maxSpoolPointerBytes and fails when the file is larger.
func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxSpoolPointerBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSpoolPointerBytes {
		return nil, fs.ErrInvalid
	}
	return data, nil
}

// VerifiedSpoolStore returns a read-only store for the spool entry id when the
// registry at its store confirms it. ok is false when the spool holds no
// verified entry for id, including when the store is gone.
func VerifiedSpoolStore(id string) (store *FileSessionStore, ok bool, err error) {
	entries, err := ReadSpoolEntries(nil)
	if err != nil {
		return nil, false, err
	}
	for _, e := range entries {
		if e.ID == id && e.State == SpoolVerified {
			st, err := OpenSessionStore(e.Store)
			if err != nil {
				return nil, false, err
			}
			return st, true, nil
		}
	}
	return nil, false, nil
}
