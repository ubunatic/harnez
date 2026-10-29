// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/fsutil"
)

// The Pi background-task runner is mirrored into the global Pi agent dir as one directory extension:
// <piAgentRoot>/extensions/background-tasks/. Pi loads it through package.json (pi.extensions). The TypeScript sources come
// from the embedded repository tree, so the installed code is the code the tests run; the spec comes from cfg.FS, like
// other spec-driven values. Installed under the skills component; without it, a managed install is reverted.
//
// Ownership: the directory is harnez's only while .harnez-managed is a valid ledger (pi_background_ledger.go). A file is
// harnez's only while its bytes match the digest the ledger records. Other states are a clash, which is never written or
// deleted there. A modified or unrecorded file is refused by apply and kept by revert.
//
// Apply: decide everything from the disk first. Refuse before any write when a file is modified or unrecorded. A symlink,
// special file or unexpected entry point is unsafe: apply refuses and deactivates an unmodified package.json. Otherwise:
// deactivate package.json when a dependency changes, record the old and new digests in the ledger, write the dependencies,
// remove owned leftovers, write package.json last, then commit the ledger.
//
// Revert: decide from the ledger's digests. package.json goes first, then owned files. Anything modified, unrecorded or
// a symlink stays, and while anything stays the marker and directories stay too. A changed package.json or an unexpected
// entry point keeps the whole package.
//
// Confinement: every read and write goes through an os.Root opened on the validated Pi agent dir, so the OS refuses a
// symlink that leaves that dir. Each path is also checked with Lstat before use, so a symlink is never followed inside
// the dir either. Limits: the Pi agent dir's own parents are trusted, and a same-user process that swaps paths inside the
// Pi dir during a run can still race these checks. Harnez does not claim a stronger guarantee than this.

// piBackgroundDirName is the directory under <piAgentRoot>/extensions/ that holds the mirrored package.
const piBackgroundDirName = "background-tasks"

// piBackgroundRelDir is the managed directory relative to the Pi agent dir. Every os.Root call uses this form.
const piBackgroundRelDir = "extensions/" + piBackgroundDirName

// piBackgroundMarker is the ownership ledger. It shares the skill marker name.
const piBackgroundMarker = ManagedSkillMarker

// piBackgroundManifestName is the package manifest, the activation switch.
const piBackgroundManifestName = "package.json"

// piBackgroundSourceFiles are copied byte-for-byte from the embedded repository tree.
var piBackgroundSourceFiles = []string{
	".pi/extensions/background-tasks.ts",
	".pi/lib/background-task-runner.ts",
}

// piBackgroundSpecPath is read from cfg.FS and installed next to the extension, which reads it relative to itself.
const piBackgroundSpecPath = "spec/pi-background-tasks.yaml"

// piBackgroundIndexFiles are entry points Pi could pick up by file name. Harnez never installs them, so one present in the
// managed directory is unexpected: apply refuses, and revert keeps the whole package.
var piBackgroundIndexFiles = []string{"index.ts", "index.js"}

// Path states for one expected file or ledger row, as diff and status show them.
const (
	piPathMissing  = "missing"
	piPathClash    = "clash"    // a symlink or special file where a regular file belongs
	piPathOK       = "ok"       // recorded, and the bytes this build installs
	piPathDrifted  = "drifted"  // modified, or present without a ledger record
	piPathOutdated = "outdated" // recorded and unmodified, but older than the bytes this build installs
	piPathStale    = "stale"    // the ledger differs from what this build records, or a recorded file is no longer shipped
)

// piKind is what sits at a path, found without following symlinks.
type piKind int

const (
	piKindMissing piKind = iota
	piKindDir
	piKindFile
	piKindOther // a symlink, a special file, or a path blocked by a non-directory
)

// piBackgroundOwner classifies what sits at the managed directory.
type piBackgroundOwner int

const (
	piBackgroundAbsent  piBackgroundOwner = iota // nothing at the managed path
	piBackgroundManaged                          // the ledger is a valid harnez ledger
	piBackgroundClash                            // anything else; never written or deleted
)

// piBackgroundFile is one expected file of the install, relative to the managed directory.
type piBackgroundFile struct {
	rel  string
	data []byte
}

// piBackgroundRow is the state of one managed path: ok, missing, drifted or clash, and the other states above.
type piBackgroundRow struct {
	path  string
	state string
}

// piBackgroundEntry is one entry under the managed directory, relative to it. Symlinks and special files are neither
// directories nor files here, and are never followed.
type piBackgroundEntry struct {
	rel     string
	dir     bool
	special bool
}

// piBackgroundManifest is package.json for the mirrored package. Pi reads pi.extensions for the entry point.
type piBackgroundManifest struct {
	Name    string `json:"name"`
	Private bool   `json:"private"`
	Type    string `json:"type"`
	Pi      struct {
		Extensions []string `json:"extensions"`
	} `json:"pi"`
}

