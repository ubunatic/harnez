//go:build race

package main

// raceEnabled reports whether the test binary is built with -race.
const raceEnabled = true

// raceFactor scales wall-clock budgets: race instrumentation slows code
// several-fold.
const raceFactor = 4
