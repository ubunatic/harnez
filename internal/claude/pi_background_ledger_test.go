// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const piLedgerTestHeader = "managed: config\nextension: background-tasks\n"

// piBackgroundFinalLedger renders the ledger a finished install of cfg records. It hashes the payload bytes itself and
// formats the lines by hand, so the expectation does not depend on the production renderer.
func piBackgroundFinalLedger(t *testing.T, cfg *Config) []byte {
	t.Helper()
	manifest, deps, err := piBackgroundPayload(cfg)
	if err != nil {
		t.Fatal(err)
	}
	files := append(deps, manifest)
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	var b strings.Builder
	b.WriteString(piLedgerTestHeader)
	for _, f := range files {
		fmt.Fprintf(&b, "file: %s %x\n", f.rel, sha256.Sum256(f.data))
	}
	return []byte(b.String())
}

// piBackgroundPayloadBytes returns the bytes a finished install of cfg holds for rel.
func piBackgroundPayloadBytes(t *testing.T, cfg *Config, rel string) []byte {
	t.Helper()
	manifest, deps, err := piBackgroundPayload(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range append(deps, manifest) {
		if f.rel == rel {
			return f.data
		}
	}
	t.Fatalf("%s is not part of the payload", rel)
	return nil
}

// piBackgroundInstallV1 writes the previous release of the package: every dependency except omit holds "v1 <rel>", package.json
// holds the current manifest, and the ledger records the digests of exactly those bytes. Nothing else is written.
func piBackgroundInstallV1(t *testing.T, root string, cfg *Config, omit string) {
	t.Helper()
	dir := piBackgroundDirPath(root)
	manifest := piBackgroundPayloadBytes(t, cfg, "package.json")
	deps := []string{".pi/extensions/background-tasks.ts", ".pi/lib/background-task-runner.ts", "spec/pi-background-tasks.yaml"}
	entries := []piLedgerEntry{{rel: "package.json", installed: piDigest(manifest)}}
	writeTestFile(t, filepath.Join(dir, "package.json"), manifest)
	for _, rel := range deps {
		if rel == omit {
			continue
		}
		data := []byte("v1 " + rel + "\n")
		writeTestFile(t, filepath.Join(dir, filepath.FromSlash(rel)), data)
		entries = append(entries, piLedgerEntry{rel: rel, installed: piDigest(data)})
	}
	writeTestFile(t, filepath.Join(dir, ".harnez-managed"), renderPiLedger(entries))
}

// piBackgroundAssertNoBrokenPackage checks that Pi would discover either a complete package or none: when package.json is
// present, every dependency holds the current bytes and the real loader registers all tools.
func piBackgroundAssertNoBrokenPackage(t *testing.T, root string, cfg *Config) {
	t.Helper()
	dir := piBackgroundDirPath(root)
	if piBackgroundPresent(filepath.Join(dir, "package.json")) {
		for _, rel := range []string{".pi/extensions/background-tasks.ts", ".pi/lib/background-task-runner.ts", "spec/pi-background-tasks.yaml"} {
			assertFileBytes(t, filepath.Join(dir, filepath.FromSlash(rel)), piBackgroundPayloadBytes(t, cfg, rel))
		}
		if got := runPiLoader(t, root); got.Errors != 0 || len(got.Tools) != 4 {
			t.Fatalf("package.json is present but the loader sees %+v, want 4 tools and no errors", got)
		}
		return
	}
	if got := runPiLoader(t, root); got.Errors != 0 || len(got.Tools) != 0 {
		t.Fatalf("package.json is absent but the loader sees %+v, want nothing", got)
	}
}

func TestPiBackgroundLedgerParseRejects(t *testing.T) {
	good := strings.Repeat("a", 64)
	cases := map[string]string{
		"bare header":       piLedgerTestHeader,
		"truncated line":    piLedgerTestHeader + "file: package.json " + good,
		"two fields":        piLedgerTestHeader + "file: package.json\n",
		"wrong prefix":      piLedgerTestHeader + "fil: package.json " + good + "\n",
		"parent escape":     piLedgerTestHeader + "file: ../x " + good + "\n",
		"absolute path":     piLedgerTestHeader + "file: /etc/passwd " + good + "\n",
		"unclean path":      piLedgerTestHeader + "file: a/../b " + good + "\n",
		"marker as payload": piLedgerTestHeader + "file: .harnez-managed " + good + "\n",
		"backslash":         piLedgerTestHeader + "file: a\\b " + good + "\n",
		"duplicate path":    piLedgerTestHeader + "file: a " + good + "\nfile: a " + good + "\n",
		"uppercase digest":  piLedgerTestHeader + "file: a " + strings.Repeat("A", 64) + "\n",
		"short digest":      piLedgerTestHeader + "file: a " + strings.Repeat("a", 63) + "\n",
		"bad staged digest": piLedgerTestHeader + "file: a " + good + " xyz\n",
		"unsorted paths":    piLedgerTestHeader + "file: b " + good + "\nfile: a " + good + "\n",
		"changed header":    "managed: skills\nextension: background-tasks\nfile: a " + good + "\n",
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parsePiLedger([]byte(data)); err == nil {
				t.Errorf("parsePiLedger accepted %q", data)
			}
		})
	}

	entries := []piLedgerEntry{
		{rel: "a", installed: good},
		{rel: "b", installed: good, staged: strings.Repeat("b", 64)},
	}
	got, err := parsePiLedger(renderPiLedger(entries))
	if err != nil || !reflect.DeepEqual(got, entries) {
		t.Errorf("round trip = %+v, %v; want %+v", got, err, entries)
	}
}