// piBackgroundSite is the validated Pi agent dir and what it holds for the managed package.
type piBackgroundSite struct {
	dir       string   // absolute Pi agent dir
	root      *os.Root // confines every operation; nil when the dir is absent or refused
	owner     piBackgroundOwner
	clashPath string // the path that makes the owner a clash, for messages
	reason    string
}

// piRemnant is an entry revert leaves in place, with the reason.
type piRemnant struct {
	rel    string
	reason string
}

// piBackgroundFault is a test seam. When set, apply calls it after each step, and an error stops apply right there, the way
// a crash would: no cleanup runs, so the disk keeps exactly what the earlier steps wrote.
var piBackgroundFault func(point string) error

// piFault runs the test seam at one step boundary.
func piFault(point string) error {
	if piBackgroundFault == nil {
		return nil
	}
	return piBackgroundFault(point)
}

// managedDir is the absolute managed directory.
func (s piBackgroundSite) managedDir() string {
	return filepath.Join(s.dir, filepath.FromSlash(piBackgroundRelDir))
}

// display is the messages' form of a path relative to the managed directory.
func (s piBackgroundSite) display(rel string) string {
	return fsutil.ContractHome(filepath.Join(s.managedDir(), filepath.FromSlash(rel)))
}

// close releases the confined root. It is safe to call twice.
func (s *piBackgroundSite) close() {
	if s.root != nil {
		_ = s.root.Close()
		s.root = nil
	}
}

// entries reads the ledger from disk. With no ledger there are no records, so nothing is owned.
func (s piBackgroundSite) entries() ([]piLedgerEntry, error) {
	if s.root == nil {
		return nil, nil
	}
	kind, data, err := piReadRegular(s.root, piInManaged(piBackgroundMarker))
	if err != nil || kind != piKindFile {
		return nil, err
	}
	return parsePiLedger(data)
}

// piInManaged joins rel to the managed directory's Pi-root-relative form.
func piInManaged(rel string) string {
	return piBackgroundRelDir + "/" + rel
}

// openPiBackgroundSite validates the Pi agent dir and classifies the managed directory. A symlink or non-directory at the
// dir is a clash, and nothing is opened there. With create, a missing dir is made first; without it, a missing dir is absent.
func openPiBackgroundSite(dir string, create bool) (piBackgroundSite, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return piBackgroundSite{}, err
	}
	site := piBackgroundSite{dir: abs, clashPath: abs}
	info, err := os.Lstat(abs)
	switch {
	case errors.Is(err, syscall.ENOTDIR):
		site.owner, site.reason = piBackgroundClash, "a parent of the Pi agent dir is not a directory"
		return site, nil
	case errors.Is(err, fs.ErrNotExist) && create:
		if err := os.MkdirAll(abs, 0755); err != nil {
			return site, err
		}
		info, err = os.Lstat(abs)
	case errors.Is(err, fs.ErrNotExist):
		site.owner = piBackgroundAbsent
		return site, nil
	}
	if err != nil {
		return site, err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		site.owner, site.reason = piBackgroundClash, "Pi agent dir is a symlink"
		return site, nil
	case !info.IsDir():
		site.owner, site.reason = piBackgroundClash, "Pi agent dir is not a directory"
		return site, nil
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return site, err
	}
	site.root = root
	if err := site.classify(); err != nil {
		site.close()
		return site, err
	}
	return site, nil
}

// classify sets the owner from the Pi root. extensions and the managed directory must be real directories or missing, and
// the ledger must be a regular file that parses as a harnez ledger.
func (s *piBackgroundSite) classify() error {
	for _, rel := range []string{"extensions", piBackgroundRelDir} {
		kind, err := piKindAt(s.root, rel)
		if err != nil {
			return err
		}
		switch kind {
		case piKindMissing:
			s.owner = piBackgroundAbsent
			return nil
		case piKindDir:
		default:
			s.owner, s.clashPath, s.reason = piBackgroundClash, filepath.Join(s.dir, filepath.FromSlash(rel)), "not a real directory"
			return nil
		}
	}
	markerPath := filepath.Join(s.dir, filepath.FromSlash(piInManaged(piBackgroundMarker)))
	kind, data, err := piReadRegular(s.root, piInManaged(piBackgroundMarker))
	if err != nil {
		return err
	}
	switch kind {
	case piKindMissing:
		s.owner, s.clashPath, s.reason = piBackgroundClash, s.managedDir(), "no "+piBackgroundMarker+" marker"
		return nil
	case piKindFile:
	default:
		s.owner, s.clashPath, s.reason = piBackgroundClash, markerPath, piBackgroundMarker+" is not a regular file"
		return nil
	}
	if _, err := parsePiLedger(data); err != nil {
		s.owner, s.clashPath, s.reason = piBackgroundClash, markerPath, piBackgroundMarker+" is not a valid ledger: "+err.Error()
		return nil
	}
	s.owner = piBackgroundManaged
	return nil
}

