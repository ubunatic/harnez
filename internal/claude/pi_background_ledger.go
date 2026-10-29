// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
)

// The ledger is the managed directory's .harnez-managed file. It records, per file harnez installed, the SHA-256 digest of the
// bytes last committed, and while an apply is interrupted, the digest of the bytes that apply is writing. A file is harnez's
// only while its bytes hash to one of those two digests. Revert and apply decide from the ledger, never from the bytes this
// build embeds, so an older install is still recognised after an upgrade.
//
// Format, one line each, with the file lines sorted by path:
//
//	managed: config
//	extension: background-tasks
//	file: <clean relative path> <64 lowercase hex installed> [<64 lowercase hex staged>]

// piBackgroundLedgerHeader opens every ledger. The header alone is a bare marker and is rejected.
const piBackgroundLedgerHeader = "managed: config\nextension: background-tasks\n"

// piLedgerEntry is one ledger record. staged is empty unless an apply recorded a new digest and has not committed it yet.
type piLedgerEntry struct {
	rel       string // clean path relative to the managed dir
	installed string // SHA-256 hex of the last committed bytes
	staged    string // SHA-256 hex of the bytes an apply is writing; empty when none
}

// renderPiLedger writes entries in the canonical form: the header, then one sorted line per entry.
func renderPiLedger(entries []piLedgerEntry) []byte {
	sorted := append([]piLedgerEntry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].rel < sorted[j].rel })
	var b strings.Builder
	b.WriteString(piBackgroundLedgerHeader)
	for _, e := range sorted {
		fmt.Fprintf(&b, "file: %s %s", e.rel, e.installed)
		if e.staged != "" {
			fmt.Fprintf(&b, " %s", e.staged)
		}
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// parsePiLedger reads a ledger strictly. Anything it cannot read exactly as renderPiLedger writes is an error, and the caller
// treats the marker as not harnez's: a bare header, a truncated or malformed line, an unsafe or duplicate path, an unsorted
// list, or a digest that is not 64 lowercase hex characters.
func parsePiLedger(data []byte) ([]piLedgerEntry, error) {
	if !bytes.HasPrefix(data, []byte(piBackgroundLedgerHeader)) {
		return nil, errors.New("the header is missing or changed")
	}
	body := string(data[len(piBackgroundLedgerHeader):])
	if body == "" {
		return nil, errors.New("bare marker: no file entries")
	}
	if !strings.HasSuffix(body, "\n") {
		return nil, errors.New("truncated: the last line has no newline")
	}
	var entries []piLedgerEntry
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
		fields := strings.Split(line, " ")
		if (len(fields) != 3 && len(fields) != 4) || fields[0] != "file:" {
			return nil, fmt.Errorf("malformed line %q", line)
		}
		e := piLedgerEntry{rel: fields[1], installed: fields[2]}
		if len(fields) == 4 {
			e.staged = fields[3]
		}
		switch {
		case !validPiLedgerRel(e.rel):
			return nil, fmt.Errorf("unsafe path %q", e.rel)
		case seen[e.rel]:
			return nil, fmt.Errorf("duplicate path %q", e.rel)
		case !isPiDigest(e.installed) || (len(fields) == 4 && !isPiDigest(e.staged)):
			return nil, fmt.Errorf("malformed digest in %q", line)
		case len(entries) > 0 && entries[len(entries)-1].rel > e.rel:
			return nil, fmt.Errorf("path %q is out of order", e.rel)
		}
		seen[e.rel] = true
		entries = append(entries, e)
	}
	return entries, nil
}

// validPiLedgerRel accepts only a clean, relative path that stays under the managed dir and is not the ledger itself.
func validPiLedgerRel(rel string) bool {
	if rel == "" || rel == piBackgroundMarker || path.IsAbs(rel) || strings.ContainsAny(rel, "\\\x00") {
		return false
	}
	return path.Clean(rel) == rel && rel != "." && rel != ".." && !strings.HasPrefix(rel, "../")
}

// isPiDigest reports whether s is a lowercase hex SHA-256 digest.
func isPiDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// piDigest is the SHA-256 hex digest of data.
func piDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// piLedgerEntryFor returns the record for rel, or nil when the ledger does not record it.
func piLedgerEntryFor(entries []piLedgerEntry, rel string) *piLedgerEntry {
	for i := range entries {
		if entries[i].rel == rel {
			return &entries[i]
		}
	}
	return nil
}

// piOwnedBy reports whether data is harnez's under e: its digest is the installed one, or the staged one of an interrupted
// apply. A missing record owns nothing.
func piOwnedBy(e *piLedgerEntry, data []byte) bool {
	if e == nil {
		return false
	}
	d := piDigest(data)
	return d == e.installed || (e.staged != "" && d == e.staged)
}

// piReadRegular reports what rel is under r and, for a regular file, its bytes. It never follows a symlink.
func piReadRegular(r *os.Root, rel string) (piKind, []byte, error) {
	kind, err := piKindAt(r, rel)
	if err != nil || kind != piKindFile {
		return kind, nil, err
	}
	data, err := r.ReadFile(rel)
	return piKindFile, data, err
}
