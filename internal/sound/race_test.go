//go:build race

package sound

// raceFactor scales wall-clock budgets: race instrumentation slows code
// several-fold.
const raceFactor = 4