// piKindAt reports what rel is under r. It checks each component with Lstat and stops at the first symlink or non-directory,
// so it never follows a link.
func piKindAt(r *os.Root, rel string) (piKind, error) {
	parts := strings.Split(rel, "/")
	for i := range parts {
		info, err := r.Lstat(strings.Join(parts[:i+1], "/"))
		if errors.Is(err, fs.ErrNotExist) {
			return piKindMissing, nil
		}
		if errors.Is(err, syscall.ENOTDIR) {
			return piKindOther, nil
		}
		if err != nil {
			return piKindOther, err
		}
		last := i == len(parts)-1
		switch {
		case info.Mode()&os.ModeSymlink != 0, !last && !info.IsDir():
			return piKindOther, nil
		case !last:
			continue
		case info.IsDir():
			return piKindDir, nil
		case info.Mode().IsRegular():
			return piKindFile, nil
		default:
			return piKindOther, nil
		}
	}
	return piKindMissing, nil
}

// piBackgroundManifestBytes renders package.json with a trailing newline.
func piBackgroundManifestBytes() ([]byte, error) {
	var m piBackgroundManifest
	m.Name = "harnez-pi-background-tasks"
	m.Private = true
	m.Type = "module"
	m.Pi.Extensions = []string{".pi/extensions/background-tasks.ts"}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// piBackgroundPayload returns package.json and the other expected files. package.json activates the package, so apply
// writes it last and revert removes it first.
func piBackgroundPayload(cfg *Config) (piBackgroundFile, []piBackgroundFile, error) {
	manifest, err := piBackgroundManifestBytes()
	if err != nil {
		return piBackgroundFile{}, nil, fmt.Errorf("pi background tasks manifest: %w", err)
	}
	var deps []piBackgroundFile
	for _, rel := range piBackgroundSourceFiles {
		data, err := fs.ReadFile(harnez.DefaultFS, rel)
		if err != nil {
			return piBackgroundFile{}, nil, fmt.Errorf("pi background tasks source %s: %w", rel, err)
		}
		deps = append(deps, piBackgroundFile{rel: rel, data: data})
	}
	if cfg.FS == nil {
		return piBackgroundFile{}, nil, fmt.Errorf("pi background tasks: config has no file system to read %s", piBackgroundSpecPath)
	}
	spec, err := fs.ReadFile(cfg.FS, piBackgroundSpecPath)
	if err != nil {
		return piBackgroundFile{}, nil, fmt.Errorf("pi background tasks spec: %w", err)
	}
	deps = append(deps, piBackgroundFile{rel: piBackgroundSpecPath, data: spec})
	return piBackgroundFile{rel: piBackgroundManifestName, data: manifest}, deps, nil
}

// piBackgroundTarget is the expected files in apply order: dependencies first, package.json last.
func piBackgroundTarget(manifest piBackgroundFile, deps []piBackgroundFile) []piBackgroundFile {
	return append(append([]piBackgroundFile(nil), deps...), manifest)
}

// piFinalEntries is the ledger a finished install records: every expected file at the digest this build installs.
func piFinalEntries(target []piBackgroundFile) []piLedgerEntry {
	out := make([]piLedgerEntry, 0, len(target))
	for _, f := range target {
		out = append(out, piLedgerEntry{rel: f.rel, installed: piDigest(f.data)})
	}
	return out
}

// piTargetFile is one expected file set against the disk and its ledger record.
type piTargetFile struct {
	f     piBackgroundFile
	entry *piLedgerEntry
	kind  piKind
	now   []byte // the bytes on disk when kind is a regular file
	state string
}

// piPlanTargets reads each expected file and states it: missing, clash, drifted, outdated or ok.
func piPlanTargets(r *os.Root, entries []piLedgerEntry, target []piBackgroundFile) ([]piTargetFile, error) {
	out := make([]piTargetFile, 0, len(target))
	for _, f := range target {
		t := piTargetFile{f: f, entry: piLedgerEntryFor(entries, f.rel), kind: piKindMissing}
		if r != nil {
			kind, data, err := piReadRegular(r, piInManaged(f.rel))
			if err != nil {
				return nil, fmt.Errorf("pi background task %s: %w", f.rel, err)
			}
			t.kind, t.now = kind, data
		}
		t.state = piTargetState(t)
		out = append(out, t)
	}
	return out, nil
}

// piTargetState states one expected file. Only a file whose bytes are recorded in the ledger is harnez's; of those, the
// bytes this build installs are ok and older ones are outdated.
func piTargetState(t piTargetFile) string {
	switch t.kind {
	case piKindMissing:
		return piPathMissing
	case piKindFile:
	default:
		return piPathClash
	}
	if !piOwnedBy(t.entry, t.now) {
		return piPathDrifted
	}
	if bytes.Equal(t.now, t.f.data) {
		return piPathOK
	}
	return piPathOutdated
}

// piObsolete is a ledger record for a file this build no longer ships.
type piObsolete struct {
	entry piLedgerEntry
	state string // stale: owned, apply removes it; drifted: kept; clash: unsafe; missing: dropped
}

// piPlanObsolete states every recorded file that is not in target.
func piPlanObsolete(r *os.Root, entries []piLedgerEntry, target []piBackgroundFile) ([]piObsolete, error) {
	var out []piObsolete
	for _, e := range entries {
		if piTargetHas(target, e.rel) {
			continue
		}
		kind, data, err := piReadRegular(r, piInManaged(e.rel))
		if err != nil {
			return nil, err
		}
		state := piPathDrifted
		switch {
		case kind == piKindMissing:
			state = piPathMissing
		case kind != piKindFile:
			state = piPathClash
		case piOwnedBy(&e, data):
			state = piPathStale
		}
		out = append(out, piObsolete{entry: e, state: state})
	}
	return out, nil
}

// piTargetHas reports whether target includes rel.
func piTargetHas(target []piBackgroundFile, rel string) bool {
	for _, f := range target {
		if f.rel == rel {
			return true
		}
	}
	return false
}

// piBackgroundLedgerState states the ledger row: missing, clash, ok when it is exactly the ledger a finished install records,
// or stale.
func piBackgroundLedgerState(site piBackgroundSite, target []piBackgroundFile) (string, error) {
	if site.root == nil {
		return piPathMissing, nil
	}
	kind, data, err := piReadRegular(site.root, piInManaged(piBackgroundMarker))
	if err != nil {
		return "", err
	}
	switch kind {
	case piKindMissing:
		return piPathMissing, nil
	case piKindFile:
	default:
		return piPathClash, nil
	}
	if bytes.Equal(data, renderPiLedger(piFinalEntries(target))) {
		return piPathOK, nil
	}
	return piPathStale, nil
}

// piBackgroundRows states the ledger, every expected file, then recorded leftovers and any unexpected entry point.
func piBackgroundRows(cfg *Config, site piBackgroundSite) ([]piBackgroundRow, error) {
	manifest, deps, err := piBackgroundPayload(cfg)
	if err != nil {
		return nil, err
	}
	target := piBackgroundTarget(manifest, deps)
	entries, err := site.entries()
	if err != nil {
		return nil, err
	}
	state, err := piBackgroundLedgerState(site, target)
	if err != nil {
		return nil, err
	}
	rows := []piBackgroundRow{{path: site.display(piBackgroundMarker), state: state}}
	files, err := piPlanTargets(site.root, entries, target)
	if err != nil {
		return nil, err
	}
	for _, t := range files {
		rows = append(rows, piBackgroundRow{path: site.display(t.f.rel), state: t.state})
	}
	if site.root == nil {
		return rows, nil
	}
	obsolete, err := piPlanObsolete(site.root, entries, target)
	if err != nil {
		return nil, err
	}
	for _, o := range obsolete {
		if o.state != piPathMissing {
			rows = append(rows, piBackgroundRow{path: site.display(o.entry.rel), state: o.state})
		}
	}
	for _, idx := range piBackgroundIndexFiles {
		kind, err := piKindAt(site.root, piInManaged(idx))
		if err != nil {
			return nil, err
		}
		if kind != piKindMissing {
			rows = append(rows, piBackgroundRow{path: site.display(idx), state: piPathClash})
		}
	}
	return rows, nil
}

// applyPiBackgroundTasks installs or repairs the managed directory when the skills component is selected, and reverts a
// managed one when it is not. A top-level clash (a symlink or non-directory on the Pi root path, or an unmarked directory)
// is reported and kept, and apply succeeds. Any other refusal makes apply fail; see installPiBackground.
func applyPiBackgroundTasks(r *applyReport, cfg *Config, selection Set) error {
	root := piAgentRoot(cfg)
	if root == "" {
		return nil
	}
	selected := selection.HasComponent("skills")
	site, err := openPiBackgroundSite(root, selected)
	if err != nil {
		return fmt.Errorf("pi background tasks: %w", err)
	}
	defer site.close()
	if site.owner == piBackgroundClash {
		if selected {
			fmt.Printf("  clash   %s is not a harnez-managed Pi extension (%s); kept it\n", fsutil.ContractHome(site.clashPath), site.reason)
		}
		return nil
	}
	if !selected {
		if site.owner == piBackgroundManaged {
			n, err := revertPiBackground(site, false)
			if err != nil {
				return fmt.Errorf("pi background tasks: %w", err)
			}
			r.changes += n
		}
		return nil
	}
	if err := installPiBackground(site, cfg, r); err != nil {
		return fmt.Errorf("pi background tasks: %w", err)
	}
	r.addStat("pi tasks", fsutil.ContractHome(site.managedDir()))
	return nil
}

// piApplyPlan is what apply will do, decided from the disk before anything is written.
type piApplyPlan struct {
	targets  []piTargetFile // dependencies first, then package.json
	obsolete []piObsolete
	refused  []string // display paths of modified or unrecorded files, which apply never overwrites
	unsafe   []string // display paths of clashes and unexpected entry points
}

// planPiBackgroundApply states the install against the disk. It writes nothing.
func planPiBackgroundApply(site piBackgroundSite, target []piBackgroundFile) (piApplyPlan, error) {
	var plan piApplyPlan
	entries, err := site.entries()
	if err != nil {
		return plan, err
	}
	if plan.targets, err = piPlanTargets(site.root, entries, target); err != nil {
		return plan, err
	}
	for _, t := range plan.targets {
		switch t.state {
		case piPathDrifted:
			plan.refused = append(plan.refused, site.display(t.f.rel))
		case piPathClash:
			plan.unsafe = append(plan.unsafe, site.display(t.f.rel))
		}
	}
	for _, idx := range piBackgroundIndexFiles {
		kind, err := piKindAt(site.root, piInManaged(idx))
		if err != nil {
			return plan, err
		}
		if kind != piKindMissing {
			plan.unsafe = append(plan.unsafe, site.display(idx))
		}
	}
	if plan.obsolete, err = piPlanObsolete(site.root, entries, target); err != nil {
		return plan, err
	}
	for _, o := range plan.obsolete {
		if o.state == piPathClash {
			plan.unsafe = append(plan.unsafe, site.display(o.entry.rel))
		}
	}
	return plan, nil
}

// piStagedEntries is the ledger an apply records before it writes. Each expected file keeps the digest it has now as its
// installed one, when harnez owns those bytes, and records the new digest as staged when that differs. Files this build no
// longer ships stay recorded until they are removed, so an interruption cannot forget them.
func piStagedEntries(plan piApplyPlan) []piLedgerEntry {
	var out []piLedgerEntry
	for _, t := range plan.targets {
		want := piDigest(t.f.data)
		installed := want
		switch {
		case t.kind == piKindFile && piOwnedBy(t.entry, t.now):
			installed = piDigest(t.now)
		case t.entry != nil:
			installed = t.entry.installed
		}
		e := piLedgerEntry{rel: t.f.rel, installed: installed}
		if installed != want {
			e.staged = want
		}
		out = append(out, e)
	}
	for _, o := range plan.obsolete {
		out = append(out, o.entry)
	}
	return out
}

// piRefuseUnsafe stops apply at a symlink, special file or unexpected entry point. An active package is deactivated first, so
// Pi cannot load the suspect tree, and nothing else is written.
func piRefuseUnsafe(site piBackgroundSite, paths []string) error {
	note := ""
	if _, err := deactivatePiBackground(site); err != nil {
		note = fmt.Sprintf(" (deactivating the package also failed: %v)", err)
	} else if !piManifestPresent(site) {
		note = " (package deactivated)"
	}
	return fmt.Errorf("unsafe path(s) in the managed package: %s; nothing was written%s", strings.Join(paths, ", "), note)
}

// piManifestPresent reports whether anything sits at package.json. An error counts as present, so no message claims a
// deactivation that may not have happened.
func piManifestPresent(site piBackgroundSite) bool {
	kind, err := piKindAt(site.root, piInManaged(piBackgroundManifestName))
	return err != nil || kind != piKindMissing
}

// deactivatePiBackground removes package.json, the switch Pi discovers the package by. It removes the file only while the
// ledger still owns its bytes; a changed or symlinked manifest belongs to someone else and stays. It reports whether it removed one.
func deactivatePiBackground(site piBackgroundSite) (bool, error) {
	entries, err := site.entries()
	if err != nil {
		return false, err
	}
	kind, data, err := piReadRegular(site.root, piInManaged(piBackgroundManifestName))
	if err != nil {
		return false, err
	}
	if kind != piKindFile || !piOwnedBy(piLedgerEntryFor(entries, piBackgroundManifestName), data) {
		return false, nil
	}
	if err := site.root.Remove(piInManaged(piBackgroundManifestName)); err != nil {
		return false, err
	}
	return true, nil
}

// failPiWrite deactivates the package after a failed write, so Pi cannot discover a half-written one. The message says what
// is kept and whether the package is now inactive.
func failPiWrite(site piBackgroundSite, what string, err error) error {
	if _, derr := deactivatePiBackground(site); derr != nil {
		return fmt.Errorf("%s: %w (deactivating the package also failed: %v)", what, err, derr)
	}
	if piManifestPresent(site) {
		return fmt.Errorf("%s: %w (marker and written files kept for recovery)", what, err)
	}
	return fmt.Errorf("%s: %w (package deactivated; marker and written files kept for recovery)", what, err)
}

// reportPiWrite counts and prints a write that changed a file.
func reportPiWrite(r *applyReport, site piBackgroundSite, rel string, changed bool) {
	if changed {
		r.changes++
		fmt.Printf("  wrote   %s\n", site.display(rel))
	}
}

// installPiBackground applies the install in the order that keeps a half-applied package undiscoverable. The steps are:
// refuse if anything is unsafe or unrecorded, deactivate when a dependency changes, record the staged ledger, write the
// dependencies, remove owned leftovers, write package.json last, then commit the ledger. Refusals write nothing.
func installPiBackground(site piBackgroundSite, cfg *Config, r *applyReport) error {
	if site.root == nil {
		return errors.New("no Pi agent dir to install into")
	}
	manifest, deps, err := piBackgroundPayload(cfg)
	if err != nil {
		return err
	}
	target := piBackgroundTarget(manifest, deps)
	plan, err := planPiBackgroundApply(site, target)
	if err != nil {
		return err
	}
	if len(plan.unsafe) > 0 {
		return piRefuseUnsafe(site, plan.unsafe)
	}
	if len(plan.refused) > 0 {
		return fmt.Errorf("modified or unrecorded file(s) would be overwritten: %s; nothing was written (move them aside or remove them, then apply again)", strings.Join(plan.refused, ", "))
	}

	needs := func(t piTargetFile) bool { return t.state == piPathMissing || t.state == piPathOutdated }
	depFiles, manifestFile := plan.targets[:len(plan.targets)-1], plan.targets[len(plan.targets)-1]
	depsChange := false
	for _, t := range depFiles {
		if needs(t) {
			depsChange = true
		}
	}
	var leftovers []string
	for _, o := range plan.obsolete {
		if o.state == piPathStale {
			leftovers = append(leftovers, o.entry.rel)
			depsChange = true
		}
	}
	manifestNeeds := needs(manifestFile)
	if depsChange {
		removed, err := deactivatePiBackground(site)
		if err != nil {
			return fmt.Errorf("deactivate %s: %w; nothing else was written", site.display(piBackgroundManifestName), err)
		}
		if removed {
			manifestNeeds = true
			r.changes++
			fmt.Printf("  removed %s (deactivated before the dependencies change)\n", site.display(piBackgroundManifestName))
		}
	}
	if err := piFault("deactivated"); err != nil {
		return err
	}

	// The ledger counts as one change per apply: the staged write and the commit both feed into that one count.
	ledgerChanged, err := writePiBackgroundFile(site, piBackgroundMarker, renderPiLedger(piStagedEntries(plan)))
	if err != nil {
		return failPiWrite(site, "write ledger", err)
	}
	if err := piFault("staged"); err != nil {
		return err
	}

	for _, t := range depFiles {
		if needs(t) {
			changed, err := writePiBackgroundFile(site, t.f.rel, t.f.data)
			if err != nil {
				return failPiWrite(site, "write "+site.display(t.f.rel), err)
			}
			reportPiWrite(r, site, t.f.rel, changed)
		}
		if err := piFault("wrote " + t.f.rel); err != nil {
			return err
		}
	}

	for _, rel := range leftovers {
		if err := site.root.Remove(piInManaged(rel)); err != nil {
			return failPiWrite(site, "remove "+site.display(rel), err)
		}
		r.changes++
		fmt.Printf("  removed %s\n", site.display(rel))
	}
	for _, o := range plan.obsolete {
		if o.state == piPathDrifted {
			fmt.Printf("  kept    %s (no longer shipped; changed since apply)\n", site.display(o.entry.rel))
		}
	}

	if manifestNeeds {
		changed, err := writePiBackgroundFile(site, manifest.rel, manifest.data)
		if err != nil {
			return failPiWrite(site, "write "+site.display(manifest.rel), err)
		}
		reportPiWrite(r, site, manifest.rel, changed)
	}
	if err := piFault("activated"); err != nil {
		return err
	}

	changed, err := writePiBackgroundFile(site, piBackgroundMarker, renderPiLedger(piFinalEntries(target)))
	if err != nil {
		return failPiWrite(site, "commit ledger", err)
	}
	reportPiWrite(r, site, piBackgroundMarker, ledgerChanged || changed)
	return piFault("finalized")
}

// writePiBackgroundFile replaces rel under the managed dir with data, atomically: a new temp file in the same directory is
// synced and renamed over the target. Temp names are random and created exclusively, so a leftover is never mistaken for
// one of ours by pattern. It reports whether the file changed; identical bytes are not rewritten.
func writePiBackgroundFile(site piBackgroundSite, rel string, data []byte) (bool, error) {
	full := piInManaged(rel)
	kind, got, err := piReadRegular(site.root, full)
	if err != nil {
		return false, err
	}
	if kind == piKindFile && bytes.Equal(got, data) {
		return false, nil
	}
	if kind == piKindOther {
		return false, fmt.Errorf("%s is a symlink or special file", site.display(rel))
	}
	if err := site.root.MkdirAll(path.Dir(full), 0755); err != nil {
		return false, err
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return false, err
	}
	tmp := path.Join(path.Dir(full), "."+path.Base(full)+"."+hex.EncodeToString(suffix[:])+".tmp")
	out, err := site.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return false, err
	}
	_, err = out.Write(data)
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = site.root.Rename(tmp, full)
	}
	if err != nil {
		_ = site.root.Remove(tmp)
		return false, err
	}
	return true, nil
}

