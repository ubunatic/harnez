// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"ubunatic.com/harnez"
)

// piBackgroundExpectedFiles are the five paths of a complete install, relative to the managed directory.
var piBackgroundExpectedFiles = []string{
	".harnez-managed",
	"package.json",
	".pi/extensions/background-tasks.ts",
	".pi/lib/background-task-runner.ts",
	"spec/pi-background-tasks.yaml",
}

// piBackgroundIsolateHome points HOME and the XDG dirs at fresh temp dirs, so no test reads or writes the real user config.
func piBackgroundIsolateHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
}

// piBackgroundFixture points the Pi root at a fresh temp dir through PI_CODING_AGENT_DIR and the config target.
func piBackgroundFixture(t *testing.T) (string, *Config) {
	t.Helper()
	root := t.TempDir()
	return root, piBackgroundFixtureAt(t, root)
}

// piBackgroundFixtureAt points the Pi root at root, which may be a symlink or a file in the clash tests.
func piBackgroundFixtureAt(t *testing.T, root string) *Config {
	t.Helper()
	piBackgroundIsolateHome(t)
	t.Setenv("PI_CODING_AGENT_DIR", root)
	cfg := loadTestConfig(t)
	cfg.PiAgentTarget = root
	return cfg
}

func piBackgroundSelection(t *testing.T, name string) Set {
	t.Helper()
	set, err := Parse(name)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// piBackgroundApply runs the installer alone and returns how many changes it made.
func piBackgroundApply(t *testing.T, cfg *Config, selection Set) int {
	t.Helper()
	r := &applyReport{}
	if err := applyPiBackgroundTasks(r, cfg, selection); err != nil {
		t.Fatalf("applyPiBackgroundTasks: %v", err)
	}
	return r.changes
}

func piBackgroundDiffChanged(t *testing.T, cfg *Config, selection Set) bool {
	t.Helper()
	changed, err := diffPiBackgroundTasks(cfg, selection)
	if err != nil {
		t.Fatalf("diffPiBackgroundTasks: %v", err)
	}
	return changed
}

// listTree returns every regular file and symlink under dir, relative and sorted; a missing dir is an empty list.
func listTree(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func piBackgroundDirPath(root string) string {
	return filepath.Join(root, "extensions", piBackgroundDirName)
}

// piBackgroundStates maps each status row's full path to its state. Full paths keep rows with the same base name apart.
func piBackgroundStates(t *testing.T, cfg *Config, selection Set) map[string]string {
	t.Helper()
	rows, err := piBackgroundStatus(cfg, selection)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, row := range rows {
		states[row.label] = row.state
	}
	return states
}

func TestPiBackgroundInstallsExactFiles(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	if n := piBackgroundApply(t, cfg, nil); n != len(piBackgroundExpectedFiles) {
		t.Errorf("first apply changes = %d, want %d", n, len(piBackgroundExpectedFiles))
	}
	dir := piBackgroundDirPath(root)
	if got := listFiles(t, dir); !reflect.DeepEqual(got, sortedCopy(piBackgroundExpectedFiles)) {
		t.Fatalf("installed files = %v, want exactly %v", got, sortedCopy(piBackgroundExpectedFiles))
	}

	// The TypeScript sources are the embedded repository bytes; the spec is the config's bytes.
	for _, rel := range piBackgroundSourceFiles {
		want, err := fs.ReadFile(harnez.DefaultFS, rel)
		if err != nil {
			t.Fatal(err)
		}
		repo, err := os.ReadFile(filepath.Join("..", "..", rel))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(repo, want) {
			t.Fatalf("embedded %s differs from the repository file; rebuild before testing the installer", rel)
		}
		assertFileBytes(t, filepath.Join(dir, rel), want)
	}
	spec, err := fs.ReadFile(cfg.FS, piBackgroundSpecPath)
	if err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, filepath.Join(dir, piBackgroundSpecPath), spec)
	assertFileBytes(t, filepath.Join(dir, ".harnez-managed"), piBackgroundFinalLedger(t, cfg))

	var manifest struct {
		Type string `json:"type"`
		Pi   struct {
			Extensions []string `json:"extensions"`
		} `json:"pi"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("package.json: %v", err)
	}
	if manifest.Type != "module" || !reflect.DeepEqual(manifest.Pi.Extensions, []string{".pi/extensions/background-tasks.ts"}) {
		t.Errorf("package.json = %s; want type module and pi.extensions to name the extension", data)
	}
}

func TestPiBackgroundSecondApplyChangesNothing(t *testing.T) {
	_, cfg := piBackgroundFixture(t)
	piBackgroundApply(t, cfg, nil)
	if n := piBackgroundApply(t, cfg, nil); n != 0 {
		t.Errorf("second apply changes = %d, want 0", n)
	}
}

func TestPiBackgroundDiffCleanAndStatusOK(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	piBackgroundApply(t, cfg, nil)
	if piBackgroundDiffChanged(t, cfg, nil) {
		t.Error("diff reports drift right after apply")
	}
	dir := piBackgroundDirPath(root)
	states := piBackgroundStates(t, cfg, nil)
	for _, rel := range piBackgroundExpectedFiles {
		if got := states[filepath.Join(dir, rel)]; got != "ok" {
			t.Errorf("status %s = %q, want ok", rel, got)
		}
	}
}

// Each payload file and the marker, corrupted or removed, must show as drift; apply repairs it while the marker stands.
func TestPiBackgroundCorruptionAndRemovalGiveDriftAndRepair(t *testing.T) {
	for _, rel := range piBackgroundExpectedFiles {
		if rel == ".harnez-managed" {
			continue
		}
		// A modified payload is not repaired in place: apply refuses it, names it, and keeps the edit. Removing the edit
		// then lets apply repair the file. This replaces the round-2 overwrite-on-drift behaviour, which could lose edits.
		t.Run("corrupt "+rel, func(t *testing.T) {
			root, cfg := piBackgroundFixture(t)
			piBackgroundApply(t, cfg, nil)
			path := filepath.Join(piBackgroundDirPath(root), rel)
			writeTestFile(t, path, []byte("tampered\n"))
			if !piBackgroundDiffChanged(t, cfg, nil) {
				t.Fatal("diff is clean after corruption")
			}
			r := &applyReport{}
			err := applyPiBackgroundTasks(r, cfg, nil)
			if err == nil || !strings.Contains(err.Error(), rel) {
				t.Fatalf("apply over a modified %s: err = %v, want a refusal naming it", rel, err)
			}
			if r.changes != 0 {
				t.Errorf("refused apply changed %d paths, want 0", r.changes)
			}
			assertFileBytes(t, path, []byte("tampered\n"))
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			piBackgroundApply(t, cfg, nil)
			if piBackgroundDiffChanged(t, cfg, nil) {
				t.Error("diff still reports drift after apply repaired it")
			}
		})
		t.Run("remove "+rel, func(t *testing.T) {
			root, cfg := piBackgroundFixture(t)
			piBackgroundApply(t, cfg, nil)
			path := filepath.Join(piBackgroundDirPath(root), rel)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if !piBackgroundDiffChanged(t, cfg, nil) {
				t.Fatal("diff is clean after removal")
			}
			piBackgroundApply(t, cfg, nil)
			if piBackgroundDiffChanged(t, cfg, nil) {
				t.Error("diff still reports drift after apply repaired it")
			}
		})
	}

	// A changed marker is no longer proof of ownership: apply must keep it and its files, and report the clash as drift.
	t.Run("corrupt marker", func(t *testing.T) {
		root, cfg := piBackgroundFixture(t)
		piBackgroundApply(t, cfg, nil)
		dir := piBackgroundDirPath(root)
		marker := filepath.Join(dir, ".harnez-managed")
		writeTestFile(t, marker, []byte("edited by hand\n"))
		before := listTree(t, dir)
		if !piBackgroundDiffChanged(t, cfg, nil) {
			t.Fatal("diff is clean after the marker was corrupted")
		}
		if n := piBackgroundApply(t, cfg, nil); n != 0 {
			t.Errorf("apply changed %d paths under a foreign marker, want 0", n)
		}
		assertFileBytes(t, marker, []byte("edited by hand\n"))
		if got := listTree(t, dir); !reflect.DeepEqual(got, before) {
			t.Errorf("files after apply = %v, want unchanged %v", got, before)
		}
	})

	// A removed marker makes the directory unmarked, so it is a collision: reported, not repaired, nothing deleted.
	t.Run("remove marker", func(t *testing.T) {
		root, cfg := piBackgroundFixture(t)
		piBackgroundApply(t, cfg, nil)
		dir := piBackgroundDirPath(root)
		before := listTree(t, dir)
		if err := os.Remove(filepath.Join(dir, ".harnez-managed")); err != nil {
			t.Fatal(err)
		}
		if !piBackgroundDiffChanged(t, cfg, nil) {
			t.Fatal("diff is clean after the marker was removed")
		}
		if n := piBackgroundApply(t, cfg, nil); n != 0 {
			t.Errorf("apply changed %d paths in an unmarked directory, want 0", n)
		}
		want := without(before, ".harnez-managed")
		if got := listTree(t, dir); !reflect.DeepEqual(got, want) {
			t.Errorf("files after apply = %v, want the same as before minus the marker: %v", got, want)
		}
	})
}

func TestPiBackgroundUnmarkedDirectoryIsNeverTouched(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	dir := piBackgroundDirPath(root)
	userFile := filepath.Join(dir, "user.ts")
	writeTestFile(t, userFile, []byte("someone else's extension\n"))

	if n := piBackgroundApply(t, cfg, nil); n != 0 {
		t.Errorf("apply changed %d paths in an unmarked collision, want 0", n)
	}
	assertFileBytes(t, userFile, []byte("someone else's extension\n"))
	if got := listTree(t, dir); !reflect.DeepEqual(got, []string{"user.ts"}) {
		t.Errorf("unmarked directory now holds %v, want only user.ts", got)
	}
	if !piBackgroundDiffChanged(t, cfg, nil) {
		t.Error("diff reports no change for an unmarked collision; it must report the clash")
	}
	if got := piBackgroundStates(t, cfg, nil); got[dir] != "clash" {
		t.Errorf("status = %v, want a clash row for the directory", got)
	}
	if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, userFile, []byte("someone else's extension\n"))
}

// Symlinks anywhere on the managed path must never let apply or revert write or delete outside it. The link itself
// is not ours either, so revert leaves it in place.
func TestPiBackgroundSymlinkEscapeWritesNothingExternal(t *testing.T) {
	cases := map[string]func(t *testing.T, cfg *Config, root, external string){
		"managed directory is a symlink": func(t *testing.T, _ *Config, root, external string) {
			mustSymlink(t, external, piBackgroundDirPath(root))
		},
		"extensions directory is a symlink": func(t *testing.T, _ *Config, root, external string) {
			mustSymlink(t, external, filepath.Join(root, "extensions"))
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			root, cfg := piBackgroundFixture(t)
			external := t.TempDir()
			writeTestFile(t, filepath.Join(external, "keep.txt"), []byte("external\n"))
			writeTestFile(t, filepath.Join(external, "pi", "keep.ts"), []byte("external\n"))
			before := listTree(t, external)

			setup(t, cfg, root, external)
			piBackgroundApply(t, cfg, nil)
			if !piBackgroundDiffChanged(t, cfg, nil) {
				t.Error("diff reports no change while a symlink is on the managed path")
			}
			if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
				t.Fatal(err)
			}
			if got := listTree(t, external); !reflect.DeepEqual(got, before) {
				t.Errorf("external tree changed to %v, want %v", got, before)
			}
			assertFileBytes(t, filepath.Join(external, "keep.txt"), []byte("external\n"))
			assertFileBytes(t, filepath.Join(external, "pi", "keep.ts"), []byte("external\n"))
		})
	}
}

// Revert removes exactly the managed files. Siblings, unexpected files in the directory, and changed payloads stay.
func TestPiBackgroundRevertRemovesOnlyManagedFiles(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	piBackgroundApply(t, cfg, nil)
	dir := piBackgroundDirPath(root)
	sibling := filepath.Join(root, "extensions", "other-extension", "index.ts")
	writeTestFile(t, sibling, []byte("another extension\n"))
	notes := filepath.Join(dir, "notes.txt")
	writeTestFile(t, notes, []byte("kept\n"))
	changedRunner := filepath.Join(dir, ".pi", "lib", "background-task-runner.ts")
	writeTestFile(t, changedRunner, []byte("edited after apply\n"))

	if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, sibling, []byte("another extension\n"))
	assertFileBytes(t, notes, []byte("kept\n"))
	assertFileBytes(t, changedRunner, []byte("edited after apply\n"))
	// Remnants stay, so the marker and the directories stay too: a later apply can recover them.
	want := []string{
		".harnez-managed", ".pi", ".pi/extensions", ".pi/lib", ".pi/lib/background-task-runner.ts", "notes.txt", "spec",
	}
	if got := listTree(t, dir); !reflect.DeepEqual(got, want) {
		t.Errorf("after revert the managed dir holds %v, want %v", got, want)
	}
	if _, err := os.Lstat(filepath.Join(dir, "package.json")); !os.IsNotExist(err) {
		t.Error("package.json is a managed file and should have been removed")
	}
	assertFileBytes(t, filepath.Join(dir, ".harnez-managed"), piBackgroundFinalLedger(t, cfg))
}

func TestPiBackgroundRevertRemovesEmptyInstall(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	piBackgroundApply(t, cfg, nil)
	if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(piBackgroundDirPath(root)); !os.IsNotExist(err) {
		t.Errorf("empty managed directory should be gone after revert, stat err = %v", err)
	}
	// Full selection: an absent install is drift, because apply would install it again.
	if !piBackgroundDiffChanged(t, cfg, nil) {
		t.Error("full selection should report the reverted install as missing")
	}
	// Without the skills component nothing is managed any more, so the diff is clean.
	if piBackgroundDiffChanged(t, cfg, piBackgroundSelection(t, "telemetry-only")) {
		t.Error("telemetry-only selection reports drift after a full revert")
	}
}

// The skills component decides installation. Full and docs-only install; telemetry-only and agents-only remove a managed install.
func TestPiBackgroundFollowsSkillsComponent(t *testing.T) {
	cases := []struct {
		selection   string
		wantPresent bool
	}{
		{"full", true},
		{"docs-only", true},
		{"telemetry-only", false},
		{"agents-only", false},
	}
	root, cfg := piBackgroundFixture(t)
	dir := piBackgroundDirPath(root)
	for _, tc := range cases {
		t.Run(tc.selection, func(t *testing.T) {
			selection := piBackgroundSelection(t, tc.selection)
			piBackgroundApply(t, cfg, selection)
			if got := piBackgroundPresent(dir); got != tc.wantPresent {
				t.Fatalf("after apply with %s: installed = %v, want %v", tc.selection, got, tc.wantPresent)
			}
			if piBackgroundDiffChanged(t, cfg, selection) {
				t.Errorf("diff reports drift after apply with %s", tc.selection)
			}
		})
	}

	// From a removed state, a telemetry-only run leaves nothing behind and reports nothing to change.
	t.Run("absent stays absent without skills", func(t *testing.T) {
		if n := piBackgroundApply(t, cfg, piBackgroundSelection(t, "agents-only")); n != 0 {
			t.Errorf("agents-only apply on an absent install changed %d paths, want 0", n)
		}
		if piBackgroundPresent(dir) {
			t.Error("agents-only apply created the managed directory")
		}
	})
}

func piBackgroundPresent(dir string) bool {
	_, err := os.Lstat(dir)
	return err == nil
}

// PI_CODING_AGENT_DIR overrides the config target; without it the config target is used.
func TestPiBackgroundHonorsPiRootOverride(t *testing.T) {
	piBackgroundIsolateHome(t)
	configRoot := t.TempDir()
	envRoot := t.TempDir()

	cfg := loadTestConfig(t)
	cfg.PiAgentTarget = configRoot
	t.Setenv("PI_CODING_AGENT_DIR", envRoot)
	piBackgroundApply(t, cfg, nil)
	if !piBackgroundPresent(piBackgroundDirPath(envRoot)) {
		t.Error("install did not land under PI_CODING_AGENT_DIR")
	}
	if piBackgroundPresent(piBackgroundDirPath(configRoot)) {
		t.Error("install wrote under the config target while PI_CODING_AGENT_DIR was set")
	}

	t.Setenv("PI_CODING_AGENT_DIR", "")
	piBackgroundApply(t, cfg, nil)
	if !piBackgroundPresent(piBackgroundDirPath(configRoot)) {
		t.Error("without PI_CODING_AGENT_DIR the install should use the config target")
	}
}

// ApplyAll, RunStatus and CleanAll drive the installer through the normal entry points.
func TestPiBackgroundIntegratesWithApplyStatusAndCleanAll(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	target := t.TempDir()
	cfg.AgentsMD.Global.Target = filepath.Join(target, "CLAUDE.md")
	cfg.AgentsMD.Global.Symlink = ""
	cfg.AgentsMD.Agents = nil

	if err := ApplyAll(target, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll: %v", err)
	}
	if !piBackgroundPresent(piBackgroundDirPath(root)) {
		t.Fatal("ApplyAll did not install the Pi background tasks")
	}
	if err := RunStatus("test.yaml", cfg, target, nil); err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if err := CleanAll(target, cfg); err != nil {
		t.Fatalf("CleanAll: %v", err)
	}
	if piBackgroundPresent(piBackgroundDirPath(root)) {
		t.Error("CleanAll left the managed Pi background directory behind")
	}
}

// The mirrored package must be discovered and register its real tools and commands through Pi's own loader. Skips
// with a reason when no Pi package or node is available; it never reads or writes the user's Pi config.
func TestPiBackgroundMirroredPackageLoadsInRealPi(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	piBackgroundApply(t, cfg, nil)
	loaded := runPiLoader(t, root)
	if loaded.Errors != 0 {
		t.Errorf("real Pi loader reported %d errors", loaded.Errors)
	}
	wantTools := []string{"background_task", "background_task_cancel", "background_task_list", "background_task_result"}
	wantCommands := []string{"task", "task-cancel", "task-result", "tasks"}
	if !reflect.DeepEqual(loaded.Tools, wantTools) {
		t.Errorf("tools = %v, want %v", loaded.Tools, wantTools)
	}
	if !reflect.DeepEqual(loaded.Commands, wantCommands) {
		t.Errorf("commands = %v, want %v", loaded.Commands, wantCommands)
	}
}

// A symlink or file where the Pi root or extensions dir belongs is a clash: apply and revert keep it and its target,
// write nothing, and succeed with no changes.
func TestPiBackgroundClashedRootPathsKeepTheirTargets(t *testing.T) {
	cases := map[string]func(t *testing.T) (root, keep string){
		"root is a symlink to a real dir": func(t *testing.T) (string, string) {
			target := t.TempDir()
			keep := filepath.Join(target, "keep.txt")
			writeTestFile(t, keep, []byte("external\n"))
			link := filepath.Join(t.TempDir(), "agent")
			mustSymlink(t, target, link)
			return link, keep
		},
		"root is a regular file": func(t *testing.T) (string, string) {
			keep := filepath.Join(t.TempDir(), "agent")
			writeTestFile(t, keep, []byte("not a directory\n"))
			return keep, keep
		},
		"root's parent is a regular file": func(t *testing.T) (string, string) {
			parent := filepath.Join(t.TempDir(), "file")
			writeTestFile(t, parent, []byte("file\n"))
			return filepath.Join(parent, "agent"), parent
		},
		"extensions is a regular file": func(t *testing.T) (string, string) {
			root := t.TempDir()
			keep := filepath.Join(root, "extensions")
			writeTestFile(t, keep, []byte("not a directory\n"))
			return root, keep
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			root, keep := setup(t)
			cfg := piBackgroundFixtureAt(t, root)
			want, err := os.ReadFile(keep)
			if err != nil {
				t.Fatal(err)
			}
			r := &applyReport{}
			if err := applyPiBackgroundTasks(r, cfg, nil); err != nil {
				t.Fatalf("apply over a clash: %v", err)
			}
			if r.changes != 0 {
				t.Errorf("apply changed %d paths over a clash, want 0", r.changes)
			}
			if !piBackgroundDiffChanged(t, cfg, nil) {
				t.Error("diff reports no change over a clash; it must report the clash")
			}
			if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
				t.Fatalf("revert over a clash: %v", err)
			}
			assertFileBytes(t, keep, want)
		})
	}
}

// A marker with any other content is not proof of ownership: apply and revert neither write nor delete under the managed dir.
func TestPiBackgroundArbitraryMarkerIsKept(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	dir := piBackgroundDirPath(root)
	writeTestFile(t, filepath.Join(dir, ".harnez-managed"), []byte("someone else's marker\n"))
	writeTestFile(t, filepath.Join(dir, "package.json"), []byte("{}\n"))
	before := listTree(t, dir)

	if n := piBackgroundApply(t, cfg, nil); n != 0 {
		t.Errorf("apply changed %d paths under a foreign marker, want 0", n)
	}
	if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
		t.Fatal(err)
	}
	if got := listTree(t, dir); !reflect.DeepEqual(got, before) {
		t.Errorf("tree changed to %v, want %v", got, before)
	}
	assertFileBytes(t, filepath.Join(dir, ".harnez-managed"), []byte("someone else's marker\n"))
	assertFileBytes(t, filepath.Join(dir, "package.json"), []byte("{}\n"))
}

// Once package.json is edited, the package is no longer harnez's to take apart: revert keeps every file and the marker.
func TestPiBackgroundEditedManifestKeepsEverything(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	piBackgroundApply(t, cfg, nil)
	dir := piBackgroundDirPath(root)
	writeTestFile(t, filepath.Join(dir, "package.json"), []byte("{\"name\": \"mine\"}\n"))
	writeTestFile(t, filepath.Join(dir, ".pi", "extensions", "background-tasks.ts"), []byte("edited entry\n"))
	before := listTree(t, dir)

	if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
		t.Fatal(err)
	}
	if got := listTree(t, dir); !reflect.DeepEqual(got, before) {
		t.Errorf("revert changed the tree to %v, want all of %v kept", got, before)
	}
	assertFileBytes(t, filepath.Join(dir, "package.json"), []byte("{\"name\": \"mine\"}\n"))
	assertFileBytes(t, filepath.Join(dir, ".pi", "extensions", "background-tasks.ts"), []byte("edited entry\n"))
	assertFileBytes(t, filepath.Join(dir, ".harnez-managed"), piBackgroundFinalLedger(t, cfg))
}

// Revert deactivates first: package.json goes before anything else, so a half-removed package is never discovered. Changed
// and unknown files stay with the marker, a second revert changes nothing, and the next apply repairs what it owns.
func TestPiBackgroundRevertDeactivatesAndApplyRecovers(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	piBackgroundApply(t, cfg, nil)
	dir := piBackgroundDirPath(root)
	lib := filepath.Join(dir, ".pi", "lib", "background-task-runner.ts")
	notes := filepath.Join(dir, "notes.txt")
	writeTestFile(t, lib, []byte("edited lib\n"))
	writeTestFile(t, notes, []byte("my notes\n"))

	if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
		t.Fatal(err)
	}
	if piBackgroundPresent(filepath.Join(dir, "package.json")) {
		t.Error("package.json is still present after revert; it must be deactivated first")
	}
	for _, rel := range []string{".pi/extensions/background-tasks.ts", "spec/pi-background-tasks.yaml"} {
		if piBackgroundPresent(filepath.Join(dir, filepath.FromSlash(rel))) {
			t.Errorf("unchanged owned file %s survived revert", rel)
		}
	}
	assertFileBytes(t, lib, []byte("edited lib\n"))
	assertFileBytes(t, notes, []byte("my notes\n"))
	assertFileBytes(t, filepath.Join(dir, ".harnez-managed"), piBackgroundFinalLedger(t, cfg))

	before := listTree(t, dir)
	if n, err := removeManagedPiBackgroundTasks(cfg); err != nil || n != 0 {
		t.Errorf("second revert = (%d, %v), want (0, nil)", n, err)
	}
	if got := listTree(t, dir); !reflect.DeepEqual(got, before) {
		t.Errorf("second revert changed the tree to %v, want %v", got, before)
	}

	// The edited runner is someone's edit, so apply refuses while it is present and writes nothing. This replaces the
	// round-2 behaviour, which overwrote it. Once the edit is removed, apply repairs the runner, entry, spec and package.json.
	r := &applyReport{}
	if err := applyPiBackgroundTasks(r, cfg, nil); err == nil || !strings.Contains(err.Error(), "background-task-runner.ts") {
		t.Fatalf("apply over the edited runner: err = %v, want a refusal naming it", err)
	}
	if r.changes != 0 {
		t.Errorf("refused apply changed %d paths, want 0", r.changes)
	}
	assertFileBytes(t, lib, []byte("edited lib\n"))
	if err := os.Remove(lib); err != nil {
		t.Fatal(err)
	}
	if n := piBackgroundApply(t, cfg, nil); n != 4 {
		t.Errorf("apply after revert changed %d paths, want 4 (runner, entry, spec, package.json)", n)
	}
	if piBackgroundDiffChanged(t, cfg, nil) {
		t.Error("diff reports drift after the repairing apply")
	}
	repo, err := fs.ReadFile(harnez.DefaultFS, piBackgroundSourceFiles[1])
	if err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, lib, repo)
	assertFileBytes(t, notes, []byte("my notes\n"))
}

// An expected child that is a symlink refuses the install before any write, so no new package is discoverable.
func TestPiBackgroundChildSymlinkClashFailsClosed(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	dir := piBackgroundDirPath(root)
	writeTestFile(t, filepath.Join(dir, ".harnez-managed"), piBackgroundFinalLedger(t, cfg))
	external := t.TempDir()
	writeTestFile(t, filepath.Join(external, "keep.ts"), []byte("external\n"))
	mustSymlink(t, external, filepath.Join(dir, ".pi", "lib"))

	r := &applyReport{}
	if err := applyPiBackgroundTasks(r, cfg, nil); err == nil {
		t.Fatal("apply succeeded over a symlinked payload path")
	}
	if r.changes != 0 {
		t.Errorf("apply changed %d paths before refusing, want 0", r.changes)
	}
	for _, rel := range []string{"package.json", ".pi/extensions/background-tasks.ts", "spec/pi-background-tasks.yaml"} {
		if piBackgroundPresent(filepath.Join(dir, filepath.FromSlash(rel))) {
			t.Errorf("%s was written before the refusal", rel)
		}
	}
	assertFileBytes(t, filepath.Join(external, "keep.ts"), []byte("external\n"))
	if got := runPiLoader(t, root); got.Errors != 0 || len(got.Tools) != 0 {
		t.Errorf("real Pi loader sees a package after the refusal: %+v", got)
	}
}

// A payload parent swapped for a symlink after a good install: apply refuses and deactivates the package, since its entry
// now points outside. Pi's loader then registers nothing, and revert leaves the user's link and the external files alone.
func TestPiBackgroundPayloadParentSymlinkDeactivatesPackage(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	piBackgroundApply(t, cfg, nil)
	dir := piBackgroundDirPath(root)
	external := t.TempDir()
	writeTestFile(t, filepath.Join(external, "pi", "keep.ts"), []byte("external\n"))
	before := listTree(t, external)
	if err := os.RemoveAll(filepath.Join(dir, ".pi")); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, filepath.Join(external, "pi"), filepath.Join(dir, ".pi"))

	r := &applyReport{}
	err := applyPiBackgroundTasks(r, cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "deactivated") {
		t.Fatalf("apply error = %v, want a refusal that says the package was deactivated", err)
	}
	if piBackgroundPresent(filepath.Join(dir, "package.json")) {
		t.Error("package.json is still present; Pi would discover a package with a broken entry")
	}
	if got := runPiLoader(t, root); got.Errors != 0 || len(got.Tools) != 0 {
		t.Errorf("real Pi loader still registers the package: %+v", got)
	}
	if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
		t.Fatal(err)
	}
	if got := listTree(t, external); !reflect.DeepEqual(got, before) {
		t.Errorf("external tree changed to %v, want %v", got, before)
	}
	if info, err := os.Lstat(filepath.Join(dir, ".pi")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the user's symlink at .pi was touched by revert: %v", err)
	}
}

// A write that fails partway deactivates the package, keeps the marker, and reports it; a retry once the directory is
// writable completes the install. Skipped as root, which ignores directory permissions.
func TestPiBackgroundWriteFailureThenRetry(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("write-failure test needs a non-root user; root ignores directory permissions")
	}
	root, cfg := piBackgroundFixture(t)
	piBackgroundApply(t, cfg, nil)
	dir := piBackgroundDirPath(root)
	spec := filepath.Join(dir, "spec")
	if err := os.Remove(filepath.Join(spec, "pi-background-tasks.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(spec, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(spec, 0755) })

	r := &applyReport{}
	err := applyPiBackgroundTasks(r, cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "deactivated") {
		t.Fatalf("apply error = %v, want a write failure that says the package was deactivated", err)
	}
	if piBackgroundPresent(filepath.Join(dir, "package.json")) {
		t.Error("package.json is still present after a failed write")
	}
	if !piBackgroundPresent(filepath.Join(dir, ".harnez-managed")) {
		t.Error("marker was removed after a failed write; recovery needs it")
	}

	if err := os.Chmod(spec, 0755); err != nil {
		t.Fatal(err)
	}
	if n := piBackgroundApply(t, cfg, nil); n != 2 {
		t.Errorf("retry changed %d paths, want 2 (spec file and package.json)", n)
	}
	if piBackgroundDiffChanged(t, cfg, nil) {
		t.Error("diff reports drift after the retry")
	}
}

// Status prints one row per managed path with its state. A managed install the selection no longer wants is printed as
// "not selected", with the directory a revert would remove.
func TestPiBackgroundStatusOutputAndSelection(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	target := t.TempDir()
	piBackgroundApply(t, cfg, nil)
	dir := piBackgroundDirPath(root)

	out, err := captureStdout(t, func() error {
		return RunStatus("test.yaml", cfg, target, piBackgroundSelection(t, "full"))
	})
	if err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	for _, rel := range piBackgroundExpectedFiles {
		if line := statusLine(out, filepath.Join(dir, rel)); !strings.HasSuffix(line, " ok") {
			t.Errorf("status line for %s = %q, want state ok", rel, line)
		}
	}

	out, err = captureStdout(t, func() error {
		return RunStatus("test.yaml", cfg, target, piBackgroundSelection(t, "telemetry-only"))
	})
	if err != nil {
		t.Fatalf("RunStatus telemetry-only: %v", err)
	}
	if line := statusLine(out, dir); !strings.HasSuffix(line, "not selected") {
		t.Errorf("telemetry-only status line for %s = %q, want not selected", dir, line)
	}
}

// statusLine returns the first printed line that names path, or "" when none does.
func statusLine(out, path string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, path) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// captureStdout returns what fn prints to stdout. fn must not call t.Fatal, which would leave stdout redirected.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	ferr := fn()
	os.Stdout = saved
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out, ferr
}

// piLoaded is what Pi's own loader registered from an agent dir.
type piLoaded struct {
	Errors   int      `json:"errors"`
	Tools    []string `json:"tools"`
	Commands []string `json:"commands"`
}

// runPiLoader runs Pi's discovery and loading over root, as a session would. It skips when no Pi package or node is
// available. HOME is a temp dir, so the user's Pi config is never read.
func runPiLoader(t *testing.T, root string) piLoaded {
	t.Helper()
	pkgDir := locatePiPackageDir(t)
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("real Pi loader test skipped: node is not on PATH")
	}
	script := filepath.Join(t.TempDir(), "load.mjs")
	writeTestFile(t, script, []byte(piLoaderScript))
	cwd := t.TempDir()
	cmd := exec.Command(node, script, pkgDir, root, cwd)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), "PI_CODING_AGENT_DIR="+root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("real Pi loader: %v\n%s", err, out)
	}
	var loaded piLoaded
	if err := json.Unmarshal(out, &loaded); err != nil {
		t.Fatalf("loader output %q: %v", out, err)
	}
	return loaded
}

// piLoaderScript discovers and loads extensions the way Pi's session does, and prints the registered names as JSON.
const piLoaderScript = `import { join } from "node:path";
import { pathToFileURL } from "node:url";
const [pkgDir, agentDir, cwd] = process.argv.slice(2);
const url = (rel) => pathToFileURL(join(pkgDir, rel)).href;
const loader = await import(url("dist/core/extensions/loader.js"));
const { createEventBus } = await import(url("dist/core/event-bus.js"));
const result = await loader.discoverAndLoadExtensions([], cwd, agentDir, createEventBus());
const names = (key) => result.extensions.flatMap((extension) => [...extension[key].keys()]).sort();
process.stdout.write(JSON.stringify({ errors: result.errors.length, tools: names("tools"), commands: names("commands") }));
`

// locatePiPackageDir finds the installed Pi package the same way test/pi does: HARNEZ_PI_PACKAGE_DIR, else the pi launcher.
func locatePiPackageDir(t *testing.T) string {
	t.Helper()
	const loaderRel = "dist/core/extensions/loader.js"
	if dir := os.Getenv("HARNEZ_PI_PACKAGE_DIR"); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, loaderRel)); err != nil {
			t.Skipf("real Pi loader test skipped: HARNEZ_PI_PACKAGE_DIR has no %s", loaderRel)
		}
		return dir
	}
	launcher, err := exec.LookPath("pi")
	if err != nil {
		t.Skip("real Pi loader test skipped: no pi on PATH and HARNEZ_PI_PACKAGE_DIR is unset")
	}
	real, err := filepath.EvalSymlinks(launcher)
	if err != nil {
		t.Skipf("real Pi loader test skipped: %v", err)
	}
	agentDir := filepath.Dir(filepath.Dir(real))
	version, err := os.ReadFile(filepath.Join(agentDir, "install", "current-version"))
	if err != nil {
		t.Skipf("real Pi loader test skipped: no current-version under %s; set HARNEZ_PI_PACKAGE_DIR", agentDir)
	}
	dir := filepath.Join(agentDir, "install", "releases", strings.TrimSpace(string(version)), "node_modules", "@earendil-works", "pi-coding-agent")
	if _, err := os.Stat(filepath.Join(dir, loaderRel)); err != nil {
		t.Skipf("real Pi loader test skipped: Pi package not found at %s", dir)
	}
	return dir
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s: bytes differ (got %d bytes, want %d)", path, len(got), len(want))
	}
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

// listFiles is listTree without directories: only regular files and symlinks.
func listFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	for _, rel := range listTree(t, dir) {
		info, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			out = append(out, rel)
		}
	}
	return out
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func without(in []string, drop string) []string {
	var out []string
	for _, s := range in {
		if s != drop {
			out = append(out, s)
		}
	}
	return out
}
