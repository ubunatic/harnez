package main

import (
	"ubunatic.com/harnez/internal/subagent"
)

// appendSpoolSessions adds the spool entries the registry does not hold to xs.
// Each row's SOURCE column shows its trust state: spool-verified, spool-stale or
// spool-unverified. Nothing here writes to the spool or the registry, and host
// stop and delete never act on a spool row.
func appendSpoolSessions(xs []*subagent.Session, registry []*subagent.Session) ([]*subagent.Session, error) {
	known := make(map[string]bool, len(registry)+len(xs))
	for _, s := range registry {
		known[s.ID] = true
	}
	for _, s := range xs {
		known[s.ID] = true
	}
	entries, err := subagent.ReadSpoolEntries(known)
	if err != nil {
		return xs, err
	}
	for _, e := range entries {
		s := e.Session
		if s == nil {
			s = &subagent.Session{Name: e.ID}
		}
		s.ID = e.ID
		s.Source = "spool-" + e.State
		xs = append(xs, s)
	}
	return xs, nil
}