// walkPiBackground lists every entry under the managed directory except the top-level ledger, with paths relative to it.
// Symlinks and special files are listed as special and never followed.
func walkPiBackground(r *os.Root) ([]piBackgroundEntry, error) {
	var out []piBackgroundEntry
	var walk func(rel string) error
	walk = func(rel string) error {
		f, err := r.Open(path.Join(piBackgroundRelDir, rel))
		if err != nil {
			return err
		}
		entries, err := f.ReadDir(-1)
		_ = f.Close()
		if err != nil {
			return err
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, e := range entries {
			child := path.Join(rel, e.Name())
			if rel == "" && e.Name() == piBackgroundMarker {
				continue
			}
			switch {
			case e.IsDir():
				out = append(out, piBackgroundEntry{rel: child, dir: true})
				if err := walk(child); err != nil {
					return err
				}
			case e.Type().IsRegular():
				out = append(out, piBackgroundEntry{rel: child})
			default:
				out = append(out, piBackgroundEntry{rel: child, special: true})
			}
		}
		return nil
	}
	err := walk("")
	return out, err
}

// piRevertPlan is what revert does, decided from the ledger and the disk before anything is removed.
type piRevertPlan struct {
	keepAll  []piRemnant // non-empty: the whole package stays, for these reasons
	remove   []string    // owned files to remove, package.json first
	remnants []piRemnant // entries that stay after the removals
	dirs     []string    // directories, deepest first
}

// planPiBackgroundRevert classifies every recorded file from its ledger digests. A changed or unrecorded package.json, a
// symlink there, or an unexpected entry point keeps the whole package: removing any part of it could leave a package Pi
// still discovers and breaks on.
func planPiBackgroundRevert(site piBackgroundSite) (piRevertPlan, error) {
	var plan piRevertPlan
	entries, err := site.entries()
	if err != nil {
		return plan, err
	}
	for _, idx := range piBackgroundIndexFiles {
		kind, err := piKindAt(site.root, piInManaged(idx))
		if err != nil {
			return plan, err
		}
		if kind != piKindMissing {
			plan.keepAll = append(plan.keepAll, piRemnant{rel: idx, reason: "unexpected entry point"})
		}
	}
	manifest := piBackgroundManifestName
	kind, data, err := piReadRegular(site.root, piInManaged(manifest))
	if err != nil {
		return plan, err
	}
	switch {
	case kind == piKindMissing:
	case kind != piKindFile:
		plan.keepAll = append(plan.keepAll, piRemnant{rel: manifest, reason: "symlink or special file"})
	case piLedgerEntryFor(entries, manifest) == nil:
		plan.keepAll = append(plan.keepAll, piRemnant{rel: manifest, reason: "not recorded by harnez"})
	case !piOwnedBy(piLedgerEntryFor(entries, manifest), data):
		plan.keepAll = append(plan.keepAll, piRemnant{rel: manifest, reason: "changed since apply"})
	default:
		plan.remove = append(plan.remove, manifest)
	}
	if len(plan.keepAll) > 0 {
		return plan, nil
	}

	reasons := map[string]string{}
	for _, e := range entries {
		if e.rel == manifest {
			continue
		}
		kind, data, err := piReadRegular(site.root, piInManaged(e.rel))
		if err != nil {
			return plan, err
		}
		switch {
		case kind == piKindMissing:
		case kind != piKindFile:
			reasons[e.rel] = "symlink or special file"
		case piOwnedBy(&e, data):
			plan.remove = append(plan.remove, e.rel)
		default:
			reasons[e.rel] = "changed since apply"
		}
	}

	removing := map[string]bool{}
	for _, rel := range plan.remove {
		removing[rel] = true
	}
	walked, err := walkPiBackground(site.root)
	if err != nil {
		return plan, err
	}
	for _, w := range walked {
		if w.dir {
			plan.dirs = append(plan.dirs, w.rel)
			continue
		}
		if removing[w.rel] {
			continue
		}
		why := reasons[w.rel]
		switch {
		case w.special:
			why = "symlink or special file"
		case why == "":
			why = "not recorded by harnez"
		}
		plan.remnants = append(plan.remnants, piRemnant{rel: w.rel, reason: why})
	}
	// Walk order is parent first; reversed, children come before their parents.
	for i, j := 0, len(plan.dirs)-1; i < j; i, j = i+1, j-1 {
		plan.dirs[i], plan.dirs[j] = plan.dirs[j], plan.dirs[i]
	}
	return plan, nil
}

// revertPiBackground removes the managed package in the order that never leaves a broken one discoverable: package.json
// first, then the owned dependencies. The ledger and the directories go only when nothing else remains. dry counts the
// changes without touching the disk, so diff and status report exactly what a revert would do.
func revertPiBackground(site piBackgroundSite, dry bool) (int, error) {
	plan, err := planPiBackgroundRevert(site)
	if err != nil {
		return 0, err
	}
	managed := fsutil.ContractHome(site.managedDir())
	if len(plan.keepAll) > 0 {
		if !dry {
			for _, k := range plan.keepAll {
				fmt.Printf("  kept    %s (%s)\n", site.display(k.rel), k.reason)
			}
			fmt.Printf("  kept    %s (whole package kept: a partial removal could break what Pi discovers)\n", managed)
		}
		return 0, nil
	}
	removed := 0
	for _, rel := range plan.remove {
		if !dry {
			if err := site.root.Remove(piInManaged(rel)); err != nil {
				return removed, fmt.Errorf("remove %s: %w", site.display(rel), err)
			}
			fmt.Printf("  removed %s\n", site.display(rel))
		}
		removed++
	}
	if len(plan.remnants) > 0 {
		if !dry {
			for _, k := range plan.remnants {
				fmt.Printf("  kept    %s (%s)\n", site.display(k.rel), k.reason)
			}
			fmt.Printf("  kept    %s (marker kept so a later apply can recover)\n", managed)
		}
		return removed, nil
	}
	if dry {
		return removed + 1, nil
	}
	for _, d := range plan.dirs {
		if err := site.root.Remove(piInManaged(d)); err != nil {
			return removed, fmt.Errorf("remove %s: %w", site.display(d), err)
		}
	}
	if err := site.root.Remove(piInManaged(piBackgroundMarker)); err != nil {
		return removed, fmt.Errorf("remove marker: %w", err)
	}
	if err := site.root.Remove(piBackgroundRelDir); err != nil {
		return removed, fmt.Errorf("remove %s: %w", managed, err)
	}
	fmt.Printf("  removed %s\n", managed)
	return removed + 1, nil
}

// removeManagedPiBackgroundTasks reverts a managed install from CleanAll. It returns how many paths it removed. A clash is kept.
func removeManagedPiBackgroundTasks(cfg *Config) (int, error) {
	root := piAgentRoot(cfg)
	if root == "" {
		return 0, nil
	}
	site, err := openPiBackgroundSite(root, false)
	if err != nil {
		return 0, err
	}
	defer site.close()
	switch site.owner {
	case piBackgroundAbsent:
		return 0, nil
	case piBackgroundClash:
		fmt.Printf("  kept    %s (not harnez-managed: %s)\n", fsutil.ContractHome(site.clashPath), site.reason)
		return 0, nil
	}
	return revertPiBackground(site, false)
}

// diffPiBackgroundTasks reports drift without mutation. It returns true when apply, or revert without the skills
// component, would change something.
func diffPiBackgroundTasks(cfg *Config, selection Set) (bool, error) {
	root := piAgentRoot(cfg)
	if root == "" {
		return false, nil
	}
	site, err := openPiBackgroundSite(root, false)
	if err != nil {
		return false, err
	}
	defer site.close()
	if !selection.HasComponent("skills") {
		if site.owner != piBackgroundManaged {
			return false, nil
		}
		n, err := revertPiBackground(site, true)
		return n > 0, err
	}
	switch site.owner {
	case piBackgroundClash:
		fmt.Printf("  clash   %s is not a harnez-managed Pi extension (%s); apply keeps it\n", fsutil.ContractHome(site.clashPath), site.reason)
		return true, nil
	case piBackgroundAbsent:
		return true, nil
	}
	rows, err := piBackgroundRows(cfg, site)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.state != piPathOK {
			return true, nil
		}
	}
	return false, nil
}