// Corrupt or bare ledgers are a top-level clash: apply and revert keep every byte and change nothing.
func TestPiBackgroundCorruptLedgerIsPreserved(t *testing.T) {
	good := strings.Repeat("a", 64)
	variants := map[string]string{
		"bare":      piLedgerTestHeader,
		"truncated": piLedgerTestHeader + "file: package.json " + good,
		"malformed": piLedgerTestHeader + "file: package.json\n",
		"duplicate": piLedgerTestHeader + "file: a " + good + "\nfile: a " + good + "\n",
	}
	for name, data := range variants {
		t.Run(name, func(t *testing.T) {
			root, cfg := piBackgroundFixture(t)
			piBackgroundApply(t, cfg, nil)
			dir := piBackgroundDirPath(root)
			marker := filepath.Join(dir, ".harnez-managed")
			writeTestFile(t, marker, []byte(data))
			before := listTree(t, dir)

			if n := piBackgroundApply(t, cfg, nil); n != 0 {
				t.Errorf("apply changed %d paths under a corrupt ledger, want 0", n)
			}
			if n, err := removeManagedPiBackgroundTasks(cfg); err != nil || n != 0 {
				t.Errorf("revert = (%d, %v), want (0, nil)", n, err)
			}
			assertFileBytes(t, marker, []byte(data))
			if got := listTree(t, dir); !reflect.DeepEqual(got, before) {
				t.Errorf("tree = %v, want unchanged %v", got, before)
			}
		})
	}
}

// A ledger that is a symlink is never followed, read or replaced.
func TestPiBackgroundLedgerSymlinkIsClash(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	piBackgroundApply(t, cfg, nil)
	dir := piBackgroundDirPath(root)
	marker := filepath.Join(dir, ".harnez-managed")
	external := filepath.Join(t.TempDir(), "ledger")
	writeTestFile(t, external, piBackgroundFinalLedger(t, cfg))
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, external, marker)

	if n := piBackgroundApply(t, cfg, nil); n != 0 {
		t.Errorf("apply changed %d paths behind a symlinked ledger, want 0", n)
	}
	if n, err := removeManagedPiBackgroundTasks(cfg); err != nil || n != 0 {
		t.Errorf("revert = (%d, %v), want (0, nil)", n, err)
	}
	assertFileBytes(t, external, piBackgroundFinalLedger(t, cfg))
	if _, err := os.Lstat(marker); err != nil {
		t.Errorf("the symlinked ledger was removed: %v", err)
	}
}

