//go:build !race

package telemetry

// raceFactor scales wall-clock budgets: race instrumentation slows code
// several-fold.
const raceFactor = 1
