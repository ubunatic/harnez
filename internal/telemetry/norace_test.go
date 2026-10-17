//go:build !race

package telemetry

// raceEnabled reports whether the test binary is built with -race.
const raceEnabled = false

// raceFactor scales wall-clock budgets: race instrumentation slows code
// several-fold.
const raceFactor = 1
