package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/forgetracker"
	"ubunatic.com/harnez/internal/prwait"
)

// findIssuesLocalOnly lists `find issues` subcommands that the forge reader does not
// serve; they keep the local behaviour and its "no local issue tracker" error.
var findIssuesLocalOnly = map[string]bool{
	"next": true, "last": true, "history": true, "similar": true,
	"open": true, "blocked": true, "closed": true, "draft": true,
}

// forgeTrackerSource is the live forge reader; tests replace it.
var forgeTrackerSource = func(spec prwait.Spec) forgetracker.Source { return forgetracker.NewSource(spec) }

// resolveForgeTracker reports the forge that holds dir's tickets when dir has no
// issues/ dir (issue 1116). ok=false keeps the local tracker behaviour.
func resolveForgeTracker(stderr io.Writer, dir string) (forgetracker.Target, forgetracker.Source, prwait.Spec, bool, error) {
	// A local issues/ dir wins before the spec is even read, so a broken spec
	// never changes the local tracker's behaviour.
	if _, err := os.Stat(filepath.Join(dir, "issues")); !errors.Is(err, fs.ErrNotExist) {
		return forgetracker.Target{}, nil, prwait.Spec{}, false, nil
	}
	spec, err := prwait.LoadSpec()
	if err != nil {
		return forgetracker.Target{}, nil, spec, false, err
	}
	t, err := forgetracker.Resolve(dir, spec)
	if errors.Is(err, forgetracker.ErrNotApplicable) {
		return t, nil, spec, false, nil
	}
	if err != nil {
		return t, nil, spec, false, err
	}
	fmt.Fprintf(stderr, "no local issues/ dir: reading tickets from %s (read-only)\n", t)
	return t, forgeTrackerSource(spec), spec, true, nil
}

// forgeTrackerHint adds the token hint to a 404 or 401, which a private repo returns
// when no token is set.
func forgeTrackerHint(err error, t forgetracker.Target) error {
	if err == nil {
		return nil
	}
	var apiErr *forge.APIError
	if errors.As(err, &apiErr) && (apiErr.Status == 404 || apiErr.Status == 401) {
		return fmt.Errorf("%w (a private repo needs a token: set one of %s)", err, strings.Join(t.Forge.TokenEnv, ", "))
	}
	return err
}