// piBackgroundStatusRow is one line of `harnez status`.
type piBackgroundStatusRow struct {
	label string
	state string
}

// piBackgroundStatus lists the install for `harnez status`: one row per managed path, a clash, or a managed install that the
// selection no longer wants ("not selected" when a revert would remove it, "kept" when only kept remnants remain).
func piBackgroundStatus(cfg *Config, selection Set) ([]piBackgroundStatusRow, error) {
	root := piAgentRoot(cfg)
	if root == "" {
		return nil, nil
	}
	site, err := openPiBackgroundSite(root, false)
	if err != nil {
		return nil, err
	}
	defer site.close()
	if site.owner == piBackgroundClash {
		return []piBackgroundStatusRow{{label: site.clashPath, state: "clash"}}, nil
	}
	if !selection.HasComponent("skills") {
		if site.owner != piBackgroundManaged {
			return nil, nil
		}
		n, err := revertPiBackground(site, true)
		if err != nil {
			return nil, err
		}
		state := "kept"
		if n > 0 {
			state = "not selected"
		}
		return []piBackgroundStatusRow{{label: site.managedDir(), state: state}}, nil
	}
	rows, err := piBackgroundRows(cfg, site)
	if err != nil {
		return nil, err
	}
	out := make([]piBackgroundStatusRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, piBackgroundStatusRow{label: row.path, state: row.state})
	}
	return out, nil
}