// Previous release, unedited: apply upgrades every dependency and the manifest, and leaves a clean install.
func TestPiBackgroundUpgradeFromV1(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	piBackgroundInstallV1(t, root, cfg, "")
	piBackgroundApply(t, cfg, nil)
	dir := piBackgroundDirPath(root)
	for _, rel := range []string{".pi/extensions/background-tasks.ts", ".pi/lib/background-task-runner.ts", "spec/pi-background-tasks.yaml", "package.json"} {
		assertFileBytes(t, filepath.Join(dir, filepath.FromSlash(rel)), piBackgroundPayloadBytes(t, cfg, rel))
	}
	assertFileBytes(t, filepath.Join(dir, ".harnez-managed"), piBackgroundFinalLedger(t, cfg))
	if piBackgroundDiffChanged(t, cfg, nil) {
		t.Error("diff reports drift after the upgrade")
	}
	piBackgroundAssertNoBrokenPackage(t, root, cfg)
}

// Previous release, unedited, and no new apply: revert removes all of it.
func TestPiBackgroundRevertRemovesUneditedV1(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	piBackgroundInstallV1(t, root, cfg, "")
	if n, err := removeManagedPiBackgroundTasks(cfg); err != nil || n == 0 {
		t.Fatalf("revert = (%d, %v), want removals", n, err)
	}
	if _, err := os.Lstat(piBackgroundDirPath(root)); !os.IsNotExist(err) {
		t.Errorf("managed dir should be gone after reverting an unedited v1, stat err = %v", err)
	}
}

// An edited previous-release file is never overwritten by apply, never deleted by revert, and the edit survives both.
func TestPiBackgroundEditedV1IsNeverOverwritten(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	piBackgroundInstallV1(t, root, cfg, "")
	dir := piBackgroundDirPath(root)
	runner := filepath.Join(dir, ".pi", "lib", "background-task-runner.ts")
	writeTestFile(t, runner, []byte("my edit\n"))

	for i := range 2 {
		r := &applyReport{}
		err := applyPiBackgroundTasks(r, cfg, nil)
		if err == nil || !strings.Contains(err.Error(), "background-task-runner.ts") {
			t.Fatalf("apply %d over an edited v1: err = %v, want a refusal naming the runner", i+1, err)
		}
		if r.changes != 0 {
			t.Errorf("apply %d changed %d paths, want 0", i+1, r.changes)
		}
		assertFileBytes(t, runner, []byte("my edit\n"))
	}

	if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, runner, []byte("my edit\n"))
	if piBackgroundPresent(filepath.Join(dir, "package.json")) {
		t.Error("package.json should be removed by revert; only the edited runner is kept")
	}
}

// A path the previous release did not ship, now present with someone's content, is unowned: apply refuses and revert keeps it.
func TestPiBackgroundNewUnownedPathIsPreserved(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	piBackgroundInstallV1(t, root, cfg, "spec/pi-background-tasks.yaml")
	dir := piBackgroundDirPath(root)
	spec := filepath.Join(dir, "spec", "pi-background-tasks.yaml")
	writeTestFile(t, spec, []byte("user spec\n"))

	r := &applyReport{}
	if err := applyPiBackgroundTasks(r, cfg, nil); err == nil || !strings.Contains(err.Error(), "pi-background-tasks.yaml") {
		t.Fatalf("apply over an unowned spec: err = %v, want a refusal naming it", err)
	}
	if r.changes != 0 {
		t.Errorf("refused apply changed %d paths, want 0", r.changes)
	}
	assertFileBytes(t, spec, []byte("user spec\n"))

	if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, spec, []byte("user spec\n"))
}

