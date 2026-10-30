// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func updateSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		value := fmt.Sprintf("%v %v", info.Mode(), info.ModTime().UnixNano())
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			value += target
		} else if info.Mode().IsRegular() {
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			value += string(data)
		}
		result[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestPiBackgroundUpdateRefusesWithoutMutation(t *testing.T) {
	cases := map[string]func(*testing.T, string, *Config){
		"drift": func(t *testing.T, root string, cfg *Config) {
			writeTestFile(t, filepath.Join(piBackgroundDirPath(root), ".pi/lib/background-task-runner.ts"), []byte("edited"))
		},
		"unrecorded": func(t *testing.T, root string, cfg *Config) {
			writeTestFile(t, filepath.Join(piBackgroundDirPath(root), "extra"), []byte("user"))
		},
		"unrecorded target": func(t *testing.T, root string, cfg *Config) {
			p := filepath.Join(piBackgroundDirPath(root), ".harnez-managed")
			entries, err := parsePiLedger(piBackgroundFinalLedger(t, cfg))
			if err != nil {
				t.Fatal(err)
			}
			var kept []piLedgerEntry
			for _, e := range entries {
				if e.rel != ".pi/lib/background-task-runner.ts" {
					kept = append(kept, e)
				}
			}
			writeTestFile(t, p, renderPiLedger(kept))
		},
		"invalid ledger": func(t *testing.T, root string, cfg *Config) {
			writeTestFile(t, filepath.Join(piBackgroundDirPath(root), ".harnez-managed"), []byte("bad"))
		},
		"missing ledger": func(t *testing.T, root string, cfg *Config) {
			if err := os.Remove(filepath.Join(piBackgroundDirPath(root), ".harnez-managed")); err != nil {
				t.Fatal(err)
			}
		},
		"unsafe entry": func(t *testing.T, root string, cfg *Config) {
			writeTestFile(t, filepath.Join(piBackgroundDirPath(root), "index.ts"), []byte("user"))
		},
		"symlink": func(t *testing.T, root string, cfg *Config) {
			p := filepath.Join(piBackgroundDirPath(root), ".pi/lib/background-task-runner.ts")
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			mustSymlink(t, filepath.Join(t.TempDir(), "outside"), p)
		},
		"edited obsolete": func(t *testing.T, root string, cfg *Config) {
			p := piBackgroundInstallRetired(t, cfg, root, []byte("old"))
			writeTestFile(t, p, []byte("edited"))
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			root, cfg := piBackgroundFixture(t)
			if err := UpdatePiBackgroundTasks(cfg); err != nil {
				t.Fatal(err)
			}
			setup(t, root, cfg)
			before := updateSnapshot(t, root)
			if err := UpdatePiBackgroundTasks(cfg); err == nil {
				t.Fatal("expected refusal")
			}
			if after := updateSnapshot(t, root); !reflect.DeepEqual(before, after) {
				t.Fatalf("refusal changed tree: before %v, after %v", before, after)
			}
		})
	}
}

func TestPiBackgroundUpdateRecoveryAndLoader(t *testing.T) {
	for _, scenario := range []string{"fresh", "upgrade"} {
		for _, point := range piBackgroundCrashPoints() {
			t.Run(scenario+"/"+point, func(t *testing.T) {
				root, cfg := piBackgroundFixture(t)
				if scenario == "upgrade" {
					piBackgroundInstallV1(t, root, cfg, "")
				}
				piBackgroundFault = func(p string) error {
					if p == point {
						return errPiSimulatedCrash
					}
					return nil
				}
				err := UpdatePiBackgroundTasks(cfg)
				piBackgroundFault = nil
				if err == nil {
					t.Fatal("expected interruption")
				}
				piBackgroundAssertNoBrokenPackage(t, root, cfg)
				if err := UpdatePiBackgroundTasks(cfg); err != nil {
					t.Fatal(err)
				}
				assertFileBytes(t, filepath.Join(piBackgroundDirPath(root), ".harnez-managed"), piBackgroundFinalLedger(t, cfg))
				before := updateSnapshot(t, root)
				if err := UpdatePiBackgroundTasks(cfg); err != nil {
					t.Fatal(err)
				}
				if after := updateSnapshot(t, root); !reflect.DeepEqual(before, after) {
					t.Fatal("second update changed the tree")
				}
			})
		}
	}
	t.Run("real loader", func(t *testing.T) {
		root, cfg := piBackgroundFixture(t)
		if err := UpdatePiBackgroundTasks(cfg); err != nil {
			t.Fatal(err)
		}
		loaded := runPiLoader(t, root)
		if loaded.Errors != 0 || !reflect.DeepEqual(loaded.Tools, []string{"background_task", "background_task_cancel", "background_task_list", "background_task_result"}) {
			t.Fatalf("Pi loader: %+v", loaded)
		}
	})
}

func TestPiBackgroundUpdateTargetAndPreservation(t *testing.T) {
	root, cfg := piBackgroundFixture(t)
	configured := t.TempDir()
	cfg.PiAgentTarget = configured
	unrelated := filepath.Join(root, "extensions", "other", "file")
	writeTestFile(t, unrelated, []byte("keep"))
	before := updateSnapshot(t, configured)
	if err := UpdatePiBackgroundTasks(cfg); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, unrelated, []byte("keep"))
	if after := updateSnapshot(t, configured); !reflect.DeepEqual(before, after) {
		t.Fatal("configured target changed despite environment override")
	}
	t.Setenv("PI_CODING_AGENT_DIR", "")
	if err := UpdatePiBackgroundTasks(cfg); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, filepath.Join(piBackgroundDirPath(configured), ".harnez-managed"), piBackgroundFinalLedger(t, cfg))
	cfg.PiAgentTarget = ""
	if err := UpdatePiBackgroundTasks(cfg); err == nil {
		t.Fatal("expected missing target error")
	}
}
