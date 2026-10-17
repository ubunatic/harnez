//go:build !race

package telemetry

// raceEnabled reports whether the test binary is built with -race.
const raceEnabled = false