// Status names each file of an edited previous release, by its full path: a drifted runner, outdated files, a stale ledger.
func TestPiBackgroundStatusShowsOutdatedAndDrifted(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	piBackgroundInstallV1(t, root, cfg, "")
	dir := piBackgroundDirPath(root)
	writeTestFile(t, filepath.Join(dir, ".pi", "lib", "background-task-runner.ts"), []byte("edited\n"))

	states := piBackgroundStates(t, cfg, nil)
	want := map[string]string{
		filepath.Join(dir, ".pi", "lib", "background-task-runner.ts"):  piPathDrifted,
		filepath.Join(dir, ".pi", "extensions", "background-tasks.ts"): piPathOutdated,
		filepath.Join(dir, "spec", "pi-background-tasks.yaml"):         piPathOutdated,
		filepath.Join(dir, "package.json"):                             piPathOK,
		filepath.Join(dir, ".harnez-managed"):                          piPathStale,
	}
	for path, state := range want {
		if got := states[path]; got != state {
			t.Errorf("status %s = %q, want %q", path, got, state)
		}
	}
}

// piBackgroundCrashPoints are the apply steps a crash can stop at, in order.
func piBackgroundCrashPoints() []string {
	return []string{
		"deactivated",
		"staged",
		"wrote .pi/extensions/background-tasks.ts",
		"wrote .pi/lib/background-task-runner.ts",
		"wrote spec/pi-background-tasks.yaml",
		"activated",
		"finalized",
	}
}

var errPiSimulatedCrash = errors.New("simulated crash")

// piBackgroundCrashAt runs apply and stops it at point, the way a crash would: no cleanup runs.
func piBackgroundCrashAt(t *testing.T, cfg *Config, point string) {
	t.Helper()
	piBackgroundFault = func(p string) error {
		if p == point {
			return errPiSimulatedCrash
		}
		return nil
	}
	defer func() { piBackgroundFault = nil }()
	r := &applyReport{}
	if err := applyPiBackgroundTasks(r, cfg, nil); !errors.Is(err, errPiSimulatedCrash) {
		t.Fatalf("apply stopped at %q with err = %v, want the simulated crash", point, err)
	}
}

// A crash at any apply step must leave no discoverable broken package. Retry must converge, and revert must clear the
// install, from a fresh install and from an upgrade of the previous release.
func TestPiBackgroundCrashSweep(t *testing.T) {
	scenarios := map[string]func(t *testing.T, root string, cfg *Config){
		"fresh":   func(t *testing.T, root string, cfg *Config) {},
		"upgrade": func(t *testing.T, root string, cfg *Config) { piBackgroundInstallV1(t, root, cfg, "") },
	}
	for name, setup := range scenarios {
		for _, point := range piBackgroundCrashPoints() {
			t.Run(name+" crash at "+point, func(t *testing.T) {
				root, cfg := piBackgroundFixture(t)
				setup(t, root, cfg)
				piBackgroundCrashAt(t, cfg, point)
				piBackgroundAssertNoBrokenPackage(t, root, cfg)

				piBackgroundApply(t, cfg, nil)
				if piBackgroundDiffChanged(t, cfg, nil) {
					t.Error("diff reports drift after retrying the interrupted apply")
				}
				piBackgroundAssertNoBrokenPackage(t, root, cfg)
			})
			t.Run(name+" revert after crash at "+point, func(t *testing.T) {
				root, cfg := piBackgroundFixture(t)
				setup(t, root, cfg)
				piBackgroundCrashAt(t, cfg, point)
				if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Lstat(piBackgroundDirPath(root)); !os.IsNotExist(err) {
					t.Errorf("managed dir survives revert after a crash, stat err = %v", err)
				}
				piBackgroundAssertNoBrokenPackage(t, root, cfg)
			})
		}
	}
}
