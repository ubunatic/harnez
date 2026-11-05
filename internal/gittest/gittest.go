// Package gittest isolates tests from the developer's and CI's git config.
//
// A global core.hooksPath, url.<base>.insteadOf or similar setting changes
// how the git commands in tests behave (issue 1120: the Jules VM sets
// core.hooksPath=/dev/null, which disables the hooks tests rely on). Call
// Isolate from TestMain in every package whose tests run git.
package gittest

import (
	"os"
	"path/filepath"
)

// config is the only global git config tests see: an identity, so commits
// work, and the default branch the tests expect.
const config = `[user]
	name = harnez test
	email = test@harnez.invalid
[init]
	defaultBranch = main
`

// Isolate points GIT_CONFIG_GLOBAL at a fresh minimal config file and
// disables the system config for this process and its children. It returns
// a cleanup that removes the file; call it after m.Run.
func Isolate() (cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "harnez-gittest-")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL":   path,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_SYSTEM":   os.DevNull,
	} {
		if err := os.Setenv(k, v); err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
	}
	// GIT_CONFIG_COUNT/KEY/VALUE inject config too; drop any inherited ones.
	_ = os.Unsetenv("GIT_CONFIG_COUNT")
	return func() { _ = os.RemoveAll(dir) }, nil
}

// MustIsolate is Isolate for TestMain: it panics on error.
func MustIsolate() (cleanup func()) {
	cleanup, err := Isolate()
	if err != nil {
		panic("gittest: " + err.Error())
	}
	return cleanup
}
