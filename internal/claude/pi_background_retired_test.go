// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// piRetiredRel is a payload file a previous release recorded and the current build no longer ships.
const piRetiredRel = ".pi/extensions/retired-task.ts"

// piBackgroundLedgerWithRetired renders the ledger of a finished install plus one retired file recorded with the digest of
// retired. Like piBackgroundFinalLedger, it hashes the bytes itself and formats the lines by hand.
func piBackgroundLedgerWithRetired(t *testing.T, cfg *Config, rel string, retired []byte) []byte {
	t.Helper()
	manifest, deps, err := piBackgroundPayload(cfg)
	if err != nil {
		t.Fatal(err)
	}
	files := append(append([]piBackgroundFile{}, deps...), manifest, piBackgroundFile{rel: rel, data: retired})
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	var b strings.Builder
	b.WriteString(piLedgerTestHeader)
	for _, f := range files {
		fmt.Fprintf(&b, "file: %s %x\n", f.rel, sha256.Sum256(f.data))
	}
	return []byte(b.String())
}

// piBackgroundInstallRetired makes a finished install that also holds a retired file with content data, recorded in the ledger.
func piBackgroundInstallRetired(t *testing.T, cfg *Config, root string, data []byte) string {
	t.Helper()
	piBackgroundApply(t, cfg, nil)
	dir := piBackgroundDirPath(root)
	retired := filepath.Join(dir, filepath.FromSlash(piRetiredRel))
	writeTestFile(t, retired, data)
	writeTestFile(t, filepath.Join(dir, ".harnez-managed"), piBackgroundLedgerWithRetired(t, cfg, piRetiredRel, data))
	return retired
}

// A retired file the ledger records and nobody edited is removed by the next apply, and the ledger drops it.
func TestPiBackgroundRetiredUneditedIsRemoved(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	retired := piBackgroundInstallRetired(t, cfg, root, []byte("old\n"))
	dir := piBackgroundDirPath(root)

	if n := piBackgroundApply(t, cfg, nil); n == 0 {
		t.Error("apply changed nothing, want the unedited retired file removed")
	}
	if _, err := os.Lstat(retired); !os.IsNotExist(err) {
		t.Errorf("unedited retired file is still present, stat err = %v", err)
	}
	assertFileBytes(t, filepath.Join(dir, ".harnez-managed"), piBackgroundFinalLedger(t, cfg))
	if piBackgroundDiffChanged(t, cfg, nil) {
		t.Error("diff reports drift after the retired file was removed")
	}
	piBackgroundAssertNoBrokenPackage(t, root, cfg)
}

// A retired file someone edited is kept by apply, is no longer in the ledger, and is not drifted again on the next apply.
func TestPiBackgroundRetiredEditedBecomesUserOwned(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	retired := piBackgroundInstallRetired(t, cfg, root, []byte("old\n"))
	dir := piBackgroundDirPath(root)
	writeTestFile(t, retired, []byte("my edit\n"))

	r := &applyReport{}
	if err := applyPiBackgroundTasks(r, cfg, nil); err != nil {
		t.Fatalf("apply over an edited retired file: %v, want success that keeps the file", err)
	}
	assertFileBytes(t, retired, []byte("my edit\n"))
	assertFileBytes(t, filepath.Join(dir, ".harnez-managed"), piBackgroundFinalLedger(t, cfg))

	if n := piBackgroundApply(t, cfg, nil); n != 0 {
		t.Errorf("second apply changed %d paths, want 0: the kept file must not drift again", n)
	}
	if piBackgroundDiffChanged(t, cfg, nil) {
		t.Error("diff reports drift for a kept retired file")
	}
	if state, ok := piBackgroundStates(t, cfg, nil)[retired]; ok {
		t.Errorf("status still lists the kept retired file as %q", state)
	}
	assertFileBytes(t, retired, []byte("my edit\n"))
	piBackgroundAssertNoBrokenPackage(t, root, cfg)
}

// Revert keeps an edited retired file, because the ledger records it as changed. Its content survives untouched.
func TestPiBackgroundRevertPreservesEditedRetired(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	retired := piBackgroundInstallRetired(t, cfg, root, []byte("old\n"))
	dir := piBackgroundDirPath(root)
	writeTestFile(t, retired, []byte("my edit\n"))

	if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
		t.Fatalf("revert: %v", err)
	}
	assertFileBytes(t, retired, []byte("my edit\n"))
	if piBackgroundPresent(filepath.Join(dir, "package.json")) {
		t.Error("package.json should be removed by revert; only the edited retired file is kept")
	}
	if piBackgroundPresent(filepath.Join(dir, ".pi", "lib", "background-task-runner.ts")) {
		t.Error("an unedited dependency should be removed by revert")
	}
}

// A retired path that became a symlink is refused: apply writes nothing, keeps the link and never follows it. Revert keeps it too.
func TestPiBackgroundRetiredSymlinkIsRefused(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	piBackgroundApply(t, cfg, nil)
	dir := piBackgroundDirPath(root)
	retired := filepath.Join(dir, filepath.FromSlash(piRetiredRel))
	external := filepath.Join(t.TempDir(), "outside.ts")
	writeTestFile(t, external, []byte("outside\n"))
	writeTestFile(t, filepath.Join(dir, ".harnez-managed"), piBackgroundLedgerWithRetired(t, cfg, piRetiredRel, []byte("old\n")))
	mustSymlink(t, external, retired)

	r := &applyReport{}
	err := applyPiBackgroundTasks(r, cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "retired-task.ts") {
		t.Fatalf("apply over a symlinked retired path: err = %v, want a refusal naming it", err)
	}
	if r.changes != 0 {
		t.Errorf("refused apply changed %d paths, want 0", r.changes)
	}
	assertFileBytes(t, external, []byte("outside\n"))
	piBackgroundAssertSymlink(t, retired)
	piBackgroundAssertNoBrokenPackage(t, root, cfg)

	if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
		t.Fatalf("revert: %v", err)
	}
	assertFileBytes(t, external, []byte("outside\n"))
	piBackgroundAssertSymlink(t, retired)
}

// piBackgroundAssertSymlink fails unless path is a symbolic link itself, which Lstat reports without following it.
func piBackgroundAssertSymlink(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("%s is no longer a symlink (mode %v)", path, fi.Mode())
	}
}
